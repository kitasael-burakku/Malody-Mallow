// Package commandmeta comparte solo nombres, argumentos y descripciones.
// La ejecución pertenece a cada cliente; este catálogo no despacha acciones.
package commandmeta

import (
	"strings"

	"maly/internal/i18n"
)

type Entry struct {
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
	{Name: "play", ArgsKey: "cmd.args.play", DescKey: "cli.play"},
	{Name: "select", DescKey: "cli.select"},
	{Name: "pause", DescKey: "cli.pause"},
	{Name: "toggle", DescKey: "cli.toggle"},
	{Name: "stop", DescKey: "cli.stop"},
	{Name: "next", DescKey: "cli.next"},
	{Name: "prev", DescKey: "cli.prev"},
	{Name: "jump", ArgsKey: "cmd.args.jump", DescKey: "cli.jump"},
	{Name: "move", ArgsKey: "cmd.args.move", DescKey: "cli.move"},
	{Name: "undo", DescKey: "cli.undo"},
	{Name: "remove", ArgsKey: "cmd.args.remove", DescKey: "cli.remove"},
	{Name: "playnext", ArgsKey: "cmd.args.add", DescKey: "cli.playnext"},
	{Name: "add", ArgsKey: "cmd.args.add", DescKey: "cli.add"},
	{Name: "queue", DescKey: "cli.queue"},
	{Name: "clear", DescKey: "cli.clear"},
	{Name: "status", DescKey: "cli.status"},
	{Name: "vol", ArgsKey: "cmd.args.vol", DescKey: "cli.vol"},
	{Name: "seek", ArgsKey: "cmd.args.seek", DescKey: "cli.seek"},
	{Name: "shuffle", ArgsKey: "cmd.args.shuffle", DescKey: "cli.shuffle"},
	{Name: "repeat", ArgsKey: "cmd.args.repeat", DescKey: "cli.repeat"},
	{Name: "scan", ArgsKey: "cmd.args.scan", DescKey: "cli.scan"},
	{Name: "search", ArgsKey: "cmd.args.search", DescKey: "cli.search"},
	{Name: "get", ArgsKey: "cmd.args.get", DescKey: "cli.get"},
	{Name: "playlist", ArgsKey: "cmd.args.playlist", DescKey: "cli.playlist"},
	{Name: "controls", ArgsKey: "cmd.args.controls", DescKey: "cli.controls"},
	{Name: "logo", ArgsKey: "cmd.args.logo", DescKey: "cli.logo"},
	{Name: "lang", ArgsKey: "cmd.args.lang", DescKey: "cli.lang_cmd"},
	{Name: "info", DescKey: "cli.info"},
	{Name: "doctor", DescKey: "cli.doctor"},
	{Name: "config", DescKey: "cli.config"},
	{Name: "update", DescKey: "cli.update"},
	{Name: "kill", DescKey: "cli.kill"},
	{Name: "completions", ArgsKey: "cmd.args.completions", DescKey: "cli.completions", CLIOnly: true},
	{Name: "help", DescKey: "cli.help_cmd"},
	{Name: "version", DescKey: "cli.version_cmd"},
	{Name: "daemon", DescKey: "cli.usage_daemon", CLIOnly: true},
	{Name: "quit", DescKey: "con.cmd_quit"},
	{Name: "exit", DescKey: "con.cmd_quit"},
	{Name: "cls", DescKey: "con.cmd_cls"},
	{Name: "viz", DescKey: "con.cmd_viz"},
	{Name: "rescan", ArgsKey: "cmd.args.rescan", DescKey: "cli.scan"},
	{Name: "playlist list", DescKey: "cmd.desc.playlist_list"},
	{Name: "playlist show", ArgsKey: "cmd.args.playlist_show", DescKey: "cmd.desc.playlist_show"},
	{Name: "playlist create", ArgsKey: "cmd.args.playlist_create", DescKey: "cmd.desc.playlist_create"},
	{Name: "playlist delete", ArgsKey: "cmd.args.playlist_delete", DescKey: "cmd.desc.playlist_delete"},
	{Name: "playlist add", ArgsKey: "cmd.args.playlist_add", DescKey: "cmd.desc.playlist_add"},
	{Name: "playlist remove", ArgsKey: "cmd.args.playlist_remove", DescKey: "cmd.desc.playlist_remove"},
	{Name: "playlist play", ArgsKey: "cmd.args.playlist_play", DescKey: "cmd.desc.playlist_play"},
	{Name: "playlist save", ArgsKey: "cmd.args.playlist_save", DescKey: "cmd.desc.playlist_save"},
	{Name: "playlist export", ArgsKey: "cmd.args.playlist_export", DescKey: "cmd.desc.playlist_export"},
	{Name: "playlist import", ArgsKey: "cmd.args.playlist_import", DescKey: "cmd.desc.playlist_import"},
	{Name: "get pick", ArgsKey: "cmd.args.get_pick", DescKey: "cmd.desc.get_pick"},
	{Name: "get playlist", ArgsKey: "cmd.args.get_playlist", DescKey: "cmd.desc.get_playlist"},
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
