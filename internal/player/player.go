// Package player lanza y supervisa un proceso mpv (--idle --no-video) y lo
// controla por su IPC JSON sobre socket Unix.
package player

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"maly/internal/i18n"
	"time"
)

// State es el estado observado de mpv, actualizado por eventos.
type State struct {
	Path     string
	Paused   bool
	Position float64
	Duration float64
	Volume   float64
	Idle     bool // sin archivo cargado
}

// Player es un wrapper sobre un proceso mpv propio.
type Player struct {
	mu       sync.Mutex
	cmd      *exec.Cmd
	conn     net.Conn
	reqID    int64
	pending  map[int64]chan mpvReply
	state    State
	onEnd    func(reason, next string) // pista terminada; next = entrada anexada que mpv encadena
	onChange func()                    // cambio de estado observado (pausa, pista, posición…)
	closed   bool
	done     chan struct{}
	exited   chan struct{} // cerrado cuando el proceso mpv termina

	// Espejo de la entrada anexada por SetNext (ventana gapless), para no
	// re-consultar la playlist de mpv cuando no cambió nada. nextKnown se
	// apaga cuando la playlist cambia por fuera (replace, stop, avance).
	nextPath  string
	nextKnown bool

	// Un end-file (eof|error) queda pendiente hasta el evento que revela su
	// desenlace: start-file (mpv encadenó a la anexada) o idle (no había
	// nada que encadenar). Resolverlo al leer el end-file adivinaría: con
	// archivos que fallan al instante, mpv decide antes de que un append en
	// vuelo le llegue, y el espejo leído después miente. pendingGen congela
	// la generación de cargas: si un loadfile replace propio se cruza, el
	// desenlace ya no es de mpv y el evento pendiente se descarta.
	pendingEnd string
	pendingGen int64
	loadGen    int64

	loads int64 // loadfile replace emitidos; diagnóstico de gapless
}

type mpvReply struct {
	Err  string          `json:"error"`
	Data json.RawMessage `json:"data"`
}

type mpvEvent struct {
	Event     string          `json:"event"`
	Name      string          `json:"name"`
	Data      json.RawMessage `json:"data"`
	Reason    string          `json:"reason"`
	RequestID int64           `json:"request_id"`
	Err       string          `json:"error"`
}

// Start lanza mpv y conecta con su socket IPC. onEnd se invoca cuando una
// pista termina sin intervención del demonio: reason "eof" si acabó por sí
// sola, "error" si mpv no pudo reproducirla; next es la ruta que SetNext
// tenía anexada en ese momento ("" = ninguna) — mpv encadena a ella solo,
// así que el demonio reconcilia sin consultar (justo tras el end-file mpv
// está entre entradas y su propiedad path aún no existe). onChange se
// invoca ante cambios de estado observados (lo usa el demonio para MPRIS).
func Start(socketPath string, onEnd func(reason, next string), onChange func()) (*Player, error) {
	mpvBin, err := exec.LookPath("mpv")
	if err != nil {
		return nil, errors.New(i18n.T("p.no_mpv"))
	}
	// Asegurar el directorio del socket (0700) y limpiar un socket huérfano de
	// un mpv que murió sin borrarlo.
	if err := os.MkdirAll(filepath.Dir(socketPath), 0o700); err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Tf("lib.mkdir", filepath.Dir(socketPath)), err)
	}
	os.Remove(socketPath)

	// --input-terminal=no (en vez de --no-terminal) evita que mpv toque la
	// terminal de la TUI pero deja que escriba a stdout el motivo de una
	// muerte temprana (opción inválida, etc.); lo capturamos acotado para
	// diagnosticar. Idle, mpv no escribe nada, así que en el caso sano el
	// buffer queda vacío.
	cmd := exec.Command(mpvBin,
		"--idle=yes", "--no-video", "--input-terminal=no",
		"--audio-display=no",
		"--input-ipc-server="+socketPath,
		"--volume=100",
	)
	var out boundedBuffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.T("p.launch"), err)
	}
	exited := make(chan struct{})
	go func() { cmd.Wait(); close(exited) }() // evitar zombi y detectar muerte

	// mpv tarda en crear el socket; en hardware lento puede pasar de un par de
	// segundos. Reintentar hasta ~5 s, distinguiendo "mpv murió" (el socket no
	// va a aparecer) de "aún no está listo".
	var conn net.Conn
	deadline := time.Now().Add(5 * time.Second)
	for {
		if conn, err = net.Dial("unix", socketPath); err == nil {
			break
		}
		select {
		case <-exited:
			return nil, mpvStartError(cmd, out.String())
		default:
		}
		if time.Now().After(deadline) {
			cmd.Process.Kill()
			return nil, fmt.Errorf("%s: %w", i18n.T("p.connect"), err)
		}
		time.Sleep(50 * time.Millisecond)
	}

	p := &Player{
		cmd:      cmd,
		conn:     conn,
		pending:  map[int64]chan mpvReply{},
		state:    State{Idle: true, Volume: 100},
		onEnd:    onEnd,
		onChange: onChange,
		done:     make(chan struct{}),
		exited:   exited,
	}
	go p.readLoop()

	for i, prop := range []string{"pause", "time-pos", "duration", "volume", "idle-active", "path"} {
		if _, err := p.command("observe_property", int64(i+1), prop); err != nil {
			p.Close()
			return nil, fmt.Errorf("%s: %w", i18n.T("p.configure"), err)
		}
	}
	return p, nil
}

func (p *Player) readLoop() {
	sc := bufio.NewScanner(p.conn)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		var ev mpvEvent
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
			continue
		}
		if ev.Event == "" && ev.RequestID != 0 {
			p.mu.Lock()
			ch := p.pending[ev.RequestID]
			delete(p.pending, ev.RequestID)
			p.mu.Unlock()
			if ch != nil {
				ch <- mpvReply{Err: ev.Err, Data: ev.Data}
			}
			continue
		}
		p.handleEvent(ev)
	}
	close(p.done)
}

func (p *Player) handleEvent(ev mpvEvent) {
	switch ev.Event {
	case "property-change":
		p.mu.Lock()
		changed := false
		switch ev.Name {
		case "pause":
			old := p.state.Paused
			json.Unmarshal(ev.Data, &p.state.Paused)
			changed = p.state.Paused != old
		case "time-pos":
			var v *float64
			json.Unmarshal(ev.Data, &v)
			if v != nil {
				changed = p.state.Position != *v
				p.state.Position = *v
			}
		case "duration":
			var v *float64
			json.Unmarshal(ev.Data, &v)
			if v != nil {
				changed = p.state.Duration != *v
				p.state.Duration = *v
			}
		case "volume":
			old := p.state.Volume
			json.Unmarshal(ev.Data, &p.state.Volume)
			changed = p.state.Volume != old
		case "idle-active":
			old := p.state.Idle
			json.Unmarshal(ev.Data, &p.state.Idle)
			changed = p.state.Idle != old
		case "path":
			var s *string
			json.Unmarshal(ev.Data, &s)
			old := p.state.Path
			if s == nil {
				p.state.Path = ""
			} else {
				p.state.Path = *s
			}
			changed = p.state.Path != old
		}
		p.mu.Unlock()
		// Async como onEnd: en línea bloquearía readLoop, y con él las
		// respuestas de mpv que el demonio pueda estar esperando.
		if changed && p.onChange != nil {
			go p.onChange()
		}
	case "end-file":
		p.mu.Lock()
		p.nextKnown = false // la playlist de mpv cambió: espejo no confiable
		// "stop" (Stop propio o loadfile replace) y "quit" no son fin de
		// pista: solo el eof natural y el fallo de reproducción avanzan.
		if ev.Reason == "eof" || ev.Reason == "error" {
			p.pendingEnd = ev.Reason
			p.pendingGen = p.loadGen
		}
		p.mu.Unlock()

	case "start-file":
		// Desenlace de un end-file pendiente: mpv arrancó otra entrada por
		// su cuenta — la anexada por SetNext (nextPath sigue vigente: el
		// espejo se invalida pero el valor no se pisa hasta otro SetNext).
		if reason, next, ok := p.resolveEnd(); ok {
			go p.onEnd(reason, next)
		}

	case "idle":
		// Desenlace contrario: no había nada que encadenar.
		if reason, _, ok := p.resolveEnd(); ok {
			go p.onEnd(reason, "")
		}
	}
}

// command envía un comando a mpv y espera su respuesta.
func (p *Player) command(args ...any) (json.RawMessage, error) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil, errors.New(i18n.T("p.not_running"))
	}
	p.reqID++
	id := p.reqID
	ch := make(chan mpvReply, 1)
	p.pending[id] = ch
	msg, _ := json.Marshal(map[string]any{"command": args, "request_id": id})
	_, err := p.conn.Write(append(msg, '\n'))
	p.mu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.T("p.write"), err)
	}
	select {
	case r := <-ch:
		if r.Err != "success" {
			return nil, fmt.Errorf("mpv: %s", r.Err)
		}
		return r.Data, nil
	case <-time.After(5 * time.Second):
		// Retirar la entrada abandonada: un mpv que nunca contesta iría
		// acumulando canales en pending para siempre. Si la respuesta llega
		// tarde igual no bloquea a readLoop (canal con capacidad 1).
		p.mu.Lock()
		delete(p.pending, id)
		p.mu.Unlock()
		return nil, errors.New(i18n.T("p.no_reply"))
	case <-p.done:
		return nil, errors.New(i18n.T("p.exited"))
	}
}

// Load carga y reproduce un archivo (reemplaza lo que suene; en mpv,
// loadfile replace limpia la playlist entera, incluida una anexada).
func (p *Player) Load(path string) error {
	p.mu.Lock()
	p.nextKnown = false
	p.loadGen++ // los desenlaces que siguen son de esta carga
	p.loads++
	p.mu.Unlock()
	if _, err := p.command("loadfile", path, "replace"); err != nil {
		return err
	}
	_, err := p.command("set_property", "pause", false)
	return err
}

// LoadPaused carga un archivo dejándolo en pausa (la pausa va antes del
// loadfile para que no llegue a sonar ni un instante). Lo usa el demonio al
// restaurar la sesión: nunca debe arrancar sonando solo.
func (p *Player) LoadPaused(path string) error {
	p.mu.Lock()
	p.nextKnown = false
	p.loadGen++
	p.loads++
	p.mu.Unlock()
	if err := p.SetPause(true); err != nil {
		return err
	}
	_, err := p.command("loadfile", path, "replace")
	return err
}

// resolveEnd consume el end-file pendiente y devuelve su reason y la
// entrada anexada a la que mpv encadena. ok=false si no había pendiente, si
// un loadfile replace propio se cruzó (el desenlace es de esa carga, no del
// fin de pista) o si no hay callback.
func (p *Player) resolveEnd() (reason, next string, ok bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	reason = p.pendingEnd
	p.pendingEnd = ""
	if reason == "" || p.pendingGen != p.loadGen || p.onEnd == nil {
		return "", "", false
	}
	return reason, p.nextPath, true
}

// SetNext deja la playlist interna de mpv como ventana de dos entradas —
// [actual, path], o solo [actual] con path vacío. Es el corazón del
// gapless: al terminar la actual, mpv encadena a la anexada sin cortar el
// audio.
//
// Usa playlist-clear (conserva la entrada actual) a propósito: es la única
// operación segura en plena transición de pista. Las propiedades
// playlist-pos/path van REZAGADAS justo tras un end-file (pos aún apunta a
// la entrada vieja cuando la nueva ya suena), así que podar por índices
// aquí puede quitar exactamente la entrada que mpv está encadenando —
// verificado contra mpv real.
func (p *Player) SetNext(path string) error {
	p.mu.Lock()
	if p.nextKnown && p.nextPath == path {
		p.mu.Unlock()
		return nil
	}
	p.mu.Unlock()

	if _, err := p.command("playlist-clear"); err != nil {
		return err
	}
	appended := ""
	if path != "" {
		// Con mpv idle esto deja una entrada huérfana que no suena sola
		// (append no arranca reproducción) y que cualquier loadfile replace
		// posterior se lleva; inofensiva.
		if _, err := p.command("loadfile", path, "append"); err != nil {
			return err
		}
		appended = path
	}
	p.mu.Lock()
	p.nextPath = appended
	p.nextKnown = true
	p.mu.Unlock()
	return nil
}

// CurrentPath consulta a mpv la ruta cargada en este instante ("" si está
// idle o la consulta falla). Es la verdad viva: State().Path puede ir por
// detrás de los eventos justo después de un cambio de pista.
func (p *Player) CurrentPath() string {
	data, err := p.command("get_property", "path")
	if err != nil {
		return ""
	}
	var s string
	if json.Unmarshal(data, &s) != nil {
		return ""
	}
	return s
}

// PlaylistCount devuelve cuántas entradas tiene la playlist interna de mpv
// (la ventana gapless: 1 sin promesa anexada, 2 con ella).
func (p *Player) PlaylistCount() (int, error) { return p.intProp("playlist-count") }

// LoadCount devuelve cuántos loadfile replace se han emitido. Un cambio de
// pista que NO lo incrementa fue encadenado por mpv (gapless de verdad).
func (p *Player) LoadCount() int64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.loads
}

func (p *Player) intProp(name string) (int, error) {
	data, err := p.command("get_property", name)
	if err != nil {
		return 0, err
	}
	var v int
	if err := json.Unmarshal(data, &v); err != nil {
		return 0, err
	}
	return v, nil
}

// SetPause pone o quita pausa.
func (p *Player) SetPause(paused bool) error {
	if _, err := p.command("set_property", "pause", paused); err != nil {
		return err
	}
	p.mu.Lock()
	p.state.Paused = paused
	p.mu.Unlock()
	return nil
}

// Toggle alterna pausa y sincroniza el estado antes de volver, para que la
// respuesta al cliente ya refleje el cambio.
func (p *Player) Toggle() error {
	if _, err := p.command("cycle", "pause"); err != nil {
		return err
	}
	if data, err := p.command("get_property", "pause"); err == nil {
		var b bool
		if json.Unmarshal(data, &b) == nil {
			p.mu.Lock()
			p.state.Paused = b
			p.mu.Unlock()
		}
	}
	return nil
}

// Stop detiene y descarga la pista actual (en mpv, stop además vacía la
// playlist: se lleva también una entrada anexada por SetNext).
func (p *Player) Stop() error {
	p.mu.Lock()
	p.nextKnown = false
	p.loadGen++ // el idle que sigue es de este stop, no de un fin de pista
	p.mu.Unlock()
	_, err := p.command("stop")
	return err
}

// SeekRel salta sec segundos (negativo retrocede).
func (p *Player) SeekRel(sec float64) error {
	return p.seek("seek", sec, "relative")
}

// SeekAbs salta a la posición sec.
func (p *Player) SeekAbs(sec float64) error {
	return p.seek("seek", sec, "absolute")
}

// seek reintenta una vez: mpv rechaza seeks mientras el archivo aún carga
// (p. ej. un seek inmediatamente después de next). Luego refresca la posición
// para que el estado devuelto ya sea el nuevo.
func (p *Player) seek(args ...any) error {
	_, err := p.command(args...)
	if err != nil {
		time.Sleep(250 * time.Millisecond)
		if _, err = p.command(args...); err != nil {
			return fmt.Errorf("%s: %w", i18n.T("p.seek"), err)
		}
	}
	if data, err := p.command("get_property", "time-pos"); err == nil {
		var v float64
		if json.Unmarshal(data, &v) == nil {
			p.mu.Lock()
			p.state.Position = v
			p.mu.Unlock()
		}
	}
	return nil
}

// SetVolume fija el volumen (0-100).
func (p *Player) SetVolume(v float64) error {
	if v < 0 {
		v = 0
	}
	if v > 100 {
		v = 100
	}
	_, err := p.command("set_property", "volume", v)
	return err
}

// State devuelve una copia del estado observado.
func (p *Player) State() State {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.state
}

// Close pide a mpv que salga y lo mata si no obedece.
func (p *Player) Close() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true
	msg, _ := json.Marshal(map[string]any{"command": []any{"quit"}})
	p.conn.Write(append(msg, '\n'))
	p.conn.Close()
	p.mu.Unlock()

	// Esperar a que mpv obedezca el quit; si no, matarlo. No se lee
	// cmd.ProcessState directamente: lo escribe la goroutine de Wait.
	select {
	case <-p.exited:
	case <-time.After(2 * time.Second):
		p.cmd.Process.Kill()
	}
}

// boundedBuffer conserva los últimos boundedBufferMax bytes escritos: acota la
// salida de mpv que capturamos para diagnóstico sin crecer sin límite.
type boundedBuffer struct {
	mu  sync.Mutex
	buf []byte
}

const boundedBufferMax = 8 << 10

func (b *boundedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf = append(b.buf, p...)
	if len(b.buf) > boundedBufferMax {
		b.buf = b.buf[len(b.buf)-boundedBufferMax:]
	}
	return len(p), nil
}

func (b *boundedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.buf)
}

// mpvStartError arma el error cuando mpv terminó antes de crear el socket,
// incluyendo su código de salida y lo que haya escrito (motivo del fallo).
func mpvStartError(cmd *exec.Cmd, out string) error {
	out = strings.TrimSpace(out)
	detail := ""
	if cmd.ProcessState != nil {
		detail = cmd.ProcessState.String()
	}
	if out != "" {
		return fmt.Errorf("%s (%s): %s", i18n.T("p.died"), detail, out)
	}
	return fmt.Errorf("%s (%s)", i18n.T("p.died"), detail)
}
