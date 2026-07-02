package cmd

import (
	"testing"
	"time"
)

// --- parseSnoozeTime ---
//
// `notification snooze --until` accepts a YYYY-MM-DD date (interpreted in the
// caller's local timezone, not UTC, so a plain date doesn't land a day early
// west of UTC) or an RFC3339 timestamp.

func TestParseSnoozeTime_DateIsLocalMidnight(t *testing.T) {
	got, err := parseSnoozeTime("2026-08-01")
	if err != nil {
		t.Fatalf("parseSnoozeTime(date) error = %v, want nil", err)
	}
	want := time.Date(2026, 8, 1, 0, 0, 0, 0, time.Local)
	if !got.Equal(want) {
		t.Errorf("parseSnoozeTime(\"2026-08-01\") = %v, want %v (local midnight)", got, want)
	}
	if got.Location() != time.Local {
		t.Errorf("parseSnoozeTime date location = %v, want Local", got.Location())
	}
}

func TestParseSnoozeTime_RFC3339(t *testing.T) {
	got, err := parseSnoozeTime("2026-08-01T15:30:00Z")
	if err != nil {
		t.Fatalf("parseSnoozeTime(rfc3339) error = %v, want nil", err)
	}
	want := time.Date(2026, 8, 1, 15, 30, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("parseSnoozeTime(rfc3339) = %v, want %v", got, want)
	}
}

func TestParseSnoozeTime_Invalid(t *testing.T) {
	for _, s := range []string{"", "not-a-date", "08/01/2026", "next week"} {
		if _, err := parseSnoozeTime(s); err == nil {
			t.Errorf("parseSnoozeTime(%q) = nil error, want error", s)
		}
	}
}
