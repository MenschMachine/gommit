package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestProcessDiffTruncatesLargeFile(t *testing.T) {
	chunk := strings.Repeat("a", 50)
	diff := "diff --git a/foo.txt b/foo.txt\n" + chunk

	out, total, truncated := processDiff(diff, 20)
	if total != len(diff) {
		t.Fatalf("expected total %d, got %d", len(diff), total)
	}
	if !strings.Contains(out, "diff truncated") {
		t.Fatalf("expected truncation marker in output")
	}
	if len(truncated) != 1 || truncated[0] != "foo.txt" {
		t.Fatalf("expected truncated file foo.txt, got %v", truncated)
	}
}

func TestIsBinaryFile(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "bin.dat")
	if err := os.WriteFile(binPath, []byte{0x00, 0x01, 0x02}, 0o600); err != nil {
		t.Fatalf("write binary: %v", err)
	}
	if !isBinaryFile(binPath) {
		t.Fatalf("expected binary file detection")
	}

	textPath := filepath.Join(dir, "text.txt")
	if err := os.WriteFile(textPath, []byte("hello world"), 0o600); err != nil {
		t.Fatalf("write text: %v", err)
	}
	if isBinaryFile(textPath) {
		t.Fatalf("expected text file to not be binary")
	}
}

func TestCollectDiffFromBaseTracksModifications(t *testing.T) {
	repo := initTestRepo(t)
	writeTestFile(t, repo, "file.txt", "before\n")
	gitTest(t, repo, "add", ".")
	gitTest(t, repo, "commit", "-m", "base")
	base := strings.TrimSpace(gitTest(t, repo, "rev-parse", "HEAD"))

	writeTestFile(t, repo, "file.txt", "after\n")

	result, err := CollectDiffFromBase(repo, base, ScopeStagedUnstaged, 0)
	if err != nil {
		t.Fatalf("CollectDiffFromBase: %v", err)
	}
	if !strings.Contains(result.Diff, "+after") {
		t.Fatalf("expected tracked modification in diff, got:\n%s", result.Diff)
	}
}

func TestCollectDiffFromBaseIncludesUntrackedWithAll(t *testing.T) {
	repo := initTestRepo(t)
	writeTestFile(t, repo, "tracked.txt", "tracked\n")
	gitTest(t, repo, "add", ".")
	gitTest(t, repo, "commit", "-m", "base")
	base := strings.TrimSpace(gitTest(t, repo, "rev-parse", "HEAD"))

	writeTestFile(t, repo, "new.txt", "new\n")

	result, err := CollectDiffFromBase(repo, base, ScopeAll, 0)
	if err != nil {
		t.Fatalf("CollectDiffFromBase: %v", err)
	}
	if !strings.Contains(result.Diff, "diff --git a/new.txt b/new.txt") || !strings.Contains(result.Diff, "+new") {
		t.Fatalf("expected untracked file in -A diff, got:\n%s", result.Diff)
	}
}

func TestCollectDiffFromBaseIncludesDeletedFiles(t *testing.T) {
	repo := initTestRepo(t)
	writeTestFile(t, repo, "deleted.txt", "delete me\n")
	gitTest(t, repo, "add", ".")
	gitTest(t, repo, "commit", "-m", "base")
	base := strings.TrimSpace(gitTest(t, repo, "rev-parse", "HEAD"))

	if err := os.Remove(filepath.Join(repo, "deleted.txt")); err != nil {
		t.Fatalf("remove file: %v", err)
	}

	result, err := CollectDiffFromBase(repo, base, ScopeAll, 0)
	if err != nil {
		t.Fatalf("CollectDiffFromBase: %v", err)
	}
	if !strings.Contains(result.Diff, "deleted file mode") || !strings.Contains(result.Diff, "-delete me") {
		t.Fatalf("expected deleted file in diff, got:\n%s", result.Diff)
	}
}

func TestCollectDiffFromBaseReturnsEmptyWhenNoChangesSinceBase(t *testing.T) {
	repo := initTestRepo(t)
	writeTestFile(t, repo, "file.txt", "same\n")
	gitTest(t, repo, "add", ".")
	gitTest(t, repo, "commit", "-m", "base")
	base := strings.TrimSpace(gitTest(t, repo, "rev-parse", "HEAD"))

	result, err := CollectDiffFromBase(repo, base, ScopeAll, 0)
	if err != nil {
		t.Fatalf("CollectDiffFromBase: %v", err)
	}
	if strings.TrimSpace(result.Diff) != "" || len(result.Binary) != 0 {
		t.Fatalf("expected no changes since base, got diff=%q binary=%v", result.Diff, result.Binary)
	}
}

func TestCollectDiffFromBaseDoesNotMutateRealIndex(t *testing.T) {
	repo := initTestRepo(t)
	writeTestFile(t, repo, "file.txt", "base\n")
	gitTest(t, repo, "add", ".")
	gitTest(t, repo, "commit", "-m", "base")
	base := strings.TrimSpace(gitTest(t, repo, "rev-parse", "HEAD"))

	writeTestFile(t, repo, "file.txt", "staged\n")
	gitTest(t, repo, "add", "file.txt")
	writeTestFile(t, repo, "file.txt", "unstaged\n")
	writeTestFile(t, repo, "untracked.txt", "new\n")
	before := gitTest(t, repo, "ls-files", "--stage")

	if _, err := CollectDiffFromBase(repo, base, ScopeAll, 0); err != nil {
		t.Fatalf("CollectDiffFromBase: %v", err)
	}

	after := gitTest(t, repo, "ls-files", "--stage")
	if before != after {
		t.Fatalf("real index changed\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func initTestRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	gitTest(t, repo, "init")
	gitTest(t, repo, "config", "user.email", "test@example.com")
	gitTest(t, repo, "config", "user.name", "Test User")
	return repo
}

func writeTestFile(t *testing.T, repo, path, contents string) {
	t.Helper()
	fullPath := filepath.Join(repo, path)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(fullPath, []byte(contents), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

func gitTest(t *testing.T, repo string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}
