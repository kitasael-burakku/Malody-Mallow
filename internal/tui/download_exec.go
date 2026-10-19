package tui

import (
	"io"
	"os/exec"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"maly/internal/i18n"
	"maly/internal/safetext"
)

// downloadExec conserva el progreso en el terminal y una cola de diagnóstico
// en memoria. No interpreta la salida humana ni decide el éxito: eso sigue
// perteneciendo a getter.Collect. No escribe logs persistentes con URLs/cookies.
type downloadExec struct {
	*exec.Cmd
	tail downloadTail
}

const downloadTailLimit = 16 * 1024

type downloadTail struct {
	mu   sync.Mutex // stdout y stderr pueden escribir desde goroutines distintas
	data []byte
}

func (t *downloadTail) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	n := len(p)
	if n >= downloadTailLimit {
		t.data = append(t.data[:0], p[n-downloadTailLimit:]...)
	} else {
		keep := min(len(t.data), downloadTailLimit-n)
		copy(t.data, t.data[len(t.data)-keep:])
		t.data = t.data[:keep]
		t.data = append(t.data, p...)
	}
	return n, nil
}
func (t *downloadTail) lines() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	// CR también separa actualizaciones de progreso; el saneado es de formato,
	// no un parser de mensajes de yt-dlp. Cada línea pasa por la frontera segura.
	text := strings.ReplaceAll(string(t.data), "\r", "\n")
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		line = safetext.Clean(ansi.Strip(line))
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) > 40 {
		lines = lines[len(lines)-40:]
	}
	return lines
}
func (c *downloadExec) SetStdin(r io.Reader)  { c.Stdin = r }
func (c *downloadExec) SetStdout(w io.Writer) { c.Stdout = io.MultiWriter(w, &c.tail) }
func (c *downloadExec) SetStderr(w io.Writer) { c.Stderr = io.MultiWriter(w, &c.tail) }
func execDownload(cmd *exec.Cmd, done func(error, []string) tea.Msg) tea.Cmd {
	c := &downloadExec{Cmd: cmd}
	return tea.Exec(c, func(err error) tea.Msg { return done(err, c.tail.lines()) })
}
func (m *Model) printDownloadOutput(lines []string) {
	if len(lines) == 0 {
		return
	}
	m.conPrint(m.st.accent.Bold(true).Render(i18n.T("get.output_tail")))
	for _, line := range lines {
		m.conPrint(m.st.text.Render(safetext.Clean(line)))
	}
}
