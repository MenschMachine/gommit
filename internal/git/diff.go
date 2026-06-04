package git

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

type DiffScope int

const (
	ScopeStaged DiffScope = iota
	ScopeStagedUnstaged
	ScopeAll
)

type BinaryFile struct {
	Path string
	Size int64
}

type DiffResult struct {
	Diff             string
	Binary           []BinaryFile
	TruncatedFiles   []string
	TotalOriginalLen int
}

func CollectDiff(root string, scope DiffScope, perFileLimit int) (DiffResult, error) {
	var combined []string
	binaryFiles := map[string]BinaryFile{}
	var truncated []string
	totalOriginal := 0

	if scope >= ScopeStaged {
		bins, err := collectBinaryFiles(root, true)
		if err != nil {
			return DiffResult{}, err
		}
		for _, bf := range bins {
			binaryFiles[bf.Path] = bf
		}
		out, err := runGitAllowExitCodes(root, []int{0, 1}, "diff", "--cached")
		if err != nil {
			return DiffResult{}, err
		}
		diffText, origLen, trunc := processDiff(out, perFileLimit)
		totalOriginal += origLen
		truncated = append(truncated, trunc...)
		if diffText != "" {
			combined = append(combined, diffText)
		}
	}

	if scope >= ScopeStagedUnstaged {
		bins, err := collectBinaryFiles(root, false)
		if err != nil {
			return DiffResult{}, err
		}
		for _, bf := range bins {
			binaryFiles[bf.Path] = bf
		}
		out, err := runGitAllowExitCodes(root, []int{0, 1}, "diff")
		if err != nil {
			return DiffResult{}, err
		}
		diffText, origLen, trunc := processDiff(out, perFileLimit)
		totalOriginal += origLen
		truncated = append(truncated, trunc...)
		if diffText != "" {
			combined = append(combined, diffText)
		}
	}

	if scope == ScopeAll {
		files, err := listUntracked(root)
		if err != nil {
			return DiffResult{}, err
		}
		for _, file := range files {
			abs := filepath.Join(root, file)
			if isBinaryFile(abs) {
				bf := BinaryFile{Path: file, Size: fileSize(abs)}
				binaryFiles[bf.Path] = bf
				continue
			}
			out, err := runGitAllowExitCodes(root, []int{0, 1}, "diff", "--no-index", "/dev/null", file)
			if err != nil {
				return DiffResult{}, err
			}
			diffText, origLen, trunc := processDiff(out, perFileLimit)
			totalOriginal += origLen
			truncated = append(truncated, trunc...)
			if diffText != "" {
				combined = append(combined, diffText)
			}
		}
	}

	binaryList := make([]BinaryFile, 0, len(binaryFiles))
	for _, bf := range binaryFiles {
		binaryList = append(binaryList, bf)
	}

	return DiffResult{
		Diff:             strings.Join(combined, "\n"),
		Binary:           binaryList,
		TruncatedFiles:   uniqueStrings(truncated),
		TotalOriginalLen: totalOriginal,
	}, nil
}

func CollectDiffFromBase(root, base string, scope DiffScope, perFileLimit int) (DiffResult, error) {
	baseTree, err := resolveBaseTree(root, base)
	if err != nil {
		return DiffResult{}, err
	}

	var out string
	var binary []BinaryFile
	switch scope {
	case ScopeStaged:
		out, err = runGitAllowExitCodes(root, []int{0, 1}, "diff", "--cached", baseTree)
		if err != nil {
			return DiffResult{}, err
		}
		binary, err = collectBinaryFilesForDiff(root, "diff", "--cached", "--numstat", baseTree)
		if err != nil {
			return DiffResult{}, err
		}
	case ScopeStagedUnstaged:
		tree, cleanup, err := writeTemporaryWorktreeTree(root, false)
		if cleanup != nil {
			defer cleanup()
		}
		if err != nil {
			return DiffResult{}, err
		}
		out, err = runGitAllowExitCodes(root, []int{0, 1}, "diff", baseTree, tree)
		if err != nil {
			return DiffResult{}, err
		}
		binary, err = collectBinaryFilesForDiff(root, "diff", "--numstat", baseTree, tree)
		if err != nil {
			return DiffResult{}, err
		}
	case ScopeAll:
		tree, cleanup, err := writeTemporaryWorktreeTree(root, true)
		if cleanup != nil {
			defer cleanup()
		}
		if err != nil {
			return DiffResult{}, err
		}
		out, err = runGitAllowExitCodes(root, []int{0, 1}, "diff", baseTree, tree)
		if err != nil {
			return DiffResult{}, err
		}
		binary, err = collectBinaryFilesForDiff(root, "diff", "--numstat", baseTree, tree)
		if err != nil {
			return DiffResult{}, err
		}
	default:
		return DiffResult{}, fmt.Errorf("unknown diff scope: %d", scope)
	}

	diffText, origLen, truncated := processDiff(out, perFileLimit)
	return DiffResult{
		Diff:             diffText,
		Binary:           binary,
		TruncatedFiles:   uniqueStrings(truncated),
		TotalOriginalLen: origLen,
	}, nil
}

func resolveBaseTree(root, base string) (string, error) {
	base = strings.TrimSpace(base)
	if base == "" {
		return "", fmt.Errorf("diff base is required")
	}
	out, err := runGit(root, "rev-parse", "--verify", base+"^{tree}")
	if err != nil {
		return "", fmt.Errorf("invalid diff base %q: %w", base, err)
	}
	return strings.TrimSpace(out), nil
}

func writeTemporaryWorktreeTree(root string, includeUntracked bool) (string, func(), error) {
	file, err := os.CreateTemp("", "gommit-index-*")
	if err != nil {
		return "", nil, err
	}
	indexPath := file.Name()
	if err := file.Close(); err != nil {
		_ = os.Remove(indexPath)
		return "", nil, err
	}
	cleanup := func() {
		_ = os.Remove(indexPath)
	}

	if err := seedTemporaryIndex(root, indexPath); err != nil {
		cleanup()
		return "", nil, err
	}

	env := []string{"GIT_INDEX_FILE=" + indexPath}
	addArgs := []string{"add", "-u"}
	if includeUntracked {
		addArgs = []string{"add", "-A"}
	}
	if _, err := runGitWithEnv(root, env, addArgs...); err != nil {
		cleanup()
		return "", nil, err
	}
	tree, err := runGitWithEnv(root, env, "write-tree")
	if err != nil {
		cleanup()
		return "", nil, err
	}
	return strings.TrimSpace(tree), cleanup, nil
}

func seedTemporaryIndex(root, indexPath string) error {
	indexOut, err := runGit(root, "rev-parse", "--git-path", "index")
	if err == nil {
		realIndex := strings.TrimSpace(indexOut)
		if realIndex != "" && !filepath.IsAbs(realIndex) {
			realIndex = filepath.Join(root, realIndex)
		}
		if err := copyFile(realIndex, indexPath); err == nil {
			return nil
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	env := []string{"GIT_INDEX_FILE=" + indexPath}
	_ = os.Remove(indexPath)
	if _, err := runGitWithEnv(root, env, "read-tree", "HEAD"); err != nil {
		return err
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

func collectBinaryFiles(root string, cached bool) ([]BinaryFile, error) {
	args := []string{"diff", "--numstat"}
	if cached {
		args = append(args, "--cached")
	}
	return collectBinaryFilesForDiff(root, args...)
}

func collectBinaryFilesForDiff(root string, args ...string) ([]BinaryFile, error) {
	out, err := runGitAllowExitCodes(root, []int{0, 1}, args...)
	if err != nil {
		return nil, err
	}
	var binaries []BinaryFile
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, "\t")
		if len(parts) < 3 {
			continue
		}
		if parts[0] == "-" || parts[1] == "-" {
			path := parts[2]
			binaries = append(binaries, BinaryFile{Path: path, Size: fileSize(filepath.Join(root, path))})
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return binaries, nil
}

func listUntracked(root string) ([]string, error) {
	out, err := runGit(root, "ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	var files []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		files = append(files, line)
	}
	return files, nil
}

func processDiff(diff string, perFileLimit int) (string, int, []string) {
	diff = strings.TrimSpace(diff)
	if diff == "" {
		return "", 0, nil
	}
	chunks := splitDiffChunks(diff)
	var kept []string
	var truncated []string
	totalOriginal := 0
	for _, chunk := range chunks {
		if isBinaryChunk(chunk) {
			continue
		}
		origLen := len(chunk)
		totalOriginal += origLen
		if perFileLimit > 0 && origLen > perFileLimit {
			path := parseDiffPath(chunk)
			head := perFileLimit / 2
			tail := perFileLimit - head
			marker := fmt.Sprintf("\n[gommit] diff truncated for %s: showing first %d and last %d chars of %d total\n", path, head, tail, origLen)
			chunk = chunk[:head] + marker + chunk[origLen-tail:]
			if path != "" {
				truncated = append(truncated, path)
			}
		}
		kept = append(kept, chunk)
	}
	return strings.Join(kept, "\n"), totalOriginal, truncated
}

func SplitDiffChunks(diff string) []string {
	return splitDiffChunks(diff)
}

func splitDiffChunks(diff string) []string {
	if strings.HasPrefix(diff, "diff --git ") {
		parts := strings.Split(diff, "\ndiff --git ")
		chunks := make([]string, 0, len(parts))
		for i, part := range parts {
			if i == 0 {
				chunks = append(chunks, part)
				continue
			}
			chunks = append(chunks, "diff --git "+part)
		}
		return chunks
	}
	return []string{diff}
}

func ParseDiffPath(chunk string) string {
	return parseDiffPath(chunk)
}

func parseDiffPath(chunk string) string {
	lines := strings.SplitN(chunk, "\n", 2)
	if len(lines) == 0 {
		return ""
	}
	line := strings.TrimSpace(lines[0])
	if !strings.HasPrefix(line, "diff --git ") {
		return ""
	}
	fields := strings.Fields(line)
	if len(fields) < 4 {
		return ""
	}
	bPath := strings.TrimPrefix(fields[3], "b/")
	if bPath == "/dev/null" {
		bPath = strings.TrimPrefix(fields[2], "a/")
	}
	return bPath
}

func isBinaryChunk(chunk string) bool {
	return strings.Contains(chunk, "GIT binary patch") || strings.Contains(chunk, "Binary files ")
}

func fileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return -1
	}
	return info.Size()
}

func isBinaryFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	buf := make([]byte, 8000)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return false
	}
	buf = buf[:n]
	if bytesContainZero(buf) {
		return true
	}
	return !utf8.Valid(buf)
}

func bytesContainZero(buf []byte) bool {
	for _, b := range buf {
		if b == 0 {
			return true
		}
	}
	return false
}

func uniqueStrings(items []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, item := range items {
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}
