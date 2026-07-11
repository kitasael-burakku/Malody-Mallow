package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"maly/internal/i18n"
	"maly/internal/ipc"
	"maly/internal/library"
)

// El selector de canciones (ctrl+o) es un picker fuzzy sobre toda la
// biblioteca; su única responsabilidad es elegir canciones (enter reproduce,
// tab agrega). `maly select` reutiliza estas mismas entradas.

// songItems convierte pistas en entradas del picker (label visible, ruta
// como valor).
func songItems(tracks []library.Track) []pickerItem {
	items := make([]pickerItem, 0, len(tracks))
	for _, t := range tracks {
		label := t.String()
		if t.Album != "" {
			label += "  [" + t.Album + "]"
		}
		items = append(items, newPickerItem(label, t.Path))
	}
	return items
}

func (m *Model) openSongs() tea.Cmd {
	m.songsOpen = true
	m.songs = newPicker(m.st, i18n.T("songs.ph"))
	m.songs.setItems(songItems(m.tree.all))
	return textinput.Blink
}

func (m *Model) handleSongsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", m.keys["songs"]:
		m.songsOpen = false
		return m, nil
	case "enter":
		if it, ok := m.songs.current(); ok {
			m.songsOpen = false
			return m, m.req(ipc.Request{Cmd: "playnow", Paths: []string{it.value}})
		}
		return m, nil
	case "tab": // agrega a la cola sin cerrar el selector
		if it, ok := m.songs.current(); ok {
			m.setFlash(i18n.Tf("songs.added", it.label), false)
			return m, m.req(ipc.Request{Cmd: "add", Paths: []string{it.value}})
		}
		return m, nil
	}
	return m, m.songs.handleKey(msg)
}

func (m *Model) songsView() string {
	w := pickerWidth(m.width)
	maxRows := m.height - 10
	if maxRows > 14 {
		maxRows = 14
	}
	hint := fmt.Sprintf(i18n.T("songs.hint"), len(m.songs.matches))
	box := m.songs.render(i18n.T("songs.title"), hint, w, maxRows)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
