package tui

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"maly/internal/config"
	"maly/internal/getter"
	"maly/internal/i18n"
	"maly/internal/ipc"
	"maly/internal/library"
	"maly/internal/safetext"
	"maly/internal/update"
	"maly/internal/version"
)

// La paleta de comandos (ctrl+p) es una consola integrada: se escriben
// comandos estilo CLI ("maly next", "vol +5", "queue"…) y la salida se
// muestra dentro de la propia paleta sin salir de ella.

// conMsg trae la salida de un comando ejecutado contra el demonio o la DB.
// reload = la operación mutó playlists: el árbol de la biblioteca debe
// recargarse (las playlists cuelgan de él y el flash no se ve bajo el modal).
type conMsg struct {
	lines  []string
	reload bool
}

// getDoneMsg vuelve de yt-dlp (tea.ExecProcess); err nil = descarga ok.
//
// fromModal distingue quién lanzó la descarga, y no es cosmético: el
// desenlace se reporta con conPrint/conErr, o sea al log de la consola, que
// solo se ve con la consola ABIERTA. Lanzada desde una pantalla que se cierra
// antes de ceder el terminal a yt-dlp, el usuario no vería nada —ni el "listo"
// ni el error— salvo que el árbol se actualizara solo. Con fromModal el
// desenlace va además al flash del pie, que ahí sí es visible porque no queda
// ningún modal tapándolo.
type getDoneMsg struct {
	err       error
	fromModal bool
	// dir/before son la foto del destino ANTES de la descarga: el código de
	// salida de yt-dlp no alcanza como criterio (una búsqueda sin resultados
	// sale 0 sin bajar nada), así que quien decide si funcionó es el diff del
	// directorio — y con él se limpia la miniatura que deja un 403.
	dir    string
	before map[string]bool
}

// getPlaylistDoneMsg vuelve de yt-dlp para `get playlist` (tea.ExecProcess).
// musicDir/name/dir/before llevan lo que conGetPlaylist ya decidió antes de
// lanzar el proceso: ExecProcess solo puede devolver el error, así que el
// resto del estado tiene que viajar en el mensaje. name == "" significa que
// no había nombre explícito y dir/before son los datos para el diffing.
// createdDir dice si dir lo creó ESTA corrida (nombre explícito): solo
// entonces se limpia el directorio vacío que deja una descarga fallida.
// filesBefore es el snapshot de dir con nombre explícito (G3): lo que YA
// estaba ahí y por tanto NO pertenece a esta descarga.
type getPlaylistDoneMsg struct {
	err         error
	musicDir    string
	name        string
	dir         string
	before      map[string]bool
	filesBefore map[string]bool
	createdDir  bool
}

// updRunMsg trae el instalador listo para correr (hay release nuevo);
// updDoneMsg vuelve cuando terminó.
type updRunMsg struct {
	latest  string
	cmd     *exec.Cmd
	cleanup func()
}
type updDoneMsg struct{ err error }

// conMaxLines limita el historial de salida de la consola.
const conMaxLines = 200

// conHistMax limita el historial de COMANDOS (↑/↓), mismo criterio que
// conMaxLines para la salida.
const conHistMax = 50

func (m *Model) openConsole() tea.Cmd {
	m.consoleOpen = true
	m.conInput = textinput.New()
	m.conInput.Prompt = "❯ "
	m.conInput.PromptStyle = m.st.accent
	m.conInput.TextStyle = m.st.text
	m.conInput.Placeholder = i18n.T("con.ph")
	m.conInput.CharLimit = 200
	m.conInput.Focus()
	m.conHistIdx = len(m.conHistory)
	return textinput.Blink
}

// conPrint agrega una línea (ya estilizada) a la salida de la consola.
func (m *Model) conPrint(line string) {
	m.conLines = append(m.conLines, line)
	if len(m.conLines) > conMaxLines {
		m.conLines = m.conLines[len(m.conLines)-conMaxLines:]
	}
}

// conErr imprime un error; los mensajes de uso multilínea (pl.usage) salen
// línea a línea para que el recorte por ancho no se coma los saltos.
func (m *Model) conErr(text string) {
	for _, l := range strings.Split(text, "\n") {
		m.conPrint(m.st.errSt.Render(l))
	}
}

func (m *Model) handleConsoleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", m.keys["palette"]:
		m.consoleOpen = false
		return m, nil
	case "enter":
		line := strings.TrimSpace(m.conInput.Value())
		m.conInput.SetValue("")
		if line == "" {
			return m, nil
		}
		// Volver siempre al fondo al ejecutar un comando nuevo: si el
		// usuario había hecho pgup para leer output viejo, el resultado de
		// ESTE comando (y su eco) podía quedar fuera de la vista sin
		// ninguna señal de que se ejecutó (hallazgo UX-N1 de la auditoría).
		m.conScroll = 0
		m.conPrint(m.st.accent.Render("❯ ") + m.st.text.Render(line))
		m.conHistory = append(m.conHistory, line)
		if len(m.conHistory) > conHistMax {
			m.conHistory = m.conHistory[len(m.conHistory)-conHistMax:]
		}
		m.conHistIdx = len(m.conHistory)
		return m.execConsole(line)
	case "up":
		if len(m.conHistory) == 0 {
			return m, nil
		}
		if m.conHistIdx == len(m.conHistory) {
			m.conHistDraft = m.conInput.Value()
		}
		if m.conHistIdx > 0 {
			m.conHistIdx--
		}
		m.conInput.SetValue(m.conHistory[m.conHistIdx])
		m.conInput.CursorEnd()
		return m, nil
	case "down":
		if len(m.conHistory) == 0 || m.conHistIdx >= len(m.conHistory) {
			return m, nil
		}
		m.conHistIdx++
		if m.conHistIdx == len(m.conHistory) {
			m.conInput.SetValue(m.conHistDraft)
		} else {
			m.conInput.SetValue(m.conHistory[m.conHistIdx])
		}
		m.conInput.CursorEnd()
		return m, nil
	case "pgup":
		m.conScroll += 5
		return m, nil
	case "pgdown":
		m.conScroll -= 5
		if m.conScroll < 0 {
			m.conScroll = 0
		}
		return m, nil
	case "ctrl+home":
		// "home"/"end" sueltos no sirven acá: la consola tiene un textinput
		// activo (m.conInput) y esas teclas ya mueven el cursor dentro del
		// comando que se está escribiendo — ctrl+home/ctrl+end, como sugiere
		// la propia auditoría, no chocan con eso. Valor a propósito más
		// grande que cualquier maxScroll real: el propio render() lo acota
		// (ver su "if m.conScroll > maxScroll"), así que no hace falta
		// duplicar acá el cálculo de maxRows/ancho.
		m.conScroll = len(m.conLines)
		return m, nil
	case "ctrl+end":
		m.conScroll = 0
		return m, nil
	}
	var cmd tea.Cmd
	m.conInput, cmd = m.conInput.Update(msg)
	return m, cmd
}

// ConsoleCommands son los nombres que execConsole acepta — mantener esta
// lista al día con el switch de abajo (nombres primarios, sin alias como
// "-h"). cmd/maly la usa para el test de paridad contra la tabla `commands`
// de la CLI (ver commands_test.go: TestConsoleParityConCLI): sin esa red, el
// switch podía derivar de la tabla real sin que ningún test lo notara — el
// gap real que la destapó fue `remove`, que existía en la CLI y no acá hasta
// que se cerró junto con este test. El propio paquete tui verifica además
// que cada nombre de ESTA lista es aceptado de verdad por el switch
// (TestConsoleCommandsSonReales en console_test.go): sin eso, la lista sería
// una tercera copia a mano y el test de cmd/maly compararía copia contra
// copia.
var ConsoleCommands = []string{
	"help", "quit", "exit", "kill", "cls", "viz",
	"play", "pause", "toggle", "stop", "next", "prev", "clear",
	"add", "jump", "move", "remove", "vol", "seek", "shuffle", "repeat",
	"status", "queue", "search", "select", "playlist", "get",
	"controls", "logo", "lang", "info", "doctor", "config", "version", "update",
	"scan", "rescan",
}

// execConsole interpreta una línea como si fuera la CLI de maly. El prefijo
// "maly" es opcional.
func (m *Model) execConsole(line string) (tea.Model, tea.Cmd) {
	fields := strings.Fields(line)
	if fields[0] == "maly" {
		fields = fields[1:]
	}
	if len(fields) == 0 {
		m.conHelp()
		return m, nil
	}
	cmd, args := fields[0], fields[1:]

	// Borrado de playlist pendiente de confirmar: esta guarda va ANTES del
	// switch, y consume la línea entera pase lo que pase — igual que
	// confirmYesNo en la CLI, que lee UNA línea y termina el comando. Una
	// playlist armada a mano no tiene deshacer (C15/T26); la CLI y el panel
	// ctrl+l ya confirmaban, y la consola era el tercer camino, el único que
	// borraba de una (auditoría 2026-09-04, A-04). Wording default-no: solo
	// sí/y confirma.
	if name := m.conPlConfirm; name != "" {
		m.conPlConfirm = ""
		switch strings.ToLower(strings.Join(fields, " ")) {
		case "s", "si", "sí", "y", "yes":
			return m, m.conPlaylistDelete(name)
		}
		m.conPrint(m.st.dim.Render(i18n.Tf("pl.delete_kept", name)))
		return m, nil
	}

	switch cmd {
	case "help", "-h", "--help":
		m.conHelp()
		return m, nil
	case "quit", "exit":
		return m, tea.Quit
	case "kill":
		// Apaga el demonio y la TUI. Con demonio embebido basta salir: el
		// defer d.Close() de runTUI lo apaga; con uno externo se le manda
		// shutdown y luego se sale (la TUI se quedaría sin backend).
		if m.embedded {
			return m, tea.Quit
		}
		sock := m.sock
		return m, func() tea.Msg {
			if c, err := ipc.Dial(sock); err == nil {
				c.Do(ipc.Request{Cmd: "shutdown"})
				c.Close()
			}
			return tea.Quit()
		}
	case "cls":
		m.conLines = nil
		return m, nil
	case "viz":
		m.vizOn = !m.vizOn
		if m.vizOn {
			m.conPrint(m.st.playing.Render(i18n.T("con.viz_on")))
		} else {
			m.conPrint(m.st.dim.Render(i18n.T("con.viz_off")))
		}
		return m, m.armVizTick()
	case "play":
		return m, m.conReq(ipc.Request{Cmd: "play", Query: strings.Join(args, " ")})
	case "pause", "toggle", "stop", "next", "prev", "clear":
		return m, m.conReq(ipc.Request{Cmd: cmd})
	case "add":
		if len(args) == 0 {
			m.conErr(i18n.T("con.usage_add"))
			return m, nil
		}
		return m, m.conReq(ipc.Request{Cmd: "add", Query: strings.Join(args, " ")})
	case "jump":
		if len(args) != 1 {
			m.conErr(i18n.T("con.usage_jump"))
			return m, nil
		}
		n, err := strconv.Atoi(args[0])
		if err != nil || n < 1 {
			m.conErr(i18n.T("con.usage_jump"))
			return m, nil
		}
		return m, m.conReq(ipc.Request{Cmd: "jump", Index: n - 1})
	case "move":
		if len(args) != 2 {
			m.conErr(i18n.T("con.usage_move"))
			return m, nil
		}
		from, errF := strconv.Atoi(args[0])
		to, errT := strconv.Atoi(args[1])
		if errF != nil || errT != nil || from < 1 || to < 1 {
			m.conErr(i18n.T("con.usage_move"))
			return m, nil
		}
		return m, m.conReq(ipc.Request{Cmd: "move", Index: from - 1, To: to - 1})
	case "remove":
		if len(args) != 1 {
			m.conErr(i18n.T("con.usage_remove"))
			return m, nil
		}
		n, err := strconv.Atoi(args[0])
		if err != nil || n < 1 {
			m.conErr(i18n.T("con.usage_remove"))
			return m, nil
		}
		return m, m.conReq(ipc.Request{Cmd: "remove", Index: n - 1})
	case "vol":
		if len(args) != 1 {
			m.conErr(i18n.T("con.usage_vol"))
			return m, nil
		}
		return m, m.conReq(ipc.Request{Cmd: "vol", Value: args[0]})
	case "seek":
		if len(args) != 1 {
			m.conErr(i18n.T("con.usage_seek"))
			return m, nil
		}
		return m, m.conReq(ipc.Request{Cmd: "seek", Value: args[0]})
	case "shuffle", "repeat":
		req := ipc.Request{Cmd: cmd}
		if len(args) > 0 {
			req.Value = args[0]
		}
		return m, m.conReq(req)
	case "status":
		return m, m.conQuery(ipc.Request{Cmd: "status"}, func(st styles, resp ipc.Response) []string {
			return statusLines(st, resp.Status)
		})
	case "queue":
		return m, m.conQuery(ipc.Request{Cmd: "queue"}, queueLines)
	case "search":
		if len(args) == 0 {
			m.conErr(i18n.T("cli.usage_search"))
			return m, nil
		}
		return m, m.conQuery(ipc.Request{Cmd: "search", Query: strings.Join(args, " ")}, searchLines)
	case "select":
		m.consoleOpen = false
		return m, m.openSongs()
	case "playlist":
		return m.conPlaylist(args)
	case "get":
		return m.conGet(args)
	case "controls":
		return m.conControls(args)
	case "logo":
		return m.conLogo(args)
	case "lang":
		return m.conLang(args)
	case "info":
		return m, m.conInfo()
	case "doctor":
		return m, m.conDoctor()
	case "config":
		return m, m.conConfig()
	case "version":
		return m, m.conVersion()
	case "update":
		return m, m.conUpdate()
	case "scan", "rescan":
		m.conPrint(m.st.dim.Render(i18n.T("con.scanning")))
		return m, m.conScan(strings.Join(args, " "))
	default:
		m.conErr(i18n.Tf("con.unknown", cmd))
		return m, nil
	}
}

// conHelp imprime la ayuda de la consola reutilizando las descripciones CLI.
func (m *Model) conHelp() {
	m.conPrint(m.st.accent.Render(i18n.T("con.help_head")))
	rows := [][2]string{
		{"play [q]", i18n.T("cli.play")},
		{"select", i18n.T("cli.select")},
		{"pause / toggle / stop", i18n.T("cli.toggle")},
		{"next / prev", i18n.T("cli.next") + " · " + i18n.T("cli.prev")},
		{"jump <pos>", i18n.T("cli.jump")},
		{"move <from> <to>", i18n.T("cli.move")},
		{"remove <pos>", i18n.T("cli.remove")},
		{"add <q>", i18n.T("cli.add")},
		{"queue / status", i18n.T("cli.queue") + " · " + i18n.T("cli.status")},
		{"vol <0-100|+N|-N>", i18n.T("cli.vol")},
		{"seek <+N|-N|mm:ss>", i18n.T("cli.seek")},
		{"shuffle [on|off]", i18n.T("cli.shuffle")},
		{"repeat [off|all|one]", i18n.T("cli.repeat")},
		{"clear", i18n.T("cli.clear")},
		{"scan [path]", i18n.T("cli.scan")},
		{"search <q>", i18n.T("cli.search")},
		{"get <url|q>", i18n.T("cli.get")},
		{"playlist <sub> [args]", i18n.T("cli.playlist")},
		{"controls [preset]", i18n.T("cli.controls")},
		{"logo [hex… | default]", i18n.T("cli.logo")},
		{"lang [en|es]", i18n.T("cli.lang_cmd")},
		{"info", i18n.T("cli.info")},
		{"doctor", i18n.T("cli.doctor")},
		{"config", i18n.T("cli.config")},
		{"version", i18n.T("cli.version_cmd")},
		{"update", i18n.T("cli.update")},
		{"kill", i18n.T("cli.kill")},
	}
	// clip por columna en vez de confiar en que ningún label/descripción
	// exceda su presupuesto: padTo no acorta, solo rellena, así que una fila
	// más ancha que su columna corría la siguiente y rompía la alineación
	// (con una descripción larga de verdad, como la de playlist, incluso
	// desbordaba el borde de la caja — auditoría de UX post-1.12.0).
	w := pickerWidth(m.width)
	innerW := w - 2
	const labelW = 22
	descW := innerW - 2 - labelW // 2 = indentación "  "
	if descW < 0 {
		descW = 0
	}
	for _, r := range rows {
		m.conPrint(formatHelpRow(m.st, r[0], r[1], labelW, descW))
	}
	m.conPrint(m.st.dim.Render(i18n.T("con.help_local")))
}

// formatHelpRow arma una fila de conHelp() con clip independiente por
// columna: sin esto, un label o una descripción más ancha que su
// presupuesto corría la columna siguiente (padTo no acorta, solo rellena) y
// con una descripción larga de verdad podía desbordar el borde de la caja
// (auditoría de UX post-1.12.0). labelW/descW ya vienen acotados a >= 0.
func formatHelpRow(st styles, label, desc string, labelW, descW int) string {
	label = clip(label, labelW)
	desc = clip(desc, descW)
	return "  " + st.accent.Render(padTo(label, labelW)) + st.text.Render(desc)
}

// conReq ejecuta una acción simple y devuelve su mensaje como salida.
func (m *Model) conReq(r ipc.Request) tea.Cmd {
	sock, st := m.sock, m.st
	return func() tea.Msg {
		c, err := ipc.Dial(sock)
		if err != nil {
			return conMsg{lines: []string{st.errSt.Render(err.Error())}}
		}
		defer c.Close()
		resp, err := c.Do(r)
		switch {
		case err != nil:
			return conMsg{lines: []string{st.errSt.Render(err.Error())}}
		case !resp.OK:
			return conMsg{lines: []string{st.errSt.Render(resp.Error)}}
		case resp.Msg != "":
			return conMsg{lines: []string{st.playing.Render(resp.Msg)}}
		default:
			return conMsg{lines: []string{st.dim.Render(i18n.T("con.ok"))}}
		}
	}
}

// conQuery ejecuta un comando de consulta y formatea la respuesta en líneas.
func (m *Model) conQuery(req ipc.Request, format func(styles, ipc.Response) []string) tea.Cmd {
	sock, st := m.sock, m.st
	return func() tea.Msg {
		c, err := ipc.Dial(sock)
		if err != nil {
			return conMsg{lines: []string{st.errSt.Render(err.Error())}}
		}
		defer c.Close()
		resp, err := c.Do(req)
		if err != nil {
			return conMsg{lines: []string{st.errSt.Render(err.Error())}}
		}
		if !resp.OK {
			return conMsg{lines: []string{st.errSt.Render(resp.Error)}}
		}
		return conMsg{lines: format(st, resp)}
	}
}

// conLib corre una operación sobre la biblioteca (apertura transitoria de la
// DB, como loadPlaylists) y vuelca sus líneas ya estilizadas en la consola.
// reload = la operación mutó playlists: recargar el árbol al terminar.
func (m *Model) conLib(reload bool, fn func(*library.Library) ([]string, error)) tea.Cmd {
	st := m.st
	return func() tea.Msg {
		lib, err := library.Open(config.DBPath())
		if err != nil {
			return conMsg{lines: []string{st.errSt.Render(err.Error())}}
		}
		defer lib.Close()
		lines, err := fn(lib)
		if err != nil {
			return conMsg{lines: []string{st.errSt.Render(err.Error())}}
		}
		if reload {
			// Mutó playlists: avisar al demonio para que las otras TUIs
			// también recarguen (la propia recarga vía conMsg.reload).
			notifyRefresh()
		}
		return conMsg{lines: lines, reload: reload}
	}
}

func (m *Model) conScan(query string) tea.Cmd {
	sock, st := m.sock, m.st
	// Ruta relativa a absoluta: el demonio externo tiene otro cwd (mismo
	// motivo que runScan en la CLI).
	if q := strings.TrimSpace(query); q != "" {
		if abs, err := filepath.Abs(config.ExpandTilde(q)); err == nil {
			query = abs
		}
	}
	// La ruta viaja RESUELTA (hallazgo A-03, espejo de runScan): el config
	// del demonio es de cuando arrancó, el de la TUI es de cuando se abrió.
	// Y, como el demonio ya no puede formar el mensaje de "no existe" —lo
	// recibe todo como explícito—, lo forma el cliente antes de dialar.
	dir, origin, explicit := m.cfg.ScanTarget(query)
	if _, err := os.Stat(dir); errors.Is(err, fs.ErrNotExist) {
		e := config.ScanNoExistErr(dir, origin, explicit)
		return func() tea.Msg {
			return conMsg{lines: []string{st.errSt.Render(e.Error())}}
		}
	}
	return func() tea.Msg {
		c, err := ipc.Dial(sock)
		if err != nil {
			return conMsg{lines: []string{st.errSt.Render(err.Error())}}
		}
		defer c.Close()
		c.Timeout = 10 * time.Minute // una biblioteca grande no cabe en los 30 s default
		resp, err := c.Do(ipc.Request{Cmd: "scan", Query: dir})
		if err != nil {
			return conMsg{lines: []string{st.errSt.Render(err.Error())}}
		}
		if !resp.OK {
			return conMsg{lines: []string{st.errSt.Render(resp.Error)}}
		}
		// La recarga del árbol no se pide aquí: el scan subió LibGen y la
		// foto de estado siguiente la dispara (en esta TUI y en las demás).
		return conMsg{lines: []string{st.playing.Render(resp.Msg)}}
	}
}

// conGet espeja `maly get`: yt-dlp toma el terminal (la TUI se suspende) y
// su progreso pasa directo, cero parsing; al volver, getDoneMsg dispara el
// re-escaneo vía demonio (sube LibGen y todos los árboles recargan solos).
func (m *Model) conGet(args []string) (tea.Model, tea.Cmd) {
	if len(args) == 0 {
		m.conErr(i18n.T("cli.usage_get_cmd"))
		return m, nil
	}
	if args[0] == "playlist" {
		return m.conGetPlaylist(args[1:])
	}
	spec := getter.Spec(strings.Join(args, " "))
	cmd, err := m.startGet(spec, false)
	if err != nil {
		m.conErr(err.Error())
		return m, nil
	}
	m.conPrint(m.st.dim.Render(i18n.Tf("cli.get_start", spec, m.cfg.MusicPath())))
	return m, cmd
}

// startGet lanza yt-dlp sobre spec y devuelve el tea.Cmd que espera su
// salida. Es el punto único de descarga de una pista dentro de la TUI: desde
// acá el flujo —getDoneMsg → re-escaneo → LibGen → todos los clientes
// recargan— es el mismo venga de donde venga, así que quien quiera descargar
// solo tiene que aportar un spec.
//
// Los errores (falta yt-dlp/ffmpeg, no se pudo crear music_dir) vuelven al
// llamador en vez de imprimirse acá: la consola los escribe en su log y otras
// pantallas los muestran a su manera. fromModal viaja hasta getDoneMsg (ver
// su comentario).
func (m *Model) startGet(spec string, fromModal bool) (tea.Cmd, error) {
	if err := getter.Tools(); err != nil {
		return nil, err
	}
	dir := m.cfg.MusicPath()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	// La foto va ANTES de lanzar nada; el desenlace la compara (ver
	// getDoneMsg). Un error de lectura acá no debería impedir descargar: se
	// degrada a before nil, que hace el diff inservible pero no rompe.
	before, _ := getter.Snapshot(dir)
	cmd := getter.Command(getter.Opts{Dir: dir, Spec: spec, Cookies: m.cfg.Ytdlp.CookiesFromBrowser})
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return getDoneMsg{err: err, fromModal: fromModal, dir: dir, before: before}
	}), nil
}

// conGetPlaylist espeja `maly get playlist`: descarga una playlist completa
// de yt-dlp a un subdirectorio de music_dir y crea una playlist de maly con
// esas pistas, en orden. Misma lógica que runGetPlaylist (cmd/maly/get.go,
// ver su comentario para el porqué completo) reimplementada aquí porque
// internal/tui no puede importar cmd/maly (package main); la diferencia es
// que el post-proceso (diffing, escaneo, armado de playlist) no puede
// correr en línea tras cmd.Run() —ExecProcess solo devuelve el error— así
// que viaja en getPlaylistDoneMsg y lo termina conGetPlaylistFinish.
func (m *Model) conGetPlaylist(args []string) (tea.Model, tea.Cmd) {
	if len(args) == 0 || !strings.Contains(args[0], "://") {
		m.conErr(i18n.T("cli.usage_get_playlist"))
		return m, nil
	}
	if err := getter.Tools(); err != nil {
		m.conErr(err.Error())
		return m, nil
	}
	done, opts, err := m.planGetPlaylist(args[0], strings.TrimSpace(strings.Join(args[1:], " ")))
	if err != nil {
		m.conErr(err.Error())
		return m, nil
	}

	m.conPrint(m.st.dim.Render(i18n.Tf("cli.get_pl_start", opts.Spec, done.musicDir)))
	cmd := getter.Command(opts)
	return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
		done.err = err
		return done
	})
}

// planGetPlaylist toma TODAS las decisiones previas a la descarga —validar el
// nombre, rechazar el choque, crear el destino, sacar los snapshots— y
// devuelve el mensaje ya armado con ellas. Está separada de conGetPlaylist
// por una razón concreta: tea.ExecProcess solo se resuelve dentro del runtime
// de bubbletea, así que con todo junto no hay forma de comprobar en un test
// qué decidió esta mitad. Es el mínimo para que A-04 sea verificable; la
// extracción de verdad, a funciones puras compartidas con la CLI en
// internal/getter, es la mitad estructural del hallazgo y va aparte.
func (m *Model) planGetPlaylist(url, name string) (getPlaylistDoneMsg, getter.Opts, error) {
	musicDir := m.cfg.MusicPath()
	if err := os.MkdirAll(musicDir, 0o755); err != nil {
		return getPlaylistDoneMsg{}, getter.Opts{}, err
	}

	opts := getter.Opts{Dir: musicDir, Spec: url, Cookies: m.cfg.Ytdlp.CookiesFromBrowser, Playlist: true}
	var dir string
	var before map[string]bool      // diff de music_dir (rama sin nombre): qué subdirectorio es nuevo
	var filesBefore map[string]bool // diff de dir (rama con nombre): qué archivos NO son de esta descarga
	createdDir := false
	if name != "" {
		// Nombre explícito: validarlo como componente de ruta ANTES de
		// tocar el filesystem o la red — es entrada del usuario
		// volviéndose ruta.
		if filepath.Base(name) != name || name == "." || name == ".." {
			return getPlaylistDoneMsg{}, opts, errors.New(i18n.Tf("cli.get_pl_bad_name", name))
		}
		// Choque de nombre ANTES de tocar filesystem o red: sin esto se
		// descargaba la playlist entera y recién fallaba CreatePlaylist al
		// final (hallazgo G2, cerrado en la CLI en la 1.12.0 y vivo acá
		// hasta la auditoría 2026-09-04, A-04). Comparación exacta, igual
		// que la restricción real de la tabla.
		plib, err := library.Open(config.DBPath())
		if err != nil {
			return getPlaylistDoneMsg{}, opts, err
		}
		lists, err := plib.Playlists()
		plib.Close()
		if err != nil {
			return getPlaylistDoneMsg{}, opts, err
		}
		for _, pl := range lists {
			if pl.Name == name {
				return getPlaylistDoneMsg{}, opts, errors.New(i18n.Tf("lib.pl_exists", name))
			}
		}
		dir = filepath.Join(musicDir, name)
		// Espejo del hallazgo G7 en la CLI: el destino se crea antes de
		// invocar a yt-dlp, así que un intento fallido dejaba un directorio
		// vacío en music_dir. Se limpia solo el que creamos acá, y con
		// os.Remove, que se niega si no está vacío.
		_, statErr := os.Stat(dir)
		createdDir = os.IsNotExist(statErr)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return getPlaylistDoneMsg{}, opts, err
		}
		opts.Dir = dir
		// Con nombre explícito, dir puede ser un directorio preexistente con
		// música ajena: sin este snapshot la playlist se llevaba TODO lo que
		// hubiera ahí y no solo lo recién bajado (hallazgo G3, misma
		// historia que G2).
		if filesBefore, err = getter.Snapshot(dir); err != nil {
			return getPlaylistDoneMsg{}, opts, err
		}
	} else {
		opts.PlaylistSubdir = true
		var err error
		if before, err = getter.Snapshot(musicDir); err != nil {
			return getPlaylistDoneMsg{}, opts, err
		}
	}

	return getPlaylistDoneMsg{musicDir: musicDir, name: name, dir: dir,
		before: before, filesBefore: filesBefore, createdDir: createdDir}, opts, nil
}

// newDirEntry devuelve el único subdirectorio de parent que no estaba en
// before: el que yt-dlp acaba de crear con el título de la playlist. Con más
// de uno el caso es genuinamente ambiguo (mejor pedir un nombre explícito que
// adivinar mal); con CERO y partial=true la causa probable no es ambigüedad
// sino que la descarga falló antes de crear ningún directorio, y el mensaje
// lo dice.
//
// Es GEMELA de la de cmd/maly/get.go, byte a byte: hasta A-04 esta versión no
// conocía `partial` y por eso el comentario decía que eran distintas a
// propósito. Unificarlas exige moverlas a internal/getter, que es la
// extracción planificada aparte (A-04, mitad estructural).
func newDirEntry(parent string, before map[string]bool, partial bool) (string, error) {
	found, n := getter.NewSubdir(parent, before)
	switch {
	case n == 1:
		return found, nil
	case n == 0 && partial:
		return "", errors.New(i18n.T("cli.get_pl_dl_failed"))
	default:
		return "", errors.New(i18n.T("cli.get_pl_ambiguous"))
	}
}

// newTrackIDs devuelve, en orden de nombre, los ids de biblioteca de las
// pistas de dir que pertenecen a ESTA descarga. filesBefore (nil = no aplica,
// que es la rama sin nombre explícito, donde dir lo acaba de crear yt-dlp)
// lista lo que YA estaba ahí: con nombre explícito dir puede ser un
// directorio preexistente con música ajena, y sin este filtro la playlist se
// llevaba todo lo que hubiera dentro (hallazgo G3).
//
// Función aparte para poder comprobar el filtro en un test: dentro de
// conGetPlaylistFinish solo se llega pasando por el demonio.
func newTrackIDs(lib *library.Library, dir string, filesBefore map[string]bool) ([]int64, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	var ids []int64
	for _, e := range entries {
		if e.IsDir() || !library.IsAudio(e.Name()) {
			continue
		}
		if filesBefore != nil && filesBefore[e.Name()] {
			continue
		}
		if t, ok := lib.ByPath(filepath.Join(dir, e.Name())); ok {
			ids = append(ids, t.ID)
		}
	}
	return ids, nil
}

// conGetPlaylistFinish corre tras la descarga: re-escanea, resuelve el
// nombre auto-detectado si hacía falta, arma la playlist de maly y avisa al
// demonio. Todo en el mismo tea.Cmd —E/S bloqueante, como conLib/conScan—
// porque son unos pocos pasos secuenciales sin nada que la UI necesite ver
// a medias.
func (m *Model) conGetPlaylistFinish(msg getPlaylistDoneMsg, partial bool) tea.Cmd {
	sock, st := m.sock, m.st
	musicDir, name, dir := msg.musicDir, msg.name, msg.dir
	before, filesBefore, createdDir := msg.before, msg.filesBefore, msg.createdDir
	return func() tea.Msg {
		// Cualquier salida por error deja el destino como estaba: si lo
		// creamos nosotros y quedó vacío, se va con él (hallazgo G7).
		errLine := func(err error) tea.Msg {
			if createdDir {
				os.Remove(dir)
			}
			return conMsg{lines: []string{st.errSt.Render(err.Error())}}
		}

		c, err := ipc.Dial(sock)
		if err != nil {
			return errLine(err)
		}
		defer c.Close()
		c.Timeout = 10 * time.Minute // una biblioteca grande no cabe en los 30 s default
		resp, err := c.Do(ipc.Request{Cmd: "scan", Query: musicDir})
		if err != nil {
			return errLine(err)
		}
		if !resp.OK {
			return errLine(errors.New(resp.Error))
		}

		if name == "" {
			found, err := newDirEntry(musicDir, before, partial)
			if err != nil {
				return errLine(err)
			}
			// El título de YouTube es texto ajeno volviéndose nombre de
			// playlist: la misma frontera de saneado que ReadTags/ParseLRC.
			name = safetext.Clean(found)
			if name == "" {
				return errLine(errors.New(i18n.T("cli.get_pl_no_title")))
			}
			dir = filepath.Join(musicDir, found)
		}

		lib, err := library.Open(config.DBPath())
		if err != nil {
			return errLine(err)
		}
		defer lib.Close()
		ids, err := newTrackIDs(lib, dir, filesBefore)
		if err != nil {
			return errLine(err)
		}
		if len(ids) == 0 {
			return errLine(errors.New(i18n.Tf("cli.get_pl_empty", dir)))
		}
		if err := lib.CreatePlaylist(name); err != nil {
			return errLine(err)
		}
		if err := lib.AddToPlaylist(name, ids); err != nil {
			return errLine(err)
		}
		notifyRefresh()
		return conMsg{lines: []string{st.playing.Render(i18n.Tf("cli.get_pl_done", name, len(ids)))}, reload: true}
	}
}

// conControls espeja `maly controls`; al fijar un preset recarga el config ya
// mezclado (defaults ← preset ← [keys] del usuario) y lo aplica en vivo.
func (m *Model) conControls(args []string) (tea.Model, tea.Cmd) {
	if len(args) == 0 {
		active := m.cfg.Controls
		if !config.ValidPreset(active) {
			active = "default"
		}
		m.conPrint(m.st.accent.Render(i18n.T("cli.controls_head")))
		for _, name := range config.PresetNames() {
			mark := "  "
			if name == active {
				mark = "* "
			}
			m.conPrint("  " + m.st.text.Render(mark+padTo(name, 11)) + m.st.dim.Render(i18n.T("cli.preset_"+name)))
		}
		return m, nil
	}
	name := args[0]
	if !config.ValidPreset(name) {
		m.conErr(i18n.Tf("cli.controls_invalid", name, strings.Join(config.PresetNames(), ", ")))
		return m, nil
	}
	if err := config.SaveControls(name); err != nil {
		m.conErr(err.Error())
		return m, nil
	}
	if cfg, err := config.Load(); err == nil {
		m.cfg = cfg
		m.keys = cfg.Keys
	}
	m.conPrint(m.st.playing.Render(i18n.Tf("cli.controls_set", name)))
	return m, nil
}

// conLogo muestra o cambia las paradas del gradiente del banner MALODY;
// aplica en vivo (recalcula el ramp) y persiste solo la clave logo de [theme].
func (m *Model) conLogo(args []string) (tea.Model, tea.Cmd) {
	if len(args) == 0 {
		m.conPrint(m.st.accent.Render(i18n.T("cli.logo_current")) + " " + m.st.text.Render(strings.Join(m.cfg.Theme.Logo, " ")))
		m.conPrint(m.st.dim.Render(i18n.T("cli.logo_usage")))
		return m, nil
	}
	var stops []string
	if len(args) == 1 && args[0] == "default" {
		stops = config.Default().Theme.Logo
	} else {
		if len(args) < 2 || len(args) > 8 {
			m.conErr(i18n.T("cli.logo_usage"))
			return m, nil
		}
		stops = make([]string, len(args))
		for i, a := range args {
			s := strings.ToLower(a)
			if !strings.HasPrefix(s, "#") {
				s = "#" + s
			}
			if !config.ValidHex(s) {
				m.conErr(i18n.Tf("cli.logo_invalid", a))
				return m, nil
			}
			stops[i] = s
		}
	}
	if err := config.SaveThemeLogo(stops); err != nil {
		m.conErr(err.Error())
		return m, nil
	}
	m.cfg.Theme.Logo = stops
	m.logo.ramp = logoRamp(stops)
	m.conPrint(m.st.playing.Render(i18n.T("cli.logo_set")))
	return m, nil
}

// conLang espeja `maly lang`: sin argumento abre el selector de idioma; con
// código lo fija en caliente (mismos pasos que handleLangKey).
func (m *Model) conLang(args []string) (tea.Model, tea.Cmd) {
	if len(args) == 0 {
		m.consoleOpen = false
		m.langOpen = true
		return m, nil
	}
	code := args[0]
	if code != "en" && code != "es" {
		m.conErr(i18n.Tf("cli.lang_invalid", code))
		return m, nil
	}
	i18n.Set(code)
	m.cfg.Language = code
	m.filterInput.Placeholder = i18n.T("tui.filter_ph")
	m.conInput.Placeholder = i18n.T("con.ph")
	if err := config.SaveLanguage(code); err != nil {
		m.conErr(err.Error())
		return m, nil
	}
	m.conPrint(m.st.playing.Render(i18n.Tf("cli.lang_set", langLabel(code))))
	// Recargar la biblioteca para que las etiquetas "(desconocido)" etc.
	// se generen en el idioma elegido.
	return m, loadLibrary
}

// conUpdate espeja `maly update`: chequea el último release y, si hay uno
// nuevo, entrega el instalador en un updRunMsg para correrlo con
// tea.ExecProcess (como get: la TUI se suspende y el instalador interactivo
// usa el terminal). Con un binario de un gestor de paquetes
// (version.Packaged()) ni entrega el instalador: remite al gestor, mismo
// gate que runUpdate en cmd/maly/update.go.
func (m *Model) conUpdate() tea.Cmd {
	st := m.st
	return func() tea.Msg {
		latest, err := update.Latest()
		if err != nil {
			return conMsg{lines: []string{st.errSt.Render(err.Error())}}
		}
		update.SaveCache(latest)
		if !update.Newer(latest, version.Version) {
			return conMsg{lines: []string{st.playing.Render(i18n.Tf("up.current", version.Version))}}
		}
		if version.Packaged() {
			return conMsg{lines: []string{st.playing.Render(i18n.Tf("up.found_packaged", latest, version.Version))}}
		}
		cmd, cleanup, err := update.InstallerCmd(latest)
		if err != nil {
			return conMsg{lines: []string{st.errSt.Render(err.Error())}}
		}
		return updRunMsg{latest: latest, cmd: cmd, cleanup: cleanup}
	}
}

// conVersion espeja `maly version`: versión propia y, si el demonio
// responde, la suya (tras actualizar el binario conviene enterarse).
func (m *Model) conVersion() tea.Cmd {
	sock, st := m.sock, m.st
	return func() tea.Msg {
		lines := []string{st.text.Render("Malody Mallow (maly) v" + version.Version)}
		c, err := ipc.Dial(sock)
		if err != nil {
			return conMsg{lines: lines}
		}
		defer c.Close()
		resp, err := c.Do(ipc.Request{Cmd: "ping"})
		if err != nil || !resp.OK {
			return conMsg{lines: lines}
		}
		svc := resp.Version
		if svc == "" {
			svc = "< 0.5.0" // demonios anteriores no reportan versión
		}
		if svc == version.Version {
			lines = append(lines, st.dim.Render(i18n.Tf("cli.version_svc", svc)))
		} else {
			lines = append(lines, st.errSt.Render(i18n.Tf("cli.version_svc_old", svc)))
		}
		return conMsg{lines: lines}
	}
}

func statusLines(st styles, s *ipc.Status) []string {
	if s == nil {
		return nil
	}
	// Espejo de printStatus (cmd/maly/client.go), aviso incluido: es la
	// paridad que A-04 dejó como regla.
	notice := func(lines []string) []string {
		if s.Notice != "" {
			lines = append(lines, st.errSt.Render(s.Notice))
		}
		return lines
	}
	if s.Track == nil {
		return notice([]string{st.dim.Render(i18n.Tf("st.stopped",
			s.Volume, ipc.OnOff(s.Shuffle), s.Repeat, s.QueueLen))})
	}
	icon := "▶"
	if s.Paused {
		icon = "⏸"
	}
	name := s.Track.String()
	if s.Track.Album != "" {
		name += " [" + s.Track.Album + "]"
	}
	return notice([]string{
		st.text.Render(icon + " " + name),
		st.dim.Render(i18n.Tf("st.line2", ipc.FmtTime(s.Position), ipc.FmtTime(s.Duration),
			s.Volume, ipc.OnOff(s.Shuffle), s.Repeat, s.QueueIndex+1, s.QueueLen)),
	})
}

func queueLines(st styles, resp ipc.Response) []string {
	if len(resp.Queue) == 0 {
		return []string{st.dim.Render(i18n.T("con.queue_empty"))}
	}
	cur := -1
	if resp.Status != nil {
		cur = resp.Status.QueueIndex
	}
	out := make([]string, 0, len(resp.Queue))
	for i, t := range resp.Queue {
		mark, style := "  ", st.text
		if i == cur {
			mark, style = "▶ ", st.playing
		}
		out = append(out, style.Render(fmt.Sprintf("%s%3d. %s", mark, i+1, t)))
	}
	return out
}

// searchLines formatea los resultados de `search` (vienen en resp.Queue).
func searchLines(st styles, resp ipc.Response) []string {
	if len(resp.Queue) == 0 {
		return []string{st.dim.Render(i18n.T("cli.search_none"))}
	}
	out := make([]string, 0, len(resp.Queue))
	for i, t := range resp.Queue {
		name := t.String()
		if t.Album != "" {
			name += "  [" + t.Album + "]"
		}
		out = append(out, st.text.Render(fmt.Sprintf("%3d. %s", i+1, name)))
	}
	return out
}

func (m *Model) consoleView() string {
	w := pickerWidth(m.width)
	innerW := w - 2
	maxRows := m.height - 10
	if maxRows > 24 {
		maxRows = 24
	}
	if maxRows < 4 {
		maxRows = 4
	}

	// El textinput de bubbles nunca recibía Width, así que su propio scroll
	// horizontal quedaba desactivado (handleOverflow: "Width <= 0" = sin
	// ventana) y View() emitía el valor completo — un comando largo (una URL
	// de get, por ejemplo) rompía el borde derecho en vivo, mientras se
	// escribe. Se reasigna en cada render, mismo criterio que p.page en
	// picker.render(): barato, y así el próximo Update() recalcula el
	// scroll con el ancho real de la caja. clip() es la red de seguridad
	// para el frame en que Width cambió pero handleOverflow todavía no
	// corrió de nuevo (un resize sin ninguna tecla de por medio).
	promptW := lipgloss.Width(m.conInput.Prompt)
	m.conInput.Width = innerW - promptW
	if m.conInput.Width < 0 {
		m.conInput.Width = 0
	}
	lines := []string{clip(m.conInput.View(), innerW), m.st.dim.Render(strings.Repeat("─", innerW))}
	out := m.conLines
	maxScroll := len(out) - maxRows
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.conScroll > maxScroll {
		m.conScroll = maxScroll
	}
	end := len(out) - m.conScroll
	start := end - maxRows
	if start < 0 {
		start = 0
	}
	out = out[start:end]
	for _, l := range out {
		lines = append(lines, clip(l, innerW))
	}
	// La consola tapa el pie (footer()), que es el único lugar donde vivía
	// el avance del scan — con la consola abierta un scan lanzado desde acá
	// (o desde cualquier otro cliente) quedaba mudo por minutos en una
	// biblioteca grande. Mismos datos que footer() (llegan por la
	// suscripción, no hace falta estado nuevo), mismo formato (auditoría
	// 2026-07-31, hallazgo T12).
	if m.status != nil && m.status.Scanning {
		txt := i18n.Tf("cli.scan_progress", m.status.ScanSeen)
		if m.status.ScanTotal > 0 {
			txt = i18n.Tf("cli.scan_durations", m.status.ScanSeen, m.status.ScanTotal)
		}
		lines = append(lines, m.st.accent.Render(clip(txt, innerW)))
	}
	lines = append(lines, m.st.dim.Render(clip("  "+i18n.T("con.hint"), innerW)))

	box := m.st.panel(i18n.T("con.title"), lines, w, len(lines)+2, true)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
