package daemon

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"maly/internal/i18n"
	"maly/internal/ipc"
	"maly/internal/library"
	"maly/internal/player"
)

// Reproducción: avance de pista, ventana gapless, carga y seek.

// advance es la política de avance de la cola cuando una pista termina sin
// intervención de un cliente: eof natural o fallo de reproducción (archivo
// corrupto, borrado…). Con gapless, mpv normalmente ya encadenó solo a la
// entrada anexada por syncWindowLocked; aquí se reconcilia la cola con lo
// que mpv hizo, se repara a mano cuando no pudo encadenar, y se re-arma la
// ventana con la promesa siguiente. chained es la entrada que el player
// tenía anexada al terminar la pista ("" = ninguna): mpv encadena a ella.
func (d *Daemon) advance(reason, chained string, gen int64) {
	// El cuerpo bajo d.mu va en una función inmediata con defer (no
	// Lock/Unlock a mano, que tenía tres salidas distintas): esto corre
	// desde el callback onEnd/onChange del player, que ahora se protege con
	// recover en player.go — pero recover solo evita que el proceso muera,
	// no restaura un mutex que un panic a mitad de este cuerpo hubiera
	// dejado tomado. skipNotify replica exactamente los tres desenlaces
	// originales (eco tras stop = sin notify; racha agotada = notify sin
	// syncWindowLocked; camino normal = ambos). queueFailed viaja aparte
	// para que su Fprintln corra DESPUÉS de soltar d.mu, como en el código
	// original — moverlo adentro (como quedó en una primera versión de este
	// refactor) dejaba un stderr bloqueado (pipe lleno, journald bajo
	// presión) congelando el demonio ENTERO mientras dura el Write, en vez
	// de solo demorar una línea de log (hallazgo de la revisión posterior).
	skipNotify, queueFailed := func() (bool, bool) {
		d.mu.Lock()
		defer d.mu.Unlock()

		if gen != d.pl.LoadGen() {
			// Un loadfile replace de otro cliente (jump, play, next, stop…)
			// se cruzó entre resolveEnd y este punto: el desenlace es de una
			// carga ya superada y `chained` describe una promesa que mpv ya
			// no tiene. Ni cuenta para la racha ni avanza la cola.
			//
			// Hace falta ADEMÁS de la comparación con PeekNext de más abajo,
			// que no lo cubre: esa compara la promesa vieja contra la COLA,
			// no contra lo que mpv reproduce, así que un jump al índice que
			// ya era el actual deja PeekNext idéntico y matchea por
			// coincidencia — avanzando sobre una premisa anulada y saltándose
			// la pista prometida (se pierde entera: el syncWindowLocked del
			// final la saca de la playlist de mpv). La generación es lo único
			// que distingue "mpv encadenó de verdad" de "alguien recargó".
			//
			// skipNotify como el eco tras stop: el mutador que subió la
			// generación ya realineó la ventana y notificó por su handle().
			return true, false
		}

		// Un fin aceptado cierra la ventana de recuperación, incluso al agotar
		// la cola. Los callbacks de generaciones anteriores ya salieron arriba.
		d.q.DiscardUndo()
		if reason == "error" {
			if d.stopped {
				// Eco de una entrada que seguía en vuelo cuando paramos a
				// propósito: ni cuenta para la racha ni rearranca nada.
				return true, false
			}
			if t, ok := d.q.Current(); ok {
				// Al stderr (queda en el journal para un postmortem) Y en
				// Status, que es lo que de verdad ve el usuario: hasta A-25
				// solo existía lo primero y la pista se saltaba sola, sin
				// explicación en pantalla.
				fmt.Fprintln(os.Stderr, "maly: "+i18n.Tf("d.track_failed", t))
				d.setNoticeLocked("d.track_failed", t.String())
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
				// Pisa al aviso de pista saltada: este es el estado terminal
				// —no suena nada y no va a arrancar solo— y es el que hay que
				// contar. Es el caso del disco de música desmontado.
				d.setNoticeLocked("d.queue_failed")
				return false, true
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
		} else if !d.stopped && d.q.PendingCount() > 0 {
			// EOF cruzado con playnext: mpv encadenó la promesa anterior.
			// Honrar la prioridad sin marcar esa entrada normal como oída.
			// LoadGen ya descartó callbacks de cargas explícitas superadas.
			if t, ok := d.q.Next(true); ok {
				var err error
				if d.pl.State().Paused {
					err = d.pl.LoadPaused(t.Path)
				} else {
					err = d.pl.Load(t.Path)
				}
				if err != nil {
					d.setNoticeLocked("d.track_failed", t.String())
				}
			}
		}
		// else: lo anexado ya no es la promesa — un comando mutó la cola (y
		// cargó/realineó) entre el fin de pista y este punto; avanzar además
		// saltearía una pista.
		d.syncWindowLocked()
		return false, false
	}()
	if queueFailed {
		fmt.Fprintln(os.Stderr, "maly: "+i18n.T("d.queue_failed"))
	}
	if !skipNotify {
		d.notify()
	}
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
	d.notice, d.noticeArgs = "", nil // una carga sana: el problema pasó
	return nil
}

// setNoticeLocked guarda el último aviso para el usuario. Guarda la CLAVE y
// sus argumentos, no el texto ya armado: quien pregunta puede tener otro
// idioma que el demonio, y statusLocked lo traduce con el de cada quien.
func (d *Daemon) setNoticeLocked(key string, args ...any) {
	d.notice, d.noticeArgs = key, args
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
	d.q.DiscardUndo() // arranque desde idle; reanudar una pausa salió arriba
	t, ok := d.q.Current()
	if !ok {
		if d.q.PendingCount() > 0 {
			t, ok = d.q.Next(false)
		} else {
			t, ok = d.q.JumpTo(0)
		}
		if !ok {
			return fail(errors.New(i18n.TL(lang, "d.queue_empty_hint")))
		}
	}
	if err := d.loadLocked(t); err != nil {
		return fail(err)
	}
	return okStatus(i18n.TLf(lang, "d.playing", t))
}

// seek parsea el valor y se lo pasa al player. Corre SIN d.mu (ver dispatch):
// no toca la cola ni ningún otro estado del demonio.
func (d *Daemon) seek(lang, val string) error {
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
		if err != nil || !finite(n) {
			return errors.New(i18n.TLf(lang, "d.seek_offset", val))
		}
		return d.pl.SeekRel(n)
	}
	n, err := strconv.ParseFloat(val, 64)
	if err != nil || !finite(n) {
		return errors.New(i18n.TLf(lang, "d.seek_abs", val))
	}
	return d.pl.SeekAbs(n)
}

// clockOf devuelve la posición y la duración del espejo del player si
// describen el archivo de t, y 0, 0 si todavía describen otro. Justo después
// de un next la cola ya dice la pista nueva pero mpv no soltó la vieja: contesta
// al loadfile —y al set pause que Load manda detrás, en la misma vuelta— ANTES
// de emitir los eventos del cambio de archivo. Medido tras un next: 25 de 30
// fotos llevaban la posición de la pista anterior y 29 de 30 su duración, con
// la pista nueva en Track; MPRIS publicaba esa posición justo cuando los
// clientes la preguntan, y learnDuration escribía esa duración en la fila de
// la nueva.
//
// Comparar path no depende de tiempos: es la ÚLTIMA propiedad que observa
// player.Start, mpv manda las de un mismo lote en orden de registro, y el path
// nuevo no puede llegar antes de que el archivo arranque — así que cuando el
// espejo dice el path de t, los null con que mpv suelta el anterior ya pasaron.
func clockOf(st player.State, t library.Track) (pos, dur float64) {
	if st.Path != t.Path {
		return 0, 0
	}
	return st.Position, st.Duration
}

// learnDuration aprende perezosamente la duración de la pista actual cuando
// mpv la reporta: los tags no la traen, así que la biblioteca la va
// completando a medida que suena música. La copia en memoria de la cola
// hace que solo se escriba una vez por pista.
func (d *Daemon) learnDuration() {
	// Función inmediata con defer en vez de Lock/Unlock a mano (mismo motivo
	// que advance(): esto corre desde el callback onChange del player).
	path, secs, ok := func() (string, float64, bool) {
		d.mu.Lock()
		defer d.mu.Unlock()
		st := d.pl.State()
		t, ok := d.q.Current()
		if !ok || st.Idle {
			return "", 0, false
		}
		// La de mpv solo si es de t (ver clockOf): si no, era la duración de
		// la pista anterior escrita en la fila de la nueva.
		_, dur := clockOf(st, t)
		if dur <= 0 || abs(t.Duration-dur) < 0.5 {
			return "", 0, false
		}
		for i := range d.q.Items {
			if d.q.Items[i].Path == t.Path {
				d.q.Items[i].Duration = dur
			}
		}
		// Escritura EN SITIO: no pasa por ningún mutador de queue, así que
		// hay que subir la generación a mano o el push nunca llevaría la
		// duración recién aprendida (A-05).
		d.q.Touch()
		return t.Path, st.Duration, true
	}()
	if !ok {
		return
	}

	// La escritura va FUERA de d.mu: era la única del demonio que corría con el
	// mutex tomado, y encima se dispara en cada cambio de pista. Soltar antes no
	// reabre nada: la guarda contra escrituras repetidas es la copia en memoria
	// de la cola, que ya quedó actualizada arriba bajo el lock, y el UPDATE es
	// por ruta, así que da igual qué esté sonando cuando llegue.
	// Fuera de la biblioteca (pista suelta por ruta) no toca ninguna fila.
	d.lib.SetDuration(path, secs)
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
