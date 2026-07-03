package cmd

import "testing"

// --- resolveEmoji ---
//
// A UUID reference is returned as-is without an API round-trip; only a name
// needs the emoji(id) lookup (covered by manual/smoke verification).

func TestResolveEmoji_UUIDShortCircuits(t *testing.T) {
	uuid := "d5b746fb-e47d-48c0-a299-ca5ecb6d33f2"
	// A nil client is safe here: the UUID path returns before any API call.
	got, err := resolveEmoji(nil, nil, uuid)
	if err != nil {
		t.Fatalf("resolveEmoji(uuid) returned error: %v", err)
	}
	if got != uuid {
		t.Errorf("resolveEmoji(uuid) = %q, want %q", got, uuid)
	}
}
