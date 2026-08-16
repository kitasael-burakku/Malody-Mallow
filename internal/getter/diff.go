package getter

// Qué produjo una descarga, medido mirando el DIRECTORIO y no la salida de
// yt-dlp. Es el mismo mecanismo que `get playlist` usa desde la 1.11.0 para
// aprender el subdirectorio que yt-dlp creó, ahora compartido y con un
// segundo trabajo encima: decidir si la descarga funcionó.
//
// Hace falta porque el código de salida de yt-dlp NO alcanza como criterio.
// Medido caso por caso (sin pipes de por medio, que devuelven el estado del
// último comando y no el de yt-dlp — esa trampa costó un diagnóstico entero):
//
//	descarga correcta      → 0, con el mp3
//	HTTP 403               → 1, dejando SOLO la miniatura .webp
//	video inexistente      → 1, sin dejar nada
//	búsqueda sin resultados → 0, SIN DEJAR NADA   ← el agujero
//
// O sea que el código de salida acierta en los fallos de descarga, pero una
// consulta que no encuentra nada —un typo basta— sale 0 y hacía que maly
// anunciara "Descarga lista" y luego "La biblioteca está vacía". Mirar el
// directorio cubre ese caso y cualquier otro futuro, es determinista, y no
// obliga a parsear la salida de yt-dlp, que es justo lo que el proyecto
// evita.
//
// Importar library desde acá no arrastra nada: IsAudio es el filtro único de
// extensiones del proyecto (duplicar su lista sería peor), y los dos
// paquetes que usan getter —cmd/maly e internal/tui— ya importan library.

import (
	"os"
	"path/filepath"
	"strings"

	"maly/internal/library"
)

// leftoverExts son los archivos INTERMEDIOS que yt-dlp deja al fallar a
// mitad: la miniatura que iba a embeber y las descargas parciales. La lista
// es corta y explícita a propósito — Cleanup borra archivos, así que solo
// puede tocar lo que es inequívocamente basura de una descarga fallida.
var leftoverExts = map[string]bool{
	".webp": true, ".jpg": true, ".jpeg": true, ".png": true,
	".part": true, ".ytdl": true, ".temp": true,
}

// Snapshot lista los nombres de entrada de dir (no recursivo), para poder
// diffear después de la descarga.
func Snapshot(dir string) (map[string]bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	m := make(map[string]bool, len(entries))
	for _, e := range entries {
		m[e.Name()] = true
	}
	return m, nil
}

// NewAudio devuelve el único archivo de audio que apareció en dir respecto a
// before. ok=false significa que la descarga NO dejó ninguna pista — con
// yt-dlp saliendo 0 pase lo que pase, es la única señal fiable de fallo.
//
// Con más de uno también devuelve false: una búsqueda puede resolver a más
// de un ítem, y ahí no hay una pista que nombrar. Los llamadores distinguen
// los dos casos con Count si les importa.
func NewAudio(dir string, before map[string]bool) (string, bool) {
	found := NewAudioAll(dir, before)
	if len(found) != 1 {
		return "", false
	}
	return found[0], true
}

// NewAudioAll devuelve todos los archivos de audio nuevos en dir respecto a
// before. Es lo que separa "no bajó nada" (cero) de "bajó varios".
func NewAudioAll(dir string, before map[string]bool) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var found []string
	for _, e := range entries {
		if e.IsDir() || before[e.Name()] || !library.IsAudio(e.Name()) {
			continue
		}
		found = append(found, e.Name())
	}
	return found
}

// NewSubdir devuelve el único subdirectorio que apareció en parent respecto
// a before —el que yt-dlp creó con el título de la playlist— y cuántos hubo.
// El llamador decide qué error dar según el conteo (0 y >1 tienen causas
// distintas y mensajes distintos; ver runGetPlaylist).
func NewSubdir(parent string, before map[string]bool) (string, int) {
	entries, err := os.ReadDir(parent)
	if err != nil {
		return "", 0
	}
	var found string
	n := 0
	for _, e := range entries {
		if !e.IsDir() || before[e.Name()] {
			continue
		}
		found = e.Name()
		n++
	}
	return found, n
}

// Cleanup borra los intermedios que dejó una descarga fallida y devuelve
// cuántos quitó. Solo toca archivos que cumplen LAS TRES condiciones: no
// existían antes de esta descarga, están en el primer nivel de dir, y su
// extensión está en leftoverExts. Sin las tres, esto sería un borrador de
// archivos del usuario con pasos extra.
//
// El caso que lo justifica es el .webp de la miniatura, que sin esto se
// acumula en music_dir con cada 403 de YouTube (medido: yt-dlp baja la
// miniatura ANTES del audio, así que un 403 la deja huérfana). NUNCA se llama tras una
// descarga exitosa: ahí los intermedios ya los limpió yt-dlp, y lo que
// quedara podría ser una carátula que el usuario quiere.
func Cleanup(dir string, before map[string]bool) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		if e.IsDir() || before[e.Name()] {
			continue
		}
		if !leftoverExts[strings.ToLower(filepath.Ext(e.Name()))] {
			continue
		}
		if os.Remove(filepath.Join(dir, e.Name())) == nil {
			n++
		}
	}
	return n
}
