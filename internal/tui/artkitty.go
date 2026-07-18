package tui

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"fmt"
	"image"
	"os"
	"strings"

	"maly/internal/media"
)

// Carátula vía el protocolo gráfico de kitty: transmisión directa (t=d, los
// píxeles RGB viajan en el propio escape, zlib+base64) y placement virtual
// con Unicode placeholders (U=1). Los placeholders son la variante del
// protocolo pensada para TUIs de celdas: la imagen vive en caracteres
// normales (U+10EEEE con el id de imagen en el color de tinta y la
// fila/columna en diacríticos combinantes), así el diff de Bubble Tea, los
// modales que tapan la capa y el cierre de ctrl+t funcionan solos — no hay
// placements que borrar ni cursor que mover. La transmisión viaja pegada a
// la primera fila del render: como npView cachea las filas (npArtLines) y el
// renderer solo repinta líneas que cambian, la imagen se transmite una vez
// por pista/tamaño, no por frame. Referencia byte a byte: la salida de
// `kitten icat --unicode-placeholder` (así se cazó que el placeholder es
// U+10EEEE — U+10FFFD parece pero NO es, y kitty calla el mismatch).

const (
	// kittyImgID identifica la única imagen de maly en kitty: reusar el id
	// hace que cada carátula nueva reemplace a la anterior (cero fugas de
	// memoria en el terminal, nada que borrar). Debe caber en 24 bits: los
	// placeholders lo codifican en el color de tinta.
	kittyImgID = 0x6D4C59 // "mLY"
	// kittyPlaceholder es EL carácter placeholder del protocolo (U+10EEEE).
	kittyPlaceholder = rune(0x10EEEE)
	// Píxeles transmitidos por celda, por encima de la densidad típica de
	// una celda real: mejor que kitty reduzca (nítido) a que amplíe (borroso).
	kittyCellW = 20
	kittyCellH = 40
	// Máximo de payload por escape según el protocolo (chunking obligatorio).
	kittyChunk = 4096
)

// kittyDiacritics son los primeros 64 codepoints de rowcolumn-diacritics.txt
// de kitty (la tabla oficial del protocolo): el n-ésimo marca "fila n" o
// "columna n" de un placeholder. 64 alcanza de sobra — npView acota la
// carátula a 14 filas × 28 columnas — y render degrada si no.
var kittyDiacritics = []rune{
	0x0305, 0x030D, 0x030E, 0x0310, 0x0312, 0x033D, 0x033E, 0x033F,
	0x0346, 0x034A, 0x034B, 0x034C, 0x0350, 0x0351, 0x0352, 0x0357,
	0x035B, 0x0363, 0x0364, 0x0365, 0x0366, 0x0367, 0x0368, 0x0369,
	0x036A, 0x036B, 0x036C, 0x036D, 0x036E, 0x036F, 0x0483, 0x0484,
	0x0485, 0x0486, 0x0487, 0x0592, 0x0593, 0x0594, 0x0595, 0x0597,
	0x0598, 0x0599, 0x059C, 0x059D, 0x059E, 0x059F, 0x05A0, 0x05A1,
	0x05A8, 0x05A9, 0x05AB, 0x05AC, 0x05AF, 0x05C4, 0x0610, 0x0611,
	0x0612, 0x0613, 0x0614, 0x0615, 0x0616, 0x0617, 0x0657, 0x0658,
}

// kittyGfx implementa coverRenderer con el protocolo gráfico de kitty.
type kittyGfx struct{}

func (kittyGfx) render(src image.Image, wCells, hCells int) []string {
	if wCells < 1 || hCells < 1 ||
		wCells > len(kittyDiacritics) || hCells > len(kittyDiacritics) {
		return halfBlocks{}.render(src, wCells, hCells)
	}
	px := media.ScaleBox(src, wCells*kittyCellW, hCells*kittyCellH)
	b := px.Bounds()
	w, h := b.Dx(), b.Dy()

	// RGBA de ScaleBox → RGB plano (f=24): el alfa siempre es opaco y son
	// 25 % menos bytes por el pty.
	raw := make([]byte, 0, w*h*3)
	for i := 0; i < len(px.Pix); i += 4 {
		raw = append(raw, px.Pix[i], px.Pix[i+1], px.Pix[i+2])
	}
	var zb bytes.Buffer
	zw := zlib.NewWriter(&zb)
	zw.Write(raw)
	zw.Close()
	payload := base64.StdEncoding.EncodeToString(zb.Bytes())

	// Transmisión en chunks. El primero lleva el control completo: a=T
	// transmite y muestra, U=1 crea el placement virtual de c×r celdas que
	// los placeholders referencian, o=z declara el zlib. q=2 es obligatorio
	// aquí: sin él kitty CONTESTA por stdin y esa respuesta caería en el
	// parser de Bubble Tea como teclas fantasma.
	var tx strings.Builder
	for first := true; payload != ""; first = false {
		n := min(kittyChunk, len(payload))
		chunk := payload[:n]
		payload = payload[n:]
		more := 0
		if payload != "" {
			more = 1
		}
		if first {
			// C=1: el cursor no se mueve (icat también lo manda con a=T).
			fmt.Fprintf(&tx, "\x1b_Ga=T,U=1,q=2,f=24,o=z,C=1,i=%d,s=%d,v=%d,c=%d,r=%d,m=%d;%s\x1b\\",
				kittyImgID, w, h, wCells, hCells, more, chunk)
		} else {
			fmt.Fprintf(&tx, "\x1b_Gm=%d;%s\x1b\\", more, chunk)
		}
	}

	// Las filas de placeholders: tinta = id de imagen, cada celda lleva su
	// diacrítico de fila y de columna. Ancho visible: wCells celdas justas
	// (los diacríticos combinan, los APC no imprimen nada). La tinta va en
	// la forma con dos puntos, la misma que emite icat.
	fg := fmt.Sprintf("\x1b[38:2:%d:%d:%dm",
		(kittyImgID>>16)&0xff, (kittyImgID>>8)&0xff, kittyImgID&0xff)
	lines := make([]string, hCells)
	var sb strings.Builder
	for r := 0; r < hCells; r++ {
		sb.Reset()
		if r == 0 {
			sb.WriteString(tx.String())
		}
		sb.WriteString(fg)
		for c := 0; c < wCells; c++ {
			sb.WriteRune(kittyPlaceholder)
			sb.WriteRune(kittyDiacritics[r])
			sb.WriteRune(kittyDiacritics[c])
		}
		sb.WriteString("\x1b[0m")
		lines[r] = sb.String()
	}
	return lines
}

// supportsKittyGfx detecta un kitty real. Bajo tmux se degrada a half-blocks
// aunque KITTY_WINDOW_ID se herede: los escapes gráficos necesitarían
// passthrough envuelto y tmux no reserva las celdas.
func supportsKittyGfx() bool {
	if os.Getenv("TMUX") != "" {
		return false
	}
	return os.Getenv("KITTY_WINDOW_ID") != "" ||
		strings.Contains(os.Getenv("TERM"), "kitty")
}

// pickCoverRenderer elige el renderer de carátulas para este terminal.
func pickCoverRenderer() coverRenderer {
	if supportsKittyGfx() {
		return kittyGfx{}
	}
	return halfBlocks{}
}
