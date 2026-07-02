package cmd

import (
	"strings"
	"testing"
)

// --- parseTeamOwnerRole ---
//
// `team set-role` sets a team membership's role, which Linear models as an
// owner boolean. Only "owner" and "member" are valid.

func TestParseTeamOwnerRole_Owner(t *testing.T) {
	owner, err := parseTeamOwnerRole("owner")
	if err != nil {
		t.Fatalf("parseTeamOwnerRole(owner) error = %v", err)
	}
	if !owner {
		t.Error("parseTeamOwnerRole(owner) = false, want true")
	}
}

func TestParseTeamOwnerRole_Member(t *testing.T) {
	owner, err := parseTeamOwnerRole("member")
	if err != nil {
		t.Fatalf("parseTeamOwnerRole(member) error = %v", err)
	}
	if owner {
		t.Error("parseTeamOwnerRole(member) = true, want false")
	}
}

func TestParseTeamOwnerRole_CaseInsensitive(t *testing.T) {
	owner, err := parseTeamOwnerRole("OWNER")
	if err != nil {
		t.Fatalf("parseTeamOwnerRole(OWNER) error = %v", err)
	}
	if !owner {
		t.Error("parseTeamOwnerRole(OWNER) = false, want true")
	}
}

func TestParseTeamOwnerRole_Invalid(t *testing.T) {
	_, err := parseTeamOwnerRole("admin")
	if err == nil {
		t.Fatal("expected error for an org role passed to a team command, got nil")
	}
	// The error should steer the user to the valid team-scope values.
	if !strings.Contains(err.Error(), "owner") || !strings.Contains(err.Error(), "member") {
		t.Errorf("expected error to list owner/member, got: %v", err)
	}
}

// --- validateUserRole ---
//
// `user set-role` sets an organization role: owner, admin, guest, user, or app.
// "member" is a team concept and must be rejected here.

func TestValidateUserRole_Valid(t *testing.T) {
	for _, role := range []string{"owner", "admin", "guest", "user", "app"} {
		got, err := validateUserRole(role)
		if err != nil {
			t.Errorf("validateUserRole(%q) error = %v", role, err)
			continue
		}
		if string(got) != role {
			t.Errorf("validateUserRole(%q) = %q, want %q", role, got, role)
		}
	}
}

func TestValidateUserRole_CaseInsensitive(t *testing.T) {
	got, err := validateUserRole("Admin")
	if err != nil {
		t.Fatalf("validateUserRole(Admin) error = %v", err)
	}
	if string(got) != "admin" {
		t.Errorf("validateUserRole(Admin) = %q, want %q", got, "admin")
	}
}

func TestValidateUserRole_RejectsMember(t *testing.T) {
	_, err := validateUserRole("member")
	if err == nil {
		t.Fatal("expected error for team-scope role 'member' in a user command, got nil")
	}
	if !strings.Contains(err.Error(), "owner") || !strings.Contains(err.Error(), "app") {
		t.Errorf("expected error to list the valid org roles, got: %v", err)
	}
}
