package tui

import (
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"maly/internal/library"
)

// El lenguaje del filtro `/` de los paneles biblioteca y cola. Todo acá es
// PURO —sin Model— para que la semántica se fije con tablas de tests y no a
// ojo contra la TUI.
//
// Una consulta son términos separados por espacios, todos obligatorios (AND):
//
//	palabra     en título, artista, álbum o nombre de archivo
//	a:palabra   solo en el artista (t: título, al: álbum, f: archivo)
//	-palabra    excluye; combina con prefijo (-a:palabra)
//
// Un término positivo matchea por SUBCADENA o, si no la hay, por subsecuencia
// difusa ("htsm" → "hot summer"). Lo difuso tiene dos límites, y los dos son
// para que el filtro siga filtrando: solo desde fuzzyMinRunes runas (con una o
// dos letras casi cualquier pista contiene la subsecuencia), y siempre DENTRO
// de un campo, nunca saltando de uno a otro (sobre la concatenación, tres
// letras sueltas aparecen en casi cualquier título+artista+álbum+archivo).
// La negación es SOLO por subcadena: un "-xyz" difuso excluiría casi todo.
//
// Toda subcadena puntúa por encima de cualquier coincidencia difusa, y el
// resultado se ordena por puntuación (estable: a igual puntuación, el orden
// original). Es lo que hace soportable lo difuso: lo exacto queda arriba.

type filterField int

const (
	fieldAny filterField = iota
	fieldTitle
	fieldArtist
	fieldAlbum
	fieldFile
)

// filterPrefixes son los prefijos de campo. Abreviaturas que sirven igual en
// inglés y en español (t título/title, a artista/artist, al álbum/album, f
// fichero/file).
var filterPrefixes = []struct {
	prefix string
	field  filterField
}{
	{"t:", fieldTitle},
	{"a:", fieldArtist},
	{"al:", fieldAlbum},
	{"f:", fieldFile},
}

const (
	fuzzyMinRunes = 3

	// scoreSubstr es el piso de una coincidencia por subcadena; fuzzyScore
	// se topa por debajo, así que lo exacto siempre gana.
	scoreSubstr     = 1000
	bonusFieldStart = 200
	bonusWordStart  = 100
	fuzzyConsec     = 10
	fuzzyWordStart  = 8
)

type filterTerm struct {
	neg   bool
	field filterField
	text  string // ya plegado (library.Fold)
	runes []rune
}

// filterQuery es una consulta ya parseada. La vacía (sin términos) deja pasar
// todo.
type filterQuery struct {
	terms []filterTerm
}

func (q filterQuery) empty() bool { return len(q.terms) == 0 }

// parseFilter parsea el texto del input. Los términos vacíos ("-", "a:")
// aparecen al escribir y se ignoran en vez de filtrar por "nada". Un prefijo
// que no es de campo ("feat:x") es texto literal.
func parseFilter(s string) filterQuery {
	var q filterQuery
	for _, w := range strings.Fields(library.Fold(s)) {
		var t filterTerm
		if strings.HasPrefix(w, "-") {
			t.neg = true
			w = w[1:]
		}
		for _, p := range filterPrefixes {
			if strings.HasPrefix(w, p.prefix) {
				t.field = p.field
				w = w[len(p.prefix):]
				break
			}
		}
		if w == "" {
			continue
		}
		t.text = w
		t.runes = []rune(w)
		q.terms = append(q.terms, t)
	}
	return q
}

// foldedTrack son los campos buscables de una pista, ya plegados. Se cachea
// por pista (libTree.folded, Model.queueFolded): plegar Unicode la biblioteca
// entera en cada tecla pesa con 40k pistas.
type foldedTrack struct {
	title, artist, album, file string
}

// foldTrack pliega los campos de una pista. file es el nombre del archivo sin
// carpeta ni extensión: sirve con pistas sin tags. Solo se BUSCA en él, nunca
// se pinta — por eso Path no pasa por safetext, como manda el invariante.
func foldTrack(title, artist, album, path string) foldedTrack {
	ft := foldedTrack{
		title:  library.Fold(title),
		artist: library.Fold(artist),
		album:  library.Fold(album),
	}
	if path != "" {
		base := filepath.Base(path)
		ft.file = library.Fold(strings.TrimSuffix(base, filepath.Ext(base)))
	}
	return ft
}

// fields devuelve los campos en los que busca un término.
func (ft *foldedTrack) fields(f filterField) []string {
	switch f {
	case fieldTitle:
		return []string{ft.title}
	case fieldArtist:
		return []string{ft.artist}
	case fieldAlbum:
		return []string{ft.album}
	case fieldFile:
		return []string{ft.file}
	}
	return []string{ft.title, ft.artist, ft.album, ft.file}
}

// match reporta si la pista pasa la consulta y con qué puntuación.
func (q filterQuery) match(ft *foldedTrack) (int, bool) {
	total := 0
	for _, t := range q.terms {
		fs := ft.fields(t.field)
		if t.neg {
			for _, s := range fs {
				if strings.Contains(s, t.text) {
					return 0, false
				}
			}
			continue
		}
		best := 0
		for _, s := range fs {
			if sc := t.score(s); sc > best {
				best = sc
			}
		}
		if best == 0 {
			return 0, false
		}
		total += best
	}
	return total, true
}

// score puntúa el término contra un campo; 0 es "no matchea".
func (t filterTerm) score(s string) int {
	if i := strings.Index(s, t.text); i >= 0 {
		switch {
		case i == 0:
			return scoreSubstr + bonusFieldStart
		case wordStartAt(s, i):
			return scoreSubstr + bonusWordStart
		}
		return scoreSubstr
	}
	if len(t.runes) < fuzzyMinRunes {
		return 0
	}
	return fuzzyScore(s, t.runes)
}

// wordStartAt reporta si el byte i de s empieza una palabra.
func wordStartAt(s string, i int) bool {
	if i == 0 {
		return true
	}
	r, _ := utf8.DecodeLastRuneInString(s[:i])
	return !unicode.IsLetter(r) && !unicode.IsDigit(r)
}

// fuzzyScore busca pat como subsecuencia de s (voraz, de izquierda a
// derecha) y puntúa las letras consecutivas y las que empiezan palabra. 0 si
// no está; nunca llega a scoreSubstr.
func fuzzyScore(s string, pat []rune) int {
	score, pi := 1, 0
	prevMatched := false
	prev := ' '
	for _, r := range s {
		if pi < len(pat) && r == pat[pi] {
			if prevMatched {
				score += fuzzyConsec
			}
			if !unicode.IsLetter(prev) && !unicode.IsDigit(prev) {
				score += fuzzyWordStart
			}
			pi++
			prevMatched = true
		} else {
			prevMatched = false
		}
		prev = r
	}
	if pi < len(pat) {
		return 0
	}
	return min(score, scoreSubstr-1)
}

// filterRank devuelve los índices de folded que pasan q, ordenados por
// puntuación de mayor a menor y, a igual puntuación, en el orden original.
// Sin términos positivos todas puntúan 0 y el orden queda intacto.
func filterRank(q filterQuery, folded []foldedTrack) []int {
	type hit struct{ idx, score int }
	hits := make([]hit, 0, len(folded))
	for i := range folded {
		if sc, ok := q.match(&folded[i]); ok {
			hits = append(hits, hit{i, sc})
		}
	}
	sort.SliceStable(hits, func(a, b int) bool { return hits[a].score > hits[b].score })
	out := make([]int, len(hits))
	for i, h := range hits {
		out[i] = h.idx
	}
	return out
}

// highlight marca las runas de label que coinciden con algún término
// positivo, para resaltarlas en la fila. Solo cuentan los términos que pueden
// estar en la etiqueta ("Artista — Título"): sin prefijo, t: y a:. Es
// cosmético: no decide qué pasa el filtro, eso es match.
//
// Pliega RUNA A RUNA y no la cadena entera: Fold puede cambiar la longitud
// (descompone y quita marcas), y las posiciones tienen que caer sobre las
// runas de label tal cual se pintan.
func (q filterQuery) highlight(label string) []bool {
	if q.empty() {
		return nil
	}
	src := []rune(label)
	hay := make([]rune, len(src))
	for i, r := range src {
		hay[i] = foldRune(r)
	}
	marks := make([]bool, len(src))
	marked := false
	for _, t := range q.terms {
		if t.neg || (t.field != fieldAny && t.field != fieldTitle && t.field != fieldArtist) {
			continue
		}
		found := false
		for from := 0; from+len(t.runes) <= len(hay); {
			i := indexRunes(hay[from:], t.runes)
			if i < 0 {
				break
			}
			for k := range t.runes {
				marks[from+i+k] = true
			}
			found, marked = true, true
			from += i + len(t.runes)
		}
		if found || len(t.runes) < fuzzyMinRunes {
			continue
		}
		// Sin subcadena: la subsecuencia, si la hay entera.
		var pos []int
		pi := 0
		for i, r := range hay {
			if pi < len(t.runes) && r == t.runes[pi] {
				pos = append(pos, i)
				pi++
			}
		}
		if pi == len(t.runes) {
			for _, p := range pos {
				marks[p] = true
			}
			marked = true
		}
	}
	if !marked {
		return nil
	}
	return marks
}

// foldRune pliega una runa suelta como library.Fold, conservando el 1:1: si
// el plegado no da exactamente una runa (una marca combinante sola da cero),
// queda la minúscula de la original.
func foldRune(r rune) rune {
	f := library.Fold(string(r))
	if fr, n := utf8.DecodeRuneInString(f); n > 0 && n == len(f) {
		return fr
	}
	return unicode.ToLower(r)
}

func indexRunes(hay, needle []rune) int {
	if len(needle) == 0 {
		return -1
	}
outer:
	for i := 0; i+len(needle) <= len(hay); i++ {
		for k, r := range needle {
			if hay[i+k] != r {
				continue outer
			}
		}
		return i
	}
	return -1
}
