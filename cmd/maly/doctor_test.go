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

// TestCheckLinkedDirs cubre A-13 (mitad barata): filepath.WalkDir no sigue
// enlaces simbólicos, así que un directorio enlazado bajo music_dir se salta
// ENTERO y el escaneo dice "0 nuevas" sin distinguir "no hay música" de "hay
// música detrás de un enlace que no miro". Organizar la biblioteca así
// —~/Music/Discos → /mnt/nas/flac— es un patrón habitual en Linux.
func TestCheckLinkedDirs(t *testing.T) {
	tmp := t.TempDir()
	music := filepath.Join(tmp, "music")
	externo := filepath.Join(tmp, "externo", "album")
	if err := os.MkdirAll(externo, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(music, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{MusicDir: music}

	// Sin enlaces: el chequeo no aparece. Una fila que sale siempre es ruido.
	if got := checkLinkedDirs(cfg); len(got) != 0 {
		t.Fatalf("sin enlaces no debía reportar nada, dio %+v", got)
	}

	// Un ARCHIVO enlazado tampoco: ese sí se indexa, no hay nada que avisar.
	if err := os.WriteFile(filepath.Join(tmp, "externo", "suelta.mp3"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(tmp, "externo", "suelta.mp3"), filepath.Join(music, "suelta.mp3")); err != nil {
		t.Fatal(err)
	}
	if got := checkLinkedDirs(cfg); len(got) != 0 {
		t.Fatalf("un archivo enlazado se indexa igual: no debía avisar, dio %+v", got)
	}

	// Un DIRECTORIO enlazado sí.
	if err := os.Symlink(externo, filepath.Join(music, "album-enlazado")); err != nil {
		t.Fatal(err)
	}
	got := checkLinkedDirs(cfg)
	if len(got) != 1 {
		t.Fatalf("con un directorio enlazado esperaba un chequeo, dio %+v", got)
	}
	if got[0].lvl != lvlInfo {
		t.Errorf("debía ser info y no warn/fail: no seguir enlaces es una decisión, no una falla (lvl=%v)", got[0].lvl)
	}
	if len(got[0].cont) != 1 {
		t.Fatalf("esperaba una línea de remedio por enlace, dio %v", got[0].cont)
	}
	// El remedio tiene que apuntar al DESTINO. Escanear el enlace NO sirve:
	// WalkDir hace lstat de su propia raíz, así que ni siquiera lo recorre
	// —comprobado en vivo— y sugerirlo mandaría al usuario a un callejón.
	if !strings.Contains(got[0].cont[0], externo) {
		t.Errorf("el remedio debía apuntar al destino %q, dice %q", externo, got[0].cont[0])
	}
	if strings.Contains(got[0].cont[0], filepath.Join(music, "album-enlazado")) {
		t.Errorf("el remedio no debe sugerir escanear el ENLACE: %q", got[0].cont[0])
	}
}

// TestScanNoSigueDirectorioEnlazado fija el comportamiento que el chequeo
// anterior existe para explicar, y de paso deja escrito que escanear el
// ENLACE tampoco funciona (es lo que hace que el remedio bueno sea el
// destino). Si algún día se implementa la mitad de Phase 3 —seguir enlaces de
// primer nivel— este test es el que hay que actualizar a propósito.
func TestScanNoSigueDirectorioEnlazado(t *testing.T) {
	xdgSandbox(t)
	tmp := t.TempDir()
	music := filepath.Join(tmp, "music")
	externo := filepath.Join(tmp, "externo", "album")
	if err := os.MkdirAll(externo, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(music, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(externo, "uno.mp3"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(externo, filepath.Join(music, "enlazado")); err != nil {
		t.Fatal(err)
	}

	lib, err := openLibrary()
	if err != nil {
		t.Fatal(err)
	}
	defer lib.Close()

	res, err := lib.Scan(music, nil)
	if err != nil || res.Added != 0 {
		t.Fatalf("escanear music_dir no debía indexar lo del enlace: %+v, %v", res, err)
	}
	// Escanear el enlace tampoco: WalkDir hace lstat de su raíz.
	res, err = lib.Scan(filepath.Join(music, "enlazado"), nil)
	if err != nil || res.Added != 0 {
		t.Fatalf("escanear el enlace tampoco indexa: %+v, %v", res, err)
	}
	// El destino real sí.
	res, err = lib.Scan(externo, nil)
	if err != nil || res.Added != 1 {
		t.Fatalf("escanear el destino real debía indexar 1: %+v, %v", res, err)
	}
}
