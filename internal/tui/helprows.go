package tui

import (
	"strings"

	"maly/internal/i18n"
)

// HelpRows es la FUENTE ÚNICA de los atajos de la TUI: la comparten el modal
// `?` (helpView) y la sección de atajos de `maly -h` (cmd/maly/commands.go),
// igual que la tabla de comandos ya es fuente única de dispatch, help y
// completions.
//
// Antes la CLI llevaba su propia lista, escrita a mano y con las teclas
// literales: quedó desfasada (ctrl+g, el buscador de descargas de la 1.15.0,
// nunca llegó a aparecer) y encima mentía con un preset de `controls` o unas
// teclas propias en `[keys]`, porque anunciaba los defaults pasara lo que
// pasara. Con una sola lista, un atajo nuevo sale en los dos lados por
// construcción; lo fija TestHelpRowsParidad.
//
// keys son las teclas YA resueltas (defaults ← preset ← [keys]), o sea
// cfg.Keys después de config.Load. Cada fila es {teclas, descripción}, las
// dos listas para pintar: la descripción ya viene traducida y la tecla, con
// el espacio sustituido por su nombre (una celda en blanco no se ve).
func HelpRows(keys map[string]string) [][2]string {
	k := func(action string) string { return keyLabel(keys[action]) }
	pair := func(a, b string) string { return k(a) + " / " + k(b) }

	return [][2]string{
		{k("play_pause"), i18n.T("help.play_pause")},
		{pair("next", "prev"), i18n.T("help.next_prev")},
		{pair("vol_up", "vol_down"), i18n.T("help.volume")},
		{pair("seek_forward", "seek_back"), i18n.T("help.seek")},
		{k("switch_panel"), i18n.T("help.switch")},
		{"enter", i18n.T("help.enter")},
		{k("add"), i18n.T("help.add")},
		{k("remove"), i18n.T("help.remove")},
		{pair("move_up", "move_down"), i18n.T("help.move")},
		{k("filter"), i18n.T("help.filter")},
		{"h j k l", i18n.T("help.vim_nav")},
		{"gg/G ctrl+d/u", i18n.T("help.jump_scroll")},
		{"pgup/pgdn home/end", i18n.T("help.page_keys")},
		{pair("shuffle", "repeat"), i18n.T("help.shuffle_repeat")},
		{k("toggle_viz"), i18n.T("help.toggle_viz")},
		{k("now_playing"), i18n.T("help.now_playing")},
		{k("palette"), i18n.T("help.palette")},
		{k("songs"), i18n.T("help.songs")},
		{k("playlists"), i18n.T("help.playlists")},
		{k("get"), i18n.T("help.get")},
		{k("playlist_add"), i18n.T("help.playlist_add")},
		{k("quit"), i18n.T("help.quit")},
	}
}

// keyLabel hace visible la tecla espacio, que de otro modo se pinta como una
// celda vacía. Se aplica por TECLA y no por fila ya compuesta, así que también
// cubre el espacio como segunda mitad de un par. Solo la de espacio: el resto
// se muestran tal cual las escribe el usuario en [keys].
func keyLabel(key string) string {
	if strings.TrimSpace(key) == "" && key != "" {
		return i18n.T("help.space")
	}
	return key
}
