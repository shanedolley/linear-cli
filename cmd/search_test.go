package cmd

import (
	"testing"

	"github.com/shanedolley/lincli/pkg/api"
)

// --- parseSemanticTypes ---
//
// The --types flag is a comma-separated subset of issue, project, initiative,
// document, matched case-insensitively. An empty string means "all types" and
// returns a nil slice (no server-side filter).

func TestParseSemanticTypes_Valid(t *testing.T) {
	got, err := parseSemanticTypes("issue, Project ,DOCUMENT")
	if err != nil {
		t.Fatalf("parseSemanticTypes returned error: %v", err)
	}
	want := []api.SemanticSearchResultType{
		api.SemanticSearchResultTypeIssue,
		api.SemanticSearchResultTypeProject,
		api.SemanticSearchResultTypeDocument,
	}
	if len(got) != len(want) {
		t.Fatalf("parseSemanticTypes len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("parseSemanticTypes[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestParseSemanticTypes_EmptyMeansAll(t *testing.T) {
	got, err := parseSemanticTypes("")
	if err != nil {
		t.Fatalf("parseSemanticTypes(\"\") returned error: %v", err)
	}
	if got != nil {
		t.Errorf("parseSemanticTypes(\"\") = %v, want nil (all types)", got)
	}
}

func TestParseSemanticTypes_Invalid(t *testing.T) {
	if _, err := parseSemanticTypes("issue,widget"); err == nil {
		t.Error("parseSemanticTypes with an unknown type = nil error, want error")
	}
}
