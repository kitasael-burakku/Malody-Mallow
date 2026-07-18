package tui

import (
	"fmt"
	"image"
	"strings"

	"maly/internal/media"
)

// coverRenderer convierte la carátula decodificada en líneas listas para
// incrustar en el View; cada implementación escala a su propia densidad. La
// implementación por defecto son half-blocks ANSI, que funcionan en
// cualquier terminal truecolor; en kitty se usa su protocolo gráfico
// (kittyGfx, artkitty.go). pickCoverRenderer elige.
type coverRenderer interface {
	render(img image.Image, wCells, hCells int) []string
}

// halfBlocks dibuja 2 píxeles verticales por celda con "▀": el de arriba en
// el color de tinta y el de abajo en el de fondo. Emite SGR crudo — un
// lipgloss.Style por celda infla la salida una barbaridad — y solo cambia de
// secuencia cuando cambia el par de colores.
type halfBlocks struct{}

func (halfBlocks) render(src image.Image, wCells, hCells int) []string {
	img := media.ScaleBox(src, wCells, hCells*2)
	b := img.Bounds()
	rgb := func(x, y int) (uint32, uint32, uint32) {
		r, g, bl, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
		return r >> 8, g >> 8, bl >> 8
	}
	lines := make([]string, hCells)
	var sb strings.Builder
	for row := 0; row < hCells; row++ {
		sb.Reset()
		var prevTop, prevBot [3]uint32
		for col := 0; col < wCells; col++ {
			tr, tg, tb := rgb(col, row*2)
			br, bg, bb := rgb(col, row*2+1)
			top, bot := [3]uint32{tr, tg, tb}, [3]uint32{br, bg, bb}
			if col == 0 || top != prevTop || bot != prevBot {
				fmt.Fprintf(&sb, "\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm",
					tr, tg, tb, br, bg, bb)
				prevTop, prevBot = top, bot
			}
			sb.WriteRune('▀')
		}
		sb.WriteString("\x1b[0m")
		lines[row] = sb.String()
	}
	return lines
}
