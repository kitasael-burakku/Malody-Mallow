package queue

import (
	"testing"

	"maly/internal/library"
)

func mk(n int) []library.Track {
	out := make([]library.Track, n)
	for i := range out {
		out[i] = library.Track{ID: int64(i + 1), Title: string(rune('a' + i))}
	}
	return out
}

func TestNextRepeatOff(t *testing.T) {
	q := New()
	q.Replace(mk(3))
	q.JumpTo(0)
	if tr, ok := q.Next(false); !ok || tr.ID != 2 {
		t.Fatalf("Next = %v %v, quería pista 2", tr.ID, ok)
	}
	q.JumpTo(2)
	if _, ok := q.Next(true); ok {
		t.Fatal("al final de la cola sin repeat, Next debe fallar")
	}
}

func TestRepeatAllWraps(t *testing.T) {
	q := New()
	q.Replace(mk(2))
	q.Repeat = RepeatAll
	q.JumpTo(1)
	if tr, ok := q.Next(true); !ok || tr.ID != 1 {
		t.Fatalf("con repeat all debe envolver a la pista 1, fue %v %v", tr.ID, ok)
	}
}

func TestRepeatOneOnlyNatural(t *testing.T) {
	q := New()
	q.Replace(mk(3))
	q.Repeat = RepeatOne
	q.JumpTo(1)
	if tr, _ := q.Next(true); tr.ID != 2 {
		t.Fatalf("fin natural con repeat one debe repetir la pista 2, fue %v", tr.ID)
	}
	if tr, _ := q.Next(false); tr.ID != 3 {
		t.Fatalf("`maly next` debe avanzar aunque haya repeat one, fue %v", tr.ID)
	}
}

func TestShuffleNeverRepeatsCurrent(t *testing.T) {
	q := New()
	q.Replace(mk(5))
	q.Shuffle = true
	q.JumpTo(0)
	prev := 0
	for i := 0; i < 50; i++ {
		_, ok := q.Next(false)
		if !ok {
			t.Fatal("shuffle Next no debe fallar con cola no vacía")
		}
		if q.Index == prev {
			t.Fatal("shuffle eligió la misma pista consecutiva")
		}
		prev = q.Index
	}
}

func TestShufflePrevUsesHistory(t *testing.T) {
	q := New()
	q.Replace(mk(5))
	q.Shuffle = true
	q.JumpTo(0)
	q.Next(false)
	second := q.Index
	q.Next(false)
	q.Prev()
	if q.Index != second {
		t.Fatalf("Prev en shuffle debe volver a %d, fue %d", second, q.Index)
	}
}

func TestRemoveAt(t *testing.T) {
	q := New()
	q.Replace(mk(3))
	q.JumpTo(2)
	if q.RemoveAt(0) {
		t.Fatal("quitar una pista anterior no es quitar la actual")
	}
	if q.Index != 1 {
		t.Fatalf("el índice debe ajustarse a 1, fue %d", q.Index)
	}
	if !q.RemoveAt(1) {
		t.Fatal("quitar la actual debe reportarlo")
	}
}
