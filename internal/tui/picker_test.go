package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// linesFitWithin falla si alguna línea de out (separadas por "\n") mide más
// que w celdas — la garantía que panel() asume que el llamador ya cumplió.
func linesFitWithin(t *testing.T, out string, w int) {
	t.Helper()
	for i, l := range strings.Split(out, "\n") {
		if lw := lipgloss.Width(l); lw > w {
			t.Errorf("línea %d mide %d celdas, más que el ancho de la caja (%d): %q", i, lw, w, l)
		}
	}
}

func pickerWith(labels ...string) *picker {
	p := newPicker(styles{}, "buscar…")
	items := make([]pickerItem, 0, len(labels))
	for _, l := range labels {
		items = append(items, newPickerItem(l, l))
	}
	p.setItems(items)
	return p
}

// TestPickerFilterFoldAware: el fuzzy match es insensible a acentos y
// mayúsculas — "aurea" debe encontrar "Proporción Áurea", igual que la
// búsqueda de la biblioteca.
func TestPickerFilterFoldAware(t *testing.T) {
	p := pickerWith("Proporción Áurea", "Luna Llena", "Sol de Medianoche")

	// Sin consulta: todos, en el orden original.
	if len(p.matches) != 3 || p.matches[0] != 0 || p.matches[2] != 2 {
		t.Fatalf("sin filtro: %v", p.matches)
	}

	p.input.SetValue("aurea")
	p.filter()
	if len(p.matches) != 1 || p.matches[0] != 0 {
		t.Fatalf("\"aurea\" debía dar solo Proporción Áurea: %v", p.matches)
	}
	it, ok := p.current()
	if !ok || it.value != "Proporción Áurea" {
		t.Fatalf("current: %+v, %v", it, ok)
	}

	p.input.SetValue("zzz")
	p.filter()
	if len(p.matches) != 0 {
		t.Fatalf("\"zzz\" no debía dar resultados: %v", p.matches)
	}
	if _, ok := p.current(); ok {
		t.Fatal("current sin matches debe reportar false")
	}
}

// TestPickerCursorClamp: el cursor sobrevive a que el filtro encoja la lista.
func TestPickerCursorClamp(t *testing.T) {
	p := pickerWith("uno", "dos", "tres")
	p.cursor = 2
	p.input.SetValue("uno")
	p.filter()
	if p.cursor != 0 {
		t.Errorf("cursor tras encoger a 1 resultado: %d", p.cursor)
	}
	p.input.SetValue("")
	p.filter()
	p.cursor = -3
	p.clamp()
	if p.cursor != 0 {
		t.Errorf("cursor negativo debe quedar en 0: %d", p.cursor)
	}
}

// TestPickerSetItemsKeeping: en una recarga en vivo el cursor debe quedarse
// sobre el MISMO elemento aunque la lista cambie por arriba. Con setItems a
// secas el índice se queda quieto y la selección se corre sola — y en el
// panel de playlists eso significa que ctrl+x borra otra.
func TestPickerSetItemsKeeping(t *testing.T) {
	items := func(labels ...string) []pickerItem {
		out := make([]pickerItem, 0, len(labels))
		for _, l := range labels {
			out = append(out, newPickerItem(l, l))
		}
		return out
	}

	p := pickerWith("ambient", "jazz", "rock", "trap")
	p.cursor = 2 // rock
	// Desaparece una de arriba y entra otra: el índice 2 pasaría a ser trap.
	p.setItemsKeeping(items("blues", "jazz", "rock", "trap"))
	if it, _ := p.current(); it.value != "rock" {
		t.Fatalf("la selección se movió a %q, quería rock", it.value)
	}

	// Con el elegido fuera de la lista, el cursor queda dentro de rango.
	p.setItemsKeeping(items("blues", "jazz"))
	if p.cursor < 0 || p.cursor >= len(p.matches) {
		t.Fatalf("cursor fuera de rango: %d de %d", p.cursor, len(p.matches))
	}

	// El filtro escrito sigue mandando tras la recarga.
	p = pickerWith("ambient", "jazz", "rock")
	p.input.SetValue("ja")
	p.filter()
	p.setItemsKeeping(items("ambient", "jazz", "jarana", "rock"))
	if len(p.matches) != 2 {
		t.Fatalf("el filtro debía seguir aplicándose: %d resultados", len(p.matches))
	}
}

// TestPickerRenderLongQueryNoOverflow: escribir una consulta más larga que
// la caja no debía romper el borde derecho — el textinput de bubbles nunca
// recibía Width, así que su scroll horizontal quedaba desactivado y View()
// emitía el valor completo (auditoría de UX post-1.12.0, reportado por el
// dueño en la Command Palette).
func TestPickerRenderLongQueryNoOverflow(t *testing.T) {
	p := pickerWith("una pista")
	p.input.SetValue(strings.Repeat("x", 200))
	out := p.render("Songs", "hint", 60, 10)
	linesFitWithin(t, out, 60)
}

// TestPickerRenderEmptyLibraryNoOverflow: sel.none_empty es un texto fijo
// largo (~68-74 celdas); en una caja angosta (terminal chico) desbordaba el
// borde porque era la única línea de render() sin clip (auditoría de UX
// post-1.12.0).
func TestPickerRenderEmptyLibraryNoOverflow(t *testing.T) {
	p := pickerWith() // sin items: dispara sel.none_empty
	out := p.render("Songs", "hint", 40, 10)
	linesFitWithin(t, out, 40)
}

// TestPickerWidth cubre los tres tramos: proporcional, mínimo y tope.
func TestPickerWidth(t *testing.T) {
	cases := []struct{ term, want int }{
		{200, 100}, // 2/3 = 133, tope 100
		{90, 60},   // 2/3 justo
		{60, 56},   // 2/3 = 40 < 50: term - 4
	}
	for _, c := range cases {
		if got := pickerWidth(c.term); got != c.want {
			t.Errorf("pickerWidth(%d) = %d, quería %d", c.term, got, c.want)
		}
	}
}
