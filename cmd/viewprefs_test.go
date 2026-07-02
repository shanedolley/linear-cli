package cmd

import (
	"testing"

	"github.com/shanedolley/lincli/pkg/api"
)

// --- parseViewPreferencesType ---
//
// `view prefs create --scope` selects the preference level: user or
// organization, matched case-insensitively and returned as the typed enum.

func TestParseViewPreferencesType_Valid(t *testing.T) {
	cases := map[string]api.ViewPreferencesType{
		"user":           api.ViewPreferencesTypeUser,
		"organization":   api.ViewPreferencesTypeOrganization,
		"USER":           api.ViewPreferencesTypeUser,
		" Organization ": api.ViewPreferencesTypeOrganization,
	}
	for in, want := range cases {
		got, err := parseViewPreferencesType(in)
		if err != nil {
			t.Errorf("parseViewPreferencesType(%q) returned error: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("parseViewPreferencesType(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseViewPreferencesType_Invalid(t *testing.T) {
	for _, in := range []string{"", "workspace", "team"} {
		if _, err := parseViewPreferencesType(in); err == nil {
			t.Errorf("parseViewPreferencesType(%q) = nil error, want error", in)
		}
	}
}

// --- parseJSONObject ---
//
// An empty string yields an empty (non-nil) map, which callers then reject:
// the client's stripNulls drops empty maps, so an empty preferences object
// cannot be sent. A non-empty object round-trips; malformed JSON errors.

func TestParseJSONObject_EmptyStringIsEmptyMap(t *testing.T) {
	m, err := parseJSONObject("")
	if err != nil {
		t.Fatalf("parseJSONObject(\"\") returned error: %v", err)
	}
	if m == nil {
		t.Fatal("parseJSONObject(\"\") = nil, want empty non-nil map")
	}
	if len(m) != 0 {
		t.Errorf("parseJSONObject(\"\") len = %d, want 0", len(m))
	}
}

func TestParseJSONObject_Object(t *testing.T) {
	m, err := parseJSONObject(`{"grouping":"assignee"}`)
	if err != nil {
		t.Fatalf("parseJSONObject returned error: %v", err)
	}
	if m["grouping"] != "assignee" {
		t.Errorf("parseJSONObject grouping = %v, want assignee", m["grouping"])
	}
}

func TestParseJSONObject_Invalid(t *testing.T) {
	if _, err := parseJSONObject("{not json"); err == nil {
		t.Error("parseJSONObject with malformed JSON = nil error, want error")
	}
}
