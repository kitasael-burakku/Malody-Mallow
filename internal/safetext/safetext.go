// Package safetext descarta los caracteres de control del texto que maly no
// controla —tags de las pistas, letras, nombres de archivo, respuestas del
// demonio— antes de que llegue a un terminal o al bus D-Bus.
//
// Hace falta porque nadie más lo hacía: los tags entran por dhowden/tag tal
// cual, y el recorte de la TUI (reflow/truncate) es ANSI-aware, así que
// CONSERVA las secuencias de escape en vez de comérselas. Un título con
// "\x1b]0;…\x07" cambia el título de la ventana del usuario, y con OSC 52 le
// escribe el portapapeles: basta con indexar un mp3 ajeno.
//
// Es un paquete propio y no una función de library porque también lo necesitan
// media e ipc, y ninguno de los dos importa library — hacerlo arrastraría
// SQLite hasta mpris.
package safetext

import "strings"

// Clean devuelve s sin ningún carácter de control: C0 (< 0x20, incluye ESC,
// BEL, \n y \t), DEL (0x7F) y C1 (0x80–0x9F). El resto de Unicode pasa intacto:
// acentos, CJK y emoji son metadata legítima y no se tocan.
//
// Filtra RUNAS, no bytes: quitar solo ESC dejaría pasar el CSI de 8 bits
// (U+009B) y el OSC de 8 bits (U+009D), que son esas mismas órdenes en un solo
// carácter.
//
// El tabulador se va a propósito: printTracks arma su tabla con text/tabwriter
// usando \t de separador, así que uno dentro de un título le correría las
// columnas.
//
// Descarta el carácter en vez de la secuencia entera —"\x1b[31m" queda como
// "[31m"—: el filtro es inelidible por construcción, y de paso el intento queda
// visible en vez de desaparecer sin rastro.
func Clean(s string) string {
	// strings.Map no reserva nada si ningún carácter cambia, que es el caso
	// normal; y de regalo normaliza el UTF-8 inválido a U+FFFD.
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f) {
			return -1
		}
		return r
	}, s)
}
