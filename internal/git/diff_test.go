package git

import (
	"os"
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
