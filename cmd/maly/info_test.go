package main

import (
	"os"
	"strings"
	"testing"

	"maly/internal/config"
	"maly/internal/version"
)

// TestLibraryStatsNoDB: sin base de datos, libraryStats debe reportar ok=false
// sin fabricar la base — un diagnóstico que crea la DB vacía y luego reporta 0
// pistas se estaría diagnosticando a sí mismo (doctor.go, info.go).
func TestLibraryStatsNoDB(t *testing.T) {
	xdgSandbox(t)

	tracks, playlists, ok := libraryStats()
	if ok {
		t.Fatalf("libraryStats() debía reportar ok=false sin DB, dio tracks=%d playlists=%d", tracks, playlists)
	}
	if tracks != 0 || playlists != 0 {
		t.Fatalf("sin DB los contadores deben quedar en cero, dio tracks=%d playlists=%d", tracks, playlists)
	}
	if _, err := os.Stat(config.DBPath()); err == nil {
		t.Fatal("libraryStats() creó la base de datos: debía abrir por library.Open, que no crea, y no por library.Create")
	}
}

// TestRunInfoChannel: la fila de canal en `maly info` refleja
// version.Packaged() — hecho de la instalación, no veredicto.
func TestRunInfoChannel(t *testing.T) {
	xdgSandbox(t)

	old := version.Channel
	defer func() { version.Channel = old }()

	version.Channel = ""
	out := captureStdout(t, func() {
		if err := runInfo(nil); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "manual") {
		t.Errorf("canal manual: la salida de info no lo menciona:\n%s", out)
	}

	version.Channel = "pacman"
	out = captureStdout(t, func() {
		if err := runInfo(nil); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "package manager") {
		t.Errorf("canal empaquetado: la salida de info no lo menciona:\n%s", out)
	}
}
