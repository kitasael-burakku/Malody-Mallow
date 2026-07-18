package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

func (m *Model) openConsole() tea.Cmd {
	m.consoleOpen = true
	m.conInput = textinput.New()
	m.conInput.Prompt = "❯ "
	m.conInput.PromptStyle = m.st.accent
	m.conInput.TextStyle = m.st.text
	m.conInput.Placeholder = i18n.T("con.ph")
	m.conInput.CharLimit = 200
	m.conInput.Focus()
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
		m.conPrint(m.st.accent.Render("❯ ") + m.st.text.Render(line))
		return m.execConsole(line)
	}
	var cmd tea.Cmd
	m.conInput, cmd = m.conInput.Update(msg)
	return m, cmd
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
		return m, nil
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
		{"version", i18n.T("cli.version_cmd")},
		{"update", i18n.T("cli.update")},
		{"kill", i18n.T("cli.kill")},
	}
	for _, r := range rows {
		m.conPrint("  " + m.st.accent.Render(padTo(r[0], 22)) + m.st.text.Render(r[1]))
	}
	m.conPrint(m.st.dim.Render(i18n.T("con.help_local")))
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
	cmd := getter.Command(dir, spec, m.cfg.Ytdlp.CookiesFromBrowser)
	return m, tea.ExecProcess(cmd, func(err error) tea.Msg { return getDoneMsg{err: err} })
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
// usa el terminal).
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

	lines := []string{m.conInput.View(), m.st.dim.Render(strings.Repeat("─", innerW))}
	out := m.conLines
	if len(out) > maxRows {
		out = out[len(out)-maxRows:]
	}
	for _, l := range out {
		lines = append(lines, clip(l, innerW))
	}
	lines = append(lines, m.st.dim.Render(clip("  "+i18n.T("con.hint"), innerW)))

	box := m.st.panel(i18n.T("con.title"), lines, w, len(lines)+2, true)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
