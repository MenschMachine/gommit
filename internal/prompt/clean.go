package prompt

import (
	"regexp"
	"strings"
)

var codeFenceRe = regexp.MustCompile("(?s)`{3}\\w*\\s*\\n(.*?)\\n\\s*`{3}")

var preambleLineRe = regexp.MustCompile(`(?i)^(.*\b(?:commit|message)\b[^:]*)\s*:\s*`)

var postambleFillerRe = regexp.MustCompile(`(?i)^\s*(?:` +
	`i\s+(?:chose|used|decided|selected)\s|` +
	`the\s+reason\s+for\s|` +
	`this\s+is\s+because\s|` +
	`the\s+purpose\s+of\s|` +
	`this\s+commit\s|` +
	`this\s+change\s|` +
	`the\s+changes\s|` +
	`note\s*:|note\s+that\s|please\s+note\s|` +
	`summary\s*:|in\s+summary,?\s|` +
	`key\s+(?:changes|modifications)\s*:|` +
	`would\s+you\s+like\s|` +
	`let\s+me\s+know\b|` +
	`feel\s+free\s+to\s` +
	`)`)

// CleanResponse strips common LLM preamble, postamble filler, and code fences
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

	// Strip postamble filler lines and clean up resulting blank lines
	lines := strings.Split(s, "\n")
	var kept []string
	for _, line := range lines {
		if postambleFillerRe.MatchString(line) {
			// Also remove preceding blank line if present
			if len(kept) > 0 && strings.TrimSpace(kept[len(kept)-1]) == "" {
				kept = kept[:len(kept)-1]
			}
			continue
		}
		// Skip consecutive blank lines
		if strings.TrimSpace(line) == "" && len(kept) > 0 && strings.TrimSpace(kept[len(kept)-1]) == "" {
			continue
		}
		kept = append(kept, line)
	}
	s = strings.TrimSpace(strings.Join(kept, "\n"))

	return s
}
