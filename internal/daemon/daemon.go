// Package daemon implementa el servidor de maly: escucha en un socket Unix,
// mantiene la cola y controla mpv. La TUI puede embeberlo en su proceso.
package daemon

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"maly/internal/config"
	"maly/internal/i18n"
	"maly/internal/ipc"
	"maly/internal/library"
	"maly/internal/mpris"
	"maly/internal/player"
	"maly/internal/queue"
	"maly/internal/version"
)

// ErrAlreadyRunning indica que otro demonio ya posee el socket.
var ErrAlreadyRunning = errors.New("another maly daemon is already running")

type Daemon struct {
	mu       sync.Mutex
	cfg      config.Config
	lib      *library.Library
	pl       *player.Player
	q        *queue.Queue
	ln       net.Listener
	mpris    *mpris.Service // nil si no hay bus de sesión
	scanning atomic.Bool    // guarda contra escaneos simultáneos (scan corre sin d.mu)

	// libGen es la generación de la biblioteca: arranca en 1 y crece con
	// cada scan exitoso. statusLocked la adjunta a todo Status, y los
	// clientes recargan su copia de la biblioteca al verla cambiar.
	libGen atomic.Uint64

	// Pistas fallidas seguidas desde la última reproducción sana (bajo d.mu).
	// Guarda de advance: al acumular una pasada completa de la cola sin que
	// nada suene, se detiene en vez de ciclar para siempre. Es aproximada a
	// propósito: se resetea con el eof natural y con cada carga manual.
	errStreak int

	// stopped marca silencio deliberado (stop, clear, la guarda de advance;
	// bajo d.mu): un end-file en vuelo que llegue después de parar no debe
	// rearrancar la reproducción ni contar para la racha de errores. Toda
	// carga lo apaga.
	stopped bool

	// Suscriptores IPC (comando subscribe). Mutex propio: notify los marca
	// desde caminos que ya tienen (o van a tomar) d.mu.
	subMu sync.Mutex
	subs  map[*subscriber]struct{}

	// Persistencia de sesión: notify marca dirty y sessionSaver guarda en
	// caliente; Close cierra sessStop y hace el guardado final.
	sessDirty atomic.Bool
	sessStop  chan struct{}

	closeOnce sync.Once
}

// subscriber es una conexión en modo push. dirty tiene capacidad 1: una
// ráfaga de cambios mientras se escribe el push anterior colapsa en uno solo.
type subscriber struct {
	conn  net.Conn
	dirty chan struct{}
}

// New prepara el demonio: reclama el socket, abre la biblioteca y lanza mpv.
func New(cfg config.Config) (*Daemon, error) {
	sock := config.SocketPath()
	// EnsureRuntimeDir además valida dueño/permisos: los sockets (maly, mpv)
	// y el caché de carátulas solo se crean dentro de un directorio de fiar.
	if _, err := config.EnsureRuntimeDir(); err != nil {
		return nil, err
	}
	// Fallar rápido si ya hay demonio, ANTES de abrir DB y lanzar mpv. Solo
	// sondea: el socket huérfano se limpia más abajo, tras el EADDRINUSE —
	// borrarlo aquí podía desanclar el socket recién bindeado de otro demonio
	// arrancando a la vez (TOCTOU entre el ping y el listen).
	if ipc.Ping(sock) {
		return nil, ErrAlreadyRunning
	}

	lib, err := library.Open(config.DBPath())
	if err != nil {
		return nil, err
	}
	d := &Daemon{
		cfg:      cfg,
		lib:      lib,
		q:        queue.New(),
		subs:     map[*subscriber]struct{}{},
		sessStop: make(chan struct{}),
	}
	d.libGen.Store(1) // 0 queda reservado a demonios sin soporte (omitempty)

	pl, err := player.Start(filepath.Join(config.RuntimeDir(), "mpv.sock"), d.advance, d.notify)
	if err != nil {
		lib.Close()
		return nil, err
	}
	d.pl = pl

	// Reponer la sesión anterior antes del listener y de MPRIS, para que el
	// primer cliente (y playerctl) ya vean el estado restaurado; la ventana
	// gapless se arma de una vez (sin d.mu: aún no hay concurrencia).
	d.restoreSession()
	d.syncWindowLocked()
	go d.sessionSaver()

	ln, err := net.Listen("unix", sock)
	if errors.Is(err, syscall.EADDRINUSE) {
		// El path está ocupado: o hay un demonio vivo (arranques simultáneos:
		// mismo diagnóstico que el ping inicial) o quedó un socket huérfano de
		// una sesión que murió sin limpiar. Solo tras confirmar que nadie
		// contesta se borra y se reintenta una vez.
		if ipc.Ping(sock) {
			pl.Close()
			lib.Close()
			return nil, ErrAlreadyRunning
		}
		os.Remove(sock)
		ln, err = net.Listen("unix", sock)
		if errors.Is(err, syscall.EADDRINUSE) {
			pl.Close()
			lib.Close()
			return nil, ErrAlreadyRunning
		}
	}
	if err != nil {
		pl.Close()
		lib.Close()
		return nil, fmt.Errorf("%s: %w", i18n.Tf("d.listen", sock), err)
	}
	d.ln = ln

	// MPRIS es opcional: sin bus de sesión (p. ej. headless) el demonio
	// funciona igual, solo sin integración playerctl/Waybar.
	if m, err := mpris.Start(d, filepath.Join(config.RuntimeDir(), "art")); err != nil {
		fmt.Fprintln(os.Stderr, "maly: "+i18n.Tf("d.mpris_off", err))
	} else {
		d.mu.Lock()
		d.mpris = m
		d.mu.Unlock()
	}
	return d, nil
}

// Run atiende conexiones hasta que se cierre el listener (vía Close).
func (d *Daemon) Run() error {
	for {
		conn, err := d.ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		go d.serve(conn)
	}
}

// Close para todo: MPRIS, mpv, listener, socket y biblioteca. Antes guarda
// la sesión con la posición exacta (mpv sigue vivo en este punto). Es
// idempotente: solo el primer llamado hace trabajo.
func (d *Daemon) Close() { d.closeOnce.Do(d.doClose) }

func (d *Daemon) doClose() {
	close(d.sessStop)
	d.saveSessionNow()

	d.mu.Lock()
	m := d.mpris
	d.mpris = nil
	d.mu.Unlock()
	if m != nil {
		m.Close()
	}
	// Cortar los suscriptores: su lector detecta el cierre y la goroutine
	// de subscribe termina sola.
	d.subMu.Lock()
	for s := range d.subs {
		s.conn.Close()
	}
	d.subMu.Unlock()
	d.ln.Close()
	os.Remove(config.SocketPath())
	d.pl.Close()
	os.Remove(filepath.Join(config.RuntimeDir(), "mpv.sock"))
	d.lib.Close()
}

func (d *Daemon) serve(conn net.Conn) {
	defer conn.Close()
	sc := bufio.NewScanner(conn)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		var req ipc.Request
		var resp ipc.Response
		if err := json.Unmarshal(sc.Bytes(), &req); err != nil {
			resp = ipc.Response{Error: i18n.Tf("d.invalid_req", err.Error())}
		} else if req.Cmd == "subscribe" {
			// La conexión pasa a modo push y no vuelve: subscribe escribe
			// el estado inicial y luego un push por cada cambio.
			d.subscribe(conn, sc)
			return
		} else if req.Cmd == "shutdown" {
			// Como subscribe, se intercepta antes de handle: la respuesta
			// debe salir antes de que Close tumbe listener y conexiones, y
			// dentro de dispatch el Close deadlockearía con d.mu.
			resp = ipc.Response{OK: true, Msg: i18n.TL(req.Lang, "d.bye"), Version: version.Version}
			data, _ := json.Marshal(resp)
			conn.Write(append(data, '\n'))
			d.Close()
			return
		} else {
			resp = d.handle(req)
		}
		resp.Version = version.Version
		data, _ := json.Marshal(resp)
		if _, err := conn.Write(append(data, '\n')); err != nil {
			return
		}
	}
}

// advance es la política de avance de la cola cuando una pista termina sin
// intervención de un cliente: eof natural o fallo de reproducción (archivo
// corrupto, borrado…). Con gapless, mpv normalmente ya encadenó solo a la
// entrada anexada por syncWindowLocked; aquí se reconcilia la cola con lo
// que mpv hizo, se repara a mano cuando no pudo encadenar, y se re-arma la
// ventana con la promesa siguiente. chained es la entrada que el player
// tenía anexada al terminar la pista ("" = ninguna): mpv encadena a ella.
func (d *Daemon) advance(reason, chained string) {
	d.mu.Lock()
	if reason == "error" {
		if d.stopped {
			// Eco de una entrada que seguía en vuelo cuando paramos a
			// propósito: ni cuenta para la racha ni rearranca nada.
			d.mu.Unlock()
			return
		}
		if t, ok := d.q.Current(); ok {
			fmt.Fprintln(os.Stderr, "maly: "+i18n.Tf("d.track_failed", t))
		}
		d.errStreak++
		if d.errStreak >= d.q.Len() {
			// Una pasada entera sin nada reproducible (o cola ya vacía):
			// detenerse; seguir saltando ciclaría para siempre con repeat
			// all. Stop además vacía la playlist de mpv, cortando una
			// entrada anexada que estuviera por fallar igual.
			d.errStreak = 0
			d.stopped = true
			d.pl.Stop()
			d.mu.Unlock()
			fmt.Fprintln(os.Stderr, "maly: "+i18n.T("d.queue_failed"))
			d.notify()
			return
		}
	} else {
		d.errStreak = 0
	}

	if t, ok := d.q.PeekNext(); ok && chained == t.Path {
		// mpv está encadenando a la promesa anexada (gapless): solo
		// confirmar el avance en la cola, sin tocar la reproducción.
		d.q.Next(true)
	} else if chained == "" && !d.stopped {
		// No había nada anexado (fin de cola, o el append falló): mpv quedó
		// idle; cargar a mano, como antes de gapless. pl.Load directo y NO
		// loadLocked: una carga de salto no abre pasada nueva o la racha se
		// resetearía a cada intento y la guarda jamás cortaría el ciclo.
		if t, ok := d.q.Next(true); ok {
			if err := d.pl.Load(t.Path); err != nil {
				// mpv no contestó (murió, socket roto): sin end-file que
				// reintente, se deja constancia.
				fmt.Fprintf(os.Stderr, "maly: %s: %v\n", i18n.Tf("d.track_failed", t), err)
			}
		}
	}
	// else: lo anexado ya no es la promesa — un comando mutó la cola (y
	// cargó/realineó) entre el fin de pista y este punto; avanzar además
	// saltearía una pista.
	d.syncWindowLocked()
	d.mu.Unlock()
	d.notify()
}

// syncWindowLocked alinea la entrada anexada de mpv (la ventana gapless)
// con la promesa vigente de la cola; requiere d.mu. Es best-effort: si el
// append falla, el siguiente advance repara cargando a mano.
func (d *Daemon) syncWindowLocked() {
	next := ""
	if t, ok := d.q.PeekNext(); ok {
		next = t.Path
	}
	d.pl.SetNext(next)
}

// loadLocked carga t en el player (requiere d.mu). Una carga pedida por un
// cliente que mpv acepta abre pasada nueva para la guarda de advance y
// termina cualquier silencio deliberado.
func (d *Daemon) loadLocked(t library.Track) error {
	if err := d.pl.Load(t.Path); err != nil {
		return err
	}
	d.errStreak = 0
	d.stopped = false
	return nil
}

// subscribe atiende una conexión en modo push desde la goroutine de serve:
// estado inicial, y uno nuevo cada vez que notify marca dirty, con un mínimo
// de 250 ms entre pushes (los ticks de time-pos de mpv llegan varios por
// segundo). Vuelve —y serve cierra la conexión— cuando el cliente cuelga.
func (d *Daemon) subscribe(conn net.Conn, sc *bufio.Scanner) {
	s := &subscriber{conn: conn, dirty: make(chan struct{}, 1)}
	// Registrar antes del primer push: un cambio entre la foto inicial y el
	// registro se perdería; así a lo sumo genera un push extra inmediato.
	d.subMu.Lock()
	d.subs[s] = struct{}{}
	d.subMu.Unlock()
	defer func() {
		d.subMu.Lock()
		delete(d.subs, s)
		d.subMu.Unlock()
	}()

	// El cliente ya no habla: cualquier retorno del lector es que colgó.
	done := make(chan struct{})
	go func() {
		for sc.Scan() {
		}
		close(done)
	}()

	if s.push(d.state()) != nil {
		return
	}
	for {
		select {
		case <-done:
			return
		case <-s.dirty:
			if s.push(d.state()) != nil {
				return
			}
			select {
			case <-done:
				return
			case <-time.After(250 * time.Millisecond):
			}
		}
	}
}

// push escribe una respuesta en la conexión suscrita. El deadline evita que
// un cliente colgado (buffer lleno) deje la goroutine clavada para siempre.
func (s *subscriber) push(resp ipc.Response) error {
	data, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	s.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_, err = s.conn.Write(append(data, '\n'))
	return err
}

// state arma la foto completa que reciben los suscriptores, con la misma
// forma que la respuesta del comando queue.
func (d *Daemon) state() ipc.Response {
	d.mu.Lock()
	defer d.mu.Unlock()
	return ipc.Response{OK: true, Status: d.statusLocked(), Queue: toInfos(d.q.Items), Version: version.Version}
}

// handle ejecuta la petición y refleja los cambios en MPRIS y suscriptores.
func (d *Daemon) handle(req ipc.Request) ipc.Response {
	resp := d.dispatch(req)
	switch req.Cmd {
	case "ping", "status", "queue", "search", "scan":
		// solo lectura: nada que reflejar
	case "seek":
		if m, st := d.mprisState(); m != nil {
			m.Update(st)
			if resp.OK {
				m.Seeked(int64(st.Position * 1e6))
			}
		}
		d.wakeSubs()
	default:
		// Cualquier mutador puede haber cambiado la promesa de la cola (add
		// al final, remove de la prometida, shuffle…): realinear la ventana
		// gapless de mpv antes de notificar. Con la promesa sin cambios es
		// gratis (SetNext corta por su espejo).
		d.mu.Lock()
		d.syncWindowLocked()
		d.mu.Unlock()
		d.notify()
	}
	return resp
}

// Do ejecuta una petición como si llegara por el socket; lo usa el servicio
// MPRIS para no duplicar la lógica de comandos.
func (d *Daemon) Do(req ipc.Request) ipc.Response { return d.handle(req) }

// Status devuelve una copia del estado actual (también para MPRIS).
func (d *Daemon) Status() *ipc.Status {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.statusLocked()
}

// mprisState toma el servicio y una copia coherente del estado, o nil si
// MPRIS no está activo.
func (d *Daemon) mprisState() (*mpris.Service, *ipc.Status) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.mpris == nil {
		return nil, nil
	}
	return d.mpris, d.statusLocked()
}

// notify refleja el estado actual en MPRIS, despierta a los suscriptores
// IPC y marca la sesión para el guardado en caliente; los eventos de mpv que
// no pasan por handle (pausa externa, fin de pista, ticks de posición)
// llegan aquí vía el onChange del player.
func (d *Daemon) notify() {
	d.learnDuration()
	if m, st := d.mprisState(); m != nil {
		m.Update(st)
	}
	d.wakeSubs()
	d.sessDirty.Store(true)
}

// learnDuration aprende perezosamente la duración de la pista actual cuando
// mpv la reporta: los tags no la traen, así que la biblioteca la va
// completando a medida que suena música. La copia en memoria de la cola
// hace que solo se escriba una vez por pista.
func (d *Daemon) learnDuration() {
	d.mu.Lock()
	defer d.mu.Unlock()
	st := d.pl.State()
	t, ok := d.q.Current()
	if !ok || st.Idle || st.Duration <= 0 || abs(t.Duration-st.Duration) < 0.5 {
		return
	}
	for i := range d.q.Items {
		if d.q.Items[i].Path == t.Path {
			d.q.Items[i].Duration = st.Duration
		}
	}
	// Fuera de la biblioteca (pista suelta por ruta) el UPDATE no toca filas.
	d.lib.SetDuration(t.Path, st.Duration)
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// wakeSubs marca dirty a cada suscriptor; el envío no bloquea (cap 1: si ya
// hay una marca pendiente, este cambio viaja en ese mismo push).
func (d *Daemon) wakeSubs() {
	d.subMu.Lock()
	for s := range d.subs {
		select {
		case s.dirty <- struct{}{}:
		default:
		}
	}
	d.subMu.Unlock()
}

// dispatch ejecuta el comando bajo el mutex del demonio.
func (d *Daemon) dispatch(req ipc.Request) ipc.Response {
	if req.Cmd == "scan" {
		// Sin d.mu: solo toca lib (thread-safe) y cfg (inmutable). Con el
		// mutex tomado, un escaneo largo congelaría play/status/TUI.
		return d.scan(req.Lang, req.Query)
	}

	// play/add resuelven sus pistas ANTES de tomar d.mu: resolveTracks puede
	// recorrer un directorio leyendo tags (IO lento) y trackFromFile leerlos
	// de rutas fuera de la biblioteca — bajo el lock congelarían status, TUI
	// y MPRIS, la misma lección que sacó a scan de aquí. lib es thread-safe.
	var resolved []library.Track
	var resolveErr error
	switch {
	case req.Cmd == "play" && strings.TrimSpace(req.Query) != "",
		req.Cmd == "add" && len(req.Paths) == 0:
		resolved, resolveErr = d.resolveTracks(req.Lang, req.Query)
	case (req.Cmd == "add" || req.Cmd == "playnow") && len(req.Paths) > 0:
		for _, p := range req.Paths {
			resolved = append(resolved, trackFromFile(d.lib, p))
		}
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	// Responder en el idioma del cliente (los clientes viejos no mandan
	// lang: TL cae en el idioma del proceso).
	lang := req.Lang

	fail := func(err error) ipc.Response { return ipc.Response{Error: err.Error()} }
	okStatus := func(msg string) ipc.Response {
		return ipc.Response{OK: true, Msg: msg, Status: d.statusLocked()}
	}

	switch req.Cmd {
	case "ping":
		return ipc.Response{OK: true}

	case "status":
		return ipc.Response{OK: true, Status: d.statusLocked()}

	case "queue":
		return ipc.Response{OK: true, Status: d.statusLocked(), Queue: toInfos(d.q.Items)}

	case "search":
		tracks, err := d.lib.Search(req.Query)
		if err != nil {
			return fail(err)
		}
		return ipc.Response{OK: true, Queue: toInfos(tracks)}

	case "play":
		if strings.TrimSpace(req.Query) != "" {
			if resolveErr != nil {
				return fail(resolveErr)
			}
			d.q.Replace(resolved)
			t, _ := d.q.JumpTo(0)
			if err := d.loadLocked(t); err != nil {
				return fail(err)
			}
			return okStatus(i18n.TLf(lang, "d.playing_n", t, len(resolved)))
		}
		return d.resumeLocked(lang, fail, okStatus)

	case "pause":
		if err := d.pl.SetPause(true); err != nil {
			return fail(err)
		}
		return okStatus(i18n.TL(lang, "d.paused"))

	case "toggle":
		if d.pl.State().Idle {
			return d.resumeLocked(lang, fail, okStatus)
		}
		if err := d.pl.Toggle(); err != nil {
			return fail(err)
		}
		return okStatus("")

	case "stop":
		if err := d.pl.Stop(); err != nil {
			return fail(err)
		}
		d.stopped = true
		return okStatus(i18n.TL(lang, "d.stopped"))

	case "next":
		t, ok := d.q.Next(false)
		if !ok {
			return fail(errors.New(i18n.TL(lang, "d.no_next")))
		}
		if err := d.loadLocked(t); err != nil {
			return fail(err)
		}
		return okStatus(i18n.TLf(lang, "d.playing", t))

	case "prev":
		t, ok := d.q.Prev()
		if !ok {
			return fail(errors.New(i18n.TL(lang, "d.queue_empty")))
		}
		if err := d.loadLocked(t); err != nil {
			return fail(err)
		}
		return okStatus(i18n.TLf(lang, "d.playing", t))

	case "playnow":
		// Agrega pistas exactas (rutas, ya resueltas arriba) y salta a la
		// primera; usado por la TUI.
		if len(resolved) == 0 {
			return fail(errors.New(i18n.TL(lang, "d.playnow_paths")))
		}
		first := d.q.Len()
		d.q.Add(resolved...)
		t, _ := d.q.JumpTo(first)
		if err := d.loadLocked(t); err != nil {
			return fail(err)
		}
		return okStatus(i18n.TLf(lang, "d.playing", t))

	case "add":
		if resolveErr != nil {
			return fail(resolveErr)
		}
		tracks := resolved
		wasEmpty := d.q.Len() == 0
		d.q.Add(tracks...)
		msg := i18n.TLf(lang, "d.added_n", len(tracks))
		if wasEmpty && d.pl.State().Idle {
			if t, ok := d.q.JumpTo(0); ok {
				if err := d.loadLocked(t); err != nil {
					return fail(err)
				}
				msg += i18n.TLf(lang, "d.also_playing", t)
			}
		}
		return okStatus(msg)

	case "jump":
		t, ok := d.q.JumpTo(req.Index)
		if !ok {
			return fail(errors.New(i18n.TLf(lang, "d.jump_oob", req.Index+1)))
		}
		if err := d.loadLocked(t); err != nil {
			return fail(err)
		}
		return okStatus(i18n.TLf(lang, "d.playing", t))

	case "remove":
		removed, wasCurrent := d.q.RemoveAt(req.Index)
		if !removed {
			return fail(errors.New(i18n.TLf(lang, "d.jump_oob", req.Index+1)))
		}
		if wasCurrent {
			// Continuar con la siguiente respetando el estado del player:
			// pausado sigue pausado, y si nada sonaba no se arranca nada.
			st := d.pl.State()
			t, ok := d.q.Current()
			switch {
			case st.Idle || !ok:
				if err := d.pl.Stop(); err != nil {
					return fail(err)
				}
				d.stopped = true
			case st.Paused:
				if err := d.pl.LoadPaused(t.Path); err != nil {
					return fail(err)
				}
				d.errStreak = 0
				d.stopped = false
			default:
				if err := d.loadLocked(t); err != nil {
					return fail(err)
				}
			}
		}
		return okStatus(i18n.TL(lang, "d.removed"))

	case "clear":
		d.q.Clear()
		d.pl.Stop()
		d.stopped = true
		return okStatus(i18n.TL(lang, "d.cleared"))

	case "vol":
		cur := d.pl.State().Volume
		v, err := parseAdjust(req.Value, cur, 0, 100)
		if err != nil {
			return fail(errors.New(i18n.TLf(lang, "d.vol_invalid", req.Value)))
		}
		if err := d.pl.SetVolume(v); err != nil {
			return fail(err)
		}
		return okStatus(i18n.TLf(lang, "d.vol_set", int(v)))

	case "seek":
		if err := d.seekLocked(lang, req.Value); err != nil {
			return fail(err)
		}
		return okStatus("")

	case "shuffle":
		switch req.Value {
		case "on":
			d.q.Shuffle = true
		case "off":
			d.q.Shuffle = false
		default:
			d.q.Shuffle = !d.q.Shuffle
		}
		d.q.Invalidate() // el sorteo prometido dependía del modo anterior
		if d.q.Shuffle {
			return okStatus(i18n.TL(lang, "d.shuffle_on"))
		}
		return okStatus(i18n.TL(lang, "d.shuffle_off"))

	case "repeat":
		switch req.Value {
		case "off", "all", "one":
			d.q.Repeat = queue.RepeatMode(req.Value)
		case "":
			d.q.CycleRepeat()
		default:
			return fail(errors.New(i18n.TLf(lang, "d.repeat_invalid", req.Value)))
		}
		d.q.Invalidate() // repeat one/all cambian la promesa vigente
		return okStatus(i18n.TLf(lang, "d.repeat", string(d.q.Repeat)))

	case "playlist_play":
		tracks, err := d.lib.PlaylistTracks(req.Value)
		if err != nil {
			return fail(err)
		}
		if len(tracks) == 0 {
			return fail(errors.New(i18n.TLf(lang, "d.pl_empty", req.Value)))
		}
		d.q.Replace(tracks)
		t, _ := d.q.JumpTo(0)
		if err := d.loadLocked(t); err != nil {
			return fail(err)
		}
		return okStatus(i18n.TLf(lang, "d.playing_pl", req.Value, len(tracks)))

	default:
		return fail(errors.New(i18n.TLf(lang, "d.unknown_cmd", req.Cmd)))
	}
}

// scan (re)indexa dir sin tomar d.mu: library serializa sus sentencias en su
// única conexión SQLite, y así play/status siguen respondiendo durante el
// escaneo. Solo se permite un escaneo a la vez.
func (d *Daemon) scan(lang, query string) ipc.Response {
	if !d.scanning.CompareAndSwap(false, true) {
		return ipc.Response{Error: i18n.TL(lang, "d.scan_busy")}
	}
	defer d.scanning.Store(false)

	dir, origin, explicit := d.cfg.ScanTarget(query)
	res, err := d.lib.Scan(dir)
	if err != nil {
		if !explicit && errors.Is(err, fs.ErrNotExist) {
			return ipc.Response{Error: i18n.TLf(lang, "cli.scan_noexist", dir, i18n.TL(lang, origin))}
		}
		return ipc.Response{Error: err.Error()}
	}
	total, _ := d.lib.Count()
	if res.Added+res.Updated+res.Removed > 0 {
		// La biblioteca cambió de generación: despertar a los suscriptores
		// aquí mismo (handle trata scan como solo-lectura y no lo haría). Un
		// scan sin cambios no recarga el árbol de nadie.
		d.libGen.Add(1)
		d.wakeSubs()
	}
	msg := i18n.TLf(lang, "d.scan_done", res.Added, res.Updated, res.Removed, total)
	if len(res.Errors) > 0 {
		// Vía IPC los errores por archivo no viajan (serían cientos de
		// líneas): el detalle va al stderr del demonio (como los fallos de
		// pista) y la respuesta al menos dice cuántos hubo.
		for _, e := range res.Errors {
			fmt.Fprintln(os.Stderr, "maly: "+i18n.Tf("cli.scan_warn", e))
		}
		msg += i18n.TLf(lang, "d.scan_errs", len(res.Errors))
	}
	return ipc.Response{OK: true, Msg: msg}
}

// resumeLocked reanuda: quita pausa si hay pista, o arranca la cola si mpv
// está idle.
func (d *Daemon) resumeLocked(lang string, fail func(error) ipc.Response, okStatus func(string) ipc.Response) ipc.Response {
	st := d.pl.State()
	if !st.Idle {
		if err := d.pl.SetPause(false); err != nil {
			return fail(err)
		}
		return okStatus("")
	}
	t, ok := d.q.Current()
	if !ok {
		if t, ok = d.q.JumpTo(0); !ok {
			return fail(errors.New(i18n.TL(lang, "d.queue_empty_hint")))
		}
	}
	if err := d.loadLocked(t); err != nil {
		return fail(err)
	}
	return okStatus(i18n.TLf(lang, "d.playing", t))
}

func (d *Daemon) seekLocked(lang, val string) error {
	val = strings.TrimSpace(val)
	if val == "" {
		return errors.New(i18n.TL(lang, "d.seek_usage"))
	}
	if strings.Contains(val, ":") {
		// mm:ss o hh:mm:ss (mixes y podcasts pasan de la hora).
		parts := strings.Split(val, ":")
		if len(parts) > 3 {
			return errors.New(i18n.TLf(lang, "d.seek_mmss", val))
		}
		secs := 0
		for i, p := range parts {
			n, err := strconv.Atoi(p)
			// Solo el campo más significativo (horas, o minutos sin horas)
			// puede pasar de 59.
			if err != nil || n < 0 || (i > 0 && n > 59) {
				return errors.New(i18n.TLf(lang, "d.seek_mmss", val))
			}
			secs = secs*60 + n
		}
		return d.pl.SeekAbs(float64(secs))
	}
	if strings.HasPrefix(val, "+") || strings.HasPrefix(val, "-") {
		n, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return errors.New(i18n.TLf(lang, "d.seek_offset", val))
		}
		return d.pl.SeekRel(n)
	}
	n, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return errors.New(i18n.TLf(lang, "d.seek_abs", val))
	}
	return d.pl.SeekAbs(n)
}

// resolveTracks convierte una consulta o ruta en pistas: archivo suelto,
// directorio (recursivo) o búsqueda en la biblioteca.
func (d *Daemon) resolveTracks(lang, q string) ([]library.Track, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return nil, errors.New(i18n.TL(lang, "d.missing_query"))
	}
	p := config.ExpandTilde(q)
	if abs, err := filepath.Abs(p); err == nil {
		if fi, err := os.Stat(abs); err == nil {
			if fi.IsDir() {
				return tracksFromDir(lang, d.lib, abs)
			}
			return []library.Track{trackFromFile(d.lib, abs)}, nil
		}
	}
	tracks, err := d.lib.Search(q)
	if err != nil {
		return nil, err
	}
	if len(tracks) == 0 {
		return nil, errors.New(i18n.TLf(lang, "d.no_results", q))
	}
	return tracks, nil
}

func trackFromFile(lib *library.Library, path string) library.Track {
	if t, ok := lib.ByPath(path); ok {
		return t
	}
	// fuera de la biblioteca: leer los tags al vuelo para no encolar la
	// pista con el nombre de archivo como único dato
	return library.ReadTags(path)
}

func tracksFromDir(lang string, lib *library.Library, dir string) ([]library.Track, error) {
	var out []library.Track
	err := filepath.WalkDir(dir, func(path string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() || !library.IsAudio(path) {
			return nil
		}
		out = append(out, trackFromFile(lib, path))
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, errors.New(i18n.TLf(lang, "d.no_audio", dir))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// errAdjust cubre todo valor inválido de parseAdjust. El caller arma el
// mensaje visible con i18n (d.vol_invalid); estos errores nunca llegan al
// usuario, por eso no llevan texto traducido.
var errAdjust = errors.New("invalid adjust value")

func parseAdjust(val string, cur, min, max float64) (float64, error) {
	val = strings.TrimSpace(val)
	if val == "" {
		return 0, errAdjust
	}
	if strings.HasPrefix(val, "+") || strings.HasPrefix(val, "-") {
		n, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return 0, errAdjust
		}
		val := cur + n
		if val < min {
			val = min
		}
		if val > max {
			val = max
		}
		return val, nil
	}
	n, err := strconv.ParseFloat(val, 64)
	if err != nil || n < min || n > max {
		return 0, errAdjust
	}
	return n, nil
}

func toInfos(tracks []library.Track) []ipc.TrackInfo {
	out := make([]ipc.TrackInfo, len(tracks))
	for i, t := range tracks {
		out[i] = infoOf(t)
	}
	return out
}

func infoOf(t library.Track) ipc.TrackInfo {
	return ipc.TrackInfo{ID: t.ID, Path: t.Path, Title: t.Title, Artist: t.Artist,
		Album: t.Album, AlbumArtist: t.AlbumArtist, Genre: t.Genre, TrackNo: t.TrackNo,
		Duration: t.Duration}
}

func (d *Daemon) statusLocked() *ipc.Status {
	st := d.pl.State()
	s := &ipc.Status{
		Playing:    !st.Idle,
		Paused:     st.Paused,
		Position:   st.Position,
		Duration:   st.Duration,
		Volume:     int(st.Volume + 0.5),
		Shuffle:    d.q.Shuffle,
		Repeat:     string(d.q.Repeat),
		QueueIndex: d.q.Index,
		QueueLen:   d.q.Len(),
		LibGen:     d.libGen.Load(),
	}
	if t, ok := d.q.Current(); ok && !st.Idle {
		info := infoOf(t)
		s.Track = &info
	}
	return s
}
