package media

import (
	"bytes"
	"image"

	// Los formatos de carátula admitidos (los mismos que safeExt en mpris).
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)

// DecodeImage decodifica una carátula embebida (jpeg/png/gif).
func DecodeImage(data []byte) (image.Image, error) {
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
