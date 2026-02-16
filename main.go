package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
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
		planSpinner := ui.StartSpinner(os.Stderr, "Generating split plan")
		planText, err := client.ChatCompletion(ctx, prompt.SystemPrompt(), splitPrompt)
		planSpinner.Stop()
		if err != nil {
			fatal(err.Error())
		}
		plan, planErr := parseSplitPlan(planText)
		fmt.Println("Proposed commit plan:")
		fmt.Println("---")
		if planErr != nil {
			fmt.Println(planText)
			fmt.Println("---")
			fmt.Fprintf(os.Stderr, "gommit: unable to parse split plan JSON: %s\n", planErr.Error())
		} else {
			fmt.Println(formatSplitPlan(plan))
			fmt.Println("---")
		}

		ui.DisplayFileBox(os.Stdout, changedFiles, 5)
		fmt.Println()

		if autoAccept && forceSplit {
			if planErr != nil {
				fatal(planErr.Error())
			}
			if err := applySplitPlan(root, scope, plan, changedFiles); err != nil {
				fatal(err.Error())
			}
			fmt.Println("Split commits created.")
			return
		}
		optionsList := []string{}
		if planErr == nil {
			optionsList = append(optionsList, "Accept plan and commit")
		}
		optionsList = append(optionsList, "Force single commit message", "Cancel")

		action, err := ui.SelectOption("What would you like to do?", optionsList)
		if err != nil {
			fatal(err.Error())
		}
		switch action {
		case "Accept plan and commit":
			if err := applySplitPlan(root, scope, plan, changedFiles); err != nil {
				fatal(err.Error())
			}
			fmt.Println("Split commits created.")
			return
		case "Cancel":
			return
		case "Force single commit message":
			// continue to single-message flow
		}
	}

	for {
		singlePrompt := prompt.BuildSinglePromptWithMax(cfg.Style, scopeLabel, result.Diff, result.Binary, result.TruncatedFiles, cfg.MaxPromptChars)
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
			if err := commitMessage(root, message, scope); err != nil {
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
			continue
		case "Edit in editor":
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
		case "Accept":
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

type splitPlan struct {
	Commits []splitCommit `json:"commits"`
}

type splitCommit struct {
	Message   string   `json:"message"`
	Rationale string   `json:"rationale"`
	Files     []string `json:"files"`
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

func runGitOutput(root string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.Output()
	if err == nil {
		return string(out), nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return "", fmt.Errorf("git %s failed: %s", strings.Join(args, " "), strings.TrimSpace(string(exitErr.Stderr)))
	}
	return "", err
}

func parseSplitPlan(text string) (splitPlan, error) {
	raw := strings.TrimSpace(text)
	if raw == "" {
		return splitPlan{}, fmt.Errorf("empty split plan")
	}

	var plan splitPlan
	if err := json.Unmarshal([]byte(raw), &plan); err == nil {
		return plan, nil
	}

	var commits []splitCommit
	if err := json.Unmarshal([]byte(raw), &commits); err == nil {
		return splitPlan{Commits: commits}, nil
	}

	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		if err := json.Unmarshal([]byte(raw[start:end+1]), &plan); err == nil {
			return plan, nil
		}
	}
	start = strings.Index(raw, "[")
	end = strings.LastIndex(raw, "]")
	if start >= 0 && end > start {
		if err := json.Unmarshal([]byte(raw[start:end+1]), &commits); err == nil {
			return splitPlan{Commits: commits}, nil
		}
	}

	return splitPlan{}, fmt.Errorf("invalid JSON split plan")
}

func applySplitPlan(root string, scope git.DiffScope, plan splitPlan, changedFiles []string) error {
	if len(plan.Commits) == 0 {
		return fmt.Errorf("split plan has no commits")
	}

	expected := map[string]struct{}{}
	for _, file := range changedFiles {
		expected[file] = struct{}{}
	}

	seen := map[string]struct{}{}
	for i, commit := range plan.Commits {
		msg := strings.TrimSpace(commit.Message)
		if msg == "" {
			return fmt.Errorf("commit %d missing message", i+1)
		}
		if len(commit.Files) == 0 {
			return fmt.Errorf("commit %d has no files", i+1)
		}
		for _, file := range commit.Files {
			cleaned, err := sanitizePlanFile(file)
			if err != nil {
				return fmt.Errorf("commit %d has invalid file %q: %w", i+1, file, err)
			}
			if _, ok := expected[cleaned]; !ok {
				return fmt.Errorf("commit %d references unknown file %q", i+1, cleaned)
			}
			if _, ok := seen[cleaned]; ok {
				return fmt.Errorf("file %q appears in multiple commits", cleaned)
			}
			seen[cleaned] = struct{}{}
		}
	}

	if len(seen) != len(expected) {
		var missing []string
		for file := range expected {
			if _, ok := seen[file]; !ok {
				missing = append(missing, file)
			}
		}
		sort.Strings(missing)
		return fmt.Errorf("split plan missing files: %s", strings.Join(missing, ", "))
	}

	if scope == git.ScopeStaged {
		dirty, err := hasUnstagedChanges(root)
		if err != nil {
			return err
		}
		if dirty {
			return fmt.Errorf("unstaged changes detected; staged-only split commits require a clean working tree")
		}
	}

	for i, commit := range plan.Commits {
		fmt.Printf("Creating commit %d/%d: %s\n", i+1, len(plan.Commits), strings.TrimSpace(commit.Message))
		if err := runGitCmd(root, "reset", "-q"); err != nil {
			return err
		}
		files, err := sanitizePlanFiles(commit.Files)
		if err != nil {
			return err
		}
		if err := stageFiles(root, files); err != nil {
			return err
		}
		if err := commitMessage(root, commit.Message, git.ScopeStaged); err != nil {
			return err
		}
	}

	return nil
}

func formatSplitPlan(plan splitPlan) string {
	var b strings.Builder
	for i, commit := range plan.Commits {
		if i > 0 {
			b.WriteString("\n")
		}
		message := strings.TrimSpace(commit.Message)
		if message == "" {
			message = "(no message)"
		}
		b.WriteString(fmt.Sprintf("%d. %s\n", i+1, message))
		rationale := strings.TrimSpace(commit.Rationale)
		if rationale != "" {
			b.WriteString("Rationale: " + rationale + "\n")
		}
		if len(commit.Files) > 0 {
			files := make([]string, 0, len(commit.Files))
			for _, file := range commit.Files {
				file = strings.TrimSpace(file)
				if file == "" {
					continue
				}
				files = append(files, file)
			}
			if len(files) > 0 {
				b.WriteString("Files: " + strings.Join(files, ", ") + "\n")
			}
		}
	}
	return strings.TrimSpace(b.String())
}

func sanitizePlanFiles(files []string) ([]string, error) {
	out := make([]string, 0, len(files))
	for _, file := range files {
		cleaned, err := sanitizePlanFile(file)
		if err != nil {
			return nil, err
		}
		out = append(out, cleaned)
	}
	return out, nil
}

func sanitizePlanFile(path string) (string, error) {
	cleaned := strings.TrimSpace(path)
	if cleaned == "" {
		return "", fmt.Errorf("empty path")
	}
	cleaned = strings.TrimPrefix(cleaned, "a/")
	cleaned = strings.TrimPrefix(cleaned, "b/")
	cleaned = strings.TrimPrefix(cleaned, "./")
	cleaned = filepath.Clean(cleaned)
	if cleaned == "." || strings.HasPrefix(cleaned, "..") || filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("invalid path")
	}
	return cleaned, nil
}

func stageFiles(root string, files []string) error {
	args := append([]string{"add", "--"}, files...)
	return runGitCmd(root, args...)
}

func hasUnstagedChanges(root string) (bool, error) {
	out, err := runGitOutput(root, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	for _, line := range strings.Split(out, "\n") {
		if len(line) < 2 {
			continue
		}
		if line[1] != ' ' {
			return true, nil
		}
	}
	return false, nil
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
