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

// conBrowsing: paleta recién abierta, sin salida ni texto. Es el único
// estado en que las sugerencias ocupan la caja entera como catálogo por
// categorías; al escribir pasan a ser una lista corta encima de la salida.
func (m *Model) conBrowsing() bool {
	return m.conPlConfirm == "" && len(m.conLines) == 0 && strings.TrimSpace(m.conInput.Value()) == ""
}

// conRow es una fila de la ventana de sugerencias: cabecera de categoría
// (head != "") o entrada (índice en conSuggestions).
type conRow struct {
	head  string
	entry int
}

// conSuggestionLines dibuja como mucho budget filas y devuelve también la
// etiqueta del borde (cuántos comandos, o cuál está elegido). El marcador
// textual sobrevive a NO_COLOR. La ventana sigue la selección; navegando el
// catálogo, la categoría de la primera fila visible se mantiene arriba.
func (m *Model) conSuggestionLines(width, budget int) ([]string, string) {
	if m.conPlConfirm != "" || budget <= 0 {
		return nil, ""
	}
	entries := m.conSuggestions()
	if len(entries) == 0 || (strings.TrimSpace(m.conInput.Value()) == "" && len(m.conLines) > 0 && m.conChoice == "") {
		if e, ok := m.conContext(); ok {
			return []string{m.st.dim.Render(conFit(e.Usage(), width))}, ""
		}
		if strings.TrimSpace(m.conInput.Value()) != "" && len(entries) == 0 {
			return styledLines(i18n.T("con.no_matches"), width, min(2, budget), m.st.dim), i18n.Tf("con.count", 0)
		}
		return nil, ""
	}
	chosen := -1
	for i, e := range entries {
		if e.Name == m.conChoice {
			chosen = i
			break
		}
	}
	label := i18n.Tf("con.count", len(entries))
	if len(entries) == 1 {
		label = i18n.T("con.count_one")
	}
	if chosen >= 0 {
		label = i18n.Tf("con.pos", chosen+1, len(entries))
	}
	usageW, wide := conColumns(width)
	var tail []string
	if (!wide || chosen >= 0) && budget >= 4 {
		tail = styledLines(i18n.T(entries[max(0, chosen)].DescKey), width, min(2, budget-3), m.st.text)
		budget -= len(tail) + 1
		tail = append([]string{""}, tail...)
	}
	browse := m.conBrowsing()
	var rows []conRow
	group := ""
	for i, e := range entries {
		if browse && e.Group != group {
			group = e.Group
			rows = append(rows, conRow{head: group, entry: -1})
		}
		rows = append(rows, conRow{entry: i})
	}
	target := 0
	for j, r := range rows {
		if r.entry == max(0, chosen) {
			target = j
			break
		}
	}
	start := max(0, target-budget+1)
	sticky := ""
	if browse && start > 0 && rows[start].head == "" && budget > 1 {
		start = max(0, target-budget+2)
		if rows[start].head == "" {
			sticky = entries[rows[start].entry].Group
		}
	}
	var lines []string
	if sticky != "" {
		lines = append(lines, m.conHeading(sticky, width))
	}
	lastHead := false
	for j := start; j < len(rows) && len(lines) < budget; j++ {
		r := rows[j]
		lastHead = r.head != ""
		if lastHead {
			lines = append(lines, m.conHeading(r.head, width))
		} else {
			e := entries[r.entry]
			col := usageW
			if browse {
				col = conGroupColumn(e.Group, usageW)
			}
			lines = append(lines, m.conEntryRow(e, width, col, wide, r.entry == chosen))
		}
	}
	if lastHead {
		lines = lines[:len(lines)-1] // una categoría sin ninguna entrada debajo
	}
	return append(lines, tail...), label
}
