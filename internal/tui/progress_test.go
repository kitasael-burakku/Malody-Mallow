package tui

import (
	"math"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"maly/internal/config"
)

// testTheme es un tema completo (derivados incluidos) para las funciones de
// barra, que leen progress_low/high/shadow y border.
func testTheme() config.Theme {
	th := config.Theme{Accent: "#7ab8b8", Text: "#d4dadb", Dim: "#6b7a7e", Border: "#3a4448"}
	th.ResolveDerived()
	return th
}

// TestProgressBarCasosLimite: ninguna combinación puede entrar en pánico ni
// devolver algo más ancho que w. Los no finitos están en la tabla porque NaN
// es false en toda comparación —`dur <= 0` lo deja pasar— y +Inf desbordaba
// int() al mínimo de int64, que llegaba negativo a strings.Repeat y hacía
// entrar en pánico a la TUI entera (hallazgo #8 de la 1.6.1).
func TestProgressBarCasosLimite(t *testing.T) {
	th := testTheme()
	cases := []struct {
		name     string
		pos, dur float64
		w        int
		wantOK   bool // true = debe dibujar algo, false = cadena vacía
	}{
		{"duración cero", 10, 0, 40, false},
		{"duración negativa", 10, -5, 40, false},
		{"duración NaN", 10, math.NaN(), 40, false},
		{"duración Inf", 10, math.Inf(1), 40, true},
		{"posición NaN", math.NaN(), 200, 40, true},
		{"posición Inf", math.Inf(1), 200, 40, true},
		{"posición negativa", -30, 200, 40, true},
		{"posición mayor que duración", 900, 200, 40, true},
		{"duración diminuta frente a posición", 200, 1e-9, 40, true},
		{"ancho cero", 10, 200, 0, false},
		{"ancho negativo", 10, 200, -7, false},
		{"ancho por debajo del mínimo", 10, 200, progMinWidth - 1, false},
		{"ancho mínimo", 10, 200, progMinWidth, true},
		{"ratio 0", 0, 200, 40, true},
		{"ratio 1", 200, 200, 40, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := progressBar(c.pos, c.dur, c.w, th)
			if c.wantOK {
				if lipgloss.Width(got) != c.w {
					t.Errorf("ancho = %d, quería exactamente %d", lipgloss.Width(got), c.w)
				}
			} else if got != "" {
				t.Errorf("quería cadena vacía, salió %q (%d celdas)", got, lipgloss.Width(got))
			}
			// La sombra nunca puede pasarse del ancho, dibuje lo que dibuje
			// (con w negativo el tope es 0: no hay nada donde dibujar).
			tope := max(c.w, 0)
			if sh := progressShadow(c.pos, c.dur, c.w, th); lipgloss.Width(sh) > tope {
				t.Errorf("sombra = %d celdas, no puede superar %d", lipgloss.Width(sh), tope)
			}
		})
	}
}

// TestProgressBarAnchoExacto es el criterio duro del rediseño: la barra no
// desborda el panel ni lo deja corto, a ningún ancho y en ninguna posición.
// Los anchos incluyen impares y primos porque el reparto de octavos y la
// cuantización del gradiente redondean, y ahí es donde se cuela la celda de
// más o de menos.
func TestProgressBarAnchoExacto(t *testing.T) {
	th := testTheme()
	for _, w := range []int{4, 5, 7, 8, 13, 16, 17, 20, 33, 40, 81, 120, 199} {
		for i := 0; i <= 20; i++ {
			ratio := float64(i) / 20
			got := progressBar(ratio*300, 300, w, th)
			if n := lipgloss.Width(got); n != w {
				t.Errorf("w=%d ratio=%.2f: ancho renderizado %d", w, ratio, n)
			}
		}
	}
}

// TestProgressBarSubCelda: dos posiciones separadas por menos de una celda
// tienen que dar salidas DISTINTAS. Es lo que separa a esta barra de la
// anterior, que solo contaba celdas enteras y por tanto se quedaba quieta
// varios segundos y después saltaba de golpe.
func TestProgressBarSubCelda(t *testing.T) {
	th := testTheme()
	const w, dur = 20, 200.0
	celda := dur / w // 10 s por celda con estos números
	a := progressBar(5*celda, dur, w, th)
	b := progressBar(5*celda+celda/4, dur, w, th)
	c := progressBar(5*celda+celda/2, dur, w, th)
	if a == b || b == c || a == c {
		t.Error("avances de un cuarto y medio de celda dieron la misma barra: se perdió la resolución de octavos")
	}
	// Y ese avance NO puede haber cambiado el número de celdas.
	if lipgloss.Width(a) != w || lipgloss.Width(b) != w || lipgloss.Width(c) != w {
		t.Error("el avance sub-celda alteró el ancho renderizado")
	}
}

// TestProgressBarMonotona: al avanzar la reproducción, el tramo lleno nunca
// retrocede.
func TestProgressBarMonotona(t *testing.T) {
	th := testTheme()
	const w, dur = 60, 240.0
	prev := -1
	for i := 0; i <= 240; i++ {
		full := strings.Count(progressBar(float64(i), dur, w, th), "█")
		if full < prev {
			t.Fatalf("en pos=%d el tramo lleno bajó de %d a %d celdas", i, prev, full)
		}
		prev = full
	}
	if prev != w {
		t.Errorf("al final quedaron %d celdas llenas de %d: la barra no se completa", prev, w)
	}
}

// TestProgressBarEstelaYPista: la composición de una barra a media
// reproducción — tramo lleno, cabeza, estela de disolución y pista — y que a
// ratio 1 no quede ni estela ni pista.
func TestProgressBarEstelaYPista(t *testing.T) {
	th := testTheme()
	mitad := progressBar(100, 200, 40, th)
	for _, ch := range []string{"█", "▓", "▒", "░"} {
		if !strings.Contains(mitad, ch) {
			t.Errorf("a media reproducción falta %q en la barra", ch)
		}
	}
	lleno := progressBar(200, 200, 40, th)
	for _, ch := range []string{"▓", "▒", "░"} {
		if strings.Contains(lleno, ch) {
			t.Errorf("con la pista terminada no debería quedar %q", ch)
		}
	}
	vacio := progressBar(0, 200, 40, th)
	if strings.ContainsAny(vacio, "█▓▒") {
		t.Error("sin nada reproducido la barra debería ser solo pista")
	}
	if strings.Count(vacio, "░") != 40 {
		t.Errorf("la pista vacía mide %d celdas, quería 40", strings.Count(vacio, "░"))
	}
}

// TestProgressShadowDesplazada: la sombra arranca una columna a la derecha y
// cubre lo reproducido, no el ancho entero.
func TestProgressShadowDesplazada(t *testing.T) {
	th := testTheme()
	got := progressShadow(100, 200, 40, th)
	if !strings.HasPrefix(got, " ") {
		t.Error("la sombra debe arrancar desplazada una columna")
	}
	if n := strings.Count(got, "▀"); n != 20 {
		t.Errorf("sombra de %d celdas a media reproducción, quería 20", n)
	}
	if progressShadow(0, 200, 40, th) != "" {
		t.Error("sin nada reproducido no hay sombra que dibujar")
	}
	// Ni siquiera al final puede desbordar: el desplazamiento se paga del ancho.
	if n := lipgloss.Width(progressShadow(200, 200, 40, th)); n != 40 {
		t.Errorf("sombra completa = %d celdas, quería 40 (39 + el desplazamiento)", n)
	}
}
