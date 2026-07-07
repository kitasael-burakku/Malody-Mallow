package i18n

import (
	"regexp"
	"testing"
)

func TestSetCode(t *testing.T) {
	t.Cleanup(func() { Set("en") })

	cases := []struct{ in, want string }{
		{"es", "es"},
		{"en", "en"},
		{"fr", "en"}, // desconocido cae en inglés
		{"", "en"},
		{"ES", "en"}, // el código es sensible a mayúsculas
	}
	for _, c := range cases {
		Set(c.in)
		if got := Code(); got != c.want {
			t.Errorf("Set(%q); Code() = %q, quería %q", c.in, got, c.want)
		}
	}
}

func TestT(t *testing.T) {
	t.Cleanup(func() { Set("en") })

	Set("en")
	if got := T("d.paused"); got != "Paused" {
		t.Errorf("T(d.paused) en inglés = %q", got)
	}
	Set("es")
	if got := T("d.paused"); got != "Pausado" {
		t.Errorf("T(d.paused) en español = %q", got)
	}
	// clave inexistente: se devuelve tal cual (falla visible)
	if got := T("no.existe"); got != "no.existe" {
		t.Errorf("T de clave inexistente = %q, quería la clave", got)
	}
}

func TestTf(t *testing.T) {
	t.Cleanup(func() { Set("en") })

	Set("es")
	if got := Tf("d.vol_set", 80); got != "Volumen 80%" {
		t.Errorf("Tf(d.vol_set, 80) = %q", got)
	}
}

func TestTL(t *testing.T) {
	t.Cleanup(func() { Set("en") })

	// idioma explícito, independiente del global
	Set("en")
	if got := TL("es", "d.stopped"); got != "Detenido" {
		t.Errorf("TL(es) con global en inglés = %q", got)
	}
	if got := TL("en", "d.stopped"); got != "Stopped" {
		t.Errorf("TL(en) = %q", got)
	}
	// código vacío o desconocido: cae en el idioma activo
	Set("es")
	if got := TL("", "d.stopped"); got != "Detenido" {
		t.Errorf("TL(\"\") con global en español = %q", got)
	}
	if got := TL("de", "d.stopped"); got != "Detenido" {
		t.Errorf("TL(de) con global en español = %q", got)
	}
	// clave inexistente
	if got := TL("en", "no.existe"); got != "no.existe" {
		t.Errorf("TL de clave inexistente = %q", got)
	}
}

func TestTLf(t *testing.T) {
	if got := TLf("en", "d.jump_oob", 7); got != "position 7 outside the queue" {
		t.Errorf("TLf(en, d.jump_oob, 7) = %q", got)
	}
	if got := TLf("es", "d.jump_oob", 7); got != "posición 7 fuera de la cola" {
		t.Errorf("TLf(es, d.jump_oob, 7) = %q", got)
	}
}

// verbRE captura los verbos de formato (%d, %s, %q, %+.3f, …); %% se filtra
// aparte porque no consume argumentos.
var verbRE = regexp.MustCompile(`%[+\-# 0-9.]*[a-zA-Z%]`)

func verbs(s string) []string {
	var out []string
	for _, v := range verbRE.FindAllString(s, -1) {
		if v != "%%" {
			out = append(out, v)
		}
	}
	return out
}

// TestTableIntegrity valida cada entrada de la tabla: ambas traducciones
// presentes y con los mismos verbos de formato en el mismo orden, para que
// Tf/TLf no exploten según el idioma.
func TestTableIntegrity(t *testing.T) {
	if len(table) == 0 {
		t.Fatal("la tabla de traducciones está vacía")
	}
	for key, tr := range table {
		if tr[en] == "" || tr[es] == "" {
			t.Errorf("%s: traducción vacía (en=%q es=%q)", key, tr[en], tr[es])
			continue
		}
		vEN, vES := verbs(tr[en]), verbs(tr[es])
		if len(vEN) != len(vES) {
			t.Errorf("%s: verbos de formato distintos: en=%v es=%v", key, vEN, vES)
			continue
		}
		for i := range vEN {
			if vEN[i] != vES[i] {
				t.Errorf("%s: verbo %d difiere: en=%q es=%q", key, i, vEN[i], vES[i])
			}
		}
	}
}
