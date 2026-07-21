package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
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
	if q == "" {
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
	case "pgup":
		p.cursor -= p.page
		p.clamp()
		return nil
	case "pgdown":
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
func pickerWidth(termW int) int {
	w := termW * 2 / 3
	if w < 50 {
		w = termW - 4
	}
	if w > 80 {
		w = 80
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

	lines := []string{p.input.View(), p.st.dim.Render(strings.Repeat("─", innerW))}
	if len(p.matches) == 0 {
		lines = append(lines, p.st.dim.Render(i18n.T("sel.none")))
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
