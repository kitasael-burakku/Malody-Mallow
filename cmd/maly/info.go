package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/charmbracelet/lipgloss"

	"maly/internal/config"
	"maly/internal/i18n"
	"maly/internal/ipc"
	"maly/internal/version"
)

// libraryStats devuelve el tamaño de la biblioteca. ok en false = todavía no
// hay base de datos, que no es un error: es lo que ve quien aún no escaneó.
// Se abre por openLibraryIfExists a propósito — library.Open CREARÍA la base,
// y un comando de diagnóstico no puede inventar el estado que está reportando
// (mismo motivo por el que las completions no la abren a ciegas).
func libraryStats() (tracks, playlists int, ok bool) {
	lib, exists := openLibraryIfExists()
	if !exists {
		return 0, 0, false
	}
	defer lib.Close()
	n, err := lib.Count()
	if err != nil {
		return 0, 0, false
	}
	lists, err := lib.Playlists()
	if err != nil {
		return n, 0, true // la cuenta de pistas sigue valiendo
	}
	return n, len(lists), true
}

// runInfo imprime los hechos de la instalación: versiones, rutas, de dónde
// sale la música, cuánta hay y las claves del config que más se consultan.
// No juzga nada —eso es `maly doctor`— y nunca falla: sin servicio, sin
// biblioteca o con el config roto sigue imprimiendo lo que sí sabe, porque
// justo entonces es cuando se consulta.
//
// No muestra qué suena (eso es `maly status`) ni títulos de pistas: así
// ningún texto ajeno pasa por aquí y la salida es pegable tal cual — lipgloss
// quita el color solo cuando stdout no es un terminal.
func runInfo([]string) error {
	// El error del config no aborta: Load devuelve igualmente los defaults, y
	// un config ilegible es justo lo que este comando debe ayudar a ver.
	cfg, cfgErr := config.Load()

	accent := lipgloss.NewStyle().Foreground(lipgloss.Color("#89b4fa")).Bold(true)
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086"))

	w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
	// Las etiquetas (primera columna) van SIN color: tabwriter mide las celdas
	// en runas y los escapes ANSI le falsearían el ancho. En la última columna
	// da igual, porque a esa no le añade relleno.
	sec := func(titleKey string) {
		fmt.Fprintln(w, "\n"+accent.Render(i18n.T(titleKey)))
	}
	row := func(labelKey, value string) {
		fmt.Fprintf(w, "  %s\t%s\n", i18n.T(labelKey), value)
	}
	note := func(text string) {
		fmt.Fprintln(w, "  "+dim.Render(text))
	}

	fmt.Fprintln(w, "Malody Mallow (maly) v"+version.Version)

	sec("info.sec_versions")
	row("info.binary", version.Version)
	if svc, ok := serviceVersion(); ok {
		line := svc
		if svc != version.Version {
			line += " " + dim.Render(i18n.T("info.svc_mismatch"))
		}
		row("info.service", line)
	} else {
		row("info.service", dim.Render(i18n.T("info.svc_none")))
	}

	sec("info.sec_paths")
	row("info.config", config.ConfigPath())
	// El arte del banner es opcional y casi nadie lo tiene: listarlo siempre
	// sugeriría que falta algo.
	if _, err := os.Stat(config.LogoArtPath()); err == nil {
		row("info.logo", config.LogoArtPath())
	}
	// El directorio de datos cubre session.json y update.json sin repetir aquí
	// nombres de archivo que son de daemon y update: si allí cambian, esta
	// línea no puede mentir.
	row("info.data", config.DataDir())
	row("info.database", config.DBPath())
	row("info.runtime", config.RuntimeDir())
	row("info.socket", config.SocketPath())

	sec("info.sec_music")
	musicDir, originKey := cfg.MusicDirOrigin()
	line := musicDir
	if _, err := os.Stat(musicDir); err != nil {
		line += " " + dim.Render(i18n.T("info.music_missing"))
	}
	row("info.music_path", line)
	row("info.music_src", i18n.T(originKey))

	sec("info.sec_library")
	if tracks, playlists, ok := libraryStats(); ok {
		row("info.tracks", fmt.Sprintf("%d", tracks))
		row("info.playlists", fmt.Sprintf("%d", playlists))
	} else {
		note(i18n.T("info.db_none"))
	}

	sec("info.sec_config")
	lang := cfg.Language
	if lang == "" {
		lang = dim.Render(i18n.T("info.unset"))
	}
	row("info.language", lang)
	controls := cfg.Controls
	if !config.ValidPreset(controls) {
		controls = "default" // mismo criterio que `maly controls`
	}
	row("info.controls", controls)
	row("info.update_check", ipc.OnOff(cfg.UpdateCheck))
	row("info.scan_durations", ipc.OnOff(cfg.ScanDurations))
	cookies := cfg.Ytdlp.CookiesFromBrowser
	if cookies == "" {
		cookies = dim.Render(i18n.T("info.unset"))
	}
	row("info.ytdlp_cookies", cookies)
	if cfgErr != nil {
		note(i18n.Tf("info.config_err", cfgErr))
	}

	return w.Flush()
}
