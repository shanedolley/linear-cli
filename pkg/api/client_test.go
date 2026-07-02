package api

import (
	"reflect"
	"testing"
)

func TestStripNulls_TopLevelNullsRemoved(t *testing.T) {
	in := map[string]interface{}{
		"eq":       "x",
		"contains": nil,
		"in":       nil,
	}
	got := stripNulls(in)
	want := map[string]interface{}{"eq": "x"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("stripNulls = %#v, want %#v", got, want)
	}
}

// Regression test: null comparator fields nested inside "or"/"and" filter
// arrays must be stripped, otherwise Linear treats them as constraints and the
// filter matches nothing.
func TestStripNulls_RecursesIntoSliceMaps(t *testing.T) {
	in := map[string]interface{}{
		"or": []interface{}{
			map[string]interface{}{
				"email": map[string]interface{}{"eq": "a@b.com", "contains": nil, "in": nil},
			},
			map[string]interface{}{
				"name": map[string]interface{}{"eq": "a@b.com", "startsWith": nil},
			},
		},
	}
	got := stripNulls(in)
	want := map[string]interface{}{
		"or": []interface{}{
			map[string]interface{}{"email": map[string]interface{}{"eq": "a@b.com"}},
			map[string]interface{}{"name": map[string]interface{}{"eq": "a@b.com"}},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("stripNulls did not strip nulls inside slice maps:\n got  %#v\n want %#v", got, want)
	}
}

func TestStripNulls_KeepsScalarSliceElements(t *testing.T) {
	in := map[string]interface{}{
		"memberIds": []interface{}{"id-1", "id-2"},
	}
	got := stripNulls(in)
	want := map[string]interface{}{"memberIds": []interface{}{"id-1", "id-2"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("stripNulls = %#v, want %#v", got, want)
	}
}

func TestStripNulls_NullSentinelBecomesNull(t *testing.T) {
	in := map[string]interface{}{"projectId": NullSentinel}
	got := stripNulls(in)
	v, ok := got["projectId"]
	if !ok || v != nil {
		t.Fatalf("NullSentinel not converted to null: got %#v", got)
	}
}

// Regression test for defect 1 in ADR 0001: an object that strips to empty must
// survive so a caller can send an intentionally empty object, rather than being
// silently dropped.
func TestStripNulls_KeepsEmptyObject(t *testing.T) {
	in := map[string]interface{}{
		"meta":  map[string]interface{}{"cleared": nil},
		"keep":  "x",
		"empty": map[string]interface{}{},
	}
	got := stripNulls(in)
	if m, ok := got["meta"].(map[string]interface{}); !ok || len(m) != 0 {
		t.Errorf("nested object that stripped to empty was dropped: got %#v", got["meta"])
	}
	if m, ok := got["empty"].(map[string]interface{}); !ok || len(m) != 0 {
		t.Errorf("intentionally empty object was dropped: got %#v", got["empty"])
	}
}

// Regression test for defect 2 in ADR 0001: because NullSentinel is randomized
// per process, a user value that resembles the old fixed marker must NOT be
// turned into null.
func TestStripNulls_DoesNotNullCollidingLiteral(t *testing.T) {
	in := map[string]interface{}{"title": "__LINCLI_NULL__"}
	got := stripNulls(in)
	if got["title"] != "__LINCLI_NULL__" {
		t.Errorf("a literal resembling the old sentinel was altered: got %#v", got["title"])
	}
}

// Regression test: LINCLI_DEBUG_GQL must not print secret-bearing variables.
// The secret lives one level down inside the mutation's input object, so the
// redaction has to recurse.
func TestRedactSensitive_RedactsNestedSecrets(t *testing.T) {
	in := map[string]interface{}{
		"input": map[string]interface{}{
			"url":    "https://example.com/hook",
			"secret": "s3cr3t-signing-key",
			"label":  "prod",
		},
	}
	got := redactSensitive(in).(map[string]interface{})
	inner := got["input"].(map[string]interface{})
	if inner["secret"] != "[REDACTED]" {
		t.Errorf("secret not redacted: got %#v", inner["secret"])
	}
	if inner["url"] != "https://example.com/hook" || inner["label"] != "prod" {
		t.Errorf("non-secret fields altered: got %#v", inner)
	}
	// The original map must be untouched (redaction returns a copy).
	if in["input"].(map[string]interface{})["secret"] != "s3cr3t-signing-key" {
		t.Errorf("redactSensitive mutated its input")
	}
}
