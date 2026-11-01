package tui

import (
	"bufio"
	"encoding/json"
	"fmt"
	"maly/internal/commandmeta"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"maly/internal/config"
	"maly/internal/i18n"
	"maly/internal/ipc"
	"maly/internal/library"
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
	boxLinesFitWithin(t, out, m.layoutOf().console.width)
}

// TestConHelpNoOverflow cubre el bug reportado: `?`/`help` dentro de la
// consola mostraba filas (get, playlist) más anchas que la caja, rompiendo
// el borde derecho. La forma playlist tiene una fila propia y cada fila se clipea
// por columna, así que ninguna — ni siquiera con una descripción tan larga
// como la de playlist — puede desbordar (auditoría de UX post-1.12.0).
func TestConHelpNoOverflow(t *testing.T) {
	m := newConModel()
	m.openConsole()
	m.width, m.height = 80, 24

	m.conHelp()
	out := m.consoleView()
	boxLinesFitWithin(t, out, m.layoutOf().console.width)

	var getRow string
	for _, l := range m.conLines {
		if strings.Contains(l, "get <url|query>") {
			getRow = l
			break
		}
	}
	if getRow == "" {
		t.Fatalf("no encontré la fila de get, salida: %q", m.conLines)
	}
	if strings.Contains(getRow, "playlist") {
		t.Errorf("la forma playlist ya no debía estar en la fila de get (tiene fila propia): %q", getRow)
	}
}

// TestConsoleScroll cubre el hallazgo T28 de la auditoría P2: la salida de
// la consola siempre mostraba la cola de conLines, sin forma de volver a ver
// las primeras líneas de un search/queue largo.
// TestConEntryBlockUsoLargoNoSePega: la tabla fija de 22 celdas recortaba el
// uso y lo pegaba a su descripción ("playlist delete <name…delete a
// playlist", captura del dueño). Un uso más ancho que su columna baja a su
// propia fila y la descripción sigue empezando en SU columna.
func TestConEntryBlockUsoLargoNoSePega(t *testing.T) {
	m := newConModel() // sin TTY: Render() no agrega ANSI y bytes == celdas
	const innerW = 80
	usageW, wide := conColumns(innerW)
	if !wide {
		t.Fatalf("a %d celdas debía haber columna de descripción", innerW)
	}
	long := commandmeta.Entry{Name: strings.Repeat("x", usageW+10), DescKey: "cli.play"}
	lines := m.conEntryBlock(long, innerW, usageW, wide)
	if len(lines) < 2 {
		t.Fatalf("el uso largo debía ir en su propia fila: %q", lines)
	}
	if strings.Contains(lines[0], i18n.T("cli.play")[:5]) {
		t.Errorf("la descripción quedó pegada al uso: %q", lines[0])
	}
	if col := len(lines[1]) - len(strings.TrimLeft(lines[1], " ")); col != conMarkW+usageW+2 {
		t.Errorf("descripción en la columna %d, quería %d: %q", col, conMarkW+usageW+2, lines[1])
	}
	short := commandmeta.Entry{Name: "next", DescKey: "cli.next"}
	row := m.conEntryBlock(short, innerW, usageW, wide)[0]
	if idx := strings.Index(row, i18n.T("cli.next")); idx != conMarkW+usageW+2 {
		t.Errorf("descripción corta en la columna %d, quería %d: %q", idx, conMarkW+usageW+2, row)
	}
	for _, l := range append(lines, row) {
		if lipgloss.Width(l) > innerW {
			t.Errorf("fila de %d celdas, máximo %d: %q", lipgloss.Width(l), innerW, l)
		}
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

// TestConsoleScrollResetOnEnter cubre el hallazgo UX-N1 de la auditoría
// técnica: si el usuario había hecho pgup para leer output viejo y luego
// ejecutaba un comando nuevo, el resultado (y su eco) podían quedar fuera
// de la vista sin ninguna señal de que el comando se ejecutó de verdad.
func TestConsoleScrollResetOnEnter(t *testing.T) {
	m := newConModel()
	m.openConsole()
	m.width, m.height = 80, 24
	for i := 0; i < 40; i++ {
		m.conPrint(fmt.Sprintf("linea %d", i))
	}

	m.handleConsoleKey(tea.KeyMsg{Type: tea.KeyPgUp})
	m.handleConsoleKey(tea.KeyMsg{Type: tea.KeyPgUp})
	if m.conScroll == 0 {
		t.Fatal("setup: pgup debía mover el scroll")
	}

	m.conInput.SetValue("status")
	m.handleConsoleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if m.conScroll != 0 {
		t.Fatalf("conScroll = %d tras ejecutar un comando, quería 0 (vuelta al fondo)", m.conScroll)
	}
}

// TestConsoleScrollHomeEnd cubre el hallazgo UX-N2: la consola solo tenía
// pgup/pgdown (±5 líneas) para su scroll, sin atajo de salto al principio o
// al final — a diferencia de la ayuda y las letras de "Ahora suena", que sí
// lo tienen. ctrl+home/ctrl+end (no home/end sueltos: esos ya mueven el
// cursor dentro del textinput activo de la consola).
func TestConsoleScrollHomeEnd(t *testing.T) {
	m := newConModel()
	m.openConsole()
	m.width, m.height = 80, 24
	for i := 0; i < 40; i++ {
		m.conPrint(fmt.Sprintf("linea %d", i))
	}

	m.handleConsoleKey(tea.KeyMsg{Type: tea.KeyCtrlHome})
	out := m.consoleView()
	if !strings.Contains(out, "linea 0 ") {
		t.Fatalf("ctrl+home debía llevar al principio: %q", out)
	}

	m.handleConsoleKey(tea.KeyMsg{Type: tea.KeyCtrlEnd})
	out = m.consoleView()
	if !strings.Contains(out, "linea 39") {
		t.Fatalf("ctrl+end debía volver al fondo: %q", out)
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

	// delete ya no borra de una: arma la confirmación (A-04) y la resuelve
	// la línea siguiente. Su cobertura propia está en
	// TestConsolePlaylistDeletePideConfirmacion.
	if _, cmd := m.execConsole("playlist delete favs"); cmd != nil {
		t.Error("delete debe armar la confirmación, no borrar de una")
	}
	if msg := run("s"); !msg.reload {
		t.Errorf("delete confirmado debe pedir recarga del árbol")
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
	m.overlay = ovConsole
	_, cmd := m.execConsole("select")
	if m.overlay == ovConsole {
		t.Errorf("select debe cerrar la consola")
	}
	if m.overlay != ovSongs || cmd == nil {
		t.Errorf("select debe abrir el picker de canciones")
	}
}

// TestConScanMandaLaRutaResuelta es el espejo de
// TestScanMandaLaRutaResuelta (cmd/maly): la consola ctrl+p también manda la
// ruta ya resuelta, para que el demonio no la decida con su copia rancia del
// config (hallazgo A-03).
//
// El test existe por lo que la propia auditoría señala en A-04: internal/tui
// no puede importar cmd/maly, así que cada arreglo de scan/get/playlist hay
// que aplicarlo dos veces — y tres arreglos ya publicados sobrevivieron en un
// solo lado por no tener test del otro.
func TestConScanMandaLaRutaResuelta(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmp, "data"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "cfg"))
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(tmp, "rt"))

	musica := filepath.Join(tmp, "musica")
	if err := os.MkdirAll(musica, 0o755); err != nil {
		t.Fatal(err)
	}
	sock := filepath.Join(tmp, "s.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	pedidos := make(chan ipc.Request, 4)
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				br := bufio.NewReader(c)
				for {
					linea, err := br.ReadBytes('\n')
					if err != nil {
						return
					}
					var req ipc.Request
					if err := json.Unmarshal(linea, &req); err != nil {
						return
					}
					pedidos <- req
					if err := json.NewEncoder(c).Encode(ipc.Response{OK: true, Msg: "ok"}); err != nil {
						return
					}
				}
			}(c)
		}
	}()

	m := &Model{
		st:   newStyles(config.Theme{}),
		sock: sock,
		cfg:  config.Config{MusicDir: musica},
	}
	if msg := m.conScan("")(); msg == nil {
		t.Fatal("conScan no devolvió ningún mensaje")
	}
	select {
	case req := <-pedidos:
		if req.Query == "" {
			t.Fatal("Query vacío: el demonio volvería a resolver la ruta con SU config rancio")
		}
		if req.Query != musica {
			t.Errorf("Query = %q, quería el music_dir resuelto %q", req.Query, musica)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("el demonio falso no recibió ninguna petición")
	}
}

// TestConsoleSearchNoMandaConsultaVacia fija la premisa con la que la 1.6.1
// decidió dejar `search` DENTRO de d.mu: la consulta vacía —la única que
// recorre la biblioteca entera, ~96 ms con 40.000 pistas— "no es alcanzable
// por el demonio". A-20 encontró que sí lo era por la CLI, pero esa ruta lee
// SQLite directo y nunca toca d.mu; la consola es el ÚNICO cliente que manda
// la op `search` al demonio, y no puede mandarla vacía porque execConsole
// parte la línea con strings.Fields.
//
// El test existe porque esa garantía es de strings.Fields y no de una guarda
// explícita: si algún día la consola aprendiera a respetar comillas (se
// discutió en C14), `search ""` llegaría al demonio y la premisa caería en
// silencio.
func TestConsoleSearchNoMandaConsultaVacia(t *testing.T) {
	for _, linea := range []string{"search", "search   ", "search \t "} {
		m := newConModel()
		if _, cmd := m.execConsole(linea); cmd != nil {
			t.Errorf("%q no debía llegar a mandar nada al demonio", linea)
		}
		if salida := strings.Join(m.conLines, "\n"); !strings.Contains(salida, i18n.T("cli.usage_search")) {
			t.Errorf("%q debía responder con el uso, salió %q", linea, salida)
		}
	}
}

// TestConsolePlaylistListNoFabricaLaBiblioteca: el espejo del test homónimo de
// cmd/maly (A-12). conLib usa su flag `reload` —true exactamente para lo que
// muta playlists— para elegir entre Create y Open, así que la consola respeta
// el mismo reparto que la CLI: consultar no fabrica, escribir sí puede.
func TestConsolePlaylistListNoFabricaLaBiblioteca(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmp, "data"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "cfg"))

	m := newConModel()
	run := func(args ...string) string {
		t.Helper()
		_, cmd := m.conPlaylist(args)
		if cmd == nil {
			return ""
		}
		msg, ok := cmd().(conMsg)
		if !ok {
			return ""
		}
		return strings.Join(msg.lines, "\n")
	}

	if out := run("list"); !strings.Contains(out, i18n.T("pl.no_library")) {
		t.Errorf("sin biblioteca, `playlist list` debía decir que no la hay; salió %q", out)
	}
	if _, err := os.Stat(config.DBPath()); err == nil {
		t.Fatal("`playlist list` en la consola creó la base de datos")
	}

	// Control positivo: el que escribe sí puede estrenarla.
	if out := run("create", "favs"); strings.Contains(out, i18n.T("pl.no_library")) {
		t.Errorf("`playlist create` debía poder estrenar la base; salió %q", out)
	}
	if _, err := os.Stat(config.DBPath()); err != nil {
		t.Fatal("`playlist create` en la consola no dejó creada la base de datos")
	}
}

// TestConsolePlaylistAddAvisaDeLasRepetidas es el espejo exacto del test
// homónimo de cmd/maly (A-23). Los dos existen porque un comando que vive en
// los dos lados se arregla en los dos —la lección de A-04—, y ninguna de las
// dos redes de paridad que hay lo cubriría: ConsoleCommands compara NOMBRES y
// parity_test.go compara comportamiento pero solo el de `get playlist`.
//
// Cada lado comprueba además lo que se espera de verdad (el mensaje del
// helper con 0 nuevas y 2 repetidas), no solo que los dos coincidan: dos
// espejos igual de rotos también coinciden, que es lo que O-03 dejó anotado.
func TestConsolePlaylistAddAvisaDeLasRepetidas(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmp, "data"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "cfg"))

	music := t.TempDir()
	for _, n := range []string{"Luna llena.mp3", "Luna nueva.mp3"} {
		if err := os.WriteFile(filepath.Join(music, n), []byte("mp3 falso"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	lib, err := library.Create(config.DBPath())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lib.Scan(music, nil); err != nil {
		t.Fatal(err)
	}
	if err := lib.CreatePlaylist("favs"); err != nil {
		t.Fatal(err)
	}
	lib.Close()

	m := newConModel()
	run := func(args ...string) string {
		t.Helper()
		_, cmd := m.conPlaylist(args)
		if cmd == nil {
			t.Fatal("conPlaylist no devolvió comando")
		}
		msg, ok := cmd().(conMsg)
		if !ok {
			t.Fatalf("conPlaylist devolvió %T", cmd())
		}
		return strings.Join(msg.lines, "\n")
	}

	primera := run("add", "favs", "Luna")
	segunda := run("add", "favs", "Luna")
	if primera == segunda {
		t.Errorf("las dos pasadas dijeron lo mismo (%q): la segunda duplicó dos pistas en silencio", primera)
	}
	if want := ipc.PlaylistAdded(0, 2, "favs"); !strings.Contains(segunda, want) {
		t.Errorf("la segunda pasada dijo %q, quería %q", segunda, want)
	}

	lib2, err := library.Open(config.DBPath())
	if err != nil {
		t.Fatal(err)
	}
	defer lib2.Close()
	if tracks, err := lib2.PlaylistTracks("favs"); err != nil || len(tracks) != 4 {
		t.Fatalf("PlaylistTracks: %d pistas, %v; avisar no debe bloquear", len(tracks), err)
	}
}

// TestSplitArgs: la consola parte la línea como la shell parte la de la CLI,
// para que `add Secreto 'Otra dimension'` sea el espejo exacto de `maly add
// Secreto 'Otra dimension'`. Una comilla en medio de una palabra es un
// apóstrofo de título, no un delimitador.
func TestSplitArgs(t *testing.T) {
	casos := []struct {
		in   string
		want []string
	}{
		{"add Secreto 'Otra dimension'", []string{"add", "Secreto", "Otra dimension"}},
		{`add "Otra dimension"  luna`, []string{"add", "Otra dimension", "luna"}},
		{"  next  ", []string{"next"}},
		{"add Don't Stop", []string{"add", "Don't", "Stop"}},
		{`add "Don't Stop Me Now"`, []string{"add", "Don't Stop Me Now"}},
		{`add 'dice "hola"'`, []string{"add", `dice "hola"`}},
		{"add x ''", []string{"add", "x", ""}},
		{"add\tx", []string{"add", "x"}},
	}
	for _, c := range casos {
		got, err := splitArgs(c.in)
		if err != nil || !reflect.DeepEqual(got, c.want) {
			t.Errorf("splitArgs(%q) = %q, %v; quería %q", c.in, got, err, c.want)
		}
	}
	for _, malo := range []string{"add 'Otra dimension", `add "x`} {
		if got, err := splitArgs(malo); err == nil {
			t.Errorf("splitArgs(%q) = %q; una comilla sin cerrar debía ser error", malo, got)
		}
	}
}

// TestConsoleAddPlayEnLote: la consola manda una consulta por argumento,
// igual que buildRequest en la CLI — se mira el ipc.Request REAL que le
// llega al demonio.
func TestConsoleAddPlayEnLote(t *testing.T) {
	sock, reqs := capturaReq(t)
	m := newConModel()
	m.sock = sock
	casos := []struct {
		linea string
		want  ipc.Request
	}{
		{"add Secreto 'Otra dimension'", ipc.Request{Cmd: "add", Queries: []string{"Secreto", "Otra dimension"}}},
		{`maly add "Otra dimension" ''`, ipc.Request{Cmd: "add", Query: "Otra dimension"}},
		{`play "pink floyd"`, ipc.Request{Cmd: "play", Query: "pink floyd"}},
		{"play pink floyd", ipc.Request{Cmd: "play", Queries: []string{"pink", "floyd"}}},
		{"play", ipc.Request{Cmd: "play"}},
	}
	for _, c := range casos {
		_, cmd := m.execConsole(c.linea)
		if cmd == nil {
			t.Fatalf("%q no mandó nada", c.linea)
		}
		cmd()
		select {
		case got := <-reqs:
			got.Lang = "" // lo pone ipc.Do; aquí importa la gramática
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("%q mandó %+v, quería %+v", c.linea, got, c.want)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("%q: el demonio falso no recibió nada", c.linea)
		}
	}

	// Comilla sin cerrar o add sin nada: error en la consola, nada al demonio.
	for _, linea := range []string{"add 'Otra dimension", "add ''"} {
		if _, cmd := m.execConsole(linea); cmd != nil {
			t.Errorf("%q no debía mandar nada", linea)
		}
	}
}

// TestConsolePlaylistAddEnLote es el espejo de TestPlaylistAddEnLote de
// cmd/maly: una consulta por argumento, en su orden, sin repetir, y las que
// no encuentran nada nombradas en la misma línea sin cancelar las demás.
func TestConsolePlaylistAddEnLote(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmp, "data"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "cfg"))
	prev := i18n.Code()
	t.Cleanup(func() { i18n.Set(prev) })
	i18n.Set("es")

	music := t.TempDir()
	for _, n := range []string{"Secreto.mp3", "Otra dimension.mp3"} {
		if err := os.WriteFile(filepath.Join(music, n), []byte("mp3 falso"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	lib, err := library.Create(config.DBPath())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lib.Scan(music, nil); err != nil {
		t.Fatal(err)
	}
	if err := lib.CreatePlaylist("favs"); err != nil {
		t.Fatal(err)
	}
	lib.Close()

	m := newConModel()
	args, err := splitArgs("add favs 'Otra dimension' nada Secreto secreto")
	if err != nil {
		t.Fatal(err)
	}
	_, cmd := m.conPlaylist(args)
	msg, ok := cmd().(conMsg)
	if !ok {
		t.Fatalf("conPlaylist devolvió %T", cmd())
	}
	got := strings.Join(msg.lines, "\n")
	want := ipc.WithNoMatch("", ipc.PlaylistAdded(2, 0, "favs"), []string{"nada"})
	if !strings.Contains(got, want) {
		t.Errorf("la consola dijo %q, quería %q", got, want)
	}

	lib2, err := library.Open(config.DBPath())
	if err != nil {
		t.Fatal(err)
	}
	defer lib2.Close()
	tracks, err := lib2.PlaylistTracks("favs")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tr := range tracks {
		names = append(names, filepath.Base(tr.Path))
	}
	if want := []string{"Otra dimension.mp3", "Secreto.mp3"}; !reflect.DeepEqual(names, want) {
		t.Errorf("playlist = %q, quería %q (orden de los argumentos, sin repetir)", names, want)
	}
}
