package srs

import (
	"math"
	"time"
)

const dateFormat = "2006-01-02"

// Thresholds defines the fast/normal minute boundaries for quality mapping.
// Duration < Fast → quality 5; < Normal → quality 4; ≥ Normal → quality 3.
type Thresholds struct {
	Fast   int
	Normal int
}

// ProblemSRS holds the spaced repetition state for a problem.
type ProblemSRS struct {
	ProblemID       int     `db:"problem_id"`
	EasinessFactor  float64 `db:"easiness_factor"`
	IntervalDays    int     `db:"interval_days"`
	RepetitionCount int     `db:"repetition_count"`
	NextReviewDate  string  `db:"next_review_date"` // "2006-01-02"
	MasteredBefore  int     `db:"mastered_before"`  // 0 or 1
}

// UpdateEF computes the new SM-2 easiness factor given the current factor and
// a quality rating (0–5). The result is clamped to a minimum of 1.3.
func UpdateEF(ef float64, quality int) float64 {
	q := float64(quality)
	ef = ef + (0.1 - (5-q)*(0.08+(5-q)*0.02))
	if ef < 1.3 {
		return 1.3
	}
	return ef
}

// QualityFromDuration maps solve duration to a quality rating (3/4/5) using
// the provided thresholds. Failure quality (1) is set by the caller.
func QualityFromDuration(durationMin int, t Thresholds) int {
	if durationMin < t.Fast {
		return 5
	}
	if durationMin < t.Normal {
		return 4
	}
	return 3
}

// UpdateSRS applies the SM-2 algorithm and returns the new SRS state.
// today is injected for testability.
func UpdateSRS(state ProblemSRS, success bool, durationMin int, t Thresholds, today time.Time) ProblemSRS {
	var quality int
	if success {
		quality = QualityFromDuration(durationMin, t)
		state.RepetitionCount++
		switch state.RepetitionCount {
		case 1:
			state.IntervalDays = 1
		case 2:
			state.IntervalDays = 6
		default:
			state.IntervalDays = int(math.Round(float64(state.IntervalDays) * state.EasinessFactor))
		}
		// Previously-mastered problems that succeed again after coming back
		// get an aggressive boost so they don't resurface for months.
		if state.MasteredBefore == 1 && state.RepetitionCount > 1 {
			state.IntervalDays *= 2
		}
		state.EasinessFactor = UpdateEF(state.EasinessFactor, quality)
		if state.IntervalDays > 60 {
			state.MasteredBefore = 1
		}
	} else {
		quality = 1
		state.RepetitionCount = 0
		state.IntervalDays = 1
		state.EasinessFactor = UpdateEF(state.EasinessFactor, quality)
	}

	state.NextReviewDate = today.AddDate(0, 0, state.IntervalDays).Format(dateFormat)
	return state
}
