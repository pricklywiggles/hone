package store

import (
	"testing"

	"github.com/pricklywiggles/hone/internal/db"
)

func TestActivePlaylistID_DefaultNil(t *testing.T) {
	d, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	id, err := ActivePlaylistID(d)
	if err != nil {
		t.Fatal(err)
	}
	if id != nil {
		t.Errorf("expected nil, got %d", *id)
	}
}

func TestActiveTopicID_DefaultNil(t *testing.T) {
	d, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	id, err := ActiveTopicID(d)
	if err != nil {
		t.Fatal(err)
	}
	if id != nil {
		t.Errorf("expected nil, got %d", *id)
	}
}

func TestSetActivePlaylist_RoundTrip(t *testing.T) {
	d, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	d.MustExec(`INSERT INTO playlists (name) VALUES ('Week 1')`)

	if err := SetActivePlaylist(d, 1); err != nil {
		t.Fatal(err)
	}
	id, err := ActivePlaylistID(d)
	if err != nil {
		t.Fatal(err)
	}
	if id == nil || *id != 1 {
		t.Errorf("expected 1, got %v", id)
	}
}

func TestSetActivePlaylist_ClearsTopic(t *testing.T) {
	d, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	d.MustExec(`INSERT INTO playlists (name) VALUES ('Week 1')`)
	d.MustExec(`INSERT INTO topics (name) VALUES ('arrays')`)

	if err := SetActiveTopic(d, 1); err != nil {
		t.Fatal(err)
	}
	if err := SetActivePlaylist(d, 1); err != nil {
		t.Fatal(err)
	}

	topicID, err := ActiveTopicID(d)
	if err != nil {
		t.Fatal(err)
	}
	if topicID != nil {
		t.Errorf("expected topic nil after setting playlist, got %d", *topicID)
	}
}

func TestSetActiveTopic_ClearsPlaylist(t *testing.T) {
	d, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	d.MustExec(`INSERT INTO playlists (name) VALUES ('Week 1')`)
	d.MustExec(`INSERT INTO topics (name) VALUES ('arrays')`)

	if err := SetActivePlaylist(d, 1); err != nil {
		t.Fatal(err)
	}
	if err := SetActiveTopic(d, 1); err != nil {
		t.Fatal(err)
	}

	playlistID, err := ActivePlaylistID(d)
	if err != nil {
		t.Fatal(err)
	}
	if playlistID != nil {
		t.Errorf("expected playlist nil after setting topic, got %d", *playlistID)
	}
}

func TestClearActivePlaylist(t *testing.T) {
	d, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	d.MustExec(`INSERT INTO playlists (name) VALUES ('Week 1')`)

	if err := SetActivePlaylist(d, 1); err != nil {
		t.Fatal(err)
	}
	if err := ClearActivePlaylist(d); err != nil {
		t.Fatal(err)
	}

	id, err := ActivePlaylistID(d)
	if err != nil {
		t.Fatal(err)
	}
	if id != nil {
		t.Errorf("expected nil after clear, got %d", *id)
	}
}

func TestClearActiveTopic(t *testing.T) {
	d, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	d.MustExec(`INSERT INTO topics (name) VALUES ('arrays')`)

	if err := SetActiveTopic(d, 1); err != nil {
		t.Fatal(err)
	}
	if err := ClearActiveTopic(d); err != nil {
		t.Fatal(err)
	}

	id, err := ActiveTopicID(d)
	if err != nil {
		t.Fatal(err)
	}
	if id != nil {
		t.Errorf("expected nil after clear, got %d", *id)
	}
}

func TestActiveFilter(t *testing.T) {
	d, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	d.MustExec(`INSERT INTO playlists (name) VALUES ('Week 1')`)

	if err := SetActivePlaylist(d, 1); err != nil {
		t.Fatal(err)
	}

	f, err := ActiveFilter(d)
	if err != nil {
		t.Fatal(err)
	}
	if f.PlaylistID == nil || *f.PlaylistID != 1 {
		t.Errorf("expected PlaylistID=1, got %v", f.PlaylistID)
	}
	if f.TopicID != nil {
		t.Errorf("expected TopicID=nil, got %d", *f.TopicID)
	}
}

func TestFK_OnDeleteSetNull(t *testing.T) {
	d, err := db.OpenMemory()
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()

	d.MustExec(`INSERT INTO playlists (name) VALUES ('Week 1')`)

	if err := SetActivePlaylist(d, 1); err != nil {
		t.Fatal(err)
	}

	d.MustExec(`DELETE FROM playlists WHERE id = 1`)

	id, err := ActivePlaylistID(d)
	if err != nil {
		t.Fatal(err)
	}
	if id != nil {
		t.Errorf("expected nil after playlist deletion (ON DELETE SET NULL), got %d", *id)
	}
}
