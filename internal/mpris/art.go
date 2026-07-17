package mpris

import (
	"crypto/sha1"
	"encoding/hex"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"maly/internal/media"
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
	emb := media.ReadEmbedded(path)
	if len(emb.Picture) == 0 {
		return ""
	}
	sum := sha1.Sum(emb.Picture)
	name := hex.EncodeToString(sum[:])
	if ext := safeExt(emb.PictureExt); ext != "" {
		name += "." + ext
	}
	file := filepath.Join(c.dir, name)
	if _, err := os.Stat(file); err != nil {
		// escritura atómica: que un cliente leyendo justo ahora no pueda
		// ver una imagen a medias
		tmp := file + ".tmp"
		if os.WriteFile(tmp, emb.Picture, 0o600) != nil || os.Rename(tmp, file) != nil {
			return ""
		}
	}
	return (&url.URL{Scheme: "file", Path: file}).String()
}

// safeExt filtra la extensión que reporta el tag: casi todos los formatos la
// derivan de un allowlist en dhowden/tag, pero el frame PIC de ID3v2.2 copia
// 3 bytes crudos del archivo — un tag malicioso podría meter separadores o
// basura en el nombre del archivo del caché. Solo pasan las conocidas; sin
// extensión los clientes MPRIS igual detectan la imagen por contenido.
func safeExt(ext string) string {
	switch strings.ToLower(ext) {
	case "jpg", "jpeg", "png", "gif":
		return strings.ToLower(ext)
	}
	return ""
}

// close borra el directorio de carátulas; nil-safe como el resto.
func (c *artCache) close() {
	if c != nil {
		os.RemoveAll(c.dir)
	}
}
