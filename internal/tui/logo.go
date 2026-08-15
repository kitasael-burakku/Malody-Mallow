package tui

import (
	"math"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"maly/internal/config"
)

// logoArt: "MALODY" en figlet fuente bloody, 6 líneas.
const logoArt = ` ███▄ ▄███▓ ▄▄▄       ██▓     ▒█████  ▓█████▄▓██   ██▓
▓██▒▀█▀ ██▒▒████▄    ▓██▒    ▒██▒  ██▒▒██▀ ██▌▒██  ██▒
▓██    ▓██░▒██  ▀█▄  ▒██░    ▒██░  ██▒░██   █▌ ▒██ ██░
▒██    ▒██ ░██▄▄▄▄██ ▒██░    ▒██   ██░░▓█▄   ▌ ░ ▐██▓░
▒██▒   ░██▒ ▓█   ▓██▒░██████▒░ ████▓▒░░▒████▓  ░ ██▒▓░
░ ▒░   ░  ░ ▒▒   ▓▒█░░ ▒░▓  ░░ ▒░▒░▒░  ▒▒▓  ▒   ██▒▒▒`

const (
	logoSteps    = 24 // resolución del gradiente precalculado
	logoFreq     = 0.22
	logoInterval = 80 * time.Millisecond // ~12.5 fps
)

type logoTickMsg time.Time

func logoTickCmd() tea.Cmd {
	return tea.Tick(logoInterval, func(t time.Time) tea.Msg { return logoTickMsg(t) })
}

// logoModel anima el logo con una onda horizontal: el color de cada columna
// recorre el gradiente configurado ([theme] logo) según sin(col·freq − fase).
// La energía del audio acelera la fase y abre el gradiente hacia la última
// parada; sin reproducción la onda queda lenta en la zona de la primera.
type logoModel struct {
	cells  [][]rune // arte paddeado al mismo ancho
	width  int
	phase  float64
	energy float64 // 0..1 suavizada entre ticks
	ramp   []lipgloss.Style
}

// newLogo arma el banner: art son las líneas de un logo.txt del usuario
// (config.Theme.LogoArt); vacío = el MALODY de fábrica.
func newLogo(stops, art []string) logoModel {
	rows := art
	if len(rows) == 0 {
		rows = strings.Split(logoArt, "\n")
	}
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
	return logoModel{cells: cells, width: w, ramp: logoRamp(stops)}
}

// artH/artW son las dimensiones del arte ya paddeado. Reemplazan a los viejos
// panelH/minRows, que medían un PANEL del banner dentro de la vista principal
// — el banner ya no vive ahí (ver [theme] banner): lo único que hace falta
// saber es si el arte cabe en pantalla para el splash.
func (l *logoModel) artH() int { return len(l.cells) }
func (l *logoModel) artW() int { return l.width }

// logoRamp interpola las paradas del gradiente del banner en logoSteps pasos.
func logoRamp(stops []string) []lipgloss.Style {
	if len(stops) < 2 {
		// Defensa por si llega un config mutado: la paleta Kitasan de siempre.
		stops = []string{kitasanCyan, kitasanBlueGray, kitasanRed}
	}
	ramp := make([]lipgloss.Style, logoSteps)
	for i := range ramp {
		// Posición dentro del gradiente multi-parada: la parte entera elige el
		// tramo y la fraccionaria interpola dentro de él (blendColor, el mismo
		// helper que usan el visualizador y las letras).
		f := float64(i) / float64(logoSteps-1) * float64(len(stops)-1)
		s := int(f)
		if s >= len(stops)-1 {
			s = len(stops) - 2
		}
		ramp[i] = blendColor(stops[s], stops[s+1], f-float64(s))
	}
	return ramp
}

// tick avanza la fase de la onda; energy (0..1) la acelera.
func (l *logoModel) tick(energy float64) {
	l.energy += (energy - l.energy) * 0.2
	l.phase += 0.10 * (1 + 6*l.energy)
}

// rampIdx devuelve el paso del gradiente de cada una de las cols columnas
// según la onda (fase + energía). Compartido por el arte y por la barra de
// título de una fila.
func (l *logoModel) rampIdx(cols int) []int {
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
	return idx
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
	idx := l.rampIdx(cols)
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

// logoTitle es el texto de la barra de título de una fila. NO usa el arte
// ASCII (ni el logo.txt del usuario): seis filas de figlet no se colapsan a
// una, y el gradiente animado se aplica igual sobre las letras del nombre.
const logoTitle = "MALODY MALLOW"

// titleLine dibuja el banner colapsado a UNA fila: el nombre centrado en w,
// con el mismo gradiente y la misma onda que el arte grande. Es la forma de
// tener banner sin pagar las seis filas que el rediseño le quitó a la cola.
func (l *logoModel) titleLine(w int) string {
	runes := []rune(logoTitle)
	if w < len(runes) {
		return ""
	}
	idx := l.rampIdx(len(runes))
	var b strings.Builder
	b.WriteString(strings.Repeat(" ", (w-len(runes))/2))
	for c := 0; c < len(runes); {
		j := c
		for j < len(runes) && idx[j] == idx[c] {
			j++
		}
		b.WriteString(l.ramp[idx[c]].Render(string(runes[c:j])))
		c = j
	}
	return b.String()
}

// logoVisible: hay banner en pantalla (la barra de título o el splash) y su
// onda debe seguir animándose. Sin banner el reloj no llega ni a armarse.
func (m *Model) logoVisible() bool {
	if m.width < minWidth || m.langOpen || m.showHelp || m.consoleOpen ||
		m.songsOpen || m.plOpen || m.npOpen {
		return false
	}
	if m.splashOn() {
		return true
	}
	return m.cfg.Theme.Banner == config.BannerTitlebar &&
		m.layoutOf().bannerH > 0
}

// titleBar es la fila del banner en modo titlebar.
func (m *Model) titleBar(w int) string {
	return padTo(m.logo.titleLine(w), w)
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

// --- Splash de arranque ---

// splashDur es lo que dura la pantalla de bienvenida. Corta a propósito: es
// el hueco que de todas formas se pasa esperando la biblioteca y el primer
// estado del demonio, no una pausa que se le impone a nadie. Cualquier tecla
// la salta.
const splashDur = 900 * time.Millisecond

type splashDoneMsg struct{}

func splashCmd() tea.Cmd {
	return tea.Tick(splashDur, func(time.Time) tea.Msg { return splashDoneMsg{} })
}

// splashOn: hay splash en pantalla ahora mismo. Exige que el arte QUEPA — un
// banner recortado a la mitad como bienvenida es peor que ninguno.
func (m *Model) splashOn() bool {
	return m.splash && m.cfg.Theme.Banner == config.BannerSplash &&
		m.height >= m.logo.artH()+2 && m.width >= m.logo.artW()+2
}

func (m *Model) splashView() string {
	art := strings.Join(m.logo.view(m.logo.width), "\n")
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, art)
}
