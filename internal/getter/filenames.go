package getter

import "fmt"

// boundedName limita un componente generado por yt-dlp a 220 bytes UTF-8.
// Quedan 35 bytes hasta NAME_MAX=255 para .webm/.mp3, .temp, .part y sufijos
// de fragmentos. B recorta por bytes y descarta una última runa incompleta;
// --trim-filenames cuenta caracteres y no da esa garantía.
//
// Primero S sanea los campos igual que la plantilla habitual, ANTES de medir
// (p. ej. '/' pasa a '⧸', que ocupa tres bytes). Solo se crean campos maly_*:
// title/artist y los demás metadatos originales siguen completos.
//
// La segunda regla compara el prefijo acotado con el nombre completo mediante
// una referencia regex. Si son iguales conserva el nombre anterior; si hay
// desbordamiento elige shortTemplate, y B limita el resultado. El ID va DELANTE
// para que no se pierda al recortar: los 11 bytes de un ID de YouTube caben
// enteros, aunque artista y título tengan el mismo prefijo en varios videos.
// Ambas reglas se ejecutan dentro de yt-dlp, antes de generar las rutas de
// audio y miniatura: no hay renombrados posteriores ni otra extracción.
func boundedName(field, template, shortTemplate string) (string, []string) {
	full, legacy, short := field+"_full", field+"_legacy", field+"_short"
	compare := fmt.Sprintf("%%(%s).220B\n%%(%s)s\n%s", full, full, shortTemplate)
	pattern := fmt.Sprintf(`(?s)\A(?P<%s>[^\n]*)\n(?P=%s)(?:\n[^\n]*|[^\n]+\n(?P<%s>[^\n]*))\Z`, legacy, legacy, short)
	return fmt.Sprintf("%%(%s,%s).220B", short, legacy), []string{
		"--parse-metadata", template + ":%(" + full + ")s",
		"--parse-metadata", compare + ":" + pattern,
	}
}
