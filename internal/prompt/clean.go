package prompt

import (
	"regexp"
	"strings"
)

var codeFenceRe = regexp.MustCompile("(?s)`{3}\\w*\\s*\\n(.*?)\\n\\s*`{3}")

var preambleLineRe = regexp.MustCompile(`(?i)^(.*\b(?:commit|message)\b[^:]*)\s*:\s*`)

// CleanResponse strips common LLM preamble, postamble, and code fences
// from a commit message response.
func CleanResponse(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return s
	}

	// Strip preamble: lines containing "commit" or "message" ending with colon
	for {
		lines := strings.SplitN(s, "\n", 2)
		if m := preambleLineRe.FindStringIndex(lines[0]); m != nil && m[0] == 0 {
			if len(lines) == 1 {
				// Same line: preamble + message on one line
				rest := strings.TrimSpace(lines[0][m[1]:])
				if rest != "" {
					s = rest
					break
				}
				return ""
			}
			s = strings.TrimSpace(lines[1])
			continue
		}
		break
	}

	// Strip code fences wrapping response
	if m := codeFenceRe.FindStringSubmatch(s); m != nil {
		s = strings.TrimSpace(m[1])
	}

	// Strip inline backtick wrapping: `message`
	if len(s) >= 2 && s[0] == '`' && s[len(s)-1] == '`' {
		s = strings.TrimSpace(s[1 : len(s)-1])
	}

	// Strip postamble: everything after first blank line
	if idx := strings.Index(s, "\n\n"); idx != -1 {
		s = strings.TrimSpace(s[:idx])
	}

	return s
}
