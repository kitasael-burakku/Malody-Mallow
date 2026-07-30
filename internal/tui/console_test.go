package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"maly/internal/config"
	"maly/internal/i18n"
)

// newConModel arma el modelo mínimo que la consola necesita (estilos y
// teclas); sin demonio: solo se ejercitan parsing y operaciones de DB.
func newConModel() *Model {
	return &Model{st: newStyles(config.Theme{}), keys: map[string]string{}}
}

func TestExecConsoleUnknown(t *testing.T) {
	m := newConModel()
	_, cmd := m.execConsole("bogus")
	if cmd != nil {
		t.Fatalf("comando desconocido no debe devolver tea.Cmd")
	}
	if len(m.conLines) == 0 || !strings.Contains(m.conLines[len(m.conLines)-1], "bogus") {
		t.Fatalf("esperaba error mencionando el comando; salida: %q", m.conLines)
	}
}

// TestConsoleUsageErrors cubre las líneas que deben fallar en el parsing sin
// llegar a tocar demonio ni DB (tea.Cmd nulo + mensaje en la consola).
func TestConsoleUsageErrors(t *testing.T) {
	lines := []string{
		"playlist",
		"playlist show",
		"playlist create",
		"playlist delete",
		"playlist add favs",
		"playlist remove favs",
		"playlist remove favs cero",
		"playlist export",
		"playlist import",
		"playlist play",
		"playlist bogus",
		"search",
		"get",
		"lang xx",
		"controls bogus",
		"logo zzz",                              // una sola parada: pide 2-8
		"logo #ff0000 zzz",                      // parada que no es hex
		"logo " + strings.Repeat("#123456 ", 9), // demasiadas paradas
	}
	for _, line := range lines {
		m := newConModel()
		before := len(m.conLines)
		_, cmd := m.execConsole(line)
		if cmd != nil {
			t.Errorf("%q: no debe devolver tea.Cmd", line)
		}
		if len(m.conLines) == before {
			t.Errorf("%q: esperaba un mensaje de error en la consola", line)
		}
	}
}

// TestConsolePlaylistRoundTrip ejercita create → list → show → delete contra
// una DB temporal, verificando que las mutaciones piden recargar el árbol.
func TestConsolePlaylistRoundTrip(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	if err := os.MkdirAll(filepath.Dir(config.DBPath()), 0o700); err != nil {
		t.Fatal(err)
	}
	m := newConModel()

	run := func(line string) conMsg {
		t.Helper()
		_, cmd := m.execConsole(line)
		if cmd == nil {
			t.Fatalf("%q: esperaba un tea.Cmd", line)
		}
		msg, ok := cmd().(conMsg)
		if !ok {
			t.Fatalf("%q: esperaba conMsg", line)
		}
		return msg
	}

	if msg := run("playlist create favs"); !msg.reload {
		t.Errorf("create debe pedir recarga del árbol")
	} else if len(msg.lines) == 0 || !strings.Contains(msg.lines[0], i18n.Tf("pl.created", "favs")) {
		t.Errorf("create: salida inesperada %q", msg.lines)
	}

	if msg := run("playlist list"); msg.reload {
		t.Errorf("list no debe pedir recarga")
	} else if len(msg.lines) != 1 || !strings.Contains(msg.lines[0], "favs  (0)") {
		t.Errorf("list: salida inesperada %q", msg.lines)
	}

	if msg := run("playlist show favs"); len(msg.lines) != 1 || !strings.Contains(msg.lines[0], i18n.Tf("d.pl_empty", "favs")) {
		t.Errorf("show vacía: salida inesperada %q", msg.lines)
	}

	if msg := run("playlist delete favs"); !msg.reload {
		t.Errorf("delete debe pedir recarga del árbol")
	} else if len(msg.lines) == 0 || !strings.Contains(msg.lines[0], i18n.Tf("pl.deleted", "favs")) {
		t.Errorf("delete: salida inesperada %q", msg.lines)
	}
}

// TestConsoleControlsList lista los presets marcando el activo, sin tea.Cmd.
func TestConsoleControlsList(t *testing.T) {
	m := newConModel()
	_, cmd := m.execConsole("controls")
	if cmd != nil {
		t.Fatalf("controls sin args no debe devolver tea.Cmd")
	}
	joined := strings.Join(m.conLines, "\n")
	for _, name := range config.PresetNames() {
		if !strings.Contains(joined, name) {
			t.Errorf("falta el preset %q en la lista:\n%s", name, joined)
		}
	}
}

// TestConsoleThemeReload: complemento de `maly theme sync` (integración
// Matugen) — relee config.toml y aplica el tema en vivo (m.st, m.logo.ramp)
// sin reiniciar la TUI.
func TestConsoleThemeReload(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	if err := os.MkdirAll(config.ConfigDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	body := "[theme]\naccent = \"#123456\"\nlogo = [\"#111111\", \"#222222\"]\n"
	if err := os.WriteFile(config.ConfigPath(), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	m := newConModel()

	if _, cmd := m.execConsole("theme"); cmd != nil {
		t.Fatal("theme sin subcomando no debe devolver tea.Cmd")
	}
	if len(m.conLines) == 0 || !strings.Contains(m.conLines[len(m.conLines)-1], i18n.T("con.theme_usage")) {
		t.Errorf("theme sin subcomando debía avisar el uso, salida: %q", m.conLines)
	}

	if m.logo.ramp != nil {
		t.Fatal("precondición: el modelo de prueba no debía traer un ramp calculado")
	}
	if _, cmd := m.execConsole("theme reload"); cmd != nil {
		t.Fatal("theme reload no debe devolver tea.Cmd")
	}
	if m.cfg.Theme.Accent != "#123456" {
		t.Fatalf("theme reload no releyó config.toml: accent = %q", m.cfg.Theme.Accent)
	}
	// logoRamp interpola en logoSteps pasos fijos (no en len(stops)): lo que
	// prueba que se recalculó es que dejó de ser nil.
	if len(m.logo.ramp) != logoSteps {
		t.Fatalf("theme reload no recalculó el ramp del logo: %d entradas, quería %d", len(m.logo.ramp), logoSteps)
	}
}

// TestThemeReloadMsg: el case de Update() que dispara la señal SIGUSR1 (ver
// Run en tui.go) hace exactamente lo mismo que `theme reload` en la
// consola, sin pasar por execConsole.
func TestThemeReloadMsg(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	if err := os.MkdirAll(config.ConfigDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	body := "[theme]\naccent = \"#654321\"\nlogo = [\"#aaaaaa\", \"#bbbbbb\"]\n"
	if err := os.WriteFile(config.ConfigPath(), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	m := newConModel()
	if m.logo.ramp != nil {
		t.Fatal("precondición: el modelo de prueba no debía traer un ramp calculado")
	}

	newModel, cmd := m.Update(themeReloadMsg{})
	if cmd != nil {
		t.Fatal("themeReloadMsg no debe devolver tea.Cmd")
	}
	got := newModel.(*Model)
	if got.cfg.Theme.Accent != "#654321" {
		t.Fatalf("themeReloadMsg no releyó config.toml: accent = %q", got.cfg.Theme.Accent)
	}
	if len(got.logo.ramp) != logoSteps {
		t.Fatalf("themeReloadMsg no recalculó el ramp del logo: %d entradas, quería %d", len(got.logo.ramp), logoSteps)
	}
}

// TestConsoleSelect cierra la consola y abre el picker de canciones.
func TestConsoleSelect(t *testing.T) {
	m := newConModel()
	m.tree = buildTree(nil, nil)
	m.consoleOpen = true
	_, cmd := m.execConsole("select")
	if m.consoleOpen {
		t.Errorf("select debe cerrar la consola")
	}
	if !m.songsOpen || cmd == nil {
		t.Errorf("select debe abrir el picker de canciones")
	}
}
