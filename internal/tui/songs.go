package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"

	"maly/internal/i18n"
	"maly/internal/ipc"
	"maly/internal/library"
)

// El selector de canciones (ctrl+o) busca con fuzzy sobre toda la biblioteca;
// su única responsabilidad es elegir canciones (enter reproduce, tab agrega).

type songItem struct {
	label  string // lo que se muestra
	folded string // texto normalizado sobre el que se hace fuzzy match
	path   string // ruta del archivo
}

type songSource []songItem

func (p songSource) String(i int) string { return p[i].folded }
func (p songSource) Len() int            { return len(p) }

// buildSongs regenera las entradas desde la biblioteca cargada.
func (m *Model) buildSongs() {
	items := make([]songItem, 0, len(m.tree.all))
	for _, t := range m.tree.all {
		label := t.Title
		if t.Artist != "" {
			label = t.Artist + " — " + t.Title
		}
		if t.Album != "" {
			label += "  [" + t.Album + "]"
		}
		items = append(items, songItem{
			label:  label,
			folded: library.Fold(label),
			path:   t.Path,
		})
	}
	m.songItems = items
	m.filterSongs()
}

// filterSongs recalcula los resultados según el texto del input.
func (m *Model) filterSongs() {
	q := strings.TrimSpace(library.Fold(m.songInput.Value()))
	m.songMatches = m.songMatches[:0]
	if q == "" {
		for i := range m.songItems {
			m.songMatches = append(m.songMatches, i)
		}
	} else {
		for _, r := range fuzzy.FindFrom(q, songSource(m.songItems)) {
			m.songMatches = append(m.songMatches, r.Index)
		}
	}
	if m.songCursor >= len(m.songMatches) {
		m.songCursor = len(m.songMatches) - 1
	}
	if m.songCursor < 0 {
		m.songCursor = 0
	}
}

func (m *Model) openSongs() tea.Cmd {
	m.songsOpen = true
	m.songInput = textinput.New()
	m.songInput.Prompt = "❯ "
	m.songInput.PromptStyle = m.st.accent
	m.songInput.TextStyle = m.st.text
	m.songInput.Placeholder = i18n.T("songs.ph")
	m.songInput.Focus()
	m.songCursor = 0
	m.buildSongs()
	return textinput.Blink
}

func (m *Model) handleSongsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", m.keys["songs"]:
		m.songsOpen = false
		return m, nil
	case "up", "ctrl+k":
		if m.songCursor > 0 {
			m.songCursor--
		}
		return m, nil
	case "down", "ctrl+j":
		if m.songCursor < len(m.songMatches)-1 {
			m.songCursor++
		}
		return m, nil
	case "enter":
		if m.songCursor < len(m.songMatches) {
			it := m.songItems[m.songMatches[m.songCursor]]
			m.songsOpen = false
			return m, m.req(ipc.Request{Cmd: "playnow", Paths: []string{it.path}})
		}
		return m, nil
	case "tab": // agrega a la cola sin cerrar el selector
		if m.songCursor < len(m.songMatches) {
			it := m.songItems[m.songMatches[m.songCursor]]
			m.setFlash(i18n.Tf("songs.added", it.label), false)
			return m, m.req(ipc.Request{Cmd: "add", Paths: []string{it.path}})
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.songInput, cmd = m.songInput.Update(msg)
	m.filterSongs()
	return m, cmd
}

func (m *Model) songsView() string {
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

	lines := []string{m.songInput.View(), m.st.dim.Render(strings.Repeat("─", innerW))}
	if len(m.songMatches) == 0 {
		lines = append(lines, m.st.dim.Render(i18n.T("songs.none")))
	}
	start := 0
	if m.songCursor >= maxRows {
		start = m.songCursor - maxRows + 1
	}
	end := start + maxRows
	if end > len(m.songMatches) {
		end = len(m.songMatches)
	}
	for i := start; i < end; i++ {
		it := m.songItems[m.songMatches[i]]
		line := clip("  "+it.label, innerW)
		if i == m.songCursor {
			line = m.st.selected.Render(padTo(line, innerW))
		} else {
			line = m.st.text.Render(line)
		}
		lines = append(lines, line)
	}
	lines = append(lines, m.st.dim.Render(clip("  "+fmt.Sprintf(i18n.T("songs.hint"), len(m.songMatches)), innerW)))

	box := m.st.panel(i18n.T("songs.title"), lines, w, len(lines)+2, true)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
