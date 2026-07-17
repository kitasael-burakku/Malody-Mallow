// Package media extrae lo que las pistas llevan embebido (carátula, letras)
// y resuelve letras externas .lrc. Lo comparten MPRIS (mpris:artUrl) y la
// capa "Ahora suena" de la TUI, para que cada pista se lea del disco una
// sola vez por consumidor.
package media

import (
	"os"

	"github.com/dhowden/tag"
)

// Embedded reúne lo que dhowden/tag saca de un archivo en una sola pasada.
type Embedded struct {
	Picture    []byte // carátula embebida; nil = sin carátula
	PictureExt string // extensión que reporta el tag, cruda ("jpg", "png"…)
	Lyrics     string // letras embebidas (USLT / ©lyr / LYRICS); "" = sin
}

// ReadEmbedded lee los tags de path. Es best-effort: en cualquier error
// devuelve el valor cero — carátula y letras son siempre opcionales.
func ReadEmbedded(path string) Embedded {
	f, err := os.Open(path)
	if err != nil {
		return Embedded{}
	}
	defer f.Close()
	md, err := tag.ReadFrom(f)
	if err != nil {
		return Embedded{}
	}
	var e Embedded
	if pic := md.Picture(); pic != nil && len(pic.Data) > 0 {
		e.Picture, e.PictureExt = pic.Data, pic.Ext
	}
	e.Lyrics = md.Lyrics()
	return e
}
