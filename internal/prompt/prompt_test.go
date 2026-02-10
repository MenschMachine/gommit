package prompt

import (
	"strings"
	"testing"

	"gommit/internal/git"
)

func TestBuildSinglePromptIncludesMetadata(t *testing.T) {
	binaries := []git.BinaryFile{{Path: "image.png", Size: 1234}}
	promptText := BuildSinglePrompt("conventional", "staged only", "diff --git a/a b/a", binaries, []string{"big.txt"})

	if !strings.Contains(promptText, "Conventional Commits") {
		t.Fatalf("expected conventional commit instructions")
	}
	if !strings.Contains(promptText, "big.txt") {
		t.Fatalf("expected truncated file mention")
	}
	if !strings.Contains(promptText, "image.png") {
		t.Fatalf("expected binary file mention")
	}
}

func TestBuildSplitPromptIncludesPlanInstructions(t *testing.T) {
	promptText := BuildSplitPrompt("freeform", "staged only", "diff --git a/a b/a", nil, nil)
	if !strings.Contains(promptText, "multi-commit plan") {
		t.Fatalf("expected split-mode instructions")
	}
	if !strings.Contains(promptText, "Total number of commits") {
		t.Fatalf("expected plan outline")
	}
}
