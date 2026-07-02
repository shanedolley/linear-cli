package cmd

import (
	"strings"
	"testing"
)

// --- normalizeAttachmentProvider ---
//
// --provider accepts the canonical names and a few aliases, case-insensitively,
// and rejects anything else with an error that lists the valid providers.

func TestNormalizeAttachmentProvider_Canonical(t *testing.T) {
	for _, p := range attachmentLinkProviders {
		got, err := normalizeAttachmentProvider(p)
		if err != nil {
			t.Errorf("normalizeAttachmentProvider(%q) returned error: %v", p, err)
			continue
		}
		if got != p {
			t.Errorf("normalizeAttachmentProvider(%q) = %q, want %q", p, got, p)
		}
	}
}

func TestNormalizeAttachmentProvider_Aliases(t *testing.T) {
	cases := map[string]string{
		"github":   "github-issue",
		"gh":       "github-issue",
		"gh-issue": "github-issue",
		"pr":       "github-pr",
		"gh-pr":    "github-pr",
		"sf":       "salesforce",
		"URL":      "url",
		" Slack ":  "slack",
		"GitHub":   "github-issue",
	}
	for in, want := range cases {
		got, err := normalizeAttachmentProvider(in)
		if err != nil {
			t.Errorf("normalizeAttachmentProvider(%q) returned error: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("normalizeAttachmentProvider(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeAttachmentProvider_Unknown(t *testing.T) {
	_, err := normalizeAttachmentProvider("discord")
	if err == nil {
		t.Fatal("expected error for unsupported provider, got nil")
	}
	// The error should list the valid providers so the user can correct it.
	for _, p := range attachmentLinkProviders {
		if !strings.Contains(err.Error(), p) {
			t.Errorf("error should list valid provider %q, got: %v", p, err)
		}
	}
}
