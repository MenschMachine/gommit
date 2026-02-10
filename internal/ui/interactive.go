package ui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func PromptChoice(reader *bufio.Reader, writer io.Writer, prompt string, options map[rune]string) (rune, error) {
	for {
		fmt.Fprintln(writer, prompt)
		for key, label := range options {
			fmt.Fprintf(writer, "  %c) %s\n", key, label)
		}
		fmt.Fprint(writer, "> ")
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return 0, err
		}
		line = strings.TrimSpace(line)
		if line == "" && err == io.EOF {
			return 0, io.EOF
		}
		if line == "" {
			continue
		}
		r := []rune(strings.ToLower(line))[0]
		if _, ok := options[r]; ok {
			return r, nil
		}
	}
}

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
