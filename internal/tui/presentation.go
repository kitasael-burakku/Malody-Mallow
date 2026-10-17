package tui

import "strings"

// terminalPresentation reconcilia el texto compuesto con el ancho que usa
// nuestro renderer. x/ansi y reflow cuentan ☕︎ como dos celdas, pero Kitty
// aplica VS15 (U+FE0E, presentación texto) y lo estrecha a una: desde ahí se
// desplazan todos los bordes de esa fila. Omitir VS15 conserva el símbolo con
// su presentación predeterminada y el ancho que ya reservó el layout.
//
// Se aplica SOLO al resultado final de las tres View de Bubble Tea, después
// de recortar, resaltar y componer. Así no cambia índices de coincidencias,
// consultas, rutas ni metadatos. El costo es perder la preferencia tipográfica
// de VS15 en pantalla. Se conservan VS16, ZWJ, acentos y los diacríticos del
// protocolo gráfico Kitty; no es una limpieza general de caracteres Unicode.
func terminalPresentation(s string) string {
	return strings.ReplaceAll(s, "\ufe0e", "")
}
