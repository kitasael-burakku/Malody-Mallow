package update

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewer(t *testing.T) {
	cases := []struct {
		remote, local string
		want          bool
	}{
		{"v1.0.3", "1.0.2", true},
		{"v1.0.2", "1.0.2", false},
		{"v1.0.1", "1.0.2", false},
		{"v1.1.0", "1.0.9", true},
		{"v2.0.0", "1.9.9", true},
		{"v1.1", "1.0.9", true}, // partes faltantes = 0
		{"basura", "1.0.2", false},
		{"v1.0.3", "basura", false},
		{"", "1.0.2", false},
	}
	for _, c := range cases {
		if got := Newer(c.remote, c.local); got != c.want {
			t.Errorf("Newer(%q, %q) = %v, quería %v", c.remote, c.local, got, c.want)
		}
	}
}

func TestLatestTag(t *testing.T) {
	out := "abc\trefs/tags/v1.0.0\n" +
		"def\trefs/tags/v1.0.10\n" + // mayor que v1.0.9 numéricamente
		"ghi\trefs/tags/v1.0.9\n" +
		"jkl\trefs/tags/beta\n" + // no versión: se ignora
		"sin-tag\n"
	if got := latestTag(out); got != "v1.0.10" {
		t.Errorf("latestTag = %q, quería v1.0.10", got)
	}
	if got := latestTag("nada\n"); got != "" {
		t.Errorf("sin tags debe dar vacío, dio %q", got)
	}
}

// TestLatestFakeGit cubre Latest con un git falso en el PATH (patrón del
// yt-dlp falso de get_test.go): imprime tags fijos sin tocar la red.
func TestLatestFakeGit(t *testing.T) {
	bin := t.TempDir()
	script := "#!/bin/sh\nprintf 'aaa\\trefs/tags/v1.0.2\\nbbb\\trefs/tags/v9.9.9\\n'\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	got, err := Latest()
	if err != nil {
		t.Fatal(err)
	}
	if got != "v9.9.9" {
		t.Errorf("Latest = %q, quería v9.9.9", got)
	}
}

// TestLatestGitFailedSurfacesStderr cubre el hallazgo D7.1 de la auditoría:
// sin red, Latest() devolvía el *exec.ExitError crudo ("exit status 128") y
// el stderr real de git —la parte útil— se descartaba. Con git falso
// imitando el fallo típico de "no hay red", el error debe contener ese
// texto y no la forma pelada "exit status N".
func TestLatestGitFailedSurfacesStderr(t *testing.T) {
	bin := t.TempDir()
	script := "#!/bin/sh\necho 'fatal: unable to access '\\''https://github.com/...'\\'': Could not resolve host: github.com' >&2\nexit 128\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	_, err := Latest()
	if err == nil {
		t.Fatal("git fallando debe propagar un error")
	}
	if strings.Contains(err.Error(), "exit status") {
		t.Errorf("el error sigue siendo el *exec.ExitError crudo: %q", err.Error())
	}
	if !strings.Contains(err.Error(), "Could not resolve host") {
		t.Errorf("el error no incluye el stderr real de git: %q", err.Error())
	}
}

func TestLatestNoGit(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if _, err := Latest(); err == nil {
		t.Error("sin git en el PATH, Latest debe fallar")
	}
}

// TestInstallerURL fija el pin del instalador al ref anunciado: antes de
// este cambio, InstallerCmd bajaba SIEMPRE el mallow-install.sh de main
// aunque el código instalado fuera un tag viejo. Puro: no toca la red.
func TestInstallerURL(t *testing.T) {
	cases := []struct{ ref, want string }{
		{"v1.2.3", "https://raw.githubusercontent.com/kitasael-burakku/Malody-Mallow/v1.2.3/mallow-install.sh"},
		{"", "https://raw.githubusercontent.com/kitasael-burakku/Malody-Mallow/main/mallow-install.sh"},
	}
	for _, c := range cases {
		if got := installerURL(c.ref); got != c.want {
			t.Errorf("installerURL(%q) = %q, quería %q", c.ref, got, c.want)
		}
	}
}

func TestCache(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	if _, fresh := Cached(); fresh {
		t.Error("sin archivo, el cache no puede estar fresco")
	}

	SaveCache("v1.2.3")
	latest, fresh := Cached()
	if !fresh || latest != "v1.2.3" {
		t.Errorf("Cached tras SaveCache = (%q, %v), quería (v1.2.3, true)", latest, fresh)
	}

	// Un chequeo viejo devuelve el valor pero ya no está fresco.
	stale, _ := json.Marshal(cache{Checked: time.Now().Add(-2 * cacheTTL), Latest: "v1.2.3"})
	if err := os.WriteFile(cachePath(), stale, 0o600); err != nil {
		t.Fatal(err)
	}
	if latest, fresh := Cached(); fresh || latest != "v1.2.3" {
		t.Errorf("cache viejo = (%q, %v), quería (v1.2.3, false)", latest, fresh)
	}
}
