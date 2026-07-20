package probe

import (
	"os"
	"path/filepath"
	"testing"
)

// fakeFfprobe instala en un dir nuevo (al frente del PATH) un ejecutable
// "ffprobe" con el cuerpo dado. Mismo molde que el mpv falso de
// internal/player: nada de depender del ffprobe real de la máquina.
func fakeFfprobe(t *testing.T, body string) {
	t.Helper()
	dir := t.TempDir()
	script := "#!/bin/sh\n" + body + "\n"
	if err := os.WriteFile(filepath.Join(dir, "ffprobe"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestAvailable(t *testing.T) {
	fakeFfprobe(t, "exit 0")
	if !Available() {
		t.Fatal("con ffprobe en el PATH, Available debe ser true")
	}
	// Sin nada en el PATH la feature se salta en silencio: no es error.
	t.Setenv("PATH", t.TempDir())
	if Available() {
		t.Fatal("sin ffprobe en el PATH, Available debe ser false")
	}
}

func TestDuration(t *testing.T) {
	fakeFfprobe(t, "echo 212.504000")
	got, err := Duration("/da/igual.mp3")
	if err != nil {
		t.Fatalf("Duration devolvió error: %v", err)
	}
	if got != 212.504 {
		t.Fatalf("Duration = %v, quería 212.504", got)
	}
}

// El archivo ilegible (corrupto, no-audio) hace salir a ffprobe con error y su
// diagnóstico va a stderr: Duration lo reporta sin ensuciar el terminal.
func TestDurationFailingFile(t *testing.T) {
	fakeFfprobe(t, "echo 'Invalid data found' >&2 ; exit 1")
	if _, err := Duration("/basura.mp3"); err == nil {
		t.Fatal("un ffprobe que falla debe devolver error")
	}
}

// Un contenedor sin duración conocida imprime "N/A" y sale con 0; no es una
// duración usable.
func TestDurationNotAvailable(t *testing.T) {
	fakeFfprobe(t, "echo N/A")
	if _, err := Duration("/raro.mp3"); err == nil {
		t.Fatal("N/A no es una duración usable")
	}
	fakeFfprobe(t, "echo 0")
	if _, err := Duration("/cero.mp3"); err == nil {
		t.Fatal("una duración 0 no es usable (es el valor de 'no aprendida')")
	}
}
