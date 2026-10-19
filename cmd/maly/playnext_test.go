package main

import (
	"maly/internal/i18n"
	"reflect"
	"strings"
	"testing"
)

func TestPlayNextCLI(t *testing.T) {
	req, err := buildRequest("playnext", []string{"Luz", "Otra toma"})
	if err != nil || req.Cmd != "playnext" || !reflect.DeepEqual(req.QueryList(), []string{"Luz", "Otra toma"}) {
		t.Fatal("gramática playnext incorrecta", req, err)
	}
	if _, err = buildRequest("playnext", nil); err == nil || err.Error() != i18n.T("cli.usage_playnext") {
		t.Fatal("falta uso sin argumentos", err)
	}
	c, ok := lookup("playnext")
	if !ok || c.run == nil || c.complete == nil {
		t.Fatal("falta dispatch/completado playnext")
	}
	if !strings.Contains(helpText(), "playnext") {
		t.Fatal("falta ayuda playnext")
	}
}
