package tui

import (
	"errors"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"maly/internal/config"
	"maly/internal/getter"
	"maly/internal/i18n"
	"maly/internal/ipc"
	"maly/internal/library"
	"maly/internal/safetext"
)

// downloadState es la fase local del mismo selector: deja de buscar URLs y
// ofrece únicamente archivos obtenidos. Ninguna transición reproduce sola.
type downloadState struct {
	outcome getter.Outcome
	dir     string
	summary string
	loading bool
	err     string
}

type downloadIndexedMsg struct {
	gen    int
	tracks []library.Track
	err    error
}

func (m *Model) finishDownload(msg getDoneMsg) (tea.Model, tea.Cmd) {
	defer os.Remove(msg.pathsFile)
	out, err := getter.Collect(msg.dir, msg.before, msg.pathsFile, msg.err)
	if err != nil {
		m.conErr(err.Error())
		// Mantener el error en una superficie visible, sin depender del flash.
		if msg.fromModal {
			m.openConsole()
			m.setFlash(err.Error(), true)
		}
		return m, nil
	}
	m.openGet()
	// La nueva operación reemplaza avisos efímeros de acciones anteriores.
	m.flash = ""
	m.get.noFilter = false
	m.get.input.Placeholder = i18n.T("songs.ph")
	m.getDownload = &downloadState{outcome: out, dir: msg.dir, summary: out.Summary()}
	m.conPrint(m.st.dim.Render(out.Summary()))
	if msg.err != nil {
		m.conErr(i18n.Tf("cli.get_err", msg.err))
	}
	return m, m.indexDownload()
}

func (m *Model) indexDownload() tea.Cmd {
	d := m.getDownload
	d.loading = true
	d.err = ""
	m.get.setItems(nil)
	m.getGen++
	gen, sock, dir, dbPath := m.getGen, m.sock, d.dir, config.DBPath()
	paths := append([]string(nil), d.outcome.Paths...)
	return func() tea.Msg {
		result := downloadIndexedMsg{gen: gen}
		c, err := ipc.Dial(sock)
		if err == nil {
			var resp ipc.Response
			resp, err = c.Do(ipc.Request{Cmd: "scan", Query: dir})
			c.Close()
			if err == nil && !resp.OK {
				err = errors.New(resp.Error)
			}
		}
		if err == nil {
			result.tracks, err = getter.Indexed(dbPath, paths)
		}
		result.err = err
		return result
	}
}

func (m *Model) applyDownloadIndexed(msg downloadIndexedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.conErr(i18n.T("get.scan_failed") + ": " + msg.err.Error())
	}
	// Cerrar el selector o empezar otra descarga invalida la presentación, no
	// el escaneo ya enviado. Su respuesta tardía no reabre ni pisa otra pantalla.
	if m.overlay != ovGet || m.getDownload == nil || msg.gen != m.getGen {
		return m, nil
	}
	d := m.getDownload
	d.loading = false
	if msg.err != nil {
		d.err = i18n.T("get.scan_failed") + "\n" + msg.err.Error()
		return m, nil
	}
	m.get.setItems(downloadItems(msg.tracks))
	m.conPrint(m.st.playing.Render(i18n.Tf("get.indexed", len(msg.tracks))))
	return m, loadLibrary
}

// showDownloaded reutiliza la fase local para una playlist que ya terminó su
// escaneo y su creación. El mensaje de éxito parcial permanece en el panel.
func (m *Model) showDownloaded(tracks []library.Track, summary string) {
	m.openGet()
	// La nueva operación reemplaza avisos efímeros de acciones anteriores.
	m.flash = ""
	m.get.noFilter = false
	m.get.input.Placeholder = i18n.T("songs.ph")
	m.getDownload = &downloadState{summary: summary}
	m.get.setItems(downloadItems(tracks))
}

func downloadItems(tracks []library.Track) []pickerItem {
	items := songItems(tracks)
	for i, t := range tracks {
		// La identidad real viaja en value. El nombre ayuda a distinguir dos
		// grabaciones cuyo título y artista coinciden.
		items[i].label = safetext.Clean(filepath.Base(t.Path)) + " · " + items[i].label
	}
	return items
}

func (m *Model) handleDownloadedKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	d := m.getDownload
	switch msg.String() {
	case "esc", m.keys["get"]:
		m.closeGet()
		return m, nil
	case "ctrl+r":
		if d.err != "" && !d.loading {
			return m, m.indexDownload()
		}
		return m, nil
	case "enter", "tab":
		if d.loading || d.err != "" {
			return m, nil
		}
		if it, ok := m.get.current(); ok {
			cmd := "add"
			if msg.String() == "enter" {
				cmd = "playnow"
				m.closeGet()
			}
			return m, m.req(ipc.Request{Cmd: cmd, Paths: []string{it.value}})
		}
		return m, nil
	}
	return m, m.get.handleKey(msg)
}

func (m *Model) downloadView() string {
	d := m.getDownload
	w := pickerWidthMax(m.width, getPickerWidth)
	rows := m.height - 10
	if rows > 14 {
		rows = 14
	}
	m.get.notice = d.summary
	m.get.loading = d.loading
	m.get.loadErr = d.err
	m.get.emptyText = i18n.T("get.no_files")
	hint := i18n.T("get.local_hint")
	if d.loading {
		hint = i18n.T("get.scanning_hint")
	} else if d.err != "" {
		hint = i18n.T("get.retry_hint")
	}
	box := m.get.render(i18n.T("get.local_title"), hint, w, rows)
	box = m.withFlash(box)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
