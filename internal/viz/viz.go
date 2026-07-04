// Package viz captura el audio que suena (monitor de PipeWire/PulseAudio),
// le aplica FFT y lo agrupa en bandas logarítmicas para el visualizador.
// Si no hay pw-record ni parec, degrada a una animación "fake".
package viz

import (
	"io"
	"math"
	"os/exec"
	"sync"
	"time"

	"gonum.org/v1/gonum/dsp/fourier"
)

const (
	sampleRate = 44100
	fftSize    = 2048
	fMin       = 35.0
	fMax       = 15000.0
)

type Viz struct {
	mu      sync.Mutex
	cmd     *exec.Cmd
	ring    []float64 // últimas fftSize muestras
	fake    bool
	backend string // "pw-record", "parec" o ""
	closed  bool

	fft     *fourier.FFT
	window  []float64 // ventana de Hann precalculada
	maxSeen float64   // autoganancia con decaimiento lento

	gravity float64
	bars    []float64
	peaks   []float64
	start   time.Time
}

// New arranca la captura del monitor de audio. Nunca falla: sin backend
// disponible queda en modo fake (Fake() lo reporta para avisar en la UI).
func New(gravity float64) *Viz {
	v := &Viz{
		ring:    make([]float64, fftSize),
		fft:     fourier.NewFFT(fftSize),
		window:  make([]float64, fftSize),
		maxSeen: 1,
		gravity: gravity,
		start:   time.Now(),
	}
	for i := range v.window {
		v.window[i] = 0.5 * (1 - math.Cos(2*math.Pi*float64(i)/float64(fftSize-1)))
	}
	if err := v.startCapture(); err != nil {
		v.fake = true
	}
	return v
}

// startCapture intenta pw-record y luego parec, leyendo s16le mono 44.1kHz.
func (v *Viz) startCapture() error {
	type candidate struct {
		name string
		args []string
	}
	candidates := []candidate{
		// stream.capture.sink=true captura el monitor del sink por defecto.
		{"pw-record", []string{"-P", "{ stream.capture.sink=true }",
			"--format", "s16", "--rate", "44100", "--channels", "1", "-"}},
		{"parec", []string{"--format=s16le", "--rate=44100", "--channels=1",
			"-d", "@DEFAULT_MONITOR@"}},
	}
	var lastErr error
	for _, c := range candidates {
		bin, err := exec.LookPath(c.name)
		if err != nil {
			lastErr = err
			continue
		}
		cmd := exec.Command(bin, c.args...)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			lastErr = err
			continue
		}
		if err := cmd.Start(); err != nil {
			lastErr = err
			continue
		}
		v.cmd = cmd
		v.backend = c.name
		go v.readLoop(stdout)
		go cmd.Wait()
		return nil
	}
	return lastErr
}

// readLoop mete el PCM crudo en el ring de muestras.
func (v *Viz) readLoop(r io.Reader) {
	buf := make([]byte, 4096)
	for {
		n, err := io.ReadFull(r, buf)
		if n > 0 {
			samples := make([]float64, n/2)
			for i := 0; i < n-1; i += 2 {
				s := int16(uint16(buf[i]) | uint16(buf[i+1])<<8)
				samples[i/2] = float64(s) / 32768
			}
			v.mu.Lock()
			v.ring = append(v.ring[len(samples):], samples...)
			v.mu.Unlock()
		}
		if err != nil {
			v.mu.Lock()
			closed := v.closed
			v.mu.Unlock()
			if !closed {
				// El proceso de captura murió: degradar a fake.
				v.mu.Lock()
				v.fake = true
				v.mu.Unlock()
			}
			return
		}
	}
}

// Fake informa si el visualizador está en modo animación (sin captura real).
func (v *Viz) Fake() bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.fake
}

// Bars devuelve n alturas (0..1) suavizadas y sus picos. playing solo se usa
// en modo fake para animar únicamente cuando hay reproducción.
func (v *Viz) Bars(n int, playing bool) (bars, peaks []float64) {
	if n <= 0 {
		return nil, nil
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	if len(v.bars) != n {
		v.bars = make([]float64, n)
		v.peaks = make([]float64, n)
	}

	var raw []float64
	if v.fake {
		raw = v.fakeBars(n, playing)
	} else {
		raw = v.fftBars(n)
	}
	for i := 0; i < n; i++ {
		if raw[i] > v.bars[i] {
			v.bars[i] = raw[i]
		} else {
			v.bars[i] *= v.gravity
		}
		v.peaks[i] -= 0.015
		if v.bars[i] > v.peaks[i] {
			v.peaks[i] = v.bars[i]
		}
		if v.peaks[i] < 0 {
			v.peaks[i] = 0
		}
	}
	return append([]float64(nil), v.bars...), append([]float64(nil), v.peaks...)
}

func (v *Viz) fftBars(n int) []float64 {
	src := make([]float64, fftSize)
	for i, s := range v.ring {
		src[i] = s * v.window[i]
	}
	coeffs := v.fft.Coefficients(nil, src)

	binHz := float64(sampleRate) / float64(fftSize)
	out := make([]float64, n)
	frameMax := 0.0
	for k := 0; k < n; k++ {
		f0 := fMin * math.Pow(fMax/fMin, float64(k)/float64(n))
		f1 := fMin * math.Pow(fMax/fMin, float64(k+1)/float64(n))
		b0 := int(f0 / binHz)
		b1 := int(f1 / binHz)
		if b1 <= b0 {
			b1 = b0 + 1
		}
		if b1 > len(coeffs) {
			b1 = len(coeffs)
		}
		mag := 0.0
		for b := b0; b < b1 && b < len(coeffs); b++ {
			m := math.Hypot(real(coeffs[b]), imag(coeffs[b]))
			if m > mag {
				mag = m
			}
		}
		out[k] = mag
		if mag > frameMax {
			frameMax = mag
		}
	}
	// Autoganancia: normalizar contra un máximo que decae despacio.
	v.maxSeen *= 0.999
	if frameMax > v.maxSeen {
		v.maxSeen = frameMax
	}
	if v.maxSeen < 1 {
		v.maxSeen = 1
	}
	for k := range out {
		val := out[k] / v.maxSeen
		out[k] = math.Pow(val, 0.55) // curva perceptual
	}
	return out
}

// fakeBars anima ondas suaves cuando hay reproducción activa.
func (v *Viz) fakeBars(n int, playing bool) []float64 {
	out := make([]float64, n)
	if !playing {
		return out
	}
	t := time.Since(v.start).Seconds()
	for i := 0; i < n; i++ {
		x := float64(i)
		a := 0.45 + 0.35*math.Sin(t*2.1+x*0.35)
		b := 0.55 + 0.45*math.Sin(t*0.83+x*0.13+1.7)
		c := 0.2 * math.Sin(t*5.3+x*0.9)
		val := a*b + c
		if val < 0.02 {
			val = 0.02
		}
		if val > 1 {
			val = 1
		}
		out[i] = val
	}
	return out
}

// Close mata el proceso de captura.
func (v *Viz) Close() {
	v.mu.Lock()
	v.closed = true
	cmd := v.cmd
	v.mu.Unlock()
	if cmd != nil && cmd.Process != nil {
		cmd.Process.Kill()
	}
}
