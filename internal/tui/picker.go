package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"

	"maly/internal/i18n"
	"maly/internal/library"
)

// picker es un selector fuzzy genérico: input de búsqueda, lista filtrada y
// cursor, dibujado con el panel estándar de maly. Hoy lo usan el selector de
// canciones (ctrl+o) y `maly select`; para playlists, álbumes o comandos
// basta con construir otro []pickerItem.

type pickerItem struct {
	label  string // lo que se muestra
	value  string // carga útil opaca (ruta, nombre…)
	folded string // texto normalizado sobre el que se hace fuzzy match
}

func newPickerItem(label, value string) pickerItem {
	return pickerItem{label: label, value: value, folded: library.Fold(label)}
}

type pickerSource []pickerItem

func (p pickerSource) String(i int) string { return p[i].folded }
func (p pickerSource) Len() int            { return len(p) }

type picker struct {
	st      styles
	input   textinput.Model
	items   []pickerItem
	matches []int
	cursor  int
	page    int // filas visibles en el último render (para pgup/pgdown)

	// noFilter: el input es una CAJA DE CONSULTA y no un filtro. Lo usa el
	// buscador de descargas (ctrl+g), cuyos ítems no son una lista local sino
	// el resultado de una búsqueda remota: filtrar difusamente diez
	// resultados recién pedidos no aporta nada, y chocaría con que enter
	// re-busque con el texto nuevo.
	noFilter bool
	// emptyText reemplaza el texto de "sin resultados" cuando el picker no
	// está sobre la biblioteca y ni sel.none ni sel.none_empty aplican. ""
	// deja los de siempre.
	emptyText string
}

func newPicker(st styles, placeholder string) *picker {
	in := textinput.New()
	in.Prompt = "❯ "
	in.PromptStyle = st.accent
	in.TextStyle = st.text
	in.Placeholder = placeholder
	in.Focus()
	return &picker{st: st, input: in, page: 10}
}

func (p *picker) setItems(items []pickerItem) {
	p.items = items
	p.filter()
}

// setItemsKeeping reemplaza los ítems conservando la selección por valor. Es
// para las recargas en vivo (otro cliente tocó la biblioteca): el cursor es
// un índice, así que sin esto un elemento que desaparezca más arriba corre la
// lista bajo los dedos del usuario — y con ctrl+x de por medio, eso borra
// otra playlist. Si lo seleccionado ya no está, queda el clamp de siempre.
func (p *picker) setItemsKeeping(items []pickerItem) {
	sel, had := p.current()
	p.setItems(items)
	if !had {
		return
	}
	for mi, idx := range p.matches {
		if p.items[idx].value == sel.value {
			p.cursor = mi
			return
		}
	}
}

// filter recalcula los resultados según el texto del input.
func (p *picker) filter() {
	q := strings.TrimSpace(library.Fold(p.input.Value()))
	p.matches = p.matches[:0]
	if q == "" || p.noFilter {
		for i := range p.items {
			p.matches = append(p.matches, i)
		}
	} else {
		for _, r := range fuzzy.FindFrom(q, pickerSource(p.items)) {
			p.matches = append(p.matches, r.Index)
		}
	}
	p.clamp()
}

// current devuelve el ítem bajo el cursor, si lo hay.
func (p *picker) current() (pickerItem, bool) {
	if p.cursor >= 0 && p.cursor < len(p.matches) {
		return p.items[p.matches[p.cursor]], true
	}
	return pickerItem{}, false
}

func (p *picker) clamp() {
	if p.cursor >= len(p.matches) {
		p.cursor = len(p.matches) - 1
	}
	if p.cursor < 0 {
		p.cursor = 0
	}
}

// handleKey procesa navegación y escritura. Las teclas de acción (enter,
// esc, tab…) las decide el dueño del picker antes de llamar aquí.
func (p *picker) handleKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "up", "ctrl+k":
		p.cursor--
		p.clamp()
		return nil
	case "down", "ctrl+j":
		p.cursor++
		p.clamp()
		return nil
	case "pgup", "ctrl+u":
		p.cursor -= p.page
		p.clamp()
		return nil
	case "pgdown", "ctrl+d":
		p.cursor += p.page
		p.clamp()
		return nil
	}
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	p.filter()
	return cmd
}

// pickerWidth calcula el ancho del modal según el ancho del terminal (mismo
// criterio que la consola ctrl+p).
//
// Tope de 100: subido en su día de 80 porque en terminales anchas las
// descripciones largas de conHelp() (p. ej. la de playlist, que lista los
// ocho subcomandos) perdían más texto del necesario — 100 les da lugar de
// sobra sin volver la caja desproporcionada en terminales normales (decisión
// del dueño, auditoría de UX post-1.12.0).
func pickerWidth(termW int) int { return pickerWidthMax(termW, 100) }

// pickerWidthMax es pickerWidth con el tope elegido por el llamador. Existe
// para las pantallas de búsqueda de descargas, que son los únicos pickers
// cuyos ítems son texto AJENO y largo (títulos de YouTube, que se recortan
// constantemente) y que además llevan duración y visitas al final.
//
// La regla de los dos tercios manda igual: el tope solo deja de estorbar en
// terminales anchas, así que subirlo NO cambia una sola celda por debajo de
// 150 columnas y nadie que hoy esté cómodo nota la diferencia.
func pickerWidthMax(termW, max int) int {
	w := termW * 2 / 3
	if w < 50 {
		w = termW - 4
	}
	if w > max {
		w = max
	}
	return w
}

// render dibuja el panel completo del picker: input, separador, resultados
// (con scroll) y una línea de pie. w incluye los bordes; maxRows limita las
// filas de resultados visibles.
func (p *picker) render(title, hint string, w, maxRows int) string {
	if maxRows < 3 {
		maxRows = 3
	}
	p.page = maxRows
	innerW := w - 2

	// Mismo arreglo que consoleView(): sin Width, el textinput de bubbles
	// desactiva su scroll horizontal y View() emite la consulta completa —
	// una búsqueda larga rompía el borde derecho en vivo. clip() de red de
	// seguridad para el frame en que Width cambió y handleOverflow todavía
	// no corrió de nuevo.
	promptW := lipgloss.Width(p.input.Prompt)
	p.input.Width = innerW - promptW
	if p.input.Width < 0 {
		p.input.Width = 0
	}
	lines := []string{clip(p.input.View(), innerW), p.st.dim.Render(strings.Repeat("─", innerW))}
	if len(p.matches) == 0 {
		// Antes esto era siempre "no matches", aunque la causa real fuera
		// que la biblioteca está vacía de entrada (no que la búsqueda no
		// encontró nada) — el panel de biblioteca y la CLI (cli.search_none)
		// ya distinguían los dos casos; el picker era el único que no
		// (auditoría 2026-07-31, hallazgo T9).
		empty := p.emptyText
		if empty == "" {
			empty = i18n.T("sel.none")
			if len(p.items) == 0 {
				empty = i18n.T("sel.none_empty")
			}
		}
		// sel.none_empty es un texto fijo largo (~68-74 celdas); sin clip
		// rompía el borde en terminales angostas.
		lines = append(lines, p.st.dim.Render(clip(empty, innerW)))
	}
	start := 0
	if p.cursor >= maxRows {
		start = p.cursor - maxRows + 1
	}
	end := start + maxRows
	if end > len(p.matches) {
		end = len(p.matches)
	}
	for i := start; i < end; i++ {
		it := p.items[p.matches[i]]
		line := clip("  "+it.label, innerW)
		if i == p.cursor {
			line = p.st.selected.Render(padTo(line, innerW))
		} else {
			line = p.st.text.Render(line)
		}
		lines = append(lines, line)
	}
	lines = append(lines, p.st.dim.Render(clip("  "+hint, innerW)))

	return p.st.panel(title, lines, w, len(lines)+2, true)
}
