package library

import (
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"maly/internal/i18n"
)

func TestSavePlaylistAtomic(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "library.db")
	lib, err := Create(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer lib.Close()
	if _, err := lib.Scan(fakeMusicDir(t, 2), nil); err != nil {
		t.Fatal(err)
	}
	tracks, err := lib.All()
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{tracks[1].Path, tracks[0].Path, tracks[1].Path, tracks[0].Path}
	check := func(name string, want []string) {
		t.Helper()
		got, err := lib.PlaylistTracks(name)
		if err != nil {
			t.Fatal(err)
		}
		var actual []string
		for _, tr := range got {
			actual = append(actual, tr.Path)
		}
		if !reflect.DeepEqual(actual, want) {
			t.Fatalf("%s: %q, quería %q", name, actual, want)
		}
	}
	if err := lib.CreatePlaylistFromPaths("Mi cola", paths); err != nil {
		t.Fatal(err)
	}
	check("Mi cola", paths)
	for _, tc := range []struct {
		name  string
		paths []string
		key   string
	}{
		{"Mi cola", []string{tracks[0].Path}, "lib.pl_exists"},
		{"vacía", nil, "pl.save_empty"},
		{"sin indexar", []string{tracks[0].Path, "/ajena\x1b[31m.flac"}, "pl.save_unindexed"},
		{"   ", paths, "lib.pl_name"},
	} {
		err := lib.CreatePlaylistFromPaths(tc.name, tc.paths)
		if err == nil {
			t.Fatalf("%q no falló", tc.name)
		}
		if strings.ContainsRune(err.Error(), '\x1b') {
			t.Fatal("escape ajeno sin sanear", err)
		}
		lists, e := lib.Playlists()
		if e != nil || len(lists) != 1 {
			t.Fatalf("escritura parcial: %v, %v", lists, e)
		}
		check("Mi cola", paths)
	}
	// Fallo después de insertar la primera entrada: también revierte el nombre.
	if _, err := lib.db.Exec(`CREATE TRIGGER fail_second BEFORE INSERT ON playlist_tracks WHEN NEW.pos=2 BEGIN SELECT RAISE(ABORT, 'fallo inducido'); END`); err != nil {
		t.Fatal(err)
	}
	if err := lib.CreatePlaylistFromPaths("fallo SQL", paths); err == nil || !strings.Contains(err.Error(), "fallo inducido") {
		t.Fatalf("fallo inesperado: %v", err)
	}
	if _, err := lib.PlaylistTracks("fallo SQL"); !errors.Is(err, ErrPlaylistNotFound) {
		t.Fatalf("quedó el nombre: %v", err)
	}
	if _, err := lib.db.Exec(`DROP TRIGGER fail_second`); err != nil {
		t.Fatal(err)
	}
	// Dos conexiones reales compiten por el mismo nombre; nunca se mezclan.
	other, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	start := make(chan struct{})
	done := make(chan error, 2)
	for _, l := range []*Library{lib, other} {
		go func(l *Library) { <-start; done <- l.CreatePlaylistFromPaths("competencia", paths) }(l)
	}
	close(start)
	e1, e2 := <-done, <-done
	if (e1 == nil) == (e2 == nil) {
		t.Fatalf("debe ganar exactamente uno: %v / %v", e1, e2)
	}
	check("competencia", paths)
	// La ruta manda incluso después de que un escaneo reasigne un ID antiguo.
	if _, err := lib.db.Exec(`DELETE FROM playlist_tracks`); err != nil {
		t.Fatal(err)
	}
	if _, err := lib.db.Exec(`UPDATE tracks SET id = id + 100`); err != nil {
		t.Fatal(err)
	}
	if err := lib.CreatePlaylistFromPaths("IDs nuevos", paths); err != nil {
		t.Fatal(err)
	}
	check("IDs nuevos", paths)
}

func TestSavePlaylistMissingPathRemedy(t *testing.T) {
	old := i18n.Code()
	defer i18n.Set(old)
	for _, lang := range []string{"es", "en"} {
		i18n.Set(lang)
		lib, err := Create(filepath.Join(t.TempDir(), "library.db"))
		if err != nil {
			t.Fatal(err)
		}
		err = lib.CreatePlaylistFromPaths("nueva", []string{"/música/no indexada.flac"})
		lib.Close()
		want := i18n.Tf("pl.save_unindexed", "/música/no indexada.flac")
		if err == nil || fmt.Sprint(err) != want || !strings.Contains(err.Error(), "maly scan") {
			t.Fatalf("%s: %v", lang, err)
		}
	}
}
