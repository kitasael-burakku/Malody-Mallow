// maly — reproductor de música local para terminal.
// Sin argumentos abre la TUI; con subcomandos actúa como cliente del demonio
// (estilo mpc/playerctl) o como herramienta de biblioteca (scan/search).
package main

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"maly/internal/config"
	"maly/internal/library"
)

const usage = `maly — reproductor de música para terminal

Uso:
  maly                     abre la TUI (arranca el demonio si hace falta)
  maly daemon              arranca solo el demonio (headless)

Reproducción (requieren demonio o TUI abierta):
  maly play [consulta]     reproduce (busca en la biblioteca si hay consulta)
  maly pause | toggle | stop
  maly next | prev
  maly add <consulta|ruta> agrega a la cola
  maly queue               muestra la cola
  maly status              estado actual
  maly vol <0-100|+N|-N>   volumen
  maly seek <+N|-N|mm:ss>  posición
  maly shuffle | repeat    alternar modos

Biblioteca (funcionan sin demonio):
  maly scan [ruta]         (re)escanea la biblioteca
  maly search <consulta>   busca por título/artista/álbum
  maly playlist list|create|delete|add|play ...
`

func main() {
	args := os.Args[1:]
	cmd := ""
	if len(args) > 0 {
		cmd = args[0]
		args = args[1:]
	}

	var err error
	switch cmd {
	case "":
		err = runTUI()
	case "daemon":
		err = runDaemon()
	case "scan":
		err = runScan(args)
	case "search":
		err = runSearch(args)
	case "play", "pause", "toggle", "stop", "next", "prev",
		"add", "queue", "status", "vol", "seek", "shuffle", "repeat", "playlist":
		err = runClient(cmd, args)
	case "help", "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "maly: subcomando desconocido %q\n\n%s", cmd, usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "maly: %v\n", err)
		os.Exit(1)
	}
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

	fmt.Printf("Escaneando %s ...\n", dir)
	res, err := lib.Scan(dir)
	if err != nil {
		return err
	}
	for _, e := range res.Errors {
		fmt.Fprintf(os.Stderr, "  aviso: %s\n", e)
	}
	total, _ := lib.Count()
	fmt.Printf("Listo: %d nuevas, %d actualizadas, %d eliminadas (%d pistas en total)\n",
		res.Added, res.Updated, res.Removed, total)
	if total == 0 {
		fmt.Printf("La biblioteca está vacía. ¿Hay música en %s? Puedes indicar otra ruta: maly scan <ruta>\n", dir)
	}
	return nil
}

func runSearch(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("uso: maly search <consulta>")
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
		fmt.Println("Sin resultados. ¿Ya escaneaste la biblioteca? (maly scan)")
		return nil
	}
	printTracks(tracks)
	return nil
}

func printTracks(tracks []library.Track) {
	w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tARTISTA\tÁLBUM\t#\tTÍTULO")
	for _, t := range tracks {
		no := ""
		if t.TrackNo > 0 {
			no = fmt.Sprintf("%d", t.TrackNo)
		}
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n", t.ID, t.Artist, t.Album, no, t.Title)
	}
	w.Flush()
}
