package cmd

import (
	"strings"
	"testing"
)

// --- validateWebhookURL ---
//
// Linear fetches a webhook's target URL server-side, so `webhook create`
// validates the URL locally and rejects anything pointing at an internal
// address (localhost, loopback, link-local, or private/internal IP ranges) to
// avoid inviting server-side request forgery. Public https/http URLs pass.

func TestValidateWebhookURL_PublicURLs(t *testing.T) {
	valid := []string{
		"https://example.com/webhook",
		"https://hooks.example.com/linear?token=abc",
		"http://public.example.org:8080/ingest",
		"https://93.184.216.34/hook", // example.com's public IP
	}
	for _, u := range valid {
		if err := validateWebhookURL(u); err != nil {
			t.Errorf("validateWebhookURL(%q) = %v, want nil", u, err)
		}
	}
}

func TestValidateWebhookURL_RejectsInternalHosts(t *testing.T) {
	blocked := []string{
		"http://localhost/hook",
		"https://localhost:3000/hook",
		"http://127.0.0.1/hook",         // IPv4 loopback
		"http://127.5.6.7/hook",         // rest of 127.0.0.0/8
		"http://[::1]/hook",             // IPv6 loopback
		"http://10.0.0.1/hook",          // private 10/8
		"https://172.16.5.4/hook",       // private 172.16/12
		"https://192.168.1.1/hook",      // private 192.168/16
		"http://169.254.169.254/latest", // link-local (cloud metadata)
	}
	for _, u := range blocked {
		if err := validateWebhookURL(u); err == nil {
			t.Errorf("validateWebhookURL(%q) = nil, want error (internal address)", u)
		}
	}
}

func TestValidateWebhookURL_RejectsBadScheme(t *testing.T) {
	bad := []string{
		"ftp://example.com/hook",
		"example.com/hook", // no scheme
		"",
		"https://", // no host
	}
	for _, u := range bad {
		if err := validateWebhookURL(u); err == nil {
			t.Errorf("validateWebhookURL(%q) = nil, want error (bad scheme/host)", u)
		}
	}
}

func TestValidateWebhookURL_ErrorMentionsURL(t *testing.T) {
	err := validateWebhookURL("http://127.0.0.1/hook")
	if err == nil {
		t.Fatal("expected error for loopback address, got nil")
	}
	if !strings.Contains(err.Error(), "127.0.0.1") {
		t.Errorf("expected error to mention the host, got: %v", err)
	}
}

// --- favoriteEntityField ---
//
// `favorite add --entity <kind>` maps a kind to the FavoriteCreateInput field
// it sets. Unknown kinds are rejected with a message listing valid kinds.

func TestFavoriteEntityField_KnownKinds(t *testing.T) {
	for _, kind := range []string{"issue", "project", "cycle", "document", "view", "label", "user", "initiative"} {
		if _, err := favoriteEntityKind(kind); err != nil {
			t.Errorf("favoriteEntityKind(%q) unexpected error: %v", kind, err)
		}
	}
}

func TestFavoriteEntityField_CaseInsensitive(t *testing.T) {
	if _, err := favoriteEntityKind("Issue"); err != nil {
		t.Errorf("favoriteEntityKind(Issue) should be case-insensitive, got: %v", err)
	}
}

func TestFavoriteEntityField_Unknown(t *testing.T) {
	_, err := favoriteEntityKind("banana")
	if err == nil {
		t.Fatal("expected error for unknown entity kind, got nil")
	}
	if !strings.Contains(err.Error(), "banana") {
		t.Errorf("expected error to name the invalid kind, got: %v", err)
	}
}
