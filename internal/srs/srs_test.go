package srs

import (
	"math"
	"testing"
)

func TestUpdateEF(t *testing.T) {
	tests := []struct {
		name    string
		ef      float64
		quality int
		want    float64
	}{
		{"perfect response", 2.5, 5, 2.6},
		{"correct with hesitation", 2.5, 4, 2.5},
		{"correct with difficulty", 2.5, 3, 2.36},
		{"incorrect easy recall", 2.5, 2, 2.18},
		{"blackout", 2.5, 1, 1.96},
		{"complete blackout", 2.5, 0, 1.7},
		{"clamp at minimum", 1.3, 0, 1.3},
		{"clamp near minimum", 1.4, 0, 1.3},
		{"high EF perfect", 3.0, 5, 3.1},
		{"repeated failures stay clamped", 1.3, 1, 1.3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := UpdateEF(tt.ef, tt.quality)
			if math.Abs(got-tt.want) > 0.001 {
				t.Errorf("UpdateEF(%v, %v) = %v, want %v", tt.ef, tt.quality, got, tt.want)
			}
		})
	}
}

func TestUpdateEF_Quality5AlwaysIncreases(t *testing.T) {
	for _, ef := range []float64{1.3, 1.5, 2.0, 2.5, 3.0} {
		got := UpdateEF(ef, 5)
		if got <= ef {
			t.Errorf("UpdateEF(%v, 5) = %v, expected increase", ef, got)
		}
	}
}
