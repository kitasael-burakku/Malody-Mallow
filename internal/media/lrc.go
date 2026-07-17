package media

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// LyricLine es una línea de letra; At es el instante en segundos en que
// empieza, o negativo si la línea no trae marca de tiempo (letra sin
// sincronía).
type LyricLine struct {
	At   float64
	Text string
}

var (
	// [mm:ss], [mm:ss.xx], [mm:ss.xxx] y la variante vieja [mm:ss:xx].
	lrcTime = regexp.MustCompile(`^\[(\d{1,3}):(\d{1,2})(?:[.:](\d{1,3}))?\]`)
	// Marcas de palabra <mm:ss.xx> (enhanced LRC): se quitan del texto.
	lrcWord = regexp.MustCompile(`<\d{1,3}:\d{1,2}(?:[.:]\d{1,3})?>`)
	// Tags de metadata [ar:...], [ti:...], [offset:...]; solo offset importa.
	lrcMeta = regexp.MustCompile(`^\[([A-Za-z][A-Za-z0-9#]*):(.*)\]\s*$`)
)

// ParseLRC parsea letras en formato .lrc: una o varias marcas [mm:ss.xx] por
// línea (los coros repetidos comparten texto), tag [offset:±ms] aplicado a
// todas las marcas, tags de metadata ignorados y marcas de palabra <...>
// eliminadas. Tolera BOM y CRLF. Si el texto no trae ninguna marca (letra
// plana), cada línea sale con At negativo; si trae alguna, las líneas sin
// marca (créditos sueltos) se descartan. La salida queda ordenada por At.
func ParseLRC(r io.Reader) []LyricLine {
	var (
		lines    []LyricLine
		offsetMS int
		timed    bool
	)
	sc := bufio.NewScanner(r)
	// Más holgura que los 64 KB por defecto: una línea que se pase del
	// buffer abortaría el Scan en silencio y truncaría las letras.
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	first := true
	for sc.Scan() {
		text := strings.TrimRight(sc.Text(), "\r")
		if first {
			text = strings.TrimPrefix(text, "\uFEFF") // BOM
			first = false
		}

		var ats []float64
		for {
			m := lrcTime.FindStringSubmatch(text)
			if m == nil {
				break
			}
			mins, _ := strconv.Atoi(m[1])
			secs, _ := strconv.Atoi(m[2])
			frac := 0.0
			if m[3] != "" {
				f, _ := strconv.Atoi(m[3])
				frac = float64(f) / [4]float64{1, 10, 100, 1000}[len(m[3])]
			}
			ats = append(ats, float64(mins*60+secs)+frac)
			text = text[len(m[0]):]
		}

		if len(ats) == 0 {
			if m := lrcMeta.FindStringSubmatch(strings.TrimSpace(text)); m != nil {
				if strings.EqualFold(m[1], "offset") {
					offsetMS, _ = strconv.Atoi(strings.TrimSpace(m[2]))
				}
				continue
			}
			if t := strings.TrimSpace(text); t != "" {
				lines = append(lines, LyricLine{At: -1, Text: t})
			}
			continue
		}

		timed = true
		clean := strings.TrimSpace(lrcWord.ReplaceAllString(text, ""))
		for _, at := range ats {
			lines = append(lines, LyricLine{At: at, Text: clean})
		}
	}

	if timed {
		// offset positivo = la letra va adelantada respecto al audio.
		shift := float64(offsetMS) / 1000
		out := lines[:0]
		for _, l := range lines {
			if l.At < 0 {
				continue // créditos sin marca dentro de un .lrc sincronizado
			}
			if l.At -= shift; l.At < 0 {
				l.At = 0
			}
			out = append(out, l)
		}
		lines = out
		sort.SliceStable(lines, func(i, j int) bool { return lines[i].At < lines[j].At })
	}
	return lines
}

// LyricsFor resuelve las letras de una pista: el sidecar .lrc (misma ruta con
// extensión .lrc) tiene prioridad; sin él se usan las letras embebidas ya
// leídas por ReadEmbedded (que también pueden venir en formato LRC). synced
// indica si hay marcas de tiempo para resaltar la línea actual.
func LyricsFor(trackPath, embedded string) (lines []LyricLine, synced bool) {
	base := strings.TrimSuffix(trackPath, filepath.Ext(trackPath))
	if f, err := os.Open(base + ".lrc"); err == nil {
		lines = ParseLRC(f)
		f.Close()
	}
	if len(lines) == 0 && embedded != "" {
		lines = ParseLRC(strings.NewReader(embedded))
	}
	for _, l := range lines {
		if l.At >= 0 {
			return lines, true
		}
	}
	return lines, false
}
