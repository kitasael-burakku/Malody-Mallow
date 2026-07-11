// Package queue implementa la cola de reproducción con shuffle y repeat.
// No es thread-safe: el demonio la protege con su propio mutex.
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
	Index   int // pista actual; -1 si nada seleccionado
	Shuffle bool
	Repeat  RepeatMode
	history []int // índices ya sonados en modo shuffle, para prev
	peeked  int   // sorteo de shuffle prometido por PeekNext; -1 = ninguno
}

func New() *Queue {
	return &Queue{Index: -1, Repeat: RepeatOff, peeked: -1}
}

func (q *Queue) Len() int { return len(q.Items) }

// Current devuelve la pista actual, si la hay.
func (q *Queue) Current() (library.Track, bool) {
	if q.Index < 0 || q.Index >= len(q.Items) {
		return library.Track{}, false
	}
	return q.Items[q.Index], true
}

// Add agrega pistas al final.
func (q *Queue) Add(tracks ...library.Track) {
	q.Items = append(q.Items, tracks...)
}

// Replace sustituye la cola entera y se coloca al inicio.
func (q *Queue) Replace(tracks []library.Track) {
	q.Items = append([]library.Track(nil), tracks...)
	q.Index = -1
	q.history = nil
	q.peeked = -1
}

// Clear vacía la cola.
func (q *Queue) Clear() {
	q.Items = nil
	q.Index = -1
	q.history = nil
	q.peeked = -1
}

// RemoveAt quita la pista en la posición i. Devuelve true si la pista
// eliminada era la actual (el llamador decide qué reproducir después).
func (q *Queue) RemoveAt(i int) bool {
	if i < 0 || i >= len(q.Items) {
		return false
	}
	wasCurrent := i == q.Index
	q.Items = append(q.Items[:i], q.Items[i+1:]...)
	q.history = nil
	q.peeked = -1
	if i < q.Index {
		q.Index--
	} else if wasCurrent {
		if q.Index >= len(q.Items) {
			q.Index = len(q.Items) - 1
		}
	}
	return wasCurrent
}

// JumpTo selecciona la pista i como actual.
func (q *Queue) JumpTo(i int) (library.Track, bool) {
	if i < 0 || i >= len(q.Items) {
		return library.Track{}, false
	}
	if q.Shuffle && q.Index >= 0 {
		q.history = append(q.history, q.Index)
	}
	q.Index = i
	q.peeked = -1
	return q.Items[i], true
}

// PeekNext devuelve la pista que el avance natural (fin de pista) va a
// elegir, sin avanzar: es la promesa que el demonio anexa a la playlist de
// mpv para el encadenado gapless. El sorteo de shuffle se decide aquí y
// Next lo honra; mutar la cola invalida la promesa.
func (q *Queue) PeekNext() (library.Track, bool) {
	i, ok := q.nextIndex(true)
	if !ok {
		return library.Track{}, false
	}
	return q.Items[i], true
}

// Invalidate descarta la promesa vigente de PeekNext. Las mutaciones de la
// cola invalidan solas; esto es para los cambios de Shuffle/Repeat, campos
// públicos que el demonio toca directo.
func (q *Queue) Invalidate() { q.peeked = -1 }

// nextIndex calcula el índice al que se avanzaría. natural aplica repeat
// one (el next manual lo ignora); en shuffle el sorteo se recuerda en
// peeked hasta consumirse o invalidarse, para que la promesa anexada a mpv
// y el avance real coincidan.
func (q *Queue) nextIndex(natural bool) (int, bool) {
	n := len(q.Items)
	if n == 0 {
		return 0, false
	}
	if natural && q.Repeat == RepeatOne && q.Index >= 0 {
		return q.Index, true
	}
	if q.Shuffle {
		if q.peeked < 0 || q.peeked >= n {
			if n == 1 {
				q.peeked = 0
			} else {
				next := rand.Intn(n - 1)
				if next >= q.Index {
					next++
				}
				q.peeked = next
			}
		}
		return q.peeked, true
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
	q.peeked = -1
	if !ok {
		return library.Track{}, false
	}
	if q.Shuffle && q.Index >= 0 && i != q.Index {
		q.history = append(q.history, q.Index)
		if n := len(q.Items); len(q.history) > n*2 {
			q.history = q.history[len(q.history)-n:]
		}
	}
	q.Index = i
	return q.Items[i], true
}

// Prev retrocede (en shuffle usa el historial).
func (q *Queue) Prev() (library.Track, bool) {
	if len(q.Items) == 0 {
		return library.Track{}, false
	}
	if q.Shuffle && len(q.history) > 0 {
		q.Index = q.history[len(q.history)-1]
		q.history = q.history[:len(q.history)-1]
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
