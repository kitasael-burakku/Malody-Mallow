package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"maly/internal/config"
	"maly/internal/i18n"
	"maly/internal/ipc"
	"maly/internal/library"
)

// El panel de playlists (ctrl+l) es un picker fuzzy sobre las playlists
// guardadas. Tiene dos modos: navegación (enter reproduce reemplazando la
// cola, tab encola al final, ctrl+n crea con el texto escrito, ctrl+x borra)
// y destino (abierto con `playlist_add` desde biblioteca o cola: enter agrega
// las pistas seleccionadas a la playlist elegida). Las operaciones van
// directo a SQLite —igual que la CLI— salvo reproducir/encolar, que hablan
// con el servicio.

type plMode int

const (
	plBrowse plMode = iota
	plTarget
)

// plListMsg trae las playlists recargadas de la base.
type plListMsg struct {
	items []pickerItem
	err   error
}

// plActMsg es el resultado de una operación de escritura (create/delete/add).
type plActMsg struct {
	msg    string
	err    error
	reload bool // la lista cambió: recargarla si el panel sigue abierto
}

func (m *Model) openPlaylists(mode plMode, pending []int64) tea.Cmd {
	m.plOpen = true
	m.plMode = mode
	m.plPending = pending
	m.pl = newPicker(m.st, i18n.T("plsel.ph"))
	return tea.Batch(textinput.Blink, loadPlaylists)
}

// loadPlaylists lee las playlists con su número de pistas (apertura
// transitoria de la DB, como loadLibrary).
func loadPlaylists() tea.Msg {
	lib, err := library.Open(config.DBPath())
	if err != nil {
		return plListMsg{err: err}
	}
	defer lib.Close()
	lists, err := lib.Playlists()
	if err != nil {
		return plListMsg{err: err}
	}
	items := make([]pickerItem, 0, len(lists))
	for _, p := range lists {
		items = append(items, newPickerItem(fmt.Sprintf("%s  (%d)", p.Name, p.Tracks), p.Name))
	}
	return plListMsg{items: items}
}

func (m *Model) handlePlaylistsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", m.keys["playlists"]:
		m.plOpen = false
		return m, nil
	case "enter":
		it, ok := m.pl.current()
		if !ok {
			return m, nil
		}
		m.plOpen = false
		if m.plMode == plTarget {
			return m, plAddCmd(it.value, m.plPending, false)
		}
		return m, m.req(ipc.Request{Cmd: "playlist_play", Value: it.value})
	case "tab":
		if m.plMode != plBrowse {
			return m, nil
		}
		if it, ok := m.pl.current(); ok {
			m.setFlash(i18n.Tf("plsel.queued", it.value), false)
			return m, plQueueCmd(m.sock, it.value)
		}
		return m, nil
	case "ctrl+n":
		name := m.pl.input.Value()
		if name == "" {
			return m, nil
		}
		if m.plMode == plTarget {
			m.plOpen = false
			return m, plAddCmd(name, m.plPending, true)
		}
		m.pl.input.SetValue("")
		m.pl.filter()
		return m, plCreateCmd(name)
	case "ctrl+x":
		if m.plMode != plBrowse {
			return m, nil
		}
		if it, ok := m.pl.current(); ok {
			return m, plDeleteCmd(it.value)
		}
		return m, nil
	}
	return m, m.pl.handleKey(msg)
}

// plQueueCmd agrega las pistas de la playlist al final de la cola (lee las
// rutas de la DB y las manda al servicio como un add normal).
func plQueueCmd(sock, name string) tea.Cmd {
	return func() tea.Msg {
		lib, err := library.Open(config.DBPath())
		if err != nil {
			return actionMsg{err: err}
		}
		tracks, err := lib.PlaylistTracks(name)
		lib.Close()
		if err != nil {
			return actionMsg{err: err}
		}
		paths := make([]string, len(tracks))
		for i, t := range tracks {
			paths[i] = t.Path
		}
		c, err := ipc.Dial(sock)
		if err != nil {
			return actionMsg{err: err}
		}
		defer c.Close()
		resp, err := c.Do(ipc.Request{Cmd: "add", Paths: paths})
		return actionMsg{resp: resp, err: err}
	}
}

// plAddCmd agrega pistas a una playlist; con create primero la crea (ctrl+n
// en modo destino).
func plAddCmd(name string, ids []int64, create bool) tea.Cmd {
	return func() tea.Msg {
		lib, err := library.Open(config.DBPath())
		if err != nil {
			return plActMsg{err: err}
		}
		defer lib.Close()
		if create {
			if err := lib.CreatePlaylist(name); err != nil {
				return plActMsg{err: err}
			}
		}
		if err := lib.AddToPlaylist(name, ids); err != nil {
			return plActMsg{err: err}
		}
		return plActMsg{msg: i18n.Tf("pl.added", len(ids), name)}
	}
}

func plCreateCmd(name string) tea.Cmd {
	return func() tea.Msg {
		lib, err := library.Open(config.DBPath())
		if err != nil {
			return plActMsg{err: err}
		}
		defer lib.Close()
		if err := lib.CreatePlaylist(name); err != nil {
			return plActMsg{err: err}
		}
		return plActMsg{msg: i18n.Tf("pl.created", name), reload: true}
	}
}

func plDeleteCmd(name string) tea.Cmd {
	return func() tea.Msg {
		lib, err := library.Open(config.DBPath())
		if err != nil {
			return plActMsg{err: err}
		}
		defer lib.Close()
		if err := lib.DeletePlaylist(name); err != nil {
			return plActMsg{err: err}
		}
		return plActMsg{msg: i18n.Tf("pl.deleted", name), reload: true}
	}
}

func (m *Model) plView() string {
	w := pickerWidth(m.width)
	maxRows := m.height - 10
	if maxRows > 14 {
		maxRows = 14
	}
	var hint string
	if m.plMode == plTarget {
		hint = fmt.Sprintf(i18n.T("plsel.hint_add"), len(m.pl.matches), len(m.plPending))
	} else {
		hint = fmt.Sprintf(i18n.T("plsel.hint"), len(m.pl.matches))
	}
	if len(m.pl.items) == 0 && m.pl.input.Value() == "" {
		hint = i18n.T("plsel.empty")
	}
	box := m.pl.render(i18n.T("plsel.title"), hint, w, maxRows)
	// A diferencia de los otros modales, aquí los flashes importan sin cerrar
	// (creada/borrada/error): se muestran bajo el panel.
	if m.flash != "" && time.Now().Before(m.flashUntil) {
		st := m.st.playing
		if m.flashErr {
			st = m.st.errSt
		}
		box = lipgloss.JoinVertical(lipgloss.Center, box, st.Render(m.flash))
	}
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
