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
}

func New() *Queue {
	return &Queue{Index: -1, Repeat: RepeatOff}
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
}

// Clear vacía la cola.
func (q *Queue) Clear() {
	q.Items = nil
	q.Index = -1
	q.history = nil
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
	return q.Items[i], true
}

// Next avanza según shuffle/repeat. ok=false significa fin de la cola.
func (q *Queue) Next(natural bool) (library.Track, bool) {
	n := len(q.Items)
	if n == 0 {
		return library.Track{}, false
	}
	// repeat one solo aplica al avance natural (fin de pista), no a `maly next`.
	if natural && q.Repeat == RepeatOne && q.Index >= 0 {
		return q.Items[q.Index], true
	}
	if q.Shuffle {
		if q.Index >= 0 {
			q.history = append(q.history, q.Index)
			if len(q.history) > n*2 {
				q.history = q.history[len(q.history)-n:]
			}
		}
		if n == 1 {
			q.Index = 0
		} else {
			next := rand.Intn(n - 1)
			if next >= q.Index {
				next++
			}
			q.Index = next
		}
		return q.Items[q.Index], true
	}
	if q.Index+1 >= n {
		if q.Repeat == RepeatAll {
			q.Index = 0
			return q.Items[0], true
		}
		return library.Track{}, false
	}
	q.Index++
	return q.Items[q.Index], true
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
