package library

import (
	"fmt"
	"os"
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

// fakeMusicDir crea n archivos .mp3 dummy repartidos en subcarpetas; los tags
// no se pueden leer y el título sale del nombre, suficiente para indexar.
func fakeMusicDir(t *testing.T, n int) string {
	t.Helper()
	dir := t.TempDir()
	for i := 0; i < n; i++ {
		sub := filepath.Join(dir, fmt.Sprintf("album%02d", i%20))
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		f := filepath.Join(sub, fmt.Sprintf("pista%04d.mp3", i))
		if err := os.WriteFile(f, []byte("no es audio"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// TestScanConcurrentSearch verifica (bajo -race) que Scan puede correr en
// paralelo con lecturas de la biblioteca, como hace el demonio desde que
// scan no toma su mutex.
func TestScanConcurrentSearch(t *testing.T) {
	lib, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer lib.Close()

	const n = 300
	dir := fakeMusicDir(t, n)

	type scanOut struct {
		res ScanResult
		err error
	}
	done := make(chan scanOut, 1)
	go func() {
		res, err := lib.Scan(dir)
		done <- scanOut{res, err}
	}()

	// Lecturas concurrentes hasta que el escaneo termine.
	var out scanOut
loop:
	for {
		select {
		case out = <-done:
			break loop
		default:
			if _, err := lib.Search("pista"); err != nil {
				t.Fatalf("Search durante el escaneo: %v", err)
			}
			if _, err := lib.Count(); err != nil {
				t.Fatalf("Count durante el escaneo: %v", err)
			}
		}
	}
	if out.err != nil {
		t.Fatalf("Scan: %v", out.err)
	}
	if out.res.Added != n {
		t.Fatalf("Added = %d, quería %d (errores: %v)", out.res.Added, n, out.res.Errors)
	}
	if total, _ := lib.Count(); total != n {
		t.Fatalf("Count = %d, quería %d", total, n)
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
