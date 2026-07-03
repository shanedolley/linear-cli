package cmd

import "testing"

// --- validateIssueRelationType ---
//
// `issue relate update --relation` accepts one of the IssueRelationType values
// (blocks, duplicate, related, similar), matched case-insensitively and
// returned normalized to lowercase.

func TestValidateIssueRelationType_Valid(t *testing.T) {
	cases := map[string]string{
		"blocks":    "blocks",
		"duplicate": "duplicate",
		"related":   "related",
		"similar":   "similar",
		"BLOCKS":    "blocks",
		" Related ": "related",
	}
	for in, want := range cases {
		got, err := validateIssueRelationType(in)
		if err != nil {
			t.Errorf("validateIssueRelationType(%q) returned error: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("validateIssueRelationType(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestValidateIssueRelationType_Invalid(t *testing.T) {
	for _, in := range []string{"", "blocked-by", "parent-of", "sub-issue-of"} {
		if _, err := validateIssueRelationType(in); err == nil {
			t.Errorf("validateIssueRelationType(%q) = nil error, want error", in)
		}
	}
}
