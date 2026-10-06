package tui

import (
	"reflect"
	"testing"
)

// TestParseFilter fija la gramática del filtro: prefijos de campo, negación
// combinable, términos vacíos ignorados (aparecen a mitad de escribir) y un
// prefijo desconocido como texto literal.
func TestParseFilter(t *testing.T) {
	type term struct {
		neg   bool
		field filterField
		text  string
	}
	cases := []struct {
		in   string
		want []term
	}{
		{"", nil},
		{"  hot   SUMMER ", []term{{false, fieldAny, "hot"}, {false, fieldAny, "summer"}}},
		{"a:Drake", []term{{false, fieldArtist, "drake"}}},
		{"al:views t:one f:rip", []term{{false, fieldAlbum, "views"}, {false, fieldTitle, "one"}, {false, fieldFile, "rip"}}},
		{"-live -a:drake", []term{{true, fieldAny, "live"}, {true, fieldArtist, "drake"}}},
		{"- a: -al: x", []term{{false, fieldAny, "x"}}},
		{"feat:x", []term{{false, fieldAny, "feat:x"}}},
		{"Ádele", []term{{false, fieldAny, "adele"}}},
	}
	for _, c := range cases {
		var got []term
		for _, tm := range parseFilter(c.in).terms {
			got = append(got, term{tm.neg, tm.field, tm.text})
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("parseFilter(%q) = %+v, quería %+v", c.in, got, c.want)
		}
	}
}

// TestFilterMatch cubre la semántica de un término contra una pista.
func TestFilterMatch(t *testing.T) {
	ft := foldTrack("Hot Summer", "f(x)", "Hot Summer", "/música/Rips/kpop_01-hs.flac")
	cases := []struct {
		q    string
		want bool
	}{
		{"", true},
		{"hot sum", true},
		{"hot drake", false},   // AND: todos los términos
		{"htsm", true},         // fuzzy dentro del título
		{"ht", false},          // < fuzzyMinRunes: solo subcadena
		{"t:hot a:f(x)", true}, // prefijos
		{"a:hot", false},       // "hot" no está en el artista
		{"f:kpop", true},       // nombre de archivo
		{"f:rips", false},      // la carpeta NO es buscable
		{"f:flac", false},      // la extensión tampoco
		{"-summer", false},     // negación
		{"-a:summer", true},    // negación acotada al artista
		{"-htsm", true},        // la negación NO es difusa
		{"MÚSICA", false},      // carpeta, aunque esté plegada
		{"hótsümmer", true},    // acentos plegados + fuzzy
		{"hsfx", false},        // fuzzy NO cruza campos (h,s del título + f,x del artista)
	}
	for _, c := range cases {
		if _, ok := parseFilter(c.q).match(&ft); ok != c.want {
			t.Errorf("match(%q) = %v, quería %v", c.q, ok, c.want)
		}
	}
}

// TestFilterRankOrden: toda subcadena va antes que cualquier coincidencia
// difusa, el inicio de campo o de palabra suma, y a igual puntuación manda
// el orden original (estable).
func TestFilterRankOrden(t *testing.T) {
	folded := []foldedTrack{
		foldTrack("Has Mo", "", "", ""),   // 0: fuzzy de "hsm"
		foldTrack("Ahsm", "", "", ""),     // 1: subcadena a mitad de palabra
		foldTrack("x hsm", "", "", ""),    // 2: subcadena a inicio de palabra
		foldTrack("hsm", "", "", ""),      // 3: subcadena a inicio de campo
		foldTrack("nada", "", "", ""),     // 4: no pasa
		foldTrack("otro hsm", "", "", ""), // 5: empata con 2
	}
	got := filterRank(parseFilter("hsm"), folded)
	want := []int{3, 2, 5, 1, 0}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("filterRank = %v, quería %v", got, want)
	}
	// Sin términos positivos todas puntúan igual: el orden queda intacto.
	got = filterRank(parseFilter("-nada"), folded)
	if want := []int{0, 1, 2, 3, 5}; !reflect.DeepEqual(got, want) {
		t.Errorf("filterRank solo negación = %v, quería %v", got, want)
	}
}

// TestHighlight: las marcas caen sobre las runas de la etiqueta TAL CUAL se
// pinta, también con acentos y caracteres anchos, y solo para los términos
// que pueden estar en ella.
func TestHighlight(t *testing.T) {
	mk := func(s string) []bool {
		out := []bool{}
		for _, r := range s {
			out = append(out, r == '^')
		}
		return out
	}
	cases := []struct {
		q, label string
		want     []bool // nil = sin marcas
	}{
		{"cafe", "Café — Él", mk("^^^^     ")},
		{"el", "Café — Él", mk("       ^^")},
		{"夜に", "YOASOBI — 夜に駆ける", mk("          ^^   ")},
		// NFD (tags y nombres venidos de macOS): Fold sobre la cadena entera
		// se come la marca combinante y corre todas las posiciones una runa.
		{"x", "Cafe\u0301 — x", mk("        ^")},
		{"ysb", "YOASOBI — x", mk("^  ^ ^     ")},               // subsecuencia
		{"a:yoa -yoa al:yoa", "YOASOBI — x", mk("^^^        ")}, // ni negación ni álbum
		{"al:yoa", "YOASOBI — x", nil},
		{"zz", "YOASOBI — x", nil},
		{"", "YOASOBI", nil},
	}
	for _, c := range cases {
		got := parseFilter(c.q).highlight(c.label)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("highlight(%q, %q) = %v, quería %v", c.q, c.label, got, c.want)
		}
	}
}
