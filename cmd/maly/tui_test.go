package main

import (
	"errors"
	"strings"
	"testing"

	"maly/internal/daemon"
	"maly/internal/i18n"
)

// TestEmbeddedStartErrTraduceAlreadyRunning cubre el hallazgo D7.2 de la
// auditoría: runDaemon (client.go) ya traducía daemon.ErrAlreadyRunning con
// i18n.T("d.already"), pero runTUI lo envolvía crudo con un prefijo en
// inglés — un usuario en español veía la mitad del mensaje en español (el
// prefijo) y la otra mitad en inglés (el sentinel, sin traducir). En inglés
// el texto de d.already coincide byte a byte con el sentinel, así que la
// prueba real solo se ve en español. El prefijo para los demás errores se
// quitó después (D7.4, ver el bloque final).
func TestEmbeddedStartErrTraduceAlreadyRunning(t *testing.T) {
	old := i18n.Code()
	i18n.Set("es")
	defer i18n.Set(old)

	err := embeddedStartErr(daemon.ErrAlreadyRunning)
	if strings.Contains(err.Error(), "already running") {
		t.Errorf("con idioma español, el sentinel en inglés no debía colarse: %v", err)
	}
	if !strings.Contains(err.Error(), "ya hay un servicio") {
		t.Errorf("esperaba el texto traducido de d.already, salió: %v", err)
	}
	if strings.Contains(err.Error(), "iniciando el servicio integrado") {
		t.Errorf("ErrAlreadyRunning no debía llevar el prefijo de arranque embebido: %v", err)
	}

	// Un error wrapeado (errors.Is debe seguir la cadena, no comparar
	// directo) también cuenta.
	err = embeddedStartErr(errWrap(daemon.ErrAlreadyRunning))
	if !strings.Contains(err.Error(), "ya hay un servicio") {
		t.Errorf("ErrAlreadyRunning envuelto también debía traducirse: %v", err)
	}

	// Cualquier otro error (p. ej. "mpv is not installed") ya es una frase
	// completa y accionable por sí sola: el prefijo genérico de arranque
	// embebido no agregaba información, solo ruido (auditoría 2026-07-31,
	// hallazgo D7.4 — revierte la decisión original de D7.2 de conservarlo).
	other := errors.New("algo salió mal")
	err = embeddedStartErr(other)
	if strings.Contains(err.Error(), "iniciando el servicio integrado") {
		t.Errorf("otros errores ya no debían llevar el prefijo genérico: %v", err)
	}
	if err.Error() != "algo salió mal" {
		t.Errorf("otros errores debían pasar tal cual, sin envolver: %v", err)
	}
}

// errWrap envuelve target con %w, como haría cualquier código real de
// producción — para probar que embeddedStartErr usa errors.Is (que sigue la
// cadena) y no una comparación directa.
func errWrap(target error) error {
	return &wrapErr{msg: "arrancando mpv", err: target}
}

type wrapErr struct {
	msg string
	err error
}

func (w *wrapErr) Error() string { return w.msg + ": " + w.err.Error() }
func (w *wrapErr) Unwrap() error { return w.err }
