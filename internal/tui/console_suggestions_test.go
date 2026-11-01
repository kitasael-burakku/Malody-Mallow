package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"maly/internal/commandmeta"
	"maly/internal/config"
	"maly/internal/i18n"
	"maly/internal/ipc"
	"maly/internal/library"
)

func conType(m *Model, s string) {
	m.handleConsoleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
}
func conKey(m *Model, k tea.KeyType) tea.Cmd {
	_, cmd := m.handleConsoleKey(tea.KeyMsg{Type: k})
	return cmd
}

func TestUX05FindAndFill(t *testing.T) {
	old := i18n.Code()
	t.Cleanup(func() { i18n.Set(old) })
	for _, tc := range []struct{ lang, query, want, usage string }{
		{"es", "pl", "play ", "play [<consulta>...]"},
		{"en", "pl", "play ", "play [<query>...]"},
		{"es", "elimina una playlist", "playlist delete ", "playlist delete <nombre>"},
		{"en", "delete a playlist", "playlist delete ", "playlist delete <name>"},
		{"es", "confirmacio\u0301n", "playlist delete ", "playlist delete <nombre>"},
		{"es", "playlist cr", "playlist create ", "playlist create <nombre>"},
		{"en", "maly playlist cr", "maly playlist create ", "playlist create <name>"},
	} {
		t.Run(tc.lang+"/"+tc.query, func(t *testing.T) {
			i18n.Set(tc.lang)
			m := newConModel()
			m.openConsole()
			m.width, m.height = 80, 24
			conType(m, tc.query)
			if !strings.Contains(m.consoleView(), tc.usage) {
				t.Fatalf("faltan sugerencia/argumentos %q: %s", tc.usage, m.consoleView())
			}
			if cmd := conKey(m, tea.KeyTab); cmd != nil {
				t.Fatal("Tab ejecutó una acción")
			}
			if cmd := conKey(m, tea.KeyEnter); cmd != nil {
				t.Fatal("aceptar una sugerencia ejecutó una acción")
			}
			if got := m.conInput.Value(); got != tc.want {
				t.Fatalf("entrada = %q, quería %q", got, tc.want)
			}
			if len(m.conHistory) != 0 || m.conPlConfirm != "" {
				t.Fatal("seleccionar alteró historial o confirmación")
			}
		})
	}
}

func TestUX05SelectionEditingHistory(t *testing.T) {
	m := newConModel()
	m.openConsole()
	m.keys = config.DefaultKeys()
	conType(m, "pl")
	conKey(m, tea.KeyTab)
	conKey(m, tea.KeyTab)
	if m.conChoice != "playnext" {
		t.Fatalf("segunda sugerencia = %q", m.conChoice)
	}
	conKey(m, tea.KeyShiftTab)
	if m.conChoice != "play" {
		t.Fatalf("retroceso = %q", m.conChoice)
	}
	conKey(m, tea.KeyEsc)
	if m.overlay != ovConsole || m.conInput.Value() != "pl" || m.conChoice != "" {
		t.Fatal("Escape no devolvió a edición")
	}
	conKey(m, tea.KeyTab)
	conKey(m, tea.KeyLeft)
	conType(m, "X")
	if m.conChoice != "" || m.conInput.Value() != "pXl" {
		t.Fatalf("edición: %q / %q", m.conChoice, m.conInput.Value())
	}
	conKey(m, tea.KeyHome)
	conType(m, "A")
	conKey(m, tea.KeyEnd)
	conType(m, "Z")
	if m.conInput.Value() != "ApXlZ" {
		t.Fatal(m.conInput.Value())
	}
	m.conInput.SetValue("cls")
	conKey(m, tea.KeyEnter)
	conType(m, "borrador con 'comillas")
	conKey(m, tea.KeyTab)
	conKey(m, tea.KeyUp)
	if m.conInput.Value() != "cls" || m.conChoice != "" {
		t.Fatal("historial mezclado con sugerencias")
	}
	conKey(m, tea.KeyDown)
	if m.conInput.Value() != "borrador con 'comillas" {
		t.Fatal("borrador perdido")
	}
	conKey(m, tea.KeyEsc)
	if m.overlay != ovNone {
		t.Fatal("Escape en edición no cerró")
	}
	m.openConsole()
	conKey(m, tea.KeyShiftTab)
	all := commandmeta.Console()
	if m.conChoice != all[len(all)-1].Name {
		t.Fatalf("Shift+Tab inicial no fue al final: %q", m.conChoice)
	}
	conKey(m, tea.KeyTab)
	if m.conChoice != "play" {
		t.Fatalf("ciclo no volvió al inicio: %q", m.conChoice)
	}
}

func TestUX05ActionAndQuotes(t *testing.T) {
	old := i18n.Code()
	t.Cleanup(func() { i18n.Set(old) })
	i18n.Set("es")
	for _, tc := range []struct {
		query, args string
		want        ipc.Request
	}{
		{"agrega", "'Otra dimensión' \"Don't Stop\"", ipc.Request{Cmd: "add", Queries: []string{"Otra dimensión", "Don't Stop"}}},
		{"vol", "+5", ipc.Request{Cmd: "vol", Value: "+5"}},
		{"remove", "3", ipc.Request{Cmd: "remove", Index: 2}},
		{"playlist play", "'Mi lista'", ipc.Request{Cmd: "playlist_play", Value: "Mi lista"}},
	} {
		t.Run(tc.query, func(t *testing.T) {
			sock, reqs := capturaReq(t)
			m := newConModel()
			m.sock = sock
			m.openConsole()
			conType(m, tc.query)
			conKey(m, tea.KeyTab)
			if conKey(m, tea.KeyEnter) != nil {
				t.Fatal("rellenar envió una acción")
			}
			conType(m, tc.args)
			got := esperaReq(t, conKey(m, tea.KeyEnter), reqs)
			got.Lang = ""
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("acción real = %+v, quería %+v", got, tc.want)
			}
		})
	}
	// El camino directo no requiere Tab, incluso con coincidencias visibles.
	sock, reqs := capturaReq(t)
	m := newConModel()
	m.sock = sock
	m.openConsole()
	conType(m, "next")
	if got := esperaReq(t, conKey(m, tea.KeyEnter), reqs); got.Cmd != "next" {
		t.Fatal(got)
	}
	for _, line := range []string{"add", "add ''", "add 'sin cerrar", "jump cero", "move 1", "remove 0", "playlist delete"} {
		m.conInput.SetValue(line)
		if conKey(m, tea.KeyEnter) != nil || m.conPlConfirm != "" {
			t.Fatalf("línea inválida ejecutada: %q", line)
		}
	}
}

func TestUX05DeleteConfirmation(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	lib, err := library.Create(config.DBPath())
	if err != nil {
		t.Fatal(err)
	}
	defer lib.Close()
	for _, response := range []string{"", "no", "'", "escape", "close", "yes"} {
		t.Run(response, func(t *testing.T) {
			name := "Mi lista " + response
			if err := lib.CreatePlaylist(name); err != nil {
				t.Fatal(err)
			}
			m := newConModel()
			m.keys = config.DefaultKeys()
			m.openConsole()
			conType(m, "playlist de")
			conKey(m, tea.KeyTab)
			if conKey(m, tea.KeyEnter) != nil || m.conPlConfirm != "" {
				t.Fatal("seleccionar inició el borrado")
			}
			if conKey(m, tea.KeyEnter) != nil || m.conPlConfirm != "" {
				t.Fatal("plantilla sin nombre inició el borrado")
			}
			m.conInput.SetValue("playlist delete '" + name + "'")
			conKey(m, tea.KeyEnter)
			if m.conPlConfirm != name {
				t.Fatalf("confirmación = %q", m.conPlConfirm)
			}
			before := m.conInput.Value()
			conKey(m, tea.KeyTab)
			if m.conChoice != "" || m.conInput.Value() != before {
				t.Fatal("sugerencias activas al confirmar")
			}
			if response == "escape" {
				conKey(m, tea.KeyEsc)
			} else if response == "close" {
				conKey(m, tea.KeyCtrlP)
				m.openConsole()
			} else {
				conType(m, response)
				cmd := conKey(m, tea.KeyEnter)
				if response == "yes" {
					if cmd == nil {
						t.Fatal("no se borró tras yes")
					}
					cmd()
				} else if cmd != nil {
					t.Fatal("cancelar produjo una acción")
				}
			}
			if m.conPlConfirm != "" {
				t.Fatal("confirmación obsoleta pendiente")
			}
			lists, err := lib.Playlists()
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, p := range lists {
				if p.Name == name {
					found = true
				}
			}
			if found == (response == "yes") {
				t.Fatalf("existencia real = %v tras %q", found, response)
			}
		})
	}
}

func TestUX05FramesAndScroll(t *testing.T) {
	old := i18n.Code()
	t.Cleanup(func() { i18n.Set(old) })
	for _, lang := range []string{"es", "en"} {
		t.Run(lang, func(t *testing.T) {
			i18n.Set(lang)
			m := newConModel()
			m.openConsole()
			for i := 0; i < 40; i++ {
				m.conPrint(fmt.Sprintf("salida-%02d", i))
			}
			conType(m, "playlist")
			for i := 0; i < len(m.conSuggestions()) && m.conChoice != "playlist import"; i++ {
				conKey(m, tea.KeyTab)
			}
			for _, size := range [][2]int{{140, 42}, {100, 28}, {80, 24}, {40, 12}, {140, 42}} {
				m.width, m.height = size[0], size[1]
				m.status = &ipc.Status{Scanning: true, ScanSeen: 17}
				out := m.consoleView()
				boxLinesFitWithin(t, out, m.layoutOf().console.width)
				boxLinesSameWidth(t, out, m.layoutOf().console.width)
				if lipgloss.Height(out) != m.height {
					t.Fatalf("alto %d en %v", lipgloss.Height(out), size)
				}
				for _, text := range []string{"❯ playlist import", "salida-39", "esc", "tab"} {
					if !strings.Contains(out, text) {
						t.Fatalf("%v perdió %q: %s", size, text, out)
					}
				}
				conKey(m, tea.KeyCtrlHome)
				out = m.consoleView()
				if !strings.Contains(out, "salida-00") || strings.Contains(out, "salida-39") {
					t.Fatalf("scroll inicio roto en %v", size)
				}
				conKey(m, tea.KeyCtrlEnd)
				if !strings.Contains(m.consoleView(), "salida-39") {
					t.Fatal("scroll final roto")
				}
			}
		})
	}
}

func TestUX05SubcommandDispatch(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	// Sin ejecutar el tea.Cmd: get puede lanzar herramientas externas. Se exige
	// la rama de uso específica, no solo ausencia de «comando desconocido».
	for name, key := range map[string]string{
		"playlist show": "pl.usage_show", "playlist create": "pl.usage_create",
		"playlist delete": "pl.usage_delete", "playlist add": "pl.usage_add",
		"playlist remove": "pl.usage_remove", "playlist play": "pl.usage_play",
		"playlist export": "pl.usage_export", "playlist import": "pl.usage_import",
		"get pick": "cli.usage_get_pick",
	} {
		m := newConModel()
		m.openConsole()
		conType(m, name)
		conKey(m, tea.KeyTab)
		conKey(m, tea.KeyEnter)
		if cmd := conKey(m, tea.KeyEnter); cmd != nil {
			t.Errorf("%s sin argumentos produjo acción", name)
		}
		if !strings.Contains(strings.Join(m.conLines, "\n"), i18n.T(key)) {
			t.Errorf("%s no llegó a validación específica: %q", name, m.conLines)
		}
	}
}

func TestUX05GetPickSearchesBeforeDownload(t *testing.T) {
	bin := t.TempDir()
	t.Setenv("PATH", bin)
	log := filepath.Join(bin, "args")
	t.Setenv("UX05_ARGS", log)
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$UX05_ARGS\"\nprintf '%s\\n' '{\"title\":\"Resultado\",\"url\":\"https://example.invalid/song\"}'\n"
	if err := os.WriteFile(filepath.Join(bin, "yt-dlp"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	m := newConModel()
	m.cfg.MusicDir = filepath.Join(t.TempDir(), "sin-descarga")
	m.openConsole()
	conType(m, `get pick "Otra dimensión"`)
	cmd := conKey(m, tea.KeyEnter)
	if m.overlay != ovGet || m.getQuery != "Otra dimensión" || m.getPhase != getSearching {
		t.Fatalf("no abrió búsqueda: overlay=%v query=%q", m.overlay, m.getQuery)
	}
	var run func(tea.Cmd)
	run = func(cmd tea.Cmd) {
		if cmd == nil {
			return
		}
		switch msg := cmd().(type) {
		case tea.BatchMsg:
			for _, c := range msg {
				run(c)
			}
		case getResultsMsg:
			m.applyGetResults(msg)
		}
	}
	run(cmd)
	defer m.stopGetSearchOnExit()
	b, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "--dump-json\n") || !strings.Contains(string(b), "ytsearch10:Otra dimensión\n") || strings.Contains(string(b), "--extract-audio") {
		t.Fatalf("argv equivocado: %s", b)
	}
	if m.getPhase != getResults || len(m.getResults) != 1 {
		t.Fatalf("no recibió el resultado: %v %q", m.getPhase, m.getErr)
	}
	if _, err := os.Stat(m.cfg.MusicDir); !os.IsNotExist(err) {
		t.Fatal("buscar preparó una descarga")
	}
}
