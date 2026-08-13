package viz

import (
	"bytes"
	"math"
	"os/exec"
	"testing"
	"time"

	"gonum.org/v1/gonum/dsp/fourier"
)

// newTestViz construye un Viz sin arrancar captura (New lanzaría un
// pw-record/parec real en la máquina de desarrollo).
func newTestViz(gravity float64, fake bool) *Viz {
	v := &Viz{
		ring:    make([]float64, fftSize),
		fft:     fourier.NewFFT(fftSize),
		window:  make([]float64, fftSize),
		maxSeen: 1,
		gravity: gravity,
		start:   time.Now(),
		fake:    fake,
	}
	for i := range v.window {
		v.window[i] = 0.5 * (1 - math.Cos(2*math.Pi*float64(i)/float64(fftSize-1)))
	}
	return v
}

// TestFFTBarsDominantBand: un seno puro debe encender exactamente la banda
// logarítmica que contiene su frecuencia.
func TestFFTBarsDominantBand(t *testing.T) {
	v := newTestViz(0.85, false)
	const freq, n = 440.0, 12
	for i := range v.ring {
		v.ring[i] = math.Sin(2 * math.Pi * freq * float64(i) / sampleRate)
	}

	out := v.fftBars(n)
	if len(out) != n {
		t.Fatalf("fftBars devolvió %d bandas", len(out))
	}
	max, argmax := 0.0, -1
	for k, val := range out {
		if val < 0 || val > 1 {
			t.Fatalf("banda %d fuera de rango: %f", k, val)
		}
		if val > max {
			max, argmax = val, k
		}
	}
	// La banda k cubre [fMin·r^(k/n), fMin·r^((k+1)/n)) con r = fMax/fMin.
	want := int(math.Floor(float64(n) * math.Log(freq/fMin) / math.Log(fMax/fMin)))
	if argmax != want {
		t.Errorf("banda dominante para %.0f Hz: %d, quería %d (%v)", freq, argmax, want, out)
	}
	if max < 0.9 {
		t.Errorf("un seno puro debe saturar su banda tras la autoganancia: %f", max)
	}
}

// TestBarsGravity: las barras suben al instante y caen multiplicándose por
// gravity en cada frame (estilo CAVA), sin bajar de cero.
func TestBarsGravity(t *testing.T) {
	v := newTestViz(0.5, true)

	if got := v.Bars(0, true); got != nil {
		t.Errorf("Bars(0) debe ser nil: %v", got)
	}

	// Fake sin reproducción: silencio.
	for _, val := range v.Bars(8, false) {
		if val != 0 {
			t.Fatalf("fake en pausa debe ser silencio: %v", val)
		}
	}

	// Con reproducción las barras suben; al pausar caen a la mitad (gravity
	// 0.5) en cada frame siguiente.
	up := v.Bars(8, true)
	nonzero := false
	for _, val := range up {
		if val > 0 {
			nonzero = true
		}
		if val < 0 || val > 1 {
			t.Fatalf("barra fuera de rango: %f", val)
		}
	}
	if !nonzero {
		t.Fatal("fake reproduciendo debe animar alguna barra")
	}
	down := v.Bars(8, false)
	for i := range down {
		if math.Abs(down[i]-up[i]*0.5) > 1e-9 {
			t.Fatalf("barra %d: %f tras gravity, quería %f", i, down[i], up[i]*0.5)
		}
	}
}

// TestReadLoopDegradesToFake: si la fuente de captura muere (EOF), el
// visualizador pasa a modo fake en vez de congelarse; las muestras que
// alcanzaron a llegar quedan en el ring (s16le → [-1, 1)).
func TestReadLoopDegradesToFake(t *testing.T) {
	v := newTestViz(0.85, false)
	v.readLoop(bytes.NewReader([]byte{0x00, 0x40, 0x00, 0xC0})) // +0.5, -0.5
	if !v.Fake() {
		t.Fatal("EOF de la captura debe degradar a fake")
	}
	ring := v.ring
	if got := [2]float64{ring[len(ring)-2], ring[len(ring)-1]}; got != [2]float64{0.5, -0.5} {
		t.Errorf("últimas muestras del ring: %v", got)
	}
}

// TestReadLoopArmsRetryOnlyWithBackend cubre el hallazgo UX-N4 de la
// auditoría técnica: perder el capturador en pleno uso (PipeWire se
// reinicia, el dispositivo desaparece) degradaba a fake para el resto de
// la sesión, sin ningún reintento. readLoop debe armar el reintento SOLO
// si hadBackend (hubo una captura real funcionando alguna vez) — en un
// sistema sin pw-record/parec instalados, reintentar no aporta nada.
func TestReadLoopArmsRetryOnlyWithBackend(t *testing.T) {
	v := newTestViz(0.85, false)
	v.readLoop(bytes.NewReader(nil)) // EOF inmediato
	if v.retrying {
		t.Fatal("sin hadBackend, readLoop no debía armar el reintento")
	}

	v = newTestViz(0.85, false)
	v.hadBackend = true
	v.readLoop(bytes.NewReader(nil))
	if !v.retrying || v.stopRetry == nil {
		t.Fatal("con hadBackend, readLoop debía armar el reintento")
	}
	v.Close()
}

// TestArmRetryDoesNotDoubleArm: una segunda llamada mientras ya hay un
// reintento en vuelo no debe reemplazarlo (evita fugas de goroutines si el
// evento de muerte se reporta más de una vez).
func TestArmRetryDoesNotDoubleArm(t *testing.T) {
	v := newTestViz(0.85, true)
	v.hadBackend = true
	v.armRetry()
	first := v.stopRetry
	if first == nil {
		t.Fatal("armRetry debía armar el reintento")
	}
	v.armRetry()
	if v.stopRetry != first {
		t.Error("una segunda llamada con un retry ya en vuelo no debía reemplazarlo")
	}
	v.Close()
}

// TestCloseStopsRetryImmediately: sin esto, un retry armado sobrevivía
// hasta su próximo tick (hasta vizRetryInterval completo) en vez de
// cortarse en el acto al cerrar el visualizador.
func TestCloseStopsRetryImmediately(t *testing.T) {
	v := newTestViz(0.85, true)
	v.hadBackend = true
	v.armRetry()
	stop := v.stopRetry
	v.Close()
	select {
	case <-stop:
	default:
		t.Fatal("Close() debía cerrar el canal de stop del retry en el acto")
	}
}

// fakeCaptureCandidate reemplaza captureCandidates por "cat /dev/zero" (un
// stand-in que arranca de verdad y tiene stdout con datos, sin depender de
// pw-record/parec) y devuelve una func para restaurar la lista real.
func fakeCaptureCandidate(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("cat"); err != nil {
		t.Skip("cat no está en PATH")
	}
	orig := captureCandidates
	captureCandidates = []candidate{{"cat", []string{"/dev/zero"}}}
	t.Cleanup(func() { captureCandidates = orig })
}

// TestStartCaptureKilledIfClosedConcurrently cubre la carrera que la
// revisión del propio fix de UX-N4 encontró: v.cmd/v.backend se escribían
// sin lock en startCapture, así que Close() —que SÍ toma el lock para leer
// v.cmd— podía leer el valor VIEJO mientras un reintento de armRetry
// terminaba de escribir el nuevo, matando el proceso equivocado y dejando
// huérfano el recién arrancado. startCapture ahora comprueba v.closed bajo
// el mismo lock que Close() usa, y si ya cerró, mata el proceso que acaba
// de arrancar en el acto en vez de registrarlo.
func TestStartCaptureKilledIfClosedConcurrently(t *testing.T) {
	fakeCaptureCandidate(t)

	v := newTestViz(0.85, false)
	v.closed = true // simula que Close() ya corrió justo antes

	err := v.startCapture("auto")
	if err != errVizClosed {
		t.Fatalf("startCapture con v.closed=true = %v, quería errVizClosed", err)
	}
	if v.cmd != nil {
		t.Error("v.cmd no debía quedar seteado: el proceso se mató antes de registrarlo")
	}
}

// TestStartCaptureResetsFakeAndRetrying cubre el TOCTOU de armRetry que la
// misma revisión encontró: antes, fake/retrying se limpiaban DESPUÉS de que
// startCapture devolviera (en la goroutine de armRetry) — si el readLoop
// recién arrancado moría al instante (backend "aleteando") y llamaba a
// armRetry de nuevo antes de esa limpieza, la veía todavía en true y no
// rearmaba nada, dejando fake en false para siempre sin captura real ni
// reintento. Ahora startCapture limpia fake/retrying atómicamente, bajo el
// mismo lock que registra v.cmd — no hay ventana entre "el proceso ya
// corre" y "el estado ya lo refleja".
func TestStartCaptureResetsFakeAndRetrying(t *testing.T) {
	fakeCaptureCandidate(t)

	v := newTestViz(0.85, true)
	v.retrying = true
	if err := v.startCapture("auto"); err != nil {
		t.Fatalf("startCapture: %v", err)
	}
	defer v.Close()

	if v.Fake() {
		t.Error("startCapture exitoso debía limpiar fake")
	}
	v.mu.Lock()
	retrying := v.retrying
	v.mu.Unlock()
	if retrying {
		t.Error("startCapture exitoso debía limpiar retrying")
	}
}

// TestCloseIsIdempotent cubre el hallazgo de la revisión: una segunda
// llamada a Close() con un retry en vuelo intentaba cerrar el mismo canal
// stopRetry dos veces y entraba en pánico (close of closed channel).
func TestCloseIsIdempotent(t *testing.T) {
	v := newTestViz(0.85, true)
	v.hadBackend = true
	v.armRetry()
	if v.stopRetry == nil {
		t.Fatal("setup: armRetry debía armar el reintento")
	}
	v.Close()
	v.Close() // no debe entrar en pánico
}

// TestFilterCandidates: "auto"/vacío/un valor no reconocido dejan la lista
// completa (en el orden de siempre); "pipewire"/"pulse" acotan a un solo
// candidato. Función pura, sin exec.LookPath ni procesos reales.
func TestFilterCandidates(t *testing.T) {
	full := filterCandidates("auto")
	if len(full) != len(captureCandidates) {
		t.Fatalf(`filterCandidates("auto") = %d candidatos, quería %d`, len(full), len(captureCandidates))
	}

	empty := filterCandidates("")
	if len(empty) != len(captureCandidates) {
		t.Fatalf(`filterCandidates("") = %d candidatos, quería la lista completa`, len(empty))
	}

	bogus := filterCandidates("bogus")
	if len(bogus) != len(captureCandidates) {
		t.Fatalf(`filterCandidates("bogus") = %d candidatos, un valor no reconocido debía comportarse como "auto"`, len(bogus))
	}

	pw := filterCandidates("pipewire")
	if len(pw) != 1 || pw[0].name != "pw-record" {
		t.Fatalf(`filterCandidates("pipewire") = %v, quería solo pw-record`, pw)
	}

	pulse := filterCandidates("pulse")
	if len(pulse) != 1 || pulse[0].name != "parec" {
		t.Fatalf(`filterCandidates("pulse") = %v, quería solo parec`, pulse)
	}
}
