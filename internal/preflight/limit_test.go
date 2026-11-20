package preflight

import (
	"reflect"
	"testing"
)

func TestLimitCapsTheHintList(t *testing.T) {
	values := []string{"a", "b", "c", "d"}
	if got := limit(values, 3); !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Fatalf("limit = %v, want the first three", got)
	}
	if got := limit(values, 4); !reflect.DeepEqual(got, values) {
		t.Fatalf("limit = %v, want every value", got)
	}
	if got := limit(nil, 2); len(got) != 0 {
		t.Fatalf("limit = %v, want an empty list", got)
	}
}
