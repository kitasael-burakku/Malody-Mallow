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

// checkOrder valida el invariante de la permutación: cubre toda la cola sin
// duplicados y order[pos] apunta a la actual.
func checkOrder(t *testing.T, q *Queue) {
	t.Helper()
	if !q.Shuffle {
		return
	}
	if len(q.order) != len(q.Items) {
		t.Fatalf("order tiene %d entradas para %d pistas", len(q.order), len(q.Items))
	}
	seen := make(map[int]bool, len(q.order))
	for _, e := range q.order {
		if e < 0 || e >= len(q.Items) || seen[e] {
			t.Fatalf("order no es permutación: %v", q.order)
		}
		seen[e] = true
	}
	if q.Index >= 0 && (q.pos < 0 || q.pos >= len(q.order) || q.order[q.pos] != q.Index) {
		t.Fatalf("order[pos] != Index: order=%v pos=%d Index=%d", q.order, q.pos, q.Index)
	}
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
	q.Repeat = RepeatAll // sin él, 50 avances agotan el ciclo
	q.SetShuffle(true)
	q.JumpTo(0)
	prev := 0
	for i := 0; i < 50; i++ {
		_, ok := q.Next(false)
		if !ok {
			t.Fatal("shuffle Next no debe fallar con repeat all")
		}
		if q.Index == prev {
			t.Fatal("shuffle eligió la misma pista consecutiva")
		}
		prev = q.Index
	}
}

func TestShufflePrevStepsBack(t *testing.T) {
	q := New()
	q.Replace(mk(5))
	q.SetShuffle(true)
	q.JumpTo(0)
	q.Next(false)
	second := q.Index
	q.Next(false)
	q.Prev()
	if q.Index != second {
		t.Fatalf("Prev en shuffle debe volver a %d, fue %d", second, q.Index)
	}
}

func TestPeekNextSequential(t *testing.T) {
	q := New()
	q.Replace(mk(3))
	q.JumpTo(0)
	if tr, ok := q.PeekNext(); !ok || tr.ID != 2 {
		t.Fatalf("PeekNext = %v %v, quería la pista 2", tr.ID, ok)
	}
	if tr, _ := q.Next(true); tr.ID != 2 {
		t.Fatalf("Next debe honrar la promesa, fue %v", tr.ID)
	}
	q.JumpTo(2)
	if _, ok := q.PeekNext(); ok {
		t.Fatal("al final sin repeat no hay promesa")
	}
	q.Repeat = RepeatAll
	if tr, ok := q.PeekNext(); !ok || tr.ID != 1 {
		t.Fatalf("con repeat all la promesa envuelve a la pista 1, fue %v %v", tr.ID, ok)
	}
	q.Repeat = RepeatOne
	if tr, ok := q.PeekNext(); !ok || tr.ID != 3 {
		t.Fatalf("con repeat one la promesa es la actual, fue %v %v", tr.ID, ok)
	}
}

func TestPeekNextShufflePromiseHeld(t *testing.T) {
	q := New()
	q.Replace(mk(10))
	q.Repeat = RepeatAll // los 50 avances cruzan varias costuras
	q.SetShuffle(true)
	q.JumpTo(0)
	tr, ok := q.PeekNext()
	if !ok {
		t.Fatal("shuffle con cola no vacía siempre promete")
	}
	// La promesa es estable entre llamadas y Next(true) la cumple.
	for i := 0; i < 5; i++ {
		if again, _ := q.PeekNext(); again.ID != tr.ID {
			t.Fatalf("la promesa cambió sola: %v → %v", tr.ID, again.ID)
		}
	}
	if got, _ := q.Next(true); got.ID != tr.ID {
		t.Fatalf("Next(true) = %v, la promesa era %v", got.ID, tr.ID)
	}
	// Tras consumirse, la siguiente promesa nunca es la pista actual.
	for i := 0; i < 50; i++ {
		p, _ := q.PeekNext()
		if int(p.ID-1) == q.Index {
			t.Fatal("la promesa de shuffle repitió la pista actual")
		}
		q.Next(true)
	}
}

func TestPeekInvalidatedByMutation(t *testing.T) {
	q := New()
	q.Replace(mk(2))
	q.Repeat = RepeatAll
	q.SetShuffle(true)
	q.JumpTo(0)
	if tr, _ := q.PeekNext(); tr.ID != 2 {
		t.Fatalf("con 2 pistas la promesa es la otra, fue %v", tr.ID)
	}
	// Quitar la prometida: la promesa no puede apuntar a una pista que ya
	// no está (n=1 con repeat all: el wrap repite la única).
	q.RemoveAt(1)
	checkOrder(t, q)
	if tr, ok := q.PeekNext(); !ok || tr.ID != 1 {
		t.Fatalf("tras el remove la promesa debe recalcularse, fue %v %v", tr.ID, ok)
	}
	// Sin repeat all, una sola pista ya sonada agota el ciclo: sin promesa
	// (antes el sorteo la repetía por siempre).
	q.Repeat = RepeatOff
	q.Invalidate()
	if _, ok := q.PeekNext(); ok {
		t.Fatal("ciclo agotado sin repeat all: no debe haber promesa")
	}
}

func TestRemoveAt(t *testing.T) {
	q := New()
	q.Replace(mk(3))
	q.JumpTo(2)
	if removed, wasCurrent := q.RemoveAt(0); !removed || wasCurrent {
		t.Fatal("quitar una pista anterior quita, pero no es la actual")
	}
	if q.Index != 1 {
		t.Fatalf("el índice debe ajustarse a 1, fue %d", q.Index)
	}
	if removed, wasCurrent := q.RemoveAt(1); !removed || !wasCurrent {
		t.Fatal("quitar la actual debe reportar ambas cosas")
	}
	// Fuera de rango: no quita nada y lo dice (antes se confundía con
	// "quité una que no era la actual").
	if removed, _ := q.RemoveAt(99); removed {
		t.Fatal("un índice fuera de rango no debe reportar eliminación")
	}
}

func TestMove(t *testing.T) {
	ids := func(q *Queue) []int64 {
		out := make([]int64, len(q.Items))
		for i, tr := range q.Items {
			out[i] = tr.ID
		}
		return out
	}
	eq := func(got []int64, want ...int64) bool {
		if len(got) != len(want) {
			return false
		}
		for i := range got {
			if got[i] != want[i] {
				return false
			}
		}
		return true
	}

	q := New()
	q.Replace(mk(3))
	q.JumpTo(1)
	// Mover la actual: Index la sigue.
	if !q.Move(1, 0) {
		t.Fatal("mover índices válidos debe funcionar")
	}
	if got := ids(q); !eq(got, 2, 1, 3) || q.Index != 0 {
		t.Fatalf("Move(1,0) = %v Index=%d, quería [2 1 3] Index=0", got, q.Index)
	}
	// Otra pista cruza por encima de la actual: Index se corre.
	if !q.Move(2, 0) {
		t.Fatal("Move(2,0) debe funcionar")
	}
	if got := ids(q); !eq(got, 3, 2, 1) || q.Index != 1 {
		t.Fatalf("Move(2,0) = %v Index=%d, quería [3 2 1] Index=1", got, q.Index)
	}
	// Y de vuelta por debajo.
	if !q.Move(0, 2) {
		t.Fatal("Move(0,2) debe funcionar")
	}
	if got := ids(q); !eq(got, 2, 1, 3) || q.Index != 0 {
		t.Fatalf("Move(0,2) = %v Index=%d, quería [2 1 3] Index=0", got, q.Index)
	}
	// Fuera de rango: no toca nada.
	if q.Move(0, 99) || q.Move(-1, 0) {
		t.Fatal("índices fuera de rango no deben mover")
	}
	if got := ids(q); !eq(got, 2, 1, 3) || q.Index != 0 {
		t.Fatalf("un Move fallido no debe mutar: %v Index=%d", got, q.Index)
	}
}

// TestMovePromiseFollowsTrack: mover remapea la permutación igual que Index,
// así la promesa sigue a la pista movida en vez de redibujarse (si el índice
// crudo sobreviviera sin remapear, tras el move apuntaría a la actual).
func TestMovePromiseFollowsTrack(t *testing.T) {
	q := New()
	q.Replace(mk(2))
	q.SetShuffle(true)
	q.JumpTo(0)
	if tr, _ := q.PeekNext(); tr.ID != 2 {
		t.Fatalf("con 2 pistas la promesa es la otra, fue %v", tr.ID)
	}
	q.Move(0, 1)
	checkOrder(t, q)
	if tr, ok := q.PeekNext(); !ok || tr.ID != 2 {
		t.Fatalf("tras el move la promesa debe seguir siendo la 2, fue %v %v", tr.ID, ok)
	}
}

// TestShuffleFirstDrawIncludesZero: sin pista actual (Index -1) el primer
// avance debe poder dar cualquier índice; la exclusión de la "actual"
// dejaba el 0 fuera para siempre.
func TestShuffleFirstDrawIncludesZero(t *testing.T) {
	seen := false
	for i := 0; i < 200 && !seen; i++ {
		q := New()
		q.Replace(mk(3))
		q.SetShuffle(true)
		if tr, ok := q.PeekNext(); ok && tr.ID == 1 {
			seen = true
		}
	}
	if !seen {
		t.Fatal("en 200 permutaciones iniciales el índice 0 nunca encabezó")
	}
}

// TestShuffleCycleCoversAll: cada ciclo visita todas las pistas exactamente
// una vez, y la costura entre ciclos no repite consecutiva.
func TestShuffleCycleCoversAll(t *testing.T) {
	const n = 8
	q := New()
	q.Replace(mk(n))
	q.Repeat = RepeatAll
	q.SetShuffle(true)
	last := -1
	for cycle := 0; cycle < 3; cycle++ {
		seen := make(map[int]bool, n)
		for i := 0; i < n; i++ {
			if _, ok := q.Next(true); !ok {
				t.Fatal("Next no debe fallar con repeat all")
			}
			if seen[q.Index] {
				t.Fatalf("el ciclo %d repitió la pista %d antes de agotarse", cycle, q.Index)
			}
			seen[q.Index] = true
			if i == 0 && q.Index == last {
				t.Fatalf("la costura repitió la pista %d", last)
			}
		}
		if len(seen) != n {
			t.Fatalf("el ciclo %d visitó %d de %d pistas", cycle, len(seen), n)
		}
		last = q.Index
	}
}

// TestShuffleEndsWithoutRepeatAll: agotar el ciclo sin repeat all termina la
// reproducción, como en el modo secuencial (el sorteo viejo seguía eterno).
func TestShuffleEndsWithoutRepeatAll(t *testing.T) {
	q := New()
	q.Replace(mk(3))
	q.SetShuffle(true)
	for i := 0; i < 3; i++ {
		if _, ok := q.Next(true); !ok {
			t.Fatalf("el avance %d del ciclo debe funcionar", i+1)
		}
	}
	if _, ok := q.Next(true); ok {
		t.Fatal("agotado el ciclo sin repeat all, Next debe fallar")
	}
	if _, ok := q.PeekNext(); ok {
		t.Fatal("agotado el ciclo sin repeat all, no hay promesa")
	}
	cur := q.Index
	if tr, ok := q.Prev(); !ok || int(tr.ID-1) == cur {
		t.Fatalf("Prev debe seguir funcionando tras el fin: %v %v", tr.ID, ok)
	}
}

// TestShufflePromiseStableAtWrap: en la costura de repeat all la promesa
// (primera pista del ciclo siguiente) es estable y Next(true) la cumple.
func TestShufflePromiseStableAtWrap(t *testing.T) {
	const n = 5
	q := New()
	q.Replace(mk(n))
	q.Repeat = RepeatAll
	q.SetShuffle(true)
	for i := 0; i < n; i++ {
		q.Next(true)
	}
	if q.pos != n-1 {
		t.Fatalf("tras %d avances pos debe ser %d, fue %d", n, n-1, q.pos)
	}
	tr, ok := q.PeekNext()
	if !ok {
		t.Fatal("con repeat all la costura siempre promete")
	}
	for i := 0; i < 5; i++ {
		if again, _ := q.PeekNext(); again.ID != tr.ID {
			t.Fatalf("la promesa del wrap cambió sola: %v → %v", tr.ID, again.ID)
		}
	}
	if got, _ := q.Next(true); got.ID != tr.ID {
		t.Fatalf("Next(true) = %v, la promesa del wrap era %v", got.ID, tr.ID)
	}
	checkOrder(t, q)
	// El ciclo nuevo tampoco repite hasta agotarse.
	seen := map[int]bool{q.Index: true}
	for i := 1; i < n; i++ {
		q.Next(true)
		if seen[q.Index] {
			t.Fatalf("el ciclo nuevo repitió la pista %d", q.Index)
		}
		seen[q.Index] = true
	}
}

// TestShuffleAddJoinsUnplayed: las pistas agregadas entran al tramo no
// sonado del ciclo; el resto del ciclo las incluye sin repetir sonadas.
func TestShuffleAddJoinsUnplayed(t *testing.T) {
	q := New()
	q.Replace(mk(4))
	q.SetShuffle(true)
	q.Next(true)
	q.Next(true)
	played := map[int64]bool{}
	for _, e := range q.order[:q.pos+1] {
		played[q.Items[e].ID] = true
	}
	q.Add(library.Track{ID: 5, Title: "e"}, library.Track{ID: 6, Title: "f"})
	checkOrder(t, q)
	rest := map[int64]bool{}
	for {
		tr, ok := q.Next(true)
		if !ok {
			break
		}
		if played[tr.ID] || rest[tr.ID] {
			t.Fatalf("el resto del ciclo repitió la pista %d", tr.ID)
		}
		rest[tr.ID] = true
	}
	if len(rest) != 4 || !rest[5] || !rest[6] {
		t.Fatalf("el resto del ciclo debía traer 2 pendientes + 2 nuevas, fue %v", rest)
	}
}

// TestShuffleRemovePreservesCycle: quitar una sonada, una futura y la actual
// mantiene el invariante y el resto del ciclo cubre justo lo pendiente.
func TestShuffleRemovePreservesCycle(t *testing.T) {
	q := New()
	q.Replace(mk(6))
	q.SetShuffle(true)
	for i := 0; i < 3; i++ {
		q.Next(true)
	}
	// Una ya sonada (la primera del ciclo, que no es la actual).
	if removed, wasCurrent := q.RemoveAt(q.order[0]); !removed || wasCurrent {
		t.Fatal("quitar la primera sonada no debe ser la actual")
	}
	checkOrder(t, q)
	// Una futura (la prometida).
	if removed, wasCurrent := q.RemoveAt(q.order[q.pos+1]); !removed || wasCurrent {
		t.Fatal("quitar la prometida no debe ser la actual")
	}
	checkOrder(t, q)
	// La actual: la nueva actual se recoloca como slot en curso.
	if removed, wasCurrent := q.RemoveAt(q.Index); !removed || !wasCurrent {
		t.Fatal("quitar la actual debe reportar wasCurrent")
	}
	checkOrder(t, q)
	// El resto del ciclo: exactamente las entradas futuras, sin tocar el
	// prefijo ya sonado ni la actual.
	future := len(q.order) - q.pos - 1
	prefix := map[int64]bool{}
	for _, e := range q.order[:q.pos+1] {
		prefix[q.Items[e].ID] = true
	}
	got := map[int64]bool{}
	for {
		tr, ok := q.Next(true)
		if !ok {
			break
		}
		if prefix[tr.ID] || got[tr.ID] {
			t.Fatalf("el resto del ciclo repitió la pista %d", tr.ID)
		}
		got[tr.ID] = true
	}
	if len(got) != future {
		t.Fatalf("el resto del ciclo trajo %d pistas, quería %d", len(got), future)
	}
}

// TestShuffleJumpToRepositions: saltar recoloca la pista como siguiente y la
// consume; Prev vuelve a la actual anterior y el ciclo no duplica.
func TestShuffleJumpToRepositions(t *testing.T) {
	q := New()
	q.Replace(mk(5))
	q.SetShuffle(true)
	q.Next(true)
	q.Next(true)
	cur := q.Index
	// Saltar a una futura (la última del ciclo).
	fut := q.order[len(q.order)-1]
	q.JumpTo(fut)
	checkOrder(t, q)
	if q.Index != fut {
		t.Fatalf("JumpTo debe fijar la actual en %d, fue %d", fut, q.Index)
	}
	q.Prev()
	if q.Index != cur {
		t.Fatalf("Prev tras el salto debe volver a %d, fue %d", cur, q.Index)
	}
	if tr, _ := q.Next(true); int(tr.ID-1) != fut {
		t.Fatalf("rehacer el avance debe volver a la saltada %d, fue %d", fut, tr.ID-1)
	}
	// Saltar a una ya sonada: se re-encola como actual sin duplicarse.
	first := q.order[0]
	q.JumpTo(first)
	checkOrder(t, q)
	seen := map[int]bool{q.Index: true}
	for {
		_, ok := q.Next(true)
		if !ok {
			break
		}
		if seen[q.Index] {
			t.Fatalf("tras los saltos el ciclo duplicó la pista %d", q.Index)
		}
		seen[q.Index] = true
	}
}

// TestShufflePrevWrap: paridad con el secuencial al inicio del ciclo.
func TestShufflePrevWrap(t *testing.T) {
	q := New()
	q.Replace(mk(3))
	q.Repeat = RepeatAll
	q.SetShuffle(true)
	q.Next(true) // pos 0
	if tr, ok := q.Prev(); !ok || int(tr.ID-1) != q.order[len(q.order)-1] {
		t.Fatalf("Prev con repeat all debe envolver al final del ciclo, fue %v %v", tr.ID, ok)
	}

	q = New()
	q.Replace(mk(3))
	q.SetShuffle(true)
	q.Next(true) // pos 0
	cur := q.Index
	if tr, ok := q.Prev(); !ok || int(tr.ID-1) != cur {
		t.Fatalf("Prev sin repeat all al inicio del ciclo debe quedarse, fue %v %v", tr.ID, ok)
	}
}

// TestShuffleRepeatOneHoldsPosition: repeat one repite sin consumir la
// permutación; el next manual sí avanza el ciclo.
func TestShuffleRepeatOneHoldsPosition(t *testing.T) {
	q := New()
	q.Replace(mk(4))
	q.SetShuffle(true)
	q.Next(true)
	q.Repeat = RepeatOne
	q.Invalidate()
	cur := q.Index
	if tr, _ := q.PeekNext(); int(tr.ID-1) != cur {
		t.Fatalf("con repeat one la promesa es la actual %d, fue %d", cur, tr.ID-1)
	}
	if tr, _ := q.Next(true); int(tr.ID-1) != cur || q.pos != 0 {
		t.Fatalf("Next(true) con repeat one debe repetir sin mover pos: %d pos=%d", tr.ID-1, q.pos)
	}
	if tr, _ := q.Next(false); int(tr.ID-1) == cur || q.pos != 1 {
		t.Fatalf("Next(false) debe avanzar la permutación: %d pos=%d", tr.ID-1, q.pos)
	}
}
