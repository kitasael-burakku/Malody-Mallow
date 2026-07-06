// maly — reproductor de música local para terminal.
// Sin argumentos abre la TUI; con subcomandos actúa como cliente del demonio
// (estilo mpc/playerctl) o como herramienta de biblioteca (scan/search).
package main

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/charmbracelet/lipgloss"

	"maly/internal/config"
	"maly/internal/i18n"
	"maly/internal/library"
)

const version = "0.3.0"

func main() {
	// Fijar el idioma antes de imprimir nada: todo texto sale de i18n.
	// Si el config no carga o no hay idioma elegido, queda inglés (default).
	if cfg, err := config.Load(); err == nil && cfg.Language != "" {
		i18n.Set(cfg.Language)
	}

	args := os.Args[1:]
	cmd := ""
	if len(args) > 0 {
		cmd = args[0]
		args = args[1:]
	}

	var err error
	switch cmd {
	case "":
		err = runTUI(false)
	case "lang", "-l", "--lang":
		err = runLang(args)
	case "daemon":
		err = runDaemon()
	case "scan":
		err = runScan(args)
	case "search":
		err = runSearch(args)
	case "select":
		err = runSelect()
	case "controls":
		err = runControls(args)
	case "play", "pause", "toggle", "stop", "next", "prev", "jump", "clear",
		"add", "queue", "status", "vol", "seek", "shuffle", "repeat", "playlist":
		err = runClient(cmd, args)
	case "version", "-v", "--version":
		fmt.Println("Malody Mallow (maly) v" + version)
	case "help", "-h", "--help":
		fmt.Print(helpText())
	default:
		fmt.Fprintf(os.Stderr, "%s\n%s\n", i18n.Tf("cli.unknown", cmd), i18n.T("cli.more"))
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "maly: %v\n", err)
		os.Exit(1)
	}
}

// helpText arma la ayuda de `maly -h`: secciones coloreadas, comandos,
// ejemplos y atajos de la TUI. lipgloss quita los colores si stdout no es
// un terminal.
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

	b.WriteString(bold.Render("Malody Mallow") + " " + dim.Render("(maly) v"+version) + " — " + i18n.T("cli.tagline") + "\n")

	sec(i18n.T("cli.sec_usage"), "")
	row("maly", "cli.usage_tui")
	row("maly daemon", "cli.usage_daemon")

	sec(i18n.T("cli.sec_playback"), i18n.T("cli.sec_playback_note"))
	row("play [<query>]", "cli.play")
	row("select", "cli.select")
	row("pause", "cli.pause")
	row("toggle", "cli.toggle")
	row("stop", "cli.stop")
	row("next", "cli.next")
	row("prev", "cli.prev")
	row("jump <pos>", "cli.jump")
	row("add <query|path>", "cli.add")
	row("queue", "cli.queue")
	row("clear", "cli.clear")
	row("status", "cli.status")
	row("vol <0-100|+N|-N>", "cli.vol")
	row("seek <+N|-N|mm:ss>", "cli.seek")
	row("shuffle [on|off]", "cli.shuffle")
	row("repeat [off|all|one]", "cli.repeat")

	sec(i18n.T("cli.sec_library"), i18n.T("cli.sec_library_note"))
	row("scan [<path>]", "cli.scan")
	row("search <query>", "cli.search")
	row("playlist <sub> [args]", "cli.playlist")

	sec(i18n.T("cli.sec_other"), "")
	row("controls [<preset>]", "cli.controls")
	row("lang [en|es], -l", "cli.lang_cmd")
	row("help, -h", "cli.help_cmd")
	row("version, -v", "cli.version_cmd")

	sec(i18n.T("cli.sec_examples"), "")
	example("maly play luna")
	example("maly jump 3")
	example("maly add ~/Music/album")
	example("maly vol +10")
	example("maly seek 1:23")
	example("maly shuffle on")
	example("maly playlist add favs luna")

	sec(i18n.T("cli.sec_keys"), "")
	key(i18n.T("help.space"), "help.play_pause")
	key("n / p", "help.next_prev")
	key("tab", "help.switch")
	key("/", "help.filter")
	key("ctrl+p", "help.palette")
	key("ctrl+o", "help.songs")
	key("v", "help.toggle_viz")
	key("?", "help.show")
	key("q", "help.quit")

	return b.String()
}

// runLang cambia el idioma: sin argumento abre la TUI con el selector;
// con "en"/"es" lo fija directamente desde la CLI.
func runLang(args []string) error {
	if len(args) == 0 {
		return runTUI(true)
	}
	code := args[0]
	if code != "en" && code != "es" {
		return fmt.Errorf("%s", i18n.Tf("cli.lang_invalid", code))
	}
	if err := config.SaveLanguage(code); err != nil {
		return err
	}
	i18n.Set(code) // el mensaje de confirmación ya sale en el idioma nuevo
	fmt.Println(i18n.Tf("cli.lang_set", langName(code)))
	return nil
}

// runControls lista los presets de controles o fija uno en el config.
func runControls(args []string) error {
	if len(args) == 0 {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		active := cfg.Controls
		if !config.ValidPreset(active) {
			active = "default"
		}
		accent := lipgloss.NewStyle().Foreground(lipgloss.Color("#89b4fa")).Bold(true)
		dim := lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086"))
		fmt.Println(accent.Render(i18n.T("cli.controls_head")))
		for _, name := range config.PresetNames() {
			mark := "  "
			if name == active {
				mark = "* "
			}
			fmt.Printf("%s%-10s %s\n", mark, name, dim.Render(i18n.T("cli.preset_"+name)))
		}
		fmt.Println(dim.Render(i18n.T("cli.controls_hint")))
		return nil
	}
	name := args[0]
	if !config.ValidPreset(name) {
		return fmt.Errorf("%s", i18n.Tf("cli.controls_invalid", name, strings.Join(config.PresetNames(), ", ")))
	}
	if err := config.SaveControls(name); err != nil {
		return err
	}
	fmt.Println(i18n.Tf("cli.controls_set", name))
	return nil
}

func langName(code string) string {
	if code == "es" {
		return "Español"
	}
	return "English"
}

func openLibrary() (*library.Library, error) {
	return library.Open(config.DBPath())
}

func runScan(args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	dir := cfg.MusicPath()
	if len(args) > 0 {
		dir = config.ExpandTilde(args[0])
	}
	lib, err := openLibrary()
	if err != nil {
		return err
	}
	defer lib.Close()

	fmt.Println(i18n.Tf("cli.scan_start", dir))
	res, err := lib.Scan(dir)
	if err != nil {
		return err
	}
	for _, e := range res.Errors {
		fmt.Fprintln(os.Stderr, i18n.Tf("cli.scan_warn", e))
	}
	total, _ := lib.Count()
	fmt.Println(i18n.Tf("cli.scan_done", res.Added, res.Updated, res.Removed, total))
	if total == 0 {
		fmt.Println(i18n.Tf("cli.scan_empty", dir))
	}
	return nil
}

func runSearch(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("%s", i18n.T("cli.usage_search"))
	}
	lib, err := openLibrary()
	if err != nil {
		return err
	}
	defer lib.Close()
	tracks, err := lib.Search(strings.Join(args, " "))
	if err != nil {
		return err
	}
	if len(tracks) == 0 {
		fmt.Println(i18n.T("cli.search_none"))
		return nil
	}
	printTracks(tracks)
	return nil
}

func printTracks(tracks []library.Track) {
	w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
	fmt.Fprintln(w, i18n.T("cli.tbl_header"))
	for _, t := range tracks {
		no := ""
		if t.TrackNo > 0 {
			no = fmt.Sprintf("%d", t.TrackNo)
		}
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n", t.ID, t.Artist, t.Album, no, t.Title)
	}
	w.Flush()
}
