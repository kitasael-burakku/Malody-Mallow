package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"maly/internal/i18n"
	"maly/internal/ipc"
)

// La paleta de comandos (ctrl+p) es una consola integrada: se escriben
// comandos estilo CLI ("maly next", "vol +5", "queue"…) y la salida se
// muestra dentro de la propia paleta sin salir de ella.

// conMsg trae la salida de un comando ejecutado contra el demonio.
type conMsg struct {
	lines     []string
	reloadLib bool // tras scan: recargar la biblioteca de la TUI
}

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

func (m *Model) conErr(text string) { m.conPrint(m.st.errSt.Render(text)) }

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
		return m, m.conQuery("status", func(st styles, resp ipc.Response) []string {
			return statusLines(st, resp.Status)
		})
	case "queue":
		return m, m.conQuery("queue", queueLines)
	case "scan", "rescan":
		m.conPrint(m.st.dim.Render(i18n.T("con.scanning")))
		return m, m.conScan()
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
		{"scan", i18n.T("cli.scan")},
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
func (m *Model) conQuery(cmd string, format func(styles, ipc.Response) []string) tea.Cmd {
	sock, st := m.sock, m.st
	return func() tea.Msg {
		c, err := ipc.Dial(sock)
		if err != nil {
			return conMsg{lines: []string{st.errSt.Render(err.Error())}}
		}
		defer c.Close()
		resp, err := c.Do(ipc.Request{Cmd: cmd})
		if err != nil {
			return conMsg{lines: []string{st.errSt.Render(err.Error())}}
		}
		if !resp.OK {
			return conMsg{lines: []string{st.errSt.Render(resp.Error)}}
		}
		return conMsg{lines: format(st, resp)}
	}
}

func (m *Model) conScan() tea.Cmd {
	sock, st := m.sock, m.st
	return func() tea.Msg {
		c, err := ipc.Dial(sock)
		if err != nil {
			return conMsg{lines: []string{st.errSt.Render(err.Error())}}
		}
		defer c.Close()
		resp, err := c.Do(ipc.Request{Cmd: "scan"})
		if err != nil {
			return conMsg{lines: []string{st.errSt.Render(err.Error())}}
		}
		if !resp.OK {
			return conMsg{lines: []string{st.errSt.Render(resp.Error)}}
		}
		return conMsg{lines: []string{st.playing.Render(resp.Msg)}, reloadLib: true}
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

func (m *Model) consoleView() string {
	w := pickerWidth(m.width)
	innerW := w - 2
	maxRows := m.height - 10
	if maxRows > 16 {
		maxRows = 16
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
