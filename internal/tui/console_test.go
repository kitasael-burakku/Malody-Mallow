package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"maly/internal/config"
	"maly/internal/i18n"
	"maly/internal/version"
)

// newConModel arma el modelo mínimo que la consola necesita (estilos y
// teclas); sin demonio: solo se ejercitan parsing y operaciones de DB.
func newConModel() *Model {
	return &Model{st: newStyles(config.Theme{}), keys: map[string]string{}}
}

// TestConsoleHistorial cubre el hallazgo T27 de la auditoría P2: la consola
// no tenía historial de comandos, así que ↑/↓ no hacían nada.
func TestConsoleHistorial(t *testing.T) {
	m := newConModel()
	m.openConsole()

	m.conInput.SetValue("primero")
	m.handleConsoleKey(tea.KeyMsg{Type: tea.KeyEnter})
	m.conInput.SetValue("segundo")
	m.handleConsoleKey(tea.KeyMsg{Type: tea.KeyEnter})

	m.conInput.SetValue("en progreso")
	m.handleConsoleKey(tea.KeyMsg{Type: tea.KeyUp})
	if got := m.conInput.Value(); got != "segundo" {
		t.Fatalf("primer ↑ debía traer el último comando, dio %q", got)
	}
	m.handleConsoleKey(tea.KeyMsg{Type: tea.KeyUp})
	if got := m.conInput.Value(); got != "primero" {
		t.Fatalf("segundo ↑ debía traer el comando anterior, dio %q", got)
	}
	// Un tercer ↑ no debe ir más allá del primero.
	m.handleConsoleKey(tea.KeyMsg{Type: tea.KeyUp})
	if got := m.conInput.Value(); got != "primero" {
		t.Fatalf("↑ en el tope del historial no debía cambiar, dio %q", got)
	}
	m.handleConsoleKey(tea.KeyMsg{Type: tea.KeyDown})
	if got := m.conInput.Value(); got != "segundo" {
		t.Fatalf("↓ debía volver a \"segundo\", dio %q", got)
	}
	m.handleConsoleKey(tea.KeyMsg{Type: tea.KeyDown})
	if got := m.conInput.Value(); got != "en progreso" {
		t.Fatalf("↓ al final debía restaurar lo que se estaba escribiendo, dio %q", got)
	}
}

// boxLinesFitWithin verifica el ancho real de la CAJA dentro de un render
// que además pasó por lipgloss.Place (consoleView centra el panel en el
// terminal completo, que siempre mide m.width — Place no trunca, así que
// medir contra m.width solo detectaría un desborde que llega a romper la
// terminal entera). El padding de Place son espacios sin estilo a los
// lados, así que recortarlos con strings.TrimSpace recupera la fila real de
// la caja (bordes incluidos) para medirla contra w.
func boxLinesFitWithin(t *testing.T, out string, w int) {
	t.Helper()
	for i, l := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(l)
		if trimmed == "" {
			continue
		}
		if lw := lipgloss.Width(trimmed); lw > w {
			t.Errorf("línea %d de la caja mide %d celdas, más que el ancho declarado (%d): %q", i, lw, w, trimmed)
		}
	}
}

// TestConsoleViewLongInputNoOverflow: el textinput de bubbles nunca recibía
// Width, así que su scroll horizontal quedaba desactivado y View() emitía
// el valor completo — escribir un comando largo (una URL de get, por
// ejemplo) rompía el borde derecho de la Command Palette en vivo, mientras
// se escribe (auditoría de UX post-1.12.0, reportado por el dueño).
func TestConsoleViewLongInputNoOverflow(t *testing.T) {
	m := newConModel()
	m.openConsole()
	m.width, m.height = 80, 24
	m.conInput.SetValue("get " + strings.Repeat("x", 200))

	out := m.consoleView()
	boxLinesFitWithin(t, out, pickerWidth(m.width))
}

// TestConHelpNoOverflow cubre el bug reportado: `?`/`help` dentro de la
// consola mostraba filas (get, playlist) más anchas que la caja, rompiendo
// el borde derecho. El label de "get" vuelve a su forma corta (con.
// help_local ya menciona la forma playlist aparte) y cada fila se clipea
// por columna, así que ninguna — ni siquiera con una descripción tan larga
// como la de playlist — puede desbordar (auditoría de UX post-1.12.0).
func TestConHelpNoOverflow(t *testing.T) {
	m := newConModel()
	m.openConsole()
	m.width, m.height = 80, 24

	m.conHelp()
	out := m.consoleView()
	boxLinesFitWithin(t, out, pickerWidth(m.width))

	var getRow string
	for _, l := range m.conLines {
		if strings.Contains(l, "get <url|q>") {
			getRow = l
			break
		}
	}
	if getRow == "" {
		t.Fatalf("no encontré la fila de get, salida: %q", m.conLines)
	}
	if strings.Contains(getRow, "playlist") {
		t.Errorf("la forma playlist ya no debía estar en la fila de get (vive en con.help_local aparte): %q", getRow)
	}
}

// TestConsoleScroll cubre el hallazgo T28 de la auditoría P2: la salida de
// la consola siempre mostraba la cola de conLines, sin forma de volver a ver
// las primeras líneas de un search/queue largo.
// TestFormatHelpRowClipsPorColumna: con los datos reales de hoy (tras
// revertir el label de get) ninguna fila de conHelp() dispara una
// diferencia visible entre clipear por columna y clipear la fila entera ya
// ensamblada — pero un label sintético más largo que su columna sí la
// dispara: sin clip independiente, padTo no acorta (solo rellena), así que
// la descripción se corre a una columna distinta de la de las demás filas.
func TestFormatHelpRowClipsPorColumna(t *testing.T) {
	st := styles{} // sin tema: Render() no agrega ANSI, así que los índices de byte coinciden con los de celda
	const labelW, descW = 22, 30

	row := formatHelpRow(st, strings.Repeat("x", 40), "descripcion", labelW, descW)
	// El label clipeado a labelW dentro de la fila: sin eso, "descripcion"
	// aparecería mucho más lejos de la columna 2+labelW que en cualquier
	// otra fila (padTo no acorta, solo rellena). Se mide en celdas, no en
	// bytes: el "…" del clip ocupa 1 celda pero 3 bytes en UTF-8.
	idx := strings.Index(row, "descripcion")
	if idx < 0 {
		t.Fatalf("no encontré la descripción en la fila: %q", row)
	}
	if col := lipgloss.Width(row[:idx]); col != 2+labelW {
		t.Errorf("la descripción debía empezar en la columna %d, apareció en %d: %q", 2+labelW, col, row)
	}
	if lipgloss.Width(row) > 2+labelW+descW {
		t.Errorf("la fila mide %d celdas, más que el presupuesto total (%d): %q",
			lipgloss.Width(row), 2+labelW+descW, row)
	}
}

func TestConsoleScroll(t *testing.T) {
	m := newConModel()
	m.openConsole()
	m.width, m.height = 80, 24
	for i := 0; i < 40; i++ {
		m.conPrint(fmt.Sprintf("linea %d", i))
	}

	out := m.consoleView()
	if !strings.Contains(out, "linea 39") {
		t.Fatalf("sin scrollear debía verse la última línea: %q", out)
	}
	if strings.Contains(out, "linea 0 ") {
		t.Fatalf("sin scrollear NO debía verse la primera línea: %q", out)
	}

	m.handleConsoleKey(tea.KeyMsg{Type: tea.KeyPgUp})
	m.handleConsoleKey(tea.KeyMsg{Type: tea.KeyPgUp})
	out = m.consoleView()
	if strings.Contains(out, "linea 39") {
		t.Fatalf("tras pgup debía dejar de verse la última línea: %q", out)
	}

	m.handleConsoleKey(tea.KeyMsg{Type: tea.KeyPgDown})
	m.handleConsoleKey(tea.KeyMsg{Type: tea.KeyPgDown})
	out = m.consoleView()
	if !strings.Contains(out, "linea 39") {
		t.Fatalf("pgdown de vuelta al fondo debía mostrar la última línea: %q", out)
	}
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

// TestConsoleCommandsSonReales cierra el lazo de ConsoleCommands (ver su
// comentario en console.go, y el test de paridad TestConsoleParityConCLI en
// cmd/maly/commands_test.go): cada nombre de la lista tiene que ser
// aceptado de verdad por el switch de execConsole, nunca caer en "comando
// desconocido" — si no, la lista sería una tercera copia a mano sin ninguna
// garantía real de que refleje el switch. Solo se llama a execConsole (sin
// invocar el tea.Cmd que devuelva): ejecutarlo de verdad tocaría red o DB, y
// lo único que hace falta comprobar es a qué case cayó el switch.
func TestConsoleCommandsSonReales(t *testing.T) {
	for _, name := range ConsoleCommands {
		m := newConModel()
		m.tree = buildTree(nil, nil) // "select" lo necesita (ver TestConsoleSelect)
		wantUnknown := i18n.Tf("con.unknown", name)
		m.execConsole(name)
		for _, line := range m.conLines {
			if strings.Contains(line, wantUnknown) {
				t.Errorf("%q: ConsoleCommands lo lista, pero execConsole lo trata como comando desconocido", name)
			}
		}
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
		"get playlist",
		"get playlist busqueda sin url",
		"get playlist https://x ..", // nombre inválido: sin yt-dlp/ffmpeg falla antes por Tools(), con ellos por el nombre — ambos caminos dan (nil, error)
		"get playlist https://x a/b",
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

// TestNewDirEntry cubre el diffing que resuelve el subdirectorio que yt-dlp
// crea con el título de la playlist (get playlist sin nombre explícito):
// exactamente un directorio nuevo se acepta, cero o más de uno se rechazan
// como ambiguos.
func TestNewDirEntry(t *testing.T) {
	dir := t.TempDir()
	before, err := dirEntries(dir)
	if err != nil {
		t.Fatal(err)
	}
	// Sin nada nuevo: ambiguo.
	if got, err := newDirEntry(dir, before); err == nil {
		t.Errorf("sin subdirectorios nuevos debía fallar, dio %q", got)
	}
	if err := os.Mkdir(filepath.Join(dir, "Mi Playlist"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Exactamente uno nuevo: se identifica.
	if got, err := newDirEntry(dir, before); err != nil || got != "Mi Playlist" {
		t.Fatalf("newDirEntry = (%q, %v), quería (\"Mi Playlist\", nil)", got, err)
	}
	// Dos nuevos: vuelve a ser ambiguo.
	if err := os.Mkdir(filepath.Join(dir, "Otra"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got, err := newDirEntry(dir, before); err == nil {
		t.Errorf("con dos subdirectorios nuevos debía fallar, dio %q", got)
	}
}

// TestConUpdatePackaged: con version.Channel fijado, conUpdate no debe
// entregar el instalador (updRunMsg) — debe resolver en un conMsg que
// mencione el gestor de paquetes, mismo gate que runUpdate en cmd/maly.
func TestConUpdatePackaged(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	bin := t.TempDir()
	script := "#!/bin/sh\nprintf 'aaa\\trefs/tags/v9.9.9\\n'\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin) // sin curl: si igual llegara a InstallerCmd, fallaría mencionándolo

	old := version.Channel
	version.Channel = "pacman"
	defer func() { version.Channel = old }()

	m := newConModel()
	msg := m.conUpdate()()
	cm, ok := msg.(conMsg)
	if !ok {
		t.Fatalf("conUpdate con canal empaquetado debía dar conMsg, dio %T (%+v)", msg, msg)
	}
	joined := strings.Join(cm.lines, "\n")
	if !strings.Contains(joined, "v9.9.9") || !strings.Contains(joined, "package manager") {
		t.Errorf("líneas = %q, esperaba mencionar v9.9.9 y el gestor de paquetes", cm.lines)
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
