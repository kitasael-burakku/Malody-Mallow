package queue

import (
	"crypto/rand"
	"slices"

	"maly/internal/library"
)

// removal guarda UNA entrada y sus lugares, nunca una foto de Items.
// Los mutadores invalidan el registro antes de que esos lugares envejezcan.
type removal struct {
	track                 library.Track
	index, slot, priority int
	current               bool
	token                 string
}

func (q *Queue) rememberRemoval(i int) {
	q.removed = &removal{
		track: q.Items[i], index: i, slot: q.orderPos(i),
		priority: slices.Index(q.pending, i), current: i == q.Index,
		// No se reutiliza tras reiniciar: un cliente con una foto vieja no puede
		// deshacer por accidente una eliminación del nuevo demonio.
		token: rand.Text(),
	}
}

// DiscardUndo invalida sin cambiar Items ni su generación.
func (q *Queue) DiscardUndo() { q.removed = nil }

// UndoToken identifica la eliminación disponible; vacío significa ninguna.
func (q *Queue) UndoToken() string {
	if q.removed == nil {
		return ""
	}
	return q.removed.token
}

// UndoRemove reinserta solo la última entrada quitada. Un token vacío pide la
// última vigente (CLI/consola); la TUI manda siempre el que observó.
// Un token viejo falla SIN descartar un registro más reciente.
func (q *Queue) UndoRemove(token string) (library.Track, bool) {
	r := q.removed
	if r == nil || (token != "" && token != r.token) {
		return library.Track{}, false
	}
	q.DiscardUndo()
	q.Items = slices.Insert(q.Items, r.index, r.track)
	if q.Index >= r.index {
		q.Index++
	}
	for k, i := range q.pending {
		if i >= r.index {
			q.pending[k]++
		}
	}
	if r.priority >= 0 {
		q.pending = slices.Insert(q.pending, r.priority, r.index)
	}
	if q.Shuffle {
		for k, i := range q.order {
			if i >= r.index {
				q.order[k]++
			}
		}
		slot := r.slot
		if r.current {
			// Quitar la actual ya eligió otra: no rebobinar su reproducción ni el
			// ciclo. La recuperada queda consumida, justo antes de la actual nueva.
			slot = max(0, q.pos)
		}
		q.order = slices.Insert(q.order, slot, r.index)
		if q.pos >= slot {
			q.pos++
		}
	}
	q.staged = nil
	q.gen++
	return r.track, true
}
