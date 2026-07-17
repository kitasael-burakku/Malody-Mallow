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

// IsAudio dice si la ruta tiene una extensión de audio que maly indexa; es
// la fuente única del filtro (la usan Scan y el demonio al resolver rutas).
func IsAudio(path string) bool {
	return audioExts[strings.ToLower(filepath.Ext(path))]
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
	Duration    float64 // segundos; 0 = aún no aprendida (ver SetDuration)
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
	search_text  TEXT NOT NULL DEFAULT '',
	duration     REAL NOT NULL DEFAULT 0
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
	// 0700: la biblioteca revela hábitos de escucha; los archivos de SQLite
	// dentro (db/-wal/-shm) quedan cubiertos por el directorio.
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
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
	// Migración para bases anteriores a 0.6.0 (CREATE IF NOT EXISTS no
	// agrega columnas); si la columna ya existe el ALTER falla y se ignora.
	db.Exec(`ALTER TABLE tracks ADD COLUMN duration REAL NOT NULL DEFAULT 0`)
	return &Library{db: db}, nil
}

func (l *Library) Close() error { return l.db.Close() }

type ScanResult struct {
	Added   int
	Updated int
	Removed int
	Errors  []string
}

// scanBatchSize es cuántas escrituras van en cada transacción del escaneo:
// un fsync por lote en vez de uno por pista (cada Exec suelto es su propia
// transacción implícita). El flush de un lote son milisegundos, así que una
// búsqueda concurrente espera como mucho eso; entre lotes la conexión queda
// libre.
const scanBatchSize = 500

const upsertTrack = `
	INSERT INTO tracks (path, title, artist, album, album_artist, genre, track_no, year, mtime, search_text)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(path) DO UPDATE SET
		title=excluded.title, artist=excluded.artist, album=excluded.album,
		album_artist=excluded.album_artist, genre=excluded.genre,
		track_no=excluded.track_no, year=excluded.year, mtime=excluded.mtime,
		search_text=excluded.search_text`

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

	// Las escrituras van en lotes dentro de transacciones cortas. NUNCA una
	// transacción única para todo el escaneo: database/sql fija la conexión
	// al Tx y, con la única conexión retenida de punta a punta, Search y
	// ByPath quedarían bloqueados hasta el final (el mismo congelamiento que
	// se arregló al sacar a scan de d.mu, a otro nivel). Leer tags —el costo
	// dominante, IO de archivo— ocurre siempre fuera de toda transacción.
	type pending struct {
		t       Track
		mtime   int64
		fold    string
		existed bool
	}
	var batch []pending
	flush := func() {
		if len(batch) == 0 {
			return
		}
		defer func() { batch = batch[:0] }()
		fail := func(err error) {
			for _, p := range batch {
				res.Errors = append(res.Errors, fmt.Sprintf("%s: %v", p.t.Path, err))
			}
		}
		tx, err := l.db.Begin()
		if err != nil {
			fail(err)
			return
		}
		stmt, err := tx.Prepare(upsertTrack)
		if err != nil {
			tx.Rollback()
			fail(err)
			return
		}
		added, updated := 0, 0
		for _, p := range batch {
			t := p.t
			if _, err := stmt.Exec(t.Path, t.Title, t.Artist, t.Album, t.AlbumArtist,
				t.Genre, t.TrackNo, t.Year, p.mtime, p.fold); err != nil {
				res.Errors = append(res.Errors, fmt.Sprintf("%s: %v", t.Path, err))
				continue
			}
			if p.existed {
				updated++
			} else {
				added++
			}
		}
		stmt.Close()
		// Los contadores se suman solo si el lote quedó aplicado de verdad.
		if err := tx.Commit(); err != nil {
			fail(err)
			return
		}
		res.Added += added
		res.Updated += updated
	}

	seen := map[string]bool{}
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			res.Errors = append(res.Errors, err.Error())
			return nil
		}
		if d.IsDir() || !IsAudio(path) {
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
		_, existed := known[path]
		batch = append(batch, pending{t: t, mtime: mtime, existed: existed,
			fold: Fold(t.Title + " " + t.Artist + " " + t.Album)})
		if len(batch) >= scanBatchSize {
			flush()
		}
		return nil
	})
	flush() // lo indexado hasta aquí se conserva aunque el walk haya fallado
	if walkErr != nil {
		return res, walkErr
	}

	// Purgar archivos desaparecidos (solo los que estaban bajo root), también
	// por lotes y contando solo lo confirmado.
	var gone []string
	for p := range known {
		if seen[p] {
			continue
		}
		// Fuera de root es rel == ".." o "../…": el prefijo solo, sin el
		// separador, también taparía entradas legítimas bajo un directorio
		// cuyo nombre empiece con ".." literal (p. ej. root/..covers/).
		rel, err := filepath.Rel(root, p)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		gone = append(gone, p)
	}
	for len(gone) > 0 {
		n := min(scanBatchSize, len(gone))
		chunk := gone[:n]
		gone = gone[n:]
		tx, err := l.db.Begin()
		if err != nil {
			res.Errors = append(res.Errors, err.Error())
			break
		}
		removed := 0
		for _, p := range chunk {
			if _, err := tx.Exec(`DELETE FROM tracks WHERE path = ?`, p); err == nil {
				removed++
			}
		}
		if err := tx.Commit(); err != nil {
			res.Errors = append(res.Errors, err.Error())
			continue
		}
		res.Removed += removed
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

const trackCols = `id, path, title, artist, album, album_artist, genre, track_no, year, duration`

func scanTrack(row interface{ Scan(...any) error }) (Track, error) {
	var t Track
	err := row.Scan(&t.ID, &t.Path, &t.Title, &t.Artist, &t.Album, &t.AlbumArtist, &t.Genre, &t.TrackNo, &t.Year, &t.Duration)
	return t, err
}

// SetDuration guarda la duración de una pista. Los tags no la traen
// (dhowden no decodifica audio), así que se aprende perezosamente: el
// demonio la escribe cuando mpv la reporta al reproducir. El upsert del
// escaneo no la toca, así que un re-scan la conserva.
func (l *Library) SetDuration(path string, secs float64) error {
	_, err := l.db.Exec(`UPDATE tracks SET duration = ? WHERE path = ?`, secs, path)
	return err
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

// likeEscaper neutraliza los comodines de LIKE en el texto del usuario: sin
// esto, buscar "100%" matchea cualquier cosa que empiece con "100".
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// Search busca cada palabra de q (normalizada) en título, artista o álbum.
// Sin límite a propósito: play/add/playlist add operan sobre TODO lo que
// matchea (un LIMIT los capaba en silencio), igual que la consulta vacía cae
// en All. Quien necesite pocos resultados corta él mismo (completeTracks).
func (l *Library) Search(q string) ([]Track, error) {
	words := strings.Fields(Fold(q))
	if len(words) == 0 {
		return l.All()
	}
	var conds []string
	var args []any
	for _, w := range words {
		conds = append(conds, `search_text LIKE ? ESCAPE '\'`)
		args = append(args, "%"+likeEscaper.Replace(w)+"%")
	}
	return l.collect(`SELECT `+trackCols+` FROM tracks WHERE `+strings.Join(conds, " AND ")+
		` ORDER BY artist COLLATE NOCASE, album COLLATE NOCASE, track_no, title COLLATE NOCASE`, args...)
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
