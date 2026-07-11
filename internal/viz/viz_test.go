package viz

import (
	"bytes"
	"math"
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
