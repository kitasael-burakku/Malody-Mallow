package library

import (
	"database/sql"
	"errors"

	"maly/internal/i18n"
)

type Playlist struct {
	ID     int64
	Name   string
	Tracks int
}

// Playlists lista todas las playlists con su número de pistas.
func (l *Library) Playlists() ([]Playlist, error) {
	rows, err := l.db.Query(`
		SELECT p.id, p.name, COUNT(pt.track_id)
		FROM playlists p LEFT JOIN playlist_tracks pt ON pt.playlist_id = p.id
		GROUP BY p.id ORDER BY p.name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Playlist
	for rows.Next() {
		var p Playlist
		if err := rows.Scan(&p.ID, &p.Name, &p.Tracks); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (l *Library) playlistID(name string) (int64, error) {
	var id int64
	err := l.db.QueryRow(`SELECT id FROM playlists WHERE name = ?`, name).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, errors.New(i18n.Tf("lib.pl_nf", name))
	}
	return id, err
}

// CreatePlaylist crea una playlist vacía.
func (l *Library) CreatePlaylist(name string) error {
	if name == "" {
		return errors.New(i18n.T("lib.pl_name"))
	}
	res, err := l.db.Exec(`INSERT OR IGNORE INTO playlists (name) VALUES (?)`, name)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errors.New(i18n.Tf("lib.pl_exists", name))
	}
	return nil
}

// DeletePlaylist elimina una playlist y sus entradas.
func (l *Library) DeletePlaylist(name string) error {
	id, err := l.playlistID(name)
	if err != nil {
		return err
	}
	_, err = l.db.Exec(`DELETE FROM playlists WHERE id = ?`, id)
	return err
}

// AddToPlaylist agrega pistas (por id) al final de la playlist.
func (l *Library) AddToPlaylist(name string, trackIDs []int64) error {
	id, err := l.playlistID(name)
	if err != nil {
		return err
	}
	var pos int
	l.db.QueryRow(`SELECT COALESCE(MAX(pos), 0) FROM playlist_tracks WHERE playlist_id = ?`, id).Scan(&pos)
	for _, tid := range trackIDs {
		pos++
		if _, err := l.db.Exec(
			`INSERT INTO playlist_tracks (playlist_id, track_id, pos) VALUES (?, ?, ?)`,
			id, tid, pos); err != nil {
			return err
		}
	}
	return nil
}

// PlaylistTracks devuelve las pistas de una playlist en orden.
func (l *Library) PlaylistTracks(name string) ([]Track, error) {
	id, err := l.playlistID(name)
	if err != nil {
		return nil, err
	}
	return l.collect(`SELECT `+qualCols("t")+` FROM playlist_tracks pt
		JOIN tracks t ON t.id = pt.track_id
		WHERE pt.playlist_id = ? ORDER BY pt.pos`, id)
}

func qualCols(alias string) string {
	return alias + ".id, " + alias + ".path, " + alias + ".title, " + alias + ".artist, " +
		alias + ".album, " + alias + ".album_artist, " + alias + ".genre, " +
		alias + ".track_no, " + alias + ".year"
}
