package main

import (
	"reflect"
	"testing"

	"github.com/MenschMachine/gommit/internal/git"
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
