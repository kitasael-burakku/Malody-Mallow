package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"maly/internal/config"
	"maly/internal/i18n"
	"maly/internal/ipc"
)

func TestUndoTUIObservedToken(t *testing.T) {
	for _, key := range []string{"u", "ctrl+y"} {
		t.Run(key, func(t *testing.T) {
			m := newLayoutTestModel(40, 12)
			m.keys = config.DefaultKeys()
			m.keys["undo_remove"] = key
			m.status = &ipc.Status{UndoToken: "eliminación-observada"}
			sock, requests := capturaReq(t)
			m.sock = sock
			msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("u")}
			if key == "ctrl+y" {
				msg = tea.KeyMsg{Type: tea.KeyCtrlY}
			}
			_, cmd := m.handleKey(msg)
			if cmd == nil {
				t.Fatal("atajo undo desconectado")
			}
			r := esperaReq(t, cmd, requests)
			if r.Cmd != "undo" || r.Value != "eliminación-observada" {
				t.Fatal("no protege eliminación observada", r)
			}
			if h := m.mainHint(); !strings.Contains(h, key) || !strings.Contains(h, i18n.T("hint.undo_remove")) {
				t.Fatal("deshacer invisible a 40 columnas", h)
			}
			m.status.UndoToken = ""
			_, cmd = m.handleKey(msg)
			if cmd != nil || m.flash != i18n.T("d.undo_expired") {
				t.Fatal("sin token pidió última eliminación ajena")
			}
			if strings.Contains(m.mainHint(), i18n.T("hint.undo_remove")) {
				t.Fatal("ayuda ofrece undo caducado")
			}
		})
	}
}

func TestUndoConsole(t *testing.T) {
	m := newLayoutTestModel(100, 28)
	sock, requests := capturaReq(t)
	m.sock = sock
	m.openConsole()
	_, cmd := m.execConsole("undo")
	if cmd == nil {
		t.Fatal("undo consola desconectado")
	}
	r := esperaReq(t, cmd, requests)
	if r.Cmd != "undo" || r.Value != "" {
		t.Fatal("consola no pide última disponible", r)
	}
	_, cmd = m.execConsole("undo basura")
	if cmd != nil {
		t.Fatal("consola ignoró argumentos")
	}
}
