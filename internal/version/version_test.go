package version

import "testing"

// TestPackagedChannelOverride: con Channel fijado (lo que hace el PKGBUILD
// vía -ldflags -X), Packaged() da true sin ni siquiera mirar la ruta del
// binario — el corto-circuito es la autoridad.
func TestPackagedChannelOverride(t *testing.T) {
	old := Channel
	defer func() { Channel = old }()

	Channel = ""
	if Packaged() {
		t.Error("sin Channel y corriendo como binario de test (no bajo /usr/), Packaged() debía dar false")
	}

	Channel = "pacman"
	if !Packaged() {
		t.Error("con Channel fijado, Packaged() debía dar true sin mirar la ruta")
	}
}

// TestIsPackagedPath cubre la heurística de FHS: /usr/ es territorio de un
// gestor de paquetes salvo /usr/local/, donde mallow-install.sh instala
// con --system.
func TestIsPackagedPath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"/usr/bin/maly", true},
		{"/usr/lib/maly-wrapper/maly", true},
		{"/usr/local/bin/maly", false},
		{"/home/user/.local/bin/maly", false},
		{"/opt/maly/maly", false},
		{"", false},
	}
	for _, c := range cases {
		if got := isPackagedPath(c.path); got != c.want {
			t.Errorf("isPackagedPath(%q) = %v, quería %v", c.path, got, c.want)
		}
	}
}
