package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"maly/internal/config"
	"maly/internal/i18n"
)

// Pantalla inicial de selección de idioma: aparece solo cuando language = ""
// en el config; la elección se persiste y no vuelve a preguntar.

var langOptions = []struct{ code, label string }{
	{"en", "English"},
	{"es", "Español"},
}

// langLabel devuelve el nombre visible de un código de idioma.
func langLabel(code string) string {
	for _, o := range langOptions {
		if o.code == code {
			return o.label
		}
	}
	return code
}

func (m *Model) handleLangKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.langCursor > 0 {
			m.langCursor--
		}
	case "down", "j":
		if m.langCursor < len(langOptions)-1 {
			m.langCursor++
		}
	case "enter":
		code := langOptions[m.langCursor].code
		i18n.Set(code)
		m.cfg.Language = code
		m.filterInput.Placeholder = i18n.T("tui.filter_ph")
		m.langOpen = false
		if err := config.SaveLanguage(code); err != nil {
			m.setFlash(err.Error(), true)
		}
		// Recargar la biblioteca para que las etiquetas "(desconocido)" etc.
		// se generen en el idioma elegido.
		return m, loadLibrary
	}
	return m, nil
}

func (m *Model) langView() string {
	w := 34
	innerW := w - 2
	lines := []string{""}
	for i, o := range langOptions {
		line := clip("   "+o.label, innerW)
		if i == m.langCursor {
			line = m.st.selected.Render(padTo(clip(" ❯ "+o.label, innerW), innerW))
		} else {
			line = m.st.text.Render(line)
		}
		lines = append(lines, line)
	}
	lines = append(lines, "", m.st.dim.Render(clip(" "+i18n.T("lang.hint"), innerW)))

	box := m.st.panel(i18n.T("lang.title"), lines, w, len(lines)+2, true)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
