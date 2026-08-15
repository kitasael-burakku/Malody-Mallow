package tui

import (
	"regexp"
	"strings"

	"maly/internal/library"
)

// Limpieza de títulos SOLO PARA MOSTRAR. Casi toda la biblioteca del dueño
// entra por `maly get`, o sea por yt-dlp, y arrastra el ruido del título de
// YouTube: el artista repetido dentro del título ("Duki — DUKI — Malbec x
// Bizarrap") y sufijos de formato ("(Official Video)", "[Lyric Video]",
// "(Video Oficial)"). Ensucia la cola, el árbol y los pickers a la vez.
//
// NO toca la base de datos ni los tipos del protocolo, y es deliberado: la
// etiqueta ID3 sigue siendo la que trae el archivo, así que `maly search`, la
// CLI y cualquier cosa que parsee su salida siguen encontrando por el título
// original. Es una capa de presentación de internal/tui y nada más — por eso
// tampoco vive en internal/safetext, que es una frontera de SEGURIDAD (ver su
// doc) y no un lugar donde meter cosmética.

// noiseWords son las palabras que, POR SÍ SOLAS, no dicen nada de la música:
// nombran el formato del video del que salió el archivo. Van normalizadas con
// library.Fold (minúsculas y sin diacríticos), así "Vídeo Oficial" entra por
// la misma puerta que "Video Oficial".
//
// Un grupo entre paréntesis o corchetes se descarta solo si TODAS sus
// palabras están acá. Es más corto y más resistente que enumerar frases —el
// corpus real trae "(Official Video)", "(Video Lyric)", "(Videoclip
// Oficial)", "[Music Video]", "(Official Animated Video)" y "(Oficial)", y
// seguirán apareciendo combinaciones nuevas— y a la vez sigue siendo
// conservador: basta UNA palabra fuera de esta lista para conservar el grupo
// entero. Por eso sobreviven "(Remix)", "(feat. …)", "(with Khea)", "(Live)",
// "(sped up)", "(prod. Orodembow)" y "(2016-2017)", que son información real
// de la pista.
var noiseWords = map[string]bool{
	"official": true, "oficial": true,
	"video": true, "videoclip": true, "vid": true, "mv": true, "clip": true,
	"music": true, "musical": true, "animated": true, "animado": true,
	"audio": true, "sonido": true,
	"lyric": true, "lyrics": true, "letra": true, "letras": true,
	"visualizer": true, "visualiser": true, "visualizador": true,
	"hd": true, "hq": true, "4k": true, "full": true,
}

// bracketGroup captura un grupo entre paréntesis o corchetes. Sin anidamiento
// a propósito: lo que no matchea, no se toca.
var bracketGroup = regexp.MustCompile(`[\(\[]([^()\[\]]*)[\)\]]`)

// titleSeps son los separadores que yt-dlp deja entre artista y título. Se
// exigen espacios alrededor para no partir un nombre propio ("Jay-Z",
// "Tyler, The Creator - IGOR").
var titleSeps = []string{" — ", " – ", " - ", " · ", " | ", " : "}

// cleanTitle devuelve el título listo para mostrar: sin el artista repetido
// al principio y sin etiquetas de formato. Si limpiar lo dejaría vacío
// devuelve el título original — una fila en blanco es peor que una sucia.
func cleanTitle(artist, title string) string {
	out := stripNoiseTags(title)
	out = dropArtistPrefix(artist, out)
	if out = tidy(out); out == "" {
		return strings.TrimSpace(title)
	}
	return out
}

// stripNoiseTags borra los grupos entre paréntesis/corchetes que son ruido de
// formato, dejando intactos los que dicen algo (remix, feat, live…).
func stripNoiseTags(s string) string {
	return bracketGroup.ReplaceAllStringFunc(s, func(g string) string {
		if allNoise(g[1 : len(g)-1]) {
			return ""
		}
		return g
	})
}

// allNoise indica si el contenido de un grupo es SOLO palabras de formato.
func allNoise(inner string) bool {
	words := strings.Fields(library.Fold(inner))
	if len(words) == 0 {
		return false // "()" se deja como está: no es ruido conocido
	}
	for _, w := range words {
		if !noiseWords[strings.Trim(w, ".,;:·-")] {
			return false
		}
	}
	return true
}

// dropArtistPrefix quita del título el "Artista - " inicial que yt-dlp deja
// cuando el nombre del canal ya es el artista. Repite porque hay títulos con
// el artista dos veces ("blackbear - blackbear - hot girl bummer"), y se para
// en seco si el título ENTERO era el artista (nunca devuelve vacío por acá).
func dropArtistPrefix(artist, title string) string {
	artist = strings.TrimSpace(artist)
	if artist == "" {
		return title
	}
	folded := library.Fold(artist)
	for range 3 {
		head, rest, ok := cutAtSep(title)
		if !ok || strings.TrimSpace(rest) == "" {
			return title
		}
		if library.Fold(strings.TrimSpace(head)) != folded {
			return title
		}
		title = rest
	}
	return title
}

// cutAtSep parte por el primer separador de los conocidos.
func cutAtSep(s string) (head, rest string, ok bool) {
	best := -1
	var sep string
	for _, sp := range titleSeps {
		if i := strings.Index(s, sp); i >= 0 && (best < 0 || i < best) {
			best, sep = i, sp
		}
	}
	if best < 0 {
		return s, "", false
	}
	return s[:best], s[best+len(sep):], true
}

// tidy deja el resultado presentable: sin espacios dobles que haya dejado un
// grupo borrado y sin separadores colgando en los extremos.
func tidy(s string) string {
	s = collapseSpaces(s)
	return strings.TrimSpace(strings.Trim(strings.TrimSpace(s), "-–—·|:"))
}

func collapseSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// trackLabel es la forma "Artista — Título" para mostrar, con el título ya
// limpio: el equivalente de presentación de ipc.TrackInfo.String, que sigue
// devolviendo el texto original (lo usan la CLI y los mensajes del demonio).
func trackLabel(artist, title string) string {
	t := cleanTitle(artist, title)
	if strings.TrimSpace(artist) == "" {
		return t
	}
	return artist + " — " + t
}
