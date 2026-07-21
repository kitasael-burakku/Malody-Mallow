package safetext

import "testing"

// Los controles se escriben SIEMPRE con escape (\x1b, \x07) o construidos por
// rune, NUNCA con el byte literal: si no, el propio archivo de test queda
// envenenado y quien lo abra o le haga `cat` se come la inyección.
//
// Los C1 van por rune y no con  porque son justo los que un editor (o un
// copiar/pegar) convierte en silencio al carácter de verdad.
var (
	csi8 = string(rune(0x9b)) // CSI de 8 bits: un solo carácter, hace lo mismo que ESC [
	osc8 = string(rune(0x9d)) // OSC de 8 bits: lo mismo que ESC ]
)

func TestClean(t *testing.T) {
	cases := []struct {
		nombre string
		in     string
		want   string
	}{
		// Lo que hay que matar.
		{"CSI de 7 bits", "Cancion\x1b[31mROJO", "Cancion[31mROJO"},
		{"OSC que cambia el título de la ventana", "x\x1b]0;HACK\x07y", "x]0;HACKy"},
		{"CSI de 8 bits", "a" + csi8 + "31mb", "a31mb"},
		{"OSC de 8 bits", "a" + osc8 + "0;HACKb", "a0;HACKb"},
		{"salto de línea", "linea1\nlinea2", "linea1linea2"},
		{"tabulador (correría las columnas de tabwriter)", "col1\tcol2", "col1col2"},
		{"retorno de carro", "a\rb", "ab"},
		{"DEL", "a\x7fb", "ab"},
		{"NUL", "a\x00b", "ab"},

		// Lo que NO se debe tocar: metadata legítima. Esta es la regresión que
		// más importa — un filtro demasiado goloso rompe bibliotecas reales.
		{"sin nada que limpiar", "Aurora — Runaway", "Aurora — Runaway"},
		{"acentos", "Áurea Canción", "Áurea Canción"},
		{"CJK", "宇多田ヒカル", "宇多田ヒカル"},
		{"cirílico", "Гражданская оборона", "Гражданская оборона"},
		{"emoji", "track 🎵", "track 🎵"},
		{"espacios (los recorta quien llama, no Clean)", "  hola  ", "  hola  "},
		{"vacío", "", ""},
	}
	for _, c := range cases {
		t.Run(c.nombre, func(t *testing.T) {
			if got := Clean(c.in); got != c.want {
				t.Errorf("Clean(%q) = %q, quería %q", c.in, got, c.want)
			}
		})
	}
}

// Barrido de todo el rango bajo: ni un solo carácter de control sobrevive, y
// ni un solo imprimible ASCII se pierde. Vale más que una lista de casos: no
// deja hueco por el que colar un control olvidado.
func TestCleanBarridoDeControles(t *testing.T) {
	for r := rune(0); r <= 0x9f; r++ {
		imprimible := r >= 0x20 && r < 0x7f
		got := Clean(string(r))
		switch {
		case imprimible && got != string(r):
			t.Errorf("Clean(%U) = %q, debía conservarse", r, got)
		case !imprimible && got != "":
			t.Errorf("Clean(%U) = %q, debía quedar vacío", r, got)
		}
	}
}
