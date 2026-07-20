// Package queue implementa la cola de reproducción con shuffle y repeat.
// No es thread-safe: el demonio la protege con su propio mutex.
//
// El shuffle es por permutación: al activarse se genera un orden aleatorio
// de toda la cola (order) y se avanza por él, así ninguna pista se repite
// hasta agotar el ciclo; con repeat all, al agotarse se genera una
// permutación nueva. Invariante con Shuffle activo: len(order)==len(Items),
// order es permutación de 0..n-1, y si Index >= 0 entonces
// order[pos] == Index. Los mutadores lo mantienen con cirugía incremental
// (no hay auto-curación: un order desalineado es un bug que debe explotar
// en tests, no esconderse).
package queue

import (
	"math/rand"

	"maly/internal/library"
)

type RepeatMode string

const (
	RepeatOff RepeatMode = "off"
	RepeatAll RepeatMode = "all"
	RepeatOne RepeatMode = "one"
)

type Queue struct {
	Items   []library.Track
	Index   int  // pista actual; -1 si nada seleccionado
	Shuffle bool // solo lectura desde fuera; mutar SIEMPRE vía SetShuffle
	Repeat  RepeatMode
	order   []int // permutación del ciclo shuffle; nil con Shuffle off
	pos     int   // posición de la actual en order; -1 si nada suena
	staged  []int // permutación del ciclo siguiente, prometida por PeekNext
}

func New() *Queue {
	return &Queue{Index: -1, Repeat: RepeatOff, pos: -1}
}

func (q *Queue) Len() int { return len(q.Items) }

// Current devuelve la pista actual, si la hay.
func (q *Queue) Current() (library.Track, bool) {
	if q.Index < 0 || q.Index >= len(q.Items) {
		return library.Track{}, false
	}
	return q.Items[q.Index], true
}

// SetShuffle enciende o apaga el shuffle regenerando o soltando la
// permutación. Es el ÚNICO camino válido para cambiar q.Shuffle; idempotente
// (reactivarlo conserva el ciclo en curso).
func (q *Queue) SetShuffle(on bool) {
	if on == q.Shuffle {
		return
	}
	q.Shuffle = on
	if on {
		q.reshuffle()
	} else {
		q.order, q.pos, q.staged = nil, -1, nil
	}
}

// reshuffle arma una permutación nueva alrededor del estado actual.
func (q *Queue) reshuffle() {
	q.staged = nil
	n := len(q.Items)
	if n == 0 {
		q.order, q.pos = nil, -1
		return
	}
	if q.Index < 0 {
		// Nada suena: permutación completa; el primer avance da order[0],
		// así el índice 0 puede salir (la exclusión de la "actual" con
		// Index -1 lo dejaba fuera para siempre).
		q.order, q.pos = rand.Perm(n), -1
		return
	}
	// La actual encabeza el ciclo; el resto, permutado detrás.
	q.order = append(make([]int, 0, n), q.Index)
	for _, v := range rand.Perm(n) {
		if v != q.Index {
			q.order = append(q.order, v)
		}
	}
	q.pos = 0
}

// nextCycle genera la permutación del ciclo siguiente (wrap de RepeatAll);
// su primera entrada evita repetir la actual en la costura, imposible con
// una sola pista. El swap no es perfectamente uniforme sobre las
// permutaciones restringidas; irrelevante musicalmente.
func (q *Queue) nextCycle() []int {
	p := rand.Perm(len(q.Items))
	if len(p) > 1 && p[0] == q.Index {
		j := 1 + rand.Intn(len(p)-1)
		p[0], p[j] = p[j], p[0]
	}
	return p
}

// orderPos localiza el índice de pista v dentro de order (lineal: n es
// chico y esto solo corre en mutaciones).
func (q *Queue) orderPos(v int) int {
	for i, e := range q.order {
		if e == v {
			return i
		}
	}
	return -1
}

// Add agrega pistas al final. En shuffle cada pista nueva entra en un slot
// aleatorio del tramo NO sonado de la permutación: puede caer en pos+1 y
// cambiar la promesa vigente — a propósito, una pista agregada al final del
// ciclo debe poder sonar; el demonio realinea la ventana tras cada mutador.
func (q *Queue) Add(tracks ...library.Track) {
	start := len(q.Items)
	q.Items = append(q.Items, tracks...)
	if !q.Shuffle {
		return
	}
	for j := start; j < len(q.Items); j++ {
		slot := q.pos + 1 + rand.Intn(len(q.order)-q.pos)
		q.order = append(q.order[:slot], append([]int{j}, q.order[slot:]...)...)
	}
	q.staged = nil
}

// Replace sustituye la cola entera y se coloca al inicio.
func (q *Queue) Replace(tracks []library.Track) {
	q.Items = append([]library.Track(nil), tracks...)
	q.Index = -1
	if q.Shuffle {
		q.reshuffle()
	}
}

// Clear vacía la cola.
func (q *Queue) Clear() {
	q.Items = nil
	q.Index = -1
	q.order, q.pos, q.staged = nil, -1, nil
}

// RemoveAt quita la pista en la posición i. removed distingue el índice
// fuera de rango (nada que quitar) y wasCurrent avisa si la eliminada era la
// actual (el llamador decide qué reproducir después).
func (q *Queue) RemoveAt(i int) (removed, wasCurrent bool) {
	if i < 0 || i >= len(q.Items) {
		return false, false
	}
	wasCurrent = i == q.Index
	q.Items = append(q.Items[:i], q.Items[i+1:]...)
	if i < q.Index {
		q.Index--
	} else if wasCurrent {
		if q.Index >= len(q.Items) {
			q.Index = len(q.Items) - 1
		}
	}
	if q.Shuffle && len(q.order) > 0 {
		p := q.orderPos(i)
		q.order = append(q.order[:p], q.order[p+1:]...)
		for k, e := range q.order {
			if e > i {
				q.order[k] = e - 1
			}
		}
		switch {
		case p < q.pos:
			q.pos--
		case p == q.pos:
			if q.Index >= 0 {
				// Se quitó la actual: recolocar la nueva actual (el Index ya
				// ajustado, que el demonio recarga) como primer slot no
				// sonado, restaurando order[pos] == Index.
				p2 := q.orderPos(q.Index)
				q.order = append(q.order[:p2], q.order[p2+1:]...)
				if p2 < q.pos {
					q.pos--
				}
				q.order = append(q.order[:q.pos], append([]int{q.Index}, q.order[q.pos:]...)...)
			} else {
				q.pos = -1 // la cola quedó vacía
			}
		}
		q.staged = nil
	}
	return true, wasCurrent
}

// Move traslada la pista en from a la posición to; devuelve false si algún
// índice está fuera de rango. Index se ajusta para seguir apuntando a la
// misma pista, y las entradas de order reciben el mismo remapeo: la promesa
// de PeekNext sigue a la pista movida en vez de redibujarse.
func (q *Queue) Move(from, to int) bool {
	n := len(q.Items)
	if from < 0 || from >= n || to < 0 || to >= n {
		return false
	}
	if from == to {
		return true
	}
	t := q.Items[from]
	q.Items = append(q.Items[:from], q.Items[from+1:]...)
	q.Items = append(q.Items[:to], append([]library.Track{t}, q.Items[to:]...)...)
	switch {
	case from == q.Index:
		q.Index = to
	case from < q.Index && to >= q.Index:
		q.Index--
	case from > q.Index && to <= q.Index:
		q.Index++
	}
	for k, e := range q.order {
		switch {
		case e == from:
			q.order[k] = to
		case from < e && to >= e:
			q.order[k] = e - 1
		case from > e && to <= e:
			q.order[k] = e + 1
		}
	}
	q.staged = nil
	return true
}

// JumpTo selecciona la pista i como actual. En shuffle su entrada se
// recoloca como siguiente en la permutación y se consume: uniforme para la
// prometida (se consume tal cual), la actual (no-op) o una ya sonada (se
// re-encola sin duplicarse), y Prev vuelve a la actual anterior.
func (q *Queue) JumpTo(i int) (library.Track, bool) {
	if i < 0 || i >= len(q.Items) {
		return library.Track{}, false
	}
	if q.Shuffle && len(q.order) > 0 {
		p := q.orderPos(i)
		q.order = append(q.order[:p], q.order[p+1:]...)
		if p <= q.pos {
			q.pos--
		}
		q.order = append(q.order[:q.pos+1], append([]int{i}, q.order[q.pos+1:]...)...)
		q.pos++
		q.staged = nil
	}
	q.Index = i
	return q.Items[i], true
}

// PeekNext devuelve la pista que el avance natural (fin de pista) va a
// elegir, sin avanzar: es la promesa que el demonio anexa a la playlist de
// mpv para el encadenado gapless. En shuffle es la siguiente entrada de la
// permutación (estable por construcción); en el wrap de RepeatAll la
// permutación del ciclo siguiente se materializa en staged y Next la honra;
// mutar la cola invalida la promesa.
func (q *Queue) PeekNext() (library.Track, bool) {
	i, ok := q.nextIndex(true)
	if !ok {
		return library.Track{}, false
	}
	return q.Items[i], true
}

// Invalidate descarta la permutación en escena del ciclo siguiente (la
// promesa del wrap). Es para los cambios de Repeat, campo público que el
// demonio escribe directo: repeat one/all cambian qué promete PeekNext,
// pero no el ciclo en curso. Shuffle cambia solo vía SetShuffle.
func (q *Queue) Invalidate() { q.staged = nil }

// nextIndex calcula el índice al que se avanzaría. natural aplica repeat
// one (el next manual lo ignora). En shuffle recorre la permutación; al
// agotarse el ciclo sin RepeatAll se termina (paridad con el secuencial).
func (q *Queue) nextIndex(natural bool) (int, bool) {
	n := len(q.Items)
	if n == 0 {
		return 0, false
	}
	if natural && q.Repeat == RepeatOne && q.Index >= 0 {
		return q.Index, true
	}
	if q.Shuffle {
		if q.pos+1 < len(q.order) {
			return q.order[q.pos+1], true
		}
		if q.Repeat != RepeatAll {
			return 0, false
		}
		if q.staged == nil {
			q.staged = q.nextCycle()
		}
		return q.staged[0], true
	}
	if q.Index+1 >= n {
		if q.Repeat == RepeatAll {
			return 0, true
		}
		return 0, false
	}
	return q.Index + 1, true
}

// Next avanza según shuffle/repeat, consumiendo la promesa de PeekNext.
// natural=true es el fin de pista. ok=false significa fin de la cola.
func (q *Queue) Next(natural bool) (library.Track, bool) {
	i, ok := q.nextIndex(natural)
	if !ok {
		return library.Track{}, false
	}
	// Misma condición que la rama repeat-one de nextIndex: repetir la
	// actual no consume la permutación.
	if q.Shuffle && !(natural && q.Repeat == RepeatOne && q.Index >= 0) {
		if q.pos+1 < len(q.order) {
			q.pos++
		} else {
			// Wrap de RepeatAll: staged pasa a ser el ciclo vigente.
			q.order, q.staged, q.pos = q.staged, nil, 0
		}
	}
	q.Index = i
	return q.Items[i], true
}

// Prev retrocede; en shuffle camina la permutación hacia atrás (paridad con
// el secuencial: con RepeatAll envuelve al final del ciclo, sin él se queda
// al inicio). Al cubrir el ciclo entero, Prev ya no se rompe con las
// mutaciones como el historial viejo.
func (q *Queue) Prev() (library.Track, bool) {
	if len(q.Items) == 0 {
		return library.Track{}, false
	}
	if q.Shuffle {
		switch {
		case q.pos > 0:
			q.pos--
		case q.Repeat == RepeatAll:
			q.pos = len(q.order) - 1
		default:
			q.pos = 0
		}
		q.Index = q.order[q.pos]
		return q.Items[q.Index], true
	}
	if q.Index <= 0 {
		if q.Repeat == RepeatAll {
			q.Index = len(q.Items) - 1
			return q.Items[q.Index], true
		}
		q.Index = 0
		return q.Items[0], true
	}
	q.Index--
	return q.Items[q.Index], true
}

// CycleRepeat rota off → all → one → off.
func (q *Queue) CycleRepeat() RepeatMode {
	switch q.Repeat {
	case RepeatOff:
		q.Repeat = RepeatAll
	case RepeatAll:
		q.Repeat = RepeatOne
	default:
		q.Repeat = RepeatOff
	}
	return q.Repeat
}
