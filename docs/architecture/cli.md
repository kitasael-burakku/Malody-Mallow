# `cmd/maly` — la CLI y el arranque de la TUI

Fuente única: esta ficha. `CLAUDE.md` solo lleva el resumen.

## `cmd/maly`

CLI. `commands.go` tiene la **tabla de comandos**: fuente única de
verdad para dispatch, help y completions de shell (bash/fish/zsh vía
`__complete` oculto). `tui.go` decide en `startOrAttach` si la TUI embebe el
demonio o entra como cliente: ante `ErrAlreadyRunning` NO se rinde —el flock
ya dijo que hay otro— sino que espera hasta 8 s a que conteste
(`waitForDaemon`), porque un demonio arrancando tarda hasta 5 s esperando a
mpv y no contesta al ping (1.16.4). `runScan` manda la ruta ya RESUELTA y
nunca `Query: ""`: el config del demonio es de cuando arrancó, así que
resolverla él anunciaba una ruta y escaneaba otra. Al agregar un subcomando de playlist, actualizar la lista
fija de `TestCompletePlaylistSubs`. El completado de pistas pide con TOPE
(`library.SearchLimit`, `completeFetch` filas) y no con `Search`: la palabra
parcial vacía es la consulta que recorre la biblioteca entera, y un TAB no
puede materializarla para mostrar treinta líneas — ver la 1.7.3. `get.go` es el wrapper de yt-dlp
(filosofía "como lazygit usa git": maly coordina herramientas externas, no
las reimplementa): descarga MP3 con metadata/carátula embebidas a `music_dir`
y re-escanea (vía IPC si el demonio responde, directo a la DB si no);
yt-dlp/ffmpeg opcionales vía `exec.LookPath` con mensaje de instalación; el
progreso de yt-dlp pasa directo al terminal, cero parsing. La ÚNICA salida
de yt-dlp que maly lee es `--dump-json` (`getter.Search`, 1.15.0): su
interfaz de máquina, no el texto de progreso — la distinción está detallada
en la cabecera de `internal/getter/search.go` y en la 1.15.0 del roadmap.
`[ytdlp] cookies_from_browser` del config viaja tal cual a
`--cookies-from-browser` (passthrough sin validar, "" = sin flag; los
comentarios del template del config son estáticos en español, sin i18n);
navegadores derivados van con ruta de perfil, p. ej. Zen (base Firefox,
perfiles en `~/.config/zen/`): `firefox:/ruta/al/perfil`. mp3 a propósito:
dhowden lee sus ID3 en el scan y la miniatura APIC es justo lo que
`mpris:artUrl` extrae. Tests sin red con un yt-dlp falso en el PATH
(`get_test.go`, mismo patrón que el mpv falso de `player_test.go`).
`info.go` y `doctor.go` son el diagnóstico, y la división entre ambos es el
contrato de salida: `info` lista HECHOS (rutas, versiones, música,
biblioteca, config) y sale siempre 0; `doctor` emite VEREDICTOS y sale 1
solo si algo impide reproducir. Ninguno necesita demonio ni red — es la
condición para que sirvan de algo — y ninguno duplica detección: reusan
`getter.Tools`, `probe.Available`, `viz.CaptureBackend`,
`mpris.BusAvailable`, `MusicDirOrigin` y `update.Cached`. `libraryStats`
(en `info.go`, compartido con doctor) abre por `openLibraryIfExists`
porque `library.Open` crearía la base. Ver el detalle y sus invariantes en
la 1.7.0 del roadmap.
