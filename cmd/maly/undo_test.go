package main

import (
	"strings"
	"testing"
)

func TestUndoCLI(t *testing.T) {
	r, err := buildRequest("undo", nil)
	if err != nil || r.Cmd != "undo" || r.Value != "" {
		t.Fatal("gramática undo incorrecta", r, err)
	}
	if _, err = buildRequest("undo", []string{"basura"}); err == nil {
		t.Fatal("undo ignoró argumentos")
	}
	c, ok := lookup("undo")
	if !ok || c.run == nil || !strings.Contains(helpText(), "undo") {
		t.Fatal("undo sin dispatch/ayuda")
	}
}
