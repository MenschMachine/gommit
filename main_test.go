package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mlahr/gommit/internal/git"
)

func TestAppendTag(t *testing.T) {
	tests := []struct {
		message string
		tag     string
		want    string
	}{
		{"feat: add login", "", "feat: add login"},
		{"feat: add login", "skip ci", "feat: add login [skip ci]"},
		{"feat: add login\n", "skip ci", "feat: add login [skip ci]"},
		{"feat: add login\n\nbody text", "skip ci", "feat: add login [skip ci]\n\nbody text"},
		{"feat: add login\n\nline1\nline2", "WIP", "feat: add login [WIP]\n\nline1\nline2"},
	}
	for _, tt := range tests {
		got := appendTag(tt.message, tt.tag)
		if got != tt.want {
			t.Errorf("appendTag(%q, %q) = %q, want %q", tt.message, tt.tag, got, tt.want)
		}
	}
}

func TestExpandNoteCommandQuotesPlaceholders(t *testing.T) {
	got := expandNoteCommand("diffcog --delta-totals {base} {commit}", "main's base", "abc123")
	want := "diffcog --delta-totals 'main'\"'\"'s base' 'abc123'"
	if got != want {
		t.Fatalf("expandNoteCommand() = %q, want %q", got, want)
	}
}

func TestNoteCommandUsesStdin(t *testing.T) {
	tests := []struct {
		command string
		want    bool
	}{
		{"cat", true},
		{"diffcog --delta-totals {base} {commit}", false},
		{"printf %s {commit}", false},
	}
	for _, tt := range tests {
		if got := noteCommandUsesStdin(tt.command); got != tt.want {
			t.Fatalf("noteCommandUsesStdin(%q) = %t, want %t", tt.command, got, tt.want)
		}
	}
}

func TestRunNoteCommandWithPlaceholders(t *testing.T) {
	repo := initMainTestRepo(t)
	got, err := runNoteCommand(repo, "abc123", NoteOptions{
		Command: "printf '%s %s' {base} {commit}",
		Base:    "base-ref",
		Ref:     "refs/notes/commits",
	})
	if err != nil {
		t.Fatalf("runNoteCommand: %v", err)
	}
	if string(got) != "base-ref abc123" {
		t.Fatalf("note command output = %q, want %q", got, "base-ref abc123")
	}
}

func TestRunNoteCommandUsesSelectedDiffOnStdin(t *testing.T) {
	repo := initMainTestRepo(t)
	writeMainTestFile(t, repo, "file.txt", "before\n")
	gitMainTest(t, repo, "add", ".")
	gitMainTest(t, repo, "commit", "-m", "base")
	base := strings.TrimSpace(gitMainTest(t, repo, "rev-parse", "HEAD"))

	writeMainTestFile(t, repo, "file.txt", "after\n")
	gitMainTest(t, repo, "add", ".")
	gitMainTest(t, repo, "commit", "-m", "change")
	commit := strings.TrimSpace(gitMainTest(t, repo, "rev-parse", "HEAD"))

	got, err := runNoteCommand(repo, commit, NoteOptions{
		Command: "cat",
		Base:    base,
		Ref:     "refs/notes/commits",
	})
	if err != nil {
		t.Fatalf("runNoteCommand: %v", err)
	}
	if !strings.Contains(string(got), "+after") {
		t.Fatalf("expected stdin diff to include committed change, got:\n%s", got)
	}
}

func TestAddGitNoteOverwritesExistingNote(t *testing.T) {
	repo := initMainTestRepo(t)
	writeMainTestFile(t, repo, "file.txt", "contents\n")
	gitMainTest(t, repo, "add", ".")
	gitMainTest(t, repo, "commit", "-m", "base")
	commit := strings.TrimSpace(gitMainTest(t, repo, "rev-parse", "HEAD"))

	if err := addGitNote(repo, commit, "refs/notes/commits", []byte("first\n")); err != nil {
		t.Fatalf("add first note: %v", err)
	}
	if err := addGitNote(repo, commit, "refs/notes/commits", []byte("second\n")); err != nil {
		t.Fatalf("overwrite note: %v", err)
	}
	got := gitMainTest(t, repo, "notes", "--ref", "refs/notes/commits", "show", commit)
	if got != "second\n" {
		t.Fatalf("note = %q, want %q", got, "second\n")
	}
}

func TestAddGitNoteAllowsEmptyNote(t *testing.T) {
	repo := initMainTestRepo(t)
	writeMainTestFile(t, repo, "file.txt", "contents\n")
	gitMainTest(t, repo, "add", ".")
	gitMainTest(t, repo, "commit", "-m", "base")
	commit := strings.TrimSpace(gitMainTest(t, repo, "rev-parse", "HEAD"))

	if err := addGitNote(repo, commit, "refs/notes/commits", nil); err != nil {
		t.Fatalf("add empty note: %v", err)
	}
	list := gitMainTest(t, repo, "notes", "--ref", "refs/notes/commits", "list", commit)
	if strings.TrimSpace(list) == "" {
		t.Fatalf("expected empty note object to exist")
	}
	got := gitMainTest(t, repo, "notes", "--ref", "refs/notes/commits", "show", commit)
	if got != "" {
		t.Fatalf("empty note content = %q, want empty string", got)
	}
}

func TestAttachNoteReportsPostCommitFailure(t *testing.T) {
	repo := initMainTestRepo(t)
	err := attachNote(repo, "abc123", NoteOptions{
		Command: "printf failure >&2; printf %s {commit} >/dev/null; exit 7",
		Ref:     "refs/notes/commits",
	})
	if err == nil {
		t.Fatalf("expected attachNote failure")
	}
	if !strings.Contains(err.Error(), "commit abc123 created") || !strings.Contains(err.Error(), "failure") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func initMainTestRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	gitMainTest(t, repo, "init")
	gitMainTest(t, repo, "config", "user.email", "test@example.com")
	gitMainTest(t, repo, "config", "user.name", "Test User")
	return repo
}

func writeMainTestFile(t *testing.T, repo, path, contents string) {
	t.Helper()
	fullPath := filepath.Join(repo, path)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(fullPath, []byte(contents), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

func gitMainTest(t *testing.T, repo string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

func TestBuildCommitArgs(t *testing.T) {
	tests := []struct {
		name        string
		scope       git.DiffScope
		messageFile string
		noVerify    bool
		want        []string
	}{
		{
			name:        "staged without no verify",
			scope:       git.ScopeStaged,
			messageFile: "/tmp/msg.txt",
			want:        []string{"commit", "-F", "/tmp/msg.txt"},
		},
		{
			name:        "staged with no verify",
			scope:       git.ScopeStaged,
			messageFile: "/tmp/msg.txt",
			noVerify:    true,
			want:        []string{"commit", "--no-verify", "-F", "/tmp/msg.txt"},
		},
		{
			name:        "staged unstaged with no verify",
			scope:       git.ScopeStagedUnstaged,
			messageFile: "/tmp/msg.txt",
			noVerify:    true,
			want:        []string{"commit", "-a", "--no-verify", "-F", "/tmp/msg.txt"},
		},
		{
			name:        "all scope with no verify",
			scope:       git.ScopeAll,
			messageFile: "/tmp/msg.txt",
			noVerify:    true,
			want:        []string{"commit", "-a", "--no-verify", "-F", "/tmp/msg.txt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildCommitArgs(tt.scope, tt.messageFile, tt.noVerify)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("buildCommitArgs(%q, %q, %t) = %v, want %v", tt.scope, tt.messageFile, tt.noVerify, got, tt.want)
			}
		})
	}
}
