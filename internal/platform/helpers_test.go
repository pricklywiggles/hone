package platform

import (
	"testing"
)

func TestDedup(t *testing.T) {
	t.Run("removes duplicates", func(t *testing.T) {
		got := Dedup([]string{"a", "b", "a", "c", "b"})
		want := []string{"a", "b", "c"}
		if len(got) != len(want) {
			t.Fatalf("got %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("got[%d] = %q, want %q", i, got[i], want[i])
			}
		}
	})

	t.Run("empty input", func(t *testing.T) {
		got := Dedup([]string{})
		if len(got) != 0 {
			t.Errorf("got %v, want empty", got)
		}
	})

	t.Run("no duplicates unchanged", func(t *testing.T) {
		got := Dedup([]string{"x", "y", "z"})
		if len(got) != 3 {
			t.Errorf("got %v, want 3 elements", got)
		}
	})
}
