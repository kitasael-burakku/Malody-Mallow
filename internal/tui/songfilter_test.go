package tui

import (
	"fmt"
	"reflect"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"maly/internal/ipc"
	"maly/internal/library"
)

func songFilterFixture() []library.Track {
	return []library.Track{
		{Title: "Has Mo", Artist: "Áurea", Album: "Horizontes", Path: "/music/01.flac"},
		{Title: "hsm", Artist: "Aure\u0301a", Album: "Horizontes", Path: "/music/02.flac"},
		{Title: "Live", Artist: "Áurea", Path: "/music/03.flac"},
		{Title: "夜に Café", Artist: "Áurea", Path: "/music/04.flac"},
		{Title: "Áurea", Artist: "Otra", Album: "Live", Path: "/music/05.flac"},
		{Path: "/hidden/Cancio\u0301n 東京.flac"},
	}
}

func TestUX04SongQueries(t *testing.T) {
	tracks := songFilterFixture()
	cases := []struct {
		q    string
		want []int
	}{
		{"", []int{0, 1, 2, 3, 4, 5}}, {"a:Áurea", []int{0, 1, 2, 3}},
		{"a:Aure\u0301a -live", []int{0, 1, 3}}, {"al:horizontes", []int{0, 1}},
		{"t:hsm", []int{1, 0}}, {"t:夜に", []int{3}}, {"cafe", []int{3}},
		{"f:cancion", []int{5}}, {"f:東京", []int{5}}, {"f:hidden", nil}, {"f:flac", nil},
		{"a:otra t:aurea", []int{4}}, {"a:live", nil}, {"al:aurea", nil},
		{"-a:aurea", []int{4, 5}}, {"-hsm", []int{0, 2, 3, 4, 5}},
		{"hsm", []int{1, 0}}, {"hm", nil}, {"aurhsm", nil}, // No cruzar título y artista.
		{"a: -", []int{0, 1, 2, 3, 4, 5}}, {"zzzz", nil},
	}
	for _, tc := range cases {
		t.Run(tc.q, func(t *testing.T) {
			m := newLayoutTestModel(100, 28)
			m.tree = buildTree(tracks, nil)
			m.openSongs()
			m.queue = nil
			for _, tr := range tracks {
				m.queue = append(m.queue, ipc.TrackInfo{Title: tr.Title, Artist: tr.Artist, Album: tr.Album, Path: tr.Path})
			}
			m.queueFilter = tc.q
			if got := append([]int(nil), m.visibleQueue()...); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("cola %q = %v; esperado %v", tc.q, got, tc.want)
			}
			p := newPicker(m.st, "")
			p.setItems(songItems(tracks)) // mismo montaje de RunSelect
			for name, pk := range map[string]*picker{"ctrl+o": m.songs, "select": p} {
				pk.input.SetValue(tc.q)
				pk.filter()
				if !reflect.DeepEqual(append([]int(nil), pk.matches...), tc.want) {
					t.Errorf("%s %q = %v; esperado %v", name, tc.q, pk.matches, tc.want)
				}
			}
			if tc.q != "" { // / vacío es árbol jerárquico, no una lista de pistas.
				m.tree.filter = tc.q
				m.tree.flatten()
				var got []string
				for _, row := range m.tree.rows {
					got = append(got, row.track.Path)
				}
				var want []string
				for _, i := range tc.want {
					want = append(want, tracks[i].Path)
				}
				if !reflect.DeepEqual(got, want) {
					t.Errorf("/ %q = %v; esperado %v", tc.q, got, want)
				}
			}
		})
	}
}

func TestUX04RefreshAndActions(t *testing.T) {
	for _, standalone := range []bool{false, true} {
		for _, key := range []tea.KeyType{tea.KeyTab, tea.KeyEnter} {
			t.Run(fmt.Sprintf("select=%v/key=%v", standalone, key), func(t *testing.T) {
				tracks := songFilterFixture()
				m := newLayoutTestModel(100, 28)
				m.tree = buildTree(tracks, nil)
				m.openSongs()
				m.songs.input.SetValue("a:aurea")
				m.songs.filter()
				m.songs.cursor = 1
				// Insertar arriba y cambiar el label de la elegida: manda su ruta.
				tracks[1].Title = "Renombrada"
				fresh := append([]library.Track{{Title: "Nueva", Artist: "Áurea", Path: "/music/new.flac"}}, tracks...)
				m.Update(libraryMsg{tracks: fresh})
				it, ok := m.songs.current()
				if !ok || it.value != "/music/02.flac" {
					t.Fatalf("selección después de refrescar: %+v, %v", it, ok)
				}
				sock, reqs := capturaReq(t)
				m.sock = sock
				var cmd tea.Cmd
				if standalone {
					s := &selectModel{sock: sock, pk: m.songs}
					_, cmd = s.Update(tea.KeyMsg{Type: key})
				} else {
					_, cmd = m.handleSongsKey(tea.KeyMsg{Type: key})
				}
				req := esperaReq(t, cmd, reqs)
				want := "add"
				if key == tea.KeyEnter {
					want = "playnow"
				}
				if req.Cmd != want || !reflect.DeepEqual(req.Paths, []string{"/music/02.flac"}) {
					t.Fatalf("acción incorrecta: %+v", req)
				}
				if key == tea.KeyTab {
					it, _ = m.songs.current()
					if it.value != "/music/02.flac" || m.songs.input.Value() != "a:aurea" {
						t.Fatal("Tab perdió filtro o identidad")
					}
				}
			})
		}
	}
}

func TestUX04OtherPickers(t *testing.T) {
	p := pickerWith("a:Áurea", "Áurea", "Live")
	p.input.SetValue("a:aurea")
	p.filter()
	if !reflect.DeepEqual(p.matches, []int{0}) {
		t.Fatalf("playlist cambió de lenguaje: %v", p.matches)
	}
	p.noFilter = true
	p.input.SetValue("-live")
	p.filter()
	if len(p.matches) != 3 {
		t.Fatal("filtrado local de resultados remotos")
	}
	// Reutilizar el picker con ítems genéricos no debe conservar campos viejos.
	p.noFilter = false
	p.setItems(songItems(songFilterFixture()))
	p.setItems([]pickerItem{newPickerItem("Live", "pl")})
	p.input.SetValue("live")
	p.filter()
	if len(p.matches) != 1 {
		t.Fatal("campos obsoletos")
	}
}

func BenchmarkUX04Picker(b *testing.B) {
	tracks := make([]library.Track, 40000)
	for i := range tracks {
		tracks[i] = library.Track{Title: fmt.Sprintf("Canción Horizonte %05d", i), Artist: fmt.Sprintf("Áurea %d", i%100), Album: "Estudio", Path: fmt.Sprintf("/music/%05d.flac", i)}
	}
	for _, q := range []string{"horizonte", "a:aurea -live", "t:cncn", "f:123"} {
		b.Run(q, func(b *testing.B) {
			p := newPicker(styles{}, "")
			p.setItems(songItems(tracks))
			p.input.SetValue(q)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				p.filter()
			}
		})
	}
	b.Run("render", func(b *testing.B) {
		p := newPicker(styles{}, "")
		p.setItems(songItems(tracks))
		p.input.SetValue("horizonte")
		p.filter()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p.render("Songs", "hint", 80, 14)
		}
	})
	b.Run("move-input", func(b *testing.B) {
		p := newPicker(styles{}, "")
		p.setItems(songItems(tracks))
		p.input.SetValue("horizonte")
		p.filter()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p.handleKey(tea.KeyMsg{Type: tea.KeyLeft})
		}
	})
}
