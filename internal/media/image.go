package media

import (
	"bytes"
	"fmt"
	"image"

	// Los formatos de carátula admitidos (los mismos que safeExt en mpris).
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)

// maxDecodePixels acota la carátula a decodificar (~40 MP): un PNG malicioso
// declara dimensiones enormes en KB comprimidos y el Decode reservaría GBs.
// Ninguna carátula real se acerca; el tope solo frena bombas de descompresión.
const maxDecodePixels = 40 << 20

// DecodeImage decodifica una carátula embebida (jpeg/png/gif). Antes de
// decodificar valida las dimensiones declaradas: DecodeConfig solo lee la
// cabecera, sin reservar la imagen.
func DecodeImage(data []byte) (image.Image, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err == nil && (cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width*cfg.Height > maxDecodePixels) {
		return nil, fmt.Errorf("cover art dimensions out of bounds: %dx%d", cfg.Width, cfg.Height)
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	return img, err
}

// ScaleBox escala src a exactamente w×h px promediando la caja de píxeles de
// origen que cae en cada píxel destino (box average: sin el aliasing de
// saltarse píxeles al reducir). Al ampliar, la caja degenera en un píxel y
// queda el vecino más cercano — suficiente para carátulas diminutas.
func ScaleBox(src image.Image, w, h int) *image.RGBA {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	if sw == 0 || sh == 0 {
		return dst
	}
	for y := 0; y < h; y++ {
		y0 := b.Min.Y + y*sh/h
		y1 := b.Min.Y + (y+1)*sh/h
		if y1 <= y0 {
			y1 = y0 + 1
		}
		for x := 0; x < w; x++ {
			x0 := b.Min.X + x*sw/w
			x1 := b.Min.X + (x+1)*sw/w
			if x1 <= x0 {
				x1 = x0 + 1
			}
			var r, g, bl, n uint64
			for sy := y0; sy < y1; sy++ {
				for sx := x0; sx < x1; sx++ {
					pr, pg, pb, _ := src.At(sx, sy).RGBA()
					r += uint64(pr >> 8)
					g += uint64(pg >> 8)
					bl += uint64(pb >> 8)
					n++
				}
			}
			i := dst.PixOffset(x, y)
			dst.Pix[i+0] = uint8(r / n)
			dst.Pix[i+1] = uint8(g / n)
			dst.Pix[i+2] = uint8(bl / n)
			dst.Pix[i+3] = 0xff
		}
	}
	return dst
}
