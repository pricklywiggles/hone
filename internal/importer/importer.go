package importer

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// ImportGroup holds an optional playlist name and the URLs that belong to it.
// An empty Playlist means the URLs have no playlist assignment.
type ImportGroup struct {
	Playlist string
	URLs     []string
}

// ParseImportFile reads a hone import file and returns ordered groups.
//
// Format:
//
//	# PlaylistName       → begins a new playlist group
//	https://...          → URL belonging to the current group
//	// comment           → ignored
//	(blank lines)        → ignored
//
// URLs before the first # header belong to a group with no playlist.
func ParseImportFile(path string) ([]ImportGroup, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening import file: %w", err)
	}
	defer f.Close()

	var groups []ImportGroup
	current := ImportGroup{}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		if name, ok := strings.CutPrefix(line, "#"); ok {
			if len(current.URLs) > 0 {
				groups = append(groups, current)
			}
			current = ImportGroup{Playlist: strings.TrimSpace(name)}
			continue
		}
		current.URLs = append(current.URLs, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading import file: %w", err)
	}
	if len(current.URLs) > 0 {
		groups = append(groups, current)
	}
	return groups, nil
}
