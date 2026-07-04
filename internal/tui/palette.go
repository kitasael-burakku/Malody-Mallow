package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"

	"maly/internal/ipc"
	"maly/internal/library"
)

// palItem es una entrada de la paleta: un comando de la app o una canción.
type palItem struct {
	label  string // lo que se muestra
	folded string // texto normalizado sobre el que se hace fuzzy match
	action string // comandos: acción interna; canciones: ""
	path   string // canciones: ruta del archivo
}

type palSource []palItem

func (p palSource) String(i int) string { return p[i].folded }
func (p palSource) Len() int            { return len(p) }

var paletteCommands = []palItem{
	{label: "⏯ Play/Pause", action: "toggle"},
	{label: "⏭ Siguiente", action: "next"},
	{label: "⏮ Anterior", action: "prev"},
	{label: "⏹ Detener", action: "stop"},
	{label: "⇄ Alternar shuffle", action: "shuffle"},
	{label: "⟲ Alternar repeat", action: "repeat"},
	{label: "♫ Vaciar cola", action: "clear"},
	{label: "⟳ Reescanear biblioteca", action: "rescan"},
	{label: "▁ Alternar visualizador", action: "viz"},
	{label: "? Ayuda", action: "help"},
	{label: "⏻ Salir de maly", action: "quit"},
}

// buildPalette regenera las entradas (comandos + biblioteca completa).
func (m *Model) buildPalette() {
	items := make([]palItem, 0, len(paletteCommands)+len(m.tree.all))
	for _, c := range paletteCommands {
		c.folded = library.Fold(c.label)
		items = append(items, c)
	}
	for _, t := range m.tree.all {
		label := t.Title
		if t.Artist != "" {
			label = t.Artist + " — " + t.Title
		}
		if t.Album != "" {
			label += "  [" + t.Album + "]"
		}
		items = append(items, palItem{
			label:  label,
			folded: library.Fold(label),
			path:   t.Path,
		})
	}
	m.palItems = items
	m.filterPalette()
}

// filterPalette recalcula los resultados según el texto del input.
func (m *Model) filterPalette() {
	q := strings.TrimSpace(library.Fold(m.palInput.Value()))
	m.palMatches = m.palMatches[:0]
	if q == "" {
		for i := range m.palItems {
			m.palMatches = append(m.palMatches, i)
		}
	} else {
		for _, r := range fuzzy.FindFrom(q, palSource(m.palItems)) {
			m.palMatches = append(m.palMatches, r.Index)
		}
	}
	if m.palCursor >= len(m.palMatches) {
		m.palCursor = len(m.palMatches) - 1
	}
	if m.palCursor < 0 {
		m.palCursor = 0
	}
}

func (m *Model) openPalette() tea.Cmd {
	m.paletteOpen = true
	m.palInput = textinput.New()
	m.palInput.Prompt = "❯ "
	m.palInput.PromptStyle = m.st.accent
	m.palInput.TextStyle = m.st.text
	m.palInput.Placeholder = "comando o canción…"
	m.palInput.Focus()
	m.palCursor = 0
	m.buildPalette()
	return textinput.Blink
}

func (m *Model) handlePaletteKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", m.keys["palette"]:
		m.paletteOpen = false
		return m, nil
	case "up", "ctrl+k":
		if m.palCursor > 0 {
			m.palCursor--
		}
		return m, nil
	case "down", "ctrl+j":
		if m.palCursor < len(m.palMatches)-1 {
			m.palCursor++
		}
		return m, nil
	case "enter":
		return m.paletteExec(false)
	case "tab":
		return m.paletteExec(true)
	}
	var cmd tea.Cmd
	m.palInput, cmd = m.palInput.Update(msg)
	m.filterPalette()
	return m, cmd
}

// paletteExec ejecuta la entrada seleccionada. addOnly (Tab) agrega la
// canción a la cola sin cerrar la paleta.
func (m *Model) paletteExec(addOnly bool) (tea.Model, tea.Cmd) {
	if m.palCursor >= len(m.palMatches) {
		return m, nil
	}
	it := m.palItems[m.palMatches[m.palCursor]]

	if it.action == "" { // canción
		if addOnly {
			m.setFlash("agregada: "+it.label, false)
			return m, m.req(ipc.Request{Cmd: "add", Paths: []string{it.path}})
		}
		m.paletteOpen = false
		return m, m.req(ipc.Request{Cmd: "playnow", Paths: []string{it.path}})
	}
	if addOnly { // Tab solo tiene sentido sobre canciones
		return m, nil
	}
	m.paletteOpen = false
	switch it.action {
	case "toggle", "next", "prev", "stop", "shuffle", "repeat", "clear":
		return m, m.req(ipc.Request{Cmd: it.action})
	case "rescan":
		m.setFlash("Escaneando biblioteca…", false)
		return m, m.rescan()
	case "viz":
		m.vizOn = !m.vizOn
		return m, nil
	case "help":
		m.showHelp = true
		return m, nil
	case "quit":
		return m, tea.Quit
	}
	return m, nil
}

type rescanMsg struct {
	resp ipc.Response
	err  error
}

// rescan pide el escaneo al demonio y, al terminar, recarga la biblioteca.
func (m *Model) rescan() tea.Cmd {
	sock := m.sock
	return func() tea.Msg {
		c, err := ipc.Dial(sock)
		if err != nil {
			return rescanMsg{err: err}
		}
		defer c.Close()
		resp, err := c.Do(ipc.Request{Cmd: "scan"})
		return rescanMsg{resp: resp, err: err}
	}
}

func (m *Model) paletteView() string {
	w := m.width * 2 / 3
	if w < 50 {
		w = m.width - 4
	}
	if w > 80 {
		w = 80
	}
	innerW := w - 2
	maxRows := m.height - 10
	if maxRows > 14 {
		maxRows = 14
	}
	if maxRows < 3 {
		maxRows = 3
	}

	lines := []string{m.palInput.View(), m.st.dim.Render(strings.Repeat("─", innerW))}
	if len(m.palMatches) == 0 {
		lines = append(lines, m.st.dim.Render("  sin coincidencias"))
	}
	start := 0
	if m.palCursor >= maxRows {
		start = m.palCursor - maxRows + 1
	}
	end := start + maxRows
	if end > len(m.palMatches) {
		end = len(m.palMatches)
	}
	for i := start; i < end; i++ {
		it := m.palItems[m.palMatches[i]]
		style := m.st.text
		if it.action != "" {
			style = m.st.accent
		}
		line := clip("  "+it.label, innerW)
		if i == m.palCursor {
			line = m.st.selected.Render(padTo(line, innerW))
		} else {
			line = style.Render(line)
		}
		lines = append(lines, line)
	}
	lines = append(lines, m.st.dim.Render(clip(fmt.Sprintf("  %d resultado(s) · enter ejecuta · tab agrega a la cola · esc cierra",
		len(m.palMatches)), innerW)))

	box := m.st.panel("Paleta", lines, w, len(lines)+2, true)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
