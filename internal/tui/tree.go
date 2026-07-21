package tui

import (
	"fmt"
	"strings"

	"maly/internal/i18n"
	"maly/internal/library"
)

type nodeKind int

const (
	artistNode nodeKind = iota
	albumNode
	playlistNode
	trackNode
)

type node struct {
	kind     nodeKind
	depth    int // nivel de indentación; el padre es el anterior menos profundo
	label    string
	track    library.Track // solo trackNode
	children []*node
	expanded bool
}

// tracks devuelve todas las pistas bajo el nodo, en orden.
func (n *node) tracks() []library.Track {
	if n.kind == trackNode {
		return []library.Track{n.track}
	}
	var out []library.Track
	for _, c := range n.children {
		out = append(out, c.tracks()...)
	}
	return out
}

// plList es una playlist con sus pistas resueltas, lista para colgar del
// árbol de la biblioteca.
type plList struct {
	name   string
	tracks []library.Track
}

// libTree es el árbol del panel Biblioteca — Artista > Álbum > pista, más
// las playlists como raíces propias al final — con una vista aplanada (rows)
// de los nodos visibles y un filtro plano opcional.
type libTree struct {
	roots  []*node
	rows   []*node
	cursor int
	offset int
	filter string // si no está vacío, rows es una lista plana de pistas
	all    []library.Track
}

func buildTree(tracks []library.Track, lists []plList) *libTree {
	t := &libTree{all: tracks}
	var curArtist, curAlbum *node
	for _, tr := range tracks {
		artist := tr.Artist
		if artist == "" {
			artist = i18n.T("tui.unknown_artist")
		}
		album := tr.Album
		if album == "" {
			album = i18n.T("tui.no_album")
		}
		if curArtist == nil || curArtist.label != artist {
			curArtist = &node{kind: artistNode, label: artist}
			t.roots = append(t.roots, curArtist)
			curAlbum = nil
		}
		if curAlbum == nil || curAlbum.label != album {
			curAlbum = &node{kind: albumNode, depth: 1, label: album}
			curArtist.children = append(curArtist.children, curAlbum)
		}
		label := tr.Title
		if tr.TrackNo > 0 {
			label = fmt.Sprintf("%02d %s", tr.TrackNo, tr.Title)
		}
		curAlbum.children = append(curAlbum.children, &node{kind: trackNode, depth: 2, label: label, track: tr})
	}
	// Las playlists cuelgan al final, con sus pistas como hijas directas
	// (numeradas por posición, la misma que usa `playlist remove`).
	for _, p := range lists {
		pn := &node{kind: playlistNode, label: fmt.Sprintf("♪ %s (%d)", p.name, len(p.tracks))}
		for i, tr := range p.tracks {
			pn.children = append(pn.children, &node{
				kind: trackNode, depth: 1,
				label: fmt.Sprintf("%2d %s", i+1, tr.String()), track: tr,
			})
		}
		t.roots = append(t.roots, pn)
	}
	t.flatten()
	return t
}

// treeState es lo que el usuario "tiene puesto" en el árbol: qué abrió, dónde
// está y qué filtró. Una recarga de la biblioteca (scan de cualquier cliente,
// mutación de playlists) reconstruye los nodos desde cero con buildTree, así
// que sin esto el árbol se colapsaría entero y el cursor saltaría al tope
// cada vez.
type treeState struct {
	expanded map[string]bool
	cursor   string // clave del nodo bajo el cursor
	row      int    // índice de respaldo si esa clave ya no existe
	offset   int
	filter   string
}

// nodeKey identifica un nodo entre recargas. Las pistas se distinguen por su
// ruta (única, y disponible también en las filas sintéticas que fabrica el
// filtro); los demás nodos, por su camino de labels desde la raíz: un álbum
// "Grandes éxitos" puede colgar de dos artistas distintos.
func nodeKey(parent string, n *node) string {
	if n.kind == trackNode {
		return parent + "\x1f" + n.track.Path
	}
	return parent + "\x1f" + n.label
}

// keys mapea cada nodo del árbol a su clave, de una pasada. Las filas
// sintéticas del modo filtrado no salen aquí: se resuelven con nodeKey("", n),
// que para una pista es su ruta.
func (t *libTree) keys() map[*node]string {
	out := map[*node]string{}
	var walk func(parent string, n *node)
	walk = func(parent string, n *node) {
		k := nodeKey(parent, n)
		out[n] = k
		for _, c := range n.children {
			walk(k, c)
		}
	}
	for _, r := range t.roots {
		walk("", r)
	}
	return out
}

// keyOf devuelve la clave de un nodo visible, venga del árbol o de una fila
// sintética del filtro.
func keyOf(keys map[*node]string, n *node) string {
	if k, ok := keys[n]; ok {
		return k
	}
	return nodeKey("", n)
}

// snapshot captura el estado navegable antes de reconstruir el árbol.
func (t *libTree) snapshot() treeState {
	s := treeState{
		expanded: map[string]bool{},
		row:      t.cursor,
		offset:   t.offset,
		filter:   t.filter,
	}
	keys := t.keys()
	for n, k := range keys {
		if n.expanded {
			s.expanded[k] = true
		}
	}
	if n := t.current(); n != nil {
		s.cursor = keyOf(keys, n)
	}
	return s
}

// restore reaplica el estado sobre un árbol recién construido.
func (t *libTree) restore(s treeState, pageH int) {
	if s.expanded == nil {
		return // primera carga: nada que restaurar
	}
	t.filter = s.filter
	keys := t.keys()
	for n, k := range keys {
		if s.expanded[k] {
			n.expanded = true
		}
	}
	t.cursor = s.row
	t.offset = s.offset
	t.flatten() // reencuadra el cursor si el árbol encogió
	if s.cursor == "" {
		t.scrollTo(pageH)
		return
	}
	// Volver a poner el cursor sobre el MISMO nodo: con el índice a secas,
	// algo que desapareciera más arriba correría la lista bajo el usuario y
	// enter reproduciría otra cosa.
	for i, n := range t.rows {
		if keyOf(keys, n) == s.cursor {
			t.cursor = i
			break
		}
	}
	t.scrollTo(pageH)
}

// flatten reconstruye rows según expansión y filtro, y reencuadra el cursor.
func (t *libTree) flatten() {
	t.rows = t.rows[:0]
	if t.filter != "" {
		q := library.Fold(t.filter)
		for _, tr := range t.all {
			hay := library.Fold(tr.Title + " " + tr.Artist + " " + tr.Album)
			if containsAll(hay, q) {
				t.rows = append(t.rows, &node{kind: trackNode, label: tr.String(), track: tr})
			}
		}
	} else {
		var walk func(n *node)
		walk = func(n *node) {
			t.rows = append(t.rows, n)
			if !n.expanded {
				return
			}
			for _, c := range n.children {
				walk(c)
			}
		}
		for _, r := range t.roots {
			walk(r)
		}
	}
	if t.cursor >= len(t.rows) {
		t.cursor = len(t.rows) - 1
	}
	if t.cursor < 0 {
		t.cursor = 0
	}
}

func containsAll(hay, q string) bool {
	for _, w := range strings.Fields(q) {
		if !strings.Contains(hay, w) {
			return false
		}
	}
	return true
}

func (t *libTree) current() *node {
	if t.cursor < 0 || t.cursor >= len(t.rows) {
		return nil
	}
	return t.rows[t.cursor]
}

func (t *libTree) move(delta, pageH int) {
	t.cursor += delta
	if t.cursor < 0 {
		t.cursor = 0
	}
	if t.cursor >= len(t.rows) {
		t.cursor = len(t.rows) - 1
	}
	t.scrollTo(pageH)
}

// scrollTo ajusta offset para que el cursor quede visible en pageH filas.
func (t *libTree) scrollTo(pageH int) {
	if pageH <= 0 {
		return
	}
	if t.cursor < t.offset {
		t.offset = t.cursor
	}
	if t.cursor >= t.offset+pageH {
		t.offset = t.cursor - pageH + 1
	}
	if t.offset < 0 {
		t.offset = 0
	}
}

// toggle expande/colapsa el nodo bajo el cursor (artista o álbum).
func (t *libTree) toggle() {
	if n := t.current(); n != nil && n.kind != trackNode {
		n.expanded = !n.expanded
		t.flatten()
	}
}

// expand (vim l) expande el nodo bajo el cursor; en pistas o con filtro
// activo no hace nada.
func (t *libTree) expand() {
	if n := t.current(); n != nil && t.filter == "" && n.kind != trackNode && !n.expanded {
		n.expanded = true
		t.flatten()
	}
}

// collapse (vim h) pliega el nodo bajo el cursor; si ya está plegado o es
// una pista, sube al nodo padre.
func (t *libTree) collapse(pageH int) {
	n := t.current()
	if n == nil || t.filter != "" {
		return
	}
	if n.kind != trackNode && n.expanded {
		n.expanded = false
		t.flatten()
		return
	}
	for i := t.cursor - 1; i >= 0; i-- {
		if t.rows[i].depth < n.depth { // el padre es el anterior menos profundo
			t.cursor = i
			break
		}
	}
	t.scrollTo(pageH)
}
