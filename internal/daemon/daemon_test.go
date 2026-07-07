package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"maly/internal/config"
	"maly/internal/ipc"
)

// newTestDaemon arranca un demonio real (con mpv) en un entorno XDG aislado.
// Se salta el test si mpv no está instalado.
func newTestDaemon(t *testing.T) *Daemon {
	t.Helper()
	if _, err := exec.LookPath("mpv"); err != nil {
		t.Skip("mpv no está en PATH")
	}
	tmp := t.TempDir()
	// XDG_RUNTIME_DIR corto: el path del socket de mpv no puede pasar de
	// ~108 caracteres (límite de sockets Unix).
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(tmp, "rt"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmp, "data"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "cfg"))
	// mpv sin salida de audio, para que el test no dependa del hardware.
	mpvDir := filepath.Join(tmp, "cfg", "mpv")
	if err := os.MkdirAll(mpvDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mpvDir, "mpv.conf"), []byte("ao=null\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	d, err := New(config.Default())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(d.Close)
	return d
}

// TestScanDoesNotBlockStatus reproduce el bug de 0.3.0: scan corría con d.mu
// tomado y congelaba status (y con él la TUI, que lo pollea cada 500 ms)
// durante todo el escaneo. Ahora status debe responder al instante mientras
// un escaneo grande está en curso, y un segundo scan debe rechazarse.
func TestScanDoesNotBlockStatus(t *testing.T) {
	d := newTestDaemon(t)

	music := t.TempDir()
	const n = 3000
	for i := 0; i < n; i++ {
		sub := filepath.Join(music, fmt.Sprintf("a%02d", i%20))
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		f := filepath.Join(sub, fmt.Sprintf("t%04d.mp3", i))
		if err := os.WriteFile(f, []byte("no es audio"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	scanDone := make(chan ipc.Response, 1)
	go func() { scanDone <- d.Do(ipc.Request{Cmd: "scan", Query: music}) }()

	// Esperar a que el escaneo arranque de verdad.
	for i := 0; i < 200 && !d.scanning.Load(); i++ {
		time.Sleep(time.Millisecond)
	}

	sampled := 0
	busyChecked := false
	for d.scanning.Load() {
		start := time.Now()
		resp := d.Do(ipc.Request{Cmd: "status"})
		elapsed := time.Since(start)
		if !resp.OK {
			t.Fatalf("status durante el escaneo: %s", resp.Error)
		}
		if elapsed > time.Second {
			t.Fatalf("status tardó %v durante el escaneo (¿d.mu tomado por scan?)", elapsed)
		}
		sampled++
		// Un segundo scan simultáneo debe rechazarse, no encolarse ni correr.
		if !busyChecked && d.scanning.Load() {
			if r := d.Do(ipc.Request{Cmd: "scan", Query: music}); r.OK {
				t.Fatal("un scan concurrente debería rechazarse")
			}
			busyChecked = true
		}
	}
	if sampled == 0 {
		t.Log("el escaneo terminó antes de poder muestrear status; sube n si pasa seguido")
	}

	resp := <-scanDone
	if !resp.OK {
		t.Fatalf("scan falló: %s", resp.Error)
	}
	if total, err := d.lib.Count(); err != nil || total != n {
		t.Fatalf("Count = %d, %v; quería %d", total, err, n)
	}
}
