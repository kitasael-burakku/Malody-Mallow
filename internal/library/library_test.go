package library

import (
	"path/filepath"
	"testing"
)

func TestFold(t *testing.T) {
	cases := map[string]string{
		"Proporción Áurea": "proporcion aurea",
		"COLCHÓN Vacío":    "colchon vacio",
		"ya-normal":        "ya-normal",
	}
	for in, want := range cases {
		if got := Fold(in); got != want {
			t.Errorf("Fold(%q) = %q, quería %q", in, got, want)
		}
	}
}

func TestPlaylistsRoundTrip(t *testing.T) {
	lib, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer lib.Close()

	if err := lib.CreatePlaylist("mix"); err != nil {
		t.Fatal(err)
	}
	if err := lib.CreatePlaylist("mix"); err == nil {
		t.Fatal("crear playlist duplicada debe fallar")
	}
	lists, err := lib.Playlists()
	if err != nil || len(lists) != 1 || lists[0].Name != "mix" {
		t.Fatalf("Playlists = %v, %v", lists, err)
	}
	if err := lib.DeletePlaylist("mix"); err != nil {
		t.Fatal(err)
	}
	if err := lib.DeletePlaylist("mix"); err == nil {
		t.Fatal("borrar playlist inexistente debe fallar")
	}
}
