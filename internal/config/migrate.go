package config

import (
	"github.com/jmoiron/sqlx"
	"github.com/pricklywiggles/hone/internal/store"
	"github.com/spf13/viper"
)

// MigrateActiveSettingsToDB moves active_playlist_id and active_topic_id from
// config.yaml to the settings table in the database. This is a one-time
// migration for users upgrading from the config-based storage.
func MigrateActiveSettingsToDB(db *sqlx.DB) error {
	playlistID := viper.GetInt("active_playlist_id")
	topicID := viper.GetInt("active_topic_id")

	if playlistID == 0 && topicID == 0 {
		return nil
	}

	if playlistID > 0 {
		var exists bool
		if err := db.Get(&exists, `SELECT COUNT(*) > 0 FROM playlists WHERE id = ?`, playlistID); err != nil {
			return err
		}
		if exists {
			if err := store.SetActivePlaylist(db, playlistID); err != nil {
				return err
			}
		}
	} else if topicID > 0 {
		var exists bool
		if err := db.Get(&exists, `SELECT COUNT(*) > 0 FROM topics WHERE id = ?`, topicID); err != nil {
			return err
		}
		if exists {
			if err := store.SetActiveTopic(db, topicID); err != nil {
				return err
			}
		}
	}

	viper.Set("active_playlist_id", nil)
	viper.Set("active_topic_id", nil)
	_ = viper.WriteConfig()

	return nil
}
