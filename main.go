package main

import (
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

var version = "dev"

const (
	scopeStaged           = "staged only"
	scopeStagedUnstaged   = "staged + unstaged"
	scopeAllWithUntracked = "staged + unstaged + untracked"
)

func main() {
	var includeUnstaged bool
	var includeAll bool
	var autoAccept bool
	var dumpContext bool
	var showVersion bool
	var maxPromptCharsFlag int
	var providerFlag string
	var modelFlag string
	var baseURLFlag string
	var styleFlag string
	var configPathFlag string
	var tagFlag string
	var skipCI bool
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
		fmt.Fprintln(out, "  --version                show version and exit")
		fmt.Fprintln(out, "  -u, --include-unstaged   include staged + unstaged")
		fmt.Fprintln(out, "  -A, --include-all        include staged + unstaged + untracked")
		fmt.Fprintln(out, "  -f, --accept             auto-accept proposed result")
		fmt.Fprintln(out, "  -d, --dump-context       print LLM request JSON and exit")
		fmt.Fprintln(out, "      --max-prompt-chars   max chars for user prompt (0 = no limit)")
		fmt.Fprintf(out, "  -p, --provider string    llm provider (openai, openrouter, anthropic) (default: %s)\n", cfgDefaults.Provider)
		fmt.Fprintln(out, "  -m, --model string       model name (required unless set in config/env)")
		fmt.Fprintln(out, "  -b, --base-url string    base url for openai-compatible api")
		fmt.Fprintf(out, "                           default: %s (openai), %s (openrouter)\n", config.DefaultBaseURL("openai"), config.DefaultBaseURL("openrouter"))
		fmt.Fprintln(out, "  -t, --tag string         append [STRING] to commit message")
		fmt.Fprintln(out, "  -s, --skip-ci            shortcut for --tag \"skip ci\"")
		fmt.Fprintf(out, "      --style string       commit style (conventional or freeform) (default: %s)\n", cfgDefaults.Style)
		fmt.Fprintf(out, "  -c, --config string      path to config file (default: %s)\n", cfgPath)
		fmt.Fprintln(out, "  -r, --openrouter-referer string  openrouter HTTP-Referer header")
		fmt.Fprintln(out, "  -T, --openrouter-title string    openrouter X-Title header")
	}

	flag.BoolVar(&includeUnstaged, "u", false, "include staged + unstaged")
	flag.BoolVar(&includeUnstaged, "include-unstaged", false, "include staged + unstaged")
	flag.BoolVar(&includeAll, "A", false, "include staged + unstaged + untracked")
	flag.BoolVar(&includeAll, "include-all", false, "include staged + unstaged + untracked")
	flag.BoolVar(&autoAccept, "f", false, "auto-accept proposed result")
	flag.BoolVar(&autoAccept, "accept", false, "auto-accept proposed result")
	flag.BoolVar(&dumpContext, "d", false, "print LLM request JSON and exit")
	flag.BoolVar(&dumpContext, "dump-context", false, "print LLM request JSON and exit")
	flag.BoolVar(&showVersion, "version", false, "show version and exit")
	flag.IntVar(&maxPromptCharsFlag, "max-prompt-chars", -1, "max chars for user prompt (0 = no limit)")
	flag.StringVar(&providerFlag, "p", "", "llm provider (openai, openrouter, anthropic)")
	flag.StringVar(&providerFlag, "provider", "", "llm provider (openai, openrouter, anthropic)")
	flag.StringVar(&modelFlag, "m", "", "model name")
	flag.StringVar(&modelFlag, "model", "", "model name")
	flag.StringVar(&baseURLFlag, "b", "", "base url for openai-compatible api")
	flag.StringVar(&baseURLFlag, "base-url", "", "base url for openai-compatible api")
	flag.StringVar(&tagFlag, "t", "", "append [STRING] to commit message")
	flag.StringVar(&tagFlag, "tag", "", "append [STRING] to commit message")
	flag.BoolVar(&skipCI, "s", false, "shortcut for --tag \"skip ci\"")
	flag.BoolVar(&skipCI, "skip-ci", false, "shortcut for --tag \"skip ci\"")
	flag.StringVar(&styleFlag, "style", "", "commit style (conventional or freeform)")
	flag.StringVar(&configPathFlag, "c", "", "path to config file")
	flag.StringVar(&configPathFlag, "config", "", "path to config file")
	flag.StringVar(&openRouterRefFlag, "r", "", "openrouter HTTP-Referer header")
	flag.StringVar(&openRouterRefFlag, "openrouter-referer", "", "openrouter HTTP-Referer header")
	flag.StringVar(&openRouterTitleFlag, "T", "", "openrouter X-Title header")
	flag.StringVar(&openRouterTitleFlag, "openrouter-title", "", "openrouter X-Title header")
	flag.Parse()

	if showVersion {
		fmt.Println("gommit", version)
		return
	}

	if skipCI {
		if tagFlag != "" {
			fatal("--tag and --skip-ci cannot be used together")
		}
		tagFlag = "skip ci"
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

	diffSpinner := ui.StartSpinner(os.Stderr, "Collecting diff")
	result, err := git.CollectDiff(root, scope, cfg.PerFileLimit)
	diffSpinner.Stop()
	if err != nil {
		fatal(err.Error())
	}
	if strings.TrimSpace(result.Diff) == "" && len(result.Binary) == 0 {
		fatal("no changes found for selected diff scope")
	}
	changedFiles := changedFilesFromResult(result)

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

	var refinementHint string
	for {
		singlePrompt := prompt.BuildSinglePromptWithMax(cfg.Style, scopeLabel, result.Diff, result.Binary, result.TruncatedFiles, cfg.MaxPromptChars)

		// Append refinement hint if provided
		if refinementHint != "" {
			singlePrompt += "\n\nAdditional guidance: " + refinementHint
		}

		if dumpContext {
			dumpLLMContext(client, prompt.SystemPrompt(), singlePrompt)
			return
		}
		msgSpinner := ui.StartSpinner(os.Stderr, "Generating commit message")
		message, err := client.ChatCompletion(ctx, prompt.SystemPrompt(), singlePrompt)
		msgSpinner.Stop()
		if err != nil {
			fatal(err.Error())
		}

		// Clear refinement hint after use
		refinementHint = ""
		fmt.Println("Proposed commit message:")
		fmt.Println("---")
		fmt.Println(message)
		fmt.Println("---")

		ui.DisplayFileBox(os.Stdout, changedFiles, 5)
		fmt.Println()

		if autoAccept {
			if strings.TrimSpace(message) == "" {
				fatal("empty commit message")
			}
			if err := commitMessage(root, appendTag(message, tagFlag), scope); err != nil {
				fatal(err.Error())
			}
			fmt.Println("Commit created.")
			return
		}

		action, err := ui.SelectOption(
			"What would you like to do with this commit message?",
			[]string{"Accept", "Edit in editor", "Retry generation", "Cancel"},
		)
		if err != nil {
			fatal(err.Error())
		}
		switch action {
		case "Cancel":
			return
		case "Retry generation":
			hint, err := ui.PromptInput(
				"Refinement hint (optional, press Enter to skip)",
				"e.g., make it shorter, focus on bug fix, etc.",
			)
			if err != nil {
				fatal(err.Error())
			}
			refinementHint = strings.TrimSpace(hint)
			continue
		case "Edit in editor":
			message, err = ui.EditInEditor(message)
			if err != nil {
				fatal(err.Error())
			}
			if strings.TrimSpace(message) == "" {
				fatal("empty commit message after edit")
			}
			if err := commitMessage(root, appendTag(message, tagFlag), scope); err != nil {
				fatal(err.Error())
			}
			fmt.Println("Commit created.")
			return
		case "Accept":
			if strings.TrimSpace(message) == "" {
				fatal("empty commit message")
			}
			if err := commitMessage(root, appendTag(message, tagFlag), scope); err != nil {
				fatal(err.Error())
			}
			fmt.Println("Commit created.")
			return
		}
	}
}

func appendTag(message, tag string) string {
	if tag == "" {
		return message
	}
	return strings.TrimRight(message, "\n") + " [" + tag + "]"
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

func changedFilesFromResult(result git.DiffResult) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, chunk := range git.SplitDiffChunks(result.Diff) {
		path := strings.TrimSpace(git.ParseDiffPath(chunk))
		if path == "" {
			continue
		}
		path = filepath.Clean(path)
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		out = append(out, path)
	}
	for _, bf := range result.Binary {
		path := strings.TrimSpace(bf.Path)
		if path == "" {
			continue
		}
		path = filepath.Clean(path)
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		out = append(out, path)
	}
	return out
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
