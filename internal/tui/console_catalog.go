package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"maly/internal/commandmeta"
	"maly/internal/i18n"
)

// Piezas visuales de la paleta. Comparten el lenguaje de la ayuda (teclas en
// accent negrita, descripción en texto) y añaden identidad propia: las
// cabeceras de categoría repiten la forma del título del marco ("─ Título ─"),
// y la fila elegida es una banda con ❯, el mismo glifo del prompt, porque
// Enter la va a escribir ahí. La banda de selección es la ÚNICA banda: si las
// cabeceras también lo fueran, no se distinguiría qué está elegido.

// conMark es el marcador textual de la fila elegida; sobrevive a NO_COLOR.
const conMark = "❯ "

// conMarkW es su ancho en celdas: len() cuenta bytes y ❯ ocupa tres.
const conMarkW = 2

// conMinDescW es lo mínimo que necesita la columna de descripción para que
// valga la pena ponerla junto al uso; por debajo, cada uno va en su fila.
const conMinDescW = 18

// conMinUsageW es la columna de uso más estrecha que admite el modo de dos
// columnas: por debajo casi todos los usos la desbordarían y la paleta pasa
// al compacto (uso solo, con la descripción de la elegida debajo).
const conMinUsageW = 20

// conColumns decide sobre el catálogo ENTERO si hay sitio para la columna de
// descripción y dónde va cuando se filtra: medida sobre todas las entradas y
// no sobre las coincidencias, la descripción no salta mientras se escribe.
// Un uso más ancho que la columna no la empuja en las demás filas: se escribe
// entero en la suya (sugerencias) o baja a su propia fila (ayuda).
func conColumns(innerW int) (usageW int, wide bool) {
	for _, e := range commandmeta.Console() {
		usageW = max(usageW, lipgloss.Width(e.Usage()))
	}
	room := innerW - conMarkW - 2 - conMinDescW // lo que deja la descripción
	return min(usageW, room), room >= conMinUsageW
}

// conGroupColumn es la columna de uso de UNA categoría, para cuando se ve
// bajo su cabecera (catálogo y ayuda): como cada tarjeta de la ayuda, se
// alinea con sus vecinas y no con el uso más largo de todo el catálogo, que
// alejaba «next» de su descripción tanto como «playlist import …».
func conGroupColumn(group string, usageW int) int {
	w := 0
	for _, e := range commandmeta.Console() {
		if e.Group == group {
			w = max(w, lipgloss.Width(e.Usage()))
		}
	}
	return min(w, usageW)
}

// conFit es clip sin su sorpresa: truncate reserva la celda de "…" aunque el
// texto quepa justo, así que clip(s, w) recorta un s de EXACTAMENTE w celdas.
// La columna de uso se mide al ancho del uso más largo, que caería siempre
// en ese caso. (clip no se cambia aquí: lo usa toda la TUI.)
func conFit(s string, w int) string {
	if lipgloss.Width(s) <= w {
		return s
	}
	return clip(s, w)
}

// conUsage pinta nombre y argumentos por separado: el nombre es lo que se
// teclea, los argumentos lo que falta rellenar. Sin estilos ancho == w.
func (m *Model) conUsage(e commandmeta.Entry, w int) string {
	u := conFit(e.Usage(), w)
	name := m.st.accent.Bold(true)
	if rest, ok := strings.CutPrefix(u, e.Name); ok {
		return padTo(name.Render(e.Name)+m.st.dim.Render(rest), w)
	}
	return padTo(name.Render(u), w)
}

// conEntryRow es una fila de sugerencia: siempre UNA línea de innerW celdas,
// para que la ventana que sigue a la selección cuente filas sin sorpresas.
// El uso manda sobre la descripción: es lo que hay que teclear. Uno más ancho
// que su columna se escribe entero y la descripción de ESA fila se corre o,
// si ya no le queda sitio, se omite (la elegida la repite el pie compacto).
func (m *Model) conEntryRow(e commandmeta.Entry, innerW, usageW int, wide, chosen bool) string {
	usageW = min(max(usageW, lipgloss.Width(e.Usage())), innerW-conMarkW)
	descW := innerW - conMarkW - usageW - 2
	desc := ""
	if wide && descW >= conMinDescW/2 {
		desc = conFit(i18n.T(e.DescKey), descW)
	}
	if chosen {
		text := conMark + padTo(conFit(e.Usage(), usageW), usageW)
		if desc != "" {
			text += "  " + desc
		}
		return m.st.selected.Render(padTo(text, innerW))
	}
	row := "  " + m.conUsage(e, usageW)
	if desc != "" {
		row += "  " + m.st.text.Render(desc)
	}
	return row
}

// conEntryBlock es la entrada completa para la ayuda: la descripción se
// envuelve en vez de recortarse, y un uso que no cabe baja a su propia fila
// en vez de pegarse a la descripción (el defecto de la tabla fija de 22).
func (m *Model) conEntryBlock(e commandmeta.Entry, innerW, usageW int, wide bool) []string {
	if !wide {
		lines := []string{"  " + m.conUsage(e, innerW-2)}
		for _, l := range wrappedLines(i18n.T(e.DescKey), innerW-4, 4) {
			lines = append(lines, "    "+m.st.text.Render(l))
		}
		return lines
	}
	indent := strings.Repeat(" ", conMarkW+usageW+2)
	descW := innerW - len(indent)
	desc := wrappedLines(i18n.T(e.DescKey), descW, 4)
	var lines []string
	if lipgloss.Width(e.Usage()) > usageW {
		lines = append(lines, "  "+m.conUsage(e, innerW-2))
	} else if len(desc) > 0 {
		lines = append(lines, "  "+m.conUsage(e, usageW)+"  "+m.st.text.Render(desc[0]))
		desc = desc[1:]
	}
	for _, l := range desc {
		lines = append(lines, indent+m.st.text.Render(l))
	}
	return lines
}

// conHeading separa categorías con la misma forma que el título del marco.
func (m *Model) conHeading(groupKey string, w int) string {
	title := conFit(i18n.T(groupKey), max(1, w-4))
	rest := max(0, w-3-lipgloss.Width(title))
	return m.st.border.Render("─ ") + m.st.titleFocus.Render(title) + " " + m.st.border.Render(strings.Repeat("─", rest))
}

// conHintLines empaqueta el pie por acciones completas: ninguna se parte
// entre dos filas, y la tecla (lo primero de cada acción) va en accent.
// Una acción más ancha que la caja se envuelve como texto plano atenuado.
func (m *Model) conHintLines(text string, w, maxLines int) []string {
	sep := " · "
	var rows [][]string
	for _, seg := range strings.Split(text, sep) {
		n := len(rows)
		if n > 0 && lipgloss.Width(strings.Join(append(rows[n-1], seg), sep)) <= w {
			rows[n-1] = append(rows[n-1], seg)
			continue
		}
		rows = append(rows, []string{seg})
	}
	var out []string
	for _, row := range rows {
		if lipgloss.Width(row[0]) > w {
			out = append(out, styledLines(row[0], w, maxLines, m.st.dim)...)
			continue
		}
		parts := make([]string, len(row))
		for i, seg := range row {
			key, rest, _ := strings.Cut(seg, " ")
			parts[i] = m.st.accent.Bold(true).Render(key)
			if rest != "" {
				parts[i] += " " + m.st.dim.Render(rest)
			}
		}
		out = append(out, strings.Join(parts, m.st.dim.Render(sep)))
	}
	if len(out) > maxLines {
		out = out[:maxLines]
	}
	return out
}
