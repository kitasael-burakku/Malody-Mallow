package tui

import "maly/internal/config"

// El reparto de la pantalla, entero y en un solo lugar. computeLayout es PURA
// —solo depende de (ancho, alto, opciones)— para que los off-by-one de bordes
// se cacen con una tabla de tests en vez de a ojo contra un terminal real:
// repartida dentro de View(), la aritmética de tres columnas más franja del
// visualizador más banner es exactamente la clase de código que se rompe una
// celda a la vez y solo en ciertos tamaños.

type layoutMode int

const (
	// layoutSingle: una sola columna, el panel enfocado. `tab` cicla entre
	// biblioteca y cola, que es lo que ya hacía switch_panel.
	layoutSingle layoutMode = iota
	// layoutTwoCol: biblioteca + cola, con "Ahora suena" colapsado a la barra
	// horizontal de siempre al pie.
	layoutTwoCol
	// layoutFull: las tres columnas, con "Ahora suena" como columna propia.
	layoutFull
)

const (
	// Umbrales de ancho. Por debajo de minWidth (view.go) no se dibuja nada:
	// el modo de una columna cubre de ahí hasta twoColMinWidth.
	twoColMinWidth = 90
	fullMinWidth   = 120

	// La biblioteca lleva ancho FIJO y no un porcentaje: muestra nombres de
	// artista de 10-25 caracteres, así que a mitad de pantalla dos tercios del
	// panel eran espacio muerto mientras la cola —títulos de 60+— vivía
	// apretada. El tope alto no baja de 26 porque el árbol también muestra
	// pistas indentadas bajo artista y álbum, y ahí cada nivel se come 2
	// celdas.
	libMinW = 26
	libMaxW = 34
	// La columna "Ahora suena" es fija por la carátula: su ancho manda sobre
	// el alto de la imagen (una celda es ~2 píxeles de alto, ver artrender),
	// y por eso el tope alto es generoso — en una pantalla ancha, 8 celdas
	// más de columna son 4 filas más de carátula, y la cola (que es la
	// elástica) ni las nota.
	npMinW = 28
	npMaxW = 40

	// Alto de la franja del visualizador. Sin panel propio: en la vista
	// anterior las barras llenaban el tercio izquierdo de una caja de ancho
	// completo y el resto quedaba vacío con un borde alrededor.
	vizStripH = 4
	// vizMinRows: con menos alto, la franja es lo PRIMERO que se sacrifica.
	vizMinRows = 24
	// bannerMinRows: la barra de título cuesta una fila, y por debajo de esto
	// esa fila hace más falta en la cola.
	bannerMinRows = 20
	// minTopH: filas mínimas de la fila de paneles (2 bordes + 3 de lista)
	// antes de empezar a sacrificar franja y banner.
	minTopH = 5

	// Reparto de la tercera columna. npCompactH es lo que ocupa "Ahora
	// suena" con la carátula compacta (bordes + aire + 10 filas de carátula
	// + la ficha): a partir de ahí, lo que sobre va al panel de letras.
	npCompactH = 18
	// lyrMinH: con menos filas no entra ni una línea de contexto a cada lado
	// de la vigente, y el panel no aporta nada.
	lyrMinH = 5
	// lyrMaxH topea las letras a la ventana que de verdad se ve: más allá de
	// maxLyricDistance las líneas se dibujan en blanco (corte de la 1.13.1),
	// así que estirar el panel solo sumaría filas vacías — mejor devolverle
	// esa altura a la carátula.
	lyrMaxH = 2*maxLyricDistance + 3
)

// layoutOpts son las decisiones del usuario que cambian el reparto. Van como
// parámetro y no leídas del Model para que computeLayout siga siendo pura.
type layoutOpts struct {
	viz    bool   // visualizador encendido
	banner string // config.BannerSplash | BannerTitlebar | BannerOff
}

// layout es el reparto ya resuelto. Todos los anchos incluyen los bordes de
// su panel, y todos los altos las filas de borde.
type layout struct {
	mode layoutMode
	// Anchos de columna. En layoutSingle libW y queueW valen ambos el ancho
	// completo: solo se dibuja el panel enfocado, nunca los dos a la vez.
	// npW es 0 fuera de layoutFull.
	libW, queueW, npW int
	topH              int // fila de paneles
	nowH              int // barra "Ahora suena" horizontal (0 en layoutFull)
	vizH              int // franja del visualizador (0 = oculta)
	bannerH           int // barra de título (0 = sin banner)
	// Reparto vertical DENTRO de la tercera columna: el panel "Ahora suena"
	// (carátula + ficha) y debajo el de letras. npH + lyrH == topH; lyrH = 0
	// cuando no hay altura para letras, y entonces npH se queda con todo (la
	// carátula crece sola, ver npColumn). Ambos 0 fuera de layoutFull.
	npH, lyrH int
}

// computeLayout reparte una terminal de w×h. Nunca devuelve medidas negativas
// y, dentro de cada modo, los anchos suman exactamente w (en layoutSingle solo
// se dibuja una columna, así que ahí "sumar" es que cada una valga w).
func computeLayout(w, h int, opts layoutOpts) layout {
	var l layout
	if w <= 0 || h <= 0 {
		return l
	}

	switch {
	case w >= fullMinWidth:
		l.mode = layoutFull
	case w >= twoColMinWidth:
		l.mode = layoutTwoCol
	default:
		l.mode = layoutSingle
	}

	// --- Reparto vertical. El orden de los sacrificios es deliberado: la
	// franja del visualizador es decoración y cae primero; el banner después;
	// la fila de paneles es la aplicación y nunca se cede.
	if opts.banner == config.BannerTitlebar && h >= bannerMinRows {
		l.bannerH = 1
	}
	if opts.viz && h >= vizMinRows {
		l.vizH = vizStripH
	}
	// La barra de reproducción va abajo y a ancho completo en TODOS los
	// modos, también con la columna "Ahora suena" en pantalla: la columna
	// aporta la carátula y la ficha de la pista, y el progreso se lee mejor
	// en una barra larga al pie que en 26 celdas de columna (decisión del
	// dueño sobre el primer intento, que la ponía solo en la columna).
	l.nowH = nowPanelH
	const footerH = 1
	l.topH = h - l.bannerH - l.vizH - l.nowH - footerH
	if l.topH < minTopH && l.vizH > 0 {
		l.topH += l.vizH
		l.vizH = 0
	}
	if l.topH < minTopH && l.bannerH > 0 {
		l.topH += l.bannerH
		l.bannerH = 0
	}
	if l.topH < minTopH && l.nowH > 0 {
		// Último recurso: sin la barra de reproducción no queda mucha app,
		// pero es peor una fila de paneles que no entra.
		l.topH += l.nowH
		l.nowH = 0
	}
	if l.topH < 0 {
		l.topH = 0
	}

	// --- Reparto horizontal. La cola es la ELÁSTICA: es la que tiene
	// contenido largo, así que en pantallas anchas crece ella.
	switch l.mode {
	case layoutFull:
		l.libW = clampInt(w/5, libMinW, libMaxW)
		l.npW = clampInt(w/5, npMinW, npMaxW)
		l.queueW = w - l.libW - l.npW
	case layoutTwoCol:
		l.libW = clampInt(w/3, libMinW, libMaxW)
		l.queueW = w - l.libW
	default: // layoutSingle
		l.libW, l.queueW = w, w
	}

	// Reparto vertical de la tercera columna.
	if l.mode == layoutFull {
		l.npH = l.topH
		if extra := l.topH - npCompactH; extra >= lyrMinH {
			l.lyrH = min(extra, lyrMaxH)
			l.npH = l.topH - l.lyrH
		}
	}
	return l
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// layoutOf es el reparto para el estado actual del Model.
func (m *Model) layoutOf() layout {
	return computeLayout(m.width, m.height, layoutOpts{
		viz:    m.vizOn,
		banner: m.cfg.Theme.Banner,
	})
}
