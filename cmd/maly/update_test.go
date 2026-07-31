package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"maly/internal/version"
)

// captureStdout redirige os.Stdout mientras corre fn y devuelve lo impreso.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = old }()

	fn()

	w.Close()
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// fakeGitNewerTag pone en el PATH un git falso que reporta un tag más
// nuevo que version.Version (mismo patrón que TestLatestFakeGit en
// internal/update/update_test.go).
func fakeGitNewerTag(t *testing.T, bin string) {
	t.Helper()
	script := "#!/bin/sh\nprintf 'aaa\\trefs/tags/v9.9.9\\n'\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

// TestRunUpdatePackaged: con version.Channel fijado, un release más nuevo
// no debe disparar el instalador — ni siquiera intentarlo. El PATH no
// tiene curl a propósito: si runUpdate igual llegara a InstallerCmd,
// fallaría con "curl no está en tu PATH" y el test lo cazaría.
func TestRunUpdatePackaged(t *testing.T) {
	xdgSandbox(t)
	bin := t.TempDir()
	fakeGitNewerTag(t, bin)
	t.Setenv("PATH", bin)

	old := version.Channel
	version.Channel = "pacman"
	defer func() { version.Channel = old }()

	out := captureStdout(t, func() {
		if err := runUpdate(nil); err != nil {
			t.Errorf("runUpdate con canal empaquetado no debía fallar: %v", err)
		}
	})
	if !strings.Contains(out, "v9.9.9") || !strings.Contains(out, "package manager") {
		t.Errorf("salida = %q, esperaba mencionar v9.9.9 y el gestor de paquetes", out)
	}
}

// TestRunUpdateNotPackagedNeedsCurl: sin Channel (el caso de siempre), un
// release más nuevo SÍ debe intentar el instalador — y sin curl en el
// PATH, fallar mencionándolo. Confirma que el gate no rompió el camino no
// empaquetado.
func TestRunUpdateNotPackagedNeedsCurl(t *testing.T) {
	xdgSandbox(t)
	bin := t.TempDir()
	fakeGitNewerTag(t, bin)
	t.Setenv("PATH", bin)

	old := version.Channel
	version.Channel = ""
	defer func() { version.Channel = old }()

	err := runUpdate(nil)
	if err == nil || !strings.Contains(err.Error(), "curl") {
		t.Errorf("runUpdate sin canal y sin curl debía fallar mencionando curl; err = %v", err)
	}
}

// TestRunUpdateCurrentVersion: si no hay nada más nuevo, el mensaje es
// "al día" sin importar el canal — el chequeo de versión gana primero.
func TestRunUpdateCurrentVersion(t *testing.T) {
	xdgSandbox(t)
	bin := t.TempDir()
	script := fmt.Sprintf("#!/bin/sh\nprintf 'aaa\\trefs/tags/v%s\\n'\n", version.Version)
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)

	old := version.Channel
	version.Channel = "pacman"
	defer func() { version.Channel = old }()

	out := captureStdout(t, func() {
		if err := runUpdate(nil); err != nil {
			t.Errorf("runUpdate al día no debía fallar: %v", err)
		}
	})
	if strings.Contains(out, "package manager") {
		t.Errorf("al día no debía mencionar el gestor de paquetes: %q", out)
	}
}
