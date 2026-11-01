package tui

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"maly/internal/commandmeta"
	"maly/internal/config"
	"maly/internal/getter"
	"maly/internal/i18n"
	"maly/internal/ipc"
	"maly/internal/library"
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
	lines           []string
	reload          bool
	downloaded      []library.Track
	downloadSummary string
	downloadGen     int
}

// getDoneMsg vuelve de yt-dlp (tea.ExecProcess). Collect verifica los archivos
// antes de decidir entre éxito, parcial, existente y fallo.
//
// fromModal distingue quién lanzó la descarga, y no es cosmético: el
// desenlace se reporta con conPrint/conErr, o sea al log de la consola, que
// solo se ve con la consola ABIERTA. Lanzada desde una pantalla que se cierra
// antes de ceder el terminal a yt-dlp, el usuario no vería nada —ni el "listo"
// ni el error— salvo que el árbol se actualizara solo. Con fromModal el
// fallo abre la consola para conservar el error; los archivos obtenidos abren
// la fase local del selector y esperan al escaneo antes de ofrecer acciones.
type getDoneMsg struct {
	output    []string
	err       error
	fromModal bool
	// dir/before son la foto del destino ANTES de la descarga: el código de
	// salida de yt-dlp no alcanza como criterio (una búsqueda sin resultados
	// sale 0 sin bajar nada), así que quien decide si funcionó es el diff del
	// directorio — y con él se limpia la miniatura que deja un 403.
	dir    string
	before map[string]bool
	// pathsFile es el temporal donde yt-dlp escribió la ruta de cada archivo
	// que produjo, incluidos los que SALTÓ por existir ya: es lo único que
	// separa "ya lo tenías" de "no encontré nada" (A-07). "" = no se pudo
	// crear, y entonces manda el diff solo, como antes.
	pathsFile string
}

// getPlaylistDoneMsg vuelve de yt-dlp para `get playlist` (tea.ExecProcess).
// Lleva el plan entero porque ExecProcess solo puede devolver el error: todo
// lo que conGetPlaylist decidió antes de lanzar el proceso tiene que viajar
// en el mensaje hasta conGetPlaylistFinish.
type getPlaylistDoneMsg struct {
	output []string
	err    error
	plan   getter.PlaylistPlan
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
	m.overlay = ovConsole
	m.conChoice = ""
	m.conPlConfirm = ""
	m.conInput = textinput.New()
	m.conInput.Prompt = "❯ "
	m.conInput.PromptStyle = m.st.accent
	m.conInput.TextStyle = m.st.text
	m.conInput.PlaceholderStyle = m.st.dim
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
	case "esc":
		if m.conChoice != "" {
			m.conChoice = ""
			return m, nil
		}
		if name := m.conPlConfirm; name != "" {
			m.conPlConfirm = ""
			m.conInput.SetValue("")
			m.conPrint(m.st.dim.Render(i18n.Tf("pl.delete_kept", name)))
			return m, nil
		}
		m.getGen++ // invalida una finalización de descarga cerrada
		m.overlay = ovNone
		return m, nil
	case m.keys["palette"]:
		m.conChoice = ""
		m.conPlConfirm = ""
		m.getGen++ // invalida una finalización de descarga cerrada
		m.overlay = ovNone
		return m, nil
	case "tab", "shift+tab":
		m.conSelectSuggestion(msg.String() == "shift+tab")
		return m, nil
	case "enter":
		if m.conAcceptSuggestion() {
			return m, nil
		}
		line := strings.TrimSpace(m.conInput.Value())
		m.conInput.SetValue("")
		if line == "" && m.conPlConfirm == "" {
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
	case "up", "ctrl+up":
		if m.conChoice != "" && msg.String() == "up" {
			m.conSelectSuggestion(true)
			return m, nil
		}
		m.conChoice = ""
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
	case "down", "ctrl+down":
		if msg.String() == "down" && (m.conChoice != "" || m.conHistIdx == len(m.conHistory)) && len(m.conSuggestions()) > 0 {
			m.conSelectSuggestion(false)
			return m, nil
		}
		m.conChoice = ""
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
		// duplicar acá el presupuesto vertical. Contamos filas envueltas.
		m.conScroll = len(m.consoleOutput(m.layoutOf().console.contentW))
		return m, nil
	case "ctrl+end":
		m.conScroll = 0
		return m, nil
	}
	m.conChoice = ""
	var cmd tea.Cmd
	m.conInput, cmd = m.conInput.Update(msg)
	return m, cmd
}

// ConsoleCommands deriva del catálogo compartido. Los tests verifican cada
// nombre contra el dispatch real, además de la paridad con la CLI.
var ConsoleCommands = func() []string {
	var names []string
	for _, e := range commandmeta.Console() {
		if !strings.Contains(e.Name, " ") {
			names = append(names, e.Name)
		}
	}
	return names
}()

// splitArgs parte una línea de la consola en argumentos como lo haría la
// shell con la CLI: por espacios, salvo dentro de comillas simples o
// dobles, que agrupan y no forman parte del argumento. Sin esto `add
// Secreto 'Otra dimension'` no podía ser el espejo de la CLI, y un
// `playlist create "mi lista"` creaba la playlist con las comillas puestas.
//
// A diferencia de la shell, una comilla solo abre si EMPIEZA un argumento:
// en medio de una palabra es un apóstrofo de título (`Don't Stop`) y la
// consola no tiene un prompt de continuación que deje cerrarla. Tampoco hay
// barra invertida de escape: los títulos no la necesitan. Una comilla que
// abre y no cierra es un error, no una adivinanza.
func splitArgs(line string) ([]string, error) {
	var out []string
	var cur strings.Builder
	inArg := false // hay un argumento en curso (puede ser "" entre comillas)
	var quote rune // comilla abierta; 0 = ninguna
	for _, r := range line {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == ' ' || r == '\t':
			if inArg {
				out = append(out, cur.String())
				cur.Reset()
				inArg = false
			}
		case (r == '\'' || r == '"') && !inArg:
			quote, inArg = r, true
		default:
			cur.WriteRune(r)
			inArg = true
		}
	}
	if quote != 0 {
		return nil, errors.New(i18n.T("con.bad_quote"))
	}
	if inArg {
		out = append(out, cur.String())
	}
	return out, nil
}

// nonBlank descarta los argumentos en blanco (`add x ""`).
func nonBlank(args []string) []string {
	var out []string
	for _, a := range args {
		if strings.TrimSpace(a) != "" {
			out = append(out, a)
		}
	}
	return out
}

// execConsole interpreta una línea como si fuera la CLI de maly. El prefijo
// "maly" es opcional.
func (m *Model) execConsole(line string) (tea.Model, tea.Cmd) {
	// Borrado de playlist pendiente de confirmar: esta guarda va ANTES del
	// switch, y consume la línea entera pase lo que pase — igual que
	// confirmYesNo en la CLI, que lee UNA línea y termina el comando. Una
	// playlist armada a mano no tiene deshacer (C15/T26); la CLI y el panel
	// ctrl+l ya confirmaban, y la consola era el tercer camino, el único que
	// borraba de una (auditoría 2026-09-04, A-04). Wording default-no: solo
	// sí/y confirma.
	if name := m.conPlConfirm; name != "" {
		m.conPlConfirm = ""
		switch strings.ToLower(strings.TrimSpace(line)) {
		case "s", "si", "sí", "y", "yes":
			return m, m.conPlaylistDelete(name)
		}
		m.conPrint(m.st.dim.Render(i18n.Tf("pl.delete_kept", name)))
		return m, nil
	}

	fields, err := splitArgs(line)
	if err != nil {
		m.conErr(err.Error())
		return m, nil
	}
	if len(fields) > 0 && fields[0] == "maly" {
		fields = fields[1:]
	}
	if len(fields) == 0 {
		m.conHelp()
		return m, nil
	}
	cmd, args := fields[0], fields[1:]

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
		if len(args) > 0 {
			if len(args) != 1 || !config.ValidVisualizerStyle(args[0]) {
				m.conErr(i18n.T("con.viz_usage"))
				return m, nil
			}
			if err := config.SaveVisualizerStyle(args[0]); err != nil {
				m.conErr(err.Error())
				return m, nil
			}
			m.cfg.Visualizer.Style = args[0]
			m.conPrint(m.st.playing.Render(i18n.Tf("con.viz_style_set", args[0])))
			return m, nil
		}
		m.vizOn = !m.vizOn
		if m.vizOn {
			m.conPrint(m.st.playing.Render(i18n.T("con.viz_on")))
		} else {
			m.conPrint(m.st.dim.Render(i18n.T("con.viz_off")))
		}
		return m, m.armVizTick()
	case "play":
		// Una consulta por argumento, como la CLI (ver buildRequest).
		req := ipc.Request{Cmd: "play"}
		req.SetQueries(nonBlank(args))
		return m, m.conReq(req)
	case "pause", "toggle", "stop", "next", "prev", "clear":
		return m, m.conReq(ipc.Request{Cmd: cmd})
	case "add", "playnext":
		items := nonBlank(args)
		if len(items) == 0 {
			if cmd == "playnext" {
				m.conErr(i18n.T("cli.usage_playnext"))
				return m, nil
			}
			m.conErr(i18n.T("con.usage_add"))
			return m, nil
		}
		req := ipc.Request{Cmd: cmd}
		req.SetQueries(items)
		return m, m.conReq(req)
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
	case "undo":
		if len(args) != 0 {
			m.conErr(i18n.T("cli.usage_undo"))
			return m, nil
		}
		return m, m.conReq(ipc.Request{Cmd: "undo"})
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
		m.overlay = ovNone
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
	// La ayuda es el catálogo completo por categorías, con las descripciones
	// ENVUELTAS: la tabla anterior recortaba cada uso a 22 celdas fijas y lo
	// pegaba a su descripción ("playlist delete <name…delete a playlist").
	// Las columnas se miden con el ancho actual de la caja (ver conColumns).
	innerW := m.layoutOf().console.contentW
	usageW, wide := conColumns(innerW)
	group := ""
	for _, e := range commandmeta.Console() {
		if e.Group != group {
			if group != "" {
				m.conPrint("")
			}
			group = e.Group
			m.conPrint(m.conHeading(group, innerW))
		}
		for _, l := range m.conEntryBlock(e, innerW, conGroupColumn(e.Group, usageW), wide) {
			m.conPrint(l)
		}
	}
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
		// reload ya distingue lo que hace falta acá: es true exactamente para
		// las operaciones que MUTAN playlists. Esas pueden estrenar la base
		// (Create); las de solo lectura la abren con Open, que no la fabrica
		// — un `playlist list` sobre una instalación recién hecha dejaba un
		// library.db vacío en $XDG_DATA_HOME (A-12). Es el mismo reparto que
		// hace runPlaylist en la CLI.
		open := library.Create
		if !reload {
			open = library.Open
		}
		lib, err := open(config.DBPath())
		if errors.Is(err, fs.ErrNotExist) {
			return conMsg{lines: []string{st.errSt.Render(i18n.T("pl.no_library"))}}
		}
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
	if args[0] == "pick" {
		query := strings.TrimSpace(strings.Join(args[1:], " "))
		if query == "" {
			m.conErr(i18n.T("cli.usage_get_pick"))
			return m, nil
		}
		// Mismo buscador que Ctrl+G: elegir precede a cualquier descarga.
		blink := m.openGet()
		m.get.input.SetValue(query)
		m.get.input.CursorEnd()
		return m, tea.Batch(blink, m.runGetSearch())
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
	// La foto va ANTES de lanzar nada. Si falla no podemos distinguir lo
	// nuevo ni limpiar intermedios con seguridad; abortar como en la CLI.
	before, err := getter.Snapshot(dir)
	if err != nil {
		return nil, err
	}
	// Archivo de rutas: separa "ya lo tenías" de "no encontré nada", que el
	// diff colapsa (A-07; espejo de downloadOne en cmd/maly). Si no se puede
	// crear se sigue sin él, con el diff solo.
	pathsFile := ""
	if f, err := os.CreateTemp("", "maly-get-*.txt"); err == nil {
		pathsFile = f.Name()
		f.Close()
	}
	m.getGen++ // una nueva descarga invalida la presentación pendiente de otra
	cmd := getter.Command(getter.Opts{Dir: dir, Spec: spec,
		Cookies: m.cfg.Ytdlp.CookiesFromBrowser, PathsFile: pathsFile})
	return execDownload(cmd, func(err error, output []string) tea.Msg {
		return getDoneMsg{err: err, output: output, fromModal: fromModal, dir: dir, before: before, pathsFile: pathsFile}
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
	plan, err := PlanGetPlaylist(m.cfg, args[0], strings.TrimSpace(strings.Join(args[1:], " ")))
	if err != nil {
		m.conErr(err.Error())
		return m, nil
	}

	m.conPrint(m.st.dim.Render(i18n.Tf("cli.get_pl_start", plan.Opts.Spec, plan.MusicDir)))
	m.getGen++ // no mostrar una playlist anterior mientras termina esta
	cmd := getter.Command(plan.Opts)
	return m, execDownload(cmd, func(err error, output []string) tea.Msg {
		return getPlaylistDoneMsg{err: err, output: output, plan: plan}
	})
}

// PlanGetPlaylist es la envoltura de la CONSOLA sobre getter.PlanPlaylist: lo
// único que aporta es leer de la biblioteca los nombres de playlist que ya
// existen, que es lo que el plan necesita para rechazar el choque ANTES de
// tocar la red (G2). La decisión en sí vive en internal/getter, compartida
// con la CLI (A-04).
//
// Exportada por el mismo motivo que ConsoleCommands: para que cmd/maly pueda
// comparar los dos espejos en un test. Ahí la paridad es de NOMBRES; acá es de
// COMPORTAMIENTO, que es la clase de divergencia que A-04 demostró peligrosa y
// que aquella red no podía cazar (O-03, ver cmd/maly/parity_test.go). Sin un
// punto de entrada visible desde fuera, esa comparación no se puede escribir:
// internal/tui no puede importar cmd/maly, y lo de acá no se alcanza sin pasar
// por el runtime de bubbletea.
func PlanGetPlaylist(cfg config.Config, url, name string) (getter.PlaylistPlan, error) {
	var existing []string
	if name != "" {
		// Solo consulta: PlaylistNames abre con Open, que no fabrica la base
		// (A-12), y tolera que todavía no haya ninguna.
		var err error
		if existing, err = library.PlaylistNames(config.DBPath()); err != nil {
			return getter.PlaylistPlan{}, err
		}
	}
	return getter.PlanPlaylist(cfg.MusicPath(), url, name, cfg.Ytdlp.CookiesFromBrowser, existing)
}

// conGetPlaylistFinish corre tras la descarga: re-escanea, resuelve el
// nombre auto-detectado si hacía falta, arma la playlist de maly y avisa al
// demonio. Todo en el mismo tea.Cmd —E/S bloqueante, como conLib/conScan—
// porque son unos pocos pasos secuenciales sin nada que la UI necesite ver
// a medias.
func (m *Model) conGetPlaylistFinish(msg getPlaylistDoneMsg, partial bool) tea.Cmd {
	sock, st, gen := m.sock, m.st, m.getGen
	plan := msg.plan
	return func() tea.Msg {
		// Cualquier salida por error deja el destino como estaba: si lo
		// creamos nosotros y quedó vacío, se va con él (hallazgo G7).
		errLine := func(err error) tea.Msg {
			if plan.CreatedDir {
				os.Remove(plan.Dir)
			}
			return conMsg{lines: []string{st.errSt.Render(err.Error())}}
		}

		c, err := ipc.Dial(sock)
		if err != nil {
			return errLine(err)
		}
		defer c.Close()
		resp, err := c.Do(ipc.Request{Cmd: "scan", Query: plan.MusicDir})
		if err != nil {
			return errLine(err)
		}
		if !resp.OK {
			return errLine(errors.New(resp.Error))
		}

		dir, name, err := plan.Resolve(partial)
		if err != nil {
			return errLine(err)
		}

		// Create: acá se ESCRIBE la playlist (A-12).
		lib, err := library.Create(config.DBPath())
		if err != nil {
			return errLine(err)
		}
		defer lib.Close()
		ids, err := getter.TrackIDs(lib, dir, plan.FilesBefore)
		if err != nil {
			return errLine(err)
		}
		if len(ids) == 0 {
			return errLine(errors.New(i18n.Tf("cli.get_pl_empty", dir)))
		}
		if err := lib.CreatePlaylist(name); err != nil {
			return errLine(err)
		}
		if _, _, err := lib.AddToPlaylist(name, ids); err != nil {
			return errLine(err)
		}
		notifyRefresh()
		tracks, err := lib.PlaylistTracks(name)
		if err != nil {
			return errLine(err)
		}
		summary := i18n.Tf("cli.get_pl_done", name, len(ids))
		if partial {
			summary = i18n.Tf("get.partial", len(tracks)) + " · " + summary
		}
		return conMsg{lines: []string{st.playing.Render(summary)}, reload: true, downloaded: tracks, downloadSummary: summary, downloadGen: gen}
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
		m.overlay = ovNone
		m.overlay = ovLang
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
		out = append(out, style.Render(fmt.Sprintf("%s%3d. %s%s", mark, i+1, ipc.NextLabel(resp.Status, i), t)))
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
	l := m.layoutOf().console
	innerW := l.contentW
	// Se reserva también la celda del cursor al final de la entrada.
	m.conInput.Width = max(1, innerW-lipgloss.Width(m.conInput.Prompt)-1)
	lines := []string{conFit(m.conInput.View(), innerW), m.st.border.Render(strings.Repeat("─", innerW))}
	hintKey := "con.hint"
	if len(m.conLines) > 0 {
		hintKey = "con.hint_output"
	}
	if m.conChoice != "" {
		hintKey = "con.hint_select"
	}
	if m.conPlConfirm != "" {
		hintKey = "con.hint_confirm"
	}
	hints := m.conHintLines(i18n.T(hintKey), innerW, 2)
	for len(hints) < 2 {
		hints = append(hints, "")
	}
	scanning := m.status != nil && m.status.Scanning
	bodyH := max(1, l.height-2-len(lines)-len(hints)-1) // separador del pie
	if scanning {
		bodyH--
	}
	budget := bodyH
	if len(m.conLines) > 0 {
		budget = min(7, max(1, bodyH/2))
	}
	suggestions, count := m.conSuggestionLines(innerW, budget)
	lines = append(lines, suggestions...)
	outputRows := bodyH - len(suggestions)
	output := m.consoleOutput(innerW)
	// Una cabecera separa visualmente los resultados de la búsqueda de acciones.
	if len(output) > 0 && len(suggestions) > 0 && outputRows >= 3 {
		lines = append(lines, m.conHeading("con.output", innerW))
		outputRows--
	}
	outputRows = max(1, outputRows)
	m.conScroll = min(m.conScroll, max(0, len(output)-outputRows))
	end := len(output) - m.conScroll
	start := max(0, end-outputRows)
	for _, line := range output[start:end] {
		lines = append(lines, conFit(line, innerW))
	}
	// El pie y la entrada conservan su posición incluso sin coincidencias.
	for len(lines) < 2+bodyH {
		lines = append(lines, "")
	}
	if scanning {
		txt := i18n.Tf("cli.scan_progress", m.status.ScanSeen)
		if m.status.ScanTotal > 0 {
			txt = i18n.Tf("cli.scan_durations", m.status.ScanSeen, m.status.ScanTotal)
		}
		lines = append(lines, m.st.accent.Render(conFit(txt, innerW)))
	}
	lines = append(lines, m.st.border.Render(strings.Repeat("─", innerW)))
	lines = append(lines, hints...)
	for i, line := range lines {
		lines[i] = strings.Repeat(" ", l.inset) + line
	}
	foot := ""
	if m.conScroll > 0 {
		foot = i18n.Tf("con.output_range", start+1, end, len(output))
	}
	box := m.st.frame(frameLabels{title: i18n.T("con.title"), right: count, foot: foot}, lines, l.width, l.height, true)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
