package cmd

import (
	"testing"
	"time"
)

// --- parseScheduleEntry ---
//
// Entries are "START|END|USER"; START/END are RFC 3339 timestamps and USER maps
// to userId (when a UUID) or userEmail (otherwise).

func TestParseScheduleEntry_Email(t *testing.T) {
	entry, err := parseScheduleEntry("2026-07-10T09:00:00Z|2026-07-17T09:00:00Z|alice@example.com")
	if err != nil {
		t.Fatalf("parseScheduleEntry returned error: %v", err)
	}
	wantStart, _ := time.Parse(time.RFC3339, "2026-07-10T09:00:00Z")
	if !entry.StartsAt.Equal(wantStart) {
		t.Errorf("StartsAt = %v, want %v", entry.StartsAt, wantStart)
	}
	if entry.UserId != nil {
		t.Errorf("UserId = %v, want nil for an email", *entry.UserId)
	}
	if entry.UserEmail == nil || *entry.UserEmail != "alice@example.com" {
		t.Errorf("UserEmail = %v, want alice@example.com", entry.UserEmail)
	}
}

func TestParseScheduleEntry_UUIDUser(t *testing.T) {
	uuid := "d5b746fb-e47d-48c0-a299-ca5ecb6d33f2"
	entry, err := parseScheduleEntry("2026-07-10T09:00:00Z|2026-07-17T09:00:00Z|" + uuid)
	if err != nil {
		t.Fatalf("parseScheduleEntry returned error: %v", err)
	}
	if entry.UserId == nil || *entry.UserId != uuid {
		t.Errorf("UserId = %v, want %s", entry.UserId, uuid)
	}
	if entry.UserEmail != nil {
		t.Errorf("UserEmail = %v, want nil for a UUID user", *entry.UserEmail)
	}
}

func TestParseScheduleEntry_Errors(t *testing.T) {
	cases := map[string]string{
		"too few fields":  "2026-07-10T09:00:00Z|alice@example.com",
		"too many fields": "a|b|c|d",
		"bad start time":  "not-a-time|2026-07-17T09:00:00Z|alice@example.com",
		"bad end time":    "2026-07-10T09:00:00Z|not-a-time|alice@example.com",
		"empty user":      "2026-07-10T09:00:00Z|2026-07-17T09:00:00Z|",
	}
	for name, in := range cases {
		if _, err := parseScheduleEntry(in); err == nil {
			t.Errorf("%s: parseScheduleEntry(%q) = nil error, want error", name, in)
		}
	}
}

// --- scheduleNewerThan ---
//
// The --newer-than filter is applied client-side; an empty or unparseable
// threshold passes everything.

func TestScheduleNewerThan(t *testing.T) {
	created, _ := time.Parse(time.RFC3339, "2026-07-10T00:00:00Z")

	if !scheduleNewerThan(created, "") {
		t.Error("empty threshold should pass all schedules")
	}
	if !scheduleNewerThan(created, "not-a-time") {
		t.Error("unparseable threshold should pass all schedules")
	}
	if !scheduleNewerThan(created, "2026-07-01T00:00:00Z") {
		t.Error("schedule created after threshold should pass")
	}
	if scheduleNewerThan(created, "2026-08-01T00:00:00Z") {
		t.Error("schedule created before threshold should be filtered out")
	}
	if !scheduleNewerThan(created, "2026-07-10T00:00:00Z") {
		t.Error("schedule created exactly at threshold should pass (inclusive)")
	}
}
