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
// Pero el directorio TAMPOCO alcanza solo, y por un caso que la 1.15.0 no
// consideró: yt-dlp no vuelve a bajar un archivo que ya existe —sale 0, no
// toca el disco— así que el diff queda vacío igual que cuando no encontró
// nada, y esos dos tienen remedios opuestos (reformular vs. no hacer nada).
// Lo separa ReadPaths, con el archivo de --print-to-file (ver Opts.PathsFile
// y A-07).
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
// before.
//
// ok=false cubre DOS casos distintos que a este nivel no se pueden separar:
// que no bajara nada (ver la tabla de la cabecera: una búsqueda sin
// resultados sale 0 sin dejar archivo) y que bajara más de uno, porque una
// búsqueda puede resolver a varios ítems y entonces no hay UNA pista que
// nombrar. Quien necesite distinguirlos usa NewAudioAll y mira el conteo.
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
// miniatura ANTES del audio, así que un fallo la deja huérfana). NUNCA se
// llama tras una descarga exitosa: ahí los intermedios ya los limpió yt-dlp,
// y lo que quedara podría ser una carátula que el usuario quiere.
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

// ReadPaths lee el archivo de rutas que dejó --print-to-file
// (Opts.PathsFile): una ruta final por línea, incluidas las de los archivos
// que yt-dlp SALTÓ por existir ya.
//
// Es lo que separa los tres desenlaces que el diff del directorio colapsaba
// en uno (A-07):
//
//	yt-dlp 0 · rutas 1 · diff 1 nuevo  → descargado
//	yt-dlp 0 · rutas 1 · diff vacío    → ya lo tenías (NO es error)
//	yt-dlp 0 · rutas 0 · diff vacío    → la búsqueda no encontró nada
//	yt-dlp ≠0                          → falló (+ Cleanup)
//
// Un archivo ausente o ilegible devuelve nil sin error: el llamador vuelve
// entonces al criterio del diff solo, que es el comportamiento anterior. Así
// una versión de yt-dlp que no escribiera nada degrada en silencio en vez de
// romper la descarga.
func ReadPaths(name string) []string {
	data, err := os.ReadFile(name)
	if err != nil {
		return nil
	}
	var out []string
	for _, l := range strings.Split(string(data), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}
