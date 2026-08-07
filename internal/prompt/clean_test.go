package prompt

import "testing"

func TestCleanResponse(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "plain message passthrough",
			in:   "feat: add user login",
			want: "feat: add user login",
		},
		{
			name: "preamble here is",
			in:   "Here is the commit message:\nfeat: add user login",
			want: "feat: add user login",
		},
		{
			name: "preamble here's",
			in:   "Here's the commit message:\nfeat: add user login",
			want: "feat: add user login",
		},
		{
			name: "preamble the commit message is",
			in:   "The commit message is:\nfeat: add user login",
			want: "feat: add user login",
		},
		{
			name: "preamble the commit message would be",
			in:   "The commit message would be:\nfeat: add user login",
			want: "feat: add user login",
		},
		{
			name: "preamble i suggest",
			in:   "I suggest the following commit message:\nfeat: add user login",
			want: "feat: add user login",
		},
		{
			name: "my suggestion is no commit/message keyword",
			in:   "My suggestion is:\nfeat: add user login",
			want: "My suggestion is:\nfeat: add user login",
		},
		{
			name: "preamble suggested commit message",
			in:   "Suggested commit message:\nfeat: add user login",
			want: "feat: add user login",
		},
		{
			name: "preamble based on changes",
			in:   "Based on the changes, here is the commit message:\nfeat: add user login",
			want: "feat: add user login",
		},
		{
			name: "preamble for the diff",
			in:   "For the changes, here's the commit message:\nfeat: add user login",
			want: "feat: add user login",
		},
		{
			name: "preamble let me generate",
			in:   "Let me generate the commit message:\nfeat: add user login",
			want: "feat: add user login",
		},
		{
			name: "preamble i'll generate",
			in:   "I'll generate the commit message:\nfeat: add user login",
			want: "feat: add user login",
		},
		{
			name: "preamble sure",
			in:   "Sure! Here is the commit message:\nfeat: add user login",
			want: "feat: add user login",
		},
		{
			name: "preamble of course",
			in:   "Of course! Here's the commit message:\nfeat: add user login",
			want: "feat: add user login",
		},
		{
			name: "code fence simple",
			in:   "```\nfeat: add user login\n```",
			want: "feat: add user login",
		},
		{
			name: "code fence with language",
			in:   "```git\nfeat: add user login\n```",
			want: "feat: add user login",
		},
		{
			name: "postamble this commit stripped",
			in:   "feat: add user login\n\nThis commit introduces authentication.",
			want: "feat: add user login",
		},
		{
			name: "postamble this change stripped",
			in:   "feat: add user login\n\nThis change adds the login endpoint.",
			want: "feat: add user login",
		},
		{
			name: "postamble the changes stripped",
			in:   "feat: add user login\n\nThe changes include:\n- Added login endpoint\n- Added session handling",
			want: "feat: add user login\n- Added login endpoint\n- Added session handling",
		},
		{
			name: "postamble i chose stripped",
			in:   "feat: add user login\n\nI chose this approach because it's simpler.",
			want: "feat: add user login",
		},
		{
			name: "postamble note stripped",
			in:   "feat: add user login\n\nNote: this is a breaking change.",
			want: "feat: add user login",
		},
		{
			name: "postamble summary stripped",
			in:   "feat: add user login\n\nSummary: added login.",
			want: "feat: add user login",
		},
		{
			name: "postamble key changes stripped",
			in:   "feat: add user login\n\nKey changes:\n- Added endpoint",
			want: "feat: add user login\n- Added endpoint",
		},
		{
			name: "postamble question stripped",
			in:   "feat: add user login\n\nWould you like me to add tests?",
			want: "feat: add user login",
		},
		{
			name: "body preserved not filler",
			in:   "feat: add login\n\nAdd OAuth2 login flow with Google and GitHub providers.",
			want: "feat: add login\n\nAdd OAuth2 login flow with Google and GitHub providers.",
		},
		{
			name: "preamble and postamble together",
			in:   "Here is the commit message:\nfeat: add user login\n\nThis commit introduces authentication.",
			want: "feat: add user login",
		},
		{
			name: "code fence with preamble outside",
			in:   "Here is the commit message:\n```\nfeat: add user login\n```",
			want: "feat: add user login",
		},
		{
			name: "whitespace trimming",
			in:   "  \n  feat: add user login  \n  ",
			want: "feat: add user login",
		},
		{
			name: "empty input",
			in:   "",
			want: "",
		},
		{
			name: "only whitespace",
			in:   "   \n   ",
			want: "",
		},
		{
			name: "no newline after colon",
			in:   "Here is the commit message: feat: add user login",
			want: "feat: add user login",
		},
		{
			name: "preamble with suggested",
			in:   "Suggested message:\nfix: resolve memory leak",
			want: "fix: resolve memory leak",
		},
		{
			name: "preamble looking at",
			in:   "Looking at the diff, here's the commit message:\nrefactor: clean up utils",
			want: "refactor: clean up utils",
		},
		{
			name: "preamble given",
			in:   "Given the changes, here is the commit message:\ndocs: update README",
			want: "docs: update README",
		},
		{
			name: "freeform message with preamble",
			in:   "Here is the commit message:\nUpdate the README with installation instructions",
			want: "Update the README with installation instructions",
		},
		{
			name: "preamble with backtick-wrapped message",
			in:   "Here is the commit message:    `fix(system-map): keep existing target spatial model without injecting base-only or standalone nodes`",
			want: "fix(system-map): keep existing target spatial model without injecting base-only or standalone nodes",
		},
		{
			name: "preamble with suitable",
			in:   "Here's a suitable commit message for the changes:\nfix: resolve memory leak",
			want: "fix: resolve memory leak",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CleanResponse(tt.in)
			if got != tt.want {
				t.Errorf("CleanResponse(%q)\n  got:  %q\n  want: %q", tt.in, got, tt.want)
			}
		})
	}
}
