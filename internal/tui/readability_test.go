package tui

import (
	"image"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"maly/internal/config"
	"maly/internal/i18n"
	"maly/internal/ipc"
	"maly/internal/library"
	"maly/internal/media"
)

const ux03Title = "Una canción con un título muy largo que necesita respirar — versión acústica"

func ux03Model(w, h int) *Model {
	m := newLayoutTestModel(w, h)
	m.status = &ipc.Status{Track: &ipc.TrackInfo{Path: "/a.flac", Title: ux03Title, Artist: "Áurea", Album: "Horizontes", Genre: "Folk", TrackNo: 1}, QueueIndex: 0, Duration: 240, Position: 3, Paused: true}
	m.npTrack = "/a.flac"
	m.npLyrics = []media.LyricLine{{At: 0, Text: "VERSO VISIBLE"}}
	m.npSynced = true
	return m
}

func TestUX03TitleBudget(t *testing.T) {
	m := ux03Model(140, 42)
	lines := m.npMetaText(26)
	if len(lines) != 5 || !strings.Contains(ansi.Strip(lines[1]), "muy largo") {
		t.Fatalf("título sin segunda fila: %q", lines)
	}
	checkStateText(t, strings.Join(lines, "\n"), []string{"Horizontes", "Folk", "…"})
	m.status.Track.Title = "Café 東京"
	lines = m.npMetaText(26)
	if len(lines) != 4 || !strings.Contains(lines[0], "Café 東京") {
		t.Fatalf("título corto alterado: %q", lines)
	}
}

func TestUX03MetadataAreas(t *testing.T) {
	defer i18n.Set(i18n.Code())
	for _, lang := range []string{"es", "en"} {
		i18n.Set(lang)
		for _, size := range [][2]int{{140, 42}, {100, 28}, {80, 24}, {40, 12}, {120, 24}, {140, 16}} {
			for _, art := range []bool{false, true} {
				m := ux03Model(size[0], size[1])
				if art {
					m.npImg = image.NewRGBA(image.Rect(0, 0, 8, 8))
				}
				out := m.npView()
				assertUX02Frame(t, out, size[0], size[1])
				checkStateText(t, out, []string{"Áurea", "Horizontes", "Folk", "00:03", "04:00", "VERSO VISIBLE", "esc"})
				rows := strings.Split(ansi.Strip(out), "\n")
				if !strings.Contains(rows[len(rows)-2], "esc") {
					t.Fatalf("ayuda desplazada: %q", rows)
				}
				if size[0] == 140 && size[1] == 42 {
					lay := m.layoutOf()
					col := m.npColumn(lay.npW, lay.npH)
					checkStateText(t, col, []string{"muy largo", "Horizontes", "Folk"})
					assertUX02Frame(t, col, lay.npW, lay.npH)
				}
			}
		}
	}
}

func TestUX03SelectionAndCurrent(t *testing.T) {
	// Strip elimina estilos: la selección debe sobrevivir incluso sin ANSI.
	for _, w := range []int{140, 100, 80, 40} {
		m := ux03Model(w, 24)
		m.queue = []ipc.TrackInfo{*m.status.Track, {Path: "/b.flac", Title: "Otra", Artist: "北の空"}}
		m.focus = panelQueue
		m.queueCursor = 0
		out := ansi.Strip(m.queuePanel(w, 15))
		checkStateText(t, out, []string{">▶"})
		m.queueCursor = 1
		out = ansi.Strip(m.queuePanel(w, 15))
		checkStateText(t, out, []string{" ▶", "> "}, ">▶")
		m.tree = buildTree([]library.Track{{Path: "/a.flac", Title: "Luz", Artist: "Áurea"}, {Path: "/b.flac", Title: "Luz dos", Artist: "Áurea"}}, nil)
		m.focus = panelLibrary
		m.applyFilter("Luz")
		checkStateText(t, ansi.Strip(m.libraryPanel(w, 15)), []string{">▶"})
		m.tree.cursor = 1
		checkStateText(t, ansi.Strip(m.libraryPanel(w, 15)), []string{" ▶", "> "}, ">▶")
		p := newPicker(newStyles(config.Default().Theme), "buscar")
		p.setItems([]pickerItem{newPickerItem("Café 東京", "a"), newPickerItem("Otra", "b")})
		checkStateText(t, ansi.Strip(p.render("Songs", "esc", w, 4)), []string{"> Café 東京"})
		p.cursor = 1
		checkStateText(t, ansi.Strip(p.render("Songs", "esc", w, 4)), []string{"> Otra"}, "> Café 東京")
	}
}
