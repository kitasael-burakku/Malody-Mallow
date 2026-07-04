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
	trackNode
)

type node struct {
	kind     nodeKind
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

// libTree es el árbol Artista > Álbum > pista del panel Biblioteca, con una
// vista aplanada (rows) de los nodos visibles y un filtro plano opcional.
type libTree struct {
	roots  []*node
	rows   []*node
	cursor int
	offset int
	filter string // si no está vacío, rows es una lista plana de pistas
	all    []library.Track
}

func buildTree(tracks []library.Track) *libTree {
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
			curAlbum = &node{kind: albumNode, label: album}
			curArtist.children = append(curArtist.children, curAlbum)
		}
		label := tr.Title
		if tr.TrackNo > 0 {
			label = fmt.Sprintf("%02d %s", tr.TrackNo, tr.Title)
		}
		curAlbum.children = append(curAlbum.children, &node{kind: trackNode, label: label, track: tr})
	}
	t.flatten()
	return t
}

// flatten reconstruye rows según expansión y filtro, y reencuadra el cursor.
func (t *libTree) flatten() {
	t.rows = t.rows[:0]
	if t.filter != "" {
		q := library.Fold(t.filter)
		for _, tr := range t.all {
			hay := library.Fold(tr.Title + " " + tr.Artist + " " + tr.Album)
			if containsAll(hay, q) {
				label := tr.Title
				if tr.Artist != "" {
					label = tr.Artist + " — " + tr.Title
				}
				t.rows = append(t.rows, &node{kind: trackNode, label: label, track: tr})
			}
		}
	} else {
		for _, a := range t.roots {
			t.rows = append(t.rows, a)
			if !a.expanded {
				continue
			}
			for _, al := range a.children {
				t.rows = append(t.rows, al)
				if !al.expanded {
					continue
				}
				t.rows = append(t.rows, al.children...)
			}
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
