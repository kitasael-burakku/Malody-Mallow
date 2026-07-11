// maly — reproductor de música local para terminal.
// Sin argumentos abre la TUI; con subcomandos actúa como cliente del demonio
// (estilo mpc/playerctl) o como herramienta de biblioteca (scan/search).
package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/charmbracelet/lipgloss"

	"maly/internal/config"
	"maly/internal/i18n"
	"maly/internal/ipc"
	"maly/internal/library"
)

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
	if cmd == "" {
		err = runTUI(false)
	} else if c, ok := lookup(cmd); ok {
		err = c.run(args)
	} else {
		fmt.Fprintf(os.Stderr, "%s\n%s\n", i18n.Tf("cli.unknown", cmd), i18n.T("cli.more"))
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "maly: %v\n", err)
		os.Exit(1)
	}
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
	query := strings.Join(args, " ")
	// Ruta relativa a absoluta: si atiende el demonio, su cwd es otro.
	if q := strings.TrimSpace(query); q != "" {
		if abs, err := filepath.Abs(config.ExpandTilde(q)); err == nil {
			query = abs
		}
	}
	dir, origin, explicit := cfg.ScanTarget(query)
	fmt.Println(i18n.Tf("cli.scan_start", dir))

	// Con el demonio vivo el escaneo va a través de él: su LibGen sube y
	// todas las TUIs abiertas recargan el árbol solas. Sin demonio, directo
	// a la DB.
	if c, err := ipc.Dial(config.SocketPath()); err == nil {
		defer c.Close()
		c.Timeout = 10 * time.Minute // una biblioteca grande no cabe en los 30 s default
		resp, err := c.Do(ipc.Request{Cmd: "scan", Query: query})
		if err != nil {
			return err
		}
		if !resp.OK {
			return errors.New(resp.Error)
		}
		fmt.Println(resp.Msg)
		return nil
	}

	lib, err := openLibrary()
	if err != nil {
		return err
	}
	defer lib.Close()
	res, err := lib.Scan(dir)
	if err != nil {
		// Ruta por defecto que no existe: decir de dónde salió y cómo apuntar
		// a la música. Con ruta explícita el usuario ya sabe qué escribió.
		if !explicit && errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("%s", i18n.Tf("cli.scan_noexist", dir, i18n.T(origin)))
		}
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
