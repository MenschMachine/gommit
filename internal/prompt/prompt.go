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

func BuildSinglePromptWithMax(style string, scope string, diff string, binaries []git.BinaryFile, truncated []string, maxChars int) string {
	if maxChars <= 0 {
		return BuildSinglePrompt(style, scope, diff, binaries, truncated)
	}
	return buildPromptWithMax(false, style, scope, diff, binaries, truncated, maxChars)
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

func BuildSplitPromptWithMax(style string, scope string, diff string, binaries []git.BinaryFile, truncated []string, maxChars int) string {
	if maxChars <= 0 {
		return BuildSplitPrompt(style, scope, diff, binaries, truncated)
	}
	return buildPromptWithMax(true, style, scope, diff, binaries, truncated, maxChars)
}

type diffChunk struct {
	Path string
	Text string
}

func buildPromptWithMax(split bool, style string, scope string, diff string, binaries []git.BinaryFile, truncated []string, maxChars int) string {
	var b strings.Builder
	if split {
		b.WriteString("The diff is large. Propose a coherent multi-commit plan.\n")
	} else {
		b.WriteString("Generate a git commit message for the following changes.\n")
	}
	b.WriteString(fmt.Sprintf("Diff scope: %s.\n", scope))
	b.WriteString("\n")

	if strings.ToLower(style) == "conventional" {
		if split {
			b.WriteString("Use Conventional Commits for each message. Format: type(scope): summary.\n")
		} else {
			b.WriteString("Use Conventional Commits. Format: type(scope): summary. Summary <= 72 chars, imperative, no trailing period.\n")
			b.WriteString("Allowed types: feat, fix, docs, style, refactor, perf, test, build, ci, chore, revert.\n")
			b.WriteString("Include body if useful, separated by a blank line.\n")
		}
	} else if split {
		b.WriteString("Use concise commit messages with summary line and optional body.\n")
	} else {
		b.WriteString("Write a concise summary line (<= 72 chars) and an optional body if helpful.\n")
	}

	b.WriteString("\nNote: diff detail may be reduced to fit max_prompt_chars.\n")

	if split {
		b.WriteString("Return a plan with:\n")
		b.WriteString("1) Total number of commits\n")
		b.WriteString("2) For each commit: message, short rationale, and affected files or areas\n")
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

	chunks := parseDiffChunks(diff)
	files := collectFiles(chunks, binaries)
	if len(files) > 0 {
		b.WriteString("\nFiles changed (all):\n")
		for _, file := range files {
			b.WriteString("- " + file + "\n")
		}
	}

	b.WriteString("\nDiff:\n")
	preamble := b.String()

	suffix := "\n\nReturn only the plan, no code fences or extra commentary."
	if !split {
		suffix = "\n\nReturn only the commit message, no code fences or extra commentary."
	}

	diffBudget := maxChars - len(preamble) - len(suffix)
	if diffBudget < 0 {
		return trimToMax(preamble+suffix, maxChars)
	}

	diffBody, _ := buildDiffWithBudget(chunks, diffBudget)
	promptText := preamble + diffBody + suffix
	if len(promptText) > maxChars {
		return trimToMax(promptText, maxChars)
	}
	return promptText
}

func parseDiffChunks(diff string) []diffChunk {
	diff = strings.TrimSpace(diff)
	if diff == "" {
		return nil
	}
	raw := git.SplitDiffChunks(diff)
	chunks := make([]diffChunk, 0, len(raw))
	for i, chunk := range raw {
		path := git.ParseDiffPath(chunk)
		if path == "" {
			path = fmt.Sprintf("unknown-%d", i+1)
		}
		chunks = append(chunks, diffChunk{Path: path, Text: chunk})
	}
	return chunks
}

func collectFiles(chunks []diffChunk, binaries []git.BinaryFile) []string {
	seen := map[string]struct{}{}
	var out []string
	binarySet := map[string]struct{}{}
	for _, bf := range binaries {
		binarySet[bf.Path] = struct{}{}
	}
	for _, chunk := range chunks {
		if _, ok := seen[chunk.Path]; ok {
			continue
		}
		seen[chunk.Path] = struct{}{}
		if _, isBinary := binarySet[chunk.Path]; isBinary {
			out = append(out, chunk.Path+" (binary)")
			continue
		}
		out = append(out, chunk.Path)
	}
	for _, bf := range binaries {
		if _, ok := seen[bf.Path]; ok {
			continue
		}
		seen[bf.Path] = struct{}{}
		out = append(out, bf.Path+" (binary)")
	}
	return out
}

type chunkVariant struct {
	header    string
	hunks     string
	condensed string
	full      string
}

func buildDiffWithBudget(chunks []diffChunk, budget int) (string, bool) {
	if budget <= 0 || len(chunks) == 0 {
		return "", len(chunks) > 0
	}

	variants := make([]chunkVariant, 0, len(chunks))
	for _, chunk := range chunks {
		header := diffHeaderOnly(chunk.Text)
		hunks := diffHunkHeadersOnly(chunk.Text)
		condensed := condenseChunk(chunk.Text, 2000)
		variants = append(variants, chunkVariant{
			header:    header,
			hunks:     hunks,
			condensed: condensed,
			full:      chunk.Text,
		})
	}

	headerTotal := sumVariantSizes(variants, func(v chunkVariant) string { return v.header })
	if len(variants) > 1 {
		headerTotal += len(variants) - 1
	}
	if budget < headerTotal {
		return buildPartialHeaders(variants, budget)
	}

	current := make([]string, len(variants))
	for i, v := range variants {
		current[i] = v.header
	}

	remaining := budget - headerTotal
	upgrade := func(next func(v chunkVariant) string) {
		for i, v := range variants {
			target := next(v)
			extra := len(target) - len(current[i])
			if extra <= 0 {
				current[i] = target
				continue
			}
			if extra <= remaining {
				current[i] = target
				remaining -= extra
			}
		}
	}

	upgrade(func(v chunkVariant) string { return v.hunks })
	upgrade(func(v chunkVariant) string { return v.condensed })
	upgrade(func(v chunkVariant) string { return v.full })

	return strings.Join(current, "\n"), false
}

func sumVariantSizes(variants []chunkVariant, getter func(chunkVariant) string) int {
	total := 0
	for _, v := range variants {
		total += len(getter(v))
	}
	return total
}

func buildPartialHeaders(variants []chunkVariant, budget int) (string, bool) {
	var out []string
	remaining := budget
	for _, v := range variants {
		sep := 0
		if len(out) > 0 {
			sep = 1
		}
		needed := len(v.header) + sep
		if needed > remaining {
			return strings.Join(out, "\n"), true
		}
		if sep > 0 {
			remaining -= sep
		}
		out = append(out, v.header)
		remaining -= len(v.header)
	}
	return strings.Join(out, "\n"), false
}

func diffHeaderOnly(chunk string) string {
	lines := strings.SplitN(chunk, "\n", 2)
	if len(lines) == 0 {
		return chunk
	}
	return lines[0]
}

func diffHunkHeadersOnly(chunk string) string {
	lines := strings.Split(chunk, "\n")
	var out []string
	for _, line := range lines {
		if isDiffMetaLine(line) {
			out = append(out, line)
		}
	}
	if len(out) == 0 {
		return diffHeaderOnly(chunk)
	}
	return strings.Join(out, "\n")
}

func isDiffMetaLine(line string) bool {
	switch {
	case strings.HasPrefix(line, "diff --git "):
		return true
	case strings.HasPrefix(line, "index "):
		return true
	case strings.HasPrefix(line, "--- "):
		return true
	case strings.HasPrefix(line, "+++ "):
		return true
	case strings.HasPrefix(line, "@@ "):
		return true
	case strings.HasPrefix(line, "new file mode "):
		return true
	case strings.HasPrefix(line, "deleted file mode "):
		return true
	case strings.HasPrefix(line, "old mode "):
		return true
	case strings.HasPrefix(line, "new mode "):
		return true
	case strings.HasPrefix(line, "similarity index "):
		return true
	case strings.HasPrefix(line, "dissimilarity index "):
		return true
	case strings.HasPrefix(line, "rename from "):
		return true
	case strings.HasPrefix(line, "rename to "):
		return true
	default:
		return false
	}
}

func condenseChunk(chunk string, maxLen int) string {
	if maxLen <= 0 || len(chunk) <= maxLen {
		return chunk
	}
	marker := "\n[gommit] diff truncated to fit max_prompt_chars\n"
	if maxLen <= len(marker)+2 {
		return chunk[:maxLen]
	}
	keep := maxLen - len(marker)
	head := keep / 2
	tail := keep - head
	return chunk[:head] + marker + chunk[len(chunk)-tail:]
}

func trimToMax(text string, maxChars int) string {
	if maxChars <= 0 || len(text) <= maxChars {
		return text
	}
	marker := "\n[gommit] prompt truncated to fit max_prompt_chars\n"
	if maxChars <= len(marker)+2 {
		return text[:maxChars]
	}
	keep := maxChars - len(marker)
	head := keep / 2
	tail := keep - head
	return text[:head] + marker + text[len(text)-tail:]
}
