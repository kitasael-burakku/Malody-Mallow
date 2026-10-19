package queue

import (
	"fmt"
	"reflect"
	"testing"
)

func TestUndoRemoveEntry(t *testing.T) {
	for _, shuffle := range []bool{false, true} {
		for current := -1; current < 4; current++ {
			for removed := 0; removed < 4; removed++ {
				t.Run(fmt.Sprintf("shuffle=%v/current=%d/remove=%d", shuffle, current, removed), func(t *testing.T) {
					q := New()
					q.Add(named("igual", "igual", "C", "D")...)
					if current >= 0 {
						q.JumpTo(current)
					}
					q.SetShuffle(shuffle)
					// Metadatos distinguen las entradas que comparten ruta.
					for i := range q.Items {
						q.Items[i].ID = int64(i + 1)
					}
					original := append([]int(nil), q.order...)
					q.RemoveAt(removed)
					now, had := q.Current()
					gen := q.Gen()
					token := q.UndoToken()
					if token == "" {
						t.Fatal("remove no habilitó recuperación")
					}
					if len(q.Items) > 0 {
						q.Items[0].Duration = 81
						q.Touch()
					}
					tr, ok := q.UndoRemove(token)
					if !ok || tr.ID != int64(removed+1) || len(q.Items) != 4 {
						t.Fatal("no recuperó la entrada exacta", tr, ok, q.Items)
					}
					for i, tr := range q.Items {
						if tr.ID != int64(i+1) {
							t.Fatal("orden normal alterado", q.Items)
						}
					}
					got, has := q.Current()
					if has != had || (has && got.ID != now.ID) {
						t.Fatal("undo cambió la actual", now, got)
					}
					other := 0
					if removed == 0 {
						other = 1
					}
					if q.Items[other].Duration != 81 {
						t.Fatal("restauró foto vieja y pisó duración")
					}
					if q.Gen() != gen+2 {
						t.Fatal("generación no refleja Touch e inserción")
					}
					checkOrder(t, q)
					if shuffle && removed != current && !reflect.DeepEqual(q.order, original) {
						t.Fatal("no restauró lugar en shuffle", q.order, original)
					}
					if shuffle && removed == current && q.orderPos(removed) >= q.pos {
						t.Fatal("antigua actual quedó sin consumir")
					}
					if q.UndoToken() != "" {
						t.Fatal("undo quedó disponible dos veces")
					}
					if _, ok = q.UndoRemove(token); ok {
						t.Fatal("duplicó la recuperación")
					}
				})
			}
		}
	}
}

func TestUndoRemovePriorityAndEmpty(t *testing.T) {
	for _, shuffle := range []bool{false, true} {
		q := New()
		q.Add(named("A", "D")...)
		q.JumpTo(0)
		q.SetShuffle(shuffle)
		q.AddNext(named("B", "C", "C")...)
		q.RemoveAt(2)
		if _, ok := q.UndoRemove(q.UndoToken()); !ok {
			t.Fatal("no restauró prioridad")
		}
		if !reflect.DeepEqual(q.Pending(), []int{1, 2, 3}) {
			t.Fatal("FIFO no recuperado", q.Pending())
		}
		for _, want := range []string{"B", "C", "C"} {
			tr, ok := q.Next(true)
			if !ok || tr.Path != want {
				t.Fatal("orden efectivo incorrecto", tr)
			}
		}
		q.Clear()
		q.Add(named("única")...)
		q.JumpTo(0)
		q.RemoveAt(0)
		q.UndoRemove(q.UndoToken())
		if q.Index != -1 || q.Len() != 1 {
			t.Fatal("recuperar única seleccionó audio")
		}
		checkOrder(t, q)
	}
}

func TestUndoRemoveExpiry(t *testing.T) {
	mutations := map[string]func(*Queue){
		"add": func(q *Queue) { q.Add(named("N")...) }, "next insertion": func(q *Queue) { q.AddNext(named("N")...) },
		"replace": func(q *Queue) { q.Replace(named("N")) }, "clear": func(q *Queue) { q.Clear() },
		"move": func(q *Queue) { q.Move(0, 1) }, "jump": func(q *Queue) { q.JumpTo(1) },
		"next": func(q *Queue) { q.Next(false) }, "natural": func(q *Queue) { q.Next(true) }, "prev": func(q *Queue) { q.Prev() },
		"shuffle": func(q *Queue) { q.SetShuffle(true) }, "repeat": func(q *Queue) { q.CycleRepeat() },
		"invalidate": func(q *Queue) { q.Invalidate() }, "stop": func(q *Queue) { q.DiscardUndo() },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			q := New()
			q.Add(named("A", "B", "C")...)
			q.JumpTo(0)
			q.RemoveAt(2)
			token := q.UndoToken()
			mutate(q)
			before := append([]string(nil), paths(q)...)
			if _, ok := q.UndoRemove(token); ok {
				t.Fatal("acción caducada aceptada", name)
			}
			if !reflect.DeepEqual(paths(q), before) {
				t.Fatal("undo caducado alteró la cola")
			}
		})
	}
	q := New()
	q.Add(named("A", "B", "C")...)
	q.RemoveAt(2)
	old := q.UndoToken()
	q.RemoveAt(1)
	fresh := q.UndoToken()
	if fresh == old {
		t.Fatal("token reutilizado")
	}
	if _, ok := q.UndoRemove(old); ok {
		t.Fatal("cliente viejo recuperó otra eliminación")
	}
	if q.UndoToken() != fresh {
		t.Fatal("cliente viejo borró la recuperación nueva")
	}
	if tr, ok := q.UndoRemove(fresh); !ok || tr.Path != "B" {
		t.Fatal("se perdió última eliminación")
	}
}

func paths(q *Queue) []string {
	var out []string
	for _, tr := range q.Items {
		out = append(out, tr.Path)
	}
	return out
}
