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
progreso de yt-dlp pasa directo al terminal, cero parsing. Las ÚNICAS salidas
de yt-dlp que maly lee son sus interfaces de MÁQUINA, nunca el texto de
progreso: `--dump-json` (`getter.Search`, 1.15.0) y `--print-to-file
after_move:filepath` (`Opts.PathsFile` + `getter.ReadPaths`, A-07). La
distinción está detallada en la cabecera de `internal/getter/search.go` y en
la 1.15.0 del roadmap.

El **criterio de éxito de una descarga** necesita las dos señales, y ninguna
sola basta: el código de salida no ve que una búsqueda sin resultados sale 0
sin bajar nada (lección de la 1.15.0), y el diff del directorio no ve que
yt-dlp NO rebaja lo que ya existe —sale 0, no toca el disco, diff vacío— con
lo que "ya lo tenías" y "no encontré nada" colapsaban en el mismo error, con
código 1, pese a tener remedios opuestos (A-07). Con el archivo de rutas los
cuatro desenlaces quedan separados:

| yt-dlp | rutas | diff | maly dice |
| --- | --- | --- | --- |
| 0 | 1 | 1 nuevo | descargado |
| 0 | 1 | vacío | **ya lo tenías** (y NO es error) |
| 0 | 0 | vacío | la búsqueda no encontró nada |
| ≠0 | — | — | falló (+ `Cleanup` de intermedios) |

Que el hook dispare también para un archivo SALTADO se comprobó en la fuente
de yt-dlp (las dos ramas de `report_file_already_downloaded` no retornan
temprano, así que se llega a `run_all_pps("after_move")`) **y** en vivo contra
yt-dlp real. Un archivo de rutas ausente o vacío degrada al criterio anterior,
el diff solo.
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
