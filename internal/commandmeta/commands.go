// Package commandmeta comparte solo nombres, argumentos y descripciones.
// La ejecución pertenece a cada cliente; este catálogo no despacha acciones.
package commandmeta

import (
	"strings"

	"maly/internal/i18n"
)

// Group es la clave i18n de la categoría con la que la paleta agrupa su
// catálogo. Las entradas de un grupo van SEGUIDAS: el orden de la tabla es
// el orden en pantalla, y un test exige que ningún grupo reaparezca.
type Entry struct {
	Group   string
	Name    string
	ArgsKey string
	DescKey string
	CLIOnly bool
}

// Usage se resuelve al mostrarlo: lang puede cambiar dentro de la TUI.
func (e Entry) Usage() string {
	if e.ArgsKey == "" {
		return e.Name
	}
	return e.Name + " " + i18n.T(e.ArgsKey)
}

var entries = []Entry{
	{Group: "cmd.group.playback", Name: "play", ArgsKey: "cmd.args.play", DescKey: "cli.play"},
	{Group: "cmd.group.playback", Name: "select", DescKey: "cli.select"},
	{Group: "cmd.group.playback", Name: "pause", DescKey: "cli.pause"},
	{Group: "cmd.group.playback", Name: "toggle", DescKey: "cli.toggle"},
	{Group: "cmd.group.playback", Name: "stop", DescKey: "cli.stop"},
	{Group: "cmd.group.playback", Name: "next", DescKey: "cli.next"},
	{Group: "cmd.group.playback", Name: "prev", DescKey: "cli.prev"},
	{Group: "cmd.group.playback", Name: "seek", ArgsKey: "cmd.args.seek", DescKey: "cli.seek"},
	{Group: "cmd.group.playback", Name: "vol", ArgsKey: "cmd.args.vol", DescKey: "cli.vol"},
	{Group: "cmd.group.playback", Name: "shuffle", ArgsKey: "cmd.args.shuffle", DescKey: "cli.shuffle"},
	{Group: "cmd.group.playback", Name: "repeat", ArgsKey: "cmd.args.repeat", DescKey: "cli.repeat"},
	{Group: "cmd.group.playback", Name: "status", DescKey: "cli.status"},
	{Group: "cmd.group.queue", Name: "add", ArgsKey: "cmd.args.add", DescKey: "cli.add"},
	{Group: "cmd.group.queue", Name: "playnext", ArgsKey: "cmd.args.add", DescKey: "cli.playnext"},
	{Group: "cmd.group.queue", Name: "queue", DescKey: "cli.queue"},
	{Group: "cmd.group.queue", Name: "jump", ArgsKey: "cmd.args.jump", DescKey: "cli.jump"},
	{Group: "cmd.group.queue", Name: "move", ArgsKey: "cmd.args.move", DescKey: "cli.move"},
	{Group: "cmd.group.queue", Name: "remove", ArgsKey: "cmd.args.remove", DescKey: "cli.remove"},
	{Group: "cmd.group.queue", Name: "undo", DescKey: "cli.undo"},
	{Group: "cmd.group.queue", Name: "clear", DescKey: "cli.clear"},
	{Group: "cmd.group.library", Name: "scan", ArgsKey: "cmd.args.scan", DescKey: "cli.scan"},
	{Group: "cmd.group.library", Name: "rescan", ArgsKey: "cmd.args.rescan", DescKey: "cli.scan"},
	{Group: "cmd.group.library", Name: "search", ArgsKey: "cmd.args.search", DescKey: "cli.search"},
	{Group: "cmd.group.playlists", Name: "playlist", ArgsKey: "cmd.args.playlist", DescKey: "cli.playlist"},
	{Group: "cmd.group.playlists", Name: "playlist list", DescKey: "cmd.desc.playlist_list"},
	{Group: "cmd.group.playlists", Name: "playlist show", ArgsKey: "cmd.args.playlist_show", DescKey: "cmd.desc.playlist_show"},
	{Group: "cmd.group.playlists", Name: "playlist create", ArgsKey: "cmd.args.playlist_create", DescKey: "cmd.desc.playlist_create"},
	{Group: "cmd.group.playlists", Name: "playlist delete", ArgsKey: "cmd.args.playlist_delete", DescKey: "cmd.desc.playlist_delete"},
	{Group: "cmd.group.playlists", Name: "playlist add", ArgsKey: "cmd.args.playlist_add", DescKey: "cmd.desc.playlist_add"},
	{Group: "cmd.group.playlists", Name: "playlist remove", ArgsKey: "cmd.args.playlist_remove", DescKey: "cmd.desc.playlist_remove"},
	{Group: "cmd.group.playlists", Name: "playlist play", ArgsKey: "cmd.args.playlist_play", DescKey: "cmd.desc.playlist_play"},
	{Group: "cmd.group.playlists", Name: "playlist save", ArgsKey: "cmd.args.playlist_save", DescKey: "cmd.desc.playlist_save"},
	{Group: "cmd.group.playlists", Name: "playlist export", ArgsKey: "cmd.args.playlist_export", DescKey: "cmd.desc.playlist_export"},
	{Group: "cmd.group.playlists", Name: "playlist import", ArgsKey: "cmd.args.playlist_import", DescKey: "cmd.desc.playlist_import"},
	{Group: "cmd.group.downloads", Name: "get", ArgsKey: "cmd.args.get", DescKey: "cli.get"},
	{Group: "cmd.group.downloads", Name: "get pick", ArgsKey: "cmd.args.get_pick", DescKey: "cmd.desc.get_pick"},
	{Group: "cmd.group.downloads", Name: "get playlist", ArgsKey: "cmd.args.get_playlist", DescKey: "cmd.desc.get_playlist"},
	{Group: "cmd.group.interface", Name: "viz", ArgsKey: "cmd.args.viz", DescKey: "con.cmd_viz"},
	{Group: "cmd.group.interface", Name: "viz bars", DescKey: "con.viz_bars"},
	{Group: "cmd.group.interface", Name: "viz continuous", DescKey: "con.viz_continuous"},
	{Group: "cmd.group.interface", Name: "logo", ArgsKey: "cmd.args.logo", DescKey: "cli.logo"},
	{Group: "cmd.group.interface", Name: "lang", ArgsKey: "cmd.args.lang", DescKey: "cli.lang_cmd"},
	{Group: "cmd.group.interface", Name: "controls", ArgsKey: "cmd.args.controls", DescKey: "cli.controls"},
	{Group: "cmd.group.interface", Name: "cls", DescKey: "con.cmd_cls"},
	{Group: "cmd.group.system", Name: "info", DescKey: "cli.info"},
	{Group: "cmd.group.system", Name: "doctor", DescKey: "cli.doctor"},
	{Group: "cmd.group.system", Name: "config", DescKey: "cli.config"},
	{Group: "cmd.group.system", Name: "update", DescKey: "cli.update"},
	{Group: "cmd.group.system", Name: "version", DescKey: "cli.version_cmd"},
	{Group: "cmd.group.system", Name: "help", DescKey: "cli.help_cmd"},
	{Group: "cmd.group.system", Name: "kill", DescKey: "cli.kill"},
	{Group: "cmd.group.system", Name: "quit", DescKey: "con.cmd_quit"},
	{Group: "cmd.group.system", Name: "exit", DescKey: "con.cmd_quit"},
	{Group: "cmd.group.system", Name: "completions", ArgsKey: "cmd.args.completions", DescKey: "cli.completions", CLIOnly: true},
	{Group: "cmd.group.system", Name: "daemon", DescKey: "cli.usage_daemon", CLIOnly: true},
}

func Lookup(name string) (Entry, bool) {
	for _, e := range entries {
		if e.Name == name {
			return e, true
		}
	}
	return Entry{}, false
}

// Console devuelve una copia: nadie puede modificar el catálogo compartido.
func Console() []Entry {
	var out []Entry
	for _, e := range entries {
		if !e.CLIOnly {
			out = append(out, e)
		}
	}
	return out
}

func Subcommands(parent string) []string {
	var out []string
	for _, e := range entries {
		if sub, ok := strings.CutPrefix(e.Name, parent+" "); ok {
			out = append(out, sub)
		}
	}
	return out
}
