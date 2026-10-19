package queue

import "maly/internal/library"

// AddNext inserta entradas NUEVAS, en FIFO, después de las ya solicitadas
// (o de la actual). No cambia la pista actual ni inicia reproducción.
// La prioridad es por entrada: pedir dos veces una ruta crea dos entradas.
func (q *Queue) AddNext(tracks ...library.Track) {
	if len(tracks) == 0 {
		return
	}
	at := q.Index + 1
	if len(q.pending) > 0 {
		at = q.pending[len(q.pending)-1] + 1
	}
	n := len(tracks)
	q.Items = append(q.Items, make([]library.Track, n)...)
	copy(q.Items[at+n:], q.Items[at:len(q.Items)-n])
	copy(q.Items[at:], tracks)
	if q.Index >= at {
		q.Index += n
	}
	for i, v := range q.pending {
		if v >= at {
			q.pending[i] += n
		}
	}
	for i, v := range q.order {
		if v >= at {
			q.order[i] += n
		}
	}
	for i := at; i < at+n; i++ {
		q.pending = append(q.pending, i)
		if q.Shuffle {
			q.order = append(q.order, i)
		}
	}
	q.staged = nil
	q.gen++
}

// Pending devuelve una copia para persistencia. No expone memoria mutable.
func (q *Queue) Pending() []int { return append([]int(nil), q.pending...) }

// NextPosition es 1-based para la presentación; cero significa sin prioridad.
func (q *Queue) NextPosition() int {
	if len(q.pending) == 0 {
		return 0
	}
	return q.pending[0] + 1
}

func (q *Queue) PendingCount() int { return len(q.pending) }

// RestorePending descarta índices corruptos, repetidos y la entrada actual.
// Los índices ya deben estar remapeados tras quitar archivos desaparecidos.
func (q *Queue) RestorePending(indices []int) {
	q.pending = nil
	seen := make(map[int]bool, len(indices))
	for _, i := range indices {
		if i >= 0 && i < len(q.Items) && i != q.Index && !seen[i] {
			q.pending = append(q.pending, i)
			seen[i] = true
		}
	}
	q.staged = nil
}

func (q *Queue) consumePending(index int) {
	for i, v := range q.pending {
		if v == index {
			q.pending = append(q.pending[:i], q.pending[i+1:]...)
			return
		}
	}
}
