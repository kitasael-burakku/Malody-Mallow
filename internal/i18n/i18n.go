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
	"tui.too_small":             {"Minimum %d×%d. Enlarge the terminal.", "Mínimo %d×%d. Amplía la terminal."},
	"tui.lib_title":             {"Library (%d)", "Biblioteca (%d)"},
	"tui.lib_title_pos":         {"Library (%d) · %d/%d", "Biblioteca (%d) · %d/%d"},
	"tui.queue_title":           {"Queue (%d)", "Cola (%d)"},
	"tui.queue_title_pos":       {"Queue (%d) · %d/%d", "Cola (%d) · %d/%d"},
	"tui.now_title":             {"Now Playing", "Ahora suena"},
	"tui.lyrics_title":          {"Lyrics", "Letras"},
	"tui.help_title":            {"Help", "Ayuda"},
	"tui.loading":               {"  loading…", "  cargando…"},
	"tui.filter_clear":          {"  esc: clear filter", "  esc: limpiar filtro"},
	"tui.lib_empty":             {" (library empty)", " (biblioteca vacía)"},
	"tui.queue_empty":           {" (queue empty)", " (cola vacía)"},
	"tui.nothing":               {" ⏹ nothing playing — enter in the Library plays", " ⏹ nada suena — enter en la Biblioteca reproduce"},
	"tui.scan_hint":             {"Run maly scan", "Ejecuta maly scan"},
	"tui.queue_add_hint":        {"From the library: %s adds", "Desde biblioteca: %s agrega"},
	"hint.help":                 {"help", "ayuda"},
	"hint.filter":               {"filter", "filtra"},
	"hint.add":                  {"add", "agrega"},
	"hint.remove":               {"remove", "quita"},
	"hint.play":                 {"play", "reproduce"},
	"hint.expand":               {"expand", "expande"},
	"hint.collapse":             {"collapse", "pliega"},
	"hint.pause":                {"play/pause", "play/pausa"},
	"hint.palette":              {"commands", "comandos"},
	"hint.songs":                {"songs", "canciones"},
	"hint.to_queue":             {"queue", "cola"},
	"hint.to_library":           {"library", "biblioteca"},
	"hint.filter_edit":          {"esc clears · enter accepts · ↑/↓ move", "esc limpia · enter fija · ↑/↓ mueve"},
	"help.group_queue":          {"Queue & playlists", "Cola y playlists"},
	"help.group_console":        {"Command palette", "Paleta de comandos"},
	"help.group_downloads":      {"Downloads", "Descargas"},
	"help.console_enter":        {"fill suggestion; press again to run", "rellenar sugerencia; otra pulsación ejecuta"},
	"help.console_scroll":       {"scroll output", "desplazar salida"},
	"help.console_ends":         {"start / end of output", "inicio / final de la salida"},
	"help.get_retry":            {"retry indexing after a failed scan", "reintentar indexación tras fallar el escaneo"},
	"help.intro":                {"Shortcuts by task · keys follow your settings", "Atajos por tarea · teclas según tu configuración"},
	"help.range":                {"lines %d–%d of %d", "filas %d–%d de %d"},
	"get.output_tail":           {"Last output from yt-dlp:", "Última salida de yt-dlp:"},
	"help.group_playback":       {"Playback", "Reproducción"},
	"help.group_library":        {"Library & navigation", "Biblioteca y navegación"},
	"help.group_tools":          {"Views & exit", "Vistas y salida"},
	"tui.footer_embedded":       {" (embedded daemon)", " (servicio integrado)"},
	"tui.no_daemon":             {" the daemon isn't responding — it may have crashed; try closing and reopening maly", " el servicio no responde — puede haberse caído; probá cerrar y volver a abrir maly"},
	"tui.viz_fake":              {"pw-record/parec not found: visualizer in animation mode", "sin pw-record/parec: visualizador en modo animación"},
	"tui.svc_version":           {"the daemon runs v%s (another binary): restart it to update", "el servicio corre v%s (otro binario): reinícialo para actualizar"},
	"tui.update_avail":          {"%s available — update with: maly update", "%s disponible — actualiza con: maly update"},
	"tui.update_avail_packaged": {"%s available — update with your package manager", "%s disponible — actualízalo con tu gestor de paquetes"},
	"tui.lib_err":               {"library: %s", "biblioteca: %s"},
	"tui.pl_no_id":              {"that track is outside the library; scan it first to add it to playlists", "esa pista está fuera de la biblioteca; escanéala primero para meterla en playlists"},
	"tui.lib_empty_flash":       {"Library is empty: run `maly scan` or check music_dir in the config", "Biblioteca vacía: ejecuta `maly scan` o revisa music_dir en el config"},
	"tui.lib_empty_flash_tui":   {"Library is empty: press %s then `scan`, or run `maly scan` from a terminal", "Biblioteca vacía: pulsa %s y escribe `scan`, o ejecuta `maly scan` desde una terminal"},
	"tui.filter_ph":             {"text · a: al: t: f: · -exclude", "texto · a: al: t: f: · -excluir"},
	"tui.filter_title":          {"Filter", "Filtrar"},
	"tui.unknown_artist":        {"(unknown)", "(desconocido)"},
	"tui.no_album":              {"(no album)", "(sin álbum)"},

	// ---- TUI: ayuda (?) ----
	"help.play_pause":     {"play / pause", "reproducir / pausar"},
	"help.next_prev":      {"next / previous", "siguiente / anterior"},
	"help.volume":         {"volume ±5%", "volumen ±5%"},
	"help.seek":           {"seek ±5 seconds", "avanzar / retroceder 5 segundos"},
	"help.switch":         {"switch panel", "cambiar de panel"},
	"help.enter":          {"play / expand", "reproducir / expandir"},
	"help.add":            {"add to queue", "agregar a la cola"},
	"help.remove":         {"remove from queue", "quitar de la cola"},
	"help.move":           {"move track up / down the queue", "subir / bajar la pista en la cola"},
	"help.filter":         {"filter this panel: a: al: t: f: -word", "filtrar este panel: a: al: t: f: -palabra"},
	"help.shuffle_repeat": {"shuffle / repeat", "shuffle / repeat"},
	"help.toggle_viz":     {"toggle visualizer", "alternar visualizador"},
	"help.now_playing":    {"now playing screen (cover & lyrics)", "pantalla ahora suena (carátula y letras)"},
	"np.no_lyrics":        {"no lyrics — drop a .lrc next to the file or embed them", "sin letras — deja un .lrc junto al archivo o embébelas"},
	"np.art_hidden":       {"(widen the terminal to see the cover art)", "(ensanchá la terminal para ver la carátula)"},
	"np.hint":             {"esc closes", "esc cierra"},
	"np.scroll_hint":      {"esc closes · j/k scroll", "esc cierra · j/k desplazan"},
	"np.track_no":         {"track %d", "pista %d"},
	"help.palette":        {"command palette", "paleta de comandos"},
	"help.songs":          {"find songs: a:artist al:album t:title f:file", "buscar canciones: a:artista al:álbum t:título f:archivo"},
	"help.playlists":      {"playlist panel", "panel de playlists"},
	"help.get":            {"search and download music", "buscar y descargar música"},
	"help.playlist_add":   {"add selection to a playlist", "agregar selección a una playlist"},
	"help.quit":           {"quit", "salir"},
	"help.vim_nav":        {"queue: move · library: fold / expand", "cola: mover · biblioteca: plegar / expandir"},
	"help.jump_scroll":    {"top / bottom · half page", "inicio / final · media página"},
	"help.page_keys":      {"page up/down · top / bottom", "avanzar/retroceder página · inicio / final"},
	"help.close":          {"  esc closes", "  esc cierra"},
	"help.scroll_hint":    {"esc closes · ↑/↓ pgup/pgdn scroll", "esc cierra · ↑/↓ pgup/pgdn desplazan"},
	"help.selection":      {"selection / current track (can coincide)", "selección / pista actual (pueden coincidir)"},
	"help.space":          {"space", "espacio"},

	// ---- TUI: selector de idioma ----
	"lang.title": {"Choose language", "Elige idioma"},
	"lang.hint":  {"↑/↓ move · enter/esc select", "↑/↓ mover · enter/esc elegir"},

	// ---- TUI: paleta de comandos (consola) ----
	"cmd.args.play":            {"[<query>...]", "[<consulta>...]"},
	"cmd.args.jump":            {"<pos>", "<pos>"},
	"cmd.args.move":            {"<from> <to>", "<de> <a>"},
	"cmd.args.remove":          {"<pos>", "<pos>"},
	"cmd.args.add":             {"<query|path>...", "<consulta|ruta>..."},
	"cmd.args.vol":             {"<0-100|+N|-N>", "<0-100|+N|-N>"},
	"cmd.args.seek":            {"<+N|-N|mm:ss>", "<+N|-N|mm:ss>"},
	"cmd.args.shuffle":         {"[on|off]", "[on|off]"},
	"cmd.args.repeat":          {"[off|all|one]", "[off|all|one]"},
	"cmd.args.scan":            {"[<path>]", "[<ruta>]"},
	"cmd.args.search":          {"<query>", "<consulta>"},
	"cmd.args.get":             {"<url|query>", "<url|consulta>"},
	"cmd.args.playlist":        {"<sub> [args]", "<sub> [args]"},
	"cmd.args.controls":        {"[<preset>]", "[<preset>]"},
	"cmd.args.logo":            {"[hex… | default]", "[hex… | default]"},
	"cmd.args.lang":            {"[en|es]", "[en|es]"},
	"cmd.args.completions":     {"<shell>", "<shell>"},
	"cmd.args.rescan":          {"[<path>]", "[<ruta>]"},
	"con.cmd_quit":             {"close the TUI", "cierra la TUI"},
	"con.cmd_cls":              {"clear console output", "limpia la salida de la consola"},
	"con.cmd_viz":              {"toggle visualizer", "alterna el visualizador"},
	"cmd.desc.playlist_list":   {"list playlists", "lista las playlists"},
	"cmd.desc.playlist_show":   {"show playlist tracks", "muestra canciones de una playlist"},
	"cmd.args.playlist_show":   {"<name>", "<nombre>"},
	"cmd.desc.playlist_create": {"create a playlist", "crea una playlist"},
	"cmd.args.playlist_create": {"<name>", "<nombre>"},
	"cmd.desc.playlist_delete": {"delete a playlist (confirmation required)", "elimina una playlist (requiere confirmación)"},
	"cmd.args.playlist_delete": {"<name>", "<nombre>"},
	"cmd.desc.playlist_add":    {"add songs to a playlist", "agrega canciones a una playlist"},
	"cmd.args.playlist_add":    {"<name> <query>...", "<nombre> <consulta>..."},
	"cmd.desc.playlist_remove": {"remove a song from a playlist", "quita una canción de una playlist"},
	"cmd.args.playlist_remove": {"<name> <pos>", "<nombre> <pos>"},
	"cmd.desc.playlist_play":   {"play a playlist", "reproduce una playlist"},
	"cmd.args.playlist_play":   {"<name>", "<nombre>"},
	"cmd.desc.playlist_export": {"export playlist to M3U", "exporta una playlist a M3U"},
	"cmd.args.playlist_export": {"<name> [<file>]", "<nombre> [<archivo>]"},
	"cmd.desc.playlist_import": {"import playlist from M3U", "importa una playlist desde M3U"},
	"cmd.args.playlist_import": {"<file> [<name>]", "<archivo> [<nombre>]"},
	"cmd.desc.get_pick":        {"search and choose a download", "busca y elige una descarga"},
	"cmd.args.get_pick":        {"<search>", "<búsqueda>"},
	"cmd.desc.get_playlist":    {"download a playlist", "descarga una playlist"},
	"cmd.args.get_playlist":    {"<url> [<name>]", "<url> [<nombre>]"},
	"con.title":                {"Command Palette", "Paleta de comandos"},
	"con.ph":                   {"command… e.g. maly next", "comando… p. ej. maly next"},
	"con.hint":                 {"esc closes · enter runs · tab/shift+tab choose · ↑↓ history · pgup/pgdn output", "esc cierra · enter ejecuta · tab/shift+tab elige · ↑↓ historial · pgup/pgdn salida"},
	"con.hint_select":          {"esc edits · enter fills · tab/shift+tab choose · ↑↓ history", "esc edita · enter rellena · tab/shift+tab elige · ↑↓ historial"},
	"con.hint_confirm":         {"esc cancels · y + enter: yes · empty enter: no", "esc cancela · s + enter: sí · enter vacío: no"},
	"help.console_keys":        {"choose a suggestion · esc cancels selection", "elegir sugerencia · esc cancela selección"},
	"help.console_history":     {"command history / draft", "historial de comandos / borrador"},
	"con.unknown":              {"unknown command %q — try help", "comando desconocido %q — prueba help"},
	"con.help_head":            {"available commands:", "comandos disponibles:"},
	"con.usage_add":            {"usage: add <query|path>... (a phrase goes in quotes)", "uso: add <consulta|ruta>... (una frase va entre comillas)"},
	"con.bad_quote":            {"unclosed quote", "comilla sin cerrar"},
	"con.usage_jump":           {"usage: jump <position> (positions: queue)", "uso: jump <posición> (posiciones: queue)"},
	"con.usage_move":           {"usage: move <from> <to> (positions: queue)", "uso: move <de> <a> (posiciones: queue)"},
	"con.usage_remove":         {"usage: remove <position> (positions: queue)", "uso: remove <posición> (posiciones: queue)"},
	"con.usage_vol":            {"usage: vol <0-100|+N|-N>", "uso: vol <0-100|+N|-N>"},
	"con.usage_seek":           {"usage: seek <+N|-N|mm:ss>", "uso: seek <+N|-N|mm:ss>"},
	"con.queue_empty":          {"the queue is empty", "la cola está vacía"},
	"con.viz_on":               {"visualizer on", "visualizador activado"},
	"con.viz_off":              {"visualizer off", "visualizador desactivado"},
	"con.scanning":             {"scanning library…", "escaneando biblioteca…"},
	"con.ok":                   {"ok", "ok"},

	// ---- TUI: selector de canciones y picker genérico ----
	"songs.title":     {"Songs", "Canciones"},
	"songs.ph":        {"a: · al: · t: · f: · -live", "a: · al: · t: · f: · -live"},
	"songs.hint":      {"esc close · enter play · tab add · %s next · %d", "esc cierra · enter reproduce · tab añade · %s después · %d"},
	"songs.added":     {"added: %s", "agregada: %s"},
	"sel.title":       {"Select Track", "Elegir canción"},
	"sel.none":        {"  no matches", "  sin coincidencias"},
	"sel.edit_filter": {"  Edit the filter above", "  Edita el filtro de arriba"},
	"sel.none_empty":  {"  library empty\n  Run maly scan from a terminal", "  biblioteca vacía\n  Ejecuta maly scan desde una terminal"},

	// ---- TUI: buscador de descargas (ctrl+g) ----
	"get.title":        {"Download", "Descargar"},
	"get.ph":           {"what to download…", "qué descargar…"},
	"get.idle":         {"  type what you want and press enter", "  escribe qué quieres y pulsa enter"},
	"get.searching":    {"searching %q on YouTube…", "buscando %q en YouTube…"},
	"get.none":         {"  no results for %q — try other words", "  sin resultados para %q — prueba con otras palabras"},
	"get.hint_search":  {"esc closes · enter searches", "esc cierra · enter busca"},
	"get.hint_results": {"esc closes · enter downloads · ↑↓ choose · %d · ✓ similar title (not identity)", "esc cierra · enter descarga · ↑↓ elige · %d · ✓ título similar (no identidad)"},
	"get.hint_again":   {"esc closes · enter searches again", "esc cierra · enter busca de nuevo"},
	"get.hint_pick":    {"esc cancels · enter downloads · ↑↓ choose · %d · ✓ similar title (not identity)", "esc cancela · enter descarga · ↑↓ elige · %d · ✓ título similar (no identidad)"},
	"get.hint_cancel":  {"esc cancels", "esc cancela"},

	// ---- TUI: panel de playlists ----
	"plsel.title":          {"Playlists", "Playlists"},
	"plsel.ph":             {"search playlist…", "buscar playlist…"},
	"plsel.hint":           {"esc closes · enter plays · tab queues · ctrl+n new · ctrl+x delete · %d", "esc cierra · enter reproduce · tab encola · ctrl+n crea · ctrl+x borra · %d"},
	"plsel.hint_add":       {"esc closes · %d result(s) · enter adds %d track(s) · ctrl+n creates & adds", "esc cierra · %d resultado(s) · enter agrega %d pista(s) · ctrl+n crea y agrega"},
	"plsel.queued":         {"queued: %s", "encolada: %s"},
	"plsel.read_err":       {"playlists: %s", "playlists: %s"},
	"plsel.empty":          {"  no playlists\n  Type a name + ctrl+n", "  no hay playlists\n  Escribe un nombre + ctrl+n"},
	"plsel.confirm_delete": {"delete %q? there's no undo — y/enter confirms, any other key cancels", "¿borrar %q? no hay deshacer — y/enter confirma, cualquier otra tecla cancela"},

	// ---- Estado (CLI y consola) ----
	"st.stopped":     {"⏹ nothing playing  vol %d%%  shuffle: %s  repeat: %s  queue: %d track(s)", "⏹ nada suena  vol %d%%  shuffle: %s  repeat: %s  cola: %d pista(s)"},
	"st.line2":       {"  %s / %s  vol %d%%  shuffle: %s  repeat: %s  queue %d/%d", "  %s / %s  vol %d%%  shuffle: %s  repeat: %s  cola %d/%d"},
	"st.player_dead": {"mpv is gone: nothing will play until the service restarts", "mpv ya no está: nada va a sonar hasta reiniciar el servicio"},

	// ---- CLI: ayuda ----
	"cli.tagline":           {"a local music player for your terminal", "reproductor de música local para tu terminal"},
	"cli.sec_usage":         {"USAGE", "USO"},
	"cli.usage_tui":         {"open the TUI (starts the daemon if needed)", "abre la TUI (inicia el servicio si hace falta)"},
	"cli.usage_daemon":      {"run only the daemon (headless)", "inicia solo el servicio en segundo plano"},
	"cli.sec_playback":      {"PLAYBACK", "REPRODUCCIÓN"},
	"cli.sec_playback_note": {"require a running daemon or an open TUI", "requieren el servicio corriendo o la TUI abierta"},
	"cli.undo":              {"undo the last queue removal without restarting playback", "deshacer la última eliminación de cola sin reiniciar audio"},
	"cli.usage_undo":        {"usage: undo (no arguments)", "uso: undo (sin argumentos)"},
	"d.undo_expired":        {"Nothing to undo: no removal available, it expired or was replaced", "No se puede deshacer: no hay eliminación disponible, caducó o fue sustituida"},
	"d.undone":              {"Queue entry restored; playback unchanged", "Entrada recuperada en la cola; reproducción sin cambios"},
	"help.undo_remove":      {"undo last removal, shared by all clients\nExpires on queue/track/mode changes or restart.\nRestores the entry, without rewinding audio.", "deshacer la última eliminación, compartida\nCaduca al cambiar cola/pista/modo o reiniciar.\nRecupera la entrada, sin retroceder el audio."},
	"hint.undo_remove":      {"undo", "deshacer"},
	"cli.playnext":          {"schedule tracks next, in order, without interrupting playback", "reproducir después, en orden, sin interrumpir la pista actual"},
	"cli.usage_playnext":    {"usage: playnext <query|path> [more…]", "uso: playnext <consulta|ruta> [más…]"},
	"d.playnext":            {"Scheduled next: %d track(s); playback unchanged", "Para después: %d pista(s); reproducción sin cambios"},
	"queue.next":            {"next", "después"},
	"help.play_next":        {"play next, in chosen order\nAdds a new entry; keeps current audio.", "reproducir después, en orden de elección\nCrea una entrada; conserva el audio actual."},
	"hint.play_next":        {"next", "después"},

	"cli.play":             {"resume, or search the library and play", "reanuda; con consultas busca y reproduce"},
	"cli.select":           {"pick a track with fuzzy search and play it", "elige una canción con búsqueda fuzzy y reprodúcela"},
	"cli.jump":             {"jump to a queue position", "salta a una posición de la cola"},
	"cli.move":             {"move a queue track to another position", "mueve una pista de la cola a otra posición"},
	"cli.remove":           {"remove a track from the queue", "quita una pista de la cola"},
	"cli.pause":            {"pause playback", "pausa la reproducción"},
	"cli.toggle":           {"toggle play/pause", "alterna play/pausa"},
	"cli.stop":             {"stop playback", "detiene la reproducción"},
	"cli.next":             {"next track", "pista siguiente"},
	"cli.prev":             {"previous track", "pista anterior"},
	"cli.add":              {"add query results or paths to the queue", "agrega a la cola (consultas o rutas)"},
	"cli.queue":            {"show the queue", "muestra la cola"},
	"cli.status":           {"show current status", "muestra el estado actual"},
	"cli.vol":              {"set or adjust volume", "fija o ajusta el volumen"},
	"cli.seek":             {"seek within the track", "cambia la posición"},
	"cli.shuffle":          {"toggle or set shuffle", "alterna o fija shuffle"},
	"cli.repeat":           {"cycle or set repeat mode", "alterna o fija el modo repeat"},
	"cli.clear":            {"clear the queue", "vacía la cola"},
	"cli.sec_library":      {"LIBRARY", "BIBLIOTECA"},
	"cli.sec_library_note": {"work without the daemon (except playlist play/save)", "funcionan sin el servicio (salvo playlist play/save)"},
	"cli.scan":             {"(re)scan the music library", "(re)escanea la biblioteca"},
	"cli.search":           {"search by title/artist/album", "busca por título/artista/álbum"},
	"cli.get":              {"download audio with yt-dlp into the library", "descarga audio con yt-dlp a la biblioteca"},
	"cli.playlist":         {"manage playlists (list|show|create|delete|add|remove|play|save|export|import)", "gestiona playlists (list|show|create|delete|add|remove|play|save|export|import)"},
	"cli.sec_other":        {"OTHER", "OTROS"},
	"cli.completions":      {"print the shell completion script", "imprime el script de autocompletado del shell"},
	"cli.help_cmd":         {"show this help", "muestra esta ayuda"},
	"cli.version_cmd":      {"show version (and the running daemon's)", "muestra la versión (y la del servicio si corre)"},
	"cli.update":           {"check for a new release and update maly", "busca una versión nueva y actualiza maly"},
	"cli.kill":             {"stop the daemon (from the TUI: also closes it)", "apaga el servicio (desde la TUI: la cierra también)"},
	"cli.kill_none":        {"no daemon running", "no hay ningún servicio corriendo"},
	"cli.version_svc":      {"daemon: v%s", "servicio: v%s"},
	"cli.version_svc_old":  {"daemon: v%s — differs from the client; restart the daemon to update it", "servicio: v%s — distinta del cliente; reinicia el servicio para actualizarlo"},
	"cli.info":             {"show paths, versions and library size", "muestra rutas, versiones y tamaño de la biblioteca"},
	"cli.doctor":           {"check that everything maly needs is in place", "revisa que esté todo lo que maly necesita"},
	"cli.lang_cmd":         {"change the interface language", "cambia el idioma de la interfaz"},
	"cli.lang_set":         {"Language set to %s", "Idioma cambiado a %s"},
	"cli.lang_invalid":     {"invalid language %q (use en or es)", "idioma inválido %q (usa en o es)"},
	"cli.controls":         {"show or set the controls preset", "muestra o cambia el esquema de controles"},
	"cli.controls_head":    {"Control presets:", "Esquemas de controles:"},
	"cli.controls_hint":    {"switch with: maly controls <preset>", "cambia con: maly controls <preset>"},
	"cli.controls_set":     {"Controls set to %q", "Controles cambiados a %q"},
	"cli.controls_invalid": {"unknown preset %q (available: %s)", "esquema desconocido %q (disponibles: %s)"},
	"cli.logo":             {"show or set the banner gradient colors", "muestra o cambia los colores del banner"},
	"cli.logo_current":     {"Banner gradient:", "Gradiente del banner:"},
	"cli.logo_usage":       {"usage: logo <#hex> <#hex> [more…] (2-8 stops) | logo default", "uso: logo <#hex> <#hex> [más…] (2 a 8 paradas) | logo default"},
	"cli.logo_set":         {"Banner colors updated", "Colores del banner actualizados"},
	"cli.logo_invalid":     {"%q is not a hex color (#rrggbb)", "%q no es un color hex (#rrggbb)"},
	"cli.preset_default":   {"arrows + letters (the classic keys)", "flechas + letras (teclas clásicas)"},
	"cli.preset_vim":       {"x removes, < / > prev/next (hjkl, gg, G always work)", "x quita, < / > anterior/siguiente (hjkl, gg y G siempre funcionan)"},
	"cli.sec_examples":     {"EXAMPLES", "EJEMPLOS"},
	"cli.sec_keys":         {"TUI KEYS", "ATAJOS EN LA TUI"},
	"cli.sec_keys_note":    {"as configured; also available in the TUI help", "tal como están configuradas; también disponibles en la ayuda de la TUI"},
	"cli.unknown":          {"maly: unknown subcommand %q", "maly: subcomando desconocido %q"},
	"cli.more":             {"run `maly -h` for help", "ejecuta `maly -h` para ver la ayuda"},

	// ---- CLI: cliente y utilidades ----
	"cli.no_daemon":         {"the maly daemon is not running; open `maly` or run `maly daemon`", "el servicio de maly no está corriendo; abre `maly` o ejecuta `maly daemon`"},
	"cli.usage_add_cmd":     {"usage: maly add <query|path>... (a phrase goes in quotes)", "uso: maly add <consulta|ruta>... (una frase va entre comillas)"},
	"cli.usage_jump_cmd":    {"usage: maly jump <position>  (positions: maly queue)", "uso: maly jump <posición>  (posiciones: maly queue)"},
	"cli.usage_move_cmd":    {"usage: maly move <from> <to>  (positions: maly queue)", "uso: maly move <de> <a>  (posiciones: maly queue)"},
	"cli.usage_remove_cmd":  {"usage: maly remove <position>  (positions: maly queue)", "uso: maly remove <posición>  (posiciones: maly queue)"},
	"cli.usage_vol_cmd":     {"usage: maly vol <0-100|+N|-N>", "uso: maly vol <0-100|+N|-N>"},
	"cli.usage_seek_cmd":    {"usage: maly seek <+N|-N|mm:ss>", "uso: maly seek <+N|-N|mm:ss>"},
	"cli.usage_search":      {"usage: maly search <query>", "uso: maly search <consulta>"},
	"cli.usage_completions": {"usage: maly completions <shell>  (supported: %s)", "uso: maly completions <shell>  (disponibles: %s)"},
	"cli.daemon_listening":  {"maly daemon listening on %s", "servicio de maly escuchando en %s"},
	"cli.queue_empty":       {"The queue is empty. Use maly add <query> or maly play <query>.", "La cola está vacía. Usa maly add <consulta> o maly play <consulta>."},
	"cli.scan_start":        {"Scanning %s ...", "Escaneando %s ..."},
	"cli.scan_progress":     {"scanning… %d files", "escaneando… %d archivos"},
	"cli.scan_durations":    {"reading durations… %d/%d", "leyendo duraciones… %d/%d"},
	"cli.scan_warn":         {"  warning: %s", "  aviso: %s"},
	"cli.dur_done":          {"Durations: %d read with ffprobe", "Duraciones: %d leídas con ffprobe"},
	"cli.dur_errs":          {"  %d files without readable duration", "  %d archivos sin duración legible"},
	"cli.dur_query_failed":  {"could not check pending durations: %v", "no se pudo revisar las duraciones pendientes: %v"},
	"cli.scan_done":         {"Done: %d new, %d updated, %d removed (%d tracks total)", "Listo: %d nuevas, %d actualizadas, %d eliminadas (%d pistas en total)"},
	"cli.scan_empty":        {"The library is empty. Is there music in %s? You can pass another path: maly scan <path>", "La biblioteca está vacía. ¿Hay música en %s? Puedes indicar otra ruta: maly scan <ruta>"},
	"cli.scan_noexist":      {"%s does not exist (from %s). Point maly at your music with: maly scan <path>", "%s no existe (viene de %s). Indica dónde está tu música con: maly scan <ruta>"},
	"cli.scan_noexist_arg":  {"%s does not exist", "%s no existe"},
	"cli.search_none":       {"No results. Did you scan the library? (maly scan)", "Sin resultados. ¿Ya escaneaste la biblioteca? (maly scan)"},
	"cli.usage_get_cmd":     {"usage: maly get <url|search> | maly get pick <search> | maly get playlist <url> [name]", "uso: maly get <url|búsqueda> | maly get pick <búsqueda> | maly get playlist <url> [nombre]"},
	"cli.usage_get_pick":    {"usage: maly get pick <search> — search and choose before downloading", "uso: maly get pick <búsqueda> — busca y elige antes de descargar"},
	"cli.get_missing":       {"maly get needs %s, which is not in your PATH", "maly get necesita %s, que no está en tu PATH"},
	"cli.get_install":       {"install it: sudo pacman -S %s · sudo apt install %s · sudo dnf install %s", "instálalo: sudo pacman -S %s · sudo apt install %s · sudo dnf install %s"},
	"cli.get_start":         {"Downloading %s → %s", "Descargando %s → %s"},
	"cli.get_err":           {"yt-dlp failed: %s", "yt-dlp falló: %s"},
	"cli.get_scan":          {"Updating the library ...", "Actualizando la biblioteca ..."},
	"cli.get_done":          {"Downloaded: %s", "Descargado: %s"},
	"cli.get_already":       {"Already present: %s; nothing to download", "Ya estaba: %s; no hay nada que descargar"},
	"get.files":             {"New: %d · existing: %d", "Nuevos: %d · existentes: %d"},
	"get.partial":           {"Partial download · files kept: %d", "Descarga parcial · archivos conservados: %d"},
	"get.not_indexed":       {"file was not indexed: %s", "no se indexó el archivo: %s"},
	"get.scan_failed":       {"Files kept; indexing failed", "Archivos conservados; falló la indexación"},
	"get.cli_next":          {"Optional next steps (nothing plays automatically):", "Acciones opcionales (nada se reproduce automáticamente):"},
	"get.cli_select":        {"Use maly select to choose this file safely", "Usa maly select para elegir este archivo"},
	"get.indexed":           {"Indexed: %d files", "Indexados: %d archivos"},
	"get.local_title":       {"Download files", "Archivos de descarga"},
	"get.local_hint":        {"esc close · enter play · tab add · %s next", "esc cierra · enter reproduce · tab añade · %s después"},
	"get.scanning_hint":     {"Indexing; actions available when finished · esc close", "Indexando; acciones disponibles al terminar · esc cierra"},
	"get.retry_hint":        {"ctrl+r retry scan · esc close", "ctrl+r reintenta escaneo · esc cierra"},
	"get.no_files":          {"No files to show", "No hay archivos para mostrar"},
	"help.get_result":       {"downloaded file: play / add", "archivo descargado: reproducir / añadir"},
	"cli.get_nothing":       {"the download left no track in your music folder — the search may have matched nothing, or yt-dlp reported an error above", "la descarga no dejó ninguna pista en tu carpeta de música — puede que la búsqueda no encontrara nada, o que yt-dlp reportara un error arriba"},

	// ---- espera a un demonio que ya tiene el lock pero aún no contesta ----
	"cli.wait_daemon":         {"waiting for the maly service to finish starting...", "esperando a que termine de arrancar el servicio de maly..."},
	"cli.wait_daemon_timeout": {"the maly service is starting but did not answer within %s", "el servicio de maly está arrancando pero no respondió en %s"},

	// ---- búsqueda de descargas (getter.Search) ----
	"cli.get_search_err":     {"yt-dlp search failed: %s", "la búsqueda con yt-dlp falló: %s"},
	"cli.get_search_timeout": {"the search timed out after %s — is there a connection?", "la búsqueda agotó su tiempo tras %s — ¿hay conexión?"},

	// ---- maly get playlist ----
	"cli.usage_get_playlist": {"usage: maly get playlist <url> [name]", "uso: maly get playlist <url> [nombre]"},
	"cli.get_pl_bad_name":    {"%q cannot be a playlist name (it isn't a plain path component)", "%q no puede ser nombre de playlist (no es un componente de ruta simple)"},
	"cli.get_pl_start":       {"Downloading playlist %s → %s", "Descargando playlist %s → %s"},
	"cli.get_pl_partial":     {"yt-dlp reported a failure (%v) — continuing with whatever did download; see its output above", "yt-dlp reportó un fallo (%v) — sigo con lo que sí se haya descargado; revisa su salida arriba"},
	"cli.get_pl_dl_failed":   {"yt-dlp didn't create any playlist folder — the download seems to have failed entirely; see its output above", "yt-dlp no creó ninguna carpeta de playlist — la descarga parece haber fallado del todo; revisa su salida arriba"},
	"cli.get_pl_ambiguous":   {"couldn't tell which new folder is the playlist's; pass a name: maly get playlist <url> <name>", "no pude distinguir cuál carpeta nueva es la de la playlist; indica un nombre: maly get playlist <url> <nombre>"},
	"cli.get_pl_no_title":    {"yt-dlp didn't report a usable playlist title; pass a name: maly get playlist <url> <name>", "yt-dlp no reportó un título de playlist usable; indica un nombre: maly get playlist <url> <nombre>"},
	"cli.get_pl_empty":       {"no tracks from %s ended up in the library", "ninguna pista de %s terminó en la biblioteca"},
	"cli.get_pl_done":        {"Playlist %q created: %d tracks", "Playlist %q creada: %d pistas"},

	// ---- maly info (hechos de la instalación; no juzga nada) ----
	"info.sec_versions": {"VERSIONS", "VERSIONES"},
	"info.sec_paths":    {"PATHS", "RUTAS"},
	"info.sec_music":    {"MUSIC", "MÚSICA"},
	"info.sec_library":  {"LIBRARY", "BIBLIOTECA"},
	"info.sec_config":   {"CONFIG", "CONFIG"},

	// ---- maly doctor: etiquetas de fila traducibles. mpv/ffprobe/
	// yt-dlp + ffmpeg/mpris se quedan como literales en doctor.go: son
	// nombres propios de herramientas externas, no conceptos.
	"doc.lbl_service":     {"service", "servicio"},
	"doc.lbl_svc_config":  {"service config", "config del servicio"},
	"doc.lbl_music_dir":   {"music_dir", "music_dir"},
	"doc.lbl_links":       {"linked dirs", "dirs enlazados"},
	"doc.links_found":     {"%d linked director(y/ies) under your music folder are NOT indexed", "%d directorio(s) enlazado(s) bajo tu carpeta de música NO se indexan"},
	"doc.links_hint":      {"%s → scan the destination: maly scan %s", "%s → escanea el destino: maly scan %s"},
	"doc.lbl_player":      {"player", "reproductor"},
	"doc.player_ok":       {"mpv is running", "mpv está corriendo"},
	"doc.player_dead":     {"the service is up but its mpv is gone: nothing can play", "el servicio está en pie pero su mpv ya no está: nada puede sonar"},
	"doc.player_dead_fix": {"restart it: systemctl --user restart maly (or maly kill and reopen)", "reinicialo: systemctl --user restart maly (o maly kill y volver a abrir)"},
	"doc.lbl_library":     {"library", "biblioteca"},
	"doc.lbl_visualizer":  {"visualizer", "visualizador"},
	"doc.lbl_update":      {"update", "actualización"},
	"doc.lbl_config":      {"config", "config"},
	"doc.lbl_keys":        {"keys", "teclas"},
	"info.binary":         {"binary", "binario"},
	"info.channel":        {"channel", "canal"},
	"info.channel_pkg":    {"package manager", "gestor de paquetes"},
	"info.channel_manual": {"manual (mallow-install.sh or go build)", "manual (mallow-install.sh o go build)"},
	"info.service":        {"service", "servicio"},
	// "no responde" y no "no corre": un demonio vivo pero ocupado arrancando
	// (espera hasta 5 s a mpv) tampoco contesta, y afirmar que no corre sería
	// mentir justo en el caso que más confunde.
	"info.svc_none":       {"not responding", "no responde"},
	"info.svc_mismatch":   {"(differs from this binary — restart it)", "(distinta de este binario — reinícialo)"},
	"info.config":         {"config", "config"},
	"info.logo":           {"banner art", "arte del banner"},
	"info.data":           {"data", "datos"},
	"info.database":       {"database", "base de datos"},
	"info.runtime":        {"runtime", "runtime"},
	"info.socket":         {"socket", "socket"},
	"info.music_path":     {"path", "ruta"},
	"info.music_src":      {"source", "origen"},
	"info.music_missing":  {"(does not exist)", "(no existe)"},
	"info.tracks":         {"tracks", "pistas"},
	"info.playlists":      {"playlists", "playlists"},
	"info.db_none":        {"no library yet — run `maly scan`", "aún no hay biblioteca — ejecuta `maly scan`"},
	"info.language":       {"language", "idioma"},
	"info.controls":       {"controls", "controles"},
	"info.update_check":   {"update_check", "update_check"},
	"info.scan_durations": {"scan_durations", "scan_durations"},
	"info.ytdlp_cookies":  {"ytdlp cookies", "cookies de ytdlp"},
	"info.unset":          {"(unset)", "(sin definir)"},
	"info.config_err":     {"the config could not be read in full: %v", "el config no se pudo leer del todo: %v"},

	// ---- maly config (configuración EFECTIVA: defaults ← preset ← usuario) ----
	"cli.config":            {"show the configuration currently in effect", "muestra la configuración que está en efecto ahora mismo"},
	"config.sec_general":    {"GENERAL", "GENERAL"},
	"config.sec_daemon":     {"DAEMON", "DEMONIO"},
	"config.sec_theme":      {"THEME", "TEMA"},
	"config.sec_visualizer": {"VISUALIZER", "VISUALIZADOR"},
	"config.sec_ytdlp":      {"YTDLP", "YTDLP"},
	"config.sec_keys":       {"KEYS", "TECLAS"},
	"config.music_dir":      {"music_dir", "music_dir"},
	"config.transparent":    {"transparent", "transparent"},
	"config.accent":         {"accent", "accent"},
	"config.border":         {"border", "border"},
	"config.text":           {"text", "text"},
	"config.theme_dim":      {"dim", "dim"},
	"config.playing":        {"playing", "playing"},
	"config.error":          {"error", "error"},
	"config.accent_dim":     {"accent_dim", "accent_dim"},
	"config.surface":        {"surface", "surface"},
	"config.progress_low":   {"progress_low", "progress_low"},
	"config.progress_high":  {"progress_high", "progress_high"},
	"config.progress_shad":  {"progress_shadow", "progress_shadow"},
	"config.banner":         {"banner", "banner"},
	"config.logo":           {"logo", "logo"},
	"config.viz_enabled":    {"enabled", "enabled"},
	"config.color_low":      {"color_low", "color_low"},
	"config.color_high":     {"color_high", "color_high"},
	"config.bars_gravity":   {"bars_gravity", "bars_gravity"},
	"config.viz_backend":    {"backend", "backend"},
	"config.keys_note":      {"(defaults ← controls preset ← your [keys] overrides)", "(defaults ← preset de controls ← tus overrides en [keys])"},

	// ---- maly doctor (veredictos; solo "fail" cambia el código de salida) ----
	"doc.summary":            {"%d warning(s), %d error(s)", "%d aviso(s), %d error(es)"},
	"doc.mpv_missing":        {"not in your PATH — maly cannot play anything", "no está en tu PATH — maly no puede reproducir nada"},
	"doc.svc_none":           {"not responding — library commands still work; `maly` opens the TUI", "no responde — los comandos de biblioteca funcionan igual; `maly` abre la TUI"},
	"doc.svc_ok":             {"responding · v%s", "responde · v%s"},
	"doc.svc_cfg_ok":         {"the service is using this same config", "el servicio usa este mismo config"},
	"doc.svc_cfg_stale":      {"the config changed since the last scan; it will apply on the next `maly scan`", "el config cambió desde el último escaneo; se aplicará en el próximo `maly scan`"},
	"doc.svc_cfg_err":        {"the service could not re-read the config: %s", "el servicio no pudo releer el config: %s"},
	"doc.svc_cfg_using":      {"it keeps scanning %s", "sigue escaneando %s"},
	"doc.svc_mismatch":       {"runs v%s, this binary is v%s", "corre v%s y este binario es v%s"},
	"doc.svc_restart":        {"restart the daemon to pick up the new binary", "reinicia el servicio para estrenar el binario nuevo"},
	"doc.music_missing":      {"%s does not exist (from %s)", "%s no existe (viene de %s)"},
	"doc.music_notdir":       {"%s is not a directory", "%s no es un directorio"},
	"doc.lib_none":           {"no database yet — run `maly scan`", "aún no hay base de datos — ejecuta `maly scan`"},
	"doc.lib_empty":          {"database with no tracks — run `maly scan`", "base de datos sin pistas — ejecuta `maly scan`"},
	"doc.lib_ok":             {"%d tracks · %d playlists", "%d pistas · %d playlists"},
	"doc.ffprobe_ok":         {"durations are read while scanning", "las duraciones se leen al escanear"},
	"doc.ffprobe_missing":    {"not in your PATH — durations stay unknown until played", "no está en tu PATH — las duraciones no se sabrán hasta reproducir"},
	"doc.ffprobe_off":        {"scan_durations is off in your config", "scan_durations está apagado en tu config"},
	"doc.get_ok":             {"`maly get` ready", "`maly get` listo"},
	"doc.viz_missing":        {"no pw-record/parec — visualizer runs in animation mode", "sin pw-record/parec — el visualizador va en modo animación"},
	"doc.mpris_ok":           {"session bus available", "hay bus de sesión"},
	"doc.mpris_missing":      {"no session bus — no playerctl or media keys", "sin bus de sesión — sin playerctl ni teclas multimedia"},
	"doc.upd_avail":          {"%s available (you have v%s) — run: maly update", "%s disponible (tienes v%s) — ejecuta: maly update"},
	"doc.upd_none":           {"nothing newer in the last check (v%s)", "nada más nuevo en el último chequeo (v%s)"},
	"doc.keys_ok":            {"no conflicts", "sin colisiones"},
	"doc.keys_conflict":      {"%d key(s) mapped to more than one action — the extra ones are unreachable", "%d tecla(s) mapeada(s) a más de una acción — las de más quedan inalcanzables"},
	"doc.keys_conflict_line": {"%q: %s", "%q: %s"},

	// ---- maly update (chequeo de releases vía git + mallow-install.sh) ----
	"up.current":        {"you're up to date (v%s)", "ya estás al día (v%s)"},
	"up.found":          {"%s available (you have v%s) — launching the installer ...", "%s disponible (tienes v%s) — lanzando el instalador ..."},
	"up.found_packaged": {"%s available (you have v%s) — update it with your package manager", "%s disponible (tienes v%s) — actualízalo con tu gestor de paquetes"},
	"up.no_git":         {"the update check needs git, which is not in your PATH", "el chequeo de actualización necesita git, que no está en tu PATH"},
	"up.no_tags":        {"no version tags found in the repository", "no hay tags de versión en el repositorio"},
	"up.no_curl":        {"curl is not in your PATH — install it or run the installer yourself: %s", "curl no está en tu PATH — instálalo o corre el instalador a mano: %s"},
	"up.git_failed":     {"couldn't check for updates: %s", "no pude chequear actualizaciones: %s"},
	"up.git_timeout":    {"couldn't check for updates: timed out reaching GitHub", "no pude chequear actualizaciones: se agotó el tiempo esperando a GitHub"},
	"up.done":           {"installer finished — restart the daemon and reopen the TUI to use the new version", "instalador terminado — reinicia el servicio y reabre la TUI para estrenar la versión nueva"},
	"cli.tbl_header":    {"ID\tARTIST\tALBUM\t#\tTITLE", "ID\tARTISTA\tÁLBUM\t#\tTÍTULO"},

	// ---- CLI: playlists ----
	"pl.usage":               {"usage:\n  maly playlist list                  list playlists\n  maly playlist show <name>           list the playlist's tracks\n  maly playlist create <name>         create a playlist\n  maly playlist delete <name>         delete a playlist\n  maly playlist add <name> <query>    add search results\n  maly playlist remove <name> <pos>   remove the track at that position\n  maly playlist play <name>           play the playlist (needs daemon)\n  maly playlist save <name>           save the whole queue as a new playlist (needs daemon)\n  maly playlist export <name> [file]  write the playlist as M3U\n  maly playlist import <file> [name]  create a playlist from an M3U", "uso:\n  maly playlist list                      lista las playlists\n  maly playlist show <nombre>             lista las pistas de la playlist\n  maly playlist create <nombre>           crea una playlist\n  maly playlist delete <nombre>           elimina una playlist\n  maly playlist add <nombre> <consulta>   agrega resultados de búsqueda\n  maly playlist remove <nombre> <pos>     quita la pista en esa posición\n  maly playlist play <nombre>             reproduce la playlist (requiere el servicio)\n  maly playlist save <nombre>             guarda toda la cola como playlist nueva (requiere el servicio)\n  maly playlist export <nombre> [archivo] escribe la playlist como M3U\n  maly playlist import <archivo> [nombre] crea una playlist desde un M3U"},
	"pl.usage_save":          {"usage: maly playlist save <name>", "uso: maly playlist save <nombre>"},
	"pl.save_empty":          {"queue is empty\nadd tracks before saving it", "la cola está vacía\nagrega pistas antes de guardarla"},
	"pl.save_unindexed":      {"track not indexed (nothing saved): %q\nScan its folder and retry:\nmaly scan <folder>", "pista sin indexar (no se guardó nada): %q\nEscanea su carpeta y reintenta:\nmaly scan <carpeta>"},
	"pl.save_no_library":     {"no library; index the queue's tracks and retry:\nmaly scan <folder>", "no hay biblioteca; indexa las pistas de la cola y reintenta:\nmaly scan <carpeta>"},
	"pl.queue_saved":         {"Queue saved to %q: %d tracks", "Cola guardada en %q: %d pistas"},
	"cmd.args.playlist_save": {"<name>", "<nombre>"},
	"cmd.desc.playlist_save": {"save the whole queue as a new playlist", "guarda toda la cola como una playlist nueva"},
	"help.save_queue":        {"save the whole queue as a new playlist (enter name)", "guardar toda la cola como playlist nueva (escribir nombre)"},
	"hint.save_queue":        {"save queue", "guardar cola"},
	"pl.usage_play":          {"usage: maly playlist play <name>", "uso: maly playlist play <nombre>"},
	"pl.usage_create":        {"usage: maly playlist create <name>", "uso: maly playlist create <nombre>"},
	"pl.usage_delete":        {"usage: maly playlist delete <name>", "uso: maly playlist delete <nombre>"},
	"pl.usage_add":           {"usage: maly playlist add <name> <query>...", "uso: maly playlist add <nombre> <consulta>..."},
	"pl.usage_show":          {"usage: maly playlist show <name>", "uso: maly playlist show <nombre>"},
	"pl.usage_remove":        {"usage: maly playlist remove <name> <position>  (positions: maly playlist show <name>)", "uso: maly playlist remove <nombre> <posición>  (posiciones: maly playlist show <nombre>)"},
	"pl.removed":             {"Removed %s from %q", "Quitada %s de %q"},
	"pl.none":                {"No playlists. Create one with: maly playlist create <name>", "No hay playlists. Crea una con: maly playlist create <nombre>"},
	// Distinta de pl.none: ahí la biblioteca existe y no tiene playlists;
	// acá no hay biblioteca todavía, y el remedio es escanear (A-12).
	"pl.no_library":     {"No library yet. Scan it first with: maly scan", "Todavía no hay biblioteca. Escaneala primero con: maly scan"},
	"pl.tbl_header":     {"PLAYLIST\tTRACKS", "PLAYLIST\tPISTAS"},
	"pl.created":        {"Playlist %q created", "Playlist %q creada"},
	"pl.deleted":        {"Playlist %q deleted", "Playlist %q eliminada"},
	"pl.delete_confirm": {"delete playlist %q? this can't be undone [y/N] ", "¿eliminar la playlist %q? no se puede deshacer [s/N] "},
	"pl.delete_kept":    {"kept %q", "%q se mantiene"},
	"pl.no_results":     {"no results for %q", "sin resultados para %q"},
	"pl.added":          {"%d track(s) added to %q", "%d pista(s) agregadas a %q"},
	"pl.added_dup":      {"%d track(s) added to %q; %d were already there", "%d pista(s) agregadas a %q; %d ya estaban"},
	"pl.add_nf_hint":    {"create it first: maly playlist create %s", "créala primero: maly playlist create %s"},
	"pl.unknown":        {"unknown playlist subcommand %q", "subcomando playlist desconocido %q"},
	"pl.usage_export":   {"usage: maly playlist export <name> [file.m3u]", "uso: maly playlist export <nombre> [archivo.m3u]"},
	"pl.usage_import":   {"usage: maly playlist import <file.m3u> [name]", "uso: maly playlist import <archivo.m3u> [nombre]"},
	"pl.exported":       {"%d track(s) from %q written to %s", "%d pista(s) de %q escritas en %s"},
	"pl.imported":       {"Playlist %q created with %d track(s) from %s", "Playlist %q creada con %d pista(s) desde %s"},
	"pl.import_skip":    {"  skipped (not in the library): %s", "  saltada (no está en la biblioteca): %s"},

	// Variante de pl.delete_confirm para la consola de la TUI: ahí no hay
	// prompt de stdin, así que la confirmación es la línea SIGUIENTE que se
	// escriba (ver la guarda al principio de execConsole).
	"pl.delete_confirm_console": {"delete playlist %q? this can't be undone — type y to confirm, anything else cancels", "¿eliminar la playlist %q? no se puede deshacer — escribe s para confirmar, cualquier otra cosa cancela"},

	// ---- Demonio ----
	"d.already":              {"another maly daemon is already running", "ya hay un servicio de maly corriendo"},
	"d.bye":                  {"daemon shutting down", "apagando el servicio"},
	"d.mpris_off":            {"MPRIS disabled: %v", "MPRIS desactivado: %v"},
	"d.invalid_req":          {"invalid request: %s", "petición inválida: %s"},
	"d.internal_error":       {"internal error handling the request", "error interno al procesar la petición"},
	"d.req_too_large":        {"request too large; maly refused it without running it", "petición demasiado grande; maly la rechazó sin ejecutarla"},
	"d.too_many_subs":        {"too many subscribers, try again later", "demasiados suscriptores, intenta de nuevo más tarde"},
	"d.panic_recovered":      {"panic recovered in dispatch(%q): %v", "panic recuperado en dispatch(%q): %v"},
	"player.panic_recovered": {"panic recovered in a player callback: %v", "panic recuperado en un callback del player: %v"},
	"d.playing_n":            {"Playing %s (%d queued)", "Reproduciendo %s (%d en cola)"},
	"d.playing":              {"Playing %s", "Reproduciendo %s"},
	"d.paused":               {"Paused", "Pausado"},
	"d.stopped":              {"Stopped", "Detenido"},
	"d.no_next":              {"no next track in the queue", "no hay siguiente pista en la cola"},
	"d.queue_empty":          {"the queue is empty", "la cola está vacía"},
	"d.queue_empty_hint":     {"the queue is empty; use maly play <query> or maly add", "la cola está vacía; usa maly play <consulta> o maly add"},
	"d.playnow_paths":        {"playnow requires paths", "playnow requiere rutas"},
	"d.added_n":              {"%d track(s) added to the queue", "%d pista(s) agregadas a la cola"},
	"d.also_playing":         {"; playing %s", "; reproduciendo %s"},
	"d.jump_oob":             {"position %d outside the queue", "posición %d fuera de la cola"},
	"d.removed":              {"Track removed; undo restores it while available", "Pista quitada; undo la recupera mientras esté disponible"},
	"d.moved":                {"Track moved", "Pista movida"},
	"d.cleared":              {"Queue cleared", "Cola vaciada"},
	"d.vol_invalid":          {"invalid volume %q (use 0-100, +N or -N)", "volumen inválido %q (usa 0-100, +N o -N)"},
	"d.vol_set":              {"Volume %d%%", "Volumen %d%%"},
	"d.shuffle_on":           {"Shuffle on", "Shuffle activado"},
	"d.shuffle_off":          {"Shuffle off", "Shuffle desactivado"},
	"d.shuffle_invalid":      {"invalid value %q (on|off)", "valor inválido %q (on|off)"},
	"d.repeat_invalid":       {"invalid repeat mode %q (off|all|one)", "modo repeat inválido %q (off|all|one)"},
	"d.repeat":               {"Repeat: %s", "Repeat: %s"},
	"d.pl_empty":             {"playlist %q is empty", "la playlist %q está vacía"},
	"d.playing_pl":           {"Playing playlist %q (%d tracks)", "Reproduciendo playlist %q (%d pistas)"},
	"d.scan_busy":            {"a scan is already in progress", "ya hay un escaneo en curso"},
	"d.scan_done":            {"Scan done: %d new, %d updated, %d removed (%d total)", "Escaneo listo: %d nuevas, %d actualizadas, %d eliminadas (%d en total)"},
	"d.scan_errs":            {" — %d files failed (details on the service's stderr)", " — %d archivos con error (detalles en el stderr del servicio)"},
	"d.dur_done":             {" — %d durations read", " — %d duraciones leídas"},
	"d.dur_errs":             {" — %d without readable duration", " — %d sin duración legible"},
	"d.dur_query_failed":     {" — could not check pending durations: %v", " — no se pudo revisar las duraciones pendientes: %v"},
	"d.unknown_cmd":          {"unknown command %q", "comando desconocido %q"},
	"d.seek_usage":           {"usage: seek <+N|-N|mm:ss>", "uso: seek <+N|-N|mm:ss>"},
	"d.seek_mmss":            {"invalid position %q (use mm:ss or hh:mm:ss)", "posición inválida %q (usa mm:ss o hh:mm:ss)"},
	"d.seek_offset":          {"invalid offset %q", "desplazamiento inválido %q"},
	"d.seek_abs":             {"invalid position %q (use +N, -N or mm:ss)", "posición inválida %q (usa +N, -N o mm:ss)"},
	"d.missing_query":        {"missing query or path", "falta la consulta o ruta"},
	"d.no_results":           {"no results for %q (did you run maly scan?)", "sin resultados para %q (¿escaneaste con maly scan?)"},
	"d.no_audio":             {"no audio files in %s", "no hay audio en %s"},
	"batch.no_match":         {"no results for %s", "sin resultados para %s"},
	"d.track_failed":         {"cannot play %v; skipping", "no se pudo reproducir %v; saltando"},
	"d.queue_failed":         {"no track in the queue could be played; stopping", "ninguna pista de la cola se pudo reproducir; deteniendo"},
	"d.listen":               {"cannot listen on %s", "no pude escuchar en %s"},

	// ---- IPC y arranque ----
	"ipc.send":    {"sending to the daemon", "enviando al servicio"},
	"ipc.read":    {"reading daemon response", "leyendo respuesta del servicio"},
	"ipc.invalid": {"invalid daemon response", "respuesta inválida del servicio"},
	"ipc.timeout": {"the daemon isn't responding yet (it may still be starting up); try again in a moment", "el servicio todavía no responde (puede estar arrancando); prueba de nuevo en un momento"},

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
	"p.bad_value":   {"invalid value (not a finite number)", "valor inválido (no es un número finito)"},

	// ---- ffprobe (duraciones) ----
	"probe.bad_duration": {"ffprobe reported no usable duration for %s", "ffprobe no dio una duración usable para %s"},

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
	// Guarda de purga del escaneo. UNA sola línea: el espejo de la consola
	// la dibuja dentro de un panel y un salto la partiría (la trampa que
	// costó dos intentos de test en la 1.15.0).
	"lib.scan_empty": {"no audio files under %s, but the library has %d tracks there: not removing them (is the disk mounted?). To empty the library on purpose, delete the database that `maly info` shows", "ningún archivo de audio bajo %s, pero la biblioteca tiene %d pistas ahí: no las voy a borrar (¿está montado el disco?). Para vaciar la biblioteca a propósito, borrá la base que muestra `maly info`"},
}
