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
	onEOF    func() // pista terminada de forma natural
	onChange func() // cambio de estado observado (pausa, pista, posición…)
	closed   bool
	done     chan struct{}
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

// Start lanza mpv y conecta con su socket IPC. onEOF se invoca cuando una
// pista termina por sí sola (para avanzar la cola); onChange, ante cambios
// de estado observados en mpv (lo usa el demonio para refrescar MPRIS).
func Start(socketPath string, onEOF, onChange func()) (*Player, error) {
	mpvBin, err := exec.LookPath("mpv")
	if err != nil {
		return nil, errors.New(i18n.T("p.no_mpv"))
	}
	os.Remove(socketPath)
	cmd := exec.Command(mpvBin,
		"--idle=yes", "--no-video", "--no-terminal",
		"--audio-display=no",
		"--input-ipc-server="+socketPath,
		"--volume=100",
	)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.T("p.launch"), err)
	}

	// mpv tarda un poco en crear el socket.
	var conn net.Conn
	for i := 0; i < 50; i++ {
		conn, err = net.Dial("unix", socketPath)
		if err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		cmd.Process.Kill()
		return nil, fmt.Errorf("%s: %w", i18n.T("p.connect"), err)
	}

	p := &Player{
		cmd:      cmd,
		conn:     conn,
		pending:  map[int64]chan mpvReply{},
		state:    State{Idle: true, Volume: 100},
		onEOF:    onEOF,
		onChange: onChange,
		done:     make(chan struct{}),
	}
	go p.readLoop()
	go func() { cmd.Wait() }() // evitar zombi

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
		// Async como onEOF: en línea bloquearía readLoop, y con él las
		// respuestas de mpv que el demonio pueda estar esperando.
		if changed && p.onChange != nil {
			go p.onChange()
		}
	case "end-file":
		if ev.Reason == "eof" && p.onEOF != nil {
			go p.onEOF()
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
		return nil, fmt.Errorf("escribiendo a mpv: %w", err)
	}
	select {
	case r := <-ch:
		if r.Err != "success" {
			return nil, fmt.Errorf("mpv: %s", r.Err)
		}
		return r.Data, nil
	case <-time.After(5 * time.Second):
		return nil, errors.New("mpv no responde")
	case <-p.done:
		return nil, errors.New(i18n.T("p.exited"))
	}
}

// Load carga y reproduce un archivo (reemplaza lo que suene).
func (p *Player) Load(path string) error {
	if _, err := p.command("loadfile", path, "replace"); err != nil {
		return err
	}
	_, err := p.command("set_property", "pause", false)
	return err
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

// Stop detiene y descarga la pista actual.
func (p *Player) Stop() error {
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
			return fmt.Errorf("no pude hacer seek: %w", err)
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

	if p.cmd.Process != nil {
		waited := make(chan struct{})
		go func() {
			for i := 0; i < 20; i++ {
				if p.cmd.ProcessState != nil {
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
			close(waited)
		}()
		<-waited
		if p.cmd.ProcessState == nil {
			p.cmd.Process.Kill()
		}
	}
}
