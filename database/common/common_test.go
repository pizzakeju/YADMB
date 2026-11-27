package common

import (
	"database/sql"
	"testing"

	"github.com/TheTipo01/YADMB/database"
	"github.com/disgoorg/snowflake/v2"
	_ "modernc.org/sqlite"
)

func newTestCommon(t *testing.T) Common {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ddl := `CREATE TABLE IF NOT EXISTS userPlaylists (
		guild varchar(20) NOT NULL,
		name varchar(100) NOT NULL,
		position integer NOT NULL,
		entry varchar(500) NOT NULL,
		PRIMARY KEY (guild, name, position)
	);`
	if _, err := db.Exec(ddl); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	return NewCommon(db)
}

func TestPlaylistsWorkflow(t *testing.T) {
	c := newTestCommon(t)
	guild := snowflake.ID(123)

	if err := c.CreatePlaylist(guild, "chill"); err != nil {
		t.Fatalf("CreatePlaylist: %v", err)
	}

	if err := c.CreatePlaylist(guild, "chill"); err == nil {
		t.Fatal("expected duplicate playlist name to fail")
	}

	if err := c.AddToPlaylist(guild, "chill", "https://a"); err != nil {
		t.Fatalf("AddToPlaylist: %v", err)
	}
	if err := c.AddToPlaylist(guild, "chill", "https://b"); err != nil {
		t.Fatalf("AddToPlaylist: %v", err)
	}
	if err := c.AddToPlaylist(guild, "chill", "https://a"); err != nil {
		t.Fatalf("AddToPlaylist: %v", err)
	}

	playlists := c.ListPlaylists(guild)
	if len(playlists) != 1 || playlists[0].Name != "chill" || playlists[0].Count != 3 {
		t.Fatalf("unexpected playlists: %+v", playlists)
	}

	entries := c.GetPlaylistEntries(guild, "chill")
	if len(entries) != 3 || entries[0] != "https://a" || entries[1] != "https://b" {
		t.Fatalf("unexpected entries: %v", entries)
	}

	if err := c.RemoveDuplicates(guild, "chill"); err != nil {
		t.Fatalf("RemoveDuplicates: %v", err)
	}
	entries = c.GetPlaylistEntries(guild, "chill")
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries after dedup, got %v", entries)
	}

	if err := c.RemoveFromPlaylist(guild, "chill", 2); err != nil {
		t.Fatalf("RemoveFromPlaylist: %v", err)
	}
	entries = c.GetPlaylistEntries(guild, "chill")
	if len(entries) != 1 || entries[0] != "https://a" {
		t.Fatalf("unexpected entries after remove: %v", entries)
	}

	if err := c.RenamePlaylist(guild, "chill", "focus"); err != nil {
		t.Fatalf("RenamePlaylist: %v", err)
	}
	if len(c.GetPlaylistEntries(guild, "focus")) != 1 || len(c.GetPlaylistEntries(guild, "chill")) != 0 {
		t.Fatal("rename did not move entries")
	}

	if err := c.SavePlaylist(guild, "focus", []string{"https://x", "https://y"}); err != nil {
		t.Fatalf("SavePlaylist: %v", err)
	}
	entries = c.GetPlaylistEntries(guild, "focus")
	if len(entries) != 2 || entries[0] != "https://x" || entries[1] != "https://y" {
		t.Fatalf("unexpected entries after save: %v", entries)
	}

	if err := c.DeletePlaylist(guild, "focus"); err != nil {
		t.Fatalf("DeletePlaylist: %v", err)
	}
	if len(c.ListPlaylists(guild)) != 0 {
		t.Fatal("expected no playlists after delete")
	}
}

func TestPlaylistInfoType(t *testing.T) {
	var _ database.PlaylistInfo
}
