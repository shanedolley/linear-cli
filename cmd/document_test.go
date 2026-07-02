package cmd

import (
	"strings"
	"testing"
)

// --- validateDocumentParent ---
//
// `document create` scopes a document to exactly one parent: --project or
// --initiative. Neither and both are errors.

func TestValidateDocumentParent_ProjectOnly(t *testing.T) {
	if err := validateDocumentParent("Roadmap", ""); err != nil {
		t.Errorf("validateDocumentParent(project) error = %v, want nil", err)
	}
}

func TestValidateDocumentParent_InitiativeOnly(t *testing.T) {
	if err := validateDocumentParent("", "Platform"); err != nil {
		t.Errorf("validateDocumentParent(initiative) error = %v, want nil", err)
	}
}

func TestValidateDocumentParent_Neither(t *testing.T) {
	err := validateDocumentParent("", "")
	if err == nil {
		t.Fatal("expected error when neither parent is set, got nil")
	}
	if !strings.Contains(err.Error(), "project") || !strings.Contains(err.Error(), "initiative") {
		t.Errorf("expected error to name both flags, got: %v", err)
	}
}

func TestValidateDocumentParent_Both(t *testing.T) {
	err := validateDocumentParent("Roadmap", "Platform")
	if err == nil {
		t.Fatal("expected error when both parents are set, got nil")
	}
}

// --- documentHistoryNodes ---
//
// `document get --history` on a document with no content id leaves the history
// response nil; JSON output must still emit an empty list, not panic.

func TestDocumentHistoryNodes_Nil(t *testing.T) {
	got := documentHistoryNodes(nil)
	nodes, ok := got.([]interface{})
	if !ok {
		t.Fatalf("documentHistoryNodes(nil) = %T, want []interface{}", got)
	}
	if len(nodes) != 0 {
		t.Errorf("documentHistoryNodes(nil) len = %d, want 0", len(nodes))
	}
}
