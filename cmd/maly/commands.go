package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"maly/internal/config"
	"maly/internal/i18n"
	"maly/internal/ipc"
	"maly/internal/version"
)

// command es la fuente única de verdad de un subcomando: main() dispatcha
// desde la tabla y helpText() genera la ayuda de la misma, para que nombres,
// uso y descripciones no vivan duplicados (las futuras completions de shell
// también leerán de aquí).
type command struct {
	name    string
	aliases []string
	usage   string // columna izquierda del help, p. ej. "vol <0-100|+N|-N>"
	descKey string // clave i18n de la descripción
	section string // sección del help (ver helpSections); "" = no listado
	run     func(args []string) error

	// complete recibe los args ya escritos (sin el nombre del comando) y la
	// palabra parcial bajo el cursor; devuelve líneas "valor\tdescripción"
	// (descripción opcional). nil = sin candidatos dinámicos. Ver complete.go.
	complete func(args []string, cur string) []string
}

// client devuelve un run que delega en runClient con el subcomando fijado.
func client(name string) func([]string) error {
	return func(args []string) error { return runClient(name, args) }
}

// El orden del slice define el orden dentro de cada sección del help.
var commands = []command{
	// daemon no se lista por sección: aparece en la cabecera de uso.
	{name: "daemon", run: func([]string) error { return runDaemon() }},

	{name: "play", usage: "play [<query>]", descKey: "cli.play", section: "playback", run: client("play"), complete: completeTracks},
	{name: "select", usage: "select", descKey: "cli.select", section: "playback", run: func([]string) error { return runSelect() }},
	{name: "pause", usage: "pause", descKey: "cli.pause", section: "playback", run: client("pause")},
	{name: "toggle", usage: "toggle", descKey: "cli.toggle", section: "playback", run: client("toggle")},
	{name: "stop", usage: "stop", descKey: "cli.stop", section: "playback", run: client("stop")},
	{name: "next", usage: "next", descKey: "cli.next", section: "playback", run: client("next")},
	{name: "prev", usage: "prev", descKey: "cli.prev", section: "playback", run: client("prev")},
	{name: "jump", usage: "jump <pos>", descKey: "cli.jump", section: "playback", run: client("jump"), complete: completeJump},
	{name: "add", usage: "add <query|path>", descKey: "cli.add", section: "playback", run: client("add"), complete: completeTracks},
	{name: "queue", usage: "queue", descKey: "cli.queue", section: "playback", run: client("queue")},
	{name: "clear", usage: "clear", descKey: "cli.clear", section: "playback", run: client("clear")},
	{name: "status", usage: "status", descKey: "cli.status", section: "playback", run: client("status")},
	{name: "vol", usage: "vol <0-100|+N|-N>", descKey: "cli.vol", section: "playback", run: client("vol")},
	{name: "seek", usage: "seek <+N|-N|mm:ss>", descKey: "cli.seek", section: "playback", run: client("seek")},
	{name: "shuffle", usage: "shuffle [on|off]", descKey: "cli.shuffle", section: "playback", run: client("shuffle"), complete: completeStatic("on", "off")},
	{name: "repeat", usage: "repeat [off|all|one]", descKey: "cli.repeat", section: "playback", run: client("repeat"), complete: completeStatic("off", "all", "one")},

	{name: "scan", usage: "scan [<path>]", descKey: "cli.scan", section: "library", run: runScan},
	{name: "search", usage: "search <query>", descKey: "cli.search", section: "library", run: runSearch},
	{name: "playlist", usage: "playlist <sub> [args]", descKey: "cli.playlist", section: "library", run: runPlaylist, complete: completePlaylist},

	{name: "controls", usage: "controls [<preset>]", descKey: "cli.controls", section: "other", run: runControls, complete: completeControls},
	{name: "lang", aliases: []string{"-l", "--lang"}, usage: "lang [en|es], -l", descKey: "cli.lang_cmd", section: "other", run: runLang, complete: completeStatic("en", "es")},
	{name: "completions", usage: "completions <shell>", descKey: "cli.completions", section: "other", run: runCompletions, complete: completeStatic(supportedShells...)},
	{name: "help", aliases: []string{"-h", "--help"}, usage: "help, -h", descKey: "cli.help_cmd", section: "other"}, // run se asigna en init()
	{name: "version", aliases: []string{"-v", "--version"}, usage: "version, -v", descKey: "cli.version_cmd", section: "other", run: runVersion},

	// __complete es interno (lo invocan los scripts de shell en cada TAB):
	// sin section no sale en el help, y completeCommands salta los "__*".
	{name: "__complete"}, // run se asigna en init()
}

// init asigna los run de help y __complete aparte: en la tabla, el compilador
// vería commands → runHelp/runComplete → helpText/completeCommands → commands
// como ciclo de inicialización.
func init() {
	for i := range commands {
		switch commands[i].name {
		case "help":
			commands[i].run = runHelp
		case "__complete":
			commands[i].run = runComplete
		}
	}
}

// lookup busca un comando por nombre o alias.
func lookup(name string) (command, bool) {
	for _, c := range commands {
		if c.name == name {
			return c, true
		}
		for _, a := range c.aliases {
			if a == name {
				return c, true
			}
		}
	}
	return command{}, false
}

func runHelp([]string) error {
	fmt.Print(helpText())
	return nil
}

func runVersion([]string) error {
	fmt.Println("Malody Mallow (maly) v" + version.Version)
	// Si hay servicio corriendo, mostrar también su versión: tras actualizar
	// el binario el demonio viejo sigue vivo y conviene enterarse.
	c, err := ipc.Dial(config.SocketPath())
	if err != nil {
		return nil
	}
	defer c.Close()
	resp, err := c.Do(ipc.Request{Cmd: "ping"})
	if err != nil || !resp.OK {
		return nil
	}
	svc := resp.Version
	if svc == "" {
		svc = "< 0.5.0" // demonios anteriores no reportan versión
	}
	if svc == version.Version {
		fmt.Println(i18n.Tf("cli.version_svc", svc))
	} else {
		fmt.Println(i18n.Tf("cli.version_svc_old", svc))
	}
	return nil
}

// helpSections define las secciones de comandos del help y su orden.
var helpSections = []struct {
	id       string
	titleKey string
	noteKey  string // "" = sin nota
}{
	{"playback", "cli.sec_playback", "cli.sec_playback_note"},
	{"library", "cli.sec_library", "cli.sec_library_note"},
	{"other", "cli.sec_other", ""},
}

// helpText arma la ayuda de `maly -h` a partir de la tabla de comandos:
// secciones coloreadas, ejemplos y atajos de la TUI. lipgloss quita los
// colores si stdout no es un terminal.
func helpText() string {
	accent := lipgloss.NewStyle().Foreground(lipgloss.Color("#89b4fa")).Bold(true)
	cmdSt := lipgloss.NewStyle().Foreground(lipgloss.Color("#a6e3a1"))
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086"))
	bold := lipgloss.NewStyle().Bold(true)

	var b strings.Builder
	sec := func(name, note string) {
		b.WriteString("\n" + accent.Render(name))
		if note != "" {
			b.WriteString(" " + dim.Render("("+note+")"))
		}
		b.WriteByte('\n')
	}
	row := func(usage, descKey string) {
		b.WriteString(fmt.Sprintf("  %s %s\n", cmdSt.Render(fmt.Sprintf("%-28s", usage)), i18n.T(descKey)))
	}
	example := func(line string) {
		b.WriteString("  " + dim.Render("$ ") + line + "\n")
	}
	key := func(k, descKey string) {
		b.WriteString(fmt.Sprintf("  %s %s\n", cmdSt.Render(fmt.Sprintf("%-14s", k)), i18n.T(descKey)))
	}

	b.WriteString(bold.Render("Malody Mallow") + " " + dim.Render("(maly) v"+version.Version) + " — " + i18n.T("cli.tagline") + "\n")

	sec(i18n.T("cli.sec_usage"), "")
	row("maly", "cli.usage_tui")
	row("maly daemon", "cli.usage_daemon")

	for _, s := range helpSections {
		note := ""
		if s.noteKey != "" {
			note = i18n.T(s.noteKey)
		}
		sec(i18n.T(s.titleKey), note)
		for _, c := range commands {
			if c.section == s.id {
				row(c.usage, c.descKey)
			}
		}
	}

	sec(i18n.T("cli.sec_examples"), "")
	example("maly play luna")
	example("maly jump 3")
	example("maly add ~/Music/album")
	example("maly vol +10")
	example("maly seek 1:23")
	example("maly shuffle on")
	example("maly playlist add favs luna")
	example("maly playlist export favs")

	sec(i18n.T("cli.sec_keys"), i18n.T("cli.sec_keys_note"))
	key(i18n.T("help.space"), "help.play_pause")
	key("n / p", "help.next_prev")
	key("tab", "help.switch")
	key("/", "help.filter")
	key("ctrl+p", "help.palette")
	key("ctrl+o", "help.songs")
	key("ctrl+l", "help.playlists")
	key("v", "help.toggle_viz")
	key("?", "help.show")
	key("q", "help.quit")

	return b.String()
}
