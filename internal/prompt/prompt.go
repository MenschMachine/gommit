package prompt

import (
	"fmt"
	"sort"
	"strings"

	"github.com/MenschMachine/gommit/internal/git"
)

const systemPrompt = "You are a senior software engineer who writes precise git commit messages."

func SystemPrompt() string {
	return systemPrompt
}

func BuildSinglePrompt(style string, scope string, diff string, binaries []git.BinaryFile, truncated []string) string {
	var b strings.Builder
	b.WriteString("Generate a git commit message for the following changes.\n")
	b.WriteString(fmt.Sprintf("Diff scope: %s.\n", scope))
	b.WriteString("\n")

	if strings.ToLower(style) == "conventional" {
		b.WriteString("Use Conventional Commits. Format: type(scope): summary. Summary <= 72 chars, imperative, no trailing period.\n")
		b.WriteString("Allowed types: feat, fix, docs, style, refactor, perf, test, build, ci, chore, revert.\n")
		b.WriteString("Include body if useful, separated by a blank line.\n")
	} else {
		b.WriteString("Write a concise summary line (<= 72 chars) and an optional body if helpful.\n")
	}

	if len(truncated) > 0 {
		sort.Strings(truncated)
		b.WriteString("\nNote: some file diffs were truncated due to size:\n")
		for _, path := range truncated {
			b.WriteString("- " + path + "\n")
		}
	}

	if len(binaries) > 0 {
		b.WriteString("\nBinary files changed (content omitted):\n")
		sort.Slice(binaries, func(i, j int) bool { return binaries[i].Path < binaries[j].Path })
		for _, bf := range binaries {
			size := "unknown"
			if bf.Size >= 0 {
				size = fmt.Sprintf("%d bytes", bf.Size)
			}
			b.WriteString(fmt.Sprintf("- %s (%s)\n", bf.Path, size))
		}
	}

	b.WriteString("\nDiff:\n")
	b.WriteString(diff)
	b.WriteString("\n\nReturn only the commit message, no code fences or extra commentary.")
	return b.String()
}

func BuildSplitPrompt(style string, scope string, diff string, binaries []git.BinaryFile, truncated []string) string {
	var b strings.Builder
	b.WriteString("The diff is large. Propose a coherent multi-commit plan.\n")
	b.WriteString(fmt.Sprintf("Diff scope: %s.\n", scope))
	b.WriteString("\n")

	if strings.ToLower(style) == "conventional" {
		b.WriteString("Use Conventional Commits for each message. Format: type(scope): summary.\n")
	} else {
		b.WriteString("Use concise commit messages with summary line and optional body.\n")
	}

	b.WriteString("Return a plan with:\n")
	b.WriteString("1) Total number of commits\n")
	b.WriteString("2) For each commit: message, short rationale, and affected files or areas\n")

	if len(truncated) > 0 {
		sort.Strings(truncated)
		b.WriteString("\nNote: some file diffs were truncated due to size:\n")
		for _, path := range truncated {
			b.WriteString("- " + path + "\n")
		}
	}

	if len(binaries) > 0 {
		b.WriteString("\nBinary files changed (content omitted):\n")
		sort.Slice(binaries, func(i, j int) bool { return binaries[i].Path < binaries[j].Path })
		for _, bf := range binaries {
			size := "unknown"
			if bf.Size >= 0 {
				size = fmt.Sprintf("%d bytes", bf.Size)
			}
			b.WriteString(fmt.Sprintf("- %s (%s)\n", bf.Path, size))
		}
	}

	b.WriteString("\nDiff:\n")
	b.WriteString(diff)
	b.WriteString("\n\nReturn only the plan, no code fences or extra commentary.")
	return b.String()
}
