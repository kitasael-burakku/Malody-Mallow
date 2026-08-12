package tui

import (
	"errors"
	"fmt"
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
type getDoneMsg struct{ err error }

// getPlaylistDoneMsg vuelve de yt-dlp para `get playlist` (tea.ExecProcess).
// musicDir/name/dir/before llevan lo que conGetPlaylist ya decidió antes de
// lanzar el proceso: ExecProcess solo puede devolver el error, así que el
// resto del estado tiene que viajar en el mensaje. name == "" significa que
// no había nombre explícito y dir/before son los datos para el diffing.
type getPlaylistDoneMsg struct {
	err      error
	musicDir string
	name     string
	dir      string
	before   map[string]bool
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
	return func() tea.Msg {
		c, err := ipc.Dial(sock)
		if err != nil {
			return conMsg{lines: []string{st.errSt.Render(err.Error())}}
		}
		defer c.Close()
		c.Timeout = 10 * time.Minute // una biblioteca grande no cabe en los 30 s default
		resp, err := c.Do(ipc.Request{Cmd: "scan", Query: query})
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
	if err := getter.Tools(); err != nil {
		m.conErr(err.Error())
		return m, nil
	}
	dir := m.cfg.MusicPath()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		m.conErr(err.Error())
		return m, nil
	}
	spec := getter.Spec(strings.Join(args, " "))
	m.conPrint(m.st.dim.Render(i18n.Tf("cli.get_start", spec, dir)))
	cmd := getter.Command(getter.Opts{Dir: dir, Spec: spec, Cookies: m.cfg.Ytdlp.CookiesFromBrowser})
	return m, tea.ExecProcess(cmd, func(err error) tea.Msg { return getDoneMsg{err: err} })
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
	url := args[0]
	name := strings.TrimSpace(strings.Join(args[1:], " "))

	musicDir := m.cfg.MusicPath()
	if err := os.MkdirAll(musicDir, 0o755); err != nil {
		m.conErr(err.Error())
		return m, nil
	}

	opts := getter.Opts{Dir: musicDir, Spec: url, Cookies: m.cfg.Ytdlp.CookiesFromBrowser, Playlist: true}
	var dir string
	var before map[string]bool
	if name != "" {
		// Nombre explícito: validarlo como componente de ruta ANTES de
		// tocar el filesystem o la red — es entrada del usuario
		// volviéndose ruta.
		if filepath.Base(name) != name || name == "." || name == ".." {
			m.conErr(i18n.Tf("cli.get_pl_bad_name", name))
			return m, nil
		}
		dir = filepath.Join(musicDir, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			m.conErr(err.Error())
			return m, nil
		}
		opts.Dir = dir
	} else {
		opts.PlaylistSubdir = true
		var err error
		before, err = dirEntries(musicDir)
		if err != nil {
			m.conErr(err.Error())
			return m, nil
		}
	}

	m.conPrint(m.st.dim.Render(i18n.Tf("cli.get_pl_start", url, musicDir)))
	cmd := getter.Command(opts)
	return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
		return getPlaylistDoneMsg{err: err, musicDir: musicDir, name: name, dir: dir, before: before}
	})
}

// conGetPlaylistFinish corre tras la descarga: re-escanea, resuelve el
// nombre auto-detectado si hacía falta, arma la playlist de maly y avisa al
// demonio. Todo en el mismo tea.Cmd —E/S bloqueante, como conLib/conScan—
// porque son unos pocos pasos secuenciales sin nada que la UI necesite ver
// a medias.
func (m *Model) conGetPlaylistFinish(musicDir, name, dir string, before map[string]bool) tea.Cmd {
	sock, st := m.sock, m.st
	return func() tea.Msg {
		errLine := func(err error) tea.Msg { return conMsg{lines: []string{st.errSt.Render(err.Error())}} }

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
			return conMsg{lines: []string{st.errSt.Render(resp.Error)}}
		}

		if name == "" {
			found, err := newDirEntry(musicDir, before)
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
		entries, err := os.ReadDir(dir)
		if err != nil {
			return errLine(err)
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		var ids []int64
		for _, e := range entries {
			if e.IsDir() || !library.IsAudio(e.Name()) {
				continue
			}
			if t, ok := lib.ByPath(filepath.Join(dir, e.Name())); ok {
				ids = append(ids, t.ID)
			}
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

// dirEntries lista los nombres de entrada de dir (no recursivo). Duplica el
// helper homónimo de cmd/maly/get.go: internal/tui no puede importar
// package main.
func dirEntries(dir string) (map[string]bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	m := make(map[string]bool, len(entries))
	for _, e := range entries {
		m[e.Name()] = true
	}
	return m, nil
}

// newDirEntry devuelve el único subdirectorio de parent que no estaba en
// before: el que yt-dlp acaba de crear con el título de la playlist. Falla
// si no hay exactamente uno nuevo — ambiguo, mejor pedir un nombre explícito
// que adivinar mal. Duplica el helper homónimo de cmd/maly/get.go.
func newDirEntry(parent string, before map[string]bool) (string, error) {
	entries, err := os.ReadDir(parent)
	if err != nil {
		return "", err
	}
	var found string
	n := 0
	for _, e := range entries {
		if !e.IsDir() || before[e.Name()] {
			continue
		}
		found = e.Name()
		n++
	}
	if n != 1 {
		return "", errors.New(i18n.T("cli.get_pl_ambiguous"))
	}
	return found, nil
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
	if s.Track == nil {
		return []string{st.dim.Render(i18n.Tf("st.stopped",
			s.Volume, ipc.OnOff(s.Shuffle), s.Repeat, s.QueueLen))}
	}
	icon := "▶"
	if s.Paused {
		icon = "⏸"
	}
	name := s.Track.String()
	if s.Track.Album != "" {
		name += " [" + s.Track.Album + "]"
	}
	return []string{
		st.text.Render(icon + " " + name),
		st.dim.Render(i18n.Tf("st.line2", ipc.FmtTime(s.Position), ipc.FmtTime(s.Duration),
			s.Volume, ipc.OnOff(s.Shuffle), s.Repeat, s.QueueIndex+1, s.QueueLen)),
	}
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
