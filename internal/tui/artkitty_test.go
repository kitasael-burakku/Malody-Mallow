package tui

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"io"
	"strings"
	"testing"
)

// kittyEscapes separa los APC de gráficos (\x1b_G control ; payload \x1b\)
// del resto de la línea; devuelve los pares control/payload y lo que queda.
func kittyEscapes(t *testing.T, s string) (ctrls, payloads []string, rest string) {
	t.Helper()
	for {
		start := strings.Index(s, "\x1b_G")
		if start < 0 {
			return ctrls, payloads, rest + s
		}
		end := strings.Index(s[start:], "\x1b\\")
		if end < 0 {
			t.Fatalf("APC sin terminador ST: %q…", s[start:start+20])
		}
		body := s[start+3 : start+end]
		ctrl, payload, _ := strings.Cut(body, ";")
		ctrls = append(ctrls, ctrl)
		payloads = append(payloads, payload)
		rest += s[:start]
		s = s[start+end+2:]
	}
}

// TestKittyRender fija el contrato del renderer kitty: transmisión t=d con
// placement virtual (U=1) cuyos píxeles descomprimidos son los de la imagen,
// chunks de ≤4096 con m= bien encadenado, y placeholders con el id en la
// tinta y los diacríticos de fila/columna de la tabla oficial.
func TestKittyRender(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, color.RGBA{200, 10, 30, 255})
		}
	}
	wCells, hCells := 2, 2
	lines := kittyGfx{}.render(img, wCells, hCells)
	if len(lines) != hCells {
		t.Fatalf("filas = %d, quería %d", len(lines), hCells)
	}

	// Toda la transmisión viaja en la fila 0; las demás no llevan APC.
	ctrls, payloads, rest := kittyEscapes(t, lines[0])
	if len(ctrls) == 0 {
		t.Fatal("la fila 0 no trae la transmisión")
	}
	for i, c := range ctrls {
		wantMore := "m=1"
		if i == len(ctrls)-1 {
			wantMore = "m=0"
		}
		if !strings.Contains(c, wantMore) {
			t.Errorf("chunk %d: control %q sin %s", i, c, wantMore)
		}
		if len(payloads[i]) > kittyChunk {
			t.Errorf("chunk %d: payload de %d > %d", i, len(payloads[i]), kittyChunk)
		}
	}
	pxW, pxH := wCells*kittyCellW, hCells*kittyCellH
	first := ctrls[0]
	for _, want := range []string{"a=T", "U=1", "q=2", "f=24", "o=z",
		fmt.Sprintf("i=%d", kittyImgID), fmt.Sprintf("s=%d", pxW),
		fmt.Sprintf("v=%d", pxH), fmt.Sprintf("c=%d", wCells), fmt.Sprintf("r=%d", hCells)} {
		if !strings.Contains(first+",", want+",") {
			t.Errorf("control inicial sin %s: %q", want, first)
		}
	}

	// El payload completo debe descomprimir a los píxeles RGB exactos.
	zdata, err := base64.StdEncoding.DecodeString(strings.Join(payloads, ""))
	if err != nil {
		t.Fatalf("base64: %v", err)
	}
	zr, err := zlib.NewReader(bytes.NewReader(zdata))
	if err != nil {
		t.Fatalf("zlib: %v", err)
	}
	raw, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("descomprimiendo: %v", err)
	}
	if len(raw) != pxW*pxH*3 {
		t.Fatalf("píxeles = %d bytes, quería %d", len(raw), pxW*pxH*3)
	}
	if raw[0] != 200 || raw[1] != 10 || raw[2] != 30 {
		t.Errorf("primer píxel = %v, quería [200 10 30]", raw[:3])
	}

	// Placeholders: id en la tinta, U+10EEEE por celda con diacrítico de
	// fila y de columna en orden (misma estructura que emite kitten icat).
	fg := fmt.Sprintf("\x1b[38:2:%d:%d:%dm",
		(kittyImgID>>16)&0xff, (kittyImgID>>8)&0xff, kittyImgID&0xff)
	for r, line := range lines {
		body := line
		if r == 0 {
			body = rest // la fila 0 sin sus APC
		}
		if !strings.HasPrefix(body, fg) {
			t.Errorf("fila %d sin la tinta del id: %q", r, body[:20])
		}
		if n := strings.Count(body, string(kittyPlaceholder)); n != wCells {
			t.Errorf("fila %d: %d placeholders, quería %d", r, n, wCells)
		}
		want := fg
		for c := 0; c < wCells; c++ {
			want += string(kittyPlaceholder) + string(kittyDiacritics[r]) + string(kittyDiacritics[c])
		}
		want += "\x1b[0m"
		if body != want {
			t.Errorf("fila %d = %q, quería %q", r, body, want)
		}
	}
}

// TestPickCoverRenderer: kitty solo con TERM/KITTY_WINDOW_ID y nunca bajo
// tmux (hereda el env de kitty pero no pasa los escapes gráficos).
func TestPickCoverRenderer(t *testing.T) {
	set := func(term, winID, tmux string) {
		t.Setenv("TERM", term)
		t.Setenv("KITTY_WINDOW_ID", winID)
		t.Setenv("TMUX", tmux)
	}
	set("xterm-kitty", "", "")
	if _, ok := pickCoverRenderer().(kittyGfx); !ok {
		t.Error("TERM=xterm-kitty debía elegir kittyGfx")
	}
	set("xterm-256color", "1", "")
	if _, ok := pickCoverRenderer().(kittyGfx); !ok {
		t.Error("KITTY_WINDOW_ID debía elegir kittyGfx")
	}
	set("xterm-kitty", "1", "/tmp/tmux-1000/default,42,0")
	if _, ok := pickCoverRenderer().(halfBlocks); !ok {
		t.Error("bajo tmux debía caer a halfBlocks")
	}
	set("xterm-256color", "", "")
	if _, ok := pickCoverRenderer().(halfBlocks); !ok {
		t.Error("sin señas de kitty debía elegir halfBlocks")
	}
}
