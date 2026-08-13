// Package viz captura el audio que suena (monitor de PipeWire/PulseAudio),
// le aplica FFT y lo agrupa en bandas logarítmicas para el visualizador.
// Si no hay pw-record ni parec, degrada a una animación "fake".
package viz

import (
	"errors"
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

// vizRetryInterval es cada cuánto se reintenta el capturador tras perderlo
// en pleno uso (ver armRetry).
const vizRetryInterval = 15 * time.Second

type Viz struct {
	mu      sync.Mutex
	cmd     *exec.Cmd
	ring    []float64 // últimas fftSize muestras
	fake    bool
	backend string // "pw-record", "parec" o ""
	closed  bool

	// pref es la preferencia de backend del config ([visualizer] backend);
	// se fija una sola vez en New() y no se vuelve a tocar, así que leerla
	// sin lock desde la goroutine de armRetry es seguro.
	pref string
	// hadBackend marca si ALGUNA vez hubo una captura real funcionando:
	// solo entonces vale la pena reintentar tras perderla. En un sistema
	// sin pw-record/parec instalados, reintentar no tiene ningún beneficio
	// y solo gasta un exec.LookPath por vuelta para siempre.
	hadBackend bool
	retrying   bool
	stopRetry  chan struct{} // no nil solo mientras hay un retry en vuelo

	fft     *fourier.FFT
	window  []float64 // ventana de Hann precalculada
	maxSeen float64   // autoganancia con decaimiento lento

	gravity   float64
	bars      []float64
	start     time.Time
	closeOnce sync.Once
}

// New arranca la captura del monitor de audio. Nunca falla: sin backend
// disponible queda en modo fake (Fake() lo reporta para avisar en la UI).
// pref es `[visualizer] backend` del config ("auto"/""/pipewire/pulse); un
// valor no reconocido se comporta como "auto".
func New(gravity float64, pref string) *Viz {
	v := &Viz{
		ring:    make([]float64, fftSize),
		fft:     fourier.NewFFT(fftSize),
		window:  make([]float64, fftSize),
		maxSeen: 1,
		gravity: gravity,
		start:   time.Now(),
		pref:    pref,
	}
	for i := range v.window {
		v.window[i] = 0.5 * (1 - math.Cos(2*math.Pi*float64(i)/float64(fftSize-1)))
	}
	if err := v.startCapture(pref); err != nil {
		v.fake = true
	} else {
		v.hadBackend = true
	}
	return v
}

type candidate struct {
	name string
	args []string
}

// captureCandidates son los capturadores que se prueban, en orden. Viven fuera de
// startCapture para que CaptureBackend pueda mirar la MISMA lista: si aquí se
// agrega un backend, el diagnóstico se entera solo.
var captureCandidates = []candidate{
	// stream.capture.sink=true captura el monitor del sink por defecto.
	{"pw-record", []string{"-P", "{ stream.capture.sink=true }",
		"--format", "s16", "--rate", "44100", "--channels", "1", "-"}},
	{"parec", []string{"--format=s16le", "--rate=44100", "--channels=1",
		"-d", "@DEFAULT_MONITOR@"}},
}

// backendBin traduce el valor de `[visualizer] backend` del config al
// nombre del binario candidato correspondiente.
var backendBin = map[string]string{
	"pipewire": "pw-record",
	"parec":    "parec",
	"pulse":    "parec",
}

// filterCandidates recorta captureCandidates según la preferencia del
// usuario. "auto", vacío o un valor no reconocido devuelven la lista
// completa en el orden de siempre — un valor inválido en el config no debe
// romper el visualizador, se comporta como si no se hubiera pedido nada
// (mismo criterio que un preset de controls inválido cae a "default").
// Función pura a propósito: testeable sin exec.LookPath ni procesos reales.
func filterCandidates(pref string) []candidate {
	bin, ok := backendBin[pref]
	if !ok {
		return captureCandidates
	}
	for _, c := range captureCandidates {
		if c.name == bin {
			return []candidate{c}
		}
	}
	return captureCandidates
}

// CaptureBackend devuelve el capturador que se usaría ("" = ninguno, el
// visualizador caería en modo animación) respetando pref. Solo mira el
// PATH: no arranca nada, que es lo que necesita `maly doctor`.
func CaptureBackend(pref string) string {
	for _, c := range filterCandidates(pref) {
		if _, err := exec.LookPath(c.name); err == nil {
			return c.name
		}
	}
	return ""
}

// errVizClosed marca un startCapture que ganó la carrera contra Close(): el
// proceso arrancó pero se mata en el acto, así que no cuenta como éxito.
var errVizClosed = errors.New("viz: closed")

// startCapture intenta los candidatos que deje pref, leyendo s16le mono
// 44.1kHz. v.cmd/v.backend/v.fake/v.retrying se escriben TODOS bajo el mismo
// v.mu que Close() usa para leerlos, en la misma sección: antes v.cmd y
// v.backend se escribían sin lock, y Close() podía leer el v.cmd VIEJO
// mientras un reintento de armRetry escribía el nuevo — Close() mataba el
// proceso ya muerto y el recién arrancado por el reintento quedaba huérfano
// (carrera real, encontrada en la revisión del propio fix de UX-N4).
// También cierra el hueco de armRetry: si Close() ya corrió (v.closed) para
// cuando este intento consigue arrancar el proceso, se lo mata acá mismo en
// vez de dejarlo vivo sin que nadie vaya a esperarlo.
func (v *Viz) startCapture(pref string) error {
	var lastErr error
	for _, c := range filterCandidates(pref) {
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
		v.mu.Lock()
		if v.closed {
			v.mu.Unlock()
			cmd.Process.Kill()
			go cmd.Wait()
			return errVizClosed
		}
		v.cmd = cmd
		v.backend = c.name
		// fake/retrying se limpian ACÁ, no en el llamador (armRetry): si se
		// limpiaran después de que startCapture devuelva, un readLoop que
		// muriera al instante (backend "aleteando") podía ver retrying
		// todavía en true y no rearmar nada — el arreglo del otro hallazgo
		// de esta misma revisión (TOCTOU de doble-armado).
		v.fake = false
		v.retrying = false
		v.mu.Unlock()
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
				// El proceso de captura murió (PipeWire se reinició, el
				// dispositivo desapareció, lo mataron a mano): degradar a
				// fake y armar el reintento periódico para no quedarse en
				// animación falsa por el resto de la sesión.
				v.mu.Lock()
				v.fake = true
				v.mu.Unlock()
				v.armRetry()
			}
			return
		}
	}
}

// armRetry lanza (si no hay uno ya en vuelo) un reintento periódico de
// startCapture cada vizRetryInterval. Sin esto, perder el capturador en
// pleno uso dejaba el visualizador en modo animación falsa por el resto de
// la sesión, aunque el backend volviera segundos después —p. ej. un
// `systemctl --user restart pipewire`— (hallazgo UX-N4 de la auditoría
// técnica). Solo se arma si hadBackend: en un sistema sin pw-record/parec
// instalados no hay ningún beneficio en reintentar para siempre.
func (v *Viz) armRetry() {
	v.mu.Lock()
	if !v.hadBackend || v.retrying || v.closed {
		v.mu.Unlock()
		return
	}
	v.retrying = true
	stop := make(chan struct{})
	v.stopRetry = stop
	pref := v.pref
	v.mu.Unlock()

	go func() {
		ticker := time.NewTicker(vizRetryInterval)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				// El éxito ya deja fake/retrying en el estado correcto
				// (ver el comentario de startCapture); acá no hace falta
				// tocar nada más que salir.
				if err := v.startCapture(pref); err == nil {
					return
				}
			}
		}
	}()
}

// Fake informa si el visualizador está en modo animación (sin captura real).
func (v *Viz) Fake() bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.fake
}

// Bars devuelve n alturas (0..1) que siguen la amplitud suavizada: suben al
// instante y caen con gravity, estilo CAVA. playing solo se usa en modo fake
// para animar únicamente cuando hay reproducción.
func (v *Viz) Bars(n int, playing bool) []float64 {
	if n <= 0 {
		return nil
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	if len(v.bars) != n {
		v.bars = make([]float64, n)
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
	}
	return append([]float64(nil), v.bars...)
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

// Close mata el proceso de captura y, si había un reintento en vuelo, lo
// corta también (sin esto sobrevivía hasta su próximo tick, hasta
// vizRetryInterval de segundos, en vez de cerrarse en el acto). closeOnce
// la hace idempotente: sin esto, una segunda llamada intentaría cerrar de
// nuevo el mismo canal de stop ya cerrado y entraría en pánico (mismo
// patrón que closeOnce en daemon.Daemon).
func (v *Viz) Close() {
	v.closeOnce.Do(func() {
		v.mu.Lock()
		v.closed = true
		cmd := v.cmd
		stop := v.stopRetry
		v.mu.Unlock()
		if stop != nil {
			close(stop)
		}
		if cmd != nil && cmd.Process != nil {
			cmd.Process.Kill()
		}
	})
}
