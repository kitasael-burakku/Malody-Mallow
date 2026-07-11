package tui

import (
	"fmt"
	"math"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// logoArt: "MALODY" en figlet fuente bloody, 6 líneas.
const logoArt = ` ███▄ ▄███▓ ▄▄▄       ██▓     ▒█████  ▓█████▄▓██   ██▓
▓██▒▀█▀ ██▒▒████▄    ▓██▒    ▒██▒  ██▒▒██▀ ██▌▒██  ██▒
▓██    ▓██░▒██  ▀█▄  ▒██░    ▒██░  ██▒░██   █▌ ▒██ ██░
▒██    ▒██ ░██▄▄▄▄██ ▒██░    ▒██   ██░░▓█▄   ▌ ░ ▐██▓░
▒██▒   ░██▒ ▓█   ▓██▒░██████▒░ ████▓▒░░▒████▓  ░ ██▒▓░
░ ▒░   ░  ░ ▒▒   ▓▒█░░ ▒░▓  ░░ ▒░▒░▒░  ▒▒▓  ▒   ██▒▒▒`

const (
	logoPanelH   = 8  // 6 líneas de arte + bordes
	logoMinRows  = 30 // altura mínima del terminal para mostrar el banner
	logoSteps    = 24 // resolución del gradiente precalculado
	logoFreq     = 0.22
	logoInterval = 80 * time.Millisecond // ~12.5 fps
)

type logoTickMsg time.Time

func logoTickCmd() tea.Cmd {
	return tea.Tick(logoInterval, func(t time.Time) tea.Msg { return logoTickMsg(t) })
}

// logoModel anima el logo con una onda horizontal: el color de cada columna
// recorre el gradiente Kitasan según sin(col·freq − fase). La energía del
// audio acelera la fase y abre el gradiente hacia el rojo; sin reproducción
// la onda queda lenta y en la zona cian.
type logoModel struct {
	cells  [][]rune // arte paddeado al mismo ancho
	width  int
	phase  float64
	energy float64 // 0..1 suavizada entre ticks
	ramp   []lipgloss.Style
}

func newLogo() logoModel {
	rows := strings.Split(logoArt, "\n")
	w := 0
	for _, r := range rows {
		if n := lipgloss.Width(r); n > w {
			w = n
		}
	}
	cells := make([][]rune, len(rows))
	for i, r := range rows {
		cells[i] = []rune(r + strings.Repeat(" ", w-lipgloss.Width(r)))
	}
	return logoModel{cells: cells, width: w, ramp: logoRamp()}
}

// logoRamp interpola cian → azul-grisáceo → rojo apagado en logoSteps pasos.
func logoRamp() []lipgloss.Style {
	stops := [][3]int{parseHex(kitasanCyan), parseHex(kitasanBlueGray), parseHex(kitasanRed)}
	ramp := make([]lipgloss.Style, logoSteps)
	for i := range ramp {
		f := float64(i) / float64(logoSteps-1) * float64(len(stops)-1)
		s := int(f)
		if s >= len(stops)-1 {
			s = len(stops) - 2
		}
		t := f - float64(s)
		var c [3]int
		for k := 0; k < 3; k++ {
			c[k] = stops[s][k] + int(t*float64(stops[s+1][k]-stops[s][k]))
		}
		ramp[i] = lipgloss.NewStyle().Foreground(
			lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", c[0], c[1], c[2])))
	}
	return ramp
}

// tick avanza la fase de la onda; energy (0..1) la acelera.
func (l *logoModel) tick(energy float64) {
	l.energy += (energy - l.energy) * 0.2
	l.phase += 0.10 * (1 + 6*l.energy)
}

// view devuelve las líneas del logo centradas en innerW, coloreadas por
// columna. Las runas contiguas con el mismo paso del gradiente se agrupan en
// un solo Render para no inflar la salida ANSI.
func (l *logoModel) view(innerW int) []string {
	pad := (innerW - l.width) / 2
	if pad < 0 {
		pad = 0
	}
	cols := l.width
	if cols > innerW-pad {
		cols = innerW - pad
	}
	// Con más energía el gradiente se abre desde el cian hacia el rojo.
	span := 0.35 + 0.65*l.energy
	idx := make([]int, cols)
	for c := range idx {
		v := 0.5 + 0.5*math.Sin(float64(c)*logoFreq-l.phase)
		i := int(v * span * float64(logoSteps-1))
		if i >= logoSteps {
			i = logoSteps - 1
		}
		idx[c] = i
	}
	prefix := strings.Repeat(" ", pad)
	out := make([]string, len(l.cells))
	for r, row := range l.cells {
		var b strings.Builder
		b.WriteString(prefix)
		for c := 0; c < cols; {
			j := c
			for j < cols && idx[j] == idx[c] {
				j++
			}
			b.WriteString(l.ramp[idx[c]].Render(string(row[c:j])))
			c = j
		}
		out[r] = b.String()
	}
	return out
}

// logoVisible: el banner se dibuja y su tick sigue vivo solo si hay altura
// suficiente y ninguna vista a pantalla completa lo tapa.
func (m *Model) logoVisible() bool {
	return m.height >= logoMinRows && m.width >= minWidth &&
		!m.langOpen && !m.showHelp && !m.consoleOpen && !m.songsOpen && !m.plOpen
}

// logoEnergy estima la energía del audio (0..1) como media de las barras del
// visualizador, ya refrescadas cada 60 ms; 0 si no suena nada o no hay datos.
func (m *Model) logoEnergy() float64 {
	if m.status == nil || m.status.Track == nil || m.status.Paused ||
		!m.vizOn || len(m.vizBars) == 0 {
		return 0
	}
	sum := 0.0
	for _, b := range m.vizBars {
		sum += b
	}
	e := sum / float64(len(m.vizBars))
	if e > 1 {
		e = 1
	}
	return e
}

func (m *Model) logoPanel(w, h int) string {
	return m.st.panel("", m.logo.view(w-2), w, h, false)
}
