package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"maly/internal/config"
	"maly/internal/i18n"
	"maly/internal/ipc"
)

func TestSaveQueueShortcut(t *testing.T) {
	for _, tc := range []struct {
		key string
		msg tea.KeyMsg
	}{
		{"ctrl+s", tea.KeyMsg{Type: tea.KeyCtrlS}},
		{"ctrl+y", tea.KeyMsg{Type: tea.KeyCtrlY}},
	} {
		for _, origin := range []overlay{ovNone, ovNow} {
			m := newConModel()
			m.keys = config.DefaultKeys()
			m.keys["save_queue"] = tc.key
			m.overlay = origin
			m.showHelp = origin == ovNone
			m.width = 140
			m.height = 42
			m.focus = panelQueue
			m.queue = []ipc.TrackInfo{{Path: "/no-tocar", Title: "oculta"}}
			m.applyFilter("sin coincidencias")
			if !strings.Contains(m.mainHint(), tc.key+" "+i18n.T("hint.save_queue")) {
				t.Fatal("falta acceso contextual con filtro")
			}
			m.handleKey(tc.msg)
			if m.overlay != ovConsole || m.showHelp || m.conInput.Value() != "playlist save " || len(m.conHistory) != 0 {
				t.Fatalf("no preparó el nombre: %+v", m)
			}
			if origin == ovNone {
				m.filterMode = true
			} // el filtro continúa tras cancelar.
			conKey(m, tea.KeyEsc)
			if m.overlay != ovNone || len(m.queue) != 1 {
				t.Fatal("cancelar perdió contexto")
			}
		}
	}
}

func TestConsoleSaveQueueUsesIPC(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	m := newConModel()
	m.openConsole()
	sock, reqs := capturaReq(t)
	m.sock = sock
	m.queue = []ipc.TrackInfo{{ID: 1, Path: "/copia-local-vieja"}}
	_, cmd := m.execConsole(`playlist save "Mi cola"`)
	if cmd == nil {
		t.Fatal("save no está conectado a la consola")
	}
	msg, ok := cmd().(conMsg)
	if !ok {
		t.Fatal("mensaje incorrecto")
	}
	// El servidor devuelve una cola vacía; la copia local no debe guardarse.
	if msg.reload || !strings.Contains(strings.Join(msg.lines, "\n"), i18n.T("pl.save_empty")) {
		t.Fatalf("%+v", msg)
	}
	if req := <-reqs; req.Cmd != "queue" {
		t.Fatal(req)
	}
	if len(reqs) != 0 {
		t.Fatal("el fallo mutó el demonio")
	}
	if _, err := os.Stat(config.DBPath()); !os.IsNotExist(err) {
		t.Fatal("fabricó biblioteca", err)
	}
	_, cmd = m.execConsole(`playlist save " "`)
	if cmd == nil {
		t.Fatal("no hay respuesta")
	}
	msg = cmd().(conMsg)
	if !strings.Contains(strings.Join(msg.lines, "\n"), i18n.T("pl.usage_save")) {
		t.Fatal(msg)
	}
	if len(reqs) != 0 {
		t.Fatal("nombre vacío consultó IPC")
	}
	if _, err := os.Stat(filepath.Dir(config.DBPath())); !os.IsNotExist(err) {
		t.Fatal("fabricó directorio")
	}
}
