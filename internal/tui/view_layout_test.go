package tui

import (
	"image"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"maly/internal/config"
	"maly/internal/i18n"
	"maly/internal/ipc"
	"maly/internal/media"
)

// newLayoutTestModel arma un Model listo para renderizar la vista principal:
// tema completo, árbol vacío, una pista sonando y sin splash pendiente.
func newLayoutTestModel(w, h int) *Model {
	th := config.Theme{
		Accent: "#7ab8b8", Text: "#d4dadb", Dim: "#6b7a7e",
		Border: "#3a4448", Playing: "#b85c50", Error: "#c96f60",
		Banner: config.BannerOff,
	}
	th.ResolveDerived()
	cfg := config.Default()
	cfg.Theme = th
	m := &Model{
		cfg:    cfg,
		st:     newStyles(th),
		keys:   config.DefaultKeys(),
		tree:   buildTree(nil, nil),
		width:  w,
		height: h,
		status: &ipc.Status{
			Track:      &ipc.TrackInfo{Title: "Colchón Vacío", Artist: "kaisoyeon", Duration: 292},
			Position:   100,
			Duration:   292,
			Volume:     100,
			QueueIndex: 0,
		},
		queue:     []ipc.TrackInfo{{Title: "Colchón Vacío", Artist: "kaisoyeon", Duration: 292}},
		libLoaded: true,
		logo:      newLogo(th.Logo, nil),
		cover:     halfBlocks{},
	}
	return m
}

// TestViewAnchoDeCadaFila es el criterio duro del relayout: TODAS las filas
// que emite View() miden exactamente el ancho de la terminal, en los tres
// modos y con o sin franja del visualizador. Una fila más corta deja basura
// del frame anterior a la derecha; una más larga la parte el terminal y
// desplaza todo el resto de la pantalla una fila hacia abajo.
func TestViewAnchoDeCadaFila(t *testing.T) {
	for _, viz := range []bool{false, true} {
		for _, banner := range []string{config.BannerOff, config.BannerTitlebar} {
			for _, size := range [][2]int{
				{minWidth, minHeight}, {60, 20}, {78, 26}, {89, 24},
				{90, 24}, {100, 30}, {119, 23}, {120, 24}, {160, 42}, {240, 60},
			} {
				w, h := size[0], size[1]
				m := newLayoutTestModel(w, h)
				m.vizOn = viz
				m.cfg.Theme.Banner = banner
				out := m.View()
				lines := strings.Split(out, "\n")
				if len(lines) != h {
					t.Errorf("%dx%d viz=%t banner=%s: %d filas, quería %d",
						w, h, viz, banner, len(lines), h)
				}
				for i, l := range lines {
					if n := lipgloss.Width(l); n != w {
						t.Errorf("%dx%d viz=%t banner=%s: fila %d mide %d, quería %d",
							w, h, viz, banner, i, n, w)
					}
				}
			}
		}
	}
}

// TestViewModosDibujanLoQueToca: cada modo pinta exactamente sus paneles. En
// una sola columna se dibuja SOLO el panel enfocado (si no, no habría entrado
// ninguno de los dos).
func TestViewModosDibujanLoQueToca(t *testing.T) {
	lib, queue, now := i18n.T("tui.lib_title"), i18n.T("tui.queue_title"), i18n.T("tui.now_title")
	head := func(s string) string { return strings.SplitN(s, " (", 2)[0] }
	libT, queueT := head(lib), head(queue)

	full := newLayoutTestModel(160, 42).View()
	for _, want := range []string{libT, queueT, now} {
		if !strings.Contains(full, want) {
			t.Errorf("tres columnas: falta el panel %q", want)
		}
	}

	two := newLayoutTestModel(100, 30).View()
	for _, want := range []string{libT, queueT, now} {
		if !strings.Contains(two, want) {
			t.Errorf("dos columnas: falta %q (Now Playing va como barra al pie)", want)
		}
	}

	single := newLayoutTestModel(78, 26)
	out := single.View()
	if !strings.Contains(out, libT) {
		t.Error("una columna: falta el panel enfocado (biblioteca)")
	}
	if strings.Contains(out, queueT) {
		t.Error("una columna: la cola no debía dibujarse sin foco")
	}
	single.focus = panelQueue
	out = single.View()
	if !strings.Contains(out, queueT) {
		t.Error("una columna: tras tab, la cola debía tomar la pantalla")
	}
	if strings.Contains(out, libT) {
		t.Error("una columna: la biblioteca no debía dibujarse sin foco")
	}
}

// TestBarraDeProgresoUnaSola: el progreso se dibuja UNA vez y siempre en la
// barra del pie. La columna de tres columnas muestra carátula y ficha, y no
// puede repetir ni la barra ni los tiempos (la duplicación de la barra fue un
// defecto real hasta la 1.6.1, con dos copias que divergieron).
func TestBarraDeProgresoUnaSola(t *testing.T) {
	for _, size := range [][2]int{{160, 42}, {100, 30}, {78, 26}} {
		m := newLayoutTestModel(size[0], size[1])
		out := m.View()
		if !strings.Contains(out, "vol 100%") {
			t.Errorf("%dx%d: falta la barra de reproducción al pie", size[0], size[1])
		}
		if n := strings.Count(out, "▓▒"); n != 1 {
			t.Errorf("%dx%d: %d barras de progreso en pantalla, quería 1", size[0], size[1], n)
		}
		if n := strings.Count(out, "01:40"); n != 1 {
			t.Errorf("%dx%d: la posición aparece %d veces, quería 1", size[0], size[1], n)
		}
	}
}

// TestSplashSoloConSuModo: el splash depende del modo de banner y de que el
// arte QUEPA; y cualquier tecla lo salta sin tragarse la pulsación.
func TestSplashSoloConSuModo(t *testing.T) {
	m := newLayoutTestModel(160, 42)
	m.splash = true

	m.cfg.Theme.Banner = config.BannerOff
	if m.splashOn() {
		t.Error("banner=off no debe mostrar splash")
	}
	m.cfg.Theme.Banner = config.BannerTitlebar
	if m.splashOn() {
		t.Error("banner=titlebar no debe mostrar splash")
	}
	m.cfg.Theme.Banner = config.BannerSplash
	if !m.splashOn() {
		t.Error("banner=splash con espacio de sobra debía mostrar splash")
	}
	if lipgloss.Width(strings.Split(m.splashView(), "\n")[0]) != m.width {
		t.Error("el splash no llena el ancho de la terminal")
	}

	// Terminal demasiado chica para el arte: mejor nada que un banner partido.
	chico := newLayoutTestModel(minWidth, minHeight)
	chico.splash = true
	chico.cfg.Theme.Banner = config.BannerSplash
	if chico.splashOn() {
		t.Error("con el arte sin espacio no debe haber splash")
	}

	// Una tecla lo salta.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if m.splash {
		t.Error("una tecla debía saltar el splash")
	}
}

// TestNpColumnVisible decide si hace falta cargar carátula y letras sin que
// nadie abra ctrl+t: solo en tres columnas y sin ningún modal encima.
func TestNpColumnVisible(t *testing.T) {
	m := newLayoutTestModel(160, 42)
	if !m.npColumnVisible() {
		t.Error("tres columnas sin modales: la columna está visible")
	}
	m.showHelp = true
	if m.npColumnVisible() {
		t.Error("con la ayuda encima la columna no se está dibujando")
	}
	m.showHelp = false
	if two := newLayoutTestModel(100, 30); two.npColumnVisible() {
		t.Error("en dos columnas no hay columna Now Playing")
	}
}

// TestInvalidateArtTiraLosDosRenders: la columna y la capa ctrl+t cachean la
// MISMA carátula a tamaños distintos, y en kitty comparten un único id de
// imagen — si al cambiar de vista sobrevive un cache, sus placeholders
// apuntan a una imagen transmitida para otras dimensiones de celda.
func TestInvalidateArtTiraLosDosRenders(t *testing.T) {
	m := newLayoutTestModel(160, 42)
	m.npArtLines = []string{"capa"}
	m.colArtLines = []string{"columna"}
	m.invalidateArt()
	if m.npArtLines != nil || m.colArtLines != nil {
		t.Errorf("quedaron renders vivos: np=%v col=%v", m.npArtLines, m.colArtLines)
	}
}

// TestPanelesVaciosNoDesbordan: los mensajes de "biblioteca vacía" y "cola
// vacía" miden más que un panel angosto (la biblioteca es fija en ~30 celdas
// desde el relayout, y en una sola columna la cola puede quedar en 40), y
// panel() rellena pero NO acorta. Sin clip, el panel se ensancha por dentro y
// empuja a los de al lado — lo mismo que arregló la 1.12.1 en los modales.
func TestPanelesVaciosNoDesbordan(t *testing.T) {
	for _, size := range [][2]int{{minWidth, minHeight}, {90, 24}, {100, 30}, {120, 30}} {
		m := newLayoutTestModel(size[0], size[1])
		m.queue = nil // cola vacía además de la biblioteca
		m.status.QueueIndex = -1
		for i, l := range strings.Split(m.View(), "\n") {
			if n := lipgloss.Width(l); n != size[0] {
				t.Errorf("%dx%d con paneles vacíos: fila %d mide %d, quería %d",
					size[0], size[1], i, n, size[0])
			}
		}
	}
}

// TestFiltroNoRompeElBorde: el input de filtro vive dentro de un panel que
// ahora puede ser angosto (biblioteca fija en ~30). textinput sin Width no
// hace scroll horizontal — emite el valor entero— así que un filtro largo
// rompía el borde mientras se escribe, igual que le pasaba a la consola antes
// de la 1.12.1.
func TestFiltroNoRompeElBorde(t *testing.T) {
	for _, focus := range []panelID{panelLibrary, panelQueue} {
		m := newLayoutTestModel(160, 42)
		m.focus = focus
		m.filterMode = true
		m.filterInput = textinput.New()
		m.filterInput.SetValue(strings.Repeat("kaisoyeon colchón vacío ", 5))
		for i, l := range strings.Split(m.View(), "\n") {
			if n := lipgloss.Width(l); n != 160 {
				t.Fatalf("foco %d: fila %d mide %d con un filtro largo, quería 160", focus, i, n)
			}
		}
	}
}

// TestNpColumnCaratulaAmbosRenderers: la columna pide la carátula dentro de
// lo que ambos renderers admiten. El límite duro es el de kitty —sus
// placeholders codifican fila y columna con la tabla de 64 diacríticos, y
// pasarse degrada la imagen entera a half-blocks sin avisar—, así que la
// columna nunca puede pedir más que eso.
func TestNpColumnCaratulaAmbosRenderers(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for _, rend := range []coverRenderer{halfBlocks{}, kittyGfx{}} {
		for _, size := range [][2]int{{120, 24}, {160, 42}, {240, 60}} {
			m := newLayoutTestModel(size[0], size[1])
			m.cover = rend
			m.npImg = img
			m.npTrack = m.currentTrackPath()
			m.View()
			if m.colArtH == 0 {
				t.Errorf("%T %dx%d: la columna no dibujó carátula", rend, size[0], size[1])
				continue
			}
			if len(m.colArtLines) != m.colArtH {
				t.Errorf("%T: %d filas renderizadas, se pidieron %d", rend, len(m.colArtLines), m.colArtH)
			}
			if m.colArtW > len(kittyDiacritics) || m.colArtH > len(kittyDiacritics) {
				t.Errorf("%T: carátula de %dx%d celdas, kitty admite hasta %d por eje",
					rend, m.colArtW, m.colArtH, len(kittyDiacritics))
			}
			// Y la imagen no puede deformarse: una celda vale ~2 px de alto.
			if m.colArtW != m.colArtH*2 && m.colArtW/2 != m.colArtH {
				t.Errorf("%T: carátula de %dx%d celdas, no es cuadrada", rend, m.colArtW, m.colArtH)
			}
		}
	}
}

// TestLayoutColumnaPartida: la tercera columna se reparte entre "Ahora suena"
// y las letras, sumando siempre topH. Con poca altura no hay panel de letras
// (y la carátula se queda con todo); con mucha, las letras se topean y el
// resto vuelve a la carátula en vez de acumular filas en blanco.
func TestLayoutColumnaPartida(t *testing.T) {
	for _, h := range []int{minHeight, 20, 24, 30, 42, 60, 100} {
		l := computeLayout(160, h, layoutOpts{viz: true})
		if l.npH+l.lyrH != l.topH {
			t.Errorf("h=%d: npH+lyrH = %d, quería topH = %d", h, l.npH+l.lyrH, l.topH)
		}
		if l.lyrH != 0 && l.lyrH < lyrMinH {
			t.Errorf("h=%d: panel de letras de %d filas, inútil por debajo de %d", h, l.lyrH, lyrMinH)
		}
		if l.lyrH > lyrMaxH {
			t.Errorf("h=%d: lyrH = %d supera el tope %d", h, l.lyrH, lyrMaxH)
		}
	}
	// Fuera de tres columnas la columna no existe.
	for _, w := range []int{minWidth, 100} {
		if l := computeLayout(w, 42, layoutOpts{}); l.npH != 0 || l.lyrH != 0 {
			t.Errorf("ancho %d: npH=%d lyrH=%d, quería 0 y 0", w, l.npH, l.lyrH)
		}
	}
	// Terminal baja: sin letras, y la columna entera para Now Playing.
	if l := computeLayout(160, 20, layoutOpts{viz: true}); l.lyrH != 0 || l.npH != l.topH {
		t.Errorf("terminal baja: lyrH=%d npH=%d topH=%d", l.lyrH, l.npH, l.topH)
	}
}

// TestWrapLyricLineasLargas: la línea vigente se parte por palabras hasta el
// tope y solo entonces se recorta. Es el caso que motivó la pieza: en la
// columna hay ~28 celdas útiles y más de un tercio de las líneas reales
// (mediana 25, p90 55, máximo 107) no entra de una.
func TestWrapLyricLineasLargas(t *testing.T) {
	const w = 26
	corta := wrapLyric("vacío otra vez", w)
	if len(corta) != 1 || corta[0] != "vacío otra vez" {
		t.Errorf("línea corta = %q, no debía tocarse", corta)
	}

	larga := wrapLyric("me quedé con el colchón vacío otra vez y no hay forma de llenarlo", w)
	if len(larga) < 2 {
		t.Fatalf("línea de 65 caracteres sin envolver: %q", larga)
	}
	for i, row := range larga {
		if n := lipgloss.Width(row); n > w {
			t.Errorf("fila %d mide %d, quería ≤ %d: %q", i, n, w, row)
		}
	}
	if strings.Contains(strings.Join(larga, "|"), "colch|") {
		t.Error("wrapLyric cortó una palabra a la mitad")
	}

	// Ni la más larga del corpus puede comerse el panel.
	enorme := wrapLyric(strings.Repeat("palabra ", 40), w)
	if len(enorme) > maxActiveLyricRows {
		t.Errorf("%d filas, el tope es %d", len(enorme), maxActiveLyricRows)
	}
	if !strings.Contains(enorme[len(enorme)-1], "…") {
		t.Errorf("al recortar falta la marca de continuación: %q", enorme)
	}

	// Una sola palabra imposible de partir se recorta igual.
	for _, row := range wrapLyric(strings.Repeat("x", 80), w) {
		if lipgloss.Width(row) > w {
			t.Errorf("palabra sin espacios desbordó: %q", row)
		}
	}
}

// TestPanelLetrasEnLaColumna: con letras sincronizadas, el panel de la
// columna muestra la línea vigente (envuelta si hace falta) y no desborda.
func TestPanelLetrasEnLaColumna(t *testing.T) {
	m := newLayoutTestModel(160, 42)
	m.npTrack = m.currentTrackPath()
	m.npSynced = true
	m.npLyrics = []media.LyricLine{
		{At: 0, Text: "sigo pensando en vos"},
		{At: 90, Text: "me quedé con el colchón vacío otra vez y no hay forma de llenarlo"},
		{At: 200, Text: "y no vuelve más"},
	}
	m.status.Position = 100 // la segunda línea está vigente

	lay := m.layoutOf()
	if lay.lyrH == 0 {
		t.Fatal("en 160x42 tenía que haber panel de letras")
	}
	panel := m.lyricsPanel(lay.npW, lay.lyrH)
	lines := strings.Split(panel, "\n")
	if len(lines) != lay.lyrH {
		t.Errorf("panel de %d filas, quería %d", len(lines), lay.lyrH)
	}
	for i, l := range lines {
		if n := lipgloss.Width(l); n != lay.npW {
			t.Errorf("fila %d mide %d, quería %d", i, n, lay.npW)
		}
	}
	if !strings.Contains(panel, "colchón") || !strings.Contains(panel, "llenarlo") {
		t.Errorf("la línea vigente se perdió al envolverla:\n%s", panel)
	}
	if !strings.Contains(panel, i18n.T("tui.lyrics_title")) {
		t.Error("al panel le falta su título")
	}

	// Sin letras, el panel lo dice en vez de quedar en blanco.
	m.npLyrics = nil
	if !strings.Contains(m.lyricsPanel(lay.npW, lay.lyrH), "lyrics") {
		t.Error("sin letras debía verse el aviso")
	}
}
