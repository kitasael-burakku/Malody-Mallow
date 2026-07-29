package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"maly/internal/config"
)

// TestCheckServiceNoDaemonNoLock: checkService NO debe tocar el flock del
// demonio (invariante documentado en doctor.go y en la auditoría 2026-07-29:
// un intento no bloqueante que TUVIERA éxito lo retendría un instante, y un
// `maly daemon` arrancando en esa ventana moriría con ErrAlreadyRunning).
// Sin socket, debe caer al camino "no disponible" rápido, sin agotar el
// timeout de 2 s de serviceVersion contra un socket que ni existe, y sin
// dejar ningún maly.lock en el runtime dir.
func TestCheckServiceNoDaemonNoLock(t *testing.T) {
	xdgSandbox(t)

	start := time.Now()
	c := checkService()
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("checkService() tardó %v sin demonio: no debería acercarse al timeout de 2s de un Dial a un socket inexistente", elapsed)
	}
	if c.lvl != lvlInfo {
		t.Fatalf("sin demonio, checkService() debía ser lvlInfo (no un problema), fue %v: %s", c.lvl, c.detail)
	}

	lockPath := filepath.Join(config.RuntimeDir(), "maly.lock")
	if _, err := os.Stat(lockPath); err == nil {
		t.Fatal("checkService() dejó un maly.lock: no debería tocar el flock del demonio")
	}
}

// TestCheckLibraryNoDB: checkLibrary (sobre libraryStats/openLibraryIfExists)
// no debe fabricar la base de datos para poder diagnosticarla.
func TestCheckLibraryNoDB(t *testing.T) {
	xdgSandbox(t)

	c := checkLibrary()
	if c.lvl != lvlWarn {
		t.Fatalf("sin biblioteca, checkLibrary() debía ser lvlWarn, fue %v: %s", c.lvl, c.detail)
	}
	if _, err := os.Stat(config.DBPath()); err == nil {
		t.Fatal("checkLibrary() creó la base de datos")
	}
}
