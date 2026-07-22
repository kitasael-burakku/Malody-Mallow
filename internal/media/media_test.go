package media

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

func TestParseLRCTimed(t *testing.T) {
	src := "\uFEFF[ar:Alguien]\n" +
		"[offset:+500]\r\n" +
		"[00:12.00]Primera línea\n" +
		"[00:20]Segunda <00:21.00>con <00:22.50>palabras\n" +
		"[01:03.5][00:30:25]Coro repetido\n" +
		"[00:45.123]\n" + // pausa instrumental: línea vacía con marca
		"créditos sueltos sin marca\n"
	got := ParseLRC(strings.NewReader(src))

	want := []LyricLine{
		{At: 11.5, Text: "Primera línea"},
		{At: 19.5, Text: "Segunda con palabras"},
		{At: 29.75, Text: "Coro repetido"}, // [00:30:25] variante vieja = 30.25s
		{At: 44.623, Text: ""},
		{At: 63.0, Text: "Coro repetido"},
	}
	if len(got) != len(want) {
		t.Fatalf("líneas = %d, quería %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if !near(got[i].At, want[i].At) || got[i].Text != want[i].Text {
			t.Errorf("línea %d = {%v %q}, quería {%v %q}",
				i, got[i].At, got[i].Text, want[i].At, want[i].Text)
		}
	}
}

func TestParseLRCPlano(t *testing.T) {
	src := "Letra sin marcas\n\nsegunda estrofa\n"
	got := ParseLRC(strings.NewReader(src))
	if len(got) != 2 {
		t.Fatalf("líneas = %d, quería 2: %+v", len(got), got)
	}
	for i, text := range []string{"Letra sin marcas", "segunda estrofa"} {
		if got[i].At >= 0 || got[i].Text != text {
			t.Errorf("línea %d = %+v, quería {At<0 %q}", i, got[i], text)
		}
	}
}

// TestParseLRCOffsetNegativo: un offset que empujaría marcas por debajo de
// cero se recorta a 0 (no debe producir At negativo, que significa "sin
// sincronía").
func TestParseLRCOffsetNegativo(t *testing.T) {
	src := "[offset:2000]\n[00:01.00]casi al inicio\n"
	got := ParseLRC(strings.NewReader(src))
	if len(got) != 1 || !near(got[0].At, 0) {
		t.Fatalf("got = %+v, quería At 0", got)
	}
}

func TestScaleBox(t *testing.T) {
	// Damero 2×2 blanco/negro → 1×1 = promedio gris.
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{255, 255, 255, 255})
	img.Set(1, 1, color.RGBA{255, 255, 255, 255})
	img.Set(1, 0, color.RGBA{0, 0, 0, 255})
	img.Set(0, 1, color.RGBA{0, 0, 0, 255})
	small := ScaleBox(img, 1, 1)
	r, g, b, a := small.At(0, 0).RGBA()
	if r>>8 != 127 || g>>8 != 127 || b>>8 != 127 || a>>8 != 255 {
		t.Fatalf("promedio = %v %v %v %v, quería 127 gris opaco", r>>8, g>>8, b>>8, a>>8)
	}

	// 1×1 rojo → 3×2 ampliado: todo rojo (vecino más cercano).
	one := image.NewRGBA(image.Rect(0, 0, 1, 1))
	one.Set(0, 0, color.RGBA{200, 10, 30, 255})
	big := ScaleBox(one, 3, 2)
	for y := 0; y < 2; y++ {
		for x := 0; x < 3; x++ {
			r, g, b, _ := big.At(x, y).RGBA()
			if r>>8 != 200 || g>>8 != 10 || b>>8 != 30 {
				t.Fatalf("píxel (%d,%d) = %v %v %v, quería 200 10 30", x, y, r>>8, g>>8, b>>8)
			}
		}
	}
}

// Las letras (sidecar .lrc o USLT embebido) se pintan en la capa ctrl+t: con
// escapes dentro romperían el panel o secuestrarían el terminal. Sanear al leer
// la línea cubre de una vez las que llevan marca de tiempo y las que no.
func TestParseLRCSaneaControles(t *testing.T) {
	got := ParseLRC(strings.NewReader("[00:01.00]hola\x1b[31mmundo\n[00:02.50]otra\x07linea\n"))
	if len(got) != 2 {
		t.Fatalf("líneas = %d, quería 2: %+v", len(got), got)
	}
	if got[0].Text != "hola[31mmundo" {
		t.Errorf("línea 0 = %q", got[0].Text)
	}
	if got[1].Text != "otralinea" {
		t.Errorf("línea 1 = %q", got[1].Text)
	}

	plana := ParseLRC(strings.NewReader("solo\x1b]0;HACK\x07texto\n"))
	if len(plana) != 1 || plana[0].Text != "solo]0;HACKtexto" {
		t.Errorf("letra plana = %+v", plana)
	}
}

// Un .lrc corrupto de cientos de MB junto a una pista se materializaba entero
// como []LyricLine y congelaba la TUI al abrir ctrl+t: el buffer del Scanner
// acota la LÍNEA, no cuántas hay.
func TestParseLRCAcotado(t *testing.T) {
	var b strings.Builder
	for i := 0; b.Len() < maxLyricsBytes*2; i++ {
		fmt.Fprintf(&b, "[00:%02d.00]linea numero %d de relleno\n", i%60, i)
	}
	escritas := strings.Count(b.String(), "\n")

	got := ParseLRC(strings.NewReader(b.String()))
	if len(got) == 0 {
		t.Fatal("no parseó nada")
	}
	if len(got) >= escritas {
		t.Errorf("devolvió %d líneas de %d escritas: no se acotó", len(got), escritas)
	}
}

func TestLyricsFor(t *testing.T) {
	dir := t.TempDir()
	track := filepath.Join(dir, "cancion.mp3")

	// Sin .lrc: caen las embebidas, sin sincronía.
	lines, synced := LyricsFor(track, "hola\nmundo")
	if synced || len(lines) != 2 || lines[0].Text != "hola" {
		t.Fatalf("embebidas: synced=%v lines=%+v", synced, lines)
	}

	// Embebidas en formato LRC: sincronizan igual.
	lines, synced = LyricsFor(track, "[00:05.00]hola")
	if !synced || len(lines) != 1 || !near(lines[0].At, 5) {
		t.Fatalf("embebidas LRC: synced=%v lines=%+v", synced, lines)
	}

	// Con sidecar .lrc: tiene prioridad sobre las embebidas.
	lrc := filepath.Join(dir, "cancion.lrc")
	if err := os.WriteFile(lrc, []byte("[00:10.00]del sidecar\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lines, synced = LyricsFor(track, "embebida perdedora")
	if !synced || len(lines) != 1 || lines[0].Text != "del sidecar" {
		t.Fatalf("sidecar: synced=%v lines=%+v", synced, lines)
	}

	// Nada de nada.
	if lines, synced = LyricsFor(filepath.Join(dir, "otra.mp3"), ""); synced || len(lines) != 0 {
		t.Fatalf("sin letras: synced=%v lines=%+v", synced, lines)
	}
}
