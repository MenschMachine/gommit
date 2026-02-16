package ui

import (
	"bytes"
	"strings"
	"testing"
)

func TestDisplayFileBox_SingleFile(t *testing.T) {
	var buf bytes.Buffer
	files := []string{"main.go"}
	DisplayFileBox(&buf, files, 5)
	output := buf.String()

	if !strings.Contains(output, "Files to be committed (1):") {
		t.Error("expected header to show 1 file")
	}
	if !strings.Contains(output, "• main.go") {
		t.Error("expected file to be listed")
	}
	if strings.Contains(output, "and") && strings.Contains(output, "more") {
		t.Error("should not show 'more files' message for single file")
	}
}

func TestDisplayFileBox_FiveFiles(t *testing.T) {
	var buf bytes.Buffer
	files := []string{
		"file1.go",
		"file2.go",
		"file3.go",
		"file4.go",
		"file5.go",
	}
	DisplayFileBox(&buf, files, 5)
	output := buf.String()

	if !strings.Contains(output, "Files to be committed (5):") {
		t.Error("expected header to show 5 files")
	}
	for _, file := range files {
		if !strings.Contains(output, file) {
			t.Errorf("expected file %s to be listed", file)
		}
	}
	if strings.Contains(output, "and") && strings.Contains(output, "more") {
		t.Error("should not show 'more files' message when exactly at limit")
	}
}

func TestDisplayFileBox_MoreThanFiveFiles(t *testing.T) {
	var buf bytes.Buffer
	files := []string{
		"file1.go",
		"file2.go",
		"file3.go",
		"file4.go",
		"file5.go",
		"file6.go",
		"file7.go",
		"file8.go",
	}
	DisplayFileBox(&buf, files, 5)
	output := buf.String()

	if !strings.Contains(output, "Files to be committed (8):") {
		t.Error("expected header to show 8 files")
	}
	// First 5 files should be shown
	for i := 0; i < 5; i++ {
		if !strings.Contains(output, files[i]) {
			t.Errorf("expected file %s to be listed", files[i])
		}
	}
	// Last 3 files should not be shown
	for i := 5; i < 8; i++ {
		if strings.Contains(output, files[i]) {
			t.Errorf("file %s should not be listed individually", files[i])
		}
	}
	if !strings.Contains(output, "and 3 more files") {
		t.Error("expected 'and 3 more files' message")
	}
}

func TestDisplayFileBox_LongFilePath(t *testing.T) {
	var buf bytes.Buffer
	longPath := "very/long/path/to/some/deeply/nested/directory/structure/file.go"
	files := []string{longPath}
	DisplayFileBox(&buf, files, 5)
	output := buf.String()

	// Check that the output doesn't break the box structure
	lines := strings.Split(output, "\n")
	for i, line := range lines {
		if i == 0 || i == len(lines)-1 || i == len(lines)-2 {
			continue // Skip first line, last line, and empty last line
		}
		// Each content line should be exactly 64 characters (60 + borders + newline handling)
		// Actually, let's just check that lines start with │ and end with │
		if len(line) > 0 && !strings.HasPrefix(line, "│") && !strings.HasPrefix(line, "╭") && !strings.HasPrefix(line, "├") && !strings.HasPrefix(line, "╰") {
			t.Errorf("line %d doesn't start with expected box character: %q", i, line)
		}
		if len(line) > 0 && !strings.HasSuffix(strings.TrimSpace(line), "│") && !strings.HasSuffix(strings.TrimSpace(line), "╮") && !strings.HasSuffix(strings.TrimSpace(line), "┤") && !strings.HasSuffix(strings.TrimSpace(line), "╯") {
			t.Errorf("line %d doesn't end with expected box character: %q", i, line)
		}
	}
}

func TestDisplayFileBox_EmptyList(t *testing.T) {
	var buf bytes.Buffer
	files := []string{}
	DisplayFileBox(&buf, files, 5)
	output := buf.String()

	if !strings.Contains(output, "Files to be committed (0):") {
		t.Error("expected header to show 0 files")
	}
	// Should still show the box structure
	if !strings.Contains(output, "╭") || !strings.Contains(output, "╰") {
		t.Error("expected box borders to be present")
	}
}

func TestDisplayFileBox_BoxStructure(t *testing.T) {
	var buf bytes.Buffer
	files := []string{"test.go"}
	DisplayFileBox(&buf, files, 5)
	output := buf.String()

	// Check for rounded box drawing characters (lipgloss RoundedBorder)
	if !strings.Contains(output, "╭") {
		t.Error("missing top-left corner")
	}
	if !strings.Contains(output, "╮") {
		t.Error("missing top-right corner")
	}
	if !strings.Contains(output, "╰") {
		t.Error("missing bottom-left corner")
	}
	if !strings.Contains(output, "╯") {
		t.Error("missing bottom-right corner")
	}
	// Lipgloss doesn't use separators, just borders
	if !strings.Contains(output, "│") {
		t.Error("missing vertical borders")
	}
}

func TestGetBoxWidth(t *testing.T) {
	width := getBoxWidth()
	// Should be within valid range
	if width < 50 || width > 100 {
		t.Errorf("expected width between 50-100, got %d", width)
	}
}

func TestTruncatePath(t *testing.T) {
	tests := []struct {
		name          string
		path          string
		maxLen        int
		wantLen       int
		shouldTruncate bool
	}{
		{
			name:          "short path no truncation",
			path:          "main.go",
			maxLen:        20,
			wantLen:       7,
			shouldTruncate: false,
		},
		{
			name:          "long path truncated",
			path:          "very/long/path/to/some/deeply/nested/directory/structure/file.go",
			maxLen:        30,
			wantLen:       30,
			shouldTruncate: true,
		},
		{
			name:          "exact length no truncation",
			path:          "exact.go",
			maxLen:        8,
			wantLen:       8,
			shouldTruncate: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncatePath(tt.path, tt.maxLen)
			if len(result) != tt.wantLen {
				t.Errorf("expected length %d, got %d: %q", tt.wantLen, len(result), result)
			}
			if len(result) > tt.maxLen {
				t.Errorf("truncated path exceeds maxLen: %d > %d", len(result), tt.maxLen)
			}
			if tt.shouldTruncate && !strings.Contains(result, "...") {
				t.Error("expected ellipsis in truncated path")
			}
			if !tt.shouldTruncate && strings.Contains(result, "...") {
				t.Error("unexpected ellipsis in path that doesn't need truncation")
			}
		})
	}
}
