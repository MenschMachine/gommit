package main

import "testing"

func TestAppendTag(t *testing.T) {
	tests := []struct {
		message string
		tag     string
		want    string
	}{
		{"feat: add login", "", "feat: add login"},
		{"feat: add login", "skip ci", "feat: add login [skip ci]"},
		{"feat: add login\n", "skip ci", "feat: add login [skip ci]"},
		{"feat: add login\n\nbody text", "WIP", "feat: add login\n\nbody text [WIP]"},
	}
	for _, tt := range tests {
		got := appendTag(tt.message, tt.tag)
		if got != tt.want {
			t.Errorf("appendTag(%q, %q) = %q, want %q", tt.message, tt.tag, got, tt.want)
		}
	}
}
