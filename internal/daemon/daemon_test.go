package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"maly/internal/config"
	"maly/internal/ipc"
)

// testEnv prepara un entorno XDG aislado para demonios de prueba. Se salta
// el test si mpv no está instalado.
func testEnv(t *testing.T) {
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
}

// newTestDaemon arranca un demonio real (con mpv) en un entorno XDG aislado.
func newTestDaemon(t *testing.T) *Daemon {
	t.Helper()
	testEnv(t)
	d, err := New(config.Default())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(d.Close)
	return d
}

// writeWAV fabrica un WAV PCM de silencio (8 kHz, mono, 8 bits) que mpv
// puede cargar y en el que puede hacer seek de verdad.
func writeWAV(t *testing.T, path string, seconds int) {
	t.Helper()
	const rate = 8000
	n := rate * seconds
	le16 := func(v int) []byte { return []byte{byte(v), byte(v >> 8)} }
	le32 := func(v int) []byte {
		return []byte{byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24)}
	}
	buf := make([]byte, 0, 44+n)
	buf = append(buf, "RIFF"...)
	buf = append(buf, le32(36+n)...)
	buf = append(buf, "WAVEfmt "...)
	buf = append(buf, le32(16)...)   // tamaño del bloque fmt
	buf = append(buf, le16(1)...)    // PCM
	buf = append(buf, le16(1)...)    // mono
	buf = append(buf, le32(rate)...) // sample rate
	buf = append(buf, le32(rate)...) // byte rate (8 bits, mono)
	buf = append(buf, le16(1)...)    // block align
	buf = append(buf, le16(8)...)    // bits por muestra
	buf = append(buf, "data"...)
	buf = append(buf, le32(n)...)
	silence := make([]byte, n)
	for i := range silence {
		silence[i] = 0x80 // el cero de PCM sin signo
	}
	buf = append(buf, silence...)
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatal(err)
	}
}

// waitStatus pollea el estado del demonio hasta que ok lo acepte.
func waitStatus(t *testing.T, d *Daemon, what string, ok func(*ipc.Status) bool) *ipc.Status {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	var st *ipc.Status
	for time.Now().Before(deadline) {
		st = d.Do(ipc.Request{Cmd: "status"}).Status
		if st != nil && ok(st) {
			return st
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("esperando %s; último estado: %+v", what, st)
	return nil
}

// next lee el siguiente push con timeout, para que un fallo no cuelgue el test.
func next(t *testing.T, c *ipc.Client) ipc.Response {
	t.Helper()
	type result struct {
		resp ipc.Response
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		resp, err := c.Next()
		ch <- result{resp, err}
	}()
	select {
	case r := <-ch:
		if r.err != nil {
			t.Fatalf("leyendo push: %v", r.err)
		}
		return r.resp
	case <-time.After(5 * time.Second):
		t.Fatal("ningún push en 5 s")
		return ipc.Response{}
	}
}

// TestSubscribe verifica el modo push: la suscripción responde el estado
// inicial y cada comando mutador genera un push sin que el cliente pollee;
// que un suscriptor cuelgue no afecta al demonio.
func TestSubscribe(t *testing.T) {
	d := newTestDaemon(t)
	go d.Run()

	sub, err := ipc.Dial(config.SocketPath())
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()
	first, err := sub.Subscribe()
	if err != nil || !first.OK {
		t.Fatalf("subscribe: %v / %+v", err, first)
	}
	if first.Status == nil {
		t.Fatal("el estado inicial no trae Status")
	}

	// waitVolume lee pushes hasta ver el volumen esperado: los pushes son
	// fotos de estado (no eventos) y los de arranque de mpv o una ráfaga
	// coalescida pueden intercalarse antes del que interesa.
	waitVolume := func(want int) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		var got ipc.Response
		for {
			got = next(t, sub)
			if got.Status != nil && got.Status.Volume == want {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("nunca llegó el push con vol %d; último: %+v", want, got.Status)
			}
		}
	}

	// Un comando mutador por otra conexión debe llegar como push.
	if resp := d.Do(ipc.Request{Cmd: "vol", Value: "37"}); !resp.OK {
		t.Fatalf("vol: %s", resp.Error)
	}
	waitVolume(37)

	// Ráfaga: el canal dirty cap-1 y el intervalo mínimo colapsan pushes,
	// pero el último debe reflejar el estado final.
	for _, v := range []string{"40", "50", "60"} {
		if resp := d.Do(ipc.Request{Cmd: "vol", Value: v}); !resp.OK {
			t.Fatalf("vol %s: %s", v, resp.Error)
		}
	}
	waitVolume(60)

	// Colgar el suscriptor no debe tumbar nada: el demonio sigue vivo y se
	// puede volver a suscribir.
	sub.Close()
	if resp := d.Do(ipc.Request{Cmd: "vol", Value: "70"}); !resp.OK {
		t.Fatalf("vol tras colgar: %s", resp.Error)
	}
	sub2, err := ipc.Dial(config.SocketPath())
	if err != nil {
		t.Fatal(err)
	}
	defer sub2.Close()
	first2, err := sub2.Subscribe()
	if err != nil || !first2.OK || first2.Status == nil || first2.Status.Volume != 70 {
		t.Fatalf("resuscripción: %v / %+v", err, first2.Status)
	}

	// El suscriptor muerto debe haberse desregistrado (queda solo sub2).
	ok := false
	for i := 0; i < 100; i++ {
		d.subMu.Lock()
		n := len(d.subs)
		d.subMu.Unlock()
		if n == 1 {
			ok = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !ok {
		t.Fatal("el suscriptor colgado no se desregistró")
	}
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

// TestSessionPersistence es el round-trip completo: un demonio reproduce,
// se cierra, y el siguiente arranca con la cola, el volumen, los modos y la
// pista actual en pausa en la posición guardada.
func TestSessionPersistence(t *testing.T) {
	testEnv(t)
	music := t.TempDir()
	a := filepath.Join(music, "a.wav")
	b := filepath.Join(music, "b.wav")
	writeWAV(t, a, 30)
	writeWAV(t, b, 30)

	d1, err := New(config.Default())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(d1.Close) // red de seguridad; Close es idempotente

	for _, r := range []ipc.Request{
		{Cmd: "add", Query: a}, // cola vacía: encola y empieza a sonar
		{Cmd: "add", Query: b},
		{Cmd: "vol", Value: "55"},
		{Cmd: "shuffle", Value: "on"},
		{Cmd: "repeat", Value: "all"},
	} {
		if resp := d1.Do(r); !resp.OK {
			t.Fatalf("%s: %s", r.Cmd, resp.Error)
		}
	}
	waitStatus(t, d1, "que a.wav cargue", func(st *ipc.Status) bool {
		return st.Playing && st.Duration > 0
	})
	if resp := d1.Do(ipc.Request{Cmd: "seek", Value: "5"}); !resp.OK {
		t.Fatalf("seek: %s", resp.Error)
	}
	waitStatus(t, d1, "posición ~5", func(st *ipc.Status) bool {
		return st.Position >= 4.5
	})
	d1.Close()

	// El guardado final debe reflejarlo todo.
	data, err := os.ReadFile(sessionPath())
	if err != nil {
		t.Fatalf("leyendo session.json: %v", err)
	}
	var s session
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatalf("session.json inválido: %v", err)
	}
	if s.V != sessionVersion || len(s.Queue) != 2 || s.Index != 0 || !s.Playing ||
		s.Position < 4.5 || s.Volume != 55 || !s.Shuffle || s.Repeat != "all" {
		t.Fatalf("sesión guardada: %+v", s)
	}

	d2, err := New(config.Default())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(d2.Close)
	st := waitStatus(t, d2, "sesión restaurada", func(st *ipc.Status) bool {
		return st.Playing && st.Paused && st.Position >= 4 && st.Position < 7
	})
	if st.QueueLen != 2 || st.QueueIndex != 0 || st.Volume != 55 ||
		!st.Shuffle || st.Repeat != "all" {
		t.Fatalf("estado restaurado: %+v", st)
	}
	if st.Track == nil || st.Track.Path != a {
		t.Fatalf("pista restaurada: %+v", st.Track)
	}
}

// TestSessionMissingFiles: los archivos desaparecidos se saltan; si la pista
// actual ya no existe, no se adivina otra y nada queda cargado.
func TestSessionMissingFiles(t *testing.T) {
	testEnv(t)
	music := t.TempDir()
	a := filepath.Join(music, "a.wav")
	b := filepath.Join(music, "b.wav")
	writeWAV(t, a, 10)
	writeWAV(t, b, 10)
	gone := filepath.Join(music, "borrada.wav")

	if err := os.MkdirAll(config.DataDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	// La actual (índice 1) es la borrada: debe arrancar detenido con la
	// cola limpia de huecos.
	s := session{V: sessionVersion, Queue: []string{a, gone, b}, Index: 1,
		Playing: true, Position: 3, Volume: 80}
	data, _ := json.Marshal(s)
	if err := os.WriteFile(sessionPath(), data, 0o644); err != nil {
		t.Fatal(err)
	}

	d, err := New(config.Default())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(d.Close)
	st := d.Do(ipc.Request{Cmd: "status"}).Status
	if st.QueueLen != 2 || st.QueueIndex != -1 || st.Playing {
		t.Fatalf("estado tras pista actual desaparecida: %+v", st)
	}

	// Ahora la borrada va ANTES de la actual (índice 2 = b): el índice se
	// corre y b debe quedar cargada en pausa.
	d.Close()
	s = session{V: sessionVersion, Queue: []string{a, gone, b}, Index: 2,
		Playing: true, Volume: 80}
	data, _ = json.Marshal(s)
	if err := os.WriteFile(sessionPath(), data, 0o644); err != nil {
		t.Fatal(err)
	}
	d2, err := New(config.Default())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(d2.Close)
	st = waitStatus(t, d2, "b restaurada en pausa", func(st *ipc.Status) bool {
		return st.Playing && st.Paused
	})
	if st.QueueLen != 2 || st.QueueIndex != 1 || st.Track == nil || st.Track.Path != b {
		t.Fatalf("estado tras hueco anterior: %+v", st)
	}
}

// TestSessionCorruptStartsClean: un session.json roto no impide arrancar.
func TestSessionCorruptStartsClean(t *testing.T) {
	testEnv(t)
	if err := os.MkdirAll(config.DataDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sessionPath(), []byte("{esto no es json"), 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := New(config.Default())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(d.Close)
	st := d.Do(ipc.Request{Cmd: "status"}).Status
	if st.QueueLen != 0 || st.Playing {
		t.Fatalf("con sesión corrupta el arranque debe ser limpio: %+v", st)
	}
}
