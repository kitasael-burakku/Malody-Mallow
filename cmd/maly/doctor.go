package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"text/tabwriter"

	"github.com/charmbracelet/lipgloss"

	"maly/internal/config"
	"maly/internal/getter"
	"maly/internal/i18n"
	"maly/internal/mpris"
	"maly/internal/probe"
	"maly/internal/update"
	"maly/internal/version"
	"maly/internal/viz"
)

// Severidad de un chequeo. Solo lvlFail cuenta como problema y cambia el
// código de salida: lo que maly degrada en silencio (sin ffprobe, sin
// visualizador real, sin MPRIS) es información, no una falla — la misma línea
// que ya sigue internal/probe ("la ausencia NO es un error").
type level int

const (
	lvlOK level = iota
	lvlInfo
	lvlWarn
	lvlFail
)

// check es una línea del informe: severidad, etiqueta corta y detalle. Las
// líneas extra (cont) salen indentadas bajo la fila, que es donde caben los
// remedios largos sin romper las columnas.
type check struct {
	lvl    level
	label  string
	detail string
	cont   []string
}

// runDoctor revisa que maly pueda hacer su trabajo y explica qué falta. Es lo
// contrario de `maly info`: no lista hechos, emite veredictos.
//
// Reglas que se respetan aquí y conviene no romper:
//   - Funciona SIN demonio y sin red. Un diagnóstico que exige lo que
//     diagnostica no sirve justo cuando hace falta.
//   - No toca el flock del demonio: un intento no bloqueante que TUVIERA
//     éxito lo retendría un instante, y un `maly daemon` arrancando en esa
//     ventana moriría con ErrAlreadyRunning. Se pregunta por el socket.
//   - No crea la base de datos ni lanza procesos: solo mira el PATH.
//   - No sabe de gestores de paquetes. Los remedios salen de las claves que
//     ya existen (getter.Tools, p.no_mpv) o remiten al instalador, que es
//     quien sabe instalar.
func runDoctor([]string) error {
	cfg, cfgErr := config.Load()
	checks := []check{
		checkMpv(),
		checkService(),
		checkMusicDir(cfg),
		checkLibrary(),
		checkKeys(cfg),
	}
	checks = append(checks, checkOptionalTools(cfg)...)
	checks = append(checks, checkUpdate())
	if cfgErr != nil {
		checks = append(checks, check{lvlWarn, i18n.T("doc.lbl_config"), i18n.Tf("info.config_err", cfgErr), nil})
	}

	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086"))
	marks := map[level]string{
		lvlOK:   lipgloss.NewStyle().Foreground(lipgloss.Color("#a6e3a1")).Render("ok"),
		lvlInfo: dim.Render("info"),
		lvlWarn: lipgloss.NewStyle().Foreground(lipgloss.Color("#f9e2af")).Render("warn"),
		lvlFail: lipgloss.NewStyle().Foreground(lipgloss.Color("#f38ba8")).Bold(true).Render("fail"),
	}

	// La etiqueta (columna del medio) va sin color: tabwriter mide las celdas
	// en runas y los escapes ANSI le falsearían el ancho. El marcador es de
	// ancho fijo y el detalle es la última celda, así que ahí sí puede llevar.
	w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
	warns, fails := 0, 0
	for _, c := range checks {
		switch c.lvl {
		case lvlWarn:
			warns++
		case lvlFail:
			fails++
		}
		fmt.Fprintf(w, "  %s\t%s\t%s\n", marks[c.lvl], c.label, c.detail)
		for _, extra := range c.cont {
			fmt.Fprintf(w, "  \t\t%s\n", dim.Render(extra))
		}
	}
	fmt.Fprintln(w, "\n"+i18n.Tf("doc.summary", warns, fails))
	if err := w.Flush(); err != nil {
		return err
	}
	if fails > 0 {
		// El informe ya explicó qué pasa y el resumen ya contó cuántos: salir
		// con código 1 sin volver a imprimir nada (ver errQuiet en main.go).
		return errQuiet
	}
	return nil
}

// checkMpv es el único chequeo que puede fallar de verdad: sin mpv no hay
// reproducción posible, y es lo que decide el código de salida.
func checkMpv() check {
	path, err := exec.LookPath("mpv")
	if err != nil {
		return check{lvlFail, "mpv", i18n.T("doc.mpv_missing"), []string{i18n.T("p.no_mpv")}}
	}
	return check{lvlOK, "mpv", path, nil}
}

// checkService pregunta por el socket. Que no haya demonio no es un problema:
// los comandos de biblioteca funcionan sin él y la TUI lo embebe al abrirse.
func checkService() check {
	label := i18n.T("doc.lbl_service")
	svc, ok := serviceVersion()
	if !ok {
		return check{lvlInfo, label, i18n.T("doc.svc_none"), nil}
	}
	if svc != version.Version {
		return check{lvlWarn, label, i18n.Tf("doc.svc_mismatch", svc, version.Version),
			[]string{i18n.T("doc.svc_restart")}}
	}
	return check{lvlOK, label, i18n.Tf("doc.svc_ok", svc), nil}
}

// checkMusicDir avisa si la ruta de música no existe, diciendo de dónde salió
// — sin el origen, un usuario que nunca tocó music_dir no sabe por qué maly
// mira ahí (misma razón que el error de `maly scan`).
func checkMusicDir(cfg config.Config) check {
	label := i18n.T("doc.lbl_music_dir")
	dir, originKey := cfg.MusicDirOrigin()
	fi, err := os.Stat(dir)
	if err != nil {
		return check{lvlWarn, label, i18n.Tf("doc.music_missing", dir, i18n.T(originKey)), nil}
	}
	if !fi.IsDir() {
		return check{lvlWarn, label, i18n.Tf("doc.music_notdir", dir), nil}
	}
	return check{lvlOK, label, dir, nil}
}

// checkLibrary mira la base SIN crearla (openLibraryIfExists): un diagnóstico
// que fabrica la base vacía y luego reporta 0 pistas se estaría diagnosticando
// a sí mismo.
func checkLibrary() check {
	label := i18n.T("doc.lbl_library")
	tracks, playlists, ok := libraryStats()
	if !ok {
		return check{lvlWarn, label, i18n.T("doc.lib_none"), nil}
	}
	if tracks == 0 {
		return check{lvlWarn, label, i18n.T("doc.lib_empty"), nil}
	}
	return check{lvlOK, label, i18n.Tf("doc.lib_ok", tracks, playlists), nil}
}

// checkKeys avisa si dos acciones de [keys] terminaron en la misma tecla:
// is() en la TUI (internal/tui/tui.go) resuelve por igualdad de string, así
// que la segunda acción queda inalcanzable sin ningún error de carga que lo
// delate — warn, no fail: el config sigue siendo válido, solo confuso.
func checkKeys(cfg config.Config) check {
	label := i18n.T("doc.lbl_keys")
	conflicts := config.KeyConflicts(cfg.Keys)
	if len(conflicts) == 0 {
		return check{lvlOK, label, i18n.T("doc.keys_ok"), nil}
	}
	cont := make([]string, len(conflicts))
	for i, c := range conflicts {
		cont[i] = i18n.Tf("doc.keys_conflict_line", c.Key, strings.Join(c.Actions, ", "))
	}
	return check{lvlWarn, label, i18n.Tf("doc.keys_conflict", len(conflicts)), cont}
}

// checkOptionalTools cubre lo que maly degrada en silencio. Todo es lvlInfo a
// propósito: son features opcionales, no averías, y quien no usa `maly get`
// no tiene por qué ver un aviso amarillo cada vez.
func checkOptionalTools(cfg config.Config) []check {
	var out []check

	// ffprobe solo importa si la fase de duraciones está encendida.
	switch {
	case !cfg.ScanDurations:
		out = append(out, check{lvlInfo, "ffprobe", i18n.T("doc.ffprobe_off"), nil})
	case probe.Available():
		out = append(out, check{lvlOK, "ffprobe", i18n.T("doc.ffprobe_ok"), nil})
	default:
		out = append(out, check{lvlInfo, "ffprobe", i18n.T("doc.ffprobe_missing"), nil})
	}

	// getter.Tools ya devuelve el "falta X" con sus instrucciones de
	// instalación: se reusa tal cual en vez de reescribirlo aquí.
	if err := getter.Tools(); err != nil {
		lines := strings.Split(err.Error(), "\n")
		out = append(out, check{lvlInfo, "yt-dlp + ffmpeg", lines[0], lines[1:]})
	} else {
		out = append(out, check{lvlOK, "yt-dlp + ffmpeg", i18n.T("doc.get_ok"), nil})
	}

	if bin := viz.CaptureBackend(cfg.Visualizer.Backend); bin != "" {
		out = append(out, check{lvlOK, i18n.T("doc.lbl_visualizer"), bin, nil})
	} else {
		out = append(out, check{lvlInfo, i18n.T("doc.lbl_visualizer"), i18n.T("doc.viz_missing"), nil})
	}

	if mpris.BusAvailable() {
		out = append(out, check{lvlOK, "mpris", i18n.T("doc.mpris_ok"), nil})
	} else {
		out = append(out, check{lvlInfo, "mpris", i18n.T("doc.mpris_missing"), nil})
	}
	return out
}

// checkUpdate lee SOLO el cache que dejó el último chequeo. Nada de red: un
// doctor que se va diez segundos a git ls-remote deja de ser útil justo
// cuando algo va mal (y `maly update` ya existe para preguntar de verdad).
func checkUpdate() check {
	label := i18n.T("doc.lbl_update")
	latest, _ := update.Cached()
	if latest != "" && update.Newer(latest, version.Version) {
		return check{lvlInfo, label, i18n.Tf("doc.upd_avail", latest, version.Version), nil}
	}
	return check{lvlOK, label, i18n.Tf("doc.upd_none", version.Version), nil}
}
