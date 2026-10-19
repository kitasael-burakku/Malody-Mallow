package tui

import (
	"bytes"
	"errors"
	"os/exec"
	"strings"
	"sync"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"maly/internal/i18n"
)

func TestDesktopConsoleWrapKeepsTextAndStyle(t *testing.T) {
	m := newConModel()
	m.openConsole()
	m.width, m.height = 100, 28
	message := "Inicio: no hay eliminación disponible, caducó o fue sustituida. Detalle con 東京 y una_ruta_sin_espacios_muy_larga_que_tambien_debe_leerse_hasta FINAL_OK."
	m.conPrint("\x1b[31m" + message + "\x1b[0m")
	rows := m.consoleOutput(30)
	if len(rows) < 3 {
		t.Fatal("la salida no se envolvió", rows)
	}
	var plain strings.Builder
	for _, row := range rows {
		if ansi.StringWidth(row) > 30 {
			t.Fatal("desbordó", row)
		}
		if !strings.Contains(row, "\x1b[31m") {
			t.Fatal("perdió el color al continuar", row)
		}
		plain.WriteString(ansi.Strip(row))
	}
	normalize := func(s string) string { return strings.Join(strings.Fields(s), "") }
	if normalize(plain.String()) != normalize(message) {
		t.Fatal("se perdió parte del mensaje", plain.String())
	}
	if !strings.Contains(m.consoleView(), "FINAL_OK.") {
		t.Fatal("la vista sigue recortando el final")
	}
}

func TestDesktopConsoleScrollUsesVisualRows(t *testing.T) {
	m := newConModel()
	m.openConsole()
	m.width, m.height = 40, 12
	m.conPrint("PRINCIPIO " + strings.Repeat("detalle extenso ", 70) + " FINAL")
	if !strings.Contains(m.consoleView(), "FINAL") {
		t.Fatal("no aparece el final")
	}
	m.handleConsoleKey(tea.KeyMsg{Type: tea.KeyCtrlHome})
	out := m.consoleView()
	if !strings.Contains(out, "PRINCIPIO") || strings.Contains(out, "FINAL") {
		t.Fatal("Ctrl+Home contó entradas, no filas", out)
	}
	m.width = 100
	assertUX02Frame(t, m.consoleView(), 100, 12)
	m.handleConsoleKey(tea.KeyMsg{Type: tea.KeyCtrlEnd})
	if !strings.Contains(m.consoleView(), "FINAL") {
		t.Fatal("Ctrl+End no vuelve al final tras resize")
	}
}

func TestDesktopMinimumNoticeKeepsDimensions(t *testing.T) {
	defer i18n.Set(i18n.Code())
	for _, lang := range []string{"es", "en"} {
		i18n.Set(lang)
		for _, w := range []int{20, 39} {
			m := newHelpTestModel(w, 11)
			out := m.View()
			if !strings.Contains(out, "40×12") {
				t.Fatal("mínimo invisible", out)
			}
			assertUX02Frame(t, out, w, 11)
		}
	}
}

func TestDesktopHelpCardsAndResponsiveLayout(t *testing.T) {
	defer i18n.Set(i18n.Code())
	for _, lang := range []string{"es", "en"} {
		i18n.Set(lang)
		m := newHelpTestModel(200, 60)
		out := m.helpView()
		sameRow := false
		for _, line := range strings.Split(out, "\n") {
			if strings.Contains(line, i18n.T("help.group_playback")) && strings.Contains(line, i18n.T("help.group_library")) {
				sameRow = true
			}
		}
		if !sameRow {
			t.Fatal("la ayuda ancha sigue en una sola columna")
		}
		for _, size := range [][2]int{{140, 42}, {100, 28}, {80, 24}, {40, 12}} {
			m.width, m.height = size[0], size[1]
			for _, scroll := range []int{0, 15, 10000} {
				m.helpScroll = scroll
				out = m.helpView()
				assertUX02Frame(t, out, size[0], size[1])
				if !strings.Contains(out, "esc") {
					t.Fatal("salida de ayuda perdida")
				}
			}
			if !strings.Contains(out, i18n.T("help.quit")) {
				t.Fatal("no alcanza el último grupo", out)
			}
		}
	}
}

func TestDesktopDownloadExecRetainsSafeDiagnostics(t *testing.T) {
	c := &downloadExec{Cmd: exec.Command("/bin/sh", "-c", "printf 'progreso\\n'; printf '\\033[31mHTTP 404: Tokyo 東京\\033[0m\\n\\033]52;c;intruso\\007' >&2; exit 7")}
	var stdout, stderr bytes.Buffer
	c.SetStdout(&stdout)
	c.SetStderr(&stderr)
	c.SetStdin(strings.NewReader(""))
	err := c.Run()
	if err == nil || !strings.Contains(stderr.String(), "HTTP 404") || stdout.String() != "progreso\n" {
		t.Fatal("no conservó el proceso en el terminal", err)
	}
	lines := c.tail.lines()
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "HTTP 404: Tokyo 東京") {
		t.Fatal("no conservó diagnóstico", lines)
	}
	if strings.ContainsAny(joined, "\x1b\x07\u009b\u009d") {
		t.Fatal("controles en el diagnóstico", joined)
	}
	for _, playlist := range []bool{false, true} {
		m := newConModel()
		m.openConsole()
		if playlist {
			m.Update(getPlaylistDoneMsg{err: errors.New("exit status 7"), output: lines})
		} else {
			m.finishDownload(getDoneMsg{err: errors.New("exit status 7"), output: lines, fromModal: true, dir: t.TempDir(), before: map[string]bool{}})
		}
		output := strings.Join(m.conLines, "\n")
		if !strings.Contains(output, "HTTP 404") || !strings.Contains(output, i18n.T("get.output_tail")) {
			t.Fatal("el regreso a TUI perdió el diagnóstico", playlist, output)
		}
	}
}

func TestDesktopDownloadTailBoundedAndConcurrent(t *testing.T) {
	var tail downloadTail
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Go(func() {
			for j := 0; j < 100; j++ {
				tail.Write([]byte(strings.Repeat("avance\n", 100)))
			}
		})
	}
	wg.Wait()
	tail.Write([]byte("\nERROR FINAL 東京\n"))
	if len(tail.data) > downloadTailLimit {
		t.Fatal("memoria sin límite", len(tail.data))
	}
	lines := tail.lines()
	if len(lines) > 40 || lines[len(lines)-1] != "ERROR FINAL 東京" {
		t.Fatal("no preserva el final acotado", lines)
	}
	tail.Write([]byte(strings.Repeat("x", downloadTailLimit*2) + "\nOTRO FINAL\n"))
	if len(tail.data) != downloadTailLimit || !strings.Contains(strings.Join(tail.lines(), "\n"), "OTRO FINAL") {
		t.Fatal("bloque grande no acotado")
	}
}
