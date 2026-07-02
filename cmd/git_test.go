package cmd

import (
	"testing"

	"github.com/shanedolley/lincli/pkg/api"
)

// --- validateGitEvent ---
//
// The event must be one of the GitAutomationStates enum values, matched
// case-insensitively and returned as the typed enum.

func TestValidateGitEvent_Valid(t *testing.T) {
	cases := map[string]api.GitAutomationStates{
		"draft":     api.GitAutomationStatesDraft,
		"start":     api.GitAutomationStatesStart,
		"review":    api.GitAutomationStatesReview,
		"mergeable": api.GitAutomationStatesMergeable,
		"merge":     api.GitAutomationStatesMerge,
		"MERGE":     api.GitAutomationStatesMerge,
		" review ":  api.GitAutomationStatesReview,
	}
	for in, want := range cases {
		got, err := validateGitEvent(in)
		if err != nil {
			t.Errorf("validateGitEvent(%q) returned error: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("validateGitEvent(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestValidateGitEvent_Invalid(t *testing.T) {
	for _, in := range []string{"", "merged", "open", "closed"} {
		if _, err := validateGitEvent(in); err == nil {
			t.Errorf("validateGitEvent(%q) = nil error, want error", in)
		}
	}
}
