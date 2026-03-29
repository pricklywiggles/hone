package srs

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
