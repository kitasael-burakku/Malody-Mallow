package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"maly/internal/config"
	"maly/internal/i18n"
)

// TestDoctorLabelsTraducidas cubre el hallazgo D10.2 de la auditoría: las
// etiquetas de fila de checkService/checkMusicDir/checkLibrary/checkUpdate
// estaban hardcodeadas en inglés mientras el detalle (y las de `maly info`,
// el comando hermano) sí pasaban por i18n — con el idioma en español,
// "service"/"library"/"update" se veían en inglés pegados a un detalle en
// español. music_dir se queda literal a propósito: es la clave TOML real,
// mismo criterio que config.music_dir en config_cmd.go.
func TestDoctorLabelsTraducidas(t *testing.T) {
	xdgSandbox(t)
	old := i18n.Code()
	i18n.Set("es")
	defer i18n.Set(old)

	if lbl := checkService().label; lbl != "servicio" {
		t.Errorf("checkService().label = %q, quería %q", lbl, "servicio")
	}
	if lbl := checkLibrary().label; lbl != "biblioteca" {
		t.Errorf("checkLibrary().label = %q, quería %q", lbl, "biblioteca")
	}
	if lbl := checkUpdate().label; lbl != "actualización" {
		t.Errorf("checkUpdate().label = %q, quería %q", lbl, "actualización")
	}
	cfg, _ := config.Load()
	if lbl := checkMusicDir(cfg).label; lbl != "music_dir" {
		t.Errorf("checkMusicDir().label = %q, quería %q (literal, es la clave TOML)", lbl, "music_dir")
	}
}

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

// TestCheckKeys: sin colisión (los defaults de fábrica) es lvlOK; con dos
// acciones en la misma tecla es lvlWarn —nunca lvlFail, el config sigue
// siendo válido— y NO cambia el código de salida de `maly doctor` (solo
// fails lo hace, ver runDoctor). El detalle debe nombrar la tecla y las dos
// acciones en conflicto, para que el aviso apunte a la causa.
func TestCheckKeys(t *testing.T) {
	cfg := config.Default()
	if c := checkKeys(cfg); c.lvl != lvlOK {
		t.Fatalf("defaults sin colisión: checkKeys() = %v, quería lvlOK: %s", c.lvl, c.detail)
	}

	cfg.Keys["prev"] = cfg.Keys["next"] // fuerza una colisión real
	c := checkKeys(cfg)
	if c.lvl != lvlWarn {
		t.Fatalf("con colisión: checkKeys() = %v, quería lvlWarn", c.lvl)
	}
	if len(c.cont) != 1 || !strings.Contains(c.cont[0], "next") || !strings.Contains(c.cont[0], "prev") {
		t.Fatalf("el detalle no nombra las acciones en conflicto: %+v", c.cont)
	}
}
