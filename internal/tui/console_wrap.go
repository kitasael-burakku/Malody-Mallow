package tui

import (
	"github.com/charmbracelet/x/ansi"
	"strings"
)

// consoleOutput envuelve antes de paginar. Cut conserva los estilos de cada
// tramo aun cuando el borde de la caja reinicia el color entre dos filas.
// No modifica el historial: un cambio de ancho puede volver a envolverlo.
func (m *Model) consoleOutput(width int) []string {
	var out []string
	for _, entry := range m.conLines {
		for _, line := range strings.Split(terminalPresentation(entry), "\n") {
			plain := ansi.Strip(line)
			rest := plain
			offset := 0
			for _, part := range strings.Split(ansi.Wrap(plain, max(1, width), ""), "\n") {
				skip := strings.Index(rest, part)
				if skip < 0 {
					skip = 0
				}
				offset += ansi.StringWidth(rest[:skip])
				n := ansi.StringWidth(part)
				out = append(out, ansi.Cut(line, offset, offset+n))
				offset += n
				rest = rest[min(len(rest), skip+len(part)):]
			}
		}
	}
	return out
}
