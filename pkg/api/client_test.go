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
