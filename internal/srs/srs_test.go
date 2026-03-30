package srs

import (
	"math"
	"testing"
	"time"
)

var today = time.Date(2026, 3, 29, 0, 0, 0, 0, time.UTC)

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

func TestQualityFromDuration(t *testing.T) {
	easy := Thresholds{Fast: 10, Normal: 20}
	medium := Thresholds{Fast: 15, Normal: 30}
	hard := Thresholds{Fast: 20, Normal: 40}

	tests := []struct {
		name        string
		durationMin int
		thresholds  Thresholds
		want        int
	}{
		// Easy
		{"easy fast", 9, easy, 5},
		{"easy fast boundary", 10, easy, 4},
		{"easy normal", 15, easy, 4},
		{"easy normal boundary", 20, easy, 3},
		{"easy slow", 25, easy, 3},
		// Medium
		{"medium fast", 14, medium, 5},
		{"medium fast boundary", 15, medium, 4},
		{"medium normal", 22, medium, 4},
		{"medium normal boundary", 30, medium, 3},
		{"medium slow", 45, medium, 3},
		// Hard
		{"hard fast", 19, hard, 5},
		{"hard fast boundary", 20, hard, 4},
		{"hard normal", 30, hard, 4},
		{"hard normal boundary", 40, hard, 3},
		{"hard slow", 60, hard, 3},
		// Edge cases
		{"zero duration", 0, easy, 5},
		{"exactly at fast", 10, easy, 4},
		{"exactly at normal", 20, easy, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := QualityFromDuration(tt.durationMin, tt.thresholds)
			if got != tt.want {
				t.Errorf("QualityFromDuration(%v, %v) = %v, want %v", tt.durationMin, tt.thresholds, got, tt.want)
			}
		})
	}
}

func TestUpdateSRS_FirstSuccess(t *testing.T) {
	state := defaultState(1)
	result := UpdateSRS(state, true, 5, Thresholds{Fast: 10, Normal: 20}, today)

	if result.RepetitionCount != 1 {
		t.Errorf("RepetitionCount = %v, want 1", result.RepetitionCount)
	}
	if result.IntervalDays != 1 {
		t.Errorf("IntervalDays = %v, want 1", result.IntervalDays)
	}
	if result.NextReviewDate != "2026-03-30" {
		t.Errorf("NextReviewDate = %v, want 2026-03-30", result.NextReviewDate)
	}
	if math.Abs(result.EasinessFactor-2.6) > 0.001 {
		t.Errorf("EasinessFactor = %v, want 2.6", result.EasinessFactor)
	}
}

func TestUpdateSRS_SecondSuccess(t *testing.T) {
	state := defaultState(1)
	state.RepetitionCount = 1
	state.IntervalDays = 1

	result := UpdateSRS(state, true, 5, Thresholds{Fast: 10, Normal: 20}, today)

	if result.RepetitionCount != 2 {
		t.Errorf("RepetitionCount = %v, want 2", result.RepetitionCount)
	}
	if result.IntervalDays != 6 {
		t.Errorf("IntervalDays = %v, want 6", result.IntervalDays)
	}
	if result.NextReviewDate != "2026-04-04" {
		t.Errorf("NextReviewDate = %v, want 2026-04-04", result.NextReviewDate)
	}
}

func TestUpdateSRS_ThirdSuccess(t *testing.T) {
	state := defaultState(1)
	state.RepetitionCount = 2
	state.IntervalDays = 6
	state.EasinessFactor = 2.5

	result := UpdateSRS(state, true, 5, Thresholds{Fast: 10, Normal: 20}, today)

	// interval = round(6 * 2.5) = 15, EF boosted by quality 5
	if result.RepetitionCount != 3 {
		t.Errorf("RepetitionCount = %v, want 3", result.RepetitionCount)
	}
	if result.IntervalDays != 15 {
		t.Errorf("IntervalDays = %v, want 15", result.IntervalDays)
	}
}

func TestUpdateSRS_Failure(t *testing.T) {
	state := defaultState(1)
	state.RepetitionCount = 3
	state.IntervalDays = 15
	state.EasinessFactor = 2.5

	result := UpdateSRS(state, false, 0, Thresholds{Fast: 10, Normal: 20}, today)

	if result.RepetitionCount != 0 {
		t.Errorf("RepetitionCount = %v, want 0", result.RepetitionCount)
	}
	if result.IntervalDays != 1 {
		t.Errorf("IntervalDays = %v, want 1", result.IntervalDays)
	}
	if result.NextReviewDate != "2026-03-30" {
		t.Errorf("NextReviewDate = %v, want 2026-03-30", result.NextReviewDate)
	}
	// EF decreases on failure: 2.5 + (0.1 - 4*(0.08+4*0.02)) = 2.5 - 0.54 = 1.96
	if math.Abs(result.EasinessFactor-1.96) > 0.001 {
		t.Errorf("EasinessFactor = %v, want 1.96", result.EasinessFactor)
	}
}

func TestUpdateSRS_FailureClampEF(t *testing.T) {
	state := defaultState(1)
	state.EasinessFactor = 1.3

	result := UpdateSRS(state, false, 0, Thresholds{Fast: 10, Normal: 20}, today)

	if math.Abs(result.EasinessFactor-1.3) > 0.001 {
		t.Errorf("EasinessFactor = %v, want 1.3 (clamped)", result.EasinessFactor)
	}
}

func TestUpdateSRS_MasteredBeforeBoost(t *testing.T) {
	// Problem was mastered, failed once, now succeeding twice in a row
	state := defaultState(1)
	state.MasteredBefore = 1
	state.RepetitionCount = 1 // first success already recorded
	state.IntervalDays = 6
	state.EasinessFactor = 2.5

	result := UpdateSRS(state, true, 5, Thresholds{Fast: 10, Normal: 20}, today)

	// rep_count becomes 2 (> 1), mastered_before=1 → boost: interval = 6*2.5 rounded = 15, then *2 = 30
	// Wait: rep=2 → interval_days = 6 (fixed for rep 2), then boost *2 = 12
	if result.RepetitionCount != 2 {
		t.Errorf("RepetitionCount = %v, want 2", result.RepetitionCount)
	}
	if result.IntervalDays != 12 {
		t.Errorf("IntervalDays = %v, want 12 (6 * 2 boost)", result.IntervalDays)
	}
}

func TestUpdateSRS_NoBoostOnFirstSuccessAfterFailure(t *testing.T) {
	// Problem was mastered, failed, first success — no boost yet
	state := defaultState(1)
	state.MasteredBefore = 1
	state.RepetitionCount = 0 // reset by failure
	state.IntervalDays = 1

	result := UpdateSRS(state, true, 5, Thresholds{Fast: 10, Normal: 20}, today)

	// rep_count becomes 1 — boost condition (> 1) not met
	if result.IntervalDays != 1 {
		t.Errorf("IntervalDays = %v, want 1 (no boost on first success)", result.IntervalDays)
	}
}

func TestUpdateSRS_SetsMasteredBefore(t *testing.T) {
	state := defaultState(1)
	state.RepetitionCount = 5
	state.IntervalDays = 30
	state.EasinessFactor = 2.5

	result := UpdateSRS(state, true, 5, Thresholds{Fast: 10, Normal: 20}, today)

	// interval = round(30 * 2.5) = 75 > 60 → mastered_before set to 1
	if result.MasteredBefore != 1 {
		t.Errorf("MasteredBefore = %v, want 1", result.MasteredBefore)
	}
}

func TestUpdateSRS_SlowSolveQuality3(t *testing.T) {
	state := defaultState(1)
	result := UpdateSRS(state, true, 25, Thresholds{Fast: 10, Normal: 20}, today)

	// quality 3 → EF: 2.5 + (0.1 - 2*(0.08 + 2*0.02)) = 2.5 + (0.1 - 0.48) = 2.5 - 0.38 = 2.12... wait
	// EF' = 2.5 + (0.1 - (5-3)*(0.08 + (5-3)*0.02)) = 2.5 + (0.1 - 2*(0.08+0.04)) = 2.5 + (0.1 - 0.24) = 2.5 - 0.14 = 2.36
	if math.Abs(result.EasinessFactor-2.36) > 0.001 {
		t.Errorf("EasinessFactor = %v, want 2.36 for quality 3", result.EasinessFactor)
	}
}

// defaultState returns a ProblemSRS in its initial state for the given problemID.
func defaultState(problemID int) ProblemSRS {
	return ProblemSRS{
		ProblemID:       problemID,
		EasinessFactor:  2.5,
		IntervalDays:    1,
		RepetitionCount: 0,
		NextReviewDate:  "2026-03-29",
		MasteredBefore:  0,
	}
}
