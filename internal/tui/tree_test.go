package tui

import (
	"strings"
	"testing"

	"maly/internal/library"
)

// TestTreeStateSurvivesReload: una recarga de la biblioteca reconstruye el
// árbol desde cero, así que snapshot/restore tienen que devolverle al usuario
// lo que tenía abierto y dónde estaba — antes cada scan o mutación de
// playlist colapsaba todo y mandaba el cursor al tope.
func TestTreeStateSurvivesReload(t *testing.T) {
	tracks := []library.Track{
		{ID: 1, Artist: "Ana", Album: "Uno", Title: "alfa", Path: "/m/a.mp3"},
		{ID: 2, Artist: "Ana", Album: "Uno", Title: "beta", Path: "/m/b.mp3"},
		{ID: 3, Artist: "Beto", Album: "Dos", Title: "gama", Path: "/m/c.mp3"},
	}
	build := func() *libTree {
		tr := buildTree(tracks, nil)
		tr.cursor = 0
		tr.toggle() // Ana
		tr.cursor = 1
		tr.toggle() // Uno
		tr.cursor = 3
		return tr
	}

	// Recarga con el mismo contenido: misma pista bajo el cursor y los nodos
	// siguen abiertos.
	old := build()
	want := old.current().track.Path
	st := old.snapshot()
	tr := buildTree(tracks, nil)
	tr.restore(st, 10)
	if n := tr.current(); n == nil || n.track.Path != want {
		t.Fatalf("tras recargar el cursor debía seguir en %s: %+v", want, n)
	}
	if len(tr.rows) != len(old.rows) {
		t.Fatalf("la expansión no se restauró: %d filas, quería %d", len(tr.rows), len(old.rows))
	}

	// Una playlist nueva no mueve nada de lo anterior.
	tr = buildTree(tracks, []plList{{name: "favs", tracks: tracks[:1]}})
	tr.restore(old.snapshot(), 10)
	if n := tr.current(); n == nil || n.track.Path != want {
		t.Fatalf("con una playlist nueva el cursor se movió: %+v", n)
	}

	// Si la pista del cursor desaparece, el índice de respaldo lo deja dentro
	// de rango en vez de romper.
	tr = buildTree(tracks[:1], nil)
	tr.restore(old.snapshot(), 10)
	if tr.cursor < 0 || tr.cursor >= len(tr.rows) {
		t.Fatalf("cursor fuera de rango tras encoger: %d de %d", tr.cursor, len(tr.rows))
	}

	// El filtro activo también se conserva (rows son filas sintéticas).
	old = build()
	old.filter = "alfa"
	old.flatten()
	old.cursor = 0
	tr = buildTree(tracks, nil)
	tr.restore(old.snapshot(), 10)
	if tr.filter != "alfa" || len(tr.rows) != 1 || tr.rows[0].track.Path != "/m/a.mp3" {
		t.Fatalf("el filtro no sobrevivió: %q, %d filas", tr.filter, len(tr.rows))
	}
}

// TestBuildTreeWithPlaylists: las playlists cuelgan del árbol como raíces
// tras los artistas, con sus pistas numeradas como hijas directas; expandir,
// plegar (por profundidad) y tracks() funcionan igual que en artistas/álbumes.
func TestBuildTreeWithPlaylists(t *testing.T) {
	tracks := []library.Track{
		{ID: 1, Artist: "Ana", Album: "Uno", Title: "alfa", Path: "/m/a.mp3"},
		{ID: 2, Artist: "Ana", Album: "Uno", Title: "beta", Path: "/m/b.mp3"},
		{ID: 3, Artist: "Beto", Album: "Dos", Title: "gama", Path: "/m/c.mp3"},
	}
	lists := []plList{{name: "favs", tracks: []library.Track{tracks[2], tracks[0]}}}
	tr := buildTree(tracks, lists)

	// Colapsado: dos artistas y una playlist, en ese orden.
	if len(tr.rows) != 3 || tr.rows[2].kind != playlistNode {
		t.Fatalf("filas colapsadas: %d, última %v", len(tr.rows), tr.rows[len(tr.rows)-1].kind)
	}
	pl := tr.rows[2]
	if !strings.Contains(pl.label, "favs (2)") {
		t.Errorf("etiqueta de playlist: %q", pl.label)
	}

	// Expandir la playlist: pistas hijas directas con su posición 1-based
	// (la misma que usa `playlist remove`).
	tr.cursor = 2
	tr.toggle()
	if len(tr.rows) != 5 || tr.rows[3].kind != trackNode || tr.rows[3].track.ID != 3 {
		t.Fatalf("playlist expandida: %d filas, fila 3 %+v", len(tr.rows), tr.rows[3])
	}
	if !strings.HasPrefix(tr.rows[3].label, " 1 ") || !strings.HasPrefix(tr.rows[4].label, " 2 ") {
		t.Errorf("las pistas de playlist deben ir numeradas: %q, %q", tr.rows[3].label, tr.rows[4].label)
	}

	// collapse (vim h) desde una pista de playlist sube al nodo playlist:
	// el padre es el anterior menos profundo.
	tr.cursor = 4
	tr.collapse(10)
	if tr.cursor != 2 {
		t.Errorf("collapse desde pista de playlist: cursor %d, quería 2", tr.cursor)
	}

	// tracks() de la playlist conserva el orden de sus posiciones.
	got := pl.tracks()
	if len(got) != 2 || got[0].ID != 3 || got[1].ID != 1 {
		t.Errorf("tracks() de playlist: %+v", got)
	}

	// El camino artista > álbum > pista sigue intacto a tres niveles.
	tr.cursor = 0
	tr.toggle() // expande a Ana
	if tr.rows[1].kind != albumNode || tr.rows[1].depth != 1 {
		t.Fatalf("hijo de artista: %+v", tr.rows[1])
	}
	tr.cursor = 1
	tr.toggle() // expande el álbum
	if tr.rows[2].kind != trackNode || tr.rows[2].depth != 2 || tr.rows[2].track.ID != 1 {
		t.Fatalf("pista de álbum: %+v", tr.rows[2])
	}
	tr.cursor = 2
	tr.collapse(10) // desde la pista sube al álbum
	if tr.cursor != 1 {
		t.Errorf("collapse desde pista de álbum: cursor %d, quería 1", tr.cursor)
	}
}
