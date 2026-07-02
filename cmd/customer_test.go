package cmd

import (
	"strings"
	"testing"

	"github.com/shanedolley/lincli/pkg/api"
)

// --- validateCustomerNeedTarget ---
//
// customerNeedCreate rejects a need that links to neither an issue nor a
// project ("Either issueId or projectId must be provided"), so the command
// validates the target locally first.

func TestValidateCustomerNeedTarget_IssueOnly(t *testing.T) {
	if err := validateCustomerNeedTarget("ENG-1", ""); err != nil {
		t.Errorf("validateCustomerNeedTarget(issue) = %v, want nil", err)
	}
}

func TestValidateCustomerNeedTarget_ProjectOnly(t *testing.T) {
	if err := validateCustomerNeedTarget("", "Roadmap"); err != nil {
		t.Errorf("validateCustomerNeedTarget(project) = %v, want nil", err)
	}
}

func TestValidateCustomerNeedTarget_Neither(t *testing.T) {
	err := validateCustomerNeedTarget("", "")
	if err == nil {
		t.Fatal("expected error when neither issue nor project is set, got nil")
	}
	if !strings.Contains(err.Error(), "issue") || !strings.Contains(err.Error(), "project") {
		t.Errorf("expected error to name both --issue and --project, got: %v", err)
	}
}

// --- customerNeedSummary ---
//
// A need's summary prefers its manual body, falls back to attachment-derived
// content, then "-".

func TestCustomerNeedSummary_PrefersBody(t *testing.T) {
	body := "manual body"
	content := "attachment content"
	f := api.CustomerNeedFields{Body: &body, Content: &content}
	if got := customerNeedSummary(f); got != body {
		t.Errorf("customerNeedSummary = %q, want %q", got, body)
	}
}

func TestCustomerNeedSummary_FallsBackToContent(t *testing.T) {
	content := "attachment content"
	f := api.CustomerNeedFields{Content: &content}
	if got := customerNeedSummary(f); got != content {
		t.Errorf("customerNeedSummary = %q, want %q", got, content)
	}
}

func TestCustomerNeedSummary_Empty(t *testing.T) {
	f := api.CustomerNeedFields{}
	if got := customerNeedSummary(f); got != "-" {
		t.Errorf("customerNeedSummary = %q, want %q", got, "-")
	}
}
