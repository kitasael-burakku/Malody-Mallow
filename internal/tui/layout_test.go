package tui

import (
	"fmt"
	"testing"

	"maly/internal/config"
)

// anchos y altos de la tabla: los tres umbrales y sus vecinos inmediatos, que
// es donde vive cualquier off-by-one de modo.
var (
	layoutWidths  = []int{minWidth, 41, 60, 80, 89, 90, 100, 119, 120, 121, 160, 240, 400}
	layoutHeights = []int{minHeight, 13, 19, 20, 23, 24, 25, 30, 40, 60}
)

func allOpts() []layoutOpts {
	var out []layoutOpts
	for _, viz := range []bool{false, true} {
		for _, b := range []string{config.BannerSplash, config.BannerTitlebar, config.BannerOff} {
			out = append(out, layoutOpts{viz: viz, banner: b})
		}
	}
	return out
}

// TestComputeLayoutInvariantes es EL test de la fase: en ninguna combinación
// de tamaño y opciones pueden salir medidas negativas, ni los anchos sumar
// algo distinto del ancho real de la terminal. Una celda de más se lleva por
// delante el borde derecho del último panel y una de menos deja una columna
// de basura del frame anterior; ninguna de las dos se ve en el tamaño en el
// que uno prueba a mano.
func TestComputeLayoutInvariantes(t *testing.T) {
	for _, opts := range allOpts() {
		for _, w := range layoutWidths {
			for _, h := range layoutHeights {
				l := computeLayout(w, h, opts)
				name := fmt.Sprintf("%dx%d viz=%t banner=%s", w, h, opts.viz, opts.banner)

				for _, v := range []struct {
					lbl string
					n   int
				}{
					{"libW", l.libW}, {"queueW", l.queueW}, {"npW", l.npW},
					{"topH", l.topH}, {"nowH", l.nowH}, {"vizH", l.vizH}, {"bannerH", l.bannerH},
				} {
					if v.n < 0 {
						t.Errorf("%s: %s = %d (negativo)", name, v.lbl, v.n)
					}
				}

				switch l.mode {
				case layoutFull:
					if sum := l.libW + l.queueW + l.npW; sum != w {
						t.Errorf("%s: columnas suman %d, quería %d", name, sum, w)
					}
				case layoutTwoCol:
					if sum := l.libW + l.queueW; sum != w {
						t.Errorf("%s: columnas suman %d, quería %d", name, sum, w)
					}
					if l.npW != 0 {
						t.Errorf("%s: npW = %d fuera del modo de tres columnas", name, l.npW)
					}
				default:
					// Una sola columna: se dibuja SOLO el panel enfocado, así
					// que cada uno vale el ancho completo.
					if l.libW != w || l.queueW != w || l.npW != 0 {
						t.Errorf("%s: single = (%d,%d,%d), quería (%d,%d,0)",
							name, l.libW, l.queueW, l.npW, w, w)
					}
				}

				// Reparto vertical exacto, incluido el pie.
				if sum := l.bannerH + l.topH + l.vizH + l.nowH + 1; sum != h {
					t.Errorf("%s: filas suman %d, quería %d", name, sum, h)
				}
			}
		}
	}
}

// TestComputeLayoutModos fija los tres breakpoints de ancho.
func TestComputeLayoutModos(t *testing.T) {
	cases := []struct {
		w    int
		want layoutMode
	}{
		{minWidth, layoutSingle}, {60, layoutSingle}, {89, layoutSingle},
		{90, layoutTwoCol}, {100, layoutTwoCol}, {119, layoutTwoCol},
		{120, layoutFull}, {240, layoutFull},
	}
	for _, c := range cases {
		if got := computeLayout(c.w, 40, layoutOpts{}).mode; got != c.want {
			t.Errorf("ancho %d: modo %d, quería %d", c.w, got, c.want)
		}
	}
}

// TestComputeLayoutAnchosDeColumna: la biblioteca va acotada (no es un
// porcentaje: a mitad de pantalla dos tercios eran espacio muerto), la
// columna Now Playing también, y la COLA se queda con todo lo que sobra —es
// la que tiene los títulos largos, así que es la que debe crecer.
func TestComputeLayoutAnchosDeColumna(t *testing.T) {
	prev := 0
	for _, w := range []int{120, 160, 200, 300, 400} {
		l := computeLayout(w, 40, layoutOpts{})
		if l.libW < libMinW || l.libW > libMaxW {
			t.Errorf("ancho %d: libW = %d fuera de [%d,%d]", w, l.libW, libMinW, libMaxW)
		}
		if l.npW < npMinW || l.npW > npMaxW {
			t.Errorf("ancho %d: npW = %d fuera de [%d,%d]", w, l.npW, npMinW, npMaxW)
		}
		if l.queueW <= prev {
			t.Errorf("ancho %d: la cola no creció (%d ≤ %d)", w, l.queueW, prev)
		}
		prev = l.queueW
	}
	// Y en dos columnas la cola se lleva igualmente el resto.
	l := computeLayout(100, 40, layoutOpts{})
	if l.queueW != 100-l.libW {
		t.Errorf("dos columnas: queueW = %d, quería %d", l.queueW, 100-l.libW)
	}
}

// TestComputeLayoutBarraSiempre: la barra de reproducción está en los TRES
// modos, también con la columna "Ahora suena" en pantalla — la columna aporta
// carátula y ficha, el progreso se lee en la barra larga del pie. Solo
// desaparece como último recurso, cuando ya se cedieron franja y banner y
// aun así no entra la fila de paneles.
func TestComputeLayoutBarraSiempre(t *testing.T) {
	for _, w := range []int{minWidth, 100, 160, 240} {
		for _, h := range []int{20, 24, 30, 42} {
			if got := computeLayout(w, h, layoutOpts{viz: true}).nowH; got != nowPanelH {
				t.Errorf("%dx%d: nowH = %d, quería %d", w, h, got, nowPanelH)
			}
		}
	}
}

// TestComputeLayoutSacrificaEnOrden: al faltar altura cae PRIMERO la franja
// del visualizador (decoración), después el banner, y la fila de paneles
// nunca baja de su mínimo mientras algo se pueda ceder.
func TestComputeLayoutSacrificaEnOrden(t *testing.T) {
	opts := layoutOpts{viz: true, banner: config.BannerTitlebar}

	alto := computeLayout(160, 40, opts)
	if alto.vizH == 0 || alto.bannerH == 0 {
		t.Fatalf("con 40 filas debían caber franja y banner: %+v", alto)
	}

	// Justo por debajo del umbral del visualizador.
	sinViz := computeLayout(160, vizMinRows-1, opts)
	if sinViz.vizH != 0 {
		t.Errorf("con %d filas la franja debía desaparecer", vizMinRows-1)
	}
	if sinViz.bannerH == 0 {
		t.Errorf("con %d filas el banner todavía debía caber", vizMinRows-1)
	}

	// Y por debajo del umbral del banner, tampoco banner.
	sinNada := computeLayout(160, bannerMinRows-1, opts)
	if sinNada.vizH != 0 || sinNada.bannerH != 0 {
		t.Errorf("con %d filas no debía quedar ni franja ni banner: %+v", bannerMinRows-1, sinNada)
	}

	// En el peor caso posible la fila de paneles sigue teniendo altura útil.
	if peor := computeLayout(minWidth, minHeight, opts); peor.topH < minTopH {
		t.Errorf("en %dx%d topH = %d, por debajo del mínimo %d",
			minWidth, minHeight, peor.topH, minTopH)
	}
}

// TestComputeLayoutDegenerado: tamaños imposibles no producen basura (View ya
// los ataja con el mensaje de "terminal muy pequeña", pero computeLayout no
// puede depender de que alguien la llame bien).
func TestComputeLayoutDegenerado(t *testing.T) {
	for _, c := range [][2]int{{0, 0}, {-5, 20}, {80, 0}, {-1, -1}} {
		if l := computeLayout(c[0], c[1], layoutOpts{viz: true}); l != (layout{}) {
			t.Errorf("computeLayout(%d,%d) = %+v, quería el cero", c[0], c[1], l)
		}
	}
}
