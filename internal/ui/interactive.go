package ui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

func EditInEditor(initial string) (string, error) {
	editor := strings.TrimSpace(os.Getenv("EDITOR"))
	if editor == "" {
		return InlineEdit(initial)
	}

	tmpDir := os.TempDir()
	path := filepath.Join(tmpDir, "gommit_message.txt")
	if err := os.WriteFile(path, []byte(initial+"\n"), 0o600); err != nil {
		return "", err
	}

	cmd := exec.Command("sh", "-c", editor+" "+path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func InlineEdit(initial string) (string, error) {
	fmt.Println("Enter commit message. End with EOF (Ctrl-D).")
	fmt.Println("---")
	if initial != "" {
		fmt.Println(initial)
		fmt.Println("---")
	}
	reader := bufio.NewReader(os.Stdin)
	var lines []string
	for {
		line, err := reader.ReadString('\n')
		if err == io.EOF {
			if strings.TrimSpace(line) != "" {
				lines = append(lines, strings.TrimRight(line, "\n"))
			}
			break
		}
		if err != nil {
			return "", err
		}
		lines = append(lines, strings.TrimRight(line, "\n"))
	}
	return strings.TrimSpace(strings.Join(lines, "\n")), nil
}

func DisplayFileBox(writer io.Writer, files []string, maxDisplay int) {
	boxWidth := getBoxWidth()

	// Build content lines
	var lines []string

	// Header
	lines = append(lines, fmt.Sprintf("Files to be committed (%d):", len(files)))
	lines = append(lines, "") // Separator line

	// File list
	displayCount := len(files)
	if displayCount > maxDisplay {
		displayCount = maxDisplay
	}

	for i := 0; i < displayCount; i++ {
		lines = append(lines, " • "+truncatePath(files[i], boxWidth-8))
	}

	// Show "and N more files" if there are more files
	if len(files) > maxDisplay {
		remaining := len(files) - maxDisplay
		lines = append(lines, fmt.Sprintf(" • ... and %d more files", remaining))
	}

	// Create lipgloss box style
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Width(boxWidth).
		Padding(0, 1)

	// Render the box
	fmt.Fprintln(writer, boxStyle.Render(strings.Join(lines, "\n")))
}

// getBoxWidth calculates box width: min(max(50, termWidth*0.8), 100)
func getBoxWidth() int {
	// Default for non-TTY (pipes, CI)
	defaultWidth := 80

	fd := int(os.Stdout.Fd())
	width, _, err := term.GetSize(fd)
	if err != nil {
		return defaultWidth
	}

	// Calculate 80% of terminal width
	boxWidth := int(float64(width) * 0.8)

	// Apply min/max constraints
	if boxWidth < 50 {
		boxWidth = 50
	}
	if boxWidth > 100 {
		boxWidth = 100
	}

	return boxWidth
}

// truncatePath truncates a path with middle ellipsis if needed
func truncatePath(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}

	// Use middle ellipsis for long paths
	ellipsis := "..."
	if maxLen <= len(ellipsis) {
		return ellipsis
	}

	// Calculate how many chars to show on each side
	sideLen := (maxLen - len(ellipsis)) / 2
	leftLen := sideLen
	rightLen := maxLen - len(ellipsis) - leftLen

	return path[:leftLen] + ellipsis + path[len(path)-rightLen:]
}
