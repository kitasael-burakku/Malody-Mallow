package tui

import (
	"testing"
)

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

// TestPickerWidth cubre los tres tramos: proporcional, mínimo y tope.
func TestPickerWidth(t *testing.T) {
	cases := []struct{ term, want int }{
		{150, 80}, // 2/3 = 100, tope 80
		{90, 60},  // 2/3 justo
		{60, 56},  // 2/3 = 40 < 50: term - 4
	}
	for _, c := range cases {
		if got := pickerWidth(c.term); got != c.want {
			t.Errorf("pickerWidth(%d) = %d, quería %d", c.term, got, c.want)
		}
	}
}
