package tui

import (
	"testing"

	"maly/internal/ipc"
)

// TestVisibleQueue: el filtro de la cola devuelve índices reales (no
// posiciones filtradas) y es fold-aware como todo lo demás.
func TestVisibleQueue(t *testing.T) {
	m := &Model{
		queue: []ipc.TrackInfo{
			{Title: "Luna Llena", Artist: "Ana"},
			{Title: "Sol", Artist: "Beto"},
			{Title: "Eclipse", Album: "Lunática"},
		},
	}
	if got := m.visibleQueue(); len(got) != 3 {
		t.Fatalf("sin filtro: %v", got)
	}
	m.queueFilter = "luna"
	if got := m.visibleQueue(); len(got) != 2 || got[0] != 0 || got[1] != 2 {
		t.Fatalf("filtro \"luna\" (título y álbum): %v", got)
	}
	m.queueFilter = "beto sol"
	if got := m.visibleQueue(); len(got) != 1 || got[0] != 1 {
		t.Fatalf("filtro multi-palabra: %v", got)
	}
	m.queueFilter = "nada"
	if got := m.visibleQueue(); len(got) != 0 {
		t.Fatalf("filtro sin resultados: %v", got)
	}
}

// TestClipPadTo: clip corta por celdas con elipsis y padTo rellena midiendo
// ancho visible (no bytes), que es lo que importa con acentos y ANSI.
func TestClipPadTo(t *testing.T) {
	if got := clip("ñandú corre", 6); got != "ñandú…" {
		t.Errorf("clip: %q", got)
	}
	if got := clip("corto", 10); got != "corto" {
		t.Errorf("clip sin corte: %q", got)
	}
	if got := clip("lo que sea", 0); got != "" {
		t.Errorf("clip a 0: %q", got)
	}
	if got := padTo("ñu", 4); got != "ñu  " {
		t.Errorf("padTo: %q", got)
	}
	if got := padTo("largo", 3); got != "largo" {
		t.Errorf("padTo sin hueco: %q", got)
	}
}
