package daemon

import (
	"os"
	"testing"

	"maly/internal/config"
)

// TestSessionFilePrivate: session.json se guarda 0600 (lista qué escuchas);
// el tmp+rename además aprieta un session.json 0644 de versiones viejas.
func TestSessionFilePrivate(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	if err := os.MkdirAll(config.DataDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	// Un session.json previo con permisos flojos debe quedar 0600 tras guardar.
	if err := os.WriteFile(sessionPath(), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := saveSession(session{V: sessionVersion}); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(sessionPath())
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("session.json: %o, quería 0600", fi.Mode().Perm())
	}
}
