package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"maly/internal/config"
	"maly/internal/i18n"
	"maly/internal/library"
)

// xdgSandboxTUI aísla XDG_CONFIG_HOME/XDG_DATA_HOME/XDG_RUNTIME_DIR — mismo
// patrón que xdgSandbox en cmd/maly/complete_test.go. Antes de este archivo,
// console_test.go solo aislaba XDG_DATA_HOME (t.Setenv suelto en un par de
// tests), así que los tests de conInfo/conDoctor/conConfig leían el
// config.toml y el runtime dir REALES del usuario que corre `go test`. Sin
// esto, console_diag.go (348 líneas, reimplementa info/doctor/config para la
// consola) queda además como la mayor pieza duplicada del proyecto SIN
// ningún test — es exactamente donde un drift con cmd/maly/doctor.go
// pasaría inadvertido.
func xdgSandboxTUI(t *testing.T) {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "cfg"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmp, "data"))
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(tmp, "rt"))
}

// TestOpenLibIfExistsDoesNotCreate: mismo invariante que
// cmd/maly.openLibraryIfExists — library.Open crearía la base, y un
// diagnóstico que fabrica lo que diagnostica no sirve de nada.
func TestOpenLibIfExistsDoesNotCreate(t *testing.T) {
	xdgSandboxTUI(t)

	lib, ok := openLibIfExists()
	if ok || lib != nil {
		t.Fatalf("openLibIfExists() debía devolver ok=false, lib=nil; dio ok=%v lib=%v", ok, lib)
	}
	if _, err := os.Stat(config.DBPath()); err == nil {
		t.Fatal("openLibIfExists() dejó creada la base de datos")
	}
}

// TestLibStatsTUINoDB: espejo de TestLibraryStatsNoDB (cmd/maly/info_test.go).
func TestLibStatsTUINoDB(t *testing.T) {
	xdgSandboxTUI(t)

	tracks, playlists, ok := libStatsTUI()
	if ok {
		t.Fatalf("libStatsTUI() debía reportar ok=false sin DB, dio tracks=%d playlists=%d", tracks, playlists)
	}
	if tracks != 0 || playlists != 0 {
		t.Fatalf("sin DB los contadores deben quedar en cero, dio tracks=%d playlists=%d", tracks, playlists)
	}
	if _, err := os.Stat(config.DBPath()); err == nil {
		t.Fatal("libStatsTUI() creó la base de datos: debía abrir por openLibIfExists, no por library.Open")
	}
}

// TestLibStatsTUIConDB: con la base ya creada, sí reporta lo que hay dentro.
func TestLibStatsTUIConDB(t *testing.T) {
	xdgSandboxTUI(t)
	if err := os.MkdirAll(filepath.Dir(config.DBPath()), 0o700); err != nil {
		t.Fatal(err)
	}
	lib, err := library.Open(config.DBPath())
	if err != nil {
		t.Fatal(err)
	}
	if err := lib.CreatePlaylist("mix"); err != nil {
		t.Fatal(err)
	}
	lib.Close()

	tracks, playlists, ok := libStatsTUI()
	if !ok {
		t.Fatal("libStatsTUI() debía reportar ok=true con la DB ya creada")
	}
	if tracks != 0 || playlists != 1 {
		t.Fatalf("tracks=%d playlists=%d, quería 0 y 1", tracks, playlists)
	}
}

// TestDiagServiceVersionNoDaemonNoLock: mismo invariante que
// TestCheckServiceNoDaemonNoLock (cmd/maly/doctor_test.go) — sin demonio,
// diagServiceVersion debe resolver rápido (no acercarse al timeout de 2s de
// un Dial contra un socket inexistente) y NO debe dejar ningún maly.lock:
// checkService/diagServiceVersion no tocan el flock del demonio, ni siquiera
// para preguntar.
func TestDiagServiceVersionNoDaemonNoLock(t *testing.T) {
	xdgSandboxTUI(t)

	start := time.Now()
	ver, ok := diagServiceVersion(config.SocketPath())
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("diagServiceVersion() tardó %v sin demonio", elapsed)
	}
	if ok {
		t.Fatalf("sin demonio, diagServiceVersion() debía dar ok=false, dio ver=%q", ver)
	}

	lockPath := filepath.Join(config.RuntimeDir(), "maly.lock")
	if _, err := os.Stat(lockPath); err == nil {
		t.Fatal("diagServiceVersion() dejó un maly.lock: no debería tocar el flock del demonio")
	}
}

// TestConInfo: sin demonio ni biblioteca, conInfo debe resolver en un conMsg
// que mencione la versión y que la biblioteca no existe — nunca fabricarla
// (mismo invariante que TestLibStatsTUINoDB, ejercitado a través del comando
// completo tal como lo invoca la consola).
func TestConInfo(t *testing.T) {
	xdgSandboxTUI(t)
	m := newConModel()

	msg, ok := m.conInfo()().(conMsg)
	if !ok {
		t.Fatalf("conInfo() no devolvió conMsg")
	}
	joined := strings.Join(msg.lines, "\n")
	if !strings.Contains(joined, config.DBPath()) {
		t.Errorf("conInfo() no menciona la ruta de la base: %q", joined)
	}
	if _, err := os.Stat(config.DBPath()); err == nil {
		t.Fatal("conInfo() creó la base de datos")
	}
}

// TestConDoctor: sin demonio, mpv real de la máquina de test presente en el
// PATH y sin colisión de teclas, el chequeo de teclas debe salir "ok" — y con
// una colisión inyectada en el config real del sandbox, debe avisar
// nombrando la tecla y las acciones (mismo chequeo que TestCheckKeys en
// cmd/maly/doctor_test.go, pero ejercitado a través del comando de consola
// completo — es la red que impide que este espejo se desincronice del de la
// CLI sin que ningún test lo note).
func TestConDoctor(t *testing.T) {
	xdgSandboxTUI(t)
	// i18n es estado global del PROCESO (no se deriva de cfg.Language): sin
	// fijarlo, este test correría en el idioma que haya dejado el test
	// anterior. Mismo patrón que TestDoctorLabelsTraducidas en
	// cmd/maly/doctor_test.go.
	old := i18n.Code()
	i18n.Set("es")
	defer i18n.Set(old)

	if err := os.MkdirAll(config.ConfigDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	toml := "music_dir = \"/tmp\"\nlanguage = \"es\"\n\n[keys]\nprev = \"n\"\n"
	if err := os.WriteFile(config.ConfigPath(), []byte(toml), 0o600); err != nil {
		t.Fatal(err)
	}

	m := newConModel()
	msg, ok := m.conDoctor()().(conMsg)
	if !ok {
		t.Fatalf("conDoctor() no devolvió conMsg")
	}
	joined := strings.Join(msg.lines, "\n")
	if !strings.Contains(joined, "warn") || !strings.Contains(joined, "teclas") {
		t.Errorf("conDoctor() no reportó la colisión de teclas: %q", joined)
	}
	if !strings.Contains(joined, "next") || !strings.Contains(joined, "prev") {
		t.Errorf("conDoctor() no nombra las acciones en conflicto: %q", joined)
	}
}

// TestConConfig: sin demonio, conConfig debe devolver la configuración
// efectiva (defaults ← preset ← [keys] del usuario) sin tocar el socket ni
// la DB — y sin fabricar el config.toml si no existía.
func TestConConfig(t *testing.T) {
	xdgSandboxTUI(t)
	m := newConModel()

	msg, ok := m.conConfig()().(conMsg)
	if !ok {
		t.Fatalf("conConfig() no devolvió conMsg")
	}
	joined := strings.Join(msg.lines, "\n")
	if !strings.Contains(joined, "next") {
		t.Errorf("conConfig() no listó las teclas resueltas (esperaba \"next\" de DefaultKeys): %q", joined)
	}
	// Theme.Error (UX-N3) se agregó a cmd/maly/config_cmd.go pero no acá al
	// mismo tiempo — gap de paridad consola↔CLI descubierto en la revisión
	// posterior, misma clase que ya mordió al proyecto con `remove <pos>`.
	// La sola presencia del valor no basta: Visualizer.ColorHigh comparte
	// el mismo hex por defecto (#f38ba8) — hace falta la fila con SU label.
	wantRow := "error: " + config.Default().Theme.Error
	if !strings.Contains(joined, wantRow) {
		t.Errorf("conConfig() no listó la fila de Theme.Error (%q): %q", wantRow, joined)
	}
}
