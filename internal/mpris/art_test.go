package mpris

import (
	"bytes"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"maly/internal/ipc"
)

// id3v2WithCover fabrica el prefijo ID3v2.3 con un frame APIC que contiene
// img como carátula PNG. Anteponerlo a cualquier contenido da un "MP3" del
// que dhowden/tag extrae la imagen.
func id3v2WithCover(img []byte) []byte {
	body := []byte{0} // codificación latin-1
	body = append(body, "image/png"...)
	body = append(body, 0)
	body = append(body, 3) // tipo: portada frontal
	body = append(body, 0) // descripción vacía
	body = append(body, img...)

	frame := []byte("APIC")
	n := len(body)
	// en v2.3 el tamaño del frame es big-endian normal, no synchsafe
	frame = append(frame, byte(n>>24), byte(n>>16), byte(n>>8), byte(n))
	frame = append(frame, 0, 0) // sin flags
	frame = append(frame, body...)

	hdr := []byte("ID3")
	hdr = append(hdr, 3, 0, 0) // v2.3, sin flags
	n = len(frame)
	// el tamaño del tag sí es synchsafe (7 bits por byte)
	hdr = append(hdr, byte(n>>21&0x7f), byte(n>>14&0x7f), byte(n>>7&0x7f), byte(n&0x7f))
	return append(hdr, frame...)
}

func writeTrack(t *testing.T, path string, img []byte) {
	t.Helper()
	data := []byte("audio falso")
	if img != nil {
		data = append(id3v2WithCover(img), data...)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestArtCache(t *testing.T) {
	dir := t.TempDir()
	music := t.TempDir()
	img1 := []byte("\x89PNG-imagen-uno")
	img2 := []byte("\x89PNG-imagen-dos")
	a := filepath.Join(music, "a.mp3")
	b := filepath.Join(music, "b.mp3")
	c2 := filepath.Join(music, "c.mp3")
	plain := filepath.Join(music, "sin-caratula.mp3")
	writeTrack(t, a, img1)
	writeTrack(t, b, img1) // mismo álbum: misma imagen
	writeTrack(t, c2, img2)
	writeTrack(t, plain, nil)

	c := newArtCache(filepath.Join(dir, "art"))
	if c == nil {
		t.Fatal("newArtCache devolvió nil")
	}

	u := c.urlFor(a)
	if !strings.HasPrefix(u, "file://") {
		t.Fatalf("urlFor(a) = %q", u)
	}
	parsed, err := url.Parse(u)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(parsed.Path)
	if err != nil {
		t.Fatalf("leyendo la carátula cacheada: %v", err)
	}
	if !bytes.Equal(got, img1) {
		t.Errorf("la carátula cacheada no coincide con la embebida")
	}

	// misma imagen → mismo archivo (dedupe por hash del contenido)
	if ub := c.urlFor(b); ub != u {
		t.Errorf("b no compartió carátula: %q vs %q", ub, u)
	}
	// imagen distinta → archivo distinto
	if uc := c.urlFor(c2); uc == u || !strings.HasPrefix(uc, "file://") {
		t.Errorf("urlFor(c) = %q", uc)
	}
	// sin carátula → "" (y memoizado)
	if up := c.urlFor(plain); up != "" {
		t.Errorf("urlFor(sin carátula) = %q", up)
	}
	// memoización: mismo resultado sin releer
	if again := c.urlFor(a); again != u {
		t.Errorf("segunda llamada: %q, quería %q", again, u)
	}
	// archivo desaparecido → ""
	if ug := c.urlFor(filepath.Join(music, "no-existe.mp3")); ug != "" {
		t.Errorf("urlFor(inexistente) = %q", ug)
	}

	c.close()
	if _, err := os.Stat(filepath.Join(dir, "art")); !os.IsNotExist(err) {
		t.Error("close no borró el directorio de carátulas")
	}
}

func TestServiceMetadataArt(t *testing.T) {
	music := t.TempDir()
	con := filepath.Join(music, "con.mp3")
	sin := filepath.Join(music, "sin.mp3")
	writeTrack(t, con, []byte("\x89PNG-portada"))
	writeTrack(t, sin, nil)

	s := &Service{art: newArtCache(filepath.Join(t.TempDir(), "art"))}
	stOf := func(p string) *ipc.Status {
		return &ipc.Status{Track: &ipc.TrackInfo{ID: 1, Path: p, Title: "x"}}
	}

	if _, ok := s.metadata(stOf(con))["mpris:artUrl"]; !ok {
		t.Error("falta mpris:artUrl con carátula embebida")
	}
	if _, ok := s.metadata(stOf(sin))["mpris:artUrl"]; ok {
		t.Error("mpris:artUrl presente sin carátula")
	}
	// sin pista y sin cache: no debe tocar nada
	if _, ok := s.metadata(&ipc.Status{})["mpris:artUrl"]; ok {
		t.Error("mpris:artUrl presente sin pista")
	}
	nilArt := &Service{}
	if _, ok := nilArt.metadata(stOf(con))["mpris:artUrl"]; ok {
		t.Error("mpris:artUrl presente con el cache deshabilitado")
	}
}

// TestSafeExt fija el filtro de extensiones del caché: el frame PIC de
// ID3v2.2 trae 3 bytes crudos del archivo en pic.Ext, así que solo pasan
// las conocidas; cualquier otra cosa se descarta y el archivo va sin
// extensión (nunca con separadores o bytes raros en el nombre).
func TestSafeExt(t *testing.T) {
	cases := map[string]string{
		"jpg":    "jpg",
		"JPG":    "jpg",
		"jpeg":   "jpeg",
		"png":    "png",
		"gif":    "gif",
		"":       "",
		"/..":    "",
		"../":    "",
		"a/b":    "",
		"\x00xy": "",
		"webp":   "", // no está en audioExts de MPRIS: mejor sin extensión
	}
	for in, want := range cases {
		if got := safeExt(in); got != want {
			t.Errorf("safeExt(%q) = %q, quería %q", in, got, want)
		}
	}
}
