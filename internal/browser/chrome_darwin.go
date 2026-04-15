//go:build darwin

package browser

func ChromePath() (string, error) {
	return "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome", nil
}
