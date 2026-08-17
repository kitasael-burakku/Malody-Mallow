package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"maly/internal/config"
	"maly/internal/library"
)

// TestShouldDeletePlaylist cubre el hallazgo C15 de la auditoría P2:
// `playlist delete` borraba sin confirmar, postura de riesgo invertida
// frente a `playlist export`, que sí confirma antes de pisar un .m3u
// regenerable — una playlist armada a mano no tiene deshacer.
func TestShouldDeletePlaylist(t *testing.T) {
	cases := []struct {
		nombre         string
		tty, confirmed bool
		want           bool
	}{
		{"sin tty procede aunque no haya confirmación", false, false, true},
		{"sin tty procede si además confirmó", false, true, true},
		{"con tty y confirmado procede", true, true, true},
		{"con tty sin confirmar NO procede", true, false, false},
	}
	for _, c := range cases {
		t.Run(c.nombre, func(t *testing.T) {
			if got := shouldDeletePlaylist(c.tty, c.confirmed); got != c.want {
				t.Errorf("shouldDeletePlaylist(%v, %v) = %v, quería %v", c.tty, c.confirmed, got, c.want)
			}
		})
	}
}

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
	if _, err := lib.Scan(music, nil); err != nil {
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

// TestPlaylistAddNoExisteSugiereCreate cubre el hallazgo C26: agregar a una
// playlist inexistente decía solo "no existe", sin el remedio — hay que
// crearla antes. La detección es por TIPO (library.ErrPlaylistNotFound) y no
// por el texto del error, que sale de i18n y cambia con el idioma.
func TestPlaylistAddNoExisteSugiereCreate(t *testing.T) {
	xdgSandbox(t)

	// Una pista indexada, para llegar al AddToPlaylist (sin resultados el
	// comando corta antes con otro error).
	lib, err := openLibrary()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	track := filepath.Join(dir, "Luna.mp3")
	if err := os.WriteFile(track, []byte("mp3 falso"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := lib.Scan(dir, nil); err != nil {
		t.Fatal(err)
	}
	lib.Close()

	err = runPlaylist([]string{"add", "favs", "Luna"})
	if err == nil {
		t.Fatal("agregar a una playlist inexistente debe fallar")
	}
	if !strings.Contains(err.Error(), "playlist create") {
		t.Errorf("el error debía sugerir cómo crearla, dio: %v", err)
	}
	// Una sola línea: el espejo de la consola lo pinta dentro de un panel.
	if strings.Contains(err.Error(), "\n") {
		t.Errorf("el error debe caber en una línea, dio: %q", err.Error())
	}
}
