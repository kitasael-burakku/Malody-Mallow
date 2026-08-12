package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"testing"

	"maly/internal/config"
	"maly/internal/i18n"
)

// TestLangEscChoosesActiveLanguage cubre el hallazgo UX-N6 de la auditoría
// técnica: el selector inicial de idioma no tenía ninguna tecla de "salir
// sin elegir" salvo ctrl+c (que cierra la app entera) — esc no estaba
// manejado y se tragaba en silencio. Ahora esc cierra el selector con el
// idioma YA activo (el que detectó envLangHint() en main.go, o el fallback),
// igual que si se hubiera confirmado con enter, en vez de no hacer nada.
func TestLangEscChoosesActiveLanguage(t *testing.T) {
	xdgSandboxTUI(t)
	i18n.Set("en")

	m := &Model{
		st:          newStyles(config.Theme{}),
		cfg:         config.Config{Language: ""},
		langOpen:    true,
		filterInput: textinput.New(),
	}

	_, cmd := m.handleLangKey(tea.KeyMsg{Type: tea.KeyEsc})
	if m.langOpen {
		t.Error("esc debía cerrar el selector de idioma")
	}
	if m.cfg.Language != "en" {
		t.Errorf("cfg.Language = %q, quería %q (el idioma ya activo)", m.cfg.Language, "en")
	}
	if cmd == nil {
		t.Error("esc debía recargar la biblioteca, igual que enter")
	}
}
