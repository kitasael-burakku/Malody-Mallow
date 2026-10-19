package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"maly/internal/commandmeta"
	"maly/internal/config"
	"maly/internal/i18n"
	"maly/internal/ipc"
	"maly/internal/tui"
	"maly/internal/version"
)

// command fija dispatch, aliases, sección y completado de cada comando CLI.
// La gramática y descripción viven en commandmeta para compartirlas con la
// consola sin que la TUI importe package main.
type command struct {
	name    string
	aliases []string
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

	{name: "play", section: "playback", run: client("play"), complete: completeTracks},
	{name: "select", section: "playback", run: func([]string) error { return runSelect() }},
	{name: "pause", section: "playback", run: client("pause")},
	{name: "toggle", section: "playback", run: client("toggle")},
	{name: "stop", section: "playback", run: client("stop")},
	{name: "next", section: "playback", run: client("next")},
	{name: "prev", section: "playback", run: client("prev")},
	{name: "jump", section: "playback", run: client("jump"), complete: completeJump},
	{name: "move", section: "playback", run: client("move"), complete: completeMove},
	{name: "undo", section: "playback", run: client("undo")},
	{name: "remove", section: "playback", run: client("remove"), complete: completeJump},
	{name: "playnext", section: "playback", run: client("playnext"), complete: completeTracks},
	{name: "add", section: "playback", run: client("add"), complete: completeTracks},
	{name: "queue", section: "playback", run: client("queue")},
	{name: "clear", section: "playback", run: client("clear")},
	{name: "status", section: "playback", run: client("status")},
	{name: "vol", section: "playback", run: client("vol")},
	{name: "seek", section: "playback", run: client("seek")},
	{name: "shuffle", section: "playback", run: client("shuffle"), complete: completeStatic("on", "off")},
	{name: "repeat", section: "playback", run: client("repeat"), complete: completeStatic("off", "all", "one")},

	{name: "scan", section: "library", run: runScan},
	{name: "search", section: "library", run: runSearch},
	{name: "get", section: "library", run: runGet, complete: completeGet},
	{name: "playlist", section: "library", run: runPlaylist, complete: completePlaylist},

	{name: "controls", section: "other", run: runControls, complete: completeControls},
	{name: "logo", section: "other", run: runLogo},
	{name: "lang", aliases: []string{"-l", "--lang"}, section: "other", run: runLang, complete: completeStatic("en", "es")},
	{name: "info", section: "other", run: runInfo},
	{name: "doctor", section: "other", run: runDoctor},
	{name: "config", section: "other", run: runConfig},
	{name: "update", section: "other", run: runUpdate},
	{name: "kill", section: "other", run: runKill},
	{name: "completions", section: "other", run: runCompletions, complete: completeStatic(supportedShells...)},
	{name: "help", aliases: []string{"-h", "--help"}, section: "other"}, // run se asigna en init()
	{name: "version", aliases: []string{"-v", "--version"}, section: "other", run: runVersion},

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

// serviceVersion pregunta al demonio qué versión corre. ok en false significa
// que nadie contesta en el socket, que NO es un error: version, info y doctor
// lo reportan como un estado más. El timeout es corto a propósito, igual que
// el de ipc.Ping: un demonio que acepta la conexión y no responde (arrancando,
// esperando hasta 5 s a mpv) no puede colgar 30 s a quien solo pregunta.
func serviceVersion() (ver string, ok bool) {
	c, err := ipc.Dial(config.SocketPath())
	if err != nil {
		return "", false
	}
	defer c.Close()
	c.Timeout = 2 * time.Second
	resp, err := c.Do(ipc.Request{Cmd: "ping"})
	if err != nil || !resp.OK {
		return "", false
	}
	if resp.Version == "" {
		return "< 0.5.0", true // demonios anteriores no reportan versión
	}
	return resp.Version, true
}

func runVersion([]string) error {
	fmt.Println("Malody Mallow (maly) v" + version.Version)
	// Si hay servicio corriendo, mostrar también su versión: tras actualizar
	// el binario el demonio viejo sigue vivo y conviene enterarse.
	svc, ok := serviceVersion()
	if !ok {
		return nil
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
	accent := lipgloss.NewStyle().Foreground(clrAccent).Bold(true)
	cmdSt := lipgloss.NewStyle().Foreground(clrCmd)
	dim := lipgloss.NewStyle().Foreground(clrDim)
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
	// Ancho de la columna de teclas a la medida de la más larga, como en el
	// modal `?`: las teclas son configurables ([keys], preset de controls) y
	// un %-14s fijo dejaba pegadas a su descripción las filas más anchas.
	keyRows := tui.HelpRows(helpKeys())
	keyW := 0
	for _, r := range keyRows {
		if w := lipgloss.Width(r[0]); w > keyW {
			keyW = w
		}
	}
	key := func(k, desc string) {
		for i, line := range strings.Split(desc, "\n") {
			label := k
			if i > 0 {
				label = ""
			}
			b.WriteString(fmt.Sprintf("  %s %s\n", cmdSt.Render(padRight(label, keyW)), line))
		}
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
				meta, _ := commandmeta.Lookup(c.name)
				usage := meta.Usage()
				if len(c.aliases) > 0 {
					usage += ", " + c.aliases[0]
				}
				row(usage, meta.DescKey)
			}
		}
	}

	sec(i18n.T("cli.sec_examples"), "")
	example("maly play luna")
	example("maly jump 3")
	example("maly move 3 1")
	example("maly remove 2")
	example("maly add ~/Music/album")
	example("maly add luna \"dark side\"")
	example("maly vol +10")
	example("maly seek 1:23")
	example("maly shuffle on")
	example("maly playlist add favs luna")
	example("maly playlist export favs")
	example("maly get \"aurora runaway\"")
	example("maly get pick \"aurora runaway\"")
	example("maly get playlist https://youtube.com/playlist?list=... favs")

	sec(i18n.T("cli.sec_keys"), i18n.T("cli.sec_keys_note"))
	for _, r := range keyRows {
		if r[0] == "" {
			b.WriteString("\n" + bold.Render(r[1]) + "\n")
			continue
		}
		key(r[0], r[1])
	}

	return b.String()
}

// helpKeys devuelve las teclas ya resueltas (defaults ← preset ← [keys]) para
// la sección de atajos. main() ya llamó a config.Load() para fijar el idioma,
// así que acá no hay ningún efecto nuevo sobre el disco; si falla, Load
// devuelve igual el config por defecto con las teclas resueltas (retorno con
// nombre + defer), que es exactamente lo que hay que mostrar.
func helpKeys() map[string]string {
	cfg, _ := config.Load()
	return cfg.Keys
}

// padRight rellena a la derecha midiendo en CELDAS (lipgloss.Width) y no en
// bytes: %-14s contaba bytes y una tecla no ASCII habría descuadrado la
// columna.
func padRight(s string, w int) string {
	if pad := w - lipgloss.Width(s); pad > 0 {
		return s + strings.Repeat(" ", pad)
	}
	return s
}
