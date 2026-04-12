package store

import (
	"database/sql"

	"github.com/jmoiron/sqlx"
)

func ActivePlaylistID(db *sqlx.DB) (*int, error) {
	var id sql.NullInt64
	if err := db.Get(&id, `SELECT active_playlist_id FROM settings WHERE id = 1`); err != nil {
		return nil, err
	}
	if !id.Valid {
		return nil, nil
	}
	v := int(id.Int64)
	return &v, nil
}

func SetActivePlaylist(db *sqlx.DB, id int) error {
	_, err := db.Exec(`UPDATE settings SET active_playlist_id = ?, active_topic_id = NULL WHERE id = 1`, id)
	return err
}

func ClearActivePlaylist(db *sqlx.DB) error {
	_, err := db.Exec(`UPDATE settings SET active_playlist_id = NULL WHERE id = 1`)
	return err
}

func ActiveTopicID(db *sqlx.DB) (*int, error) {
	var id sql.NullInt64
	if err := db.Get(&id, `SELECT active_topic_id FROM settings WHERE id = 1`); err != nil {
		return nil, err
	}
	if !id.Valid {
		return nil, nil
	}
	v := int(id.Int64)
	return &v, nil
}

func SetActiveTopic(db *sqlx.DB, id int) error {
	_, err := db.Exec(`UPDATE settings SET active_topic_id = ?, active_playlist_id = NULL WHERE id = 1`, id)
	return err
}

func ClearActiveTopic(db *sqlx.DB) error {
	_, err := db.Exec(`UPDATE settings SET active_topic_id = NULL WHERE id = 1`)
	return err
}

func ActiveFilter(db *sqlx.DB) (PracticeFilter, error) {
	var row struct {
		PlaylistID sql.NullInt64 `db:"active_playlist_id"`
		TopicID    sql.NullInt64 `db:"active_topic_id"`
	}
	if err := db.Get(&row, `SELECT active_playlist_id, active_topic_id FROM settings WHERE id = 1`); err != nil {
		return PracticeFilter{}, err
	}
	var f PracticeFilter
	if row.PlaylistID.Valid {
		v := int(row.PlaylistID.Int64)
		f.PlaylistID = &v
	}
	if row.TopicID.Valid {
		v := int(row.TopicID.Int64)
		f.TopicID = &v
	}
	return f, nil
}
