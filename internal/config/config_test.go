package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppendToFile(t *testing.T) {
	t.Run("creates file and writes URL", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "failed.txt")
		appendToFile(path, "https://example.com/problem-1")

		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "https://example.com/problem-1\n" {
			t.Errorf("got %q, want single URL with newline", string(data))
		}
	})

	t.Run("appends to existing file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "failed.txt")
		appendToFile(path, "https://example.com/first")
		appendToFile(path, "https://example.com/second")
		appendToFile(path, "https://example.com/third")

		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		if len(lines) != 3 {
			t.Fatalf("got %d lines, want 3", len(lines))
		}
		if lines[0] != "https://example.com/first" {
			t.Errorf("line 0 = %q, want first URL", lines[0])
		}
		if lines[2] != "https://example.com/third" {
			t.Errorf("line 2 = %q, want third URL", lines[2])
		}
	})
}
