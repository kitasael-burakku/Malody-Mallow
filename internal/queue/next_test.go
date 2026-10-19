package queue

import (
	"fmt"
	"maly/internal/library"
	"reflect"
	"testing"
)

func named(names ...string) []library.Track {
	var out []library.Track
	for _, n := range names {
		out = append(out, library.Track{Path: n, Title: n})
	}
	return out
}

func TestPlayNextFIFO(t *testing.T) {
	for _, shuffle := range []bool{false, true} {
		for _, repeat := range []RepeatMode{RepeatOff, RepeatAll, RepeatOne} {
			t.Run(fmt.Sprintf("shuffle=%v/repeat=%s", shuffle, repeat), func(t *testing.T) {
				q := New()
				q.Replace(named("A", "D", "E"))
				q.JumpTo(0)
				q.SetShuffle(shuffle)
				q.Repeat = repeat
				oldGen := q.Gen()
				q.PeekNext()
				q.AddNext(named("B")...)
				q.AddNext(named("C", "C")...)
				if tr, _ := q.Current(); tr.Path != "A" {
					t.Fatal("interrumpió la actual")
				}
				if q.Gen() != oldGen+2 {
					t.Fatal("generación de contenido incorrecta")
				}
				for _, want := range []string{"B", "C", "C"} {
					if tr, ok := q.PeekNext(); !ok || tr.Path != want {
						t.Fatalf("promesa FIFO: %v %v, quería %s", tr, ok, want)
					}
					tr, ok := q.Next(true)
					if !ok || tr.Path != want {
						t.Fatal("no consumió la promesa", tr, ok)
					}
					checkOrder(t, q)
				}
				if q.PendingCount() != 0 {
					t.Fatal("prioridad no consumida")
				}
				if repeat == RepeatOne {
					if tr, _ := q.Next(true); tr.Path != "C" {
						t.Fatal("no volvió a repeat-one")
					}
					return
				}
				seen := map[string]bool{}
				for range 2 {
					tr, ok := q.Next(true)
					if !ok || seen[tr.Path] || tr.Path == "A" || tr.Path == "B" || tr.Path == "C" {
						t.Fatal("repitió o perdió entradas del ciclo", tr, ok)
					}
					seen[tr.Path] = true
					checkOrder(t, q)
				}
				if repeat == RepeatOff {
					if _, ok := q.Next(true); ok {
						t.Fatal("no terminó el ciclo")
					}
				}
			})
		}
	}
}

func TestPlayNextMutations(t *testing.T) {
	for _, shuffle := range []bool{false, true} {
		q := New()
		q.Replace(named("A", "D", "E"))
		q.JumpTo(0)
		q.SetShuffle(shuffle)
		q.AddNext(named("B", "C")...)
		// A B C D E -> D A B C E. La identidad pendiente sigue las entradas.
		q.Move(3, 0)
		q.RemoveAt(0)
		q.Add(named("F")...)
		q.SetShuffle(!shuffle)
		q.SetShuffle(shuffle)
		if !reflect.DeepEqual(q.Pending(), []int{1, 2}) {
			t.Fatal(q.Pending())
		}
		q.RemoveAt(1) // B cancelada, C queda próxima.
		if q.PendingCount() != 1 {
			t.Fatal("remove dejó una referencia pendiente")
		}
		if tr, _ := q.Next(false); tr.Path != "C" {
			t.Fatal("remove perdió la siguiente", tr)
		}
		checkOrder(t, q)
		q.AddNext(named("G", "H")...)
		q.JumpTo(q.Pending()[1])
		if tr, _ := q.PeekNext(); tr.Path != "G" {
			t.Fatal("salto explícito alteró otras pendientes")
		}
		q.Clear()
		if q.PendingCount() != 0 {
			t.Fatal("clear conservó prioridad")
		}
		q.AddNext(named("I")...)
		q.Replace(named("J"))
		if q.PendingCount() != 0 {
			t.Fatal("replace conservó prioridad")
		}
	}
}

func TestPlayNextEmptyAndRestore(t *testing.T) {
	q := New()
	q.SetShuffle(true)
	q.AddNext(named("B", "C")...)
	if _, ok := q.Current(); ok {
		t.Fatal("seleccionó sin permiso")
	}
	saved := q.Pending()
	saved[0] = 99
	if q.NextPosition() != 1 {
		t.Fatal("expuso el slice interno")
	}
	q.RestorePending([]int{-1, 1, 1, 8, 0})
	if !reflect.DeepEqual(q.Pending(), []int{1, 0}) {
		t.Fatal("restore no validó índices", q.Pending())
	}
	if tr, _ := q.Next(true); tr.Path != "C" {
		t.Fatal(tr)
	}
	checkOrder(t, q)
	q.RestorePending([]int{q.Index, 0})
	if !reflect.DeepEqual(q.Pending(), []int{0}) {
		t.Fatal("restore aceptó la actual")
	}
}

func TestPlayNextRemoveCurrentAndMovedPending(t *testing.T) {
	for _, shuffle := range []bool{false, true} {
		q := New()
		q.Replace(named("A", "D", "E"))
		q.JumpTo(0)
		q.SetShuffle(shuffle)
		q.AddNext(named("B", "C")...)
		q.Move(1, 4) // la prioridad sigue a B aunque su fila se mueva al final.
		if tr, _ := q.PeekNext(); tr.Path != "B" {
			t.Fatal("move perdió prioridad")
		}
		q.AddNext(named("F")...)
		for _, want := range []string{"B", "C", "F"} {
			tr, _ := q.Next(true)
			if tr.Path != want {
				t.Fatal("FIFO tras move", tr, want)
			}
			checkOrder(t, q)
		}
		q.Replace(named("A", "D"))
		q.JumpTo(0)
		q.AddNext(named("B", "C")...)
		q.RemoveAt(0)
		checkOrder(t, q)
		if tr, _ := q.Current(); tr.Path != "B" {
			t.Fatal("remove actual no seleccionó B")
		}
		if tr, _ := q.Next(true); tr.Path != "C" {
			t.Fatal("repitió la nueva actual")
		}
	}
}
