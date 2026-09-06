package tui

import (
	"image"
	"image/color"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	"maly/internal/config"
	"maly/internal/i18n"
	"maly/internal/ipc"
	"maly/internal/media"
)

func TestActiveLyric(t *testing.T) {
	lines := []media.LyricLine{
		{At: 5, Text: "a"}, {At: 10, Text: "b"}, {At: 20, Text: "c"},
	}
	for pos, want := range map[float64]int{
		0: -1, 4.9: -1, 5: 0, 9.9: 0, 10: 1, 15: 1, 20: 2, 999: 2,
	} {
		if got := activeLyric(lines, pos); got != want {
			t.Errorf("activeLyric(%v) = %d, quería %d", pos, got, want)
		}
	}
}

// TestLyricsPulseInRange: sin(x) garantiza el rango, pero lo encoda de
// todos modos — cubre además tiempos "raros" (cero, antes del epoch, muy
// lejos en el futuro) para los que no debería haber división por cero ni
// desborde.
func TestLyricsPulseInRange(t *testing.T) {
	times := []time.Time{
		{},
		time.Unix(0, 0),
		time.Unix(-1000, 0),
		time.Unix(1<<40, 0),
	}
	for i := 0; i < 50; i++ {
		times = append(times, time.Unix(0, 0).Add(time.Duration(i)*137*time.Millisecond))
	}
	for _, tt := range times {
		if v := lyricsPulse(tt); v < 0 || v > 1 {
			t.Errorf("lyricsPulse(%v) = %v, fuera de [0,1]", tt, v)
		}
	}
}

// TestPulseColorLevelZeroIsBaseColor: en el valle del pulso el color no debe
// cambiar — la línea activa nunca queda MÁS opaca que el accent de base.
func TestPulseColorLevelZeroIsBaseColor(t *testing.T) {
	const accent = "#89b4fa"
	if got := pulseColor(accent, 0); string(got) != accent {
		t.Errorf("pulseColor(nivel=0) = %s, quería %s sin cambios", got, accent)
	}
}

// TestPulseColorLevelOneLightensTowardWhite: en el pico, ningún canal debe
// oscurecerse ni desbordar 255, y al menos uno debe aclararse de verdad (un
// canal ya casi saturado, como el azul de #89b4fa en 250, puede truncar a 0
// de cambio sin que eso sea un bug — por eso no se exige en TODOS).
func TestPulseColorLevelOneLightensTowardWhite(t *testing.T) {
	const accent = "#89b4fa"
	base := parseHex(accent)
	got := parseHex(string(pulseColor(accent, 1)))
	lightened := false
	for i := range got {
		if got[i] < base[i] {
			t.Errorf("canal %d se oscureció: base=%d pulso=%d", i, base[i], got[i])
		}
		if got[i] > base[i] {
			lightened = true
		}
		if got[i] > 255 {
			t.Errorf("canal %d se desborda: %d", i, got[i])
		}
	}
	if !lightened {
		t.Errorf("ningún canal se aclaró: base=%v pulso=%v", base, got)
	}
}

// TestActiveLyricStyleFrozenWhenNotPlaying cubre la Fase 4: en pausa (o sin
// pista) el pulso no debe evaluarse — el estilo activo queda fijo en
// accent+bold, idéntico al que ya usaban las líneas activas antes de esta
// animación.
func TestActiveLyricStyleFrozenWhenNotPlaying(t *testing.T) {
	th := config.Theme{Accent: "#89b4fa", Dim: "#6c7086"}
	cases := []*Model{
		{st: newStyles(th), status: &ipc.Status{Track: &ipc.TrackInfo{Path: "/x.mp3"}, Paused: true}},
		{st: newStyles(th)}, // sin status → sin pista
	}
	for i, m := range cases {
		want := m.st.accent.Bold(true).Render("línea")
		if got := m.activeLyricStyle().Render("línea"); got != want {
			t.Errorf("caso %d: estilo activo en reposo = %q, quería %q", i, got, want)
		}
	}
}

// TestNpLyricsLinesActiveFrozenWhenPaused verifica el cableado real: con la
// reproducción en pausa, la línea que cae en la posición activa sale con el
// mismo estilo congelado (no depende del reloj de pared, por eso es
// determinista sin mockear tiempo).
func TestNpLyricsLinesActiveFrozenWhenPaused(t *testing.T) {
	th := config.Theme{Accent: "#89b4fa", Dim: "#6c7086"}
	m := &Model{st: newStyles(th)}
	m.npTrack = "/x.mp3"
	m.npSynced = true
	m.npLyrics = []media.LyricLine{{At: 0, Text: "a"}, {At: 5, Text: "b"}, {At: 10, Text: "c"}}
	m.status = &ipc.Status{Track: &ipc.TrackInfo{Path: "/x.mp3"}, Position: 5, Paused: true}

	out := m.npLyricsLines(20, 3)
	want := center(m.st.accent.Bold(true).Render(clip("b", 20-4)), 20)
	found := false
	for _, l := range out {
		if l == want {
			found = true
		}
	}
	if !found {
		t.Errorf("línea activa 'b' no salió con el estilo congelado esperado; out=%v", out)
	}
}

// TestStyleForLyricNoAnchorFallsBackToDim: sin línea activa (letras sin
// sincronía, o el pos aún no llega a la primera) no hay jerarquía posible —
// toda línea cae al dim plano de antes de este refinamiento. Comparación por
// Render() (no por RGB): `go test` no es una terminal real, así que lipgloss
// puede recortar el color en ambos lados por igual — la igualdad de cadenas
// sigue siendo válida sea cual sea el perfil de color detectado.
func TestStyleForLyricNoAnchorFallsBackToDim(t *testing.T) {
	m := &Model{st: newStyles(config.Theme{Accent: "#ffffff", Dim: "#404040", Border: "#202020"})}
	want := m.st.dim.Render("x")
	for _, idx := range []int{-3, -1, 0, 1, 5} {
		if got := m.styleForLyric(idx, -1).Render("x"); got != want {
			t.Errorf("styleForLyric(%d, -1) = %q, quería el dim plano %q", idx, got, want)
		}
	}
}

// TestStyleForLyricDispatch verifica el cableado de styleForLyric hacia los
// helpers de blend correctos por distancia, comparando Render() contra el
// mismo helper llamado directo — robusto al recorte de color de `go test`
// por la misma razón que TestStyleForLyricNoAnchorFallsBackToDim.
func TestStyleForLyricDispatch(t *testing.T) {
	th := config.Theme{Accent: "#89b4fa", Dim: "#6c7086", Border: "#45475a"}
	m := &Model{st: newStyles(th)}
	const active = 10
	cases := []struct {
		dist int
		want lipgloss.Style
	}{
		{1, m.lyricBlend(0.40)},
		{-1, m.lyricBlend(0.18)},
		{2, m.lyricBlend(0.08)},
		{-2, m.lyricBlend(0.08)},
		{3, m.lyricFar()},
		{-5, m.lyricFar()},
	}
	for _, c := range cases {
		want := c.want.Render("x")
		if got := m.styleForLyric(active+c.dist, active).Render("x"); got != want {
			t.Errorf("dist=%d: styleForLyric = %q, quería %q", c.dist, got, want)
		}
	}
}

// TestBlendHexGradation encoda la aritmética de la jerarquía pedida —
// current como ancla, next con más presencia que previous a la MISMA
// distancia, atenuación progresiva conforme la distancia crece— operando
// directo sobre blendHex (strings hex puros, sin pasar por lipgloss.Render):
// así el resultado no depende de si `go test` corre con color truecolor o
// lo recorta por no ser una terminal real.
func TestBlendHexGradation(t *testing.T) {
	const dim, accent, border = "#404040", "#ffffff", "#202020" // 64, 255, 32
	channel := func(hex string) int {
		rgb := parseHex(hex)
		return rgb[0]
	}
	next := channel(blendHex(dim, accent, 0.40))
	previous := channel(blendHex(dim, accent, 0.18))
	near2 := channel(blendHex(dim, accent, 0.08))
	far2 := channel(blendHex(dim, accent, 0.08)) // dist -2, mismo peso que +2
	veryFar := channel(blendHex(dim, border, 0.5))

	if next <= previous {
		t.Errorf("next (%d) debería pesar más que previous (%d) a distancia 1", next, previous)
	}
	if near2 != far2 {
		t.Errorf("dist +2 (%d) y dist -2 (%d) deberían coincidir (simétrico)", near2, far2)
	}
	if !(next > previous && previous > near2 && near2 > veryFar) {
		t.Errorf("gradación rota: next=%d previous=%d dist2=%d far=%d", next, previous, near2, veryFar)
	}
	// far nunca puede caer por debajo del propio dim de origen partido a la
	// mitad hacia border: sigue por encima de 0, "atenuado" y no "invisible".
	if veryFar <= 0 {
		t.Errorf("la línea lejana no debería quedar en negro puro: canal=%d", veryFar)
	}
}

// TestStyleForLyricCurrentIsMax: current (dist 0) debe ser el máximo
// absoluto de la jerarquía — accent liso (congelado en pausa) mientras que
// cualquier otra distancia es una mezcla dim→accent que nunca llega al
// accent puro (t<1 siempre para los helpers de blend usados).
func TestStyleForLyricCurrentIsMax(t *testing.T) {
	th := config.Theme{Accent: "#ffffff", Dim: "#404040", Border: "#202020"}
	m := &Model{st: newStyles(th), status: &ipc.Status{Track: &ipc.TrackInfo{Path: "/x.mp3"}, Paused: true}}
	want := m.st.accent.Bold(true).Render("x")
	if got := m.styleForLyric(10, 10).Render("x"); got != want {
		t.Errorf("current = %q, quería el accent liso %q", got, want)
	}
}

// TestStyleForLyricCurrentDelegatesToActiveLyricStyle: dist == 0 debe ser
// exactamente lo que ya devolvía activeLyricStyle (con o sin pulso), no una
// copia paralela — evita que las dos rutas diverjan con el tiempo.
func TestStyleForLyricCurrentDelegatesToActiveLyricStyle(t *testing.T) {
	th := config.Theme{Accent: "#89b4fa", Dim: "#6c7086", Border: "#45475a"}
	m := &Model{st: newStyles(th), status: &ipc.Status{Track: &ipc.TrackInfo{Path: "/x.mp3"}, Paused: true}}
	want := m.activeLyricStyle().Render("x")
	if got := m.styleForLyric(3, 3).Render("x"); got != want {
		t.Errorf("styleForLyric en dist=0 = %q, quería delegar en activeLyricStyle = %q", got, want)
	}
}

// TestHalfBlocksRender: una imagen 2×4 conocida debe salir como 2 filas de
// "▀" con los pares fg (píxel superior) / bg (píxel inferior) correctos.
func TestHalfBlocksRender(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 4))
	colors := [][3]uint8{
		{255, 0, 0}, {0, 255, 0}, // fila 0 de celdas: arriba
		{0, 0, 255}, {255, 255, 0}, // fila 0 de celdas: abajo
		{1, 2, 3}, {1, 2, 3}, // fila 1: arriba (iguales → un solo SGR)
		{1, 2, 3}, {1, 2, 3}, // fila 1: abajo
	}
	for i, c := range colors {
		img.Set(i%2, i/2, color.RGBA{c[0], c[1], c[2], 255})
	}
	lines := halfBlocks{}.render(img, 2, 2)
	if len(lines) != 2 {
		t.Fatalf("filas = %d, quería 2", len(lines))
	}
	if !strings.Contains(lines[0], "\x1b[38;2;255;0;0m\x1b[48;2;0;0;255m▀") ||
		!strings.Contains(lines[0], "\x1b[38;2;0;255;0m\x1b[48;2;255;255;0m▀") {
		t.Errorf("fila 0 sin los pares fg/bg esperados: %q", lines[0])
	}
	// Celdas contiguas con el mismo par comparten secuencia: un solo SGR.
	if strings.Count(lines[1], "\x1b[38;2;") != 1 || !strings.Contains(lines[1], "▀▀") {
		t.Errorf("fila 1 debía agrupar las celdas iguales: %q", lines[1])
	}
	for _, l := range lines {
		if !strings.HasSuffix(l, "\x1b[0m") {
			t.Errorf("cada fila debe cerrar con reset: %q", l)
		}
	}
}

// TestNpLyricsClamp: el scroll manual se clampa al rango real y sin letras
// sale el aviso centrado.
func TestNpLyricsClamp(t *testing.T) {
	m := &Model{st: newStyles(config.Theme{}), status: &ipc.Status{Track: &ipc.TrackInfo{Path: "/x.mp3"}}}
	m.npTrack = "/x.mp3"
	for i := 0; i < 6; i++ {
		m.npLyrics = append(m.npLyrics, media.LyricLine{At: -1, Text: "l"})
	}
	m.npScroll = 99
	if got := m.npLyricsLines(20, 4); len(got) != 4 {
		t.Fatalf("líneas = %d, quería 4", len(got))
	}
	if m.npScroll != 2 { // 6 líneas - 4 visibles
		t.Errorf("npScroll = %d, quería el clamp a 2", m.npScroll)
	}
	m.npScroll = -5
	m.npLyricsLines(20, 4)
	if m.npScroll != 0 {
		t.Errorf("npScroll = %d, quería el clamp a 0", m.npScroll)
	}

	m.npLyrics = nil
	out := m.npLyricsLines(20, 3)
	if len(out) != 3 || out[1] == "" {
		t.Errorf("sin letras debía centrar el aviso: %q", out)
	}
}

// TestNpLyricsLinesBlanksBeyondMaxDistance: en una ventana alta (más de
// 2·maxLyricDistance+1 filas), las líneas dentro del corte conservan su
// texto atenuado de siempre y las de más allá salen en blanco — no cada vez
// más tenues, sino ausentes del todo.
func TestNpLyricsLinesBlanksBeyondMaxDistance(t *testing.T) {
	th := config.Theme{Accent: "#89b4fa", Dim: "#6c7086", Border: "#45475a"}
	var lyrics []media.LyricLine
	for i := 0; i < 30; i++ {
		lyrics = append(lyrics, media.LyricLine{At: float64(i * 3), Text: "línea"})
	}
	const active = 15
	m := &Model{
		st:       newStyles(th),
		npTrack:  "/x.mp3",
		npLyrics: lyrics,
		npSynced: true,
		status:   &ipc.Status{Track: &ipc.TrackInfo{Path: "/x.mp3"}, Position: lyrics[active].At, Paused: true},
	}
	const h = 15 // generosa: sin corte se verían ±7 de contexto
	start := active - h/2
	out := m.npLyricsLines(30, h)
	for row, l := range out {
		i := start + row
		if i < 0 || i >= len(lyrics) {
			continue
		}
		dist := lyricDistance(i, active)
		if dist > maxLyricDistance {
			if l != "" {
				t.Errorf("fila %d (dist=%d, > corte %d) debería estar en blanco, salió %q", row, dist, maxLyricDistance, l)
			}
		} else if l == "" {
			t.Errorf("fila %d (dist=%d, dentro del corte) no debería estar en blanco", row, dist)
		}
	}
}

// TestNpLyricsLinesNoBlankWithinNaturalWindow: con una ventana chica (el
// caso dominante, h≈7 — ver lyrH en npView) la distancia natural nunca
// supera el corte, así que el comportamiento queda idéntico al de antes de
// este cambio: ninguna fila desaparece por el corte.
func TestNpLyricsLinesNoBlankWithinNaturalWindow(t *testing.T) {
	th := config.Theme{Accent: "#89b4fa", Dim: "#6c7086", Border: "#45475a"}
	var lyrics []media.LyricLine
	for i := 0; i < 30; i++ {
		lyrics = append(lyrics, media.LyricLine{At: float64(i * 3), Text: "línea"})
	}
	const active = 15
	m := &Model{
		st:       newStyles(th),
		npTrack:  "/x.mp3",
		npLyrics: lyrics,
		npSynced: true,
		status:   &ipc.Status{Track: &ipc.TrackInfo{Path: "/x.mp3"}, Position: lyrics[active].At, Paused: true},
	}
	const h = 7
	out := m.npLyricsLines(30, h)
	for row, l := range out {
		if l == "" {
			t.Errorf("h=%d (ventana natural) no debería producir filas en blanco por el corte; out[%d]=%q out=%v", h, row, l, out)
		}
	}
}

// TestNpMetaTimeLineNoOverflow: la línea de ícono+tiempo era la única de
// npMeta sin clip — con una duración larga (horas) y un w angosto (el piso
// de la función, 8), desbordaba (auditoría de UX post-1.12.0).
func TestNpMetaTimeLineNoOverflow(t *testing.T) {
	m := &Model{
		st: newStyles(config.Theme{}),
		status: &ipc.Status{
			Track:    &ipc.TrackInfo{Title: "T"},
			Position: 3723, // 1:02:03
			Duration: 7904, // 2:11:44
		},
	}
	for _, l := range m.npMeta(8) {
		if lw := lipgloss.Width(l); lw > 8 {
			t.Errorf("línea de npMeta mide %d celdas, más que w (8): %q", lw, l)
		}
	}
}

// TestNpViewAvisaCaratulaOcultaPorAncho cubre el hallazgo T10 de la
// auditoría P2: antes, una carátula real que no entraba por ancho quedaba
// indistinguible de una pista sin carátula embebida — ningún mensaje
// explicaba la diferencia.
func TestNpViewAvisaCaratulaOcultaPorAncho(t *testing.T) {
	m := &Model{
		st:      newStyles(config.Theme{}),
		width:   50,
		height:  30,
		status:  &ipc.Status{Track: &ipc.TrackInfo{Path: "/x.mp3", Title: "T"}},
		npTrack: "/x.mp3",
		npImg:   image.NewRGBA(image.Rect(0, 0, 4, 4)),
	}
	out := m.npView()
	if !strings.Contains(out, "widen the terminal") {
		t.Errorf("con carátula real oculta por ancho, npView debía avisarlo: %q", out)
	}

	// Sin carátula embebida (el caso normal), no hay aviso.
	m.npImg = nil
	out = m.npView()
	if strings.Contains(out, "widen the terminal") {
		t.Errorf("sin carátula embebida no debía avisar nada de ancho: %q", out)
	}
}

// --- A-24: el flash llega a las cuatro capas -------------------------------
//
// El flash del pie es el ÚNICO canal de error de la TUI y los modales de
// pantalla completa lo tapan. plView y songsView ya lo dibujaban bajo su caja
// (con el mismo bloque copiado dos veces); faltaban npView y getView, así que
// una tecla de reproducción que fallaba dentro de ctrl+t no decía nada — y el
// mismo error SÍ se ve en la vista principal.

// npFlashModel: la capa con lo mínimo para renderizar.
func npFlashModel() *Model {
	return &Model{
		st:     newStyles(config.Theme{}),
		width:  80,
		height: 30,
		status: &ipc.Status{Track: &ipc.TrackInfo{Path: "/x.mp3", Title: "T"}},
	}
}

// TestNpViewMuestraElFlash: con un flash vigente, la capa lo dice en vez del
// hint. Es el caso que motiva A-24: ctrl+t comparte las teclas de
// reproducción con la vista principal vía playbackKey.
func TestNpViewMuestraElFlash(t *testing.T) {
	m := npFlashModel()
	sinFlash := m.npView()
	if strings.Contains(sinFlash, "no hay siguiente") {
		t.Fatal("el fixture ya traía el texto del flash")
	}
	if !strings.Contains(sinFlash, i18n.T("np.hint")) {
		t.Fatalf("sin flash debía verse el hint normal:\n%s", sinFlash)
	}

	m.setFlash("no hay siguiente pista", true)
	conFlash := m.npView()
	if !strings.Contains(conFlash, "no hay siguiente pista") {
		t.Errorf("el flash no llegó a la capa: la tecla parece no haber hecho nada\n%s", conFlash)
	}
}

// TestNpViewFlashOcupaLaFilaDelHint es el riesgo que anota el propio
// hallazgo: la capa es pantalla completa con panel propio, así que el flash
// toma prestada la fila del hint en vez de AGREGAR una — una fila de más
// empuja la franja del visualizador y panel() se come la última en silencio.
//
// La aserción es que el flash aparece en el MISMO índice de fila donde estaba
// el hint. Medir solo el alto NO sirve: panel() trunca a innerH, así que la
// versión ingenua (append de una fila) devuelve exactamente el mismo número
// de filas y pasa igual — comprobado. Y comparar la salida entera tampoco lo
// caza, porque sin audio las filas del viz son idénticas entre sí y el
// desplazamiento no se nota.
func TestNpViewFlashOcupaLaFilaDelHint(t *testing.T) {
	for _, vizOn := range []bool{false, true} {
		m := npFlashModel()
		m.vizOn = vizOn

		sin := strings.Split(m.npView(), "\n")
		filaHint := -1
		for i, l := range sin {
			if strings.Contains(l, i18n.T("np.hint")) {
				filaHint = i
			}
		}
		if filaHint < 0 {
			t.Fatalf("vizOn=%v: no se encontró la fila del hint", vizOn)
		}

		m.setFlash("un error cualquiera", true)
		con := strings.Split(m.npView(), "\n")
		if len(con) != len(sin) {
			t.Errorf("vizOn=%v: el flash cambió el alto de %d a %d filas", vizOn, len(sin), len(con))
		}
		if !strings.Contains(con[filaHint], "un error cualquiera") {
			t.Errorf("vizOn=%v: el flash debía ocupar la fila %d (la del hint), no otra:\n%s",
				vizOn, filaHint, con[filaHint])
		}
		if strings.Contains(con[filaHint], i18n.T("np.hint")) {
			t.Errorf("vizOn=%v: el hint debía quedar reemplazado, no acompañado", vizOn)
		}
	}
}

// TestNpViewFlashVencidoVuelveAlHint: el flash caduca a los 4 s y la fila
// tiene que volver a su contenido normal, no quedarse con el error pegado.
func TestNpViewFlashVencidoVuelveAlHint(t *testing.T) {
	m := npFlashModel()
	m.flash, m.flashErr = "error viejo", true
	m.flashUntil = time.Now().Add(-time.Second) // ya vencido
	out := m.npView()
	if strings.Contains(out, "error viejo") {
		t.Errorf("un flash vencido no debía seguir mostrándose:\n%s", out)
	}
	if !strings.Contains(out, i18n.T("np.hint")) {
		t.Errorf("con el flash vencido debía volver el hint:\n%s", out)
	}
}

// TestWithFlashCuelgaUnaFila cubre el helper extraído, que es lo que usan los
// otros tres modales. Sin flash no toca la caja; con uno, agrega exactamente
// una fila.
func TestWithFlashCuelgaUnaFila(t *testing.T) {
	m := &Model{st: newStyles(config.Theme{})}
	caja := "linea1\nlinea2"

	if got := m.withFlash(caja); got != caja {
		t.Errorf("sin flash no debía tocar la caja, dio %q", got)
	}

	m.setFlash("algo pasó", false)
	got := m.withFlash(caja)
	if n := len(strings.Split(got, "\n")); n != 3 {
		t.Errorf("con flash debía agregar UNA fila (3 en total), dio %d: %q", n, got)
	}
	if !strings.Contains(got, "algo pasó") {
		t.Errorf("el flash no aparece: %q", got)
	}
}

// TestGetViewMuestraElFlash: el buscador dibuja los estados de la BÚSQUEDA en
// el cuerpo del panel, pero un actionMsg de otra procedencia seguía sin canal.
func TestGetViewMuestraElFlash(t *testing.T) {
	m := &Model{st: newStyles(config.Theme{}), width: 90, height: 30}
	m.openGet()
	if strings.Contains(m.getView(), "algo falló") {
		t.Fatal("el fixture ya traía el texto del flash")
	}
	m.setFlash("algo falló", true)
	if out := m.getView(); !strings.Contains(out, "algo falló") {
		t.Errorf("el flash no llegó al buscador:\n%s", out)
	}
}
