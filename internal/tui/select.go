package tui

import (
	"errors"
	"fmt"
	"io/fs"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"maly/internal/config"
	"maly/internal/i18n"
	"maly/internal/ipc"
	"maly/internal/library"
)

// RunSelect implementa `maly select`: el mismo picker fuzzy de ctrl+o pero
// como mini modal inline en el terminal, sin abrir la TUI completa. enter
// reproduce, tab agrega a la cola, esc/ctrl+c cancela. Requiere el servicio
// corriendo (como cualquier comando de reproducción).
func RunSelect(cfg config.Config) error {
	sock := config.SocketPath()
	if !ipc.Ping(sock) {
		return errors.New(i18n.T("cli.no_daemon"))
	}
	// Un comando de solo lectura no debe fabricar el estado que está a punto
	// de reportar como "biblioteca vacía" (hallazgo UX-N5), así que se abre
	// con library.Open, que no crea nada.
	//
	// Y acá SÍ hay que distinguir los dos fallos, que es para lo que Open
	// devuelve un error tipado: "no existe" merece el remedio "corré maly
	// scan", pero una base corrupta, bloqueada o sin permisos NO — decirle a
	// alguien que su biblioteca está vacía cuando el problema es otro es un
	// remedio equivocado. Antes esto exigía un os.Stat propio, porque el
	// OpenIfExists de entonces conflaba los dos casos (A-12).
	lib, err := library.Open(config.DBPath())
	if errors.Is(err, fs.ErrNotExist) {
		return errors.New(i18n.T("tui.lib_empty_flash"))
	}
	if err != nil {
		return err
	}
	tracks, err := lib.All()
	lib.Close()
	if err != nil {
		return err
	}
	if len(tracks) == 0 {
		return errors.New(i18n.T("tui.lib_empty_flash"))
	}

	st := newStyles(cfg.Theme)
	m := &selectModel{st: st, sock: sock, pk: newPicker(st, i18n.T("songs.ph"))}
	m.pk.setItems(songItems(tracks))

	out, err := tea.NewProgram(m).Run()
	if err != nil {
		return err
	}
	final := out.(*selectModel)
	if final.errMsg != "" {
		return errors.New(final.errMsg)
	}
	if final.msg != "" {
		fmt.Println(final.msg)
	}
	return nil
}

type selectModel struct {
	st            styles
	sock          string
	pk            *picker
	width, height int
	flash         string // resultado del último tab, mostrado en el pie
	done          bool   // limpia la vista antes de salir
	msg           string // respuesta final del servicio, impresa en stdout
	errMsg        string
}

// selDoneMsg cierra el selector con el resultado de enter; selAddMsg deja el
// selector abierto y muestra el resultado de tab en el pie.
type selDoneMsg struct {
	msg string
	err error
}
type selAddMsg struct {
	msg string
	err error
}

func (m *selectModel) Init() tea.Cmd { return textinput.Blink }

func (m *selectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case selDoneMsg:
		m.done = true
		if msg.err != nil {
			m.errMsg = msg.err.Error()
		} else {
			m.msg = msg.msg
		}
		return m, tea.Quit

	case selAddMsg:
		if msg.err != nil {
			m.flash = m.st.errSt.Render(msg.err.Error())
		} else {
			m.flash = m.st.playing.Render(msg.msg)
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.done = true
			return m, tea.Quit
		case "enter":
			if it, ok := m.pk.current(); ok {
				return m, m.send(ipc.Request{Cmd: "playnow", Paths: []string{it.value}}, true)
			}
			return m, nil
		case "tab":
			if it, ok := m.pk.current(); ok {
				return m, m.send(ipc.Request{Cmd: "add", Paths: []string{it.value}}, false)
			}
			return m, nil
		}
		m.flash = ""
		return m, m.pk.handleKey(msg)
	}
	return m, nil
}

// send manda la petición al servicio; quit indica si el selector se cierra
// con la respuesta (enter) o sigue abierto (tab).
func (m *selectModel) send(r ipc.Request, quit bool) tea.Cmd {
	sock := m.sock
	return func() tea.Msg {
		c, err := ipc.Dial(sock)
		var resp ipc.Response
		if err == nil {
			defer c.Close()
			resp, err = c.Do(r)
			if err == nil && !resp.OK {
				err = errors.New(resp.Error)
			}
		}
		if quit {
			return selDoneMsg{msg: resp.Msg, err: err}
		}
		return selAddMsg{msg: resp.Msg, err: err}
	}
}

func (m *selectModel) View() string {
	if m.done || m.width == 0 {
		return ""
	}
	w := pickerWidth(m.width)
	maxRows := m.height - 8
	if maxRows > 12 {
		maxRows = 12
	}
	hint := i18n.Tf("songs.hint", len(m.pk.matches))
	box := m.pk.render(i18n.T("sel.title"), hint, w, maxRows)
	if m.flash != "" {
		box += "\n " + m.flash
	}
	return box
}
