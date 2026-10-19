package tui

import (
	"sort"
	"strings"

	"maly/internal/commandmeta"
	"maly/internal/i18n"
	"maly/internal/library"
)

// conCommandText quita solo el prefijo opcional, nunca interpreta argumentos.
func conCommandText(line string) string {
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimLeft(line, " \t"), "maly "))
}

// conSuggestions busca nombres y descripciones localizadas. Una vez que hay
// argumentos, se edita texto libre: no se reemplazan títulos ni comillas con
// sugerencias de comandos. No consulta DB, demonio ni red al escribir.
func (m *Model) conSuggestions() []commandmeta.Entry {
	if m.conPlConfirm != "" {
		return nil
	}
	query := library.Fold(conCommandText(m.conInput.Value()))
	var out []commandmeta.Entry
	for _, e := range commandmeta.Console() {
		name := library.Fold(e.Name)
		match := strings.HasPrefix(name, query)
		if !match {
			// Si empezó un comando real, solo caben sus subcomandos.
			first, _, _ := strings.Cut(query, " ")
			_, command := commandmeta.Lookup(first)
			if command && strings.Contains(query, " ") {
				continue
			}
			desc := library.Fold(i18n.T(e.DescKey))
			match = true
			for _, word := range strings.Fields(query) {
				if !strings.Contains(name+" "+desc, word) {
					match = false
					break
				}
			}
		}
		if match {
			out = append(out, e)
		}
	}
	rank := func(e commandmeta.Entry) int {
		if e.Name == query {
			return 0
		}
		if strings.HasPrefix(e.Name, query) {
			return 1
		}
		desc := library.Fold(i18n.T(e.DescKey))
		if strings.HasPrefix(desc, query) {
			return 2
		}
		if strings.Contains(desc, query) {
			return 3
		}
		return 4
	}
	sort.SliceStable(out, func(i, j int) bool { return rank(out[i]) < rank(out[j]) })
	return out
}

func (m *Model) conSelectSuggestion(back bool) {
	entries := m.conSuggestions()
	if len(entries) == 0 {
		m.conChoice = ""
		return
	}
	idx := -1
	for i, e := range entries {
		if e.Name == m.conChoice {
			idx = i
			break
		}
	}
	if back {
		if idx <= 0 {
			idx = len(entries)
		}
		idx--
	} else {
		idx = (idx + 1) % len(entries)
	}
	m.conChoice = entries[idx].Name
}

func (m *Model) conAcceptSuggestion() bool {
	if m.conChoice == "" {
		return false
	}
	for _, e := range m.conSuggestions() {
		if e.Name != m.conChoice {
			continue
		}
		value := e.Name
		if e.ArgsKey != "" {
			value += " "
		}
		if strings.HasPrefix(strings.TrimLeft(m.conInput.Value(), " \t"), "maly ") {
			value = "maly " + value
		}
		m.conInput.SetValue(value)
		m.conInput.CursorEnd()
		m.conChoice = ""
		m.conHistIdx = len(m.conHistory)
		return true
	}
	m.conChoice = ""
	return false
}

// conContext muestra la gramática durante la edición, incluso si una comilla
// todavía está abierta. La coincidencia más larga distingue playlist delete.
func (m *Model) conContext() (commandmeta.Entry, bool) {
	line := conCommandText(m.conInput.Value())
	var best commandmeta.Entry
	for _, e := range commandmeta.Console() {
		if (line == e.Name || strings.HasPrefix(line, e.Name+" ")) && len(e.Name) > len(best.Name) {
			best = e
		}
	}
	return best, best.Name != ""
}

// conSuggestionLines reserva pocas filas; el marcador textual sobrevive a
// NO_COLOR. La ventana sigue la selección, sin ocultar la entrada ni la salida.
func (m *Model) conSuggestionLines(width, height int) []string {
	if m.conPlConfirm != "" {
		return nil
	}
	entries := m.conSuggestions()
	if len(entries) == 0 || (strings.TrimSpace(m.conInput.Value()) == "" && len(m.conLines) > 0 && m.conChoice == "") {
		if e, ok := m.conContext(); ok {
			return []string{m.st.dim.Render(clip(e.Usage(), width))}
		}
		return nil
	}
	rows := 3
	if height <= 14 {
		rows = 1
	}
	chosen := 0
	for i, e := range entries {
		if e.Name == m.conChoice {
			chosen = i
			break
		}
	}
	start := max(0, chosen-rows+1)
	var lines []string
	for i := start; i < len(entries) && i < start+rows; i++ {
		e := entries[i]
		mark := "  "
		style := m.st.text
		if e.Name == m.conChoice {
			mark = "> "
			style = m.st.accent
		}
		lines = append(lines, style.Render(clip(mark+e.Usage(), width)))
	}
	lines = append(lines, m.st.dim.Render(clip(i18n.T(entries[chosen].DescKey), width)))
	return lines
}
