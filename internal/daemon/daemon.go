// Package daemon implementa el servidor de maly: escucha en un socket Unix,
// mantiene la cola y controla mpv. La TUI puede embeberlo en su proceso.
package daemon

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
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
	mu  sync.Mutex
	cfg config.Config
	lib *library.Library
	pl  *player.Player
	q   *queue.Queue
	ln  net.Listener
	// lock es el flock que acredita a este proceso como EL demonio. Hay que
	// retenerlo vivo: el lock pertenece al descriptor abierto (ver lock.go).
	lock     *os.File
	mpris    *mpris.Service // nil si no hay bus de sesión
	scanning atomic.Bool    // guarda contra escaneos simultáneos (scan corre sin d.mu)
	scanSeen atomic.Int64   // archivos vistos por el scan en vuelo (progreso en Status)
	// scanTotal > 0 durante la fase de duraciones del scan; entonces
	// scanSeen cuenta cuántas van de este total (ver ipc.Status.ScanTotal).
	scanTotal atomic.Int64

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

	// idleTimeout es el deadline de lectura por vuelta del bucle de serve
	// (ver su comentario). Campo de instancia y no var de paquete: un var
	// compartido lo escribiría un test mientras el serve() de OTRO demonio
	// —de un test anterior cuya goroutine Run() no había terminado de
	// desmontarse— seguía leyéndolo, y el race detector lo cazaba entre
	// tests aunque cada uno usara su propio Daemon.
	idleTimeout time.Duration
}

// subscriber es una conexión en modo push. dirty tiene capacidad 1: una
// ráfaga de cambios mientras se escribe el push anterior colapsa en uno solo.
type subscriber struct {
	conn  net.Conn
	dirty chan struct{}
}

// New prepara el demonio: reclama la identidad y el socket, abre la biblioteca
// y lanza mpv. El orden importa y es el que hace seguras las dos operaciones
// destructivas del arranque (borrar el socket viejo y matar el mpv viejo):
// hasta tener el lock no se toca nada de nadie.
func New(cfg config.Config) (*Daemon, error) {
	sock := config.SocketPath()
	// EnsureRuntimeDir además valida dueño/permisos: los sockets (maly, mpv),
	// el lock y el caché de carátulas solo se crean dentro de un dir de fiar.
	if _, err := config.EnsureRuntimeDir(); err != nil {
		return nil, err
	}
	// Este sondeo queda SOLO por compatibilidad: un demonio anterior a esta
	// versión no toma el lock, y sin preguntarle antes le robaríamos el socket
	// al actualizar el binario sin reiniciar el servicio.
	if ipc.Ping(sock) {
		return nil, ErrAlreadyRunning
	}
	// A partir de aquí somos EL demonio, y lo dice el kernel en vez de una
	// heurística: ya se puede borrar el socket que hubiera quedado, bindear el
	// nuestro y reapear el mpv de una sesión anterior (ver player.Start).
	lock, err := acquireLock()
	if err != nil {
		return nil, err
	}
	// Toda salida por error suelta el lock; el resto de recursos se cierran en
	// orden inverso a como se abrieron.
	failed := func(err error) (*Daemon, error) {
		lock.Close()
		return nil, err
	}

	os.Remove(sock)
	ln, err := net.Listen("unix", sock)
	if err != nil {
		return failed(fmt.Errorf("%s: %w", i18n.Tf("d.listen", sock), err))
	}
	// net.Listen crea el socket con el umask del proceso, no 0600 como el
	// resto de lo que vive en el runtime dir (lock, mpv.sock, art/): un
	// umask laxo del usuario lo dejaría 0777. El dir 0700 ya es la frontera
	// real, pero el socket es control total del reproductor y no cuesta
	// nada apretarlo también.
	os.Chmod(sock, 0o600)
	closeLn := func() {
		os.Remove(sock)
		ln.Close()
	}

	lib, err := library.Open(config.DBPath())
	if err != nil {
		closeLn()
		return failed(err)
	}
	d := &Daemon{
		cfg:         cfg,
		lib:         lib,
		q:           queue.New(),
		ln:          ln,
		lock:        lock,
		subs:        map[*subscriber]struct{}{},
		sessStop:    make(chan struct{}),
		idleTimeout: defaultIdleTimeout,
	}
	d.libGen.Store(1) // 0 queda reservado a demonios sin soporte (omitempty)

	pl, err := player.Start(filepath.Join(config.RuntimeDir(), "mpv.sock"), d.advance, d.notify)
	if err != nil {
		lib.Close()
		closeLn()
		return failed(err)
	}
	d.pl = pl

	// Reponer la sesión anterior antes de MPRIS, para que el primer cliente (y
	// playerctl) ya vean el estado restaurado; la ventana gapless se arma de
	// una vez (sin d.mu: aún no hay concurrencia). El socket ya está bindeado,
	// pero Run todavía no acepta: quien conecte espera en el backlog.
	d.restoreSession()
	d.syncWindowLocked()
	go d.sessionSaver()

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
	// Borrar el socket ANTES de cerrar el listener: entre cerrarlo y borrarlo,
	// otro demonio podría bindear la ruta y le estaríamos borrando el socket
	// recién creado. Mientras no cerremos seguimos siendo los dueños.
	os.Remove(config.SocketPath())
	d.ln.Close()
	d.pl.Close()
	os.Remove(filepath.Join(config.RuntimeDir(), "mpv.sock"))
	// Esperar al scan en vuelo antes de cerrar la base: si no, sus últimas
	// escrituras fallan con "database is closed" y el usuario ve errores
	// espurios al apagar. La espera está acotada porque Scan no admite
	// cancelación: uno largo la agotará y se cortará igual que antes, pero los
	// cortos —que son casi todos— terminan limpios. A cambio, un `maly kill`
	// lanzado en pleno escaneo puede tardar hasta estos 5 s.
	for i := 0; i < 100 && d.scanning.Load(); i++ {
		time.Sleep(50 * time.Millisecond)
	}
	d.lib.Close()
	// El lock, lo último: mientras lo tengamos, nadie puede arrancar y
	// encontrarse el desmontaje a medias. Cerrar el archivo libera el flock; el
	// archivo NO se borra, porque borrar un lockfile es una carrera clásica
	// (otro proceso puede tenerlo ya abierto y acabaría bloqueando un inodo
	// eliminado, creyéndose el demonio).
	if d.lock != nil {
		d.lock.Close()
	}
}

// defaultIdleTimeout es el deadline de lectura por vuelta del bucle de
// serve: un cliente que conecta y no manda nada (o deja de mandar) dejaría,
// sin esto, la goroutine y el fd clavados para siempre — N conexiones así
// agotan los descriptores del demonio. Generoso a propósito: Do responde en
// milisegundos, y las conexiones legítimamente largas son las de subscribe,
// que sale de este bucle antes de volver a este punto. Vive en
// Daemon.idleTimeout —campo de instancia, no var de paquete— para que los
// tests lo bajen en SU demonio sin correr detrás de las goroutines de
// serve() de demonios de otros tests, que un var compartido sí alcanza.
const defaultIdleTimeout = 5 * time.Minute

// serveWriteTimeout espeja el de subscriber.push: un cliente colgado con el
// buffer de recepción lleno no debe dejar la goroutine clavada en el Write.
const serveWriteTimeout = 5 * time.Second

func (d *Daemon) serve(conn net.Conn) {
	defer conn.Close()
	sc := bufio.NewScanner(conn)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for {
		conn.SetReadDeadline(time.Now().Add(d.idleTimeout))
		if !sc.Scan() {
			return
		}
		var req ipc.Request
		var resp ipc.Response
		if err := json.Unmarshal(sc.Bytes(), &req); err != nil {
			resp = ipc.Response{Error: i18n.Tf("d.invalid_req", err.Error())}
		} else if req.Cmd == "subscribe" {
			// La conexión pasa a modo push y no vuelve: subscribe escribe
			// el estado inicial y luego un push por cada cambio. Limpiar el
			// deadline de lectura antes de entregarla: subscribe bloquea a
			// propósito minutos sin que el cliente mande nada, y el
			// deadline puesto arriba para esta vuelta seguiría corriendo.
			conn.SetReadDeadline(time.Time{})
			d.subscribe(conn, sc)
			return
		} else if req.Cmd == "shutdown" {
			// Como subscribe, se intercepta antes de handle: la respuesta
			// debe salir antes de que Close tumbe listener y conexiones, y
			// dentro de dispatch el Close deadlockearía con d.mu.
			resp = ipc.Response{OK: true, Msg: i18n.TL(req.Lang, "d.bye"), Version: version.Version}
			data, _ := json.Marshal(resp)
			conn.SetWriteDeadline(time.Now().Add(serveWriteTimeout))
			conn.Write(append(data, '\n'))
			d.Close()
			return
		} else {
			resp = d.handle(req)
		}
		resp.Version = version.Version
		data, _ := json.Marshal(resp)
		conn.SetWriteDeadline(time.Now().Add(serveWriteTimeout))
		if _, err := conn.Write(append(data, '\n')); err != nil {
			return
		}
	}
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

// Do ejecuta una petición como si llegara por el socket. Los tests de este
// paquete la usan mucho como atajo directo a dispatch sin pasar por el
// socket; mpris usa la interfaz angosta de abajo, no esta.
func (d *Daemon) Do(req ipc.Request) ipc.Response { return d.handle(req) }

// Status devuelve una copia del estado actual (también para MPRIS).
func (d *Daemon) Status() *ipc.Status {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.statusLocked()
}

// Implementación de mpris.Controller: cada método reusa handle (el mismo
// camino que los clientes IPC) para no duplicar lógica de dispatch, y
// descarta la respuesta a propósito — la spec de MPRIS pide que estos
// métodos sean no-op cuando la acción no aplica (p. ej. "next" sin
// siguiente pista), así que no hay nada útil que reportarle al bus.
func (d *Daemon) Next()      { d.handle(ipc.Request{Cmd: "next"}) }
func (d *Daemon) Previous()  { d.handle(ipc.Request{Cmd: "prev"}) }
func (d *Daemon) Pause()     { d.handle(ipc.Request{Cmd: "pause"}) }
func (d *Daemon) PlayPause() { d.handle(ipc.Request{Cmd: "toggle"}) }
func (d *Daemon) Stop()      { d.handle(ipc.Request{Cmd: "stop"}) }
func (d *Daemon) Play()      { d.handle(ipc.Request{Cmd: "play"}) }

func (d *Daemon) SetVolume(pct int) { d.handle(ipc.Request{Cmd: "vol", Value: strconv.Itoa(pct)}) }

func (d *Daemon) SetShuffle(on bool) {
	v := "off"
	if on {
		v = "on"
	}
	d.handle(ipc.Request{Cmd: "shuffle", Value: v})
}

func (d *Daemon) SetRepeat(mode string) { d.handle(ipc.Request{Cmd: "repeat", Value: mode}) }

// SeekRel/SeekAbs traducen segundos (lo que pide mpris.Controller) al mismo
// formato de Value que ya entiende el case "seek" de dispatch — la
// conversión de la unidad wire vive acá, no en mpris.
func (d *Daemon) SeekRel(secs float64) {
	d.handle(ipc.Request{Cmd: "seek", Value: fmt.Sprintf("%+.3f", secs)})
}

func (d *Daemon) SeekAbs(secs float64) {
	d.handle(ipc.Request{Cmd: "seek", Value: fmt.Sprintf("%.3f", secs)})
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

// dispatch ejecuta el comando bajo el mutex del demonio, salvo las
// excepciones que se resuelven antes de tomarlo (scan, la resolución de
// pistas de play/add y el seek): todas hacen IO o esperan a mpv, y bajo el
// lock congelarían status, TUI y MPRIS.
func (d *Daemon) dispatch(req ipc.Request) ipc.Response {
	if req.Cmd == "scan" {
		// Sin d.mu: solo toca lib (thread-safe) y cfg (inmutable). Con el
		// mutex tomado, un escaneo largo congelaría play/status/TUI.
		return d.scan(req.Lang, req.Query)
	}

	// seek también se resuelve ANTES de d.mu: player.seek reintenta una vez
	// con 250 ms de sueño de por medio, y cada intento espera hasta 5 s la
	// respuesta de mpv — bajo el lock eso congelaba status, TUI y MPRIS, y
	// apilaba goroutines de notify() esperando el mutex. d.seek solo parsea
	// y habla con el player (que tiene mutex propio): no toca la cola ni
	// ningún otro estado, así que nada se corrompe fuera de d.mu. A cambio,
	// un seek concurrente con el next de otro cliente puede caer en la
	// pista nueva: daño menor y el mismo carácter que las excepciones de
	// scan y de la resolución de pistas.
	var seekErr error
	if req.Cmd == "seek" {
		seekErr = d.seek(req.Lang, req.Value)
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

	case "move":
		if !d.q.Move(req.Index, req.To) {
			bad := req.Index
			if bad >= 0 && bad < d.q.Len() {
				bad = req.To
			}
			return fail(errors.New(i18n.TLf(lang, "d.jump_oob", bad+1)))
		}
		return okStatus(i18n.TL(lang, "d.moved"))

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
		// Ya se ejecutó arriba, fuera del lock; aquí solo se reporta. El
		// Status lleva la posición nueva (player.seek la refresca).
		if seekErr != nil {
			return fail(seekErr)
		}
		return okStatus("")

	case "shuffle":
		switch req.Value {
		case "on":
			d.q.SetShuffle(true)
		case "off":
			d.q.SetShuffle(false)
		case "":
			d.q.SetShuffle(!d.q.Shuffle)
		default:
			// Antes esto caía en el mismo default de arriba (toggle) para
			// CUALQUIER basura: un typo en un script cambiaba el estado en
			// vez de avisar. `repeat`, con la misma forma de uso, ya
			// distinguía "" (toggle legítimo) de cualquier otra cosa
			// (auditoría 2026-07-31, hallazgo C11).
			return fail(errors.New(i18n.TLf(lang, "d.shuffle_invalid", req.Value)))
		}
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

	case "refresh":
		// Aviso de que la DB cambió por fuera (mutadores de playlists de la
		// CLI/TUI, que escriben SQLite directo): subir libGen basta — handle
		// trata refresh como mutador y su notify despierta a los
		// suscriptores, que recargan al ver la generación nueva.
		d.libGen.Add(1)
		return ipc.Response{OK: true}

	default:
		return fail(errors.New(i18n.TLf(lang, "d.unknown_cmd", req.Cmd)))
	}
}

// errAdjust cubre todo valor inválido de parseAdjust. El caller arma el
// mensaje visible con i18n (d.vol_invalid); estos errores nunca llegan al
// usuario, por eso no llevan texto traducido.
var errAdjust = errors.New("invalid adjust value")

// finite descarta lo que ParseFloat acepta pero no es un número usable. Hace
// falta porque "Inf" y "+Inf" parsean sin error, y sobre todo porque NaN
// sobrevive a TODA comparación: `NaN < 0` y `NaN > 100` son ambos false, así
// que se colaba por cualquier validación de rango y llegaba hasta mpv, donde
// json.Marshal lo rechaza — y con aquel error ignorado el comando se perdía y
// costaba 5 s de timeout con d.mu tomado.
func finite(f float64) bool { return !math.IsNaN(f) && !math.IsInf(f, 0) }

func parseAdjust(val string, cur, min, max float64) (float64, error) {
	val = strings.TrimSpace(val)
	if val == "" {
		return 0, errAdjust
	}
	if strings.HasPrefix(val, "+") || strings.HasPrefix(val, "-") {
		n, err := strconv.ParseFloat(val, 64)
		if err != nil || !finite(n) {
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
	if err != nil || !finite(n) || n < min || n > max {
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
	if d.scanning.Load() {
		s.Scanning = true
		s.ScanSeen = int(d.scanSeen.Load())
		s.ScanTotal = int(d.scanTotal.Load())
	}
	if t, ok := d.q.Current(); ok && !st.Idle {
		info := infoOf(t)
		s.Track = &info
	}
	return s
}
