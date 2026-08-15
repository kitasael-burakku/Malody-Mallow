package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"maly/internal/config"
)

// La barra de reproducción. Funciones PURAS (no tocan el Model): reciben
// posición, duración, ancho y el tema, y devuelven las líneas ya coloreadas.
// El Model solo aporta el tema, vía los métodos puente del final.

// eighths son los bloques horizontales de octavo: dan 8× la resolución de una
// celda, así la barra avanza suave en vez de saltar de columna en columna. Son
// horizontales A PROPÓSITO y no los del visualizador (▁▂▃▄▅▆▇█), que llenan
// por ALTURA y no sirven para partir una celda a lo ancho; la familia visual
// con el espectro la mantienen los otros tres caracteres (█▓▒░), que son del
// mismo juego de bloques.
var eighths = [8]rune{' ', '▏', '▎', '▍', '▌', '▋', '▊', '▉'}

const (
	// progFade es la estela que disuelve el corte entre el tramo reproducido
	// y la pista, en vez de cortar en seco: dos celdas de densidad
	// decreciente justo después de la cabeza.
	progFade = "▓▒"
	// progTrack es la pista todavía no reproducida.
	progTrack = "░"
	// progMinWidth: por debajo de esto no entra nada legible (cabeza +
	// estela + algo de pista), así que la barra no se dibuja.
	progMinWidth = 4
	// progSteps cuantiza el gradiente del tramo lleno. Las celdas contiguas
	// del mismo paso se pintan en UN solo Render — el mismo motivo por el que
	// el banner agrupa sus columnas: sin eso, una barra de 200 celdas emite
	// 200 secuencias ANSI por frame.
	progSteps = 16
)

// progressBar dibuja la barra en exactamente w celdas: tramo reproducido con
// gradiente progress_low → progress_high, cabeza con precisión de octavo,
// estela de disolución y pista.
//
// Devuelve "" cuando no hay nada que dibujar (duración desconocida o ancho
// insuficiente); el llamador rellena, que es lo que hace panel() de todas
// formas.
//
// Los rechazos de arriba están escritos como !(dur > 0) y no como dur <= 0 a
// propósito: NaN es false en TODA comparación, así que `dur <= 0` lo deja
// pasar. Lo mismo con el clamp del ratio. Importa porque Position y Duration
// vienen de mpv por JSON, y un no finito llegaba antes hasta strings.Repeat
// —int(+Inf) en amd64 da el mínimo de int64— que entra en pánico con conteos
// negativos y se llevaba la TUI por delante.
func progressBar(pos, dur float64, w int, th config.Theme) string {
	if !(dur > 0) || w < progMinWidth {
		return ""
	}
	ratio := pos / dur
	if !(ratio > 0) { // atrapa NaN y negativos
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	units := int(ratio * float64(w) * 8)
	full, rem := units/8, units%8
	if full > w {
		full, rem = w, 0
	}

	var b strings.Builder
	cells := 0
	// Tramo reproducido, agrupando las celdas contiguas del mismo paso.
	for i := 0; i < full; {
		step := progStep(i, w)
		j := i
		for j < full && progStep(j, w) == step {
			j++
		}
		b.WriteString(progColor(th, step).Render(strings.Repeat("█", j-i)))
		cells += j - i
		i = j
	}
	// Cabeza: la fracción de celda que el tramo lleno no alcanza a cubrir.
	if rem > 0 && cells < w {
		b.WriteString(progColor(th, progStep(cells, w)).Render(string(eighths[rem])))
		cells++
	}
	// Estela: solo si algo se reprodujo (sin cabeza no hay nada que
	// disolver) y si queda ancho. Va del color de la cabeza al de la pista.
	if cells > 0 {
		head := progHex(th, progStep(cells-1, w))
		fade := []rune(progFade)
		for k, ch := range fade {
			if cells >= w {
				break
			}
			t := float64(k+1) / float64(len(fade)+1)
			b.WriteString(blendColor(head, th.Border, t).Render(string(ch)))
			cells++
		}
	}
	if rest := w - cells; rest > 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(th.Border)).
			Render(strings.Repeat(progTrack, rest)))
	}
	return b.String()
}

// progressShadow es la segunda fila de la barra: el tramo ya reproducido
// repetido en ▀ (medio bloque superior, o sea pegado a la barra de arriba),
// desplazado una columna a la derecha y en el color de sombra del tema. Solo
// tiene sentido donde sobra altura — en la vista principal la fila saldría de
// la cola, que es justo lo que el rediseño viene a recuperar.
//
// La sombra es más corta que w a propósito (llega hasta donde llegó la
// reproducción), así que su ancho renderizado NO es w; lo único que garantiza
// es no pasarse.
func progressShadow(pos, dur float64, w int, th config.Theme) string {
	if !(dur > 0) || w < progMinWidth {
		return ""
	}
	ratio := pos / dur
	if !(ratio > 0) {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	n := int(ratio * float64(w))
	if n > w-1 { // el desplazamiento de una columna se paga del ancho
		n = w - 1
	}
	if n <= 0 {
		return ""
	}
	return " " + lipgloss.NewStyle().Foreground(lipgloss.Color(th.ProgressShadow)).
		Render(strings.Repeat("▀", n))
}

// progStep mapea la celda i de una barra de w a un paso del gradiente.
func progStep(i, w int) int {
	if w <= 1 {
		return 0
	}
	return i * (progSteps - 1) / (w - 1)
}

// progHex es el color del paso step del gradiente de la barra.
func progHex(th config.Theme, step int) string {
	return blendHex(th.ProgressLow, th.ProgressHigh, float64(step)/float64(progSteps-1))
}

func progColor(th config.Theme, step int) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(progHex(th, step)))
}

// progressBar/progressShadow del Model: el único puente entre las funciones
// puras y el tema cargado. Fuente única para el panel "Ahora suena" del
// layout normal y la capa ctrl+t, que en su día tenían el cálculo duplicado
// letra por letra (hallazgo #8 de la auditoría de la 1.6.1).
func (m *Model) progressBar(pos, dur float64, w int) string {
	return progressBar(pos, dur, w, m.st.theme)
}

func (m *Model) progressShadow(pos, dur float64, w int) string {
	return progressShadow(pos, dur, w, m.st.theme)
}
