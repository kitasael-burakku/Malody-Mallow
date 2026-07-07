package main

import (
	_ "embed"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"maly/internal/config"
	"maly/internal/i18n"
	"maly/internal/ipc"
	"maly/internal/library"
)

// maxCandidates limita los candidatos de completado: con la palabra parcial
// vacía Search devuelve la biblioteca entera, y el pager del shell no es
// lugar para miles de líneas.
const maxCandidates = 30

//go:embed completions/maly.fish
var fishScript string

// supportedShells lista los shells con script de instalación; bash y zsh
// llegan cuando el wrapper de fish esté validado.
var supportedShells = []string{"fish"}

// runCompletions imprime el script de completado del shell pedido.
func runCompletions(args []string) error {
	if len(args) == 1 && args[0] == "fish" {
		fmt.Print(fishScript)
		return nil
	}
	return fmt.Errorf("%s", i18n.Tf("cli.usage_completions", strings.Join(supportedShells, ", ")))
}

// runComplete implementa el subcomando oculto __complete: recibe los tokens
// tras "maly" (el último es la palabra parcial, quizá vacía) e imprime un
// candidato por línea. Nunca devuelve error: cualquier fallo (sin demonio,
// sin biblioteca) degrada a cero candidatos, no a texto en medio de un TAB.
func runComplete(args []string) error {
	for _, line := range completeArgs(args) {
		fmt.Println(line)
	}
	return nil
}

// completeArgs decide los candidatos según el contexto ya tokenizado.
func completeArgs(args []string) []string {
	if len(args) <= 1 {
		cur := ""
		if len(args) == 1 {
			cur = args[0]
		}
		return completeCommands(cur)
	}
	c, ok := lookup(args[0])
	if !ok || c.complete == nil {
		return nil
	}
	return c.complete(args[1:len(args)-1], args[len(args)-1])
}

// cand arma una línea candidata "valor<TAB>descripción" — el formato nativo
// del pager de fish; los wrappers de bash/zsh recortan o adaptan el tab.
func cand(value, desc string) string {
	if desc == "" {
		return value
	}
	return value + "\t" + desc
}

// completeCommands ofrece los nombres de la tabla con su descripción i18n.
// Ni aliases (completar "-l" es ruido) ni los "__*" internos.
func completeCommands(cur string) []string {
	var out []string
	for _, c := range commands {
		if strings.HasPrefix(c.name, "__") || !strings.HasPrefix(c.name, cur) {
			continue
		}
		desc := ""
		if c.descKey != "" {
			desc = i18n.T(c.descKey)
		}
		out = append(out, cand(c.name, desc))
	}
	return out
}

// completeStatic devuelve un completer de valores fijos para comandos que
// toman un único argumento (repeat, shuffle, lang, completions).
func completeStatic(values ...string) func([]string, string) []string {
	return func(args []string, cur string) []string {
		if len(args) > 0 {
			return nil
		}
		var out []string
		for _, v := range values {
			if strings.HasPrefix(v, cur) {
				out = append(out, v)
			}
		}
		return out
	}
}

// completeControls: presets con su descripción i18n.
func completeControls(args []string, cur string) []string {
	if len(args) > 0 {
		return nil
	}
	var out []string
	for _, name := range config.PresetNames() {
		if strings.HasPrefix(name, cur) {
			out = append(out, cand(name, i18n.T("cli.preset_"+name)))
		}
	}
	return out
}

// completeTracks completa títulos desde la biblioteca (SQLite directo, sin
// demonio). La consulta son TODAS las palabras ya escritas más la parcial:
// play/add las unen igual, y Search es fold-aware — "lu" encuentra "La Luna".
func completeTracks(args []string, cur string) []string {
	lib, ok := openLibraryIfExists()
	if !ok {
		return nil
	}
	defer lib.Close()
	tracks, err := lib.Search(strings.TrimSpace(strings.Join(args, " ") + " " + cur))
	if err != nil {
		return nil
	}
	seen := make(map[string]bool)
	var out []string
	for _, t := range tracks {
		if t.Title == "" || seen[t.Title] {
			continue
		}
		seen[t.Title] = true
		desc := t.Artist
		if t.Album != "" {
			if desc != "" {
				desc += " — "
			}
			desc += t.Album
		}
		out = append(out, cand(t.Title, desc))
		if len(out) == maxCandidates {
			break
		}
	}
	return out
}

// completePlaylist anida: primero el subcomando, después nombres de playlist
// donde aplica, y para `add <nombre> <query>` vuelve a completar pistas.
func completePlaylist(args []string, cur string) []string {
	if len(args) == 0 {
		var out []string
		for _, sub := range []string{"list", "create", "delete", "add", "play"} {
			if strings.HasPrefix(sub, cur) {
				out = append(out, sub)
			}
		}
		return out
	}
	switch args[0] {
	case "play", "delete":
		if len(args) == 1 {
			return completePlaylistNames(cur)
		}
	case "add":
		if len(args) == 1 {
			return completePlaylistNames(cur)
		}
		return completeTracks(args[2:], cur)
	}
	return nil
}

func completePlaylistNames(cur string) []string {
	lib, ok := openLibraryIfExists()
	if !ok {
		return nil
	}
	defer lib.Close()
	lists, err := lib.Playlists()
	if err != nil {
		return nil
	}
	fold := library.Fold(cur)
	var out []string
	for _, p := range lists {
		if !strings.HasPrefix(library.Fold(p.Name), fold) {
			continue
		}
		out = append(out, cand(p.Name, fmt.Sprintf("♪ %d", p.Tracks)))
	}
	return out
}

// completeJump pide la cola por IPC; sin demonio, cero candidatos en
// silencio. Timeout corto: un TAB no espera 30 s a un demonio colgado.
func completeJump(args []string, cur string) []string {
	if len(args) > 0 {
		return nil
	}
	c, err := ipc.Dial(config.SocketPath())
	if err != nil {
		return nil
	}
	defer c.Close()
	c.Timeout = 2 * time.Second
	resp, err := c.Do(ipc.Request{Cmd: "queue"})
	if err != nil || !resp.OK {
		return nil
	}
	var out []string
	for i, t := range resp.Queue {
		pos := strconv.Itoa(i + 1) // 1-based, como muestra `maly queue`
		if !strings.HasPrefix(pos, cur) {
			continue
		}
		name := t.Title
		if t.Artist != "" {
			name = t.Artist + " — " + t.Title
		}
		out = append(out, cand(pos, name))
		if len(out) == maxCandidates {
			break
		}
	}
	return out
}

// openLibraryIfExists abre la biblioteca solo si el archivo ya existe:
// Open la crearía vacía, y un TAB no debe dejar residuos en $XDG_DATA_HOME.
func openLibraryIfExists() (*library.Library, bool) {
	if _, err := os.Stat(config.DBPath()); err != nil {
		return nil, false
	}
	lib, err := openLibrary()
	if err != nil {
		return nil, false
	}
	return lib, true
}
