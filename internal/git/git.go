package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

func RepoRoot() (string, error) {
	out, err := runGit("", "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("not a git repository: %w", err)
	}
	return strings.TrimSpace(out), nil
}

func runGit(dir string, args ...string) (string, error) {
	return runGitAllowExitCodes(dir, []int{0}, args...)
}

func runGitAllowExitCodes(dir string, allowed []int, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return stdout.String(), nil
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		for _, code := range allowed {
			if exitErr.ExitCode() == code {
				return stdout.String(), nil
			}
		}
	}

	errMsg := strings.TrimSpace(stderr.String())
	if errMsg == "" {
		errMsg = err.Error()
	}
	return "", fmt.Errorf("git %s failed: %s", strings.Join(args, " "), errMsg)
}
