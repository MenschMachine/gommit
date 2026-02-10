package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gommit/internal/config"
	"gommit/internal/git"
	"gommit/internal/llm"
	"gommit/internal/prompt"
	"gommit/internal/ui"
)

const (
	scopeStaged           = "staged only"
	scopeStagedUnstaged   = "staged + unstaged"
	scopeAllWithUntracked = "staged + unstaged + untracked"
)

func main() {
	var includeUnstaged bool
	var includeAll bool
	var forceSingle bool
	var forceSplit bool
	var providerFlag string
	var modelFlag string
	var baseURLFlag string
	var styleFlag string
	var configPathFlag string
	var openRouterRefFlag string
	var openRouterTitleFlag string

	flag.BoolVar(&includeUnstaged, "a", false, "include staged + unstaged")
	flag.BoolVar(&includeAll, "A", false, "include staged + unstaged + untracked")
	flag.BoolVar(&forceSingle, "single", false, "force single message even if diff is large")
	flag.BoolVar(&forceSplit, "split", false, "force split-mode plan")
	flag.StringVar(&providerFlag, "provider", "", "llm provider (openai, openrouter, anthropic)")
	flag.StringVar(&modelFlag, "model", "", "model name")
	flag.StringVar(&baseURLFlag, "base-url", "", "base url for openai-compatible api")
	flag.StringVar(&styleFlag, "style", "", "commit style (conventional or freeform)")
	flag.StringVar(&configPathFlag, "config", "", "path to config file")
	flag.StringVar(&openRouterRefFlag, "openrouter-referer", "", "openrouter HTTP-Referer header")
	flag.StringVar(&openRouterTitleFlag, "openrouter-title", "", "openrouter X-Title header")
	flag.Parse()

	if forceSingle && forceSplit {
		fatal("--single and --split cannot be used together")
	}

	cfgPath := configPathFlag
	if cfgPath == "" {
		var err error
		cfgPath, err = config.DefaultConfigPath()
		if err != nil {
			fatal(err.Error())
		}
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		fatal(err.Error())
	}
	config.ApplyEnvOverrides(&cfg)

	if providerFlag != "" {
		cfg.Provider = providerFlag
	}
	if modelFlag != "" {
		cfg.Model = modelFlag
	}
	if baseURLFlag != "" {
		cfg.BaseURL = baseURLFlag
	}
	if styleFlag != "" {
		cfg.Style = styleFlag
	}
	if openRouterRefFlag != "" {
		cfg.OpenRouterRef = openRouterRefFlag
	}
	if openRouterTitleFlag != "" {
		cfg.OpenRouterTitle = openRouterTitleFlag
	}

	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))
	if provider == "" {
		provider = "openai"
	}

	if cfg.BaseURL == "" {
		cfg.BaseURL = config.DefaultBaseURL(provider)
	}
	if provider == "anthropic" && cfg.BaseURL == "" {
		fatal("anthropic requires an OpenAI-compatible base URL; set --base-url or config base_url")
	}
	if cfg.Model == "" {
		fatal("model is required; set --model or config model")
	}

	apiKey, err := config.ResolveAPIKey(provider)
	if err != nil {
		fatal(err.Error())
	}

	root, err := git.RepoRoot()
	if err != nil {
		fatal(err.Error())
	}

	scope := git.ScopeStaged
	scopeLabel := scopeStaged
	if includeAll {
		scope = git.ScopeAll
		scopeLabel = scopeAllWithUntracked
	} else if includeUnstaged {
		scope = git.ScopeStagedUnstaged
		scopeLabel = scopeStagedUnstaged
	}

	result, err := git.CollectDiff(root, scope, cfg.PerFileLimit)
	if err != nil {
		fatal(err.Error())
	}
	if strings.TrimSpace(result.Diff) == "" && len(result.Binary) == 0 {
		fatal("no changes found for selected diff scope")
	}

	headers := map[string]string{}
	if provider == "openrouter" {
		if cfg.OpenRouterRef != "" {
			headers["HTTP-Referer"] = cfg.OpenRouterRef
		}
		if cfg.OpenRouterTitle != "" {
			headers["X-Title"] = cfg.OpenRouterTitle
		}
	}
	client := llm.NewClient(cfg.BaseURL, apiKey, cfg.Model, headers)
	ctx := context.Background()
	reader := bufio.NewReader(os.Stdin)

	splitMode := forceSplit || (!forceSingle && result.TotalOriginalLen > cfg.SplitThreshold)
	if splitMode {
		splitPrompt := prompt.BuildSplitPrompt(cfg.Style, scopeLabel, result.Diff, result.Binary, result.TruncatedFiles)
		planText, err := client.ChatCompletion(ctx, prompt.SystemPrompt(), splitPrompt)
		if err != nil {
			fatal(err.Error())
		}
		fmt.Println("Proposed commit plan:")
		fmt.Println("---")
		fmt.Println(planText)
		fmt.Println("---")
		choice, err := ui.PromptChoice(reader, os.Stdout, "Split-mode options", map[rune]string{
			'a': "accept plan and exit",
			'f': "force single commit message",
			'c': "cancel",
		})
		if err != nil {
			fatal(err.Error())
		}
		switch choice {
		case 'a':
			return
		case 'c':
			return
		case 'f':
			// continue to single-message flow
		}
	}

	for {
		singlePrompt := prompt.BuildSinglePrompt(cfg.Style, scopeLabel, result.Diff, result.Binary, result.TruncatedFiles)
		message, err := client.ChatCompletion(ctx, prompt.SystemPrompt(), singlePrompt)
		if err != nil {
			fatal(err.Error())
		}
		fmt.Println("Proposed commit message:")
		fmt.Println("---")
		fmt.Println(message)
		fmt.Println("---")

		choice, err := ui.PromptChoice(reader, os.Stdout, "Choose action", map[rune]string{
			'a': "accept",
			'e': "edit",
			'r': "retry",
			'c': "cancel",
		})
		if err != nil {
			fatal(err.Error())
		}
		switch choice {
		case 'c':
			return
		case 'r':
			continue
		case 'e':
			message, err = ui.EditInEditor(message)
			if err != nil {
				fatal(err.Error())
			}
			if strings.TrimSpace(message) == "" {
				fatal("empty commit message after edit")
			}
			if err := commitMessage(root, message); err != nil {
				fatal(err.Error())
			}
			fmt.Println("Commit created.")
			return
		case 'a':
			if strings.TrimSpace(message) == "" {
				fatal("empty commit message")
			}
			if err := commitMessage(root, message); err != nil {
				fatal(err.Error())
			}
			fmt.Println("Commit created.")
			return
		}
	}
}

func commitMessage(root, message string) error {
	file, err := os.CreateTemp("", "gommit-commit-*.txt")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.WriteString(strings.TrimSpace(message) + "\n"); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}

	cmd := exec.Command("git", "commit", "-F", filepath.Clean(file.Name()))
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, "gommit:", msg)
	os.Exit(1)
}
