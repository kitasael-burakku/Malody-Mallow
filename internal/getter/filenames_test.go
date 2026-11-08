package getter

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"maly/internal/library"
	"maly/internal/media"
)

// Estas integraciones usan el ejecutable real y JSON local, sin extraer una
// URL ni leer config/cookies del usuario. Sin yt-dlp se saltan explícitamente;
// TestCommand sigue verificando los argumentos sin dependencias externas.
func localYTDLP(t *testing.T, opts Opts, info map[string]any, extra ...string) []byte {
	t.Helper()
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		t.Skip("integración local: falta yt-dlp")
	}
	dir := t.TempDir()
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "info.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if opts.PathsFile != "" {
		// La CLI/TUI crean un archivo vacío para CADA operación; yt-dlp
		// escribe con append, así que el arnés debe hacer lo mismo.
		if err := os.WriteFile(opts.PathsFile, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	cmd := Command(opts)
	// Sustituir SOLO la entrada de red por --load-info-json; conservar todos
	// los argumentos de producción, incluidos los de nombres y postproceso.
	args := append([]string{"--ignore-config", "--no-cache-dir", "--no-clean-info-json", "--enable-file-urls"}, cmd.Args[1:len(cmd.Args)-2]...)
	args = append(args, extra...)
	cmd.Args = append(cmd.Args[:1], append(args, "--load-info-json", path)...)
	cmd.Env = append(os.Environ(), "HOME="+dir, "XDG_CONFIG_HOME="+dir, "XDG_CACHE_HOME="+dir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("yt-dlp local: %v\n%s", err, output)
	}
	return output
}

func localInfo(title, artist, id, ext, source string) map[string]any {
	return map[string]any{
		"id": id, "title": title, "artist": artist, "uploader": "Canal",
		"ext": ext, "url": source, "format_id": "fixture", "vcodec": "none",
		"extractor": "youtube", "extractor_key": "Youtube",
		"webpage_url": source, // incluso el respaldo tras un fallo queda sin red
	}
}

func simulatedFilename(t *testing.T, opts Opts, info map[string]any) string {
	t.Helper()
	report := filepath.Join(t.TempDir(), "filename.json")
	localYTDLP(t, opts, info, "--simulate", "--print-to-file", "video:%(filename)j", report)
	data, err := os.ReadFile(report)
	if err != nil {
		t.Fatal(err)
	}
	var name string
	if err := json.Unmarshal(data, &name); err != nil {
		t.Fatalf("ruta JSON %q: %v", data, err)
	}
	return name
}

func TestYTDLPFilenameBytes(t *testing.T) {
	for _, tc := range []struct {
		name, title, artist string
		shortened           bool
	}{
		{"convencional", "Song", "Artist", false},
		{"japonés corto", "夜に駆ける", "YOASOBI", false},
		{"emoji corto", "Café 😺🎵", "音楽", false},
		{"límite exacto", strings.Repeat("x", 216), "A", false},
		{"un byte de más", strings.Repeat("x", 217), "A", true},
		{"título largo", strings.Repeat("Title", 200), "Artist", true},
		{"artista largo", "Song", strings.Repeat("Artista", 200), true},
		{"japonés largo", strings.Repeat("日本語", 100), "歌手", true},
		{"emoji largo", strings.Repeat("🎵😺", 100), "Café", true},
		// S debe aplicarse antes del límite: '/' se convierte en ⧸ (3 bytes).
		{"saneamiento expande bytes", strings.Repeat("a/", 100), "A", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const id = "-8C3Dqglk0E"
			name := filepath.Base(simulatedFilename(t, Opts{Dir: t.TempDir()}, localInfo(tc.title, tc.artist, id, "webm", "file:///unused.webm")))
			if !utf8.ValidString(name) || strings.ContainsRune(name, utf8.RuneError) || len(name) > 255 {
				t.Fatalf("nombre inválido: %d bytes, %q", len(name), name)
			}
			stem := strings.TrimSuffix(name, filepath.Ext(name))
			if len(stem) > 220 {
				t.Fatalf("sin margen para intermedios: %d bytes", len(stem))
			}
			for _, suffix := range []string{".mp3", ".temp.mp3", ".webm.part", ".webm.part-Frag18446744073709551615"} {
				if err := os.WriteFile(filepath.Join(t.TempDir(), stem+suffix), nil, 0o600); err != nil {
					t.Fatalf("el sistema de archivos rechazó el sufijo %s: %v", suffix, err)
				}
			}
			if tc.shortened {
				if !strings.HasPrefix(stem, "["+id+"] ") {
					t.Fatalf("se perdió la identidad del video: %q", stem)
				}
			} else if want := tc.artist + " - " + tc.title; stem != want {
				t.Fatalf("se alteró el nombre válido: %q; quería %q", stem, want)
			}
		})
	}
}

func TestYTDLPFilenameCollisionsAndExtensions(t *testing.T) {
	dir := t.TempDir()
	prefix := strings.Repeat("同じタイトル🎵", 70)
	seen := map[string]bool{}
	for i, id := range []string{"abcdefghijA", "abcdefghijB"} {
		var stem string
		for _, ext := range []string{"webm", "m4a", "opus", "wav", "flac"} {
			name := simulatedFilename(t, Opts{Dir: dir}, localInfo(fmt.Sprintf("%s-%d", prefix, i), "歌手", id, ext, "file:///unused."+ext))
			got := strings.TrimSuffix(name, filepath.Ext(name))
			if stem != "" && got != stem {
				t.Fatalf("la extensión cambió la identidad: %q != %q", got, stem)
			}
			stem = got
		}
		if seen[stem] {
			t.Fatal("colisión entre títulos que difieren después del recorte")
		}
		seen[stem] = true
	}
}

func TestYTDLPPlaylistFilenameBytes(t *testing.T) {
	for _, long := range []bool{false, true} {
		t.Run(fmt.Sprint(long), func(t *testing.T) {
			title, playlist := "曲", "お気に入り"
			if long {
				title, playlist = strings.Repeat(title, 300), strings.Repeat(playlist, 100)
			}
			info := map[string]any{
				"_type": "playlist", "id": "PLfixture", "title": playlist,
				"extractor": "youtube:tab", "extractor_key": "YoutubeTab",
				"entries": []map[string]any{localInfo(title, "歌手", "abcdefghijk", "m4a", "file:///unused.m4a")},
			}
			dir := t.TempDir()
			name := simulatedFilename(t, Opts{Dir: dir, Playlist: true, PlaylistSubdir: true}, info)
			rel, err := filepath.Rel(dir, name)
			if err != nil {
				t.Fatal(err)
			}
			parts := strings.Split(rel, string(filepath.Separator))
			if len(parts) != 2 {
				t.Fatalf("subdirectorio inesperado: %q", rel)
			}
			for _, part := range parts {
				if !utf8.ValidString(part) || len(part) > 255 {
					t.Fatalf("componente inválido: %q", part)
				}
			}
			if !strings.HasPrefix(parts[1], "01 - ") {
				t.Fatalf("se perdió el orden de playlist: %q", parts[1])
			}
			if !long && (parts[0] != playlist || !strings.HasPrefix(parts[1], "01 - 歌手 - 曲.")) {
				t.Fatalf("se alteró el nombre habitual de playlist: %q", rel)
			}
		})
	}
}

func TestYTDLPLocalAudioThumbnailAndExisting(t *testing.T) {
	for _, tool := range []string{"yt-dlp", "ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("integración local: falta %s", tool)
		}
	}
	fixtures := t.TempDir()
	source := filepath.Join(fixtures, "audio.wav")
	if out, err := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", "anullsrc", "-t", "0.15", source).CombinedOutput(); err != nil {
		t.Fatalf("generar audio: %v\n%s", err, out)
	}
	cover := filepath.Join(fixtures, "cover.png")
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	f, err := os.Create(cover)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	f.Close()
	webp := filepath.Join(fixtures, "cover.webp")
	if out, err := exec.Command("ffmpeg", "-v", "error", "-i", cover, "-lossless", "1", webp).CombinedOutput(); err != nil {
		t.Fatalf("generar portada WebP: %v\n%s", err, out)
	}
	fileURL := func(path string) string { return (&url.URL{Scheme: "file", Path: path}).String() }
	for _, long := range []bool{false, true} {
		t.Run(fmt.Sprint(long), func(t *testing.T) {
			title, artist := "曲🎵", "歌手"
			if long {
				title, artist = strings.Repeat(title, 100), strings.Repeat(artist, 100)
			}
			info := localInfo(title, artist, "-8C3Dqglk0E", "wav", fileURL(source))
			thumbnail := cover
			if long {
				thumbnail = webp // fuerza también la conversión WebP → PNG
			}
			info["thumbnails"] = []map[string]any{{"url": fileURL(thumbnail)}}
			dir := t.TempDir()
			opts := Opts{Dir: dir, PathsFile: filepath.Join(t.TempDir(), "paths.txt")}
			before, _ := Snapshot(dir)
			localYTDLP(t, opts, info)
			out, err := Collect(dir, before, opts.PathsFile, nil)
			if err != nil || out.New != 1 || len(out.Paths) != 1 {
				t.Fatalf("descarga sin audio: %+v, %v", out, err)
			}
			path := out.Paths[0]
			track := library.ReadTags(path)
			if track.Title != title || track.Artist != artist {
				t.Fatalf("metadatos truncados: título %d/%d bytes, artista %d/%d", len(track.Title), len(title), len(track.Artist), len(artist))
			}
			if len(media.ReadEmbedded(path).Picture) == 0 {
				t.Fatal("falta portada embebida")
			}
			lib, err := library.Create(filepath.Join(t.TempDir(), "library.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer lib.Close()
			if _, err := lib.Scan(dir, nil); err != nil {
				t.Fatal(err)
			}
			if indexed, ok := lib.ByPath(path); !ok || indexed.Title != title || indexed.Artist != artist {
				t.Fatal("la biblioteca no conserva la ruta y los metadatos completos")
			}
			entries, _ := os.ReadDir(dir)
			if len(entries) != 1 || !strings.HasSuffix(entries[0].Name(), ".mp3") || len(entries[0].Name()) > 255 {
				t.Fatalf("archivos inesperados después del postproceso: %v", entries)
			}
			// Descargar otro video con el MISMO prefijo largo no debe tocar
			// el primero. Un centinela permite detectar cualquier escritura.
			original, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			sentinel := string(original) + "audio existente: no sobrescribir"
			if err := os.WriteFile(path, []byte(sentinel), 0o600); err != nil {
				t.Fatal(err)
			}
			if long {
				info["id"] = "abcdefghijk"
				info["title"] = title + "otro final"
				localYTDLP(t, opts, info)
				data, _ := os.ReadFile(path)
				if string(data) != sentinel {
					t.Fatal("otro video sobrescribió el archivo existente")
				}
				info["id"], info["title"] = "-8C3Dqglk0E", title
			}
			// Una repetición conserva audio y tags. yt-dlp vuelve a ejecutar
			// sus postprocesadores sobre archivos saltados (contrato A-07).
			if err := os.WriteFile(path, original, 0o600); err != nil {
				t.Fatal(err)
			}
			before, _ = Snapshot(dir)
			localYTDLP(t, opts, info)
			track = library.ReadTags(path)
			if track.Title != title || track.Artist != artist || len(media.ReadEmbedded(path).Picture) == 0 {
				t.Fatal("la repetición dañó metadatos o portada")
			}
			out, err = Collect(dir, before, opts.PathsFile, nil)
			if err != nil || out.New != 0 || out.Existing != 1 {
				t.Fatalf("no reconoció la descarga existente: %+v, %v", out, err)
			}
		})
	}
}
