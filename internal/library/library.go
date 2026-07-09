// Package library indexa archivos de audio (tags vía dhowden/tag) en SQLite
// y ofrece búsqueda y listado para la TUI y los comandos CLI.
package library

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode"

	"github.com/dhowden/tag"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
	"maly/internal/i18n"
	_ "modernc.org/sqlite"
)

var audioExts = map[string]bool{
	".mp3":  true,
	".flac": true,
	".ogg":  true,
	".opus": true,
	".m4a":  true,
	".wav":  true,
}

type Track struct {
	ID          int64
	Path        string
	Title       string
	Artist      string
	Album       string
	AlbumArtist string
	Genre       string
	TrackNo     int
	Year        int
}

// String es la forma "Artista — Título" usada en cola, paleta y status.
func (t Track) String() string {
	if t.Artist == "" {
		return t.Title
	}
	return t.Artist + " — " + t.Title
}

type Library struct {
	db *sql.DB
}

const schema = `
CREATE TABLE IF NOT EXISTS tracks (
	id           INTEGER PRIMARY KEY,
	path         TEXT UNIQUE NOT NULL,
	title        TEXT NOT NULL DEFAULT '',
	artist       TEXT NOT NULL DEFAULT '',
	album        TEXT NOT NULL DEFAULT '',
	album_artist TEXT NOT NULL DEFAULT '',
	genre        TEXT NOT NULL DEFAULT '',
	track_no     INTEGER NOT NULL DEFAULT 0,
	year         INTEGER NOT NULL DEFAULT 0,
	mtime        INTEGER NOT NULL DEFAULT 0,
	search_text  TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_tracks_artist ON tracks(artist);
CREATE INDEX IF NOT EXISTS idx_tracks_album  ON tracks(album);
CREATE TABLE IF NOT EXISTS playlists (
	id   INTEGER PRIMARY KEY,
	name TEXT UNIQUE NOT NULL
);
CREATE TABLE IF NOT EXISTS playlist_tracks (
	playlist_id INTEGER NOT NULL REFERENCES playlists(id) ON DELETE CASCADE,
	track_id    INTEGER NOT NULL REFERENCES tracks(id) ON DELETE CASCADE,
	pos         INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_pl_tracks ON playlist_tracks(playlist_id, pos);
`

// Open abre (o crea) la base de datos en dbPath.
func Open(dbPath string) (*Library, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.Tf("lib.mkdir", filepath.Dir(dbPath)), err)
	}
	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("%s: %w", i18n.T("lib.open_db"), err)
	}
	// modernc.org/sqlite no soporta bien conexiones concurrentes de escritura.
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("%s: %w", i18n.T("lib.schema"), err)
	}
	return &Library{db: db}, nil
}

func (l *Library) Close() error { return l.db.Close() }

type ScanResult struct {
	Added   int
	Updated int
	Removed int
	Errors  []string
}

// Scan recorre root, indexa audio nuevo o modificado y elimina de la base
// las entradas cuyos archivos ya no existen.
func (l *Library) Scan(root string) (ScanResult, error) {
	var res ScanResult
	info, err := os.Stat(root)
	if err != nil {
		return res, fmt.Errorf("%s: %w", i18n.Tf("lib.no_access", root), err)
	}
	if !info.IsDir() {
		return res, errors.New(i18n.Tf("lib.not_dir", root))
	}

	// mtimes ya indexados, para saltar archivos sin cambios.
	known := map[string]int64{}
	rows, err := l.db.Query(`SELECT path, mtime FROM tracks`)
	if err != nil {
		return res, err
	}
	for rows.Next() {
		var p string
		var m int64
		if err := rows.Scan(&p, &m); err != nil {
			rows.Close()
			return res, err
		}
		known[p] = m
	}
	rows.Close()

	seen := map[string]bool{}
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			res.Errors = append(res.Errors, err.Error())
			return nil
		}
		if d.IsDir() || !audioExts[strings.ToLower(filepath.Ext(path))] {
			return nil
		}
		seen[path] = true
		fi, err := d.Info()
		if err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("%s: %v", path, err))
			return nil
		}
		mtime := fi.ModTime().Unix()
		if old, ok := known[path]; ok && old == mtime {
			return nil
		}
		t := ReadTags(path)
		_, dbErr := l.db.Exec(`
			INSERT INTO tracks (path, title, artist, album, album_artist, genre, track_no, year, mtime, search_text)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(path) DO UPDATE SET
				title=excluded.title, artist=excluded.artist, album=excluded.album,
				album_artist=excluded.album_artist, genre=excluded.genre,
				track_no=excluded.track_no, year=excluded.year, mtime=excluded.mtime,
				search_text=excluded.search_text`,
			path, t.Title, t.Artist, t.Album, t.AlbumArtist, t.Genre, t.TrackNo, t.Year, mtime,
			Fold(t.Title+" "+t.Artist+" "+t.Album))
		if dbErr != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("%s: %v", path, dbErr))
			return nil
		}
		if _, existed := known[path]; existed {
			res.Updated++
		} else {
			res.Added++
		}
		return nil
	})
	if walkErr != nil {
		return res, walkErr
	}

	// Purgar archivos desaparecidos (solo los que estaban bajo root).
	for p := range known {
		if seen[p] {
			continue
		}
		if rel, err := filepath.Rel(root, p); err != nil || strings.HasPrefix(rel, "..") {
			continue
		}
		if _, err := l.db.Exec(`DELETE FROM tracks WHERE path = ?`, p); err == nil {
			res.Removed++
		}
	}
	return res, nil
}

// ReadTags lee los metadatos de un archivo; si falla, deriva el título del
// nombre del archivo para que igualmente entre en la biblioteca. La usa Scan
// y el demonio para pistas agregadas por ruta que no están en la biblioteca.
func ReadTags(path string) Track {
	t := Track{
		Path:  path,
		Title: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
	}
	f, err := os.Open(path)
	if err != nil {
		return t
	}
	defer f.Close()
	md, err := tag.ReadFrom(f)
	if err != nil {
		return t
	}
	if v := strings.TrimSpace(md.Title()); v != "" {
		t.Title = v
	}
	t.Artist = strings.TrimSpace(md.Artist())
	t.Album = strings.TrimSpace(md.Album())
	t.AlbumArtist = strings.TrimSpace(md.AlbumArtist())
	t.Genre = strings.TrimSpace(md.Genre())
	t.TrackNo, _ = md.Track()
	t.Year = md.Year()
	return t
}

const trackCols = `id, path, title, artist, album, album_artist, genre, track_no, year`

func scanTrack(row interface{ Scan(...any) error }) (Track, error) {
	var t Track
	err := row.Scan(&t.ID, &t.Path, &t.Title, &t.Artist, &t.Album, &t.AlbumArtist, &t.Genre, &t.TrackNo, &t.Year)
	return t, err
}

func (l *Library) collect(query string, args ...any) ([]Track, error) {
	rows, err := l.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Track
	for rows.Next() {
		t, err := scanTrack(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// foldPool reparte transformers reutilizables: transform.Transformer tiene
// estado interno y no es seguro entre goroutines, y Fold se llama en paralelo
// (el demonio escanea sin d.mu mientras search/status siguen respondiendo).
var foldPool = sync.Pool{
	New: func() any {
		return transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	},
}

// Fold normaliza texto para búsqueda: minúsculas y sin diacríticos, de modo
// que "aurea" encuentre "Áurea" aunque el tag venga en NFD.
func Fold(s string) string {
	t := foldPool.Get().(transform.Transformer)
	out, _, err := transform.String(t, s) // transform.String hace Reset primero
	foldPool.Put(t)
	if err != nil {
		out = s
	}
	return strings.ToLower(out)
}

// Search busca cada palabra de q (normalizada) en título, artista o álbum.
func (l *Library) Search(q string) ([]Track, error) {
	words := strings.Fields(Fold(q))
	if len(words) == 0 {
		return l.All()
	}
	var conds []string
	var args []any
	for _, w := range words {
		conds = append(conds, `search_text LIKE ?`)
		args = append(args, "%"+w+"%")
	}
	return l.collect(`SELECT `+trackCols+` FROM tracks WHERE `+strings.Join(conds, " AND ")+
		` ORDER BY artist COLLATE NOCASE, album COLLATE NOCASE, track_no, title COLLATE NOCASE LIMIT 500`, args...)
}

// All devuelve toda la biblioteca ordenada Artista > Álbum > pista.
func (l *Library) All() ([]Track, error) {
	return l.collect(`SELECT ` + trackCols + ` FROM tracks
		ORDER BY artist COLLATE NOCASE, album COLLATE NOCASE, track_no, title COLLATE NOCASE`)
}

// Get devuelve una pista por id.
func (l *Library) Get(id int64) (Track, error) {
	t, err := scanTrack(l.db.QueryRow(`SELECT `+trackCols+` FROM tracks WHERE id = ?`, id))
	if err == sql.ErrNoRows {
		return t, errors.New(i18n.Tf("lib.track_nf", id))
	}
	return t, err
}

// ByPath devuelve una pista por ruta exacta, si está indexada.
func (l *Library) ByPath(path string) (Track, bool) {
	t, err := scanTrack(l.db.QueryRow(`SELECT `+trackCols+` FROM tracks WHERE path = ?`, path))
	return t, err == nil
}

// Count devuelve el total de pistas indexadas.
func (l *Library) Count() (int, error) {
	var n int
	err := l.db.QueryRow(`SELECT COUNT(*) FROM tracks`).Scan(&n)
	return n, err
}
