package store

import (
	"database/sql"

	"github.com/jmoiron/sqlx"
)

// Playlist mirrors the playlists table.
type Playlist struct {
	ID           int    `db:"id"`
	Name         string `db:"name"`
	CreatedAt    string `db:"created_at"`
	ProblemCount int    `db:"problem_count"`
}

// CreatePlaylist inserts a new playlist. Returns error if name already exists.
func CreatePlaylist(db *sqlx.DB, name string) (int64, error) {
	result, err := db.Exec(`INSERT INTO playlists (name) VALUES (?)`, name)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// ListPlaylists returns all playlists with their problem counts.
func ListPlaylists(db *sqlx.DB) ([]Playlist, error) {
	var playlists []Playlist
	err := db.Select(&playlists, `
		SELECT p.id, p.name, p.created_at, COUNT(pp.problem_id) AS problem_count
		FROM playlists p
		LEFT JOIN playlist_problems pp ON pp.playlist_id = p.id
		GROUP BY p.id
		ORDER BY p.name`)
	if err != nil {
		return nil, err
	}
	return playlists, nil
}

// SimpleProblem is a lightweight problem record for picker/list views.
type SimpleProblem struct {
	ID         int    `db:"id"`
	Title      string `db:"title"`
	Difficulty string `db:"difficulty"`
}

// AddProblemToPlaylist links a problem to a playlist. Idempotent.
// Position is set to one past the current max for the playlist.
func AddProblemToPlaylist(db *sqlx.DB, playlistID, problemID int) error {
	_, err := db.Exec(
		`INSERT OR IGNORE INTO playlist_problems (playlist_id, problem_id, position)
		 VALUES (?, ?, COALESCE((SELECT MAX(position) + 1 FROM playlist_problems WHERE playlist_id = ?), 0))`,
		playlistID, problemID, playlistID,
	)
	return err
}

// RemoveProblemFromPlaylist unlinks a problem from a playlist.
func RemoveProblemFromPlaylist(db *sqlx.DB, playlistID, problemID int) error {
	_, err := db.Exec(
		`DELETE FROM playlist_problems WHERE playlist_id = ? AND problem_id = ?`,
		playlistID, problemID,
	)
	return err
}

// ListAllProblemsForPicker returns all problems ordered by title, annotated with
// whether they already belong to playlistID.
func ListAllProblemsForPicker(db *sqlx.DB, playlistID int) ([]SimpleProblem, []bool, error) {
	type pickerRow struct {
		ID         int    `db:"id"`
		Title      string `db:"title"`
		Difficulty string `db:"difficulty"`
		InPlaylist bool   `db:"in_playlist"`
	}
	var rows []pickerRow
	err := db.Select(&rows, `
		SELECT p.id, p.title, p.difficulty,
			EXISTS(
				SELECT 1 FROM playlist_problems pp
				WHERE pp.playlist_id = ? AND pp.problem_id = p.id
			) AS in_playlist
		FROM problems p
		ORDER BY p.title
	`, playlistID)
	if err != nil {
		return nil, nil, err
	}
	problems := make([]SimpleProblem, len(rows))
	checked := make([]bool, len(rows))
	for i, r := range rows {
		problems[i] = SimpleProblem{ID: r.ID, Title: r.Title, Difficulty: r.Difficulty}
		checked[i] = r.InPlaylist
	}
	return problems, checked, nil
}

// GetPlaylistByName returns a playlist by name, or sql.ErrNoRows if not found.
func GetPlaylistByName(db *sqlx.DB, name string) (Playlist, error) {
	var p Playlist
	err := db.Get(&p, `
		SELECT p.id, p.name, p.created_at, COUNT(pp.problem_id) AS problem_count
		FROM playlists p
		LEFT JOIN playlist_problems pp ON pp.playlist_id = p.id
		WHERE p.name = ?
		GROUP BY p.id`, name)
	if err != nil {
		if err == sql.ErrNoRows {
			return Playlist{}, sql.ErrNoRows
		}
		return Playlist{}, err
	}
	return p, nil
}
