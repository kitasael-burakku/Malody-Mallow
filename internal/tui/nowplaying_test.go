package tui

import (
	"image"
	"image/color"
	"strings"
	"testing"

	"maly/internal/config"
	"maly/internal/ipc"
	"maly/internal/media"
)

func TestActiveLyric(t *testing.T) {
	lines := []media.LyricLine{
		{At: 5, Text: "a"}, {At: 10, Text: "b"}, {At: 20, Text: "c"},
	}
	for pos, want := range map[float64]int{
		0: -1, 4.9: -1, 5: 0, 9.9: 0, 10: 1, 15: 1, 20: 2, 999: 2,
	} {
		if got := activeLyric(lines, pos); got != want {
			t.Errorf("activeLyric(%v) = %d, quería %d", pos, got, want)
		}
	}
}

// TestHalfBlocksRender: una imagen 2×4 conocida debe salir como 2 filas de
// "▀" con los pares fg (píxel superior) / bg (píxel inferior) correctos.
func TestHalfBlocksRender(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 4))
	colors := [][3]uint8{
		{255, 0, 0}, {0, 255, 0}, // fila 0 de celdas: arriba
		{0, 0, 255}, {255, 255, 0}, // fila 0 de celdas: abajo
		{1, 2, 3}, {1, 2, 3}, // fila 1: arriba (iguales → un solo SGR)
		{1, 2, 3}, {1, 2, 3}, // fila 1: abajo
	}
	for i, c := range colors {
		img.Set(i%2, i/2, color.RGBA{c[0], c[1], c[2], 255})
	}
	lines := halfBlocks{}.render(img, 2, 2)
	if len(lines) != 2 {
		t.Fatalf("filas = %d, quería 2", len(lines))
	}
	if !strings.Contains(lines[0], "\x1b[38;2;255;0;0m\x1b[48;2;0;0;255m▀") ||
		!strings.Contains(lines[0], "\x1b[38;2;0;255;0m\x1b[48;2;255;255;0m▀") {
		t.Errorf("fila 0 sin los pares fg/bg esperados: %q", lines[0])
	}
	// Celdas contiguas con el mismo par comparten secuencia: un solo SGR.
	if strings.Count(lines[1], "\x1b[38;2;") != 1 || !strings.Contains(lines[1], "▀▀") {
		t.Errorf("fila 1 debía agrupar las celdas iguales: %q", lines[1])
	}
	for _, l := range lines {
		if !strings.HasSuffix(l, "\x1b[0m") {
			t.Errorf("cada fila debe cerrar con reset: %q", l)
		}
	}
}

// TestNpLyricsClamp: el scroll manual se clampa al rango real y sin letras
// sale el aviso centrado.
func TestNpLyricsClamp(t *testing.T) {
	m := &Model{st: newStyles(config.Theme{}), status: &ipc.Status{Track: &ipc.TrackInfo{Path: "/x.mp3"}}}
	m.npTrack = "/x.mp3"
	for i := 0; i < 6; i++ {
		m.npLyrics = append(m.npLyrics, media.LyricLine{At: -1, Text: "l"})
	}
	m.npScroll = 99
	if got := m.npLyricsLines(20, 4); len(got) != 4 {
		t.Fatalf("líneas = %d, quería 4", len(got))
	}
	if m.npScroll != 2 { // 6 líneas - 4 visibles
		t.Errorf("npScroll = %d, quería el clamp a 2", m.npScroll)
	}
	m.npScroll = -5
	m.npLyricsLines(20, 4)
	if m.npScroll != 0 {
		t.Errorf("npScroll = %d, quería el clamp a 0", m.npScroll)
	}

	m.npLyrics = nil
	out := m.npLyricsLines(20, 3)
	if len(out) != 3 || out[1] == "" {
		t.Errorf("sin letras debía centrar el aviso: %q", out)
	}
}
