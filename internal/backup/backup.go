package backup

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/pricklywiggles/hone/internal/config"
)

const backupVersion = 1

// BackupData is the full JSON backup format for hone init/export --backup.
type BackupData struct {
	Version    int               `json:"version"`
	ExportedAt string            `json:"exported_at"`
	Problems   []ProblemBackup   `json:"problems"`
	Playlists  []PlaylistBackup  `json:"playlists"`
	Attempts   []AttemptBackup   `json:"attempts"`
}

type ProblemBackup struct {
	Platform        string   `json:"platform"`
	Slug            string   `json:"slug"`
	Title           string   `json:"title"`
	Difficulty      string   `json:"difficulty"`
	CreatedAt       string   `json:"created_at"`
	Topics          []string `json:"topics"`
	EasinessFactor  float64  `json:"easiness_factor"`
	IntervalDays    int      `json:"interval_days"`
	RepetitionCount int      `json:"repetition_count"`
	NextReviewDate  string   `json:"next_review_date"`
	MasteredBefore  bool     `json:"mastered_before"`
}

type PlaylistBackup struct {
	Name     string   `json:"name"`
	Problems []string `json:"problems"` // "platform/slug"
}

type AttemptBackup struct {
	Problem         string `json:"problem"` // "platform/slug"
	StartedAt       string `json:"started_at"`
	CompletedAt     string `json:"completed_at,omitzero"`
	Result          string `json:"result,omitzero"`
	DurationSeconds int    `json:"duration_seconds,omitzero"`
	Quality         int    `json:"quality,omitzero"`
}

// ExportFullBackup collects all problems, SRS state, playlists, and attempts.
func ExportFullBackup(db *sqlx.DB) (BackupData, error) {
	data := BackupData{
		Version:    backupVersion,
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
	}

	problems, err := loadProblemBackups(db)
	if err != nil {
		return BackupData{}, fmt.Errorf("problems: %w", err)
	}
	data.Problems = problems

	playlists, err := loadPlaylistBackups(db)
	if err != nil {
		return BackupData{}, fmt.Errorf("playlists: %w", err)
	}
	data.Playlists = playlists

	attempts, err := loadAttemptBackups(db)
	if err != nil {
		return BackupData{}, fmt.Errorf("attempts: %w", err)
	}
	data.Attempts = attempts

	return data, nil
}

// ExportPlaylistFormat produces an import-compatible text file: playlists as
// "# Name" headers, each followed by the problem URLs in that playlist.
// Problems not belonging to any playlist appear at the top with no header.
func ExportPlaylistFormat(db *sqlx.DB) (string, error) {
	type urlRow struct {
		Platform string `db:"platform"`
		Slug     string `db:"slug"`
	}

	// Problems not in any playlist.
	var unowned []urlRow
	err := db.Select(&unowned, `
		SELECT p.platform, p.slug
		FROM problems p
		WHERE NOT EXISTS (
			SELECT 1 FROM playlist_problems pp WHERE pp.problem_id = p.id
		)
		ORDER BY p.platform, p.slug`)
	if err != nil {
		return "", err
	}

	type playlistRow struct {
		ID   int    `db:"id"`
		Name string `db:"name"`
	}
	var playlists []playlistRow
	if err := db.Select(&playlists, `SELECT id, name FROM playlists ORDER BY name`); err != nil {
		return "", err
	}

	var sb strings.Builder

	for _, r := range unowned {
		sb.WriteString(config.BuildURL(r.Platform, r.Slug))
		sb.WriteByte('\n')
	}

	for i, pl := range playlists {
		var members []urlRow
		err := db.Select(&members, `
			SELECT p.platform, p.slug
			FROM problems p
			JOIN playlist_problems pp ON pp.problem_id = p.id
			WHERE pp.playlist_id = ?
			ORDER BY pp.position, p.platform, p.slug`, pl.ID)
		if err != nil {
			return "", err
		}
		if len(members) == 0 {
			continue
		}
		if i > 0 || len(unowned) > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString("# ")
		sb.WriteString(pl.Name)
		sb.WriteByte('\n')
		for _, r := range members {
			sb.WriteString(config.BuildURL(r.Platform, r.Slug))
			sb.WriteByte('\n')
		}
	}

	return sb.String(), nil
}

// ExportSinglePlaylistFormat produces import-compatible text for a single
// playlist, identified by name.
func ExportSinglePlaylistFormat(db *sqlx.DB, name string) (string, error) {
	type urlRow struct {
		Platform string `db:"platform"`
		Slug     string `db:"slug"`
	}

	var playlistID int
	err := db.Get(&playlistID, `SELECT id FROM playlists WHERE name = ?`, name)
	if err != nil {
		return "", fmt.Errorf("playlist %q not found", name)
	}

	var members []urlRow
	err = db.Select(&members, `
		SELECT p.platform, p.slug
		FROM problems p
		JOIN playlist_problems pp ON pp.problem_id = p.id
		WHERE pp.playlist_id = ?
		ORDER BY pp.position, p.platform, p.slug`, playlistID)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString("# ")
	sb.WriteString(name)
	sb.WriteByte('\n')
	for _, r := range members {
		sb.WriteString(config.BuildURL(r.Platform, r.Slug))
		sb.WriteByte('\n')
	}
	return sb.String(), nil
}

// ── internal helpers ──────────────────────────────────────────────────────────

type problemRow struct {
	ID              int     `db:"id"`
	Platform        string  `db:"platform"`
	Slug            string  `db:"slug"`
	Title           string  `db:"title"`
	Difficulty      string  `db:"difficulty"`
	CreatedAt       string  `db:"created_at"`
	TopicsStr       string  `db:"topics_str"`
	EasinessFactor  float64 `db:"easiness_factor"`
	IntervalDays    int     `db:"interval_days"`
	RepetitionCount int     `db:"repetition_count"`
	NextReviewDate  string  `db:"next_review_date"`
	MasteredBefore  int     `db:"mastered_before"`
}

func loadProblemBackups(db *sqlx.DB) ([]ProblemBackup, error) {
	var rows []problemRow
	err := db.Select(&rows, `
		SELECT p.id, p.platform, p.slug, p.title, p.difficulty, p.created_at,
			COALESCE(group_concat(t.name, '|'), '') AS topics_str,
			ps.easiness_factor, ps.interval_days, ps.repetition_count,
			ps.next_review_date, ps.mastered_before
		FROM problems p
		LEFT JOIN problem_srs ps ON ps.problem_id = p.id
		LEFT JOIN problem_topics pt ON pt.problem_id = p.id
		LEFT JOIN topics t ON t.id = pt.topic_id
		GROUP BY p.id
		ORDER BY p.id`)
	if err != nil {
		return nil, err
	}

	out := make([]ProblemBackup, len(rows))
	for i, r := range rows {
		var topics []string
		if r.TopicsStr != "" {
			topics = slices.Collect(strings.SplitSeq(r.TopicsStr, "|"))
		}
		out[i] = ProblemBackup{
			Platform:        r.Platform,
			Slug:            r.Slug,
			Title:           r.Title,
			Difficulty:      r.Difficulty,
			CreatedAt:       r.CreatedAt,
			Topics:          topics,
			EasinessFactor:  r.EasinessFactor,
			IntervalDays:    r.IntervalDays,
			RepetitionCount: r.RepetitionCount,
			NextReviewDate:  r.NextReviewDate,
			MasteredBefore:  r.MasteredBefore == 1,
		}
	}
	return out, nil
}

func loadPlaylistBackups(db *sqlx.DB) ([]PlaylistBackup, error) {
	type row struct {
		Name     string `db:"name"`
		Platform string `db:"platform"`
		Slug     string `db:"slug"`
	}
	var rows []row
	err := db.Select(&rows, `
		SELECT pl.name, p.platform, p.slug
		FROM playlists pl
		JOIN playlist_problems pp ON pp.playlist_id = pl.id
		JOIN problems p ON p.id = pp.problem_id
		ORDER BY pl.name, pp.position, p.platform, p.slug`)
	if err != nil {
		return nil, err
	}

	var out []PlaylistBackup
	for _, r := range rows {
		if len(out) == 0 || out[len(out)-1].Name != r.Name {
			out = append(out, PlaylistBackup{Name: r.Name})
		}
		out[len(out)-1].Problems = append(out[len(out)-1].Problems, r.Platform+"/"+r.Slug)
	}

	// Include empty playlists too.
	type plRow struct {
		Name string `db:"name"`
	}
	var allPl []plRow
	if err := db.Select(&allPl, `SELECT name FROM playlists ORDER BY name`); err != nil {
		return nil, err
	}
	existing := make(map[string]bool, len(out))
	for _, p := range out {
		existing[p.Name] = true
	}
	for _, pl := range allPl {
		if !existing[pl.Name] {
			out = append(out, PlaylistBackup{Name: pl.Name, Problems: []string{}})
		}
	}

	return out, nil
}

func loadAttemptBackups(db *sqlx.DB) ([]AttemptBackup, error) {
	type row struct {
		Platform        string `db:"platform"`
		Slug            string `db:"slug"`
		StartedAt       string `db:"started_at"`
		CompletedAt     string `db:"completed_at"`
		Result          string `db:"result"`
		DurationSeconds int    `db:"duration_seconds"`
		Quality         int    `db:"quality"`
	}
	var rows []row
	err := db.Select(&rows, `
		SELECT p.platform, p.slug,
			a.started_at, COALESCE(a.completed_at, '') AS completed_at,
			COALESCE(a.result, '') AS result,
			COALESCE(a.duration_seconds, 0) AS duration_seconds,
			COALESCE(a.quality, 0) AS quality
		FROM attempts a
		JOIN problems p ON p.id = a.problem_id
		ORDER BY a.id`)
	if err != nil {
		return nil, err
	}

	out := make([]AttemptBackup, len(rows))
	for i, r := range rows {
		out[i] = AttemptBackup{
			Problem:         r.Platform + "/" + r.Slug,
			StartedAt:       r.StartedAt,
			CompletedAt:     r.CompletedAt,
			Result:          r.Result,
			DurationSeconds: r.DurationSeconds,
			Quality:         r.Quality,
		}
	}
	return out, nil
}
