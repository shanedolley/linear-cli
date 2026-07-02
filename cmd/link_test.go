package cmd

import "testing"

// --- validateLinkParent ---
//
// `link add` attaches an external link to exactly one parent entity. The user
// passes exactly one of --initiative or --project; zero or both is an error.

func TestValidateLinkParent_ExactlyOne(t *testing.T) {
	if kind, ref, err := validateLinkParent("Q3 Roadmap", ""); err != nil {
		t.Errorf("initiative-only returned error: %v", err)
	} else if kind != "initiative" || ref != "Q3 Roadmap" {
		t.Errorf("initiative-only = (%q, %q), want (initiative, Q3 Roadmap)", kind, ref)
	}

	if kind, ref, err := validateLinkParent("", "API"); err != nil {
		t.Errorf("project-only returned error: %v", err)
	} else if kind != "project" || ref != "API" {
		t.Errorf("project-only = (%q, %q), want (project, API)", kind, ref)
	}
}

func TestValidateLinkParent_None(t *testing.T) {
	if _, _, err := validateLinkParent("", ""); err == nil {
		t.Error("no parent = nil error, want error")
	}
}

func TestValidateLinkParent_Both(t *testing.T) {
	if _, _, err := validateLinkParent("Roadmap", "API"); err == nil {
		t.Error("both parents = nil error, want error")
	}
}
