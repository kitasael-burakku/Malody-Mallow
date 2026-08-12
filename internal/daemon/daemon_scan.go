package daemon

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"maly/internal/i18n"
	"maly/internal/ipc"
	"maly/internal/probe"
)

// Fase de escaneo de la biblioteca: corre SIN d.mu (library serializa
// sus sentencias en su única conexión SQLite) para que play/status sigan
// respondiendo durante un scan largo.

// scan (re)indexa dir sin tomar d.mu: library serializa sus sentencias en su
// única conexión SQLite, y así play/status siguen respondiendo durante el
// escaneo. Solo se permite un escaneo a la vez.
func (d *Daemon) scan(lang, query string) ipc.Response {
	if !d.scanning.CompareAndSwap(false, true) {
		return ipc.Response{Error: i18n.TL(lang, "d.scan_busy")}
	}
	// Al terminar, despertar SIEMPRE a los suscriptores (aun sin cambios o
	// con error) para que limpien el "escaneando…" de su Status — y hacerlo
	// DESPUÉS de bajar scanning, o el push final aún lo reportaría en true.
	defer func() {
		d.scanning.Store(false)
		d.wakeSubs()
	}()

	dir, origin, explicit := d.cfg.ScanTarget(query)
	d.scanSeen.Store(0)
	d.scanTotal.Store(0)
	// Los suscriptores ven el avance en Status (Scanning/ScanSeen); el dirty
	// con cap 1 y el mínimo de 250 ms entre pushes ya colapsan la avalancha.
	res, err := d.lib.Scan(dir, func(seen int) {
		d.scanSeen.Store(int64(seen))
		d.wakeSubs()
	})
	if err != nil {
		if !explicit && errors.Is(err, fs.ErrNotExist) {
			return ipc.Response{Error: i18n.TLf(lang, "cli.scan_noexist", dir, i18n.TL(lang, origin))}
		}
		return ipc.Response{Error: err.Error()}
	}
	// Segunda fase: las duraciones que el indexado no puede saber (los tags
	// no las traen). Sigue fuera de d.mu y con la misma atómica scanning,
	// así que un scan concurrente sigue rebotando con d.scan_busy.
	learned, dfailed := 0, 0
	var dqerr error
	if d.cfg.ScanDurations && probe.Available() {
		d.scanSeen.Store(0)
		learned, dfailed, dqerr = d.lib.FillDurations(dir, probe.Duration, func(done, total int) {
			d.scanSeen.Store(int64(done))
			d.scanTotal.Store(int64(total))
			d.wakeSubs()
		})
		d.refreshQueueDurations(learned)
	}

	total, _ := d.lib.Count()
	if res.Added+res.Updated+res.Removed > 0 || learned > 0 {
		// La biblioteca cambió de generación (handle trata scan como
		// solo-lectura y no la subiría). Un scan sin cambios no recarga el
		// árbol de nadie; el wakeSubs va en el defer de arriba.
		d.libGen.Add(1)
	}
	msg := i18n.TLf(lang, "d.scan_done", res.Added, res.Updated, res.Removed, total)
	if learned > 0 {
		msg += i18n.TLf(lang, "d.dur_done", learned)
	}
	if dfailed > 0 {
		// Un archivo con extensión de audio que ffprobe no sabe leer es un
		// caso normal y ruidoso: solo el conteo, sin volcar rutas al stderr
		// como hace el indexado.
		msg += i18n.TLf(lang, "d.dur_errs", dfailed)
	}
	if dqerr != nil {
		// La consulta que arma los candidatos de FillDurations falló (DB
		// bloqueada, corrupta, I/O): antes esto se descartaba con `_ =` y la
		// fase quedaba en "0 aprendidas, 0 fallidas" sin ninguna pista de
		// que ni siquiera pudo empezar (hallazgo PERF-01 de la auditoría
		// técnica).
		msg += i18n.TLf(lang, "d.dur_query_failed", dqerr)
	}
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

// refreshQueueDurations trae a la cola en memoria las duraciones que el
// relleno acaba de aprender. Hace falta porque learnDuration compara contra
// la cola, no contra la base: sin esto los items ya cargados seguirían en 0
// (y el panel de cola de la TUI sin duración) hasta que cada pista sonara.
//
// Las lecturas de la biblioteca van FUERA de d.mu, como la resolución de
// pistas de play/add; el mutex solo se toma para mirar qué falta y para
// aplicar. El emparejamiento es por ruta porque los índices pueden haber
// cambiado mientras leíamos.
func (d *Daemon) refreshQueueDurations(learned int) {
	if learned == 0 {
		return
	}
	d.mu.Lock()
	var want []string
	for _, t := range d.q.Items {
		if t.Duration <= 0 {
			want = append(want, t.Path)
		}
	}
	d.mu.Unlock()
	if len(want) == 0 {
		return
	}

	found := make(map[string]float64, len(want))
	for _, p := range want {
		if t, ok := d.lib.ByPath(p); ok && t.Duration > 0 {
			found[p] = t.Duration
		}
	}
	if len(found) == 0 {
		return
	}

	d.mu.Lock()
	for i := range d.q.Items {
		if secs, ok := found[d.q.Items[i].Path]; ok && d.q.Items[i].Duration <= 0 {
			d.q.Items[i].Duration = secs
		}
	}
	d.mu.Unlock()
	d.wakeSubs()
}
