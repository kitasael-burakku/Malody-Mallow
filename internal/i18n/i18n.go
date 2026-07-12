// Package i18n centraliza todos los textos visibles de maly en inglés y
// español. El idioma activo es global al proceso (el demonio embebido y la
// TUI comparten selección) y seguro entre goroutines.
package i18n

import (
	"fmt"
	"sync/atomic"
)

const (
	en int32 = iota
	es
)

var current atomic.Int32 // por defecto en (0): English

// Set fija el idioma activo a partir del código del config ("en" | "es").
// Cualquier otro valor cae en inglés.
func Set(code string) {
	if code == "es" {
		current.Store(es)
	} else {
		current.Store(en)
	}
}

// Code devuelve el código del idioma activo.
func Code() string {
	if current.Load() == es {
		return "es"
	}
	return "en"
}

// T devuelve la traducción de key en el idioma activo. Si la clave no
// existe devuelve la clave tal cual (falla visible, no silenciosa).
func T(key string) string {
	return TL(Code(), key)
}

// Tf es T + fmt.Sprintf.
func Tf(key string, a ...any) string {
	return fmt.Sprintf(T(key), a...)
}

// TL traduce en un idioma explícito ("en" | "es"), independiente del global.
// Lo usa el demonio para responder en el idioma del cliente que pregunta;
// un código vacío o desconocido cae en el idioma activo del proceso.
func TL(code, key string) string {
	tr, ok := table[key]
	if !ok {
		return key
	}
	switch code {
	case "es":
		return tr[es]
	case "en":
		return tr[en]
	default:
		return tr[current.Load()]
	}
}

// TLf es TL + fmt.Sprintf.
func TLf(code, key string, a ...any) string {
	return fmt.Sprintf(TL(code, key), a...)
}

// table indexa [en, es] por clave.
var table = map[string][2]string{
	// ---- TUI: paneles y estados ----
	"tui.too_small":       {"terminal too small for maly", "terminal muy pequeña para maly"},
	"tui.lib_title":       {"Library (%d)", "Biblioteca (%d)"},
	"tui.queue_title":     {"Queue (%d)", "Cola (%d)"},
	"tui.viz_title":       {"Visualizer", "Visualizador"},
	"tui.now_title":       {"Now Playing", "Ahora suena"},
	"tui.help_title":      {"Help", "Ayuda"},
	"tui.lib_empty":       {" (library empty — run maly scan)", " (biblioteca vacía — ejecuta maly scan)"},
	"tui.queue_empty":     {" (queue empty — `a` adds from the library)", " (cola vacía — `a` agrega desde la biblioteca)"},
	"tui.nothing":         {" ⏹ nothing playing — enter in the Library plays", " ⏹ nada suena — enter en la Biblioteca reproduce"},
	"tui.footer":          {" tab panels · enter play · a add · space pause · / filter · ctrl+p palette · ctrl+o songs · ? help · q quit", " tab paneles · enter reproduce · a agrega · espacio pausa · / filtra · ctrl+p paleta · ctrl+o canciones · ? ayuda · q salir"},
	"tui.footer_embedded": {" (embedded daemon)", " (servicio integrado)"},
	"tui.no_daemon":       {" the daemon is not responding…", " el servicio no responde…"},
	"tui.viz_fake":        {"pw-record/parec not found: visualizer in animation mode", "sin pw-record/parec: visualizador en modo animación"},
	"tui.svc_version":     {"the daemon runs v%s (another binary): restart it to update", "el servicio corre v%s (otro binario): reinícialo para actualizar"},
	"tui.lib_err":         {"library: %s", "biblioteca: %s"},
	"tui.lib_empty_flash": {"Library is empty: run `maly scan` or check music_dir in the config", "Biblioteca vacía: ejecuta `maly scan` o revisa music_dir en el config"},
	"tui.filter_ph":       {"filter…", "filtrar…"},
	"tui.unknown_artist":  {"(unknown)", "(desconocido)"},
	"tui.no_album":        {"(no album)", "(sin álbum)"},

	// ---- TUI: ayuda (?) ----
	"help.play_pause":     {"play / pause", "reproducir / pausar"},
	"help.next_prev":      {"next / previous", "siguiente / anterior"},
	"help.volume":         {"volume ±5%", "volumen ±5%"},
	"help.seek":           {"seek ±5s", "seek ±5s"},
	"help.switch":         {"switch panel", "cambiar de panel"},
	"help.enter":          {"play / expand", "reproducir / expandir"},
	"help.add":            {"add to queue", "agregar a la cola"},
	"help.remove":         {"remove from queue", "quitar de la cola"},
	"help.filter":         {"filter current panel", "filtrar panel actual"},
	"help.shuffle_repeat": {"shuffle / repeat", "shuffle / repeat"},
	"help.toggle_viz":     {"toggle visualizer", "alternar visualizador"},
	"help.palette":        {"command palette", "paleta de comandos"},
	"help.songs":          {"song selector", "selector de canciones"},
	"help.playlists":      {"playlist panel", "panel de playlists"},
	"help.playlist_add":   {"add selection to a playlist", "agregar selección a una playlist"},
	"help.quit":           {"quit", "salir"},
	"help.vim_nav":        {"navigate (also arrows)", "navegar (también flechas)"},
	"help.jump_scroll":    {"top / bottom · half page", "inicio / final · media página"},
	"help.close":          {"  any key closes this help", "  cualquier tecla cierra esta ayuda"},
	"help.show":           {"show this help", "mostrar esta ayuda"},
	"help.space":          {"space", "espacio"},

	// ---- TUI: selector de idioma ----
	"lang.title": {"Choose language", "Elige idioma"},
	"lang.hint":  {"↑/↓ move · enter select", "↑/↓ mover · enter elegir"},

	// ---- TUI: paleta de comandos (consola) ----
	"con.title":       {"Command Palette", "Paleta de comandos"},
	"con.ph":          {"command… e.g. maly next", "comando… p. ej. maly next"},
	"con.hint":        {"enter runs · esc closes", "enter ejecuta · esc cierra"},
	"con.unknown":     {"unknown command %q — try help", "comando desconocido %q — prueba help"},
	"con.help_head":   {"available commands:", "comandos disponibles:"},
	"con.help_local":  {"also: viz (toggle visualizer) · cls (clear output) · quit", "también: viz (alternar visualizador) · cls (limpiar salida) · quit"},
	"con.usage_add":   {"usage: add <query|path>", "uso: add <consulta|ruta>"},
	"con.usage_jump":  {"usage: jump <position> (positions: queue)", "uso: jump <posición> (posiciones: queue)"},
	"con.usage_vol":   {"usage: vol <0-100|+N|-N>", "uso: vol <0-100|+N|-N>"},
	"con.usage_seek":  {"usage: seek <+N|-N|mm:ss>", "uso: seek <+N|-N|mm:ss>"},
	"con.queue_empty": {"the queue is empty", "la cola está vacía"},
	"con.viz_on":      {"visualizer on", "visualizador activado"},
	"con.viz_off":     {"visualizer off", "visualizador desactivado"},
	"con.scanning":    {"scanning library…", "escaneando biblioteca…"},
	"con.ok":          {"ok", "ok"},

	// ---- TUI: selector de canciones y picker genérico ----
	"songs.title": {"Songs", "Canciones"},
	"songs.ph":    {"search song…", "buscar canción…"},
	"songs.hint":  {"%d result(s) · enter plays · tab adds · esc closes", "%d resultado(s) · enter reproduce · tab agrega · esc cierra"},
	"songs.added": {"added: %s", "agregada: %s"},
	"sel.title":   {"Select Track", "Elegir canción"},
	"sel.none":    {"  no matches", "  sin coincidencias"},

	// ---- TUI: panel de playlists ----
	"plsel.title":    {"Playlists", "Playlists"},
	"plsel.ph":       {"search playlist…", "buscar playlist…"},
	"plsel.hint":     {"%d · enter plays · tab queues · ctrl+n new · ctrl+x delete", "%d · enter reproduce · tab encola · ctrl+n crea · ctrl+x borra"},
	"plsel.hint_add": {"%d · enter adds %d track(s) · ctrl+n creates & adds", "%d · enter agrega %d pista(s) · ctrl+n crea y agrega"},
	"plsel.queued":   {"queued: %s", "encolada: %s"},
	"plsel.empty":    {"  no playlists — type a name and press ctrl+n", "  no hay playlists — escribe un nombre y pulsa ctrl+n"},

	// ---- Estado (CLI y consola) ----
	"st.stopped": {"⏹ nothing playing  vol %d%%  shuffle: %s  repeat: %s  queue: %d track(s)", "⏹ nada suena  vol %d%%  shuffle: %s  repeat: %s  cola: %d pista(s)"},
	"st.line2":   {"  %s / %s  vol %d%%  shuffle: %s  repeat: %s  queue %d/%d", "  %s / %s  vol %d%%  shuffle: %s  repeat: %s  cola %d/%d"},

	// ---- CLI: ayuda ----
	"cli.tagline":           {"a local music player for your terminal", "reproductor de música local para tu terminal"},
	"cli.sec_usage":         {"USAGE", "USO"},
	"cli.usage_tui":         {"open the TUI (starts the daemon if needed)", "abre la TUI (inicia el servicio si hace falta)"},
	"cli.usage_daemon":      {"run only the daemon (headless)", "inicia solo el servicio en segundo plano"},
	"cli.sec_playback":      {"PLAYBACK", "REPRODUCCIÓN"},
	"cli.sec_playback_note": {"require a running daemon or an open TUI", "requieren el servicio corriendo o la TUI abierta"},
	"cli.play":              {"resume, or search the library and play", "reanuda; con consulta busca y reproduce"},
	"cli.select":            {"pick a track with fuzzy search and play it", "elige una canción con búsqueda fuzzy y reprodúcela"},
	"cli.jump":              {"jump to a queue position", "salta a una posición de la cola"},
	"cli.pause":             {"pause playback", "pausa la reproducción"},
	"cli.toggle":            {"toggle play/pause", "alterna play/pausa"},
	"cli.stop":              {"stop playback", "detiene la reproducción"},
	"cli.next":              {"next track", "pista siguiente"},
	"cli.prev":              {"previous track", "pista anterior"},
	"cli.add":               {"add query results or a path to the queue", "agrega a la cola (consulta o ruta)"},
	"cli.queue":             {"show the queue", "muestra la cola"},
	"cli.status":            {"show current status", "muestra el estado actual"},
	"cli.vol":               {"set or adjust volume", "fija o ajusta el volumen"},
	"cli.seek":              {"seek within the track", "cambia la posición"},
	"cli.shuffle":           {"toggle or set shuffle", "alterna o fija shuffle"},
	"cli.repeat":            {"cycle or set repeat mode", "alterna o fija el modo repeat"},
	"cli.clear":             {"clear the queue", "vacía la cola"},
	"cli.sec_library":       {"LIBRARY", "BIBLIOTECA"},
	"cli.sec_library_note":  {"work without the daemon", "funcionan sin el servicio"},
	"cli.scan":              {"(re)scan the music library", "(re)escanea la biblioteca"},
	"cli.search":            {"search by title/artist/album", "busca por título/artista/álbum"},
	"cli.get":               {"download audio with yt-dlp into the library", "descarga audio con yt-dlp a la biblioteca"},
	"cli.playlist":          {"manage playlists (list|show|create|delete|add|remove|play|export|import)", "gestiona playlists (list|show|create|delete|add|remove|play|export|import)"},
	"cli.sec_other":         {"OTHER", "OTROS"},
	"cli.completions":       {"print the shell completion script", "imprime el script de autocompletado del shell"},
	"cli.help_cmd":          {"show this help", "muestra esta ayuda"},
	"cli.version_cmd":       {"show version (and the running daemon's)", "muestra la versión (y la del servicio si corre)"},
	"cli.version_svc":       {"daemon: v%s", "servicio: v%s"},
	"cli.version_svc_old":   {"daemon: v%s — differs from the client; restart the daemon to update it", "servicio: v%s — distinta del cliente; reinicia el servicio para actualizarlo"},
	"cli.lang_cmd":          {"change the interface language", "cambia el idioma de la interfaz"},
	"cli.lang_set":          {"Language set to %s", "Idioma cambiado a %s"},
	"cli.lang_invalid":      {"invalid language %q (use en or es)", "idioma inválido %q (usa en o es)"},
	"cli.controls":          {"show or set the controls preset", "muestra o cambia el esquema de controles"},
	"cli.controls_head":     {"Control presets:", "Esquemas de controles:"},
	"cli.controls_hint":     {"switch with: maly controls <preset>", "cambia con: maly controls <preset>"},
	"cli.controls_set":      {"Controls set to %q", "Controles cambiados a %q"},
	"cli.controls_invalid":  {"unknown preset %q (available: %s)", "esquema desconocido %q (disponibles: %s)"},
	"cli.preset_default":    {"arrows + letters (the classic keys)", "flechas + letras (teclas clásicas)"},
	"cli.preset_vim":        {"x removes, < / > prev/next (hjkl, gg, G always work)", "x quita, < / > anterior/siguiente (hjkl, gg y G siempre funcionan)"},
	"cli.sec_examples":      {"EXAMPLES", "EJEMPLOS"},
	"cli.sec_keys":          {"TUI KEYS", "ATAJOS EN LA TUI"},
	"cli.sec_keys_note":     {"press ? inside the TUI for the full list", "pulsa ? dentro de la TUI para la lista completa"},
	"cli.unknown":           {"maly: unknown subcommand %q", "maly: subcomando desconocido %q"},
	"cli.more":              {"run `maly -h` for help", "ejecuta `maly -h` para ver la ayuda"},

	// ---- CLI: cliente y utilidades ----
	"cli.no_daemon":         {"the maly daemon is not running; open `maly` or run `maly daemon`", "el servicio de maly no está corriendo; abre `maly` o ejecuta `maly daemon`"},
	"cli.usage_add_cmd":     {"usage: maly add <query|path>", "uso: maly add <consulta|ruta>"},
	"cli.usage_jump_cmd":    {"usage: maly jump <position>  (positions: maly queue)", "uso: maly jump <posición>  (posiciones: maly queue)"},
	"cli.usage_vol_cmd":     {"usage: maly vol <0-100|+N|-N>", "uso: maly vol <0-100|+N|-N>"},
	"cli.usage_seek_cmd":    {"usage: maly seek <+N|-N|mm:ss>", "uso: maly seek <+N|-N|mm:ss>"},
	"cli.usage_search":      {"usage: maly search <query>", "uso: maly search <consulta>"},
	"cli.usage_completions": {"usage: maly completions <shell>  (supported: %s)", "uso: maly completions <shell>  (disponibles: %s)"},
	"cli.daemon_listening":  {"maly daemon listening on %s", "servicio de maly escuchando en %s"},
	"cli.queue_empty":       {"The queue is empty. Use maly add <query> or maly play <query>.", "La cola está vacía. Usa maly add <consulta> o maly play <consulta>."},
	"cli.scan_start":        {"Scanning %s ...", "Escaneando %s ..."},
	"cli.scan_warn":         {"  warning: %s", "  aviso: %s"},
	"cli.scan_done":         {"Done: %d new, %d updated, %d removed (%d tracks total)", "Listo: %d nuevas, %d actualizadas, %d eliminadas (%d pistas en total)"},
	"cli.scan_empty":        {"The library is empty. Is there music in %s? You can pass another path: maly scan <path>", "La biblioteca está vacía. ¿Hay música en %s? Puedes indicar otra ruta: maly scan <ruta>"},
	"cli.scan_noexist":      {"%s does not exist (from %s). Point maly at your music with: maly scan <path>", "%s no existe (viene de %s). Indica dónde está tu música con: maly scan <ruta>"},
	"cli.search_none":       {"No results. Did you scan the library? (maly scan)", "Sin resultados. ¿Ya escaneaste la biblioteca? (maly scan)"},
	"cli.usage_get_cmd":     {"usage: maly get <url|search>", "uso: maly get <url|búsqueda>"},
	"cli.get_missing":       {"maly get needs %s, which is not in your PATH", "maly get necesita %s, que no está en tu PATH"},
	"cli.get_install":       {"install it: sudo pacman -S %s · sudo apt install %s · sudo dnf install %s", "instálalo: sudo pacman -S %s · sudo apt install %s · sudo dnf install %s"},
	"cli.get_start":         {"Downloading %s → %s", "Descargando %s → %s"},
	"cli.get_err":           {"yt-dlp failed: %v (see its output above)", "yt-dlp falló: %v (revisa su salida arriba)"},
	"cli.get_scan":          {"Download finished — updating the library ...", "Descarga lista — actualizando la biblioteca ..."},
	"cli.tbl_header":        {"ID\tARTIST\tALBUM\t#\tTITLE", "ID\tARTISTA\tÁLBUM\t#\tTÍTULO"},

	// ---- CLI: playlists ----
	"pl.usage":        {"usage:\n  maly playlist list                  list playlists\n  maly playlist show <name>           list the playlist's tracks\n  maly playlist create <name>         create a playlist\n  maly playlist delete <name>         delete a playlist\n  maly playlist add <name> <query>    add search results\n  maly playlist remove <name> <pos>   remove the track at that position\n  maly playlist play <name>           play the playlist (needs daemon)\n  maly playlist export <name> [file]  write the playlist as M3U\n  maly playlist import <file> [name]  create a playlist from an M3U", "uso:\n  maly playlist list                      lista las playlists\n  maly playlist show <nombre>             lista las pistas de la playlist\n  maly playlist create <nombre>           crea una playlist\n  maly playlist delete <nombre>           elimina una playlist\n  maly playlist add <nombre> <consulta>   agrega resultados de búsqueda\n  maly playlist remove <nombre> <pos>     quita la pista en esa posición\n  maly playlist play <nombre>             reproduce la playlist (requiere el servicio)\n  maly playlist export <nombre> [archivo] escribe la playlist como M3U\n  maly playlist import <archivo> [nombre] crea una playlist desde un M3U"},
	"pl.usage_play":   {"usage: maly playlist play <name>", "uso: maly playlist play <nombre>"},
	"pl.usage_create": {"usage: maly playlist create <name>", "uso: maly playlist create <nombre>"},
	"pl.usage_delete": {"usage: maly playlist delete <name>", "uso: maly playlist delete <nombre>"},
	"pl.usage_add":    {"usage: maly playlist add <name> <query>", "uso: maly playlist add <nombre> <consulta>"},
	"pl.usage_show":   {"usage: maly playlist show <name>", "uso: maly playlist show <nombre>"},
	"pl.usage_remove": {"usage: maly playlist remove <name> <position>  (positions: maly playlist show <name>)", "uso: maly playlist remove <nombre> <posición>  (posiciones: maly playlist show <nombre>)"},
	"pl.removed":      {"Removed %s from %q", "Quitada %s de %q"},
	"pl.none":         {"No playlists. Create one with: maly playlist create <name>", "No hay playlists. Crea una con: maly playlist create <nombre>"},
	"pl.tbl_header":   {"PLAYLIST\tTRACKS", "PLAYLIST\tPISTAS"},
	"pl.created":      {"Playlist %q created", "Playlist %q creada"},
	"pl.deleted":      {"Playlist %q deleted", "Playlist %q eliminada"},
	"pl.no_results":   {"no results for %q", "sin resultados para %q"},
	"pl.added":        {"%d track(s) added to %q", "%d pista(s) agregadas a %q"},
	"pl.unknown":      {"unknown playlist subcommand %q", "subcomando playlist desconocido %q"},
	"pl.usage_export": {"usage: maly playlist export <name> [file.m3u]", "uso: maly playlist export <nombre> [archivo.m3u]"},
	"pl.usage_import": {"usage: maly playlist import <file.m3u> [name]", "uso: maly playlist import <archivo.m3u> [nombre]"},
	"pl.exported":     {"%d track(s) from %q written to %s", "%d pista(s) de %q escritas en %s"},
	"pl.imported":     {"Playlist %q created with %d track(s) from %s", "Playlist %q creada con %d pista(s) desde %s"},
	"pl.import_skip":  {"  skipped (not in the library): %s", "  saltada (no está en la biblioteca): %s"},

	// ---- Demonio ----
	"d.already":          {"another maly daemon is already running", "ya hay un servicio de maly corriendo"},
	"d.mpris_off":        {"MPRIS disabled: %v", "MPRIS desactivado: %v"},
	"d.invalid_req":      {"invalid request: %s", "petición inválida: %s"},
	"d.playing_n":        {"Playing %s (%d queued)", "Reproduciendo %s (%d en cola)"},
	"d.playing":          {"Playing %s", "Reproduciendo %s"},
	"d.paused":           {"Paused", "Pausado"},
	"d.stopped":          {"Stopped", "Detenido"},
	"d.no_next":          {"no next track in the queue", "no hay siguiente pista en la cola"},
	"d.queue_empty":      {"the queue is empty", "la cola está vacía"},
	"d.queue_empty_hint": {"the queue is empty; use maly play <query> or maly add", "la cola está vacía; usa maly play <consulta> o maly add"},
	"d.playnow_paths":    {"playnow requires paths", "playnow requiere rutas"},
	"d.added_n":          {"%d track(s) added to the queue", "%d pista(s) agregadas a la cola"},
	"d.also_playing":     {"; playing %s", "; reproduciendo %s"},
	"d.jump_oob":         {"position %d outside the queue", "posición %d fuera de la cola"},
	"d.removed":          {"Track removed from the queue", "Pista quitada de la cola"},
	"d.cleared":          {"Queue cleared", "Cola vaciada"},
	"d.vol_invalid":      {"invalid volume %q (use 0-100, +N or -N)", "volumen inválido %q (usa 0-100, +N o -N)"},
	"d.vol_set":          {"Volume %d%%", "Volumen %d%%"},
	"d.shuffle_on":       {"Shuffle on", "Shuffle activado"},
	"d.shuffle_off":      {"Shuffle off", "Shuffle desactivado"},
	"d.repeat_invalid":   {"invalid repeat mode %q (off|all|one)", "modo repeat inválido %q (off|all|one)"},
	"d.repeat":           {"Repeat: %s", "Repeat: %s"},
	"d.pl_empty":         {"playlist %q is empty", "la playlist %q está vacía"},
	"d.playing_pl":       {"Playing playlist %q (%d tracks)", "Reproduciendo playlist %q (%d pistas)"},
	"d.scan_busy":        {"a scan is already in progress", "ya hay un escaneo en curso"},
	"d.scan_done":        {"Scan done: %d new, %d updated, %d removed (%d total)", "Escaneo listo: %d nuevas, %d actualizadas, %d eliminadas (%d en total)"},
	"d.unknown_cmd":      {"unknown command %q", "comando desconocido %q"},
	"d.seek_usage":       {"usage: seek <+N|-N|mm:ss>", "uso: seek <+N|-N|mm:ss>"},
	"d.seek_mmss":        {"invalid position %q (use mm:ss)", "posición inválida %q (usa mm:ss)"},
	"d.seek_offset":      {"invalid offset %q", "desplazamiento inválido %q"},
	"d.seek_abs":         {"invalid position %q (use +N, -N or mm:ss)", "posición inválida %q (usa +N, -N o mm:ss)"},
	"d.missing_query":    {"missing query or path", "falta la consulta o ruta"},
	"d.no_results":       {"no results for %q (did you run maly scan?)", "sin resultados para %q (¿escaneaste con maly scan?)"},
	"d.no_audio":         {"no audio files in %s", "no hay audio en %s"},
	"d.track_failed":     {"cannot play %v; skipping", "no se pudo reproducir %v; saltando"},
	"d.queue_failed":     {"no track in the queue could be played; stopping", "ninguna pista de la cola se pudo reproducir; deteniendo"},
	"d.listen":           {"cannot listen on %s", "no pude escuchar en %s"},

	// ---- IPC y arranque ----
	"ipc.send":         {"sending to the daemon", "enviando al servicio"},
	"ipc.read":         {"reading daemon response", "leyendo respuesta del servicio"},
	"ipc.invalid":      {"invalid daemon response", "respuesta inválida del servicio"},
	"cli.embedded_err": {"starting embedded daemon", "iniciando el servicio integrado"},

	// ---- Config ----
	"cfg.no_home":       {"cannot determine your home directory (set $HOME)", "no pude determinar tu carpeta home (define $HOME)"},
	"cfg.write_default": {"writing default config", "escribiendo config por defecto"},
	"cfg.read":          {"reading %s", "leyendo %s"},
	"cfg.invalid":       {"invalid config at %s", "config inválido en %s"},
	"cfg.runtime_bad":   {"runtime directory %s is not trustworthy (it must be a real directory owned by you); remove it or set XDG_RUNTIME_DIR", "el directorio runtime %s no es de fiar (debe ser un directorio real y tuyo); bórralo o define XDG_RUNTIME_DIR"},

	"pl.export_overwrite": {"%s already exists; overwrite? [y/N] ", "%s ya existe; ¿sobrescribir? [s/N] "},
	"pl.export_exists":    {"%s already exists (delete it or pick another name)", "%s ya existe (bórralo o elige otro nombre)"},
	"pl.export_kept":      {"%s left untouched", "%s queda intacto"},

	// ---- Reproductor (mpv) ----
	"p.no_mpv":      {"mpv is not installed (Arch: pacman -S mpv, Ubuntu: apt install mpv)", "mpv no está instalado (Arch: pacman -S mpv, Ubuntu: apt install mpv)"},
	"p.launch":      {"launching mpv", "lanzando mpv"},
	"p.connect":     {"could not connect to mpv IPC", "no pude conectar con el IPC de mpv"},
	"p.died":        {"mpv exited before creating its IPC socket", "mpv terminó antes de crear su socket IPC"},
	"p.configure":   {"configuring mpv", "configurando mpv"},
	"p.not_running": {"mpv is not running", "mpv no está corriendo"},
	"p.exited":      {"mpv exited", "mpv terminó"},
	"p.write":       {"writing to mpv", "escribiendo a mpv"},
	"p.no_reply":    {"mpv is not responding", "mpv no responde"},
	"p.seek":        {"seek failed", "no pude hacer seek"},

	// ---- MPRIS ----
	"m.name_taken": {"bus name %s is already taken", "el nombre %s ya está en uso"},

	// ---- Biblioteca ----
	"lib.mkdir":     {"creating %s", "creando %s"},
	"lib.open_db":   {"opening database", "abriendo base de datos"},
	"lib.schema":    {"creating schema", "creando esquema"},
	"lib.no_access": {"cannot access %s", "no puedo acceder a %s"},

	// ---- Origen de la ruta de música (para el error de scan) ----
	"music.src_config":   {"music_dir in your config", "music_dir de tu config"},
	"music.src_xdgenv":   {"$XDG_MUSIC_DIR", "$XDG_MUSIC_DIR"},
	"music.src_userdirs": {"XDG_MUSIC_DIR in user-dirs.dirs", "XDG_MUSIC_DIR de user-dirs.dirs"},
	"music.src_fallback": {"the default (no music dir configured)", "el valor por defecto (sin directorio configurado)"},
	"lib.not_dir":        {"%s is not a directory", "%s no es un directorio"},
	"lib.track_nf":       {"track %d not found", "pista %d no encontrada"},
	"lib.pl_nf":          {"playlist %q does not exist", "la playlist %q no existe"},
	"lib.pl_name":        {"missing playlist name", "falta el nombre de la playlist"},
	"lib.pl_exists":      {"playlist %q already exists", "la playlist %q ya existe"},
	"lib.pl_pos":         {"no position %d in %q (it has 1-%d)", "no hay posición %d en %q (tiene 1-%d)"},
	"lib.m3u_empty":      {"no library tracks found in %s (scan the music first: maly scan)", "ninguna pista de la biblioteca encontrada en %s (escanea la música primero: maly scan)"},
}
