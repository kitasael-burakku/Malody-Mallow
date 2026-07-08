package mpris

import (
	"crypto/sha1"
	"encoding/hex"
	"net/url"
	"os"
	"path/filepath"

	"github.com/dhowden/tag"
)

// artCache extrae la carátula embebida de las pistas y la sirve como
// archivo (los clientes MPRIS esperan una URL, no bytes) en un directorio
// del runtime dir: tmpfs, así que se limpia solo al apagar. El archivo se
// nombra por el hash de sus bytes: las pistas de un mismo álbum comparten
// carátula y archivo.
type artCache struct {
	dir  string
	memo map[string]string // ruta de la pista → URL file:// ("" = sin carátula)
}

// newArtCache prepara el directorio; si no se puede crear devuelve nil y
// el Metadata sale sin mpris:artUrl, como hasta ahora.
func newArtCache(dir string) *artCache {
	if dir == "" || os.MkdirAll(dir, 0o700) != nil {
		return nil
	}
	return &artCache{dir: dir, memo: map[string]string{}}
}

// urlFor devuelve la URL file:// de la carátula embebida de path, o "" si
// no tiene (o no se pudo extraer). Cada pista se lee del disco una sola
// vez; el "" también se memoiza. El llamador serializa (corre bajo s.mu).
func (c *artCache) urlFor(path string) string {
	if u, ok := c.memo[path]; ok {
		return u
	}
	u := c.extract(path)
	c.memo[path] = u
	return u
}

func (c *artCache) extract(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	md, err := tag.ReadFrom(f)
	if err != nil {
		return ""
	}
	pic := md.Picture()
	if pic == nil || len(pic.Data) == 0 {
		return ""
	}
	sum := sha1.Sum(pic.Data)
	name := hex.EncodeToString(sum[:])
	if pic.Ext != "" {
		name += "." + pic.Ext
	}
	file := filepath.Join(c.dir, name)
	if _, err := os.Stat(file); err != nil {
		// escritura atómica: que un cliente leyendo justo ahora no pueda
		// ver una imagen a medias
		tmp := file + ".tmp"
		if os.WriteFile(tmp, pic.Data, 0o600) != nil || os.Rename(tmp, file) != nil {
			return ""
		}
	}
	return (&url.URL{Scheme: "file", Path: file}).String()
}

// close borra el directorio de carátulas; nil-safe como el resto.
func (c *artCache) close() {
	if c != nil {
		os.RemoveAll(c.dir)
	}
}
