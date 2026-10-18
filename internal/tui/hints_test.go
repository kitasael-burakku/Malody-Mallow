package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"maly/internal/i18n"
	"maly/internal/ipc"
	"maly/internal/library"
)

// Pruebas a través de las vistas y mensajes existentes: compilan también
// con la producción anterior y fallan por el contenido perdido o falso.
func TestUX02ContextAndBindings(t *testing.T) {
	defer i18n.Set(i18n.Code())
	for _, lang := range []string{"es", "en"} {
		i18n.Set(lang)
		for _, size := range [][2]int{{140, 42}, {100, 28}, {80, 24}, {40, 12}, {89, 24}, {90, 24}, {119, 28}, {120, 28}} {
			t.Run(fmt.Sprintf("%s/%dx%d", lang, size[0], size[1]), func(t *testing.T) {
				m := newLayoutTestModel(size[0], size[1])
				m.tree = buildTree([]library.Track{{Title: "Luz", Artist: "Áurea", Album: "Uno", Path: "/m/luz.flac"}}, nil)
				m.keys["help"] = "f1"
				m.keys["quit"] = "f2"
				m.keys["filter"] = "f6"
				m.keys["add"] = "f7"
				m.keys["switch_panel"] = "f8"
				m.keys["remove"] = "f9"
				m.keys["play_pause"] = "ctrl+shift+p"
				want := []string{"f1 " + i18n.T("hint.help"), "f2 " + i18n.T("help.quit"), "f8"}
				if size[0] >= 80 {
					want = append(want, "f6 "+i18n.T("hint.filter"), "f7 "+i18n.T("hint.add"), "enter "+i18n.T("hint.expand"))
				}
				checkStateText(t, m.footer(), want, "? help", "? ayuda", "q quit", "q salir", "/ filter", "/ filtra")
				if lipgloss.Width(m.footer()) != size[0] {
					t.Fatal("pie desbordado")
				}
				m.handleKey(tea.KeyMsg{Type: tea.KeyEnter}) // expandir artista
				if !m.tree.current().expanded {
					t.Fatal("el control positivo no expandió")
				}
				if size[0] >= 80 {
					checkStateText(t, m.footer(), []string{"enter " + i18n.T("hint.collapse")}, "enter "+i18n.T("hint.play"))
				}
				m.applyFilter("Luz")
				if size[0] >= 80 {
					checkStateText(t, m.footer(), []string{"enter " + i18n.T("hint.play")}, "enter "+i18n.T("hint.expand"))
				}
				m.focus = panelQueue
				checkStateText(t, m.footer(), []string{"f8 " + i18n.T("hint.to_library")}, "f7 "+i18n.T("hint.add"))
				if size[0] >= 80 {
					checkStateText(t, m.footer(), []string{"f9 " + i18n.T("hint.remove")})
				}
				m.keys["palette"] = "ctrl+shift+c"
				m.Update(libraryMsg{})
				checkStateText(t, m.flash, []string{"ctrl+shift+c"}, "ctrl+p")
			})
		}
	}
}

func TestUX02WrappedStatesAndPickerExit(t *testing.T) {
	defer i18n.Set(i18n.Code())
	for _, lang := range []string{"es", "en"} {
		i18n.Set(lang)
		for _, size := range [][2]int{{140, 42}, {100, 28}, {80, 24}, {40, 12}} {
			t.Run(fmt.Sprintf("%s/%dx%d", lang, size[0], size[1]), func(t *testing.T) {
				m := newLayoutTestModel(size[0], size[1])
				m.queue = nil
				m.status = &ipc.Status{}
				m.keys["add"] = "f7"
				// El remedio debe caber en la columna fija, no solo en un panel ancho.
				checkStateText(t, m.View(), []string{"maly scan"})
				m.focus = panelQueue
				checkStateText(t, m.View(), []string{"f7"}, "`a`")
				m.openSongs()
				out := m.View()
				checkStateText(t, out, []string{"esc", "maly scan"})
				assertUX02Frame(t, out, size[0], size[1])
				m.songs.setItems([]pickerItem{newPickerItem("Luz", "luz")})
				checkStateText(t, m.View(), []string{"esc", "enter", "tab", "Luz"})
				m.setFlash(strings.Repeat("falló una acción ", 30), true)
				assertUX02Frame(t, m.View(), size[0], size[1])
				checkStateText(t, m.View(), []string{"esc", "falló"})
				m.flash = ""
				m.openPlaylists(plTarget, []int64{1})
				m.Update(plListMsg{})
				checkStateText(t, m.View(), []string{"esc", "ctrl+n"})
				assertUX02Frame(t, m.View(), size[0], size[1])
				m.openConsole()
				m.status.Scanning = true
				m.status.ScanSeen = 123
				out = m.View()
				checkStateText(t, out, []string{"esc", "123"})
				assertUX02Frame(t, out, size[0], size[1])
			})
		}
	}
}

func assertUX02Frame(t *testing.T, out string, w, h int) {
	t.Helper()
	lines := strings.Split(out, "\n")
	if len(lines) > h {
		t.Fatalf("%d filas, máximo %d", len(lines), h)
	}
	for i, line := range lines {
		if got := lipgloss.Width(line); got > w {
			t.Errorf("fila %d: %d celdas, máximo %d", i, got, w)
		}
	}
}

func TestUX02FooterPriorityAndReturn(t *testing.T) {
	m := newLayoutTestModel(80, 24)
	normal := m.footer()
	m.setFlash("respuesta observable", false)
	checkStateText(t, m.footer(), []string{"respuesta observable"}, "? help")
	m.status.Scanning = true
	m.status.ScanSeen = 42
	checkStateText(t, m.footer(), []string{"respuesta observable"}, "42")
	m.flashUntil = time.Now().Add(-time.Second)
	m.Update(tickMsg(time.Now()))
	checkStateText(t, m.footer(), []string{"42"}, "respuesta observable")
	m.status.Scanning = false
	if m.footer() != normal {
		t.Fatal("el pie no recuperó las acciones tras flash y scan")
	}
	// La última fila real sigue siendo el pie; no basta comprobar dimensiones.
	rows := strings.Split(m.View(), "\n")
	if ansi.Strip(rows[len(rows)-1]) != ansi.Strip(normal) {
		t.Fatal("las ayudas abandonaron su fila")
	}
}

func TestUX02HelpTasksAndLongKeys(t *testing.T) {
	defer i18n.Set(i18n.Code())
	for _, lang := range []string{"es", "en"} {
		i18n.Set(lang)
		m := newHelpTestModel(40, 12)
		m.keys["palette"] = "ctrl+shift+f12"
		m.keys["next"] = " "
		var all strings.Builder
		for _, r := range HelpRows(m.keys) {
			all.WriteString(r[0] + " " + r[1] + "\n")
		}
		checkStateText(t, all.String(), []string{i18n.T("help.group_playback"), i18n.T("help.group_library"), i18n.T("help.group_tools"), "ctrl+shift+f12", i18n.T("help.space")})
		// Recorrer el contenido, no comparar solo el ancho que panel() rellena.
		var screens strings.Builder
		for i := 0; i < 100; i++ {
			m.helpScroll = i
			out := m.helpView()
			assertUX02Frame(t, out, 40, 12)
			checkStateText(t, out, []string{"esc", "↑/↓"})
			screens.WriteString(ansi.Strip(out))
		}
		checkStateText(t, screens.String(), []string{"ctrl+shift+f12", i18n.T("help.quit"), i18n.T("help.group_tools")})
	}
}
