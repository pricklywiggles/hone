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
