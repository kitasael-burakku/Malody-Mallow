package tui

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"maly/internal/config"
	"maly/internal/getter"
	"maly/internal/i18n"
	"maly/internal/ipc"
	"maly/internal/library"
	"maly/internal/mpris"
	"maly/internal/probe"
	"maly/internal/update"
	"maly/internal/version"
	"maly/internal/viz"
)

// conInfo, conDoctor y conConfig espejan `maly info`/`maly doctor`/
// `maly config` (cmd/maly/info.go, doctor.go, config_cmd.go) dentro de la
// paleta de comandos — funcionan sin demonio ni red, justo lo que hace
// falta cuando la TUI misma está fallando (auditoría 2026-07-31, hallazgo
// T16). internal/tui no puede importar cmd/maly (paquete main), así que el
// ENSAMBLADO se reimplementa acá, mismo patrón ya establecido por
// conGet/conGetPlaylist con get.go; los PRIMITIVOS (config.Load, probe,
// getter.Tools, viz.CaptureBackend, mpris.BusAvailable, update.Cached) se
// reusan tal cual. No hay tabwriter en un panel angosto — cada fila es
// "label: value" y clip() (ya aplicado por consoleView a cada línea) se
// encarga del ancho.

// openLibIfExists abre la biblioteca solo si el archivo ya existe — mismo
// motivo que openLibraryIfExists en cmd/maly/complete.go: library.Open
// CREARÍA la base, y un diagnóstico no puede fabricar el estado que reporta.
func openLibIfExists() (*library.Library, bool) {
	if _, err := os.Stat(config.DBPath()); err != nil {
		return nil, false
	}
	lib, err := library.Open(config.DBPath())
	if err != nil {
		return nil, false
	}
	return lib, true
}

// libStatsTUI espeja libraryStats de cmd/maly/info.go.
func libStatsTUI() (tracks, playlists int, ok bool) {
	lib, exists := openLibIfExists()
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
		return n, 0, true
	}
	return n, len(lists), true
}

// diagServiceVersion pregunta al demonio su versión con el mismo timeout
// corto que serviceVersion en cmd/maly/commands.go: un demonio arrancando
// (esperando hasta 5s a mpv) no puede colgar la consulta.
func diagServiceVersion(sock string) (ver string, ok bool) {
	c, err := ipc.Dial(sock)
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

func orUnsetTUI(s, unset string) string {
	if s == "" {
		return unset
	}
	return s
}

// conInfo espeja `maly info`: hechos de la instalación, sin veredictos.
func (m *Model) conInfo() tea.Cmd {
	st, sock := m.st, m.sock
	return func() tea.Msg {
		cfg, cfgErr := config.Load()
		var lines []string
		sec := func(titleKey string) { lines = append(lines, st.accent.Render(i18n.T(titleKey))) }
		row := func(labelKey, value string) {
			lines = append(lines, "  "+st.dim.Render(i18n.T(labelKey))+": "+value)
		}
		note := func(text string) { lines = append(lines, "  "+st.dim.Render(text)) }

		lines = append(lines, st.text.Render("Malody Mallow (maly) v"+version.Version))

		sec("info.sec_versions")
		row("info.binary", version.Version)
		channel := i18n.T("info.channel_manual")
		if version.Packaged() {
			channel = i18n.T("info.channel_pkg")
		}
		row("info.channel", channel)
		if svc, ok := diagServiceVersion(sock); ok {
			line := svc
			if svc != version.Version {
				line += " " + st.dim.Render(i18n.T("info.svc_mismatch"))
			}
			row("info.service", line)
		} else {
			row("info.service", st.dim.Render(i18n.T("info.svc_none")))
		}

		sec("info.sec_paths")
		row("info.config", config.ConfigPath())
		row("info.database", config.DBPath())
		row("info.socket", config.SocketPath())

		sec("info.sec_music")
		musicDir, originKey := cfg.MusicDirOrigin()
		mline := musicDir
		if _, err := os.Stat(musicDir); err != nil {
			mline += " " + st.dim.Render(i18n.T("info.music_missing"))
		}
		row("info.music_path", mline)
		row("info.music_src", i18n.T(originKey))

		sec("info.sec_library")
		if tracks, playlists, ok := libStatsTUI(); ok {
			row("info.tracks", fmt.Sprintf("%d", tracks))
			row("info.playlists", fmt.Sprintf("%d", playlists))
		} else {
			note(i18n.T("info.db_none"))
		}

		sec("info.sec_config")
		lang := cfg.Language
		if lang == "" {
			lang = st.dim.Render(i18n.T("info.unset"))
		}
		row("info.language", lang)
		row("info.update_check", ipc.OnOff(cfg.UpdateCheck))
		row("info.scan_durations", ipc.OnOff(cfg.ScanDurations))
		if cfgErr != nil {
			note(i18n.Tf("info.config_err", cfgErr))
		}
		return conMsg{lines: lines}
	}
}

// diagLevel es la severidad de un chequeo de conDoctor — mismo modelo que
// level en cmd/maly/doctor.go (solo lvlFail cambiaría un código de salida
// en la CLI; acá, sin ese contrato, es puramente informativo).
type diagLevel int

const (
	diagOK diagLevel = iota
	diagInfo
	diagWarn
	diagFail
)

type diagCheck struct {
	lvl    diagLevel
	label  string
	detail string
	cont   []string
}

// conDoctor espeja `maly doctor`: los mismos veredictos, sin código de
// salida que cambiar (acá no aplica ese contrato).
func (m *Model) conDoctor() tea.Cmd {
	st, sock := m.st, m.sock
	return func() tea.Msg {
		cfg, cfgErr := config.Load()
		var checks []diagCheck

		if path, err := exec.LookPath("mpv"); err != nil {
			checks = append(checks, diagCheck{diagFail, "mpv", i18n.T("doc.mpv_missing"), []string{i18n.T("p.no_mpv")}})
		} else {
			checks = append(checks, diagCheck{diagOK, "mpv", path, nil})
		}

		if svc, ok := diagServiceVersion(sock); !ok {
			checks = append(checks, diagCheck{diagInfo, i18n.T("doc.lbl_service"), i18n.T("doc.svc_none"), nil})
		} else if svc != version.Version {
			checks = append(checks, diagCheck{diagWarn, i18n.T("doc.lbl_service"), i18n.Tf("doc.svc_mismatch", svc, version.Version), []string{i18n.T("doc.svc_restart")}})
		} else {
			checks = append(checks, diagCheck{diagOK, i18n.T("doc.lbl_service"), i18n.Tf("doc.svc_ok", svc), nil})
		}

		dir, originKey := cfg.MusicDirOrigin()
		if fi, err := os.Stat(dir); err != nil {
			checks = append(checks, diagCheck{diagWarn, i18n.T("doc.lbl_music_dir"), i18n.Tf("doc.music_missing", dir, i18n.T(originKey)), nil})
		} else if !fi.IsDir() {
			checks = append(checks, diagCheck{diagWarn, i18n.T("doc.lbl_music_dir"), i18n.Tf("doc.music_notdir", dir), nil})
		} else {
			checks = append(checks, diagCheck{diagOK, i18n.T("doc.lbl_music_dir"), dir, nil})
		}

		if tracks, playlists, ok := libStatsTUI(); !ok {
			checks = append(checks, diagCheck{diagWarn, i18n.T("doc.lbl_library"), i18n.T("doc.lib_none"), nil})
		} else if tracks == 0 {
			checks = append(checks, diagCheck{diagWarn, i18n.T("doc.lbl_library"), i18n.T("doc.lib_empty"), nil})
		} else {
			checks = append(checks, diagCheck{diagOK, i18n.T("doc.lbl_library"), i18n.Tf("doc.lib_ok", tracks, playlists), nil})
		}

		switch {
		case !cfg.ScanDurations:
			checks = append(checks, diagCheck{diagInfo, "ffprobe", i18n.T("doc.ffprobe_off"), nil})
		case probe.Available():
			checks = append(checks, diagCheck{diagOK, "ffprobe", i18n.T("doc.ffprobe_ok"), nil})
		default:
			checks = append(checks, diagCheck{diagInfo, "ffprobe", i18n.T("doc.ffprobe_missing"), nil})
		}

		if err := getter.Tools(); err != nil {
			lns := strings.Split(err.Error(), "\n")
			checks = append(checks, diagCheck{diagInfo, "yt-dlp + ffmpeg", lns[0], lns[1:]})
		} else {
			checks = append(checks, diagCheck{diagOK, "yt-dlp + ffmpeg", i18n.T("doc.get_ok"), nil})
		}

		if bin := viz.CaptureBackend(cfg.Visualizer.Backend); bin != "" {
			checks = append(checks, diagCheck{diagOK, i18n.T("doc.lbl_visualizer"), bin, nil})
		} else {
			checks = append(checks, diagCheck{diagInfo, i18n.T("doc.lbl_visualizer"), i18n.T("doc.viz_missing"), nil})
		}

		if mpris.BusAvailable() {
			checks = append(checks, diagCheck{diagOK, "mpris", i18n.T("doc.mpris_ok"), nil})
		} else {
			checks = append(checks, diagCheck{diagInfo, "mpris", i18n.T("doc.mpris_missing"), nil})
		}

		// Solo cache, sin red: mismo motivo que checkUpdate en doctor.go — un
		// diagnóstico que se va diez segundos a git ls-remote deja de servir
		// justo cuando algo va mal.
		if latest, _ := update.Cached(); latest != "" && update.Newer(latest, version.Version) {
			checks = append(checks, diagCheck{diagInfo, i18n.T("doc.lbl_update"), i18n.Tf("doc.upd_avail", latest, version.Version), nil})
		} else {
			checks = append(checks, diagCheck{diagOK, i18n.T("doc.lbl_update"), i18n.Tf("doc.upd_none", version.Version), nil})
		}

		if cfgErr != nil {
			checks = append(checks, diagCheck{diagWarn, i18n.T("doc.lbl_config"), i18n.Tf("info.config_err", cfgErr), nil})
		}

		marks := map[diagLevel]string{
			diagOK:   st.playing.Render("ok"),
			diagInfo: st.dim.Render("info"),
			diagWarn: st.accent.Render("warn"),
			diagFail: st.errSt.Render("fail"),
		}
		warns, fails := 0, 0
		lines := make([]string, 0, len(checks)*2+1)
		for _, c := range checks {
			switch c.lvl {
			case diagWarn:
				warns++
			case diagFail:
				fails++
			}
			lines = append(lines, "  "+marks[c.lvl]+" "+st.dim.Render(c.label)+": "+c.detail)
			for _, extra := range c.cont {
				lines = append(lines, "      "+st.dim.Render(extra))
			}
		}
		lines = append(lines, st.dim.Render(i18n.Tf("doc.summary", warns, fails)))
		return conMsg{lines: lines}
	}
}

// conConfig espeja `maly config`: la configuración efectiva (defaults ←
// preset ← [keys] del usuario).
func (m *Model) conConfig() tea.Cmd {
	st := m.st
	return func() tea.Msg {
		cfg, cfgErr := config.Load()
		unset := st.dim.Render(i18n.T("info.unset"))
		var lines []string
		sec := func(titleKey string) { lines = append(lines, st.accent.Render(i18n.T(titleKey))) }
		row := func(labelKey, value string) {
			lines = append(lines, "  "+st.dim.Render(i18n.T(labelKey))+": "+value)
		}
		note := func(text string) { lines = append(lines, "  "+st.dim.Render(text)) }

		lines = append(lines, st.text.Render(config.ConfigPath()))

		sec("config.sec_general")
		row("config.music_dir", orUnsetTUI(cfg.MusicDir, unset))
		row("info.language", orUnsetTUI(cfg.Language, unset))
		row("info.controls", cfg.Controls)

		sec("config.sec_daemon")
		row("info.update_check", ipc.OnOff(cfg.UpdateCheck))
		row("info.scan_durations", ipc.OnOff(cfg.ScanDurations))

		sec("config.sec_theme")
		row("config.transparent", ipc.OnOff(cfg.Theme.Transparent))
		row("config.accent", cfg.Theme.Accent)
		row("config.border", cfg.Theme.Border)
		row("config.text", cfg.Theme.Text)
		row("config.theme_dim", cfg.Theme.Dim)
		row("config.playing", cfg.Theme.Playing)
		row("config.logo", strings.Join(cfg.Theme.Logo, ", "))

		sec("config.sec_visualizer")
		row("config.viz_enabled", ipc.OnOff(cfg.Visualizer.Enabled))
		row("config.color_low", cfg.Visualizer.ColorLow)
		row("config.color_high", cfg.Visualizer.ColorHigh)
		row("config.bars_gravity", fmt.Sprintf("%g", cfg.Visualizer.BarsGravity))
		row("config.viz_backend", cfg.Visualizer.Backend)

		sec("config.sec_ytdlp")
		row("info.ytdlp_cookies", orUnsetTUI(cfg.Ytdlp.CookiesFromBrowser, unset))

		sec("config.sec_keys")
		note(i18n.T("config.keys_note"))
		names := make([]string, 0, len(cfg.Keys))
		for k := range cfg.Keys {
			names = append(names, k)
		}
		sort.Strings(names)
		for _, k := range names {
			lines = append(lines, "  "+st.dim.Render(k)+": "+cfg.Keys[k])
		}

		if cfgErr != nil {
			note(i18n.Tf("info.config_err", cfgErr))
		}
		return conMsg{lines: lines}
	}
}
