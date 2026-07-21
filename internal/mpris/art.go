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

// maxArtBytes acota el caché. El runtime dir es tmpfs, o sea RAM compartida con
// todo el escritorio: sin tope, una sesión larga saltando entre álbumes con
// carátulas de varios MB lo llena, y entonces empieza a fallar lo de los demás
// (PipeWire, units de usuario, portales XDG), no lo de maly.
const maxArtBytes = 32 << 20

// cachedArt es una carátula ya escrita en disco.
type cachedArt struct {
	file  string // ruta completa dentro de artCache.dir
	url   string
	bytes int64
}

// artCache extrae la carátula embebida de las pistas y la sirve como
// archivo (los clientes MPRIS esperan una URL, no bytes) en un directorio
// del runtime dir. El archivo se nombra por el hash de sus bytes: las pistas
// de un mismo álbum comparten carátula y archivo.
//
// files lleva el orden de creación para poder evictar por el frente cuando se
// pasa de maxArtBytes; antes solo se vaciaba en close(), que un SIGKILL o un
// SIGHUP no llegan a ejecutar nunca.
type artCache struct {
	dir   string
	memo  map[string]string // ruta de la pista → URL file:// ("" = sin carátula)
	files []cachedArt       // en orden de creación; se evicta por el frente
	bytes int64
	max   int64 // tope efectivo; campo y no const para poder bajarlo en tests
}

// newArtCache prepara el directorio; si no se puede crear devuelve nil y
// el Metadata sale sin mpris:artUrl, como hasta ahora.
func newArtCache(dir string) *artCache {
	if dir == "" {
		return nil
	}
	// Empezar de cero: si la sesión anterior murió sin ejecutar close() el
	// directorio sigue lleno, y esos archivos no están en files — no contarían
	// para el tope y se acumularían entre arranques. El caché es dato derivado:
	// tirarlo no cuesta nada.
	os.RemoveAll(dir)
	if os.MkdirAll(dir, 0o700) != nil {
		return nil
	}
	return &artCache{dir: dir, memo: map[string]string{}, max: maxArtBytes}
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
	c.evict()
	return u
}

// evict borra las carátulas más antiguas hasta bajar del tope. Nunca toca la
// última: es la de la pista que suena, justo la URL que los clientes MPRIS
// acaban de recibir en mpris:artUrl y están a punto de leer.
func (c *artCache) evict() {
	for c.bytes > c.max && len(c.files) > 1 {
		old := c.files[0]
		c.files = c.files[1:]
		c.bytes -= old.bytes
		os.Remove(old.file)
		// El memo es muchos-a-uno (varias pistas comparten carátula): hay que
		// purgar TODAS las entradas que apuntaban al archivo borrado, o urlFor
		// seguiría devolviendo la URL de algo que ya no existe.
		for k, v := range c.memo {
			if v == old.url {
				delete(c.memo, k)
			}
		}
	}
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
	// Las pistas de un álbum comparten carátula y por tanto archivo (el nombre
	// es el hash del contenido): si ya está contabilizado no se duplica ni la
	// entrada ni los bytes, y conserva su antigüedad para la evicción.
	for _, a := range c.files {
		if a.file == file {
			return a.url
		}
	}
	if _, err := os.Stat(file); err != nil {
		// escritura atómica: que un cliente leyendo justo ahora no pueda
		// ver una imagen a medias
		tmp := file + ".tmp"
		if os.WriteFile(tmp, emb.Picture, 0o600) != nil || os.Rename(tmp, file) != nil {
			return ""
		}
	}
	u := (&url.URL{Scheme: "file", Path: file}).String()
	c.files = append(c.files, cachedArt{file: file, url: u, bytes: int64(len(emb.Picture))})
	c.bytes += int64(len(emb.Picture))
	return u
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
