package commandmeta

import (
	"maly/internal/i18n"
	"reflect"
	"strings"
	"testing"
)

func TestMetadataContracts(t *testing.T) {
	old := i18n.Code()
	t.Cleanup(func() { i18n.Set(old) })
	seen := map[string]bool{}
	for _, e := range entries {
		if seen[e.Name] {
			t.Errorf("nombre duplicado: %s", e.Name)
		}
		seen[e.Name] = true
		for _, lang := range []string{"es", "en"} {
			if e.Group == "" || i18n.TL(lang, e.Group) == e.Group {
				t.Errorf("%s: grupo %q sin traducción en %s", e.Name, e.Group, lang)
			}
			for _, key := range []string{e.ArgsKey, e.DescKey} {
				if key != "" && (i18n.TL(lang, key) == key || i18n.TL(lang, key) == "") {
					t.Errorf("%s: falta %s en %s", e.Name, key, lang)
				}
			}
		}
	}
	// Expectativas independientes: comparar CLI y paleta entre sí no detectaría
	// dos consumidores que anuncien la misma sintaxis equivocada.
	for _, tc := range []struct{ name, en, es string }{
		{"play", "play [<query>...]", "play [<consulta>...]"},
		{"add", "add <query|path>...", "add <consulta|ruta>..."},
		{"remove", "remove <pos>", "remove <pos>"},
		{"vol", "vol <0-100|+N|-N>", "vol <0-100|+N|-N>"},
		{"playlist delete", "playlist delete <name>", "playlist delete <nombre>"},
		{"playlist add", "playlist add <name> <query>...", "playlist add <nombre> <consulta>..."},
		{"playlist import", "playlist import <file> [<name>]", "playlist import <archivo> [<nombre>]"},
		{"get pick", "get pick <search>", "get pick <búsqueda>"},
		{"viz", "viz [bars|continuous]", "viz [bars|continuous]"},
	} {
		e, ok := Lookup(tc.name)
		if !ok {
			t.Fatal(tc.name)
		}
		for lang, want := range map[string]string{"en": tc.en, "es": tc.es} {
			i18n.Set(lang)
			if got := e.Usage(); got != want {
				t.Errorf("%s: %q, quería %q", lang, got, want)
			}
		}
	}
	want := []string{"list", "show", "create", "delete", "add", "remove", "play", "save", "export", "import"}
	if got := Subcommands("playlist"); !reflect.DeepEqual(got, want) {
		t.Errorf("playlist: %q", got)
	}
	if got := Subcommands("get"); !reflect.DeepEqual(got, []string{"pick", "playlist"}) {
		t.Errorf("get: %q", got)
	}
	for _, e := range Console() {
		if e.CLIOnly || strings.HasPrefix(e.Name, "__") || e.Name == "daemon" || e.Name == "completions" {
			t.Errorf("acción inaplicable: %s", e.Name)
		}
	}
	// La paleta dibuja una banda por grupo al cambiar de categoría: un grupo
	// partido en dos tramos repetiría su banda y mezclaría el catálogo.
	closed := map[string]bool{}
	prev := ""
	for _, e := range Console() {
		if e.Group != prev {
			if closed[e.Group] {
				t.Errorf("grupo %s partido: reaparece en %s", e.Group, e.Name)
			}
			closed[prev] = true
			prev = e.Group
		}
	}
}
