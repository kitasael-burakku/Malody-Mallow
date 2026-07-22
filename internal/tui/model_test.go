package tui

import (
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	"maly/internal/config"
	"maly/internal/ipc"
	"maly/internal/library"
)

// El chequeo de update ocurría SOLO en Init: una TUI abierta días no volvía a
// mirar nunca. Ahora un tick largo lo repite, y respeta la clave update_check.
func TestRechequeoDeUpdate(t *testing.T) {
	m := &Model{cfg: config.Config{UpdateCheck: true}}
	if _, cmd := m.Update(updTickMsg(time.Now())); cmd == nil {
		t.Error("con update_check activo el tick debe re-chequear y re-armarse")
	}
	m.cfg.UpdateCheck = false
	if _, cmd := m.Update(updTickMsg(time.Now())); cmd != nil {
		t.Error("con update_check apagado el tick no debe hacer nada")
	}
}

// updMsg solo enciende el aviso si el release es realmente más nuevo: un
// re-chequeo que devuelva la versión instalada no debe dejar el pie encendido.
func TestUpdMsgSoloAvisaSiEsMasNuevo(t *testing.T) {
	m := &Model{}
	m.Update(updMsg{latest: "v0.0.1"})
	if m.updAvail != "" {
		t.Errorf("una versión vieja encendió el aviso: %q", m.updAvail)
	}
	m.Update(updMsg{latest: "v999.0.0"})
	if m.updAvail != "v999.0.0" {
		t.Errorf("una versión nueva no encendió el aviso: %q", m.updAvail)
	}
}

// progressBar: la guarda que faltaba era la INFERIOR. Con una Duration diminuta
// frente a Position el cociente desborda a +Inf, y int(+Inf) en amd64 da el
// mínimo de int64 — un negativo que no supera w y llegaba tal cual a
// strings.Repeat, que entra en pánico con conteos negativos y se llevaba la TUI.
func TestProgressBarValoresPatologicos(t *testing.T) {
	m := &Model{st: newStyles(config.Theme{})}
	const w = 40
	casos := []struct {
		nombre   string
		pos, dur float64
	}{
		{"cociente desbordado a +Inf", 1e308, 1e-300},
		{"posición mayor que la duración", 500, 10},
		{"duración cero", 5, 0},
		{"duración negativa", 5, -1},
		{"posición negativa", -5, 10},
		{"normal, a la mitad", 50, 100},
		{"al principio", 0, 100},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			got := m.progressBar(c.pos, c.dur, w) // no debe entrar en pánico
			if n := lipgloss.Width(got); n != w {
				t.Errorf("ancho = %d, quería exactamente %d", n, w)
			}
		})
	}
	// Ancho degenerado: tampoco puede reventar.
	if got := m.progressBar(5, 10, 0); lipgloss.Width(got) != 0 {
		t.Errorf("con w=0 debía salir vacío, salió %q", got)
	}
}

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

// TestLibraryMsgRefreshesPlaylists cubre la limitación que esto vino a
// cerrar: un panel ctrl+l ya ABIERTO no se enteraba de lo que otro cliente
// hacía con las playlists (solo se releía al reabrirlo). Ahora la recarga de
// biblioteca —que es lo que dispara el push de LibGen— también lo refresca,
// sin mover la selección.
func TestLibraryMsgRefreshesPlaylists(t *testing.T) {
	tracks := []library.Track{{ID: 1, Artist: "Ana", Album: "Uno", Title: "alfa", Path: "/m/a.mp3"}}
	lists := []plList{
		{name: "ambient", tracks: tracks},
		{name: "rock", tracks: tracks},
	}
	m := &Model{plOpen: true, pl: newPicker(styles{}, "")}
	m.pl.setItems(plItems(lists))
	m.pl.cursor = 1 // rock

	// Otro cliente borra la primera y crea una nueva al final.
	nuevas := []plList{
		{name: "rock", tracks: tracks},
		{name: "trap", tracks: tracks},
	}
	m.Update(libraryMsg{tracks: tracks, lists: nuevas})

	if len(m.pl.items) != 2 {
		t.Fatalf("el panel no se refrescó: %d entradas", len(m.pl.items))
	}
	if it, _ := m.pl.current(); it.value != "rock" {
		t.Fatalf("la selección saltó a %q, quería rock", it.value)
	}
	var names []string
	for _, it := range m.pl.items {
		names = append(names, it.value)
	}
	if names[0] != "rock" || names[1] != "trap" {
		t.Fatalf("contenido del panel: %v", names)
	}

	// Con el panel cerrado no se toca nada (ni se paga el trabajo).
	m2 := &Model{pl: newPicker(styles{}, "")}
	m2.Update(libraryMsg{tracks: tracks, lists: nuevas})
	if len(m2.pl.items) != 0 {
		t.Fatalf("panel cerrado: no debía cargarse nada, hubo %d", len(m2.pl.items))
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
