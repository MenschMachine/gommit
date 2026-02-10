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

	"github.com/MenschMachine/gommit/internal/config"
	"github.com/MenschMachine/gommit/internal/git"
	"github.com/MenschMachine/gommit/internal/llm"
	"github.com/MenschMachine/gommit/internal/prompt"
	"github.com/MenschMachine/gommit/internal/ui"
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
	var autoAccept bool
	var dumpContext bool
	var maxPromptCharsFlag int
	var providerFlag string
	var modelFlag string
	var baseURLFlag string
	var styleFlag string
	var configPathFlag string
	var openRouterRefFlag string
	var openRouterTitleFlag string

	flag.Usage = func() {
		out := flag.CommandLine.Output()
		cfgDefaults := config.DefaultConfig()
		cfgPath, err := config.DefaultConfigPath()
		if err != nil {
			cfgPath = "~/.config/gommit/config.toml"
		}

		fmt.Fprintln(out, "Usage: gommit [options]")
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, "Options:")
		fmt.Fprintln(out, "  -u, --include-unstaged   include staged + unstaged")
		fmt.Fprintln(out, "  -A, --include-all        include staged + unstaged + untracked")
		fmt.Fprintln(out, "  -s, --single             force single message even if diff is large")
		fmt.Fprintln(out, "  -S, --split              force split-mode plan")
		fmt.Fprintln(out, "  -f, --accept             auto-accept proposed result")
		fmt.Fprintln(out, "  -d, --dump-context       print LLM request JSON and exit")
		fmt.Fprintln(out, "      --max-prompt-chars   max chars for user prompt (0 = no limit)")
		fmt.Fprintf(out, "  -p, --provider string    llm provider (openai, openrouter, anthropic) (default: %s)\n", cfgDefaults.Provider)
		fmt.Fprintln(out, "  -m, --model string       model name (required unless set in config/env)")
		fmt.Fprintln(out, "  -b, --base-url string    base url for openai-compatible api")
		fmt.Fprintf(out, "                           default: %s (openai), %s (openrouter)\n", config.DefaultBaseURL("openai"), config.DefaultBaseURL("openrouter"))
		fmt.Fprintf(out, "  -t, --style string       commit style (conventional or freeform) (default: %s)\n", cfgDefaults.Style)
		fmt.Fprintf(out, "  -c, --config string      path to config file (default: %s)\n", cfgPath)
		fmt.Fprintln(out, "  -r, --openrouter-referer string  openrouter HTTP-Referer header")
		fmt.Fprintln(out, "  -T, --openrouter-title string    openrouter X-Title header")
	}

	flag.BoolVar(&includeUnstaged, "u", false, "include staged + unstaged")
	flag.BoolVar(&includeUnstaged, "include-unstaged", false, "include staged + unstaged")
	flag.BoolVar(&includeAll, "A", false, "include staged + unstaged + untracked")
	flag.BoolVar(&includeAll, "include-all", false, "include staged + unstaged + untracked")
	flag.BoolVar(&forceSingle, "s", false, "force single message even if diff is large")
	flag.BoolVar(&forceSingle, "single", false, "force single message even if diff is large")
	flag.BoolVar(&forceSplit, "S", false, "force split-mode plan")
	flag.BoolVar(&forceSplit, "split", false, "force split-mode plan")
	flag.BoolVar(&autoAccept, "f", false, "auto-accept proposed result")
	flag.BoolVar(&autoAccept, "accept", false, "auto-accept proposed result")
	flag.BoolVar(&dumpContext, "d", false, "print LLM request JSON and exit")
	flag.BoolVar(&dumpContext, "dump-context", false, "print LLM request JSON and exit")
	flag.IntVar(&maxPromptCharsFlag, "max-prompt-chars", -1, "max chars for user prompt (0 = no limit)")
	flag.StringVar(&providerFlag, "p", "", "llm provider (openai, openrouter, anthropic)")
	flag.StringVar(&providerFlag, "provider", "", "llm provider (openai, openrouter, anthropic)")
	flag.StringVar(&modelFlag, "m", "", "model name")
	flag.StringVar(&modelFlag, "model", "", "model name")
	flag.StringVar(&baseURLFlag, "b", "", "base url for openai-compatible api")
	flag.StringVar(&baseURLFlag, "base-url", "", "base url for openai-compatible api")
	flag.StringVar(&styleFlag, "t", "", "commit style (conventional or freeform)")
	flag.StringVar(&styleFlag, "style", "", "commit style (conventional or freeform)")
	flag.StringVar(&configPathFlag, "c", "", "path to config file")
	flag.StringVar(&configPathFlag, "config", "", "path to config file")
	flag.StringVar(&openRouterRefFlag, "r", "", "openrouter HTTP-Referer header")
	flag.StringVar(&openRouterRefFlag, "openrouter-referer", "", "openrouter HTTP-Referer header")
	flag.StringVar(&openRouterTitleFlag, "T", "", "openrouter X-Title header")
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
	if maxPromptCharsFlag >= 0 {
		cfg.MaxPromptChars = maxPromptCharsFlag
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
	if autoAccept && !forceSplit {
		splitMode = false
	}
	if splitMode {
		splitPrompt := prompt.BuildSplitPromptWithMax(cfg.Style, scopeLabel, result.Diff, result.Binary, result.TruncatedFiles, cfg.MaxPromptChars)
		if dumpContext {
			dumpLLMContext(client, prompt.SystemPrompt(), splitPrompt)
			return
		}
		planText, err := client.ChatCompletion(ctx, prompt.SystemPrompt(), splitPrompt)
		if err != nil {
			fatal(err.Error())
		}
		fmt.Println("Proposed commit plan:")
		fmt.Println("---")
		fmt.Println(planText)
		fmt.Println("---")
		if autoAccept && forceSplit {
			return
		}
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
		singlePrompt := prompt.BuildSinglePromptWithMax(cfg.Style, scopeLabel, result.Diff, result.Binary, result.TruncatedFiles, cfg.MaxPromptChars)
		if dumpContext {
			dumpLLMContext(client, prompt.SystemPrompt(), singlePrompt)
			return
		}
		message, err := client.ChatCompletion(ctx, prompt.SystemPrompt(), singlePrompt)
		if err != nil {
			fatal(err.Error())
		}
		fmt.Println("Proposed commit message:")
		fmt.Println("---")
		fmt.Println(message)
		fmt.Println("---")

		if autoAccept {
			if strings.TrimSpace(message) == "" {
				fatal("empty commit message")
			}
			if err := commitMessage(root, message, scope); err != nil {
				fatal(err.Error())
			}
			fmt.Println("Commit created.")
			return
		}

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
			if err := commitMessage(root, message, scope); err != nil {
				fatal(err.Error())
			}
			fmt.Println("Commit created.")
			return
		case 'a':
			if strings.TrimSpace(message) == "" {
				fatal("empty commit message")
			}
			if err := commitMessage(root, message, scope); err != nil {
				fatal(err.Error())
			}
			fmt.Println("Commit created.")
			return
		}
	}
}

func commitMessage(root, message string, scope git.DiffScope) error {
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

	var cmd *exec.Cmd
	switch scope {
	case git.ScopeStaged:
		cmd = exec.Command("git", "commit", "-F", filepath.Clean(file.Name()))
	case git.ScopeStagedUnstaged:
		cmd = exec.Command("git", "commit", "-a", "-F", filepath.Clean(file.Name()))
	case git.ScopeAll:
		if err := runGitCmd(root, "add", "."); err != nil {
			return err
		}
		cmd = exec.Command("git", "commit", "-a", "-F", filepath.Clean(file.Name()))
	default:
		cmd = exec.Command("git", "commit", "-F", filepath.Clean(file.Name()))
	}
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func runGitCmd(root string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func dumpLLMContext(client *llm.Client, systemPrompt, userPrompt string) {
	payload, err := client.BuildChatPayload(systemPrompt, userPrompt)
	if err != nil {
		fatal(err.Error())
	}
	fmt.Println(string(payload))
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, "gommit:", msg)
	os.Exit(1)
}
