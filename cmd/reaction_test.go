package cmd

import (
	"strings"
	"testing"
)

// --- findReactionID ---
//
// findReactionID picks the reaction to delete when removing "my" reaction:
// emoji alone is ambiguous once several users have reacted the same way, so a
// match requires both the emoji and the current user's id.

func TestFindReactionID_Match(t *testing.T) {
	reactions := []reactionInfo{
		{id: "r1", emoji: "👍", userID: "user-1"},
		{id: "r2", emoji: "🎉", userID: "user-1"},
		{id: "r3", emoji: "👍", userID: "user-2"},
	}

	got, err := findReactionID(reactions, "👍", "user-1")
	if err != nil {
		t.Fatalf("findReactionID() error = %v", err)
	}
	if got != "r1" {
		t.Errorf("findReactionID() = %q, want %q", got, "r1")
	}
}

func TestFindReactionID_MatchesOwnerNotJustEmoji(t *testing.T) {
	// Same emoji reacted by two users; only the current user's reaction id
	// should be returned.
	reactions := []reactionInfo{
		{id: "other", emoji: "👍", userID: "user-2"},
		{id: "mine", emoji: "👍", userID: "user-1"},
	}

	got, err := findReactionID(reactions, "👍", "user-1")
	if err != nil {
		t.Fatalf("findReactionID() error = %v", err)
	}
	if got != "mine" {
		t.Errorf("findReactionID() = %q, want %q", got, "mine")
	}
}

func TestFindReactionID_NoMatchingEmoji(t *testing.T) {
	reactions := []reactionInfo{
		{id: "r1", emoji: "tada", userID: "user-1"},
	}

	_, err := findReactionID(reactions, "👍", "user-1")
	if err == nil {
		t.Fatal("expected error when no reaction matches the emoji, got nil")
	}
	if !strings.Contains(err.Error(), "👍") {
		t.Errorf("expected error to mention the requested emoji, got: %v", err)
	}
	// The error should list the user's actual reactions so they know what to pass.
	if !strings.Contains(err.Error(), "tada") {
		t.Errorf("expected error to list the user's existing reactions, got: %v", err)
	}
}

// Linear normalizes reactions on create: a "👍" is stored as "+1". Removal by
// unicode emoji must still match the stored short name.
func TestFindReactionID_NormalizesUnicodeToShortName(t *testing.T) {
	reactions := []reactionInfo{
		{id: "r1", emoji: "+1", userID: "user-1"},
	}

	got, err := findReactionID(reactions, "👍", "user-1")
	if err != nil {
		t.Fatalf("findReactionID() error = %v", err)
	}
	if got != "r1" {
		t.Errorf("findReactionID() = %q, want %q", got, "r1")
	}
}

// A short name wrapped in colons (":tada:") should match a stored "tada".
func TestFindReactionID_NormalizesColonsAndCase(t *testing.T) {
	reactions := []reactionInfo{
		{id: "r1", emoji: "tada", userID: "user-1"},
	}

	got, err := findReactionID(reactions, ":TADA:", "user-1")
	if err != nil {
		t.Fatalf("findReactionID() error = %v", err)
	}
	if got != "r1" {
		t.Errorf("findReactionID() = %q, want %q", got, "r1")
	}
}

func TestFindReactionID_EmojiExistsButDifferentUser(t *testing.T) {
	reactions := []reactionInfo{
		{id: "r1", emoji: "👍", userID: "someone-else"},
	}

	_, err := findReactionID(reactions, "👍", "user-1")
	if err == nil {
		t.Fatal("expected error when only another user reacted with the emoji, got nil")
	}
}

func TestFindReactionID_Empty(t *testing.T) {
	_, err := findReactionID(nil, "👍", "user-1")
	if err == nil {
		t.Fatal("expected error for an entity with no reactions, got nil")
	}
}
