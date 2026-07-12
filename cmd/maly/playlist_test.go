package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"maly/internal/config"
	"maly/internal/library"
)

// TestPlaylistExportNoClobber: exportar sobre un archivo existente sin
// terminal (go test corre sin tty) debe fallar con aviso y dejar el archivo
// intacto, nunca pisarlo en silencio.
func TestPlaylistExportNoClobber(t *testing.T) {
	xdgSandbox(t)

	music := t.TempDir()
	if err := os.WriteFile(filepath.Join(music, "pista.mp3"), []byte("no es audio"), 0o644); err != nil {
		t.Fatal(err)
	}
	lib, err := library.Open(config.DBPath())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lib.Scan(music); err != nil {
		t.Fatal(err)
	}
	all, err := lib.All()
	if err != nil || len(all) != 1 {
		t.Fatalf("All: %d pistas, %v", len(all), err)
	}
	if err := lib.CreatePlaylist("mix"); err != nil {
		t.Fatal(err)
	}
	if err := lib.AddToPlaylist("mix", []int64{all[0].ID}); err != nil {
		t.Fatal(err)
	}
	lib.Close()

	out := filepath.Join(t.TempDir(), "salida.m3u")
	if err := os.WriteFile(out, []byte("contenido previo"), 0o644); err != nil {
		t.Fatal(err)
	}
	err = runPlaylist([]string{"export", "mix", out})
	if err == nil || !strings.Contains(err.Error(), out) {
		t.Fatalf("export sobre archivo existente sin tty: err = %v", err)
	}
	data, _ := os.ReadFile(out)
	if string(data) != "contenido previo" {
		t.Fatalf("el archivo existente fue pisado: %q", data)
	}

	// A un destino nuevo exporta normal.
	fresh := filepath.Join(t.TempDir(), "nueva.m3u")
	if err := runPlaylist([]string{"export", "mix", fresh}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Fatal(err)
	}
}
