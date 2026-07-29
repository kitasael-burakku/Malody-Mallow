package main

import (
	"os"
	"testing"

	"maly/internal/config"
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
		t.Fatal("libraryStats() creó la base de datos: debía abrir por openLibraryIfExists, no por Open")
	}
}

// TestOpenLibraryIfExistsDoesNotCreate: mismo invariante, sobre la función que
// libraryStats usa por dentro. library.Open crearía la base; openLibraryIfExists
// debe negarse a abrir lo que no existe.
func TestOpenLibraryIfExistsDoesNotCreate(t *testing.T) {
	xdgSandbox(t)

	lib, ok := openLibraryIfExists()
	if ok || lib != nil {
		t.Fatalf("openLibraryIfExists() debía devolver ok=false, lib=nil; dio ok=%v lib=%v", ok, lib)
	}
	if _, err := os.Stat(config.DBPath()); err == nil {
		t.Fatal("openLibraryIfExists() dejó creada la base de datos")
	}
}

// TestOpenLibraryIfExistsOpensReal: con la base ya creada (por openLibrary,
// el camino que SÍ la crea), openLibraryIfExists debe abrirla sin problema.
func TestOpenLibraryIfExistsOpensReal(t *testing.T) {
	xdgSandbox(t)

	lib, err := openLibrary()
	if err != nil {
		t.Fatalf("openLibrary() (setup): %v", err)
	}
	lib.Close()

	lib, ok := openLibraryIfExists()
	if !ok || lib == nil {
		t.Fatal("openLibraryIfExists() debía abrir la base ya existente")
	}
	lib.Close()
}
