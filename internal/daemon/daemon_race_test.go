package daemon

import (
	"path/filepath"
	"testing"
	"time"

	"maly/internal/ipc"
)

// Estos tests fijan las DOS guardas de advance() contra una promesa obsoleta.
//
// El problema que cubren: resolveEnd() (player.go) captura `chained` bajo p.mu
// y despacha el callback con `go` SIN sostener ningún lock. advance() recién
// toma d.mu después, así que en esa ventana cualquier dispatch puede ganar el
// mutex, mutar la cola y/o recargar mpv. `chained` llega obsoleto.
//
//	guarda de GENERACIÓN (gen != d.pl.LoadGen()) → un cliente RECARGÓ mpv
//	    con loadfile replace: jump, play, next, prev, stop.
//	guarda de RUTA (chained != PeekNext().Path)  → una mutación cambió la
//	    promesa SIN recargar: move, shuffle, remove de una pista posterior.
//
// Hacen falta las dos y no se solapan: la de ruta compara contra la COLA, no
// contra lo que mpv reproduce, así que un `jump` al índice que ya era el actual
// deja PeekNext idéntico y matchea por coincidencia (ahí entra la de
// generación); y un `move` no toca loadGen, así que la de generación lo deja
// pasar (ahí entra la de ruta).
//
// La intercalación se fuerza a mano —el mutador primero, advance() después— en
// vez de correr las dos cosas de verdad en paralelo: lo que se prueba es qué
// hace advance() con un desenlace obsoleto, no si el planificador de Go llega a
// producir ese orden. Que llegue a producirlo es propiedad del código, no del
// test: no hay lock sostenido a través del handoff y `chained` viaja por valor.
//
// OJO al revertir para verificar: la firma de advance() lleva `gen`, así que un
// revert que la quite NO COMPILA, y eso es señal DÉBIL. Hay que neutralizar
// SOLO el cuerpo del chequeo de generación, dejando la firma en su lugar.

// waitPath pollea hasta que mpv reporte path como el archivo cargado. La
// propiedad va rezagada durante una transición de pista, de ahí el poll (mismo
// patrón que TestGaplessEncadena).
func waitPath(t *testing.T, d *Daemon, path, what string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if d.pl.CurrentPath() == path {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s: CurrentPath = %q, quería %q", what, d.pl.CurrentPath(), path)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// waitPlaylist pollea la cantidad de entradas de la playlist de mpv (la
// ventana gapless: 2 = actual + promesa anexada).
func waitPlaylist(t *testing.T, d *Daemon, want int, what string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		n, err := d.pl.PlaylistCount()
		if err == nil && n == want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s: playlist-count = %d (err %v), quería %d", what, n, err, want)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// raceSetup deja el escenario común: cola [a b c] con a sonando y b anexada.
// secs son las duraciones de los tres WAV (a corta cuando el test necesita que
// termine sola). Devuelve el demonio y las tres rutas.
func raceSetup(t *testing.T, secs [3]int) (*Daemon, string, string, string) {
	t.Helper()
	d := newTestDaemon(t)
	music := t.TempDir()
	a := filepath.Join(music, "a.wav")
	b := filepath.Join(music, "b.wav")
	c := filepath.Join(music, "c.wav")
	for i, p := range []string{a, b, c} {
		writeWAV(t, p, secs[i])
	}
	for _, p := range []string{a, b, c} {
		if resp := d.Do(ipc.Request{Cmd: "add", Query: p}); !resp.OK {
			t.Fatalf("add %s: %s", p, resp.Error)
		}
	}
	waitStatus(t, d, "a sonando", func(st *ipc.Status) bool {
		return st.Playing && st.Track != nil && st.Track.Path == a
	})
	waitPlaylist(t, d, 2, "ventana con b anexada")
	return d, a, b, c
}

// checkSync exige el invariante: q.Current() == lo que mpv reproduce.
func checkSync(t *testing.T, d *Daemon, what string) {
	t.Helper()
	real := d.pl.CurrentPath()
	cur, ok := d.q.Current()
	if !ok {
		t.Fatalf("%s: la cola no tiene pista actual, pero mpv reproduce %q", what, real)
	}
	if cur.Path != real {
		t.Errorf("%s: DESINCRONÍA — la cola dice %q (Index=%d) y mpv reproduce %q",
			what, cur.Path, d.q.Index, real)
	}
}

// TestAdvanceObsoletoTrasJump es el caso que motivó el arreglo. `jump 0` salta
// al índice que YA era el actual: recarga mpv con loadfile replace pero deja
// Index=0, así que PeekNext sigue devolviendo b — la MISMA ruta que el chained
// obsoleto, y la guarda de ruta matchea por coincidencia. Quien tiene que
// rechazarlo es la guarda de generación.
func TestAdvanceObsoletoTrasJump(t *testing.T) {
	d, a, b, _ := raceSetup(t, [3]int{30, 30, 30})

	// Lo que resolveEnd() habría capturado al encadenar mpv a b.
	chained, gen := b, d.pl.LoadGen()

	if resp := d.Do(ipc.Request{Cmd: "jump", Index: 0}); !resp.OK {
		t.Fatalf("jump: %s", resp.Error)
	}
	waitPath(t, d, a, "tras jump 0")
	waitPlaylist(t, d, 2, "ventana rearmada tras el jump")

	d.advance("eof", chained, gen)

	checkSync(t, d, "tras jump 0 + advance obsoleto")
	if d.q.Index != 0 {
		t.Errorf("Index = %d, quería 0: mpv volvió a cargar %q, la promesa encadenada quedó anulada y no hay nada que confirmar",
			d.q.Index, a)
	}
}

// TestAdvanceObsoletoSalteaPista es el daño observable del caso anterior: sin
// el arreglo, b no llega a sonar NUNCA. El syncWindowLocked del final de
// advance rearma la ventana desde el Index ya corrido —SetNext(c)— y saca a b
// de la playlist de mpv, así que al terminar a mpv encadena directo a c.
func TestAdvanceObsoletoSalteaPista(t *testing.T) {
	d, a, b, c := raceSetup(t, [3]int{2, 30, 30}) // a corta: tiene que terminar sola

	chained, gen := b, d.pl.LoadGen()

	if resp := d.Do(ipc.Request{Cmd: "jump", Index: 0}); !resp.OK {
		t.Fatalf("jump: %s", resp.Error)
	}
	waitPath(t, d, a, "tras jump 0")
	waitPlaylist(t, d, 2, "ventana rearmada")

	d.advance("eof", chained, gen)

	// Observar qué reproduce mpv DESPUÉS de a. El corte es la primera de las
	// dos que aparezca, y por eso el test sirve en ambas direcciones sin
	// depender de las duraciones: arreglado se ve b enseguida; con el bug se
	// ve c enseguida (b ya salió de la playlist de mpv) y b no aparece nunca.
	seen := map[string]bool{}
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) && !seen[b] && !seen[c] {
		if p := d.pl.CurrentPath(); p != "" {
			seen[p] = true
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !seen[b] && !seen[c] {
		t.Fatalf("a no dio paso a ninguna otra pista: el test no observó la transición (vistas: a=%v)", seen[a])
	}
	if !seen[b] {
		t.Errorf("PISTA PERDIDA: mpv pasó de a a c sin reproducir b nunca (vistas: a=%v b=%v c=%v)",
			seen[a], seen[b], seen[c])
	}
}

// TestAdvanceObsoletoCorrompeDuracion: sin el arreglo la desincronía no se
// queda en memoria. notify() llama a learnDuration() de entrada, que toma la
// pista de q.Current() (=b) y la duración de d.pl.State() (=la de a) y termina
// en d.lib.SetDuration(b, duraciónDeA) — fuera de d.mu, directo a SQLite.
func TestAdvanceObsoletoCorrompeDuracion(t *testing.T) {
	// Duraciones bien distintas: si la fila de b termina con la de a, no hay
	// ambigüedad posible.
	d, a, b, _ := raceSetup(t, [3]int{3, 17, 25})

	// Escanear: SetDuration no toca ninguna fila para una pista suelta por
	// ruta, así que la corrupción solo es observable con la pista indexada.
	if resp := d.Do(ipc.Request{Cmd: "scan", Query: filepath.Dir(a)}); !resp.OK {
		t.Fatalf("scan: %s", resp.Error)
	}

	chained, gen := b, d.pl.LoadGen()

	if resp := d.Do(ipc.Request{Cmd: "jump", Index: 0}); !resp.OK {
		t.Fatalf("jump: %s", resp.Error)
	}
	waitPath(t, d, a, "tras jump 0")
	waitPlaylist(t, d, 2, "ventana rearmada")
	// learnDuration corta si st.Duration <= 0. Sin esperar a que mpv publique
	// la duración, el test mide su propia prisa y da un falso negativo.
	st := waitStatus(t, d, "duración de a publicada", func(st *ipc.Status) bool {
		return st.Duration > 0
	})

	d.advance("eof", chained, gen)
	d.notify()

	row, ok := d.lib.ByPath(b)
	if !ok {
		t.Fatal("b no está en la biblioteca tras el scan")
	}
	if row.Duration > st.Duration-0.5 && row.Duration < st.Duration+0.5 {
		t.Errorf("DURACIÓN CORRUPTA EN SQLITE: la fila de b quedó en %.2f s, que es la duración de a (%.2f s); b dura 17 s",
			row.Duration, st.Duration)
	}
}

// TestAdvanceObsoletoTrasNext: `next` también recarga mpv (loadLocked →
// pl.Load), así que lo ataja la guarda de generación igual que al jump. La
// diferencia con aquel es que acá next YA dejó Index en 1, o sea que un
// advance que avanzara otra vez saltearía c.
func TestAdvanceObsoletoTrasNext(t *testing.T) {
	d, _, b, _ := raceSetup(t, [3]int{30, 30, 30})

	chained, gen := b, d.pl.LoadGen()

	if resp := d.Do(ipc.Request{Cmd: "next"}); !resp.OK {
		t.Fatalf("next: %s", resp.Error)
	}
	waitPath(t, d, b, "tras next")

	d.advance("eof", chained, gen)

	checkSync(t, d, "tras next + advance obsoleto")
	if d.q.Index != 1 {
		t.Errorf("Index = %d, quería 1: next ya había avanzado, advance no debe avanzar otra vez", d.q.Index)
	}
}

// TestAdvanceObsoletoTrasMove ejercita la guarda de RUTA con la generación
// intacta: `move` reordena la cola SIN emitir ningún loadfile replace, así que
// loadGen no se mueve y el chequeo nuevo deja pasar el desenlace. Quien tiene
// que rechazarlo es la comparación contra PeekNext. Sin este test, tras el
// arreglo la guarda de generación ataparía todos los casos y la de ruta
// quedaría sin nadie que la ejerza.
func TestAdvanceObsoletoTrasMove(t *testing.T) {
	d, a, b, _ := raceSetup(t, [3]int{30, 30, 30})

	chained, gen := b, d.pl.LoadGen()

	// [a b c] → [a c b]: la promesa pasa de b a c, y mpv sigue reproduciendo a
	// (move no recarga nada).
	if resp := d.Do(ipc.Request{Cmd: "move", Index: 1, To: 2}); !resp.OK {
		t.Fatalf("move: %s", resp.Error)
	}
	if got := d.pl.LoadGen(); got != gen {
		t.Fatalf("move cambió loadGen (%d → %d): este test dejaría de probar la guarda de ruta", gen, got)
	}

	d.advance("eof", chained, gen)

	checkSync(t, d, "tras move + advance obsoleto")
	if d.q.Index != 0 {
		t.Errorf("Index = %d, quería 0: la promesa anexada ya no es la de la cola, avanzar saltearía una pista", d.q.Index)
	}
	if cur, _ := d.q.Current(); cur.Path != a {
		t.Errorf("la actual es %q, quería %q", cur.Path, a)
	}
}

// TestAdvanceGeneracionVigenteAvanza fija que el arreglo no rompe el camino
// feliz: con la generación vigente y la promesa intacta, advance CONFIRMA el
// encadenado. No usa checkSync a propósito — el test simula el desenlace sin
// que mpv haya encadenado de verdad, así que solo la cola tiene que moverse.
// El camino feliz completo con mpv real ya lo cubre TestGaplessEncadena.
func TestAdvanceGeneracionVigenteAvanza(t *testing.T) {
	d, _, b, _ := raceSetup(t, [3]int{30, 30, 30})

	d.advance("eof", b, d.pl.LoadGen())

	if d.q.Index != 1 {
		t.Errorf("Index = %d, quería 1: sin nada que invalide el desenlace, advance debe confirmar el encadenado", d.q.Index)
	}
	if cur, _ := d.q.Current(); cur.Path != b {
		t.Errorf("la actual es %q, quería %q", cur.Path, b)
	}
}
