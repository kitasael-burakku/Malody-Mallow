package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"maly/internal/commandmeta"
	"maly/internal/i18n"
)

// El rectángulo visible se mide en el frame, sin usar el cálculo que lo dibuja.
func conVisibleRect(view string) [4]int {
	rect := [4]int{-1, -1, -1, -1}
	for y, line := range strings.Split(ansi.Strip(view), "\n") {
		if x := strings.IndexRune(line, '╭'); x >= 0 {
			rect[0], rect[1] = x, y
			rect[2] = ansi.StringWidth(strings.TrimSpace(line))
		}
		if strings.ContainsRune(line, '╰') {
			rect[3] = y - rect[1] + 1
		}
	}
	return rect
}

func TestConsoleStableRoom(t *testing.T) {
	defer i18n.Set(i18n.Code())
	for _, lang := range []string{"es", "en"} {
		i18n.Set(lang)
		for _, size := range [][4]int{{140, 42, 132, 38}, {100, 28, 94, 26}, {80, 24, 74, 22}, {40, 12, 40, 12}} {
			m := newConModel()
			m.openConsole()
			m.width, m.height = size[0], size[1]
			initial := conVisibleRect(m.consoleView())
			if initial[2] != size[2] || initial[3] != size[3] {
				t.Fatalf("%s %v: marco %v", lang, size, initial)
			}
			for _, query := range []string{"playlist", "vol +5", "zzzzzz", ""} {
				m.conInput.SetValue(query)
				view := m.consoleView()
				if got := conVisibleRect(view); got != initial {
					t.Fatalf("%s %v %q: saltó de %v a %v", lang, size, query, initial, got)
				}
				assertUX02Frame(t, view, m.width, m.height)
				if !strings.Contains(view, "esc") {
					t.Fatal("salida de la paleta oculta", view)
				}
			}
			m.conInput.SetValue("playlist")
			conKey(m, tea.KeyDown)
			for i := 0; i < 50; i++ {
				m.conPrint("resultado con texto largo " + strings.Repeat("detalle ", 12))
			}
			conKey(m, tea.KeyCtrlHome)
			view := m.consoleView()
			if got := conVisibleRect(view); got != initial {
				t.Fatal("la salida alteró el marco", got, initial)
			}
			if !strings.Contains(view, "ctrl+end") {
				t.Fatal("falta la señal de salida desplazada", view)
			}
		}
	}
}

func TestConsoleArrowsAndExplicitHistory(t *testing.T) {
	m := newConModel()
	m.openConsole()
	m.conHistory = []string{"vol 23"}
	m.conHistIdx = 1
	conType(m, "pl")
	conKey(m, tea.KeyDown)
	if m.conChoice != "play" || m.conInput.Value() != "pl" {
		t.Fatal("↓ no entró en las sugerencias", m.conChoice, m.conInput.Value())
	}
	conKey(m, tea.KeyDown)
	if m.conChoice != "playnext" {
		t.Fatal("↓ no avanzó", m.conChoice)
	}
	conKey(m, tea.KeyUp)
	if m.conChoice != "play" {
		t.Fatal("↑ recuperó historial en vez de navegar", m.conChoice)
	}
	// Consultar el historial es explícito mientras se navegan sugerencias.
	conKey(m, tea.KeyCtrlUp)
	if m.conInput.Value() != "vol 23" || m.conChoice != "" {
		t.Fatal("Ctrl+↑ no recuperó historial")
	}
	conKey(m, tea.KeyCtrlDown)
	if m.conInput.Value() != "pl" {
		t.Fatal("se perdió el borrador")
	}
	conKey(m, tea.KeyUp)
	if m.conInput.Value() != "vol 23" {
		t.Fatal("↑ fuera de la lista dejó de recuperar historial")
	}
	conKey(m, tea.KeyDown)
	if m.conInput.Value() != "pl" || m.conChoice != "" {
		t.Fatal("↓ en historial no devolvió el borrador")
	}
	conKey(m, tea.KeyDown)
	if cmd := conKey(m, tea.KeyEnter); cmd != nil || m.conInput.Value() != "play " {
		t.Fatal("elegir ejecutó una acción")
	}
}

func TestConsoleEmptySearchAndFullPreview(t *testing.T) {
	defer i18n.Set(i18n.Code())
	for _, lang := range []string{"es", "en"} {
		i18n.Set(lang)
		m := newConModel()
		m.openConsole()
		m.width, m.height = 80, 24
		conType(m, "zzzzzz")
		view := m.consoleView()
		if !strings.Contains(view, i18n.Tf("con.count", 0)) || !strings.Contains(view, strings.Fields(i18n.T("con.no_matches"))[0]) {
			t.Fatal("búsqueda vacía silenciosa", view)
		}
		m.conInput.SetValue("playlist")
		conKey(m, tea.KeyDown)
		view = ansi.Strip(m.consoleView())
		e, _ := commandmeta.Lookup("playlist")
		// La descripción larga debe verse completa en la vista previa, no solo su prefijo.
		compact := func(s string) string { return strings.Join(strings.Fields(strings.ReplaceAll(s, "│", " ")), "") }
		if !strings.Contains(compact(view), compact(i18n.T(e.DescKey))) {
			t.Fatal("vista previa recortada", view)
		}
		m.conInput.SetValue("vol +5")
		m.conChoice = ""
		if strings.Contains(m.consoleView(), i18n.Tf("con.count", 0)) {
			t.Fatal("argumentos confundidos con búsqueda sin resultados")
		}
	}
}
