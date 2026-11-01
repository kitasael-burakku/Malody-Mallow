package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"maly/internal/commandmeta"
	"maly/internal/i18n"
)

// TestConBrowseCatalogo: recién abierta, la paleta enseña el catálogo por
// categorías; al bajar con Tab la categoría de la elegida sigue a la vista
// aunque su cabecera haya salido por arriba, y la caja nunca desborda.
func TestConBrowseCatalogo(t *testing.T) {
	old := i18n.Code()
	t.Cleanup(func() { i18n.Set(old) })
	for _, lang := range []string{"es", "en"} {
		i18n.Set(lang)
		for _, size := range [][2]int{{140, 42}, {100, 30}, {80, 24}, {40, 12}} {
			m := newConModel()
			m.openConsole()
			m.width, m.height = size[0], size[1]
			out := m.consoleView()
			assertUX02Frame(t, out, m.width, m.height)
			boxLinesSameWidth(t, out, m.layoutOf().console.width)
			if !strings.Contains(out, i18n.T("cmd.group.playback")) || !strings.Contains(out, "play") {
				t.Fatalf("%s %v: falta el catálogo inicial:\n%s", lang, size, out)
			}
			for m.conChoice != "playlist import" {
				conKey(m, tea.KeyTab)
			}
			out = m.consoleView()
			assertUX02Frame(t, out, m.width, m.height)
			boxLinesSameWidth(t, out, m.layoutOf().console.width)
			for _, want := range []string{conMark + "playlist import", i18n.T("cmd.group.playlists")} {
				if !strings.Contains(out, want) {
					t.Fatalf("%s %v: perdió %q:\n%s", lang, size, want, out)
				}
			}
			if m.height < 40 && strings.Contains(out, i18n.T("cmd.group.playback")) {
				t.Fatalf("%s %v: la ventana no siguió a la selección:\n%s", lang, size, out)
			}
		}
	}
}

// TestConFiltroSinCabeceras: al escribir, la lista va por relevancia y no
// repite cabeceras de categoría; el borde dice cuántas hay y cuál se eligió.
func TestConFiltroSinCabeceras(t *testing.T) {
	old := i18n.Code()
	t.Cleanup(func() { i18n.Set(old) })
	i18n.Set("en")
	m := newConModel()
	m.openConsole()
	m.width, m.height = 100, 30
	conType(m, "playlist")
	n := len(m.conSuggestions())
	out := m.consoleView()
	if strings.Contains(out, "─ "+i18n.T("cmd.group.playlists")+" ─") {
		t.Fatalf("cabecera en modo filtro:\n%s", out)
	}
	if !strings.Contains(out, i18n.Tf("con.count", n)) {
		t.Fatalf("falta el contador %d:\n%s", n, out)
	}
	conKey(m, tea.KeyTab)
	if out = m.consoleView(); !strings.Contains(out, i18n.Tf("con.pos", 1, n)) {
		t.Fatalf("falta la posición elegida:\n%s", out)
	}
}

// TestConHintLinesAccionesEnteras: el pie se envuelve por acciones, nunca
// partiendo "tab/shift+tab elige" entre dos filas.
func TestConHintLinesAccionesEnteras(t *testing.T) {
	m := newConModel()
	text := i18n.TL("es", "con.hint")
	want := strings.Split(text, " · ")
	for _, w := range []int{20, 38, 51, 98} {
		var got []string
		for _, l := range m.conHintLines(text, w, 10) {
			if lipgloss.Width(l) > w {
				t.Errorf("w=%d: fila de %d celdas: %q", w, lipgloss.Width(l), l)
			}
			got = append(got, strings.Split(l, " · ")...)
		}
		if strings.Join(got, "|") != strings.Join(want, "|") {
			t.Errorf("w=%d: acciones partidas o perdidas: %q", w, got)
		}
	}
}

// TestConHelpCompleta: la ayuda lista cada uso ENTERO y bajo su categoría, a
// cualquier ancho razonable, sin pasarse de la caja.
func TestConHelpCompleta(t *testing.T) {
	old := i18n.Code()
	t.Cleanup(func() { i18n.Set(old) })
	for _, lang := range []string{"es", "en"} {
		i18n.Set(lang)
		for _, width := range []int{80, 120, 160} {
			m := newConModel()
			m.openConsole()
			m.width, m.height = width, 40
			m.conHelp()
			innerW := computeConsoleLayout(width, 40).contentW
			all := strings.Join(m.conLines, "\n")
			for _, l := range m.conLines {
				if lipgloss.Width(l) > innerW {
					t.Errorf("%s %d: fila de %d celdas: %q", lang, width, lipgloss.Width(l), l)
				}
			}
			for _, e := range commandmeta.Console() {
				if !strings.Contains(all, e.Usage()) {
					t.Errorf("%s %d: uso recortado o ausente: %q", lang, width, e.Usage())
				}
				if !strings.Contains(all, i18n.T(e.Group)) {
					t.Errorf("%s %d: falta la categoría %s", lang, width, e.Group)
				}
			}
		}
	}
}

// TestConColumnaPorCategoria: bajo su cabecera, una categoría de usos cortos
// no hereda la columna del uso más largo del catálogo.
func TestConColumnaPorCategoria(t *testing.T) {
	old := i18n.Code()
	t.Cleanup(func() { i18n.Set(old) })
	i18n.Set("es")
	m := newConModel()
	m.openConsole()
	m.width, m.height = 160, 40
	m.conHelp()
	innerW := m.layoutOf().console.contentW
	global, _ := conColumns(innerW)
	group := conGroupColumn("cmd.group.playback", global)
	if group >= global {
		t.Fatalf("la premisa no se cumple: columna de grupo %d, global %d", group, global)
	}
	desc := i18n.T("cli.next")
	for _, l := range m.conLines {
		if strings.HasPrefix(l, "  next ") {
			if col := strings.Index(l, desc); col != conMarkW+group+2 {
				t.Fatalf("«next»: descripción en la columna %d, quería %d: %q", col, conMarkW+group+2, l)
			}
			return
		}
	}
	t.Fatalf("no encontré la fila de next: %q", m.conLines)
}

// TestConSinElipsisFalsa: clip() reserva la celda de "…" aunque el texto
// quepa justo. La cabecera de categoría mide exactamente el ancho interior y
// la entrada con el cursor al final medía una celda de más: las dos salían
// con una "…" que no ocultaba nada.
func TestConSinElipsisFalsa(t *testing.T) {
	old := i18n.Code()
	t.Cleanup(func() { i18n.Set(old) })
	i18n.Set("es")
	m := newConModel()
	m.openConsole()
	m.width, m.height = 150, 46
	m.conHelp()
	m.conScroll = len(m.conLines) // al principio: la primera cabecera
	conType(m, "vol")
	out := m.consoleView()
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, "…") {
			t.Errorf("elipsis sin nada oculto: %q", strings.TrimSpace(l))
		}
	}
}
