package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"maly/internal/i18n"
)

// wrappedLines respeta el presupuesto de celdas y filas del llamador. Envuelve
// por palabras y parte tokens largos; solo la última fila usa elipsis si no cabe.
func wrappedLines(text string, w, h int) []string {
	if w < 1 || h < 1 {
		return nil
	}
	lines := strings.Split(ansi.Wrap(text, w, ""), "\n")
	if len(lines) > h {
		lines = lines[:h]
		lines[h-1] = clip(lines[h-1]+"…", w)
	}
	return lines
}

func styledLines(text string, w, h int, st lipgloss.Style) []string {
	lines := wrappedLines(text, w, h)
	for i := range lines {
		lines[i] = st.Render(lines[i])
	}
	return lines
}

// fitHints añade acciones completas por prioridad. No recorta una tecla ni
// reserva filas a costa de la lista; la ayuda completa sigue en HelpRows.
func fitHints(w int, hints ...string) string {
	var out string
	for _, hint := range hints {
		if hint == "" {
			continue
		}
		candidate := hint
		if out != "" {
			candidate = out + " · " + hint
		}
		if lipgloss.Width(candidate) <= w {
			out = candidate
		}
	}
	return out
}

func (m *Model) mainHint() string {
	k := func(action string) string { return keyLabel(m.keys[action]) }
	action := func(key, desc string) string {
		if key == "" {
			return ""
		}
		return key + " " + desc
	}
	help := action(k("help"), i18n.T("hint.help"))
	quit := action(k("quit"), i18n.T("help.quit"))
	if m.filterMode {
		return fitHints(m.width, i18n.T("hint.filter_edit"), action(k("palette"), i18n.T("hint.palette")))
	}
	panel := i18n.T("hint.to_queue")
	if m.focus == panelQueue {
		panel = i18n.T("hint.to_library")
	}
	hints := []string{help, quit, action(k("switch_panel"), panel), action(k("filter"), i18n.T("hint.filter"))}
	if m.focus == panelLibrary {
		if m.tree != nil {
			if n := m.tree.current(); n != nil {
				hints = append(hints, action(k("add"), i18n.T("hint.add")))
				verb := i18n.T("hint.expand")
				if n.kind == trackNode {
					verb = i18n.T("hint.play")
				} else if n.expanded {
					verb = i18n.T("hint.collapse")
				}
				hints = append(hints, action("enter", verb), action(k("play_next"), i18n.T("hint.play_next")))
			}
		}
	} else if len(m.visibleQueue()) > 0 {
		hints = append(hints, action(k("play_next"), i18n.T("hint.play_next")), action(k("remove"), i18n.T("hint.remove")), action("enter", i18n.T("hint.play")))
	}
	if m.focus == panelQueue && len(m.queue) > 0 {
		hints = append(hints, action(k("save_queue"), i18n.T("hint.save_queue")))
	}
	hints = append(hints, action(k("play_pause"), i18n.T("hint.pause")), action(k("palette"), i18n.T("hint.palette")), action(k("songs"), i18n.T("hint.songs")))
	if m.embedded {
		hints = append(hints, strings.TrimSpace(i18n.T("tui.footer_embedded")))
	}
	return fitHints(m.width, hints...)
}
