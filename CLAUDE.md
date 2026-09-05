# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Qué es

**Malody Mallow** (`maly`) es un reproductor de música local para terminal, en Go.
El branding visible es "Malody Mallow", pero el comando, el módulo Go, las rutas
XDG y el socket se llaman `maly` **a propósito** — no "corregir" eso.

El proyecto está en español: comentarios, mensajes de commit y documentación se
escriben en español. Todo texto visible para el usuario sale de `internal/i18n`
(tabla clave→[en, es]); nunca hardcodear cadenas en un solo idioma.

Versión actual: la const en `internal/version/version.go` (+ badge del README, que
se actualizan juntos en cada bump). Los git tags empiezan en v1.0.0; cada
release nueva lleva bump + tag anotado. La meta sigue siendo código limpio y
entendible, no acumular features.

## Comandos de desarrollo

```sh
go build -o maly ./cmd/maly   # ojo: `go build ./...` NO regenera ./maly
go test ./...                 # daemon/player usan mpv real; hacen t.Skip sin mpv
go test -race ./internal/library/ -run TestScanConcurrentSearch
go vet ./...
```

El binario que usa el dueño del repo es `~/.local/bin/maly` (copia manual tras
compilar, no symlink). Tras un cambio, recordarle recompilar/copiar y reiniciar el
servicio (la TUI avisa si el demonio corre un binario viejo).

## Arquitectura

Demonio + clientes sobre un socket Unix con JSON por línea
(`$XDG_RUNTIME_DIR/maly/maly.sock`). El demonio posee mpv (IPC JSON por otro
socket), la cola y la biblioteca; CLI y TUI son clientes. Si no hay demonio, la
TUI lo **embebe** en su proceso (`cmd/maly/tui.go`) y muere con ella.

- `cmd/maly` — CLI. `commands.go` tiene la **tabla de comandos**: fuente única de
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
- `internal/ipc` — protocolo (Request/Response/Status/TrackInfo), cliente, y
  `display.go` con los helpers de presentación compartidos (`TrackInfo.String`,
  `FmtTime`, `OnOff`) — no re-armar "Artista — Título" a mano.
- `internal/daemon` — el ARRANQUE (`New`) va en un orden que no es negociable:
  `EnsureRuntimeDir` → `ipc.Ping` (solo por compatibilidad con un demonio
  anterior, que no toma el lock: sin esto le robaríamos el socket al actualizar
  el binario sin reiniciar) → **`acquireLock`** (`lock.go`, flock no bloqueante
  sobre `maly.lock`) → borrar socket + `Listen` → `library.Open` →
  `player.Start` → sesión → MPRIS. La identidad se reclama con flock y no con
  la heurística vieja ("si el socket no contesta, está huérfano"), que tenía dos
  agujeros: dos demonios arrancando a la vez podían borrarse el socket el uno
  al otro, y un demonio ocupado arrancando (esperando hasta 5 s a mpv) tampoco
  contesta al ping aunque esté vivo. Solo CON el lock en la mano son seguras las
  dos operaciones destructivas del arranque: borrar el socket viejo y reapear el
  mpv viejo. El `*os.File` del lock se retiene en el struct (el lock pertenece
  al descriptor abierto) y se cierra el ÚLTIMO en `doClose`; el archivo no se
  borra nunca (borrar un lockfile es una carrera clásica). En `doClose`, el
  socket se borra ANTES de cerrar el listener: al revés, otro demonio podría
  bindear entre medias y le borraríamos el suyo.
  `serve → handle → dispatch` (dispatch bajo `d.mu`; `handle`
  refleja mutadores a MPRIS/suscriptores y realinea la ventana gapless).
  play/add/playnow resuelven sus pistas (directorios, tags, rutas) ANTES de
  tomar `d.mu` — un `add <carpeta-grande>` bajo el lock congelaba
  status/TUI/MPRIS, la misma lección que sacó a scan del mutex. `seek` es la
  tercera excepción por lo mismo (cerró B11): `player.seek` reintenta con
  250 ms de sueño y cada intento espera hasta 5 s a mpv, y bajo el lock eso
  además apilaba goroutines de `notify`. `d.seek` solo parsea y habla con el
  player (mutex propio), así que no toca estado del demonio; a cambio un
  seek concurrente con el next de otro cliente puede caer en la pista nueva
  (daño menor, aceptado como en las otras dos excepciones).
  `serve` intercepta `subscribe` y `shutdown` ANTES de `handle`: `shutdown`
  (op de `maly kill`) responde primero y luego llama `d.Close()` — dentro de
  `dispatch` deadlockearía con `d.mu`; `Close` es idempotente (`closeOnce`).
  `learnDuration` aprende la duración desde mpv: muta la cola en memoria
  bajo `d.mu` pero hace su `SetDuration` FUERA (era la única escritura a
  SQLite bajo el mutex, y se dispara en cada cambio de pista).
  `advance(reason, chained, gen)` es la política de avance y salto de pistas
  irreproducibles (guarda `errStreak`, silencio deliberado `stopped`). Tiene
  DOS guardas contra una promesa obsoleta y no se solapan: `gen !=
  d.pl.LoadGen()` (primer chequeo bajo `d.mu`) detecta que un cliente RECARGÓ
  mpv entre `resolveEnd` y este punto (jump, play, next, stop), y `chained ==
  PeekNext().Path` detecta que una mutación cambió la promesa SIN recargar
  (move, shuffle, remove). La de ruta sola NO alcanza porque compara contra la
  COLA y no contra lo que mpv reproduce: un `jump` al índice que ya era el
  actual deja `PeekNext` idéntico y matchea por coincidencia — ver la entrada
  del roadmap sobre la carrera de la promesa obsoleta.
  `scan` corre SIN `d.mu` (guarda `scanning` atómica) y sube `libGen` (la
  generación de biblioteca que `statusLocked` adjunta como `Status.LibGen`)
  solo si algo cambió, despertando a los suscriptores; los clientes recargan
  su copia al verla cambiar (la TUI en `applyStatus`). Por eso `maly scan`
  CLI escanea VÍA IPC si el demonio responde (rutas relativas absolutizadas
  antes de mandarlas: el demonio tiene otro cwd) y directo a la DB si no —
  `maly get` reutiliza ese mismo camino. El scan tiene una SEGUNDA FASE
  (duraciones con ffprobe, gated por `[scan_durations]` del config y por
  `probe.Available()`): también fuera de `d.mu` y bajo la misma atómica
  `scanning`, publica su avance en `Status.ScanTotal` (>0 marca la fase; la
  de indexado no conoce su total por adelantado, por eso el número solo ya
  distingue y no hace falta un campo de fase), sube `libGen` aunque no haya
  altas/bajas si aprendió algo, y termina con `refreshQueueDurations` — la
  cola en memoria hay que refrescarla A MANO porque `learnDuration` compara
  contra ella y no contra la DB (las lecturas van fuera del lock, el
  emparejamiento por ruta). Sesión en
  `session.go` (JSON atómico en XDG_DATA_HOME, guardado cada 15 s si dirty y en
  Close; restaura la pista actual EN PAUSA).
- `internal/player` — wrapper de mpv. Gapless: `SetNext` mantiene una ventana de
  dos entradas con `playlist-clear + append` (NUNCA podar por índices: van
  rezagados tras end-file). Un end-file queda `pendingEnd` y se resuelve con el
  evento siguiente (start-file = encadenó, idle = no había nada); `loadGen`
  descarta desenlaces pisados por cargas propias. Callbacks (`onEnd`,
  `onChange`) SIEMPRE async con `go` — en línea deadlockean readLoop. Y
  justamente por ese `go`, `loadGen` se valida en DOS puntos: `resolveEnd`
  cubre la ventana `[end-file → start-file/idle]`, pero el callback sale sin
  ningún lock sostenido, así que `resolveEnd` DEVUELVE la generación y el
  consumidor la revalida contra `LoadGen()` antes de actuar — esa segunda
  ventana, `[resolveEnd → el consumidor toma su mutex]`, es la que dejaba
  perder una pista entera (roadmap: carrera de la promesa obsoleta).
  `LoadGen()` es para eso; `LoadCount()`, en cambio, es diagnóstico de gapless.
- `internal/queue` — cola con shuffle/repeat. El shuffle es por PERMUTACIÓN
  (`order`/`pos`; `staged` guarda el ciclo siguiente en el wrap de repeat
  all): nada se repite hasta agotar el ciclo, y sin repeat all el ciclo
  agotado TERMINA (paridad con el secuencial). `Shuffle` se cambia SOLO vía
  `SetShuffle` (regenera/suelta order); `Repeat` sigue siendo escritura
  directa + `Invalidate`. `PeekNext()` promete el avance natural (la promesa
  que SetNext anexa); los mutadores mantienen la permutación con cirugía
  incremental (Add entra al tramo no sonado, Move REMAPEA — la promesa sigue
  a la pista movida —, JumpTo recoloca como siguiente y consume) y `Prev`
  camina order hacia atrás (ya no hay history que las mutaciones borren).
- `internal/library` — SQLite (modernc, sin CGo, `SetMaxOpenConns(1)`, WAL).
  Búsqueda por columna `search_text` (minúsculas sin diacríticos vía `Fold`,
  que usa un `sync.Pool` porque los transformers tienen estado). Scan por LOTES
  de 500 (`flush` = Begin→N Exec→Commit); NUNCA una transacción única: fija la
  conexión y bloquearía Search/ByPath todo el escaneo. La columna `duration`
  se aprende por DOS vías y el upsert del scan nunca la toca: perezosa desde
  mpv al reproducir (`learnDuration` en el demonio) y masiva con ffprobe
  (`FillDurations`, fase 2 del scan). `FillDurations` MATERIALIZA los
  candidatos (`duration <= 0`, filtrados con `underRoot`) y CIERRA el `rows`
  antes de probar ninguno: con `SetMaxOpenConns(1)`, llamar a ffprobe dentro
  del bucle de filas retendría la única conexión durante todo el relleno —
  peor que la transacción larga que los lotes evitan; lo cuida
  `TestFillDurationsConcurrentSearch`. Las sondas van en PARALELO con un
  pool acotado (`fillWorkers` = 4; en serie eran ~28 ms/archivo con el
  resto de núcleos parados — medido 28,5 s → 7,7 s por 1.000 pistas. No se
  escala a NumCPU: el relleno corre de fondo mientras suena música). Los
  workers SOLO sondean; DB, lotes, contadores y progress quedan en la
  goroutine de FillDurations, así que el paralelismo no toca la única
  conexión SQLite, a cambio de que el prober debe tolerar llamadas
  concurrentes (un exec por llamada, como `probe.Duration`, lo es). Lo
  encoda `TestFillDurationsProbesInParallel`, verificado en ambas
  direcciones (con `fillWorkers = 1` falla). Escribe en lotes de
  `fillBatchSize` (50, no 500: cada elemento cuesta un ffprobe) y lo que
  falla queda en 0 para que el próximo scan reintente (nada de centinelas:
  todos los consumidores prueban `> 0`). `IsAudio` es el filtro único de
  extensiones. La purga de `Scan` tiene una GUARDA: si el walk no vio ni un
  archivo de audio y `countUnderRoot` dice que la base sí tiene pistas bajo
  esa raíz, no borra nada y devuelve `*ScanEmpty` (un root vacío es casi
  siempre un montaje ausente, y el `ON DELETE CASCADE` de `playlist_tracks`
  vaciaría además TODAS las playlists) — va DESPUÉS del walk porque un
  directorio vacío es indistinguible de uno montado hasta recorrerlo, y
  cuenta con el mismo `underRoot` que usa la purga; ver la 1.16.4.
- `internal/probe` — ffprobe para las duraciones, en la línea de "coordinar
  herramientas" de `internal/getter`. A diferencia de `getter.Tools`, la
  ausencia NO es error: `Available()` falso = la fase se salta en silencio.
  La ruta va tras `-i` (un archivo que empiece con `-` sería flag) y cada
  consulta lleva timeout (un montaje de red caído colgaría el scan entero).
  `library` no lo importa: el prober se INYECTA en `FillDurations`, lo que
  además permite testear sin ffprobe ni audio real.
- `internal/mpris` — MPRIS2 (godbus). `props.go` es una implementación PROPIA de
  org.freedesktop.DBus.Properties porque godbus/prop tiene una data race con
  propiedades mapa y nunca borra claves — no volver a prop. Los métodos D-Bus
  despachan `ctrl.Do` en goroutine (en línea deadlockea vía SetMust).
  `metadataOf` es pura; el wrapper `Service.metadata` añade `artUrl` (carátula
  embebida → cache SHA-1 en runtime dir, `art.go`; la extracción vive en
  `internal/media`). El cache está ACOTADO a `maxArtBytes` (32 MB) con evicción
  FIFO: el runtime dir es tmpfs, o sea RAM compartida con todo el escritorio, y
  antes solo se vaciaba en `close()`, que un SIGKILL o un SIGHUP nunca ejecutan.
  Nunca se evicta la entrada más reciente (la de la pista que suena, cuya URL
  los clientes acaban de recibir), al evictar hay que purgar también las
  entradas del `memo` que apuntaban al archivo (es muchos-a-uno: las pistas de
  un álbum comparten carátula), y `newArtCache` empieza vaciando el directorio
  por si la sesión anterior murió sin limpiarlo.
- `internal/media` — extracción compartida de lo embebido en las pistas:
  `ReadEmbedded` (carátula + letras USLT en una pasada de dhowden; OJO:
  ffmpeg escribe `-metadata lyrics=` como TXXX, no como USLT real — dhowden
  no lo ve), `DecodeImage`/`ScaleBox` (stdlib, box average) y `ParseLRC`/
  `LyricsFor` (sidecar `.lrc` con prioridad sobre las embebidas; `At < 0` =
  sin sincronía). Lo consumen mpris (artUrl) y la capa ctrl+t de la TUI.
- `internal/safetext` — `Clean` descarta los caracteres de control (C0, DEL y
  C1) del texto que maly NO controla. Paquete hoja propio y no una función de
  library porque también lo usan media e ipc, y ninguno importa library
  (arrastraría SQLite hasta mpris). Es un requisito de seguridad, no cosmética:
  el recorte de la TUI (`reflow/truncate`) es ANSI-aware y por tanto CONSERVA
  los escapes, así que un tag con `ESC ]0;…BEL` cambia el título de la ventana
  y con OSC 52 escribe el portapapeles — basta con indexar un mp3 ajeno.
  Filtra RUNAS, no bytes: quitar solo ESC dejaría pasar el CSI/OSC de 8 bits
  (U+009B/U+009D). Descarta el carácter, no la secuencia entera (`ESC[31m` →
  `[31m`): inelidible por construcción, y el intento queda visible. Se rechazó
  `charmbracelet/x/ansi.Strip` para no delegar una propiedad de seguridad en
  una librería externa.
- `internal/tui` — Bubble Tea. Recibe estado por **suscripción push**
  (`subscribe`; fallback a polling de 500 ms con reintento). **Todo el
  reparto de la pantalla vive en `computeLayout` (`layout.go`), función
  PURA** de (ancho, alto, `layoutOpts{viz, banner}`) — no lee el Model, y
  eso es innegociable: repartida dentro de `View()`, la aritmética de tres
  columnas + franja del viz + banner es justo la clase de código que se
  rompe una celda a la vez y solo en ciertos tamaños, imposible de cazar a
  ojo contra un terminal. La tabla de tests barre 13 anchos × 10 altos × 6
  combinaciones de opciones y exige que los anchos sumen exactamente `w` y
  que `bannerH + topH + vizH + nowH + 1 == h`. Tres modos por ancho
  (`layoutFull` ≥120, `layoutTwoCol` ≥90, `layoutSingle` por debajo — ahí se
  dibuja SOLO el panel enfocado y `tab` cicla), y un orden de sacrificios
  por altura que también es decisión y no accidente: cae primero la franja
  del viz (decoración), después el banner, y la fila de paneles solo cede en
  el último extremo. Anchos: biblioteca FIJA (26-34; un porcentaje dejaba
  dos tercios de espacio muerto para nombres de artista de 10-25 caracteres)
  y "Ahora suena" fija (28-32, la manda la carátula); la COLA es la elástica
  porque es la de contenido largo. La tercera columna se parte a su vez en
  `npH`/`lyrH` (carátula+ficha arriba, letras abajo), con `lyrH` topeado a
  `2*maxLyricDistance+3`: más allá de esa distancia las líneas ya salen en
  blanco (corte de la 1.13.1), así que estirar el panel solo sumaría filas
  vacías y ese sobrante vuelve a la carátula. La barra de reproducción
  (`nowPanel`) es horizontal y de ancho completo en los TRES modos (decisión
  del dueño: en 26 celdas de columna el progreso no se lee), y el
  visualizador es una FRANJA sin panel propio. `progress.go` tiene la barra
  como funciones puras: relleno con gradiente agrupado por pasos, cabeza con
  octavos horizontales (`▏▎▍▌▋▊▉` — NO los del viz, que llenan por altura y
  no parten una celda a lo ancho), estela `▓▒` y pista `░`; sus guardas se
  escriben `!(dur > 0)` y no `dur <= 0` porque NaN es false en toda
  comparación. `display.go` limpia los títulos SOLO para mostrarlos (artista
  repetido + sufijos de yt-dlp): un grupo entre paréntesis se descarta solo
  si TODAS sus palabras están en `noiseWords`, así "(Remix)", "(feat. …)" o
  "(2016-2017)" sobreviven; no toca la DB ni `ipc`/`library`, que es lo que
  mantiene a `maly search` encontrando por el título original, y por eso
  tampoco vive en `internal/safetext` (que es una frontera de SEGURIDAD, no
  cosmética). Paneles biblioteca/
  cola + consola ctrl+p (tabla propia de comandos en `console.go`, con paridad
  CLI completa: `playlist` en `console_playlist.go`, `get` vía
  `tea.ExecProcess` + `internal/getter` compartido con la CLI, `controls`
  aplica el preset en vivo recargando `m.keys`) + picker
  fuzzy genérico (`picker.go`, usado por ctrl+o canciones, ctrl+l playlists y
  `maly select`; sus knobs `noFilter`/`emptyText`, con valor cero =
  comportamiento de siempre, existen para el buscador de descargas) +
  buscador de descargas ctrl+g (`get.go`: picker sobre resultados REMOTOS de
  yt-dlp, de dos fases porque cada consulta cuesta ~1 s de red — elige una
  URL y se la pasa a `startGet`, que es el punto único de descarga de la TUI
  y NO vive acá sino en `console.go`, compartido con la consola; `get_pick.go`
  es su gemelo suelto para `maly get pick`, con el patrón de `RunSelect` pero
  SIN exigir demonio). Los modales tapan el footer: los flashes no se ven con un
  modal abierto (el panel de playlists los dibuja bajo el modal por eso).
  El árbol de la biblioteca (`tree.go`) incluye las playlists como raíces
  tras los artistas (`playlistNode`, pistas hijas directas numeradas por
  posición); la indentación y la búsqueda de padre usan el campo `depth`,
  no el kind. Toda mutación de playlists en la TUI (plActMsg, y en la
  consola `conMsg.reload`) recarga el árbol local Y manda `notifyRefresh`
  (op IPC `refresh`, best-effort: el demonio sube `libGen` y las DEMÁS
  TUIs recargan por el push; la CLI hace lo mismo tras sus 5 mutadores de
  playlist). La doble recarga de la TUI que mutó — local + push propio —
  es redundancia aceptada. El `case libraryMsg` es el PUNTO ÚNICO de
  refresco: reconstruye el árbol y realimenta los pickers abiertos (ctrl+l
  con `plItems` sobre las listas que la propia recarga ya trajo — nada de
  reconsultar la DB; ctrl+o con `songItems`). Como `buildTree` crea nodos
  nuevos, el árbol se guarda y repone con `snapshot`/`restore` (expansión,
  cursor por clave de nodo, filtro y scroll) o cada scan lo colapsaría y
  saltaría al tope; y los pickers usan `setItemsKeeping`, que conserva la
  selección POR VALOR — con el índice pelado, algo que desaparezca más
  arriba corre la lista bajo los dedos y ctrl+x borra otra playlist. La capa "Ahora suena" (ctrl+t,
  `nowplaying.go`) es una vista fullscreen con carátula (interfaz
  `coverRenderer`, cada renderer escala a su densidad: half-blocks ANSI en
  `artrender.go`, y en kitty el protocolo gráfico vía **Unicode
  placeholders** en `artkitty.go` — la imagen vive en celdas U+10EEEE con
  el id en la tinta y fila/columna en diacríticos de la tabla oficial, así
  el diff de bubbletea, los modales y el cierre funcionan solos; la
  transmisión t=d va pegada a la fila 0 del render cacheado → una vez por
  pista, no por frame; `q=2` OBLIGATORIO o kitty contesta por stdin;
  detección por TERM/KITTY_WINDOW_ID y bajo tmux cae a half-blocks),
  letras (resaltado sincronizado por `Status.Position` si hay `.lrc`) y la
  franja del viz (`vizLines` compartido con `vizPanel`); carga carátula y
  letras SIEMPRE en goroutine (`loadNowMeta`, cache por pista `npTrack` +
  render invalidado en resize), y `applyStatus` relanza la carga al cambiar
  la pista. `playbackKey` centraliza las teclas de reproducción compartidas
  entre la vista principal y la capa. Los RELOJES de animación se
  autocancelan para no gastar CPU en reposo (medido: 3,5 % → 0,7 % de un
  núcleo): la onda del banner solo corre visible Y con música
  (`armLogoTick`; congelada en pausa a propósito), el tick del viz muere
  con el viz apagado (`armVizTick`) y con las barras decaídas respira a
  500 ms en vez de 60 — NO se mata del todo: el ring del viz captura el
  audio del sistema, no solo el de maly, y debe reaccionar si otra app
  suena. Los rearmes viven en `applyStatus` (al instante) con red de
  seguridad en el `case tickMsg`; ambos guardados por
  `vizTicking`/`logoTicking` para no duplicar relojes. El comando `logo` de la consola aplica
  el gradiente del banner en vivo y lo persiste (`SaveThemeLogo` → `saveKey`,
  que edita claves dentro de secciones TOML sin tocar el resto). El **arte
  ASCII** del banner se reemplaza con `logo.txt` junto al config
  (`loadLogoArt` en `config.Load` → `Theme.LogoArt`, `toml:"-"`); a
  propósito NO es clave TOML: un string multilínea rompería el parser por
  líneas de `saveKey` y el escapado del `\` de figlet. La altura del panel
  es dinámica (`panelH`/`minRows` del `logoModel`; el arte se recorta a
  `maxLogoArt` líneas en config).
- `internal/i18n` — `T/Tf` (idioma global) y `TL/TLf` (por petición: el cliente
  manda `Request.Lang` y el demonio responde en ese idioma). `TestTableIntegrity`
  valida en/es al agregar claves.
- `internal/update` — chequeo de releases fiel a la filosofía "coordinar
  herramientas": `git ls-remote --tags` contra el repo (nada de HTTP propio),
  mayor tag semver vs `version.Version`, cache 24 h en
  `XDG_DATA_HOME/maly/update.json`. `maly update` (CLI y paleta) descarga el
  instalador con curl a un temporal y corre `sh <tmp> --update`; la TUI
  chequea en `Init` (gated por `update_check` del config) y avisa en el pie
  (`updAvail`, prioridad tras `verMismatch`).

Decisiones transversales:
- **maly NUNCA abre una conexión a internet por su cuenta.** Verificado sobre
  el árbol entero: ningún paquete importa `net/http`, y TODOS los `net.Dial`
  /`net.Listen` son sockets `unix` (IPC del demonio y de mpv). Los dos
  `net/url` de mpris solo arman rutas `file://` para D-Bus. Lo que sale a la
  red son PROCESOS externos y solo tres: **yt-dlp** —única frontera para
  contenido y metadatos (descargar y `getter.Search`)—, `git ls-remote` para
  el chequeo de releases y `curl` para bajar el instalador en `maly update`.
  Ni siquiera `internal/update` habla HTTP: pudiendo usar la API de GitHub,
  usa `git ls-remote`.
  Es una decisión del dueño, tomada explícitamente al descartar las
  miniaturas del buscador (2026-08-16), y la razón por la que la regla vale
  más que cada caso suelto: en cuanto maly baje UNA imagen por su cuenta, deja
  de ser un reproductor que coordina herramientas y pasa a ser un cliente de
  YouTube — con su gestión de timeouts, reintentos, caché, TLS y user-agent, y
  con una superficie de red propia que auditar. Cualquier idea que necesite
  traerse un recurso de la red se resuelve pidiéndoselo a yt-dlp o no se hace.
- **El demonio y sus hijos mueren juntos.** SIGHUP se maneja explícitamente en
  `runDaemon` y en `tui.Run` (donde llama a `p.Quit()` para que bubbletea
  restaure el terminal): nadie lo hacía —bubbletea solo registra SIGINT y
  SIGTERM—, así que cerrar la ventana del terminal mataba el proceso sin
  ejecutar un solo defer, dejando mpv y pw-record huérfanos y la sesión sin su
  guardado final. Para lo que ninguna señal puede cubrir (SIGKILL, OOM,
  pánico), `player.Start` REAPEA por IPC al mpv que siguiera en el socket antes
  de lanzar el suyo. NO se usa `Pdeathsig`: Go documenta que la señal se envía
  al morir el HILO creador, no el proceso (go.dev/issue/27505), así que sin una
  goroutine permanente con `LockOSThread` mataría mpv a mitad de canción.
- **Ningún carácter de control llega nunca al terminal ni al bus D-Bus.** Se
  sanea con `safetext.Clean` en DOS fronteras, y hacen falta las dos: en la
  INGESTA (`library.ReadTags`, `media.ParseLRC` — Clean ANTES de TrimSpace, que
  descartar controles deja espacios expuestos) y en la SALIDA de la biblioteca
  (`library.scanTrack`, punto único por el que pasan Search/All/Get/ByPath/
  PlaylistTracks). La de salida NO es redundante: `Scan` salta los archivos
  cuyo mtime no cambió, así que ReadTags jamás vuelve a tocar una fila ya
  indexada, y CLI y TUI leen la biblioteca directo de SQLite sin pasar por el
  demonio. También se sanean `ScanResult.Errors` y los `skipped` de `ImportM3U`
  (arrastran nombres de archivo, que son texto ajeno) y, en `ipc.Do`/`Next`,
  `Response.Msg`/`Error`. `Track.Path` NO se sanea NUNCA: tiene que seguir
  abriendo el archivo.
- Ningún valor no finito llega a mpv: `NaN` sobrevive a TODA comparación
  (`NaN < 0` y `NaN > 100` son ambos false), así que se colaba por las
  validaciones de rango de `parseAdjust` y `d.seek` hasta `json.Marshal`, que
  lo rechaza — y con aquel error descartado el comando se perdía y costaba 5 s
  de timeout con `d.mu` tomado (un `maly vol NaN` congelaba el demonio entero).
  Lo cortan `finite()` en daemon y, como última barrera, `player.command` y
  `SetVolume`.
- El demonio adjunta `Response.Version` en toda respuesta; CLI y TUI avisan si
  difiere del binario.
- `config.Load()` mezcla teclas: defaults ← preset (`controls`) ← `[keys]` del
  usuario, vía un defer con retorno con nombre — mantener ese orden si se toca.
  `ScanTarget` resuelve el directorio a escanear (query explícita o music_dir
  con origen para mensajes de error) y `ScanNoExistErr` forma el mensaje de
  "esa ruta no existe": vive ahí porque desde la 1.16.4 lo produce el CLIENTE
  —el demonio recibe la ruta ya resuelta y ya no sabe de dónde salió— y así
  los dos espejos, CLI y consola, comparten un solo punto. Una clave booleana que deba venir
  ACTIVA por defecto se puebla en `Default()` (`update_check`,
  `scan_durations`): `toml.Decode` corre sobre el struct ya inicializado, así
  que un config viejo que no la menciona conserva el default. El zero-value
  solo sirve para las que nacen apagadas (`[ytdlp]`). El template únicamente
  se escribe cuando el config NO existe: una clave nueva jamás aparece en
  configs existentes y tiene que funcionar sin tocarlos.
- bubbletea fusiona teclas rápidas: dos `g` llegan como UN KeyMsg `"gg"` — los
  paneles manejan ambos casos.

## Cómo probar en vivo (trampas conocidas)

- Sandbox: `XDG_CONFIG_HOME/XDG_DATA_HOME/XDG_RUNTIME_DIR` apuntando a un dir de
  prueba. `XDG_RUNTIME_DIR` debe ser CORTO (p. ej. `/tmp/claude-1000/mt`): el
  path del socket de mpv revienta el límite (~108 chars) de sockets Unix.
  Poner `ao=null` en `$XDG_CONFIG_HOME/mpv/mpv.conf` del sandbox.
- TUI: probar bajo tmux (`new-session -d`, `send-keys`, `capture-pane -p`);
  bajo `script -qec` el init espera ~5 s por OSC 11. El pane NO es fish aunque
  el shell del usuario lo sea: usar `env VAR=... cmd`, no `set -x`.
- Matar procesos de prueba SOLO por PID exacto (`pgrep -a -x maly`) y el mpv por
  su socket (`pkill -f "input-ipc-server=<runtime>/maly/mpv.sock"`). NUNCA
  `pkill -f` con cadenas que aparezcan en la propia línea de comandos del shell.
  El dueño corre `mpvpaper` permanente (parece mpv en pgrep).
- La DB real está en WAL: copiarla requiere los 3 archivos (`library.db`,
  `-wal`, `-shm`).
- Los pushes del demonio son FOTOS de estado, no eventos: los tests deben
  pollear hasta el estado final, nunca leer una sola vez.
- mpv con `--no-terminal` es totalmente mudo; para diagnosticar una muerte
  temprana se usa `--input-terminal=no` y se captura **stdout** (mpv escribe ahí).

## Roadmap

v1.0.2 publicada (v1.0.0 fue el primer git tag; se brincó la 0.7.0 a
propósito). La 1.0.1 cerró la revisión de seguridad (`EnsureRuntimeDir`,
`safeExt` en carátulas, `Ping` a 2 s, purga sin falsos `..`, `LIKE …
ESCAPE`, `AddToPlaylist` transaccional). La 1.0.2 trajo el **instalador
interactivo** por pantallas (menú instalar/actualizar/desinstalar, ámbito,
checklist de dependencias con yt-dlp+ffmpeg y visualizador opcionales;
en Debian/Ubuntu yt-dlp va vía pipx porque el del repo es de 2024 y no
baja de YouTube; flags --install/--update/--uninstall/--system) y cerró
los hallazgos menores diferidos: checksum SHA-256 del Go de go.dev (el
`.sha256` plano vive en dl.google.com), permisos 0600/0700 en
config/sesión/DB, `p.pending` sin fugas en timeout, `playlist export` sin
clobber (el tty se detecta con el ioctl real: /dev/null también es char
device) y EADDRINUSE → ErrAlreadyRunning. La distribución es vía
`mallow-install.sh`. Un PKGBUILD para AUR se descartó en su momento;
publicado el 2026-07-29 (paquete `maly`) — ver la entrada de la 1.10.2.

La **1.1.0** (2026-07-17) trajo la capa **"Ahora suena"** (ctrl+t: carátula
half-blocks + letras USLT/.lrc + viz, paquete `internal/media`),
**`maly kill`** (op IPC `shutdown`, CLI y consola) y los **colores del logo
configurables** (`[theme] logo` + comando `logo` en la paleta). La 1.0.3
fue solo un bump de prueba del flujo de update.

La **1.1.5** (2026-07-17) es el release de una **auditoría completa** del
proyecto (se saltó 1.1.1–1.1.4 a propósito: tanda grande de fixes guiada a
seguridad/robustez). Sus fixes: resolución de pistas fuera de `d.mu` (ver
arriba), `maly update` instala el TAG anunciado (`--ref=` del instalador; sin
él se compilaba el HEAD de main), `RemoveAt` distingue índice inválido (el
demonio ya no responde OK a un remove fuera de rango), sorteo de shuffle sin
sesgo con `Index -1`, tope `maxDecodePixels` en carátulas (bomba de
descompresión), el scan vía IPC reporta cuántos archivos fallaron (detalle al
stderr del demonio), TOCTOU del socket cerrado (el remove del huérfano va
tras `EADDRINUSE` + ping fallido), `Search` sin `LIMIT 500` (capaba
play/add/playlist add en silencio), `saveKey` exige `=` tras la clave (no
prefijo), `seek` acepta `hh:mm:ss`, el instalador avisa reiniciar el servicio
si el socket existe, buffer de 1 MB en ParseLRC y cache del folded de la cola
en la TUI (`queueFolded`). El único hallazgo diferido de esa tanda (B11: el
retry de `player.seek` durmiendo 250 ms bajo `d.mu`) se cerró en la 1.5.0.

Trampas que dejaron estos ciclos:

- Tests de `internal/viz`: construyen el `Viz` a mano (`newTestViz`) porque
  `New()` arranca un pw-record/parec REAL en la máquina de desarrollo.
- El instalador sondea /dev/tty EN SUBSHELL: `:` es un special builtin y
  POSIX manda que su redirección fallida termine el shell entero — sin
  subshell, el modo no interactivo moría mudo.
- Probar el instalador bajo tmux con HOME alterno: pasar GOMODCACHE/GOCACHE
  reales al `go build` o deja un mod-cache de solo lectura en el sandbox
  (`chmod -R u+w` antes de borrar).

La **1.2.0** (2026-07-17) empaqueta las dos features post-auditoría: la
**carátula como imagen real en kitty** en "Ahora suena" (renderer
`artkitty.go`, detalles en la sección de la TUI; la trampa que costó un
ciclo: el placeholder del protocolo es U+10EEEE, no U+10FFFD, y con q=2
kitty calla el mismatch — la referencia byte a byte es `kitten icat
--unicode-placeholder`) y el **rediseño visual del instalador** (elegido
por el dueño vía preguntas): wizard con pantalla limpia por paso y el banner
MALODY en degradado Kitasan (fallback a la caja sobria si <58 columnas o
sin stty), menús y checklist navegables con ↑↓/jk + espacio leyendo el
tty crudo (`stty -icanon` + `dd`; `RAWOK` sondea y sin él cae al modo
numérico de siempre), confirm de una tecla, y spinner braille con
tiempo (`hb_start`/`hb_done`; el sleep fraccional se sondea — sin él
gira a 1 fps). Trampas: `raw_off` va en el trap (un Ctrl-C en pleno
modo crudo dejaría el terminal sin eco ni cursor); el trap ahora se
arma ANTES de las pantallas con `TMP=''` (rm -rf de vacío no toca
nada); tras un `ESC` se leen 2 bytes con `min 0 time 2` para
distinguir flecha de ESC suelto; y `run_phase` separa el wizard (clear
por pantalla) de la fase de ejecución (log corrido: la salida de
pacman/go debe quedar en el scrollback).

La **1.2.1** (2026-07-18) agregó `[ytdlp] cookies_from_browser` al config:
passthrough tal cual a `--cookies-from-browser` de yt-dlp para videos que
piden cuenta (restricción de edad). Sin validación ni parsing de errores a
propósito (yt-dlp es el dueño de ambos); vacío = sin flag, configs viejos
sin la sección cargan igual. Probado con el yt-dlp falso de `get_test.go`
y en vivo con Zen Browser vía ruta de perfil.

La **1.3.0** (2026-07-18) empaqueta `maly move`, el progreso de scan y el
ancho dinámico de la ayuda `?` (la caja se ajusta a la fila más larga; panel
rellena pero no recorta). Se implementó **`maly move <de> <a>`** + reorden en la TUI
(`queue.Move` sigue a `Index` e invalida la promesa; campo `To` en
`ipc.Request`; la ventana gapless se realinea sola vía el `default` de
`handle`). En la TUI son las teclas `move_up`/`move_down` (K/J, solo con el
filtro vacío: con la cola filtrada el reorden sería ambiguo); las pulsaciones
rápidas fusionadas se cuentan con `keyRepeats` y viajan como UN move de n
posiciones. Paridad completa: consola ctrl+p, help `?`, completions de ambos
argumentos (`completeMove`/`queuePositions`).

También en la 1.3.0: **progreso de scan**. `Library.Scan` acepta un callback
`progress(seen)` (nil = mudo; cuenta archivos de audio vistos, incluidos los
saltados por mtime — el total no se conoce por adelantado, es contador a
propósito, no porcentaje). El demonio lo publica en `Status.Scanning/ScanSeen`
(atómica `scanSeen`) despertando suscriptores en cada callback (el dirty cap 1
+ los 250 ms mínimos del bucle del suscriptor colapsan la avalancha), y al
terminar hace `wakeSubs` SIEMPRE — tras bajar `scanning`, o el push final
diría "escaneando" — para que los clientes limpien el estado. La TUI lo pinta
en el footer; `maly scan` directo pinta `\r` en stderr (solo si es tty, ioctl
`isTTY` compartido con playlist export) y vía IPC abre una SEGUNDA conexión
suscrita mientras `Do` bloquea (los pushes de Status traen el avance; `Do` lee
una sola línea, por eso no puede ser la misma conexión).

La **1.3.1** (2026-07-18) cerró la última limitación conocida: las
mutaciones de playlists se reflejan en vivo en todos los clientes vía la op
`refresh` (detalles en la sección de la TUI).

La **1.4.0** (2026-07-19) cambió el shuffle a **permutación** (detalles en
la sección de queue): `history`/`peeked` desaparecen a favor de
`order`/`pos`/`staged`, con dos cambios de comportamiento deliberados —
shuffle + repeat off ahora TERMINA al agotar el ciclo (antes sorteaba por
siempre), y el next manual también camina la permutación. La sesión NO
persiste el orden (se regenera al restaurar; `sessionVersion` sigue en 1) y
el `canNext` de MPRIS queda optimista con el ciclo agotado (el Next
sobrante falla inofensivo, comentado en `mpris.go`). Sin cambios de
IPC/TUI/CLI: `Items` nunca se reordena.

La **1.5.0** (2026-07-20) vació la lista de candidatos con las dos últimas
piezas. **Duraciones masivas con ffprobe**: paquete nuevo `internal/probe`
y `Library.FillDurations` como segunda fase del scan (detalles en las
secciones de library/probe/daemon), con la clave `scan_durations` (default
TRUE, precedente de `update_check`: los configs viejos que no la traen la
reciben activada) y un campo nuevo `Status.ScanTotal` para el progreso. El
pago visible: el panel de cola de la TUI muestra la duración de pistas que
nunca se reprodujeron. **B11 cerrado**: el seek se resuelve fuera de `d.mu`
como tercera excepción del dispatch. El retry de `player.seek` se dejó tal
cual a propósito — afinarlo (p. ej. no dormir si mpv murió) exigiría
comparar mensajes de error que salen de i18n y cambian con el idioma del
proceso. `newTestDaemon` apaga `ScanDurations`: si no, en una máquina con
ffprobe los tests que escanean miles de dummies lanzarían un proceso por
archivo y el resultado dependería de tenerlo instalado.

La **1.5.1** (2026-07-20) cerró la última limitación menor que quedaba
anotada: el panel ctrl+l ya abierto no se enteraba de las mutaciones de
playlists de otros clientes. El arreglo vive entero en el `case libraryMsg`
de la TUI (detalles en su sección) y de paso quitó dos saltos visuales que
nadie había anotado: el árbol de Biblioteca ya no se colapsa ni salta al
tope en cada recarga, y la selección de los pickers no se corre cuando la
lista cambia por arriba. El refresco es SILENCIOSO a propósito (decisión del
dueño): el contenido correcto es el feedback, y los flashes se reservan para
las acciones propias. `plActMsg` perdió su flag `reload`: con la recarga
única ya no hacía falta.

La **1.6.0** (2026-07-21) es el release de una **segunda auditoría de
seguridad** completa, pedida por el dueño sobre la 1.5.1: 13 hallazgos,
**ninguno crítico**, de los que se cerraron los seis accionables en dos
tandas. Se saltó la 1.5.2 a propósito, como en su día la 1.1.5: es una
tanda grande guiada a seguridad, no un parche.

*Tanda 1 — validar la entrada ajena.* **Inyección ANSI/OSC desde los tags**
era el único hallazgo que cruzaba una frontera de confianza externa real:
basta con indexar un mp3 ajeno para que un título con `ESC ]0;…BEL`
secuestre el título de la ventana, y con OSC 52 escriba el portapapeles.
Agravado porque `clip()` usa `reflow/truncate`, que es ANSI-aware y por
tanto CONSERVA los escapes. Paquete nuevo `internal/safetext` y saneado en
las dos fronteras (ver Decisiones transversales). El otro: **`NaN` e `Inf`
evadían la validación** y congelaban el demonio 5 s bajo `d.mu` —medido,
con un `status` concurrente bloqueado 4,7 s—, porque `NaN` es false en toda
comparación y `json.Marshal` los rechaza con su error descartado. La
trampa que costó un ciclo: sanear en `ReadTags` + `ipc.Do` NO basta, el
PoC seguía pasando; el punto de salida bueno es `library.scanTrack`.

*Tanda 2 — ciclo de vida y recursos.* **mpv quedaba huérfano** (verificado:
dos procesos tras un SIGKILL), y al detallarlo apareció que **nadie manejaba
SIGHUP**, ni maly ni bubbletea — cerrar la ventana del terminal mataba el
proceso sin un solo defer. Además, el reorden del arranque que se había
elegido introducía por sí solo una regresión (el socket queda bindeado sin
atender varios segundos y otro maly lo tomaría por huérfano), y de ahí que
la identidad del demonio pase a reclamarse con **flock**, lo que de paso
cierra la carrera de doble arranque que la auditoría no había logrado
reproducir. Con ello, el **caché de carátulas acotado** a 32 MB y los
**permisos del directorio de datos** (0700/0600, que colgaban de un modo de
directorio que nadie comprobaba). Detalles de las tres en sus secciones.

Los tests se verificaron en AMBAS direcciones —revirtiendo el código de
producción desde HEAD y conservando los tests nuevos— y esa disciplina
pagó: dos de los tests de arranque pasaban también sin el arreglo, así que
se añadió `TestNoRoboElSocketDeUnDemonioArrancando`, el único que encoda de
verdad el invariante del lock.

La **1.6.1** (2026-07-22) cierra la auditoría: deja sus trece hallazgos a
cero, seis con código y el resto documentados con el motivo. Es un parche y
no una tanda nueva, de ahí el número.

Trae dos cosas. La primera, el **hallazgo #4 resuelto midiendo antes de
tocar**, y la medición REFUTÓ la hipótesis del informe: con 40.000 pistas un
`search` de la biblioteca entera retiene `d.mu` 96 ms, y con un scan
reescribiendo esas mismas 40.000 filas a la vez el peor caso fue 112 ms — la
contención por la única conexión SQLite añade 16 ms, no segundos. Los lotes
de 500 del scan hacen lo que promete su comentario. Por eso `search` y
`playlist_play` se quedaron DENTRO del mutex (detalles y números en la
sección de candidatos) y solo se sacó `learnDuration`, que era la única
escritura y la única que se dispara sola.

La segunda, la **tanda 4 con los menores**: #5, #8, #10, #11, #12 y #13, más
la latencia del aviso de `update`. El de más enjundia fue #8: la barra de
progreso estaba duplicada letra por letra en `nowPanel` y `npMeta` y le
faltaba la guarda inferior, así que una `Duration` diminuta desbordaba el
cociente a `+Inf` y `strings.Repeat` entraba en pánico; se reprodujo el
pánico contra el código viejo antes de arreglarlo. Quedó como
`Model.progressBar`, fuente única.

Lección de verificación que dejó esta release: al revertir producción desde
HEAD, tres de los cinco tests nuevos fallaban **solo por no compilar**
(funciones y constantes que no existen en HEAD). Eso es señal DÉBIL: no
prueba que el defecto estuviera. Para el pánico de #8 hubo que escribir un
test desechable en la copia revertida que llamara al código viejo.

La **1.6.2** (2026-07-22) cierra la mitad que quedaba del roce con el aviso
de `update`: el chequeo ocurría SOLO en `Init`, así que una TUI abierta días
no volvía a mirar nunca. `updTickCmd` lo repite cada hora y se re-arma,
respetando `update_check`. Repetir sale barato gracias al arreglo de la
1.6.1: cuando el cache ya anuncia algo más nuevo, `updateCheckCmd` resuelve
sin tocar la red. Release de una sola pieza, toda dentro de `internal/tui`.

La **1.7.0** (2026-07-22) agrega los dos comandos de diagnóstico, `maly info`
y `maly doctor`, más el avance del scan en `maly status`. Salió de una
revisión de diseño sobre cuatro propuestas, de las que se rechazaron dos y
conviene que quede escrito por qué: **`maly report`** era `info` + `doctor`
con un marco para pegar, y la salida de `info` ya es pegable porque lipgloss
se apaga fuera del tty (además, un comando cuyo propósito es producir un
volcado público roza el motivo por el que config/sesión/DB son 0600: la
biblioteca revela hábitos de escucha); y **`status --verbose`** chocaba con
que `-v` YA es alias de `version`, y con que esos datos son estado de la
instalación y no de la reproducción — la partición que mpc resolvió en su
día con `status` y `stats` por separado.

`info` lista hechos y `doctor` emite veredictos: son contratos de salida
distintos y por eso son dos comandos (docker info / brew doctor), aunque
compartan casi todos los datos. Los dos funcionan SIN demonio y sin red, que
es cuando de verdad se consultan, y ninguno reimplementa nada: tiran de
`getter.Tools`, `probe.Available`, `p.no_mpv`, `MusicDirOrigin` y
`update.Cached`. Solo `mpv` ausente es `fail` y solo `fail` cambia el código
de salida; lo que maly degrada en silencio queda en `info`, siguiendo la
línea que ya fijó `internal/probe` ("la ausencia NO es un error"). El
detector de capturador y el del bus de sesión se exportaron a sus paquetes
(`viz.CaptureBackend`, `mpris.BusAvailable`, hermanas de `probe.Available`)
para no repetir nombres de binarios ni importar godbus desde `cmd/maly`.

Tres invariantes de esta tanda, cada uno por una razón que muerde:

- **`doctor` NO toca el flock.** Un intento no bloqueante que TUVIERA éxito
  lo retendría un instante, y un `maly daemon` arrancando en esa ventana
  moriría con `ErrAlreadyRunning`. Se pregunta por el socket y punto; por eso
  el texto dice "no responde" y no "no corre" (un demonio esperando hasta 5 s
  a mpv tampoco contesta). Verificado con 40 `doctor` en paralelo contra un
  demonio arrancando.
- **`libraryStats` abre por `openLibraryIfExists`.** `library.Open` CREARÍA
  la base: un diagnóstico que la fabrica vacía y luego reporta 0 pistas se
  estaría diagnosticando a sí mismo. Mismo motivo que en las completions.
- **`checkUpdate` lee solo el cache.** Nada de `update.Latest()`: un doctor
  que se va diez segundos a `git ls-remote` deja de servir justo cuando algo
  va mal.

Detalles menores que costarían un rato redescubrir: las etiquetas de las
columnas van SIN color porque `text/tabwriter` mide en runas y los escapes
ANSI le falsean el ancho (la última celda sí puede llevarlo, que a esa no le
añade relleno); `errQuiet` en `main.go` sale con código 1 sin imprimir nada,
porque `doctor` ya escribió su informe y su resumen; y `serviceVersion` se
extrajo de `runVersion` para que `version`, `info` y `doctor` compartan una
sola copia, con timeout de 2 s como `ipc.Ping` (antes `maly version` podía
colgarse 30 s contra un demonio que acepta y no contesta). La línea de scan
en `status` se imprime SOLO con un scan en vuelo y reusa las claves de
`maly scan`: sin scan, la salida queda exactamente como siempre, que era la
condición para no romper a quien la parsea.

No se hizo, y es deliberado: salida JSON (no hay parser de flags en la CLI
—`-v`/`-l`/`-h` son COMANDOS— y el scripting de reproducción ya lo cubre
MPRIS; el API de máquina de `doctor` es su código de salida) y paridad en la
consola ctrl+p (que nunca fue espejo estricto: ya tiene `cls`, `viz`, `logo`
y `quit`, y le faltan `select` y `completions`).

Sobre 1.7.0, sin bump de versión (es un cambio solo del instalador, sin
tocar el binario), `mallow-install.sh` pasó a compilar el **último tag
estable por defecto** en vez de `main` — salió de una revisión de diseño con
auditoría de seguridad completa del script. `--main` pide explícitamente la
rama de desarrollo; `--ref=<x>` sigue con máxima prioridad e ignora ambos
(así `maly update`, que siempre lo pasó explícito, no cambió). El tag se
resuelve con una función `latest_tag()` nueva que filtra y compara
`git ls-remote --tags --refs` vía `awk`, no en sh puro: ahí "0" es un valor
legítimo de versión (`v1.0.0`) y el pelado de ceros líderes que ya usa
`ytdlp_stale` para `$(( ))` se rompería con eso. Nueva pantalla de wizard
"fuente" entre ámbito y dependencias, con el mismo patrón de `menu` que las
demás. La auditoría no encontró nada crítico (el modelo de amenaza es
ejecución local, mismo UID; la brecha real es que los tags no están
firmados, y firmar el bootstrap `curl | sh` no cierra eso — la clave viajaría
del mismo GitHub que ya se está confiando), así que se cerraron los dos
hallazgos con sustancia: sin `sha256sum` NI `shasum -a 256` el tarball de Go
ya no se extrae en silencio (confirma con terminal, aborta sin ella), y el
tag resuelto se valida por patrón antes de llegar a `--branch`. Firma de
releases (se estudió minisign vs GPG) queda fuera, deliberadamente: solo
aportaría de verdad ya con el binario instalado verificando en `maly update`
con una clave embebida en compilación, no en el propio arranque — eso exige
un proceso de release nuevo y se deja como iniciativa aparte, no parte de
este cambio. Efecto colateral encontrado en la prueba en vivo: clonar un tag
anotado con `--depth=1` (antes un caso raro de `--ref`, ahora el camino por
defecto de cualquier instalación) hace que git imprima un aviso inofensivo
("… is not a commit!") que ensuciaba el spinner; el stderr del clonado ahora
va a un archivo temporal y solo se muestra si el clonado FALLA de verdad.

La **1.7.1** (2026-07-28) salió de un **análisis de rendimiento completo**
sobre la 1.7.0 (arranque, scan, búsqueda, demonio, TUI, memoria; escalas
sintéticas de 1k a 40k pistas, macro sobre procesos reales y micro con
benchmarks desechables + pprof, borrados tras medir). El veredicto general:
nada que optimizar salvo una pieza. La línea base de la auditoría #4 se
reprodujo exacta (search'' bajo `d.mu`: 96,5 ms vs los 96 documentados; con
re-scan de 40k, 114 vs 112), el pprof del Search es 52 % VDBE de modernc +
17 % syscalls (ningún código de maly en el camino caliente), `status` por
IPC responde en <0,1 ms incluso durante un re-scan completo, y la cola es
nanosegundos. La pieza: la fase 2 del scan sondeaba EN SERIE (~28 ms por
ffprobe, núcleos parados), y es el único cambio de código del release —
las sondas de `FillDurations` van ahora por un pool acotado (detalles e
invariantes en la sección de library). Quedan anotados sin acción: la TUI
consume ~1,9 % de un núcleo en reposo sin viz (3,5 % con viz; huele a un
tick redibujando con nada sonando — mirar los `tea.Tick` si algún día
molesta en portátil) y el `search` amplio vía demonio serializa MB de JSON
(solo lo sufre la consola; la TUI lee SQLite directo).

La **1.7.2** (2026-07-28) cierra el candidato del reposo de la TUI que la
1.7.1 dejó anotado: los relojes de animación se autocancelan (detalles e
invariantes en la sección de la TUI; el dueño eligió onda del banner
congelada en reposo). Reposo con viz activo: 3,5 % → 0,7 % de un núcleo;
en pausa con barras decaídas, 1,1 %; reproduciendo, sin cambios (7,1 %,
las animaciones están haciendo su trabajo). Release de una sola pieza,
toda dentro de `internal/tui`, como la 1.6.2. La verificación destapó de
paso una trampa de arnés: `maly pause` es pausa IDEMPOTENTE y el toggle
es `maly toggle` — un arnés que "reanuda" con pause verifica un congelado
que es correcto y parece un bug.

La **1.7.3** (2026-07-28) sale de un benchmark completo de la 1.7.2 (escaneo,
IPC, CPU y RAM del demonio y de la TUI, arranque; escalas de 1k a 40k). El
veredicto volvió a ser "no hay nada que optimizar" salvo UNA pieza, y esta
vez no estaba en el demonio sino en el completado del shell: `maly play
<TAB>` con palabra parcial vacía tardaba **92 ms y asignaba 39 MB** con
40.000 pistas para imprimir treinta líneas. `completeTracks` llamaba a
`Search("")` —que cae en `All()`— y recortaba EN GO, o sea que materializaba
la biblioteca entera para tirar el 99,9 %.

El arreglo es `library.SearchLimit(q, limit)`: `Search` pasa a ser
`SearchLimit(q, 0)` y el tope, cuando lo hay, va en la sentencia. Con el
`LIMIT` en SQL, SQLite resuelve el `ORDER BY` con un montículo acotado en vez
de ordenar y devolver 40.000 filas. Medido: **92 ms → 14 ms y 36 MB → 16 MB**,
con la lista de candidatos IDÉNTICA (comprobado con `diff` contra el binario
anterior en cuatro consultas, incluida una que matchea las 40.000).

Dos cosas que no son obvias y conviene no re-descubrir:

- **El tope corta FILAS, no candidatos.** `completeTracks` deduplica por
  título DESPUÉS, así que un tope pegado a `maxCandidates` dejaría el TAB
  corto en cuanto hubiera títulos repetidos (un álbum reeditado, un disco
  doble). Por eso pide 20× de margen (`completeFetch`), que sale casi gratis:
  el costo de la consulta es recorrer la tabla, no las filas devueltas —con
  40.000 pistas, 9,1 ms con `LIMIT 30` y 9,6 ms con `LIMIT 600`, frente a
  77 ms materializándolas todas. Lo encoda `TestCompleteTracksDuplicados`,
  verificado en ambas direcciones (con `completeFetch = maxCandidates` da 3
  candidatos en vez de 30).
- **`Search` y `All` siguen SIN tope**, que es justo la lección de la 1.1.5
  (un `LIMIT 500` capaba `play`/`add`/`playlist add` en silencio). El tope es
  opt-in y de un solo llamador; `TestSearchLimit` fija ambas mitades.

Otras dos mediciones de ese benchmark, sin acción: la fase 2 del scan escala
casi lineal con `fillWorkers` (1→28,4 s, 2→14,3 s, 4→7,7 s, 8→4,4 s,
16→3,2 s por 1.000 pistas) y ffprobe cuesta 28,5 ms por archivo sea de 4 KB o
de 4,2 MB —es spawn de proceso, no lectura—, así que el número extrapola
directo; y con `fillWorkers = 4` el relleno tiene 4,2 núcleos ocupados, que
en la máquina del dueño (16) es fondo cómodo pero en un portátil de 4 es la
máquina entera. La constante no depende de `NumCPU` a propósito.

La **1.8.0** (2026-07-29) sale de una auditoría técnica exhaustiva pedida por
el dueño — la primera que cubre arquitectura, código, seguridad, rendimiento,
UX, configuración, integración con Matugen, ecosistema Linux y calidad de
proyecto a la vez, entregada como informe aparte. De los hallazgos, tres
quedaron marcados prioridad alta y se atacan en esta release; el resto
(systemd empaquetado en el instalador, `maly config`, integración Matugen,
los índices SQLite muertos, cobertura de test en `doctor`/`info`, etc.) queda
documentado en ese informe para tandas futuras.

**El espejo del gapless podía mentir tras un fallo de red.** `SetNext`
(`internal/player/player.go`) solo actualizaba `nextPath`/`nextKnown` en el
camino de éxito: si `playlist-clear` tenía éxito pero el `loadfile ... append`
posterior fallaba, mpv quedaba sin promesa mientras el espejo conservaba el
valor anterior a la llamada. Se verificó el impacto real leyendo el código,
no solo confiando en la auditoría: el `case "idle"` de `handleEvent` sí
dispara la reparación de `advance` (se refuta que la reproducción quedara
colgada), pero el gapless degrada a una carga manual audible, y además el
guard de no-op de una llamada posterior con la MISMA ruta que falló corta sin
mandar ningún comando — extiende la ventana del defecto hasta el siguiente
cambio de promesa. Arreglado: el camino de error del append también
actualiza el espejo (a `""`, reflejando la verdad); el `return err` de
`playlist-clear` queda intacto a propósito, porque ahí mpv no cambió y el
espejo seguía siendo válido. `TestSetNextAppendFailureClearsMirror` lo
encoda, verificado en ambas direcciones.

**El filtro de Biblioteca pagaba el mismo costo que la Cola ya había
resuelto.** `flatten()` (`internal/tui/tree.go`) replegaba Unicode de
`t.all` entero —tags + diacríticos vía `library.Fold`— en CADA tecla del
filtro; con 40.000 pistas, cada pulsación recorría la biblioteca completa
desde cero. La Cola ya tenía el arreglo (`queueFolded`, con el comentario
"plegar Unicode por pista por frame pesaba con colas grandes"); a la
Biblioteca, que es el caso que de verdad escala, le faltaba. Arreglado con
el mismo patrón: `folded []string` en `libTree`, poblado PEREZOSAMENTE
dentro de `flatten()` —nunca en `buildTree`, que correría en cada scan
aunque nadie filtre— con la misma detección por longitud que usa
`queueFolded` para saber si quedó desalineado. Sin invalidación explícita:
`buildTree` siempre crea un `libTree` nuevo, el campo nace nil y el chequeo
de longitud lo repuebla solo.

**CI en GitHub Actions, el primero que tiene el proyecto.** Dos jobs en
`.github/workflows/ci.yml`: `test` (build + vet + test) y `race` (`-race`
solo sobre `internal/library` e `internal/mpris`, los dos paquetes con
concurrencia real de goroutines que no dependen de mpv/ffprobe —
`TestPropsConcurrent` de mpris es justo el que habría cazado la race de
`godbus/prop` que motivó el reemplazo propio de `props.go`). Sin instalar
mpv/ffprobe: 23 de los 24 tests de `internal/daemon` se auto-saltan por
`exec.LookPath` y el paquete igual reporta `ok` — verificado corriendo la
suite completa con un `PATH` realmente sin esos binarios, no solo confiando
en que `t.Skip` los cubriera. Sin `-short`: ningún test del repo lo honra,
sería un no-op.

La **1.9.0** (2026-07-29) cierra los cinco ítems de prioridad MEDIA de la
misma auditoría que dio la 1.8.0: tres de dificultad baja primero
(`maly config`, el test de migración que le faltaba a `update_check`, y los
tests de `doctor.go`/`info.go`), y los dos de dificultad media al final
(systemd empaquetado y la integración con Matugen), en ese orden por
decisión del dueño.

**`maly config`** (`cmd/maly/config_cmd.go`) muestra la configuración
EFECTIVA — defaults ← preset de controls ← `[keys]` del usuario, el merge
que hoy solo vive dentro de `resolveKeys()` y era invisible sin leer el
código. Clona el patrón de `info.go` (tabwriter, etiquetas sin color) y
reusa `config.Load()` tal cual: sin lógica de resolución nueva, solo
mostrarla. Complementa a `maly info` en vez de reemplazar su sección de
config — `info` sigue con su subconjunto, `config` muestra todo (theme,
visualizer, las ~23 teclas resueltas).

**Tests de `doctor.go`/`info.go`**: dos invariantes que CLAUDE.md documentaba
en prosa desde la 1.7.0 sin ningún test que los encodara — el patrón de
riesgo que la 1.6.1 ya nombró ("no compilar no prueba que el defecto
estuviera"). `TestCheckServiceNoDaemonNoLock` verifica que no queda ningún
`maly.lock` en el runtime dir; `TestLibraryStatsNoDB`/
`TestOpenLibraryIfExistsDoesNotCreate` que no se fabrica la base de datos.
Ambos verificados en ambas direcciones simulando la regresión.

**Unit de systemd empaquetada** (`mallow-install.sh`): vivía solo en el
README, para copiar a mano, pese a que el propio README la recomienda como
el camino preferido. Se ofrece en modo usuario si hay `systemctl`, solo
`enable` (nunca `--now`, para no pisar algo que ya esté corriendo por
`&`), y no se reofrece si ya existe. `--uninstall` la para/deshabilita/
borra sin preguntar — a diferencia de config/biblioteca, es parte de la
instalación, no dato del usuario. Verificado con `systemctl` stubeado: la
unit generada es byte a byte idéntica a la del README.

**Integración con Matugen — `maly theme sync`.** Revertida entera en la
1.10.2, código y sistema real, por decisión del dueño. Ver esa entrada
para el motivo y el detalle de qué se sacó.

La **1.10.0** (2026-07-29) cierra los siete ítems de prioridad BAJA de la
misma auditoría, de más fácil a más difícil por decisión del dueño: tres
triviales primero, luego tres de dificultad baja, y el más invasivo
(interfaz de mpris) al final.

**Índices SQLite muertos, eliminados.** `idx_tracks_artist`/
`idx_tracks_album` (`internal/library/library.go`) no los usaba ninguna
consulta (`Search` es LIKE con comodín inicial; el `ORDER BY` usa
`COLLATE NOCASE` y los índices se crearon con colación binaria) y
costaban una escritura extra por pista en cada scan. Sin sistema de
migraciones en el proyecto, se siguió el precedente ya establecido por el
`ALTER TABLE` de `duration`: `DROP INDEX IF EXISTS` sin condición en cada
`Open()`, no-op en instalaciones nuevas.

**Makefile y CHANGELOG.md**, ambos ausentes hasta ahora. El Makefile
encapsula los comandos ya documentados (`build` con `-o maly` explícito,
`vet`, `test`, `install` con el mismo `install -Dm755` que ya usa
`mallow-install.sh`). El CHANGELOG condensa cada release en un párrafo,
para quien solo quiere saber qué cambió sin leer el roadmap completo de
este archivo.

**`[visualizer] backend`** (`auto`/`pipewire`/`pulse`) fuerza `pw-record`
o `parec` en sistemas con ambos instalados. `filterCandidates(pref)` es
la función pura nueva que recorta `captureCandidates`; un valor no
reconocido (incluido `"auto"`/vacío) se comporta como antes, mismo
criterio de degradar en silencio que un preset de `controls` inválido.

**El modal de ayuda (`?`) ya no se desborda en terminales chicas.**
`helpView` pedía siempre `h = len(lines)+2` (23 en el peor caso) sin
toparlo contra `m.height` — `panel()` ya truncaba en silencio el
contenido que no entraba en `innerH`, pero sin el tope no servía de nada.
Verificado en ambas direcciones: sin el fix, `helpView()` con
`m.height=12` producía 24 líneas de verdad.

**`daemon.go` dividido en archivos por categoría** — `daemon_scan.go`,
`daemon_playback.go`, `daemon_resolve.go` — sin tocar `dispatch()` (sigue
siendo un switch plano de ~20 comandos en `daemon.go`: dividirlo en una
tabla de funciones complicaría las tres excepciones de "antes de `d.mu`"
que hoy son ifs explícitos y auditables). Extraído con un script que
particiona el archivo completo por límites de `func`/`type`/`var`/`const`
de nivel superior, verificado que el conjunto de 32 firmas de función es
idéntico antes y después.

**Interfaz `Controller` propia para mpris, desacoplada de
`ipc.Request`/`Response`.** Antes `Do(ipc.Request) ipc.Response` acoplaba
mpris al vocabulario wire completo de IPC (~20 comandos) cuando solo usa
9. La interfaz nueva es de dominio (`Next`, `SetVolume(int)`,
`SeekRel(float64)`, etc.); la conversión al formato wire que `dispatch`
ya entiende se movió al lado del demonio, que es quien lo conoce.
`Daemon.Do` se dejó intacto a propósito: lo usan ~45 sitios de
`daemon_test.go` como atajo directo a dispatch sin pasar por el socket,
y no tiene nada que ver con mpris.

La **1.10.1** (2026-07-29) agregó recarga en caliente por `SIGUSR1` para
la integración con Matugen de la 1.9.0. Revertida entera en la 1.10.2,
código y sistema real — ver esa entrada.

La **1.10.2** (2026-07-29) saca entera la integración con Matugen (`maly
theme sync` de la 1.9.0 y la recarga por `SIGUSR1` de la 1.10.1), código y
sistema real. El dueño la probó dada de alta de verdad y decidió no
quedarse con ella: "es una mezcla de ambas, son demasiadas cosas
orientadas a un desktop cuando maly debería sentirse global o para uno
mismo, ya que corre localmente" — depender de la paleta del wallpaper del
escritorio, con plantilla + `post_hook` + señal + archivo estático +
script de restauración, terminó siendo una pieza de theming de escritorio
viviendo dentro de un reproductor de música, no la coordinación puntual de
herramientas (mpv, yt-dlp, ffprobe) que es la filosofía del proyecto.

Reversión hacia adelante, sin tocar los commits ni los tags ya pusheados
de la 1.9.0/1.10.1 (son historial público en GitHub): se borró con
commits nuevos, como cualquier revert real. Del lado del código,
`cmd/maly/theme.go` (y su test) desaparecen enteros, junto con
`SaveThemeAccent`/`SaveVisualizerColors` (`internal/config`), el bloque
`theme.*` de i18n, `conTheme`/`reloadTheme`/el `case "theme"` de la
consola, y en `internal/tui/tui.go` el tipo `themeReloadMsg` con el
goroutine que escuchaba `SIGUSR1` en `Run`; el registro no-op de
`SIGUSR1` en `runDaemon` (`cmd/maly/client.go`) también se quita — sin
`theme sync`/recarga, ya no hace falta protegerse de que la señal matara
el demonio. Del lado del sistema real (`~/.config`, fuera del repo, solo
la parte de maly): la plantilla y el bloque `[templates.maly]` de
Matugen, los dos TOML de colores en `~/.config/maly/`, las dos líneas
agregadas a `apply-static-colors.sh`, y `accent`/`logo`/
`visualizer.color_low`/`color_high` del `config.toml` real vueltos a los
valores de `config.Default()` (Kitasan Glass) — las otras 9 apps del
Matugen del dueño (kitty, waybar, rofi, wlogout, hyprlock, hypr_live,
swaync, starship, fish) no se tocaron.

Sobre 1.10.2, sin bump de versión (no toca el binario): **paquete
`maly` publicado en el AUR** — `yay -S maly`/`paru -S maly` compila
desde el tag estable más reciente, mismo criterio que el default de
`mallow-install.sh`. El PKGBUILD vive en un repo aparte
(`~/Projects/PKGBUILDS/maly`, clon de
`ssh://aur@aur.archlinux.org/maly.git`), NO dentro de este repo: el AUR
publica empujando a ese remote directo, y mezclar esa historia con la
de maly exigiría un submódulo sin beneficio real. Sin CGo (coherente
con `internal/library`, sqlite es modernc puro Go) — build limpio con
`GOFLAGS="-buildmode=pie -trimpath -mod=readonly -modcacherw"` y
`CGO_ENABLED=0`, sin la sección `CGO_CPPFLAGS/CFLAGS/LDFLAGS` que trae
el ejemplo genérico de la ArchWiki (ahí sí hace falta, acá no hay nada
que enlazar). `depends=(mpv)`; `yt-dlp`/`ffmpeg`/`pipewire`/`pulseaudio`
como `optdepends`, igual que en el instalador. Empaqueta también una
unit de systemd `--user` (`ExecStart=/usr/bin/maly daemon`, distinta de
la que genera `mallow-install.sh` con `%h/.local/bin/maly` porque acá
el binario va a `/usr/bin`) — instalada pero SIN `enable`, a propósito:
un paquete no debe arrancar servicios solo, ese consentimiento es del
usuario. `license=('GPL-3.0-only')`: el `LICENSE` es el texto de la
GPLv3 sin ninguna cabecera de copyright del proyecto que declare "or
later", así que se tomó literal. Verificado con un build real de
`makepkg` de punta a punta contra la URL real de GitHub (checksums de
`updpkgsums`, no inventados): `go vet` + la suite de tests completa
(incluido `internal/daemon` con mpv real) pasan dentro del propio
empaquetado, y el `.pkg.tar.zst` resultante trae el binario PIE,
las tres completions con contenido real (generadas con el binario
recién compilado, mismo patrón que `inst_comp` en
`mallow-install.sh`) y la licencia. De paso se corrigió una
inconsistencia real que no tenía que ver con el PKGBUILD pero salió al
verificar el campo `license`: ambos README seguían diciendo "MIT" en
el pie pese a que el badge y el `LICENSE` ya eran GPLv3 desde la
relicenciación de la sesión anterior — corregido en los dos idiomas.

La **1.11.0** (2026-07-30) sale de una **auditoría de seguridad, calidad y
UX desde cero** pedida por el dueño explícitamente sin asumir que ninguna
decisión anterior fuera correcta — cubrió ejecución de comandos externos,
manejo de rutas y symlinks, el socket IPC del demonio, el instalador, el
PKGBUILD, la cadena de supply-chain completa (repo → tag → PKGBUILD →
compilación → instalador → binario) y dependencias. **Veredicto: ninguna
vulnerabilidad real** — todo camino explotable exige el mismo UID, que ya
tiene la cuenta, y por tanto no cruza ninguna frontera de confianza (SQL
parametrizado en todos lados, `--` antes del spec de yt-dlp,
`cookies_from_browser` viajando como argv separado, `ImportM3U` solo
resolviendo rutas ya indexadas, `ExportM3U` con `O_NOFOLLOW`, el caché de
carátulas acotado y purgado, `EnsureRuntimeDir` cubriendo el fallback
predecible de `/tmp`). Se confirmaron correctas y sin tocar: no poner
auth/`SO_PEERCRED` al socket (el dir 0700 + el chequeo de dueño ya es la
frontera real; peer creds entre procesos del mismo UID serían teatro), no
firmar tags/releases (la clave viajaría del mismo GitHub que ya se
confía), y no sacar `search` de `d.mu` (la 1.6.1 ya lo midió y refutó la
hipótesis).

Lo que sí había eran seis endurecimientos concretos, ninguno crítico, de
los que esta release cierra cuatro (los otros dos —canal de paquete vs
`maly update`, arreglos menores del PKGBUILD— quedan documentados para un
ciclo aparte, con el diseño ya decidido):

- **`maly update` pinnea el instalador al tag.** `InstallerCmd` bajaba
  SIEMPRE `mallow-install.sh` de `main`, aunque el binario que iba a
  compilar fuera un tag viejo: el código quedaba pinneado pero el script
  que lo instala no. `installerURL(ref)` (`internal/update/update.go`)
  arma la URL sobre el mismo ref anunciado por el chequeo; `ref == ""`
  (el one-liner del README) sigue cayendo en `main`, como siempre.
- **`maly.sock` queda en 0600.** Verificado en vivo: `net.Listen` lo
  creaba con el umask del proceso (`srwxr-xr-x` en la máquina del dueño),
  a diferencia de `mpv.sock`, el lock y `art/`, todos 0600/0700 explícitos.
  El dir 0700 de `EnsureRuntimeDir` sigue siendo la frontera real —esto es
  defensa en profundidad, no el arreglo que cierra el vector— pero el
  socket es control total del reproductor y alinearlo no cuesta nada.
- **`serve` gana deadlines de lectura y escritura**
  (`internal/daemon/daemon.go`). `subscriber.push` ya tenía
  `SetWriteDeadline`, pero el bucle principal de `serve` no tenía ninguno:
  un cliente que conecta y no manda nada (o deja de mandar) dejaba la
  goroutine y el fd clavados para siempre, y N conexiones así agotan los
  descriptores del demonio. `Daemon.idleTimeout` (`defaultIdleTimeout`, 5
  min) es generoso a propósito —`Do` responde en milisegundos, y las
  conexiones legítimamente largas son las de `subscribe`, que sale de este
  bucle antes de volver a este punto— y se limpia (`SetReadDeadline` cero)
  justo antes de entregar la conexión a `subscribe`, que sí necesita
  bloquear minutos sin deadline. Es CAMPO DE INSTANCIA y no var de paquete
  a propósito, y costó un `-race` real descubrirlo: con un var compartido,
  el override de un test corría detrás de las goroutines de `serve()` de un
  demonio de OTRO test cuyo `Run()` no había terminado de desmontarse
  —`go test -race` lo cazó entre `TestSocketPermisos` y
  `TestServeIdleTimeoutCierraConexion`, aunque cada test usa su propio
  `Daemon`— y por eso no está en la lista de paquetes con `-race` de CI
  (solo `library`/`mpris`); si algún día se agrega, esto ya no lo dispara.
- **La guarda anti-bomba de carátulas no desborda en 32 bits**
  (`internal/media/image.go`). `cfg.Width*cfg.Height > maxDecodePixels`
  multiplicaba dos `int`, que en 386/armv6l/armv7l —arquitecturas que el
  instalador soporta explícitamente— son de 32 bits: un PNG que declare
  dimensiones lo bastante grandes desborda el producto (65536×65536 da
  EXACTAMENTE 0 en `int32`) y la guarda se cuela justo en las plataformas
  con menos RAM. `dimsOK` compara en `int64`. La verificación tuvo su
  propia trampa: esta máquina de desarrollo es de 64 bits, así que
  reproducir el desborde exige simular `int32` explícito DENTRO del test
  (`TestDimsOKNoOverflow32Bit`), no confiar en que `int` real se desborde
  aquí.

La pieza más grande, **`maly get playlist <url> [nombre]`**, cierra además
un hallazgo de impacto real en UX: `getter.Command` no pasaba
`--no-playlist`, así que un URL con `&list=` —muy común al copiar y pegar
de YouTube— bajaba la playlist ENTERA a `music_dir` sin que nadie lo
pidiera. `getter.Opts` (antes tres posicionales) formaliza el contrato:
sin `Playlist`, siempre `--no-playlist`; con ella, `--yes-playlist` +
`%(playlist_index)02d` antepuesto al nombre de archivo, y con
`PlaylistSubdir` además `%(playlist_title)s/` como componente de
directorio, para que yt-dlp cree el subdirectorio él mismo cuando no hay
nombre explícito.

`cmd/maly/get.go` reimplementa la lógica de resolución en `runGetPlaylist`,
y `internal/tui/console.go` la duplica en `conGetPlaylist` +
`conGetPlaylistFinish` (internal/tui no puede importar `cmd/maly`, que es
`package main`; el patrón ya existía entre `runGet`/`conGet`). Con nombre
explícito, se valida como componente de ruta ANTES de tocar filesystem o
red (`filepath.Base(name) != name`, ni `.` ni `..`) y las pistas caen
directo en `music_dir/<nombre>`. Sin nombre, el título lo aporta yt-dlp
creando su propio subdirectorio, y maly lo aprende **diffeando el listado
de `music_dir` antes/después** de la descarga —una lectura de directorio,
determinista, sin parsear nada de la salida de yt-dlp—; exactamente un
directorio nuevo se acepta, cero o más de uno se rechazan como ambiguos
(mejor pedir un nombre explícito que adivinar mal). El título de YouTube es
el PRIMER camino donde un nombre de playlist es texto ajeno —los demás
siempre vinieron del teclado del dueño, y por eso `Playlists()` nunca había
necesitado sanear `name`— así que pasa por `safetext.Clean`, la misma
frontera que `ReadTags`/`ParseLRC`.

Verificado con el yt-dlp falso de siempre (`get_test.go`), extendido para
"descargar" dos pistas a un subdirectorio: cubre nombre explícito, título
auto-detectado, saneado del título con una inyección OSC real (mismo PoC
que `safetext_test.go`), nombre inválido rechazado ANTES de invocar yt-dlp,
y el caso ambiguo de `newDirEntry`. Dos trampas de shell que costarían un
rato redescubrir: el PATH aislado de `getSandbox` no incluye `/usr/bin`,
así que el yt-dlp falso no puede depender de `sed`/`mkdir` externos sin
agregarlos de vuelta al PATH (detrás del bin falso, para no tapar los
mocks); y sustituir el placeholder literal `%(playlist_title)s` con
expansión de parámetros POSIX pura exige escapar el `%` a mano
(`${dir%%\%(playlist_title)s*}`), porque sin escapar el operador `%%` se
come el primer `%` del patrón junto con el operador.

Todos los fixes de seguridad se verificaron en ambas direcciones —revertir
el código de producción y confirmar que el test nuevo falla de verdad, no
solo que no compila, la disciplina que la 1.6.1 dejó como lección—: el
socket da 777 sin el `Chmod`, la conexión muda cuelga hasta el deadline del
propio cliente sin el arreglo en `serve` (el test distingue EOF de un
cierre real contra el timeout del cliente, o "pasaría" igual con el bug
presente), y `dimsOK` acepta la bomba de 65536×65536 con aritmética `int32`
simulada.

Quedan documentados para un ciclo aparte, con el diseño ya decidido: el
**canal de paquete** (`version.Channel` vía ldflags del PKGBUILD +
fallback de que el binario resida bajo `/usr/` y no `/usr/local/`, para que
`maly update` remita al gestor de paquetes en vez de instalar una segunda
copia por detrás de pacman — hoy `mallow-install.sh` nunca pisa
`/usr/bin/maly`, así que un sistema con el paquete de AUR Y el instalador
corrido alguna vez termina con dos binarios, dos juegos de completions y
potencialmente dos units de systemd, una de las cuales corre el binario
que no está en el PATH); y dos arreglos menores del PKGBUILD
(`install -Dm644 <(cmd)` no detecta un fallo del binario dentro de la
sustitución de proceso, y `check()` no exporta `CGO_ENABLED=0` así que
testea un build distinto del que empaqueta). Sin acción, por ser
mejora de dependencias y no de seguridad: `gonum` (19 MB de módulo) para
una sola FFT en `internal/viz`.

La **1.11.1** (2026-07-31) cierra el canal de paquete, el único de los tres
ítems diferidos de la 1.11.0 con impacto real — el dueño lo vivió en carne
propia esa misma sesión: con el paquete de AUR y el instalador manual
conviviendo, terminó con dos binarios, dos units de systemd y tuvo que
deshabilitar la del paquete a mano para que `~/.local/bin/maly` quedara
como el único corriendo de verdad.

**`version.Channel`** (`internal/version/version.go`) es una `var string`
nueva, `""` por defecto — a propósito NO es la const `Version`, porque
`-ldflags -X` solo puede asignar variables de paquete a nivel top-level,
nunca constantes. El PKGBUILD la fija en `build()` con
`-ldflags "-X maly/internal/version.Channel=pacman"`. **`Packaged() bool`**
es lo que consultan los llamadores: `true` si `Channel != ""`, o si no, por
un fallback de ruta (`isPackagedPath`, función pura y separada para poder
testear con rutas fabricadas): el binario resuelto
(`os.Executable()` + `filepath.EvalSymlinks`) cae bajo `/usr/` pero no
`/usr/local/` — por FHS eso es territorio de un gestor de paquetes, y
`mallow-install.sh` nunca instala ahí (confirmado leyendo su única
invocación de `go build`, que solo lleva `-ldflags '-s -w'`, sin ningún
`-X`). El fallback cubre a un packager futuro que se olvide del flag. Es
una heurística de UX y no una frontera de seguridad, por eso `Packaged()`
no memoiza el resultado: el costo real es un puñado de syscalls,
irrelevante llamado una vez por render del footer, y sin memoización cada
test controla `Channel` libremente sin pelearse con estado global.

El gate va en tres puntos, todos DESPUÉS del chequeo de "ya estás al día"
(ese mensaje sigue ganando primero, sin mencionar canal): `runUpdate`
(`cmd/maly/update.go`) y `conUpdate` (`internal/tui/console.go`, mismo
patrón de espejo que `runGet`/`conGet`) no tocan `InstallerCmd` con un
binario empaquetado — imprimen `up.found_packaged` y listo, sin acercarse a
curl ni al instalador. El pie de la TUI (`view.go`, caso `updAvail`) usa
`tui.update_avail_packaged` en el mismo lugar de la cadena de prioridad. Y
`maly info` suma una fila de canal junto a `info.binary`, siguiendo la
filosofía "info lista hechos": el canal es un hecho de la instalación, no
un veredicto de `doctor`. `internal/update/update.go` en sí NO se toca: el
chequeo de red (`Latest`/`Cached`/`SaveCache`) sigue siendo agnóstico al
canal, porque el aviso informativo vale igual aunque el binario sea del
paquete. `doctor.go` tampoco: su mensaje "run: maly update" sigue siendo
correcto tal cual (`maly update` ya redirige bien), solo un paso menos
directo — no amerita el cambio.

Verificado en ambas direcciones en cada punto con lógica nueva: sin el
gate, `TestRunUpdatePackaged`/`TestConUpdatePackaged` intentan
`InstallerCmd` de verdad y fallan mencionando curl (que a propósito no
está en el PATH del test); sin la exclusión de `/usr/local/` en
`isPackagedPath`, `TestIsPackagedPath` falla con `/usr/local/bin/maly`
clasificado como empaquetado; sin el caso condicional del footer,
`TestFooterUpdateAvailChannel` sigue mostrando "maly update" con el canal
empaquetado.

De los otros dos ítems diferidos de la 1.11.0, ninguno entró en esta
release: los dos arreglos menores del PKGBUILD quedan para cuando el dueño
confirme (son baratos y de bajo riesgo, pero no se tocan sin luz verde
explícita), y `gonum` se investigó a fondo y se descartó sacarlo — medido
en la sesión: el módulo pesa 19 MB en disco, pero el linker de Go solo mete
~65 KB en el binario final, y el `go.sum` de maly no lleva ninguna de las
dependencias pesadas que gonum declara en su propio `go.mod` (el *module
graph pruning* de Go desde 1.17 las descarta sin tocarlas). La nota "19 MB"
de la auditoría original medía el eje equivocado; reemplazar una FFT real
auditada por una propia para ahorrar 65 KB es mal cambio.

El dueño confirmó los dos arreglos del PKGBUILD (detección de fallo en las
completions, `CGO_ENABLED=0` en `check()`) en un ciclo aparte — quedaron
enteros en el repo del PKGBUILD (`pkgrel` 1 → 2), sin tocar código de este
repo.

Sobre 1.11.1, sin bump de versión (no toca el binario ni ningún paquete
Go): la unit de systemd `--user` pasa de `graphical-session.target` a
`default.target`. Salió de una duda real del dueño sobre su propio setup:
en Hyprland (y sway, y varios WMs minimalistas) nadie activa
`graphical-session.target` solo —a diferencia de GNOME/KDE, que sí—, así
que sin una unit puente hecha a mano (el dueño tenía la suya,
`hyprland-session.service`, con `ExecStart=/usr/bin/true` y
`BindsTo=graphical-session.target`) la unit de maly nunca arrancaba sola.
`default.target` lo alcanza cualquier sesión de `systemd --user` sin
necesitar que el compositor coopere, y maly no necesita nada
*gráfico* en sí —mpv corre con `--no-video`, MPRIS es solo D-Bus, el
visualizador capta audio, no pantalla— así que es el target correcto y no
solo el más compatible. Se tocaron los CUATRO lugares donde vive la unit:
`mallow-install.sh` (el generador), `maly.service` del PKGBUILD (con su
propio `pkgrel` bump y `updpkgsums`, porque el archivo tiene su propio
checksum en `source=()`), el ejemplo documentado de ambos README (que de
paso se renombró de "Hyprland" a "Servicio systemd --user" y perdió el
gancho manual de autostart —`hl.exec_cmd("systemctl --user start
maly")`—, ya innecesario con `default.target`), y la unit local del propio
dueño, reaplicada y verificada en vivo (`systemctl --user status maly`
con el symlink ahora bajo `default.target.wants/`).

La **1.12.0** (2026-08-01) cierra P0+P1+P2 de una **auditoría de UX
completa y "desde cero"** (76 hallazgos), la primera del proyecto dedicada
enteramente a experiencia de usuario y no a seguridad — mismo formato que
las de 1.1.5/1.6.0/1.11.0: por tandas de prioridad, y dentro de cada
tanda, de dificultad baja a media primero (instrucción explícita del
dueño). Quedan 13 hallazgos P3 sin tocar; diez de ellos, elegidos por el
dueño, se documentan más abajo en "Post-1.0 (candidatos)" para revisar en
un ciclo aparte.

**P0 — 7 hallazgos, los únicos con impacto en el camino feliz.** La ayuda
(`?`) tapaba cualquier modal ya abierto (paleta/songs/playlists/ahora
suena) y las teclas siguientes caían en el modal invisible de abajo (T1,
`internal/tui/tui.go`: las cuatro teclas que abren un modal cierran la
ayuda primero en vez de apilarse sobre ella). `ctrl+x` borraba una
playlist completa de un solo toque sin deshacer posible (T26,
`playlists.go`: la primera pulsación solo arma `m.plConfirm`, `y`/`enter`
confirma). Rutas relativas fallaban en `add`/`play` porque el demonio
resuelve con SU cwd, no el del cliente — y las tres completions de shell
empujan activamente a esa forma (C9, `resolveQueryPath` en `client.go`,
mismo patrón que ya usaba `runScan`). Un solo ítem privado/bloqueado en
una playlist de YouTube tiraba toda la descarga a cero pistas, sin
reintento posible (G1, `get.go`: el fallo de `cmd.Run()` en la rama de
playlist pasa a ser aviso, no corte — se sigue con lo que sobrevivió).
`maly update` sin red imprimía literalmente `exit status 128`: el
`*exec.ExitError` de `.Output()` se propagaba crudo y el stderr real de
git (la parte útil) se descartaba sin usar (D7.1, `internal/update`:
`Latest()` ahora expone el stderr). Y el instalador era ciego a una
instalación por gestor de paquetes —nunca sondeaba `/usr/bin/maly`, donde
aterriza el paquete de AUR— y su consejo final de "reinicia con `maly
kill`" no reiniciaba nada bajo una unit con `Restart=on-failure` (D14.1 y
D14.2, `mallow-install.sh`: sondeo informativo de `/usr/bin` + el mismo
mecanismo de canal de paquete que cerró la 1.11.1, y el hint de reinicio
pasa a `systemctl --user restart maly` cuando la unit existe).

**P1 — 20 hallazgos**, en 5 partes temáticas (bajas→medias dentro de cada
una). TUI: la ayuda se truncaba sin indicador de scroll en terminales
chicas y sin forma de llegar al final (T2); la capa "Ahora suena" mataba
teclas en silencio con la ayuda abierta encima (T3); el remedio de
"biblioteca vacía" mandaba a una terminal que podía no existir, en vez de
decir `ctrl+p` → `scan` (T6, junto con T7: el estado ya no se marca error
sino informativo); un scan lanzado desde la consola no reportaba avance
—la consola tapa el pie, único lugar donde vivía la barra (T12)—; `tab`
en el selector de canciones agregaba a la cola sin ninguna confirmación
(T13); y la paleta `ctrl+p` no tenía `info`/`doctor`/`config` —los tres
comandos que más hacen falta cuando la TUI misma está fallando (T16,
`internal/tui/console_diag.go` nuevo: reimplementa el ensamblado porque
`internal/tui` no puede importar `cmd/maly`). CLI: no existía `maly
remove <pos>` pese a que la op IPC y la tecla de la TUI ya existían
(C10); `shuffle` aceptaba basura y cambiaba el estado en silencio
mientras `repeat`, con la misma forma, fallaba correctamente (C11,
`daemon.go`); la ayuda integrada tenía una fila que rompía la columna
fija y una nota que afirmaba que toda la sección de biblioteca funciona
sin demonio, falso para `playlist play` (C12–13). `get playlist`: el
choque de nombre de playlist se detectaba al FINAL, tras descargar todo
(G2, ahora se consulta `lib.Playlists()` antes de tocar red); y con
nombre explícito la playlist resultante incluía TODO el audio ya presente
en el directorio destino, no solo lo recién bajado (G3, mismo mecanismo
de diff de directorio que la rama sin nombre). Demonio/i18n: un mensaje
excelente para "demonio ausente" y ninguno para "demonio ocupado" —un
timeout de I/O crudo (D7.3, `internal/ipc`); `ErrAlreadyRunning`
hardcodeado en inglés, traducido en un solo punto de tres (D7.2); el
binario nunca leía el idioma del sistema, tirando a la basura la
detección que ya hace el instalador (D10.1, `envLangHint` en `main.go`,
cascada mínima LC_ALL→LC_MESSAGES→LANG, sin persistir nada); `doctor` sin
traducir mientras `info` sí (D10.2); y el pie de la TUI perdía `? help` /
`q quit` primero y antes en español, por ser sistemáticamente más largo
(D10.3, la misma pieza que T23: la cadena se reordena para que ayuda y
salir vayan primero). Documentación: `maly config` no aparecía en ningún
README (D13.1), `maly move` faltaba entero del README en inglés (D13.2),
y ambos README subestimaban ~24× la frecuencia real del chequeo de
actualización (D13.3, ya `cada hora` desde la 1.6.2, no "una vez al día").

**P2 — 36 hallazgos** (32 arreglados + 4 documentados sin tocar), en 5
partes temáticas siguiendo el mismo patrón: paneles/estados de la TUI
(T4, T5, T8+D7.5, T9, T10, T11, T14, T15, T24 — posición del cursor en
biblioteca/cola, filtro con tecla de limpiar, error de carga persistente
en vez de un flash de 4s, picker distingue biblioteca vacía de sin
resultados, remedio real para "demonio no responde", carátula oculta por
ancho distinguida de sin carátula, bandera de carga inicial, flash al
alternar visualizador, terminal chica dice el mínimo real); ayuda y
atajos (T18, T19, T20, T29, T31 — wording de `h`/`l`, fila de
pgup/pgdown/home/end, plantilla de `[keys]` completa, cerrar la ayuda
redespacha la tecla salvo `esc` —que sigue siendo neutra por T3—, ctrl+d/
ctrl+u en el picker); consola (T17, T21, T27, T28, T30 — menciones de
`get playlist`/`rescan`/`exit`, historial de comandos con ↑/↓, scroll de
salida con pgup/pgdown); CLI (C15, C16, C17, C18, C19, C20, C21, C22 —
confirmación en `playlist delete`, completions de posición para
`playlist remove`, mensajes "amigables" a stderr, código de salida
unificado a 1, `%t` en vez de on/off en `maly config`, error limpio para
una ruta de scan explícita inexistente, `maly logo` como comando CLI
nuevo, consola menciona `get playlist`); y `get`/demonio (G4, G5, G6,
D7.4 — no filtrar `ytsearch1:` al mensaje, `maly get` reporta qué bajó
vía el mismo diff de directorio que `get playlist`, cierre de `get
playlist` con verbo y mayúscula, `embeddedStartErr` deja de prefijar
errores que ya son frases completas). Las 4 no tocadas, documentadas con
su razón en vez de diferidas por descuido: **T22** ("no responde" vs "no
está corriendo" son estados genuinamente distintos, no la misma frase con
otra ropa), **T25** (tres umbrales de altura de la TUI son presupuestos
deliberados por feature, no un bug), **T32** (`tab` con tres significados
está mitigado en la práctica — el modal que lo cambia tapa el pie donde
se lee el hint viejo), **C14** (el parseo de nombre de playlist distinto
por subcomando es inherente a su aridad; un mecanismo de comillas nuevo
sería desproporcionado).

Toda la tanda siguió la misma disciplina de verificación que las
auditorías anteriores: cada fix con estado o lógica nueva lleva un test
verificado en AMBAS direcciones (revertir el fix quirúrgicamente,
confirmar que el test falla por la razón correcta —no por un error de
compilación—, restaurar); los cambios de solo texto/i18n no llevan test
dedicado. `go build`, `go vet` y la suite completa (`go test ./...`,
incluido `internal/daemon` con mpv real) pasan en verde.

La **1.12.1** (2026-08-03) cierra un reporte del dueño sobre la 1.12.0
recién instalada: la Command Palette (`ctrl+p`) rompía su borde derecho con
contenido ancho, llegando a salirse de la pantalla. Investigación de solo
lectura (2 agentes Explore en paralelo, más lectura directa de la fuente de
`charmbracelet/bubbles@v1.0.0` y `muesli/reflow@v0.3.0`) encontró la causa
raíz y, auditando los otros 4 modales de la TUI por el mismo patrón, tres
agujeros más.

**La causa raíz real**: `m.conInput`/`p.input` (bubbles `textinput`, usados
por la consola y por el `picker` genérico de `ctrl+o`/`ctrl+l`) nunca
recibían `.Width`. En bubbles v1.0.0, `handleOverflow()` desactiva el scroll
horizontal por completo cuando `Width <= 0`, así que `View()` emitía el
valor COMPLETO sin ventana — un comando largo (una URL de `get`, por
ejemplo) rompía el borde en vivo, mientras se escribe, sin hacer falta ni
tocar Enter. Arreglado fijando `Width` en cada render (mismo criterio que
`p.page` en `picker.render()`: reasignar es barato) más un `clip()` de red
de seguridad para el frame en que `Width` cambia y `handleOverflow` todavía
no corrió de nuevo (un resize sin ninguna tecla de por medio).

**El hallazgo base que explica por qué hacía falta auditar los demás
modales**: `styles.panel()` nunca trunca por ancho — solo por alto y el
título. Para el cuerpo hace `padTo(line, innerW)`, que es un no-op cuando la
línea ya mide `>= innerW`: el contrato de `panel()` es que cada línea ya
venga del ancho correcto, y la protección depende enteramente de cada
llamador. Tres agujeros más del mismo patrón:

- **`conHelp()`** (filas de ayuda de la consola): las filas de `get`/
  `playlist` eran estructuralmente más anchas que la caja en cualquier
  tamaño de terminal — el label de `get` (ensanchado en la 1.12.0, y
  redundante con `con.help_local`) se revirtió a su forma corta, y se
  extrajo `formatHelpRow()`, que clipea label y descripción por columna
  independiente en vez de confiar en que nada exceda su presupuesto.
- **`npMeta()`** ("Ahora suena"): la línea de ícono+tiempo era la única sin
  `clip`, desbordaba con una duración larga en terminales angostas.
- **`picker.render()`**: `sel.none_empty` (el aviso de biblioteca vacía) sin
  `clip`, desbordaba en terminales angostas — afecta tanto al selector de
  canciones como al panel de playlists, que comparten esta función.

Sobre esa base, dos reportes de seguimiento del dueño, ya con el fix
anterior probado en vivo:

**El modal de Ayuda (`?`) se veía apretado.** La fila `pgup/pgdn home/end`
(19 celdas) superaba la columna fija de 14 que usaban todas las teclas, y
`padTo` no acorta — esa fila quedaba pegada a su descripción sin ningún
espacio de separación, mientras el resto sí lo tenía. Como el modal YA
agranda la caja al contenido (no trunca), el arreglo fue calcular el ancho
de columna a partir de la fila más ancha real en cada render en vez de un
14 fijo — corrige esta fila y cualquier tecla remapeada por el usuario que
la exceda en el futuro.

**La Command Palette seguía perdiendo texto** en la fila de `playlist`
(descripción de 74 celdas, la lista completa de subcomandos) incluso con el
borde ya protegido. `pickerWidth` subió su tope de 80 a 100 columnas
(decisión del dueño, sopesada contra la alternativa de hacer crecer la caja
al contenido como Ayuda — descartada porque la consola es un LOG que se va
acumulando, no una lista fija recalculada cada vez: crecer para una línea
vieja la dejaría ancha aunque esa línea ya hubiera scrolleado fuera de
vista). Con el tope más alto, `con.help_local` —un hint que ya había crecido
en la propia 1.12.0 hasta juntar viz/cls/quit/exit/rescan/get playlist en
una sola línea— seguía sin entrar ni con el nuevo tope; se recortó a lo
esencial sin perder qué alias documenta, que era el punto original de esa
adición.

Cada fix con estado o lógica nueva (Width del input, `formatHelpRow`, la
columna dinámica de Ayuda, los tres `clip` agregados) lleva un test
verificado en ambas direcciones, misma disciplina que la 1.12.0. Los
cambios de solo texto (labels revertidos, `con.help_local` recortado, el
tope de `pickerWidth`) no llevan test dedicado.

Sobre 1.12.1, sin bump de versión: la **auditoría técnica del 2026-08-08**
(commit `5f10d3f`) cierra 23 hallazgos propios en dos tandas seguidas —la
primera ataca los 2 importantes y 4 moderados, la segunda los 8 menores
accionables; los 6 restantes (incluidos los 4 de oportunidad) se cierran
solo con razonamiento, sin código, por ser abstracción especulativa o la
regresión que la propia auditoría advierte evitar.

**Tanda 1 — importantes y moderados.** `RemoveFromPlaylist` pasa a
transaccional: cerraba una ventana TOCTOU real entre el snapshot de la
playlist y el DELETE por offset. `Player.Close` espera los callbacks
`onEnd`/`onChange` en vuelo (`cbWG`) antes de devolver el control, para que
el demonio no cierre la biblioteca mientras uno sigue escribiendo
(`learnDuration`). `containsAll` ya no reparte `strings.Fields` por fila en
cada render/tecla de filtro. `config.KeyConflicts` detecta acciones de
`[keys]` mapeadas a la misma tecla; se reporta como warn en `maly doctor` y
su espejo de la TUI. `console_diag.go` (info/doctor/config de la paleta
ctrl+p) tenía cero tests pese a ser la mayor pieza duplicada del proyecto.
Una red de paridad consola↔CLI (`ConsoleCommands` + dos tests que se
verifican entre sí) destapó un gap real: `remove <pos>` existía en la CLI y
no en la consola.

**Tanda 2 — menores.** `notifyMu` serializa `notify()` en el demonio: sin
él, dos goroutines de `player.onChange` concurrentes podían empujar a MPRIS
en un orden distinto al de los eventos reales (Position momentáneamente
retrocedida). El `mpris:trackid` de pistas sin ID de biblioteca usa un hash
fnv de la ruta en vez de `QueueIndex`: reordenar la cola ya no dispara un
"cambio de canción" espurio en playerctl/Waybar. `ipc.Client` documenta que
no es seguro para uso concurrente. Ambos README suman la fila de `maly
logo`, ausente desde la 1.12.0. `ImportM3U` acota el archivo a 8 MB con
`io.LimitReader`, mismo patrón que ya usa `internal/media/lrc.go` para el
mismo bug. `TrackNo`/`Year` se clampean a `[0, 9999]` en `scanTrack`, el
punto único de salida de la biblioteca (mismo criterio que ya aplica
`safetext.Clean` a los campos de texto ahí mismo). `Library.Scan` se parte
en `loadKnownMtimes`/`flushBatch`/`purgeGone` — extracción mecánica, sin
cambio de firma pública ni de comportamiento (verificado: ningún símbolo
nuevo es exportado).

Cada arreglo con lógica o estado nuevo se verificó en ambas direcciones. `go
build`, `go vet` y `go test ./...` limpios sin ningún SKIP; `go test -race`
limpio en `daemon`/`mpris`/`library`/`player`.

Sobre este mismo punto del repo, una **segunda auditoría técnica integral**
(seguridad, arquitectura, concurrencia, rendimiento, TUI/UX, robustez,
configuración, systemd, distribución, código Go, testing y documentación —
informe aparte, `~/Audits/MalyAu/`) confirmó que ninguno de los 23
hallazgos de arriba quedaba pendiente, y no encontró nada por encima de
MEDIUM: el modelo de amenaza (mismo UID = mismo nivel de confianza) sigue
siendo correcto, sin inyección de comandos, path traversal explotable ni
SQL injection en ningún punto. El hallazgo de mayor prioridad práctica que
dejó fue justamente que este roadmap no mencionaba el commit `5f10d3f` —el
motivo del párrafo de arriba—, seguido de un puñado de MEDIUM concretos y
una cola de LOW/INFO, organizados en un roadmap de cinco fases que cierra
la 1.13.0 de abajo.

La **1.13.0** (2026-08-12) implementa las cinco fases de esa auditoría
completas. Cada fase se revisó con `/code-review` sobre el diff acumulado
ANTES de seguir a la siguiente — disciplina que pagó de verdad: la review
sobre fases 1-3 encontró que `recoverResponse` no bastaba por sí solo si
el código bajo panic tenía un mutex tomado con Lock/Unlock a mano (lo
recuperaba, pero dejaba el mutex tomado para siempre — peor que el crash
original), y una segunda review sobre la fase 5 encontró, entre otras
cosas, una carrera real de datos en el propio mecanismo de reintento del
visualizador que se acababa de escribir. Ambas rondas están detalladas
abajo porque el propio proceso de auditar-implementar-revisar es parte de
lo que esta release demuestra, no solo su resultado.

**Fase 1 — seguridad y estabilidad.** `recoverResponse` (nuevo en
`daemon.go`) envuelve `handle()` COMPLETO —no solo `dispatch()`— con un
`recover()`: un panic de programación ya no tumba el proceso del demonio
entero para TODOS los clientes, y como TODOS los caminos de entrada pasan
por `handle()` (serve() vía el socket, `Do()` de los tests, y los métodos
de `mpris.Controller`, que llaman a `handle()` directo sin pasar por el
socket), envolver ahí cubre a MPRIS también. La revisión posterior
destapó que no alcanzaba: `handle()`, `advance()`, `notify()` y
`learnDuration()` tenían `Lock()`/`Unlock()` a mano con múltiples salidas,
así que un panic a mitad de esas funciones dejaba `d.mu`/`d.notifyMu`
tomado para siempre aunque el proceso sobreviviera — las cuatro pasaron a
una función inmediata con `defer` para el lock. Los callbacks
`onEnd`/`onChange` del player (fire-and-forget, disparados por eventos
reales de mpv) quedaron protegidos aparte con `safeCall()` en
`player.go`, porque no pasan por `dispatch()` en absoluto. El cliente IPC
(`ipc.Client`) pasó de `bufio.Reader` sin tope a un lector acotado a 64
MiB (SEC-01) — pero la primera versión usaba `bufio.Scanner`, y la
revisión de la fase 5 encontró (verificado con un programa Go standalone
aparte) que el error de `Scanner` es "pegajoso": tras un timeout, TODAS
las llamadas siguientes a `Scan()` fallan con el MISMO error para
siempre, incluso con datos ya disponibles y un deadline nuevo — dejaba el
`*Client` inservible tras el primer hiccup de red, algo que el
`bufio.Reader` de antes de SEC-01 nunca tuvo. La versión final volvió a
`bufio.Reader` con el tope reimplementado a mano sobre `ReadSlice`.
`maxSubscribers` (64, campo de instancia como `idleTimeout`) evita agotar
fds/goroutines del demonio con conexiones `subscribe` sin cerrar (SEC-02);
el error de `os.Chmod(sock, 0o600)` ya no se descarta (CONC-01).

**Fase 2 — calidad.** `Load()` valida los 7 colores de tema/visualizador
que no llevaban guarda (antes solo `Logo` se autocorregía; CFG-1) — la
revisión de la fase 5 encontró de paso que `Load()` llamaba a `Default()`
tres veces (cada una releyendo `user-dirs.dirs` del disco, en el hot path
de completado de shell) y que una optimización ingenua de "una sola copia"
introducía un bug real de aliasing (`toml.Decode` puede reescribir el
array que respalda `Theme.Logo` IN PLACE; una copia superficial del
struct dejaba el snapshot de default apuntando al mismo array ya
corrompido — lo cazó `TestLoadLogoSane`, test ya existente). El error de
la query inicial de `FillDurations` ya no se descarta en silencio
(PERF-01). `TestConcurrentClientsStress` prueba ~40 conexiones reales al
socket bajo `-race` (TEST-1), `FuzzParseLRC` corrió 2M+ ejecuciones sin
panics (TEST-2), y `BenchmarkScan`/`BenchmarkSearchLimit` quedan
persistentes en `internal/library` (TEST-3).

**Fase 3 — UX/TUI.** La consola vuelve siempre al fondo (`conScroll = 0`)
al ejecutar un comando nuevo, en vez de dejar el resultado fuera de vista
si el usuario había hecho pgup (UX-N1); `ctrl+home`/`ctrl+end` saltan al
principio/final de su historial —no `home`/`end` sueltos, que chocan con
el textinput activo— (UX-N2). `Theme.Error` es color configurable (antes
hardcodeado) y la fila seleccionada pasa de `Reverse(true)` a
`Background`+`Foreground` explícitos con contraste calculado por fórmula
YIQ, que no depende de cómo cada terminal implemente el reverse video
(UX-N3) — la revisión posterior encontró que `conConfig` de la TUI no
sumó la fila `error` al mismo tiempo que `cmd/maly/config_cmd.go`, la
misma clase de gap de paridad consola↔CLI que ya mordió al proyecto con
`remove <pos>`. `library.OpenIfExists` se extrajo de `cmd/maly` al
paquete `library` para que `internal/tui/select.go` también lo usara sin
fabricar una biblioteca vacía al abrir sin escanear (UX-N5) — la revisión
encontró que la función compartida conflaba "no existe" con "existe pero
`Open()` falló" (DB corrupta/bloqueada), remedio equivocado para un
comando interactivo; `select.go` volvió a distinguir los dos casos con un
`os.Stat` propio. `esc` en el selector de idioma cierra con el idioma ya
activo en vez de tragarse la tecla (UX-N6); el hint de scroll de la ayuda
decía "cualquier otra tecla cierra" cuando pgup/pgdn/ctrl+u/ctrl+d en
realidad scrollean (UX-N7).

**Fase 5 — features.** El visualizador reintenta la captura cada 15 s
tras perderla en pleno uso (PipeWire se reinicia, el dispositivo
desaparece), en vez de quedar en animación falsa el resto de la sesión —
solo si `hadBackend` (hubo una captura real alguna vez; sin eso, un
sistema sin pw-record/parec instalados no gana nada reintentando para
siempre). La propia revisión de este mecanismo encontró tres bugs de
concurrencia reales en el primer intento: `startCapture` escribía
`v.cmd`/`v.backend` sin lock mientras `Close()` los leía CON lock (carrera
real: `Close()` podía matar el proceso viejo mientras el nuevo del
reintento quedaba huérfano); `armRetry` tenía un TOCTOU donde `retrying`
podía quedar en `true` para siempre si el backend "aleteaba" (moría al
instante tras reconectar); y `Close()` no era idempotente (`sync.Once`
ahora, como `daemon.Close`). Los tres se cerraron unificando el punto de
verdad: `v.cmd`, `v.backend`, `v.fake` y `v.retrying` se escriben TODOS
bajo el mismo lock, en la misma sección atómica de `startCapture`, con un
chequeo de `v.closed` que mata en el acto un proceso que ganó la carrera
contra `Close()`. `vizWarned` (el flash de aviso) se resetea cuando el
viz sale de fake, o una segunda pérdida más tarde en la sesión quedaba
muda. **SYSD-1**: la unit de systemd `--user` (en `mallow-install.sh` y
en el `maly.service` del PKGBUILD) suma `NoNewPrivileges`,
`ProtectSystem=strict`, `PrivateTmp`, `RestrictAddressFamilies=AF_UNIX`,
`RestrictNamespaces`, `LockPersonality` y los `Protect*` de kernel/cgroups
—deliberadamente SIN `ProtectHome`, que bloquearía `music_dir` si vive en
un disco externo. `ProtectSystem=strict` resultó más agresivo de lo que
la auditoría original asumió: su propio manpage dice que deja TODO de
solo lectura salvo `/dev`/`/proc`/`/sys` (ni siquiera `$XDG_RUNTIME_DIR`
queda exento), así que hizo falta `ReadWritePaths=%h/.config/maly
%h/.local/share/maly %t/maly` para que el demonio pudiera crear su lock,
sus sockets y el `config.toml` de la primera vez. Verificado EN VIVO antes
de tocar los archivos reales: una unit de systemd temporal y sandboxeada
(XDG_CONFIG_HOME/DATA_HOME/RUNTIME_DIR propios, apuntando al mismo binario
instalado) arrancó limpio, escaneó música real, respondió por IPC, y
cerró limpio con `maly kill` — todo sin tocar el `maly.service` real del
dueño, que siguió corriendo en todo momento.

**Fase 4 — rendimiento**: sin cambios, tal como concluyó el roadmap
original de la auditoría.

Cada fix con lógica o estado nuevo (de ambas rondas de revisión incluidas)
se verificó en ambas direcciones — revertir, confirmar que el test falla
por la razón correcta, restaurar —, con dos casos que vale la pena
recordar: el de `ipc.Client` se verificó además con un programa Go
standalone fuera del repo (reprodujo el error pegajoso de `Scanner` y la
recuperación correcta de `Reader` contra un servidor TCP real, no
simulado), y el de `Theme.Logo` lo cazó un test YA EXISTENTE
(`TestLoadLogoSane`) en vez de uno nuevo escrito para la ocasión — la
prueba de que la disciplina de correr la suite completa después de cada
cambio, no solo los tests nuevos, encuentra regresiones que un test
dirigido no habría cubierto. `go build`, `go vet`, `gofmt -l .` y
`go test ./...` limpios; `go test -race` limpio en
`daemon`/`library`/`mpris`/`player`/`viz`/`ipc`.

Sobre la 1.13.0, sin bump de versión (no toca el binario, solo la unit de
systemd): **`ReadWritePaths=%t/maly` rompía en un boot limpio de verdad**,
en la propia máquina del dueño, horas después de que SYSD-1 se diera por
"verificado en vivo". Síntoma: `maly.service` en `start-limit-hit`,
`status=226/NAMESPACE`, `journalctl` mostrando "Failed to set up mount
namespacing: /run/user/1000/maly: No such file or directory". Causa: el
manual de `systemd.exec` es explícito y se había pasado por alto —
`ReadWritePaths=` exige que la ruta YA EXISTA en el momento en que se arma
el namespace de montaje, que ocurre ANTES de que el proceso arranque (y
por lo tanto antes de que `EnsureRuntimeDir` llegue a crearla). En caliente
—reiniciando el servicio dentro de la misma sesión— el directorio ya
existía de una corrida anterior y el problema quedaba oculto; en un boot
real, `/run/user/$UID` es tmpfs recién montado y `maly` no existe ahí
todavía. La verificación en vivo de SYSD-1 no lo agarró por la misma clase de motivo
que después causó el bug: la unit de prueba sandboxeada de entonces tenía, por
conveniencia, un directorio padre más amplio (`%h/.mtst`) ya creado de
antemano, que enmascaró exactamente este caso — lección repetida de la
que ya dejaron la 1.6.1 y la fase 5 de arriba: verificar no es suficiente
si el propio arnés de prueba no reproduce las condiciones reales.

El arreglo usa el mecanismo correcto de systemd en vez de pelear con
`ReadWritePaths` a mano: `RuntimeDirectory=maly` y
`ConfigurationDirectory=maly` (con sus `*Mode=0700`) crean
`$XDG_RUNTIME_DIR/maly` y `$XDG_CONFIG_HOME/maly` ANTES de arrancar el
proceso, con la propiedad correcta, y quedan exceptuadas de
`ProtectSystem=strict` solas —sin necesitar tocar `ReadWritePaths` para
ninguna de las dos—, que es exactamente para lo que existen (`man
systemd.exec`, tabla de `RuntimeDirectory=`/`StateDirectory=`/
`CacheDirectory=`/`ConfigurationDirectory=`, columna "Below path for user
units"). `$XDG_DATA_HOME` (biblioteca y sesión) no tiene una directiva
dedicada en esa tabla —solo cubre RUNTIME/CONFIGURATION/STATE/CACHE, y
`StateDirectory=` apunta a `$XDG_STATE_HOME`, no a `$XDG_DATA_HOME`, así
que no sirve como sustituto—, así que sigue yendo a mano vía
`ReadWritePaths=`, pero con `-` al principio: el propio manual documenta
ese prefijo explícitamente para "ignorar la ruta si no existe" en vez de
abortar el arranque.

Esta vez la verificación fue con el escenario real: una unit de prueba
aparte, con nombres que no chocan con los reales (`RuntimeDirectory=maly-hwtest`,
`ConfigurationDirectory=maly-hwtest-cfg`), apuntando a un
sandbox donde NINGUNO de los tres directorios existía de antemano —ni
siquiera su padre—, confirmando que arranca limpio y que un `maly scan`
real escribe la biblioteca (76 pistas) antes de tocar ninguno de los tres
archivos reales (unit del dueño, `mallow-install.sh`, `maly.service` del
PKGBUILD, ambos README). Aplicado en caliente a la unit real del dueño sin
perder la sesión ni la biblioteca ya existentes.

La **1.13.1** (2026-08-14) refina la Lyrics View de la capa "Ahora suena"
(ctrl+t) en dos pasadas sobre el mismo `.lrc` sincronizado que ya existía.
La primera agrega una jerarquía visual por distancia a la línea vigente:
`current` queda con un pulso de brillo sutil ("respiración" — una onda
seno pura sobre el reloj de pared, congelada en pausa; sin ticker propio,
viaja gratis en los pushes de `Status` que el demonio ya manda varias
veces por segundo mientras suena música) y el resto de las líneas se
atenúa mezclando los colores YA existentes del tema (Dim→Accent para las
cercanas, Dim→Border para las lejanas), con una asimetría deliberada entre
`next` y `previous` a la misma distancia (next anticipa lo que viene,
previous ya no compite por atención). La segunda, tras probar la primera
en vivo, agrega un corte de visibilidad (`maxLyricDistance`, 4): más allá
de esa distancia el contexto ya no sigue apagándose hasta casi ilegible,
directamente desaparece (fila en blanco). En la altura de terminal
dominante (`lyrH≈7`, ver la sección de la TUI) esto casi nunca dispara —la
ventana natural ya cabe por debajo del corte—; se nota sobre todo en
terminales altas, que antes llenaban el panel entero de texto casi
invisible. Ambos cambios quedaron acotados a `internal/tui/nowplaying.go`
+ su test, verificados en vivo con mpv real además de la suite completa.

Entre esas dos pasadas se probó y se descartó una tercera: un "carrusel"
con espaciadores reservados alrededor de `current` y letter-spacing en su
texto, diseñado con ayuda de un agente que encontró y corrigió a tiempo un
bug real de aliasing en el enfoque original de splice para insertar los
espaciadores. Se implementó y se verificó en vivo (funcionaba), pero el
dueño decidió no quedarse con ella y volver al enfoque más simple de la
primera pasada — se revirtió entera sin llegar a commitear.

La **1.14.0** (2026-08-15) es el **rediseño de la pantalla principal**,
pedido por el dueño como brief propio en cinco fases, con parada y reporte
al final de cada una. El detalle de layout, barra y limpieza de títulos vive
en la sección de `internal/tui` de arriba; acá quedan las decisiones y lo
que costó descubrir.

*Antes de escribir código* se revisó el brief contra la estructura real y
salieron 13 choques, tres con consecuencias sobre el plan. El más grande:
**cambiar `config.Default()` no re-tiñe a NADIE**, porque `configTemplate`
escribe las claves de color en el archivo la primera vez y el config del
usuario no se reescribe jamás — la paleta nueva solo la ven instalaciones
nuevas. De ahí salió que `accent_dim`, `surface` y los tres `progress_*` se
DERIVEN de `accent` (`Theme.ResolveDerived`) en vez de ser literales: un
tema propio del usuario sigue siendo coherente sin tocar cinco claves más.
`Load` los vacía antes del decode (mismo patrón que `cfg.Keys = nil`) para
distinguir "lo escribió el usuario" de "viene del default"; sin eso, un
accent propio quedaba con bordes y selección de otra paleta. No llevan
`clampHex`: para ellos vacío e inválido son el mismo caso. En el template
van COMENTADOS a propósito — escribirlos mataría la derivación. Los otros
dos choques: `[progress]` como sección propia habría forzado cambiar la
firma de `newStyles` (que recibe `config.Theme`, y `styles.theme` lo es), y
la maqueta del brief dibujaba una rejilla con junctions que `panel()` no
sabe hacer; el dueño eligió `[theme] progress_*` y paneles sueltos.

**El tema del dueño no era el default viejo** sino uno propio (slate/blanco,
`accent = "#e2e8f0"`), así que no se le sobreescribió: su `config.toml` se
adaptó a la estructura nueva conservando sus colores. De paso apareció que
su `border = "#64748b88"` (9 caracteres, con alfa) llevaba desde CFG-1
clampeándose en silencio al default — maly no soporta `#rrggbbaa` en ningún
color, y soportarlo se descartó (con `transparent = true` no hay fondo
conocido contra el que componer).

Cuatro decisiones que el dueño tomó sobre el diseño ya implementado, y que
conviene no revertir por descuido: la **barra de reproducción va abajo y a
ancho completo también en tres columnas** (el primer intento la ponía solo
en la columna, donde 26 celdas no dan para leer el progreso); la columna
solo lleva **carátula + ficha**, y por eso `npMeta` se partió en
`npMetaText` (ficha) + el resto — el progreso se dibuja UNA vez, que es lo
que un test fija ahora, porque duplicarlo ya fue un defecto real hasta la
1.6.1; las **letras van en panel propio** bajo la columna y no dentro del
mismo marco; y la **línea vigente se envuelve** (hasta 3 filas, sin cortar
palabras) mientras el contexto queda a una fila con su `…`. Esa última salió
de una pregunta del dueño a mitad de implementación y se decidió MIDIENDO
sus `.lrc` reales: 266 líneas, mediana 25 caracteres, p90 55, máximo 107 —
en las ~28 celdas útiles de la columna entra solo el 63 %, así que el
problema era real y no hipotético. Se descartó la marquesina (leer texto que
se desplaza es peor que leerlo en dos filas, y contradice la autocancelación
de animaciones de la 1.7.2) y ensanchar la columna (a 40 celdas seguiría
cortando el 23 %, robándoselo a la cola).

**Tres bugs reales que destaparon los tests del relayout**, todos
preexistentes o recién introducidos pero invisibles a ojo: los mensajes de
"biblioteca vacía"/"cola vacía" nunca pasaban por `clip()` y con el panel
fijo en 30 celdas ensanchaban su propio panel empujando a los de al lado
(90×24 renderizaba 94 columnas) — misma clase que la 1.12.1, `panel()`
rellena pero NO acorta; el input de filtro seguía dimensionado a
`m.width/2 - 6` y sin `Width` correcto bubbles no hace scroll horizontal,
así que emitía el valor entero (una fila de 253 celdas en una terminal de
160), exactamente el defecto que la 1.12.1 arregló en la consola; y la
carátula se achataba cuando el límite era el alto, porque se recortaba solo
`artH` sin seguir con `artW`. El test que los cazó es el que comprueba que
TODA fila de `View()` mide exactamente el ancho de la terminal, en 10
tamaños × viz × banner.

Dos invariantes nuevas que muerden si se olvidan: la columna y la capa
ctrl+t **cachean la misma carátula a tamaños distintos**, y en kitty
comparten un único id de imagen (`kittyImgID`), así que al cruzar entre vista
y capa hay que invalidar AMBOS renders (`invalidateArt`) o los placeholders
del que se dibuja apuntan a una imagen transmitida para otras dimensiones de
celda; y `applyStatus` carga carátula y letras también cuando la columna está
visible (`npColumnVisible`), no solo con `npOpen`.

Verificación: cada pieza con lógica o estado nuevo se comprobó en ambas
direcciones, y una de esas comprobaciones falló primero por no compilar
—señal DÉBIL, la lección de la 1.6.1— y hubo que rehacerla para que fallara
por la razón correcta. La limpieza de títulos se validó además contra la
biblioteca real del dueño con un test desechable: 47 de 81 pistas mejoran y
ninguna pierde información real. Lo que NO se pudo verificar: kitty de
verdad (bajo tmux el renderer degrada a half-blocks por diseño, y desde una
sesión no gráfica no hay forma de conducir una ventana de kitty); queda
cubierto por el test de protocolo y uno nuevo que fija que la columna nunca
pide más de las 64 celdas por eje que admiten los placeholders, pero la
comprobación visual es del dueño.

Sobre la 1.14.0, la **1.14.1** (2026-08-15) corrigió lo único que se vio mal
al retratar la release en la terminal REAL del dueño (58 filas, casi el
doble que las 42 en las que se probó el relayout): la columna "Ahora suena"
quedaba con ~20 filas muertas entre la carátula y la ficha. La causa no era
el tope de altura de la carátula sino el ANCHO: la imagen es cuadrada, así
que su alto sale del ancho de la columna y no puede crecer para llenar un
panel alto. Se cerró por los dos lados — el bloque (carátula + ficha) va
CENTRADO en vertical en vez de anclado al fondo, y `npMaxW` sube de 32 a 40
para que en pantallas anchas la carátula gane filas. Lección repetida: el
tamaño de terminal en el que se prueba un layout es parte del arnés, y 42
filas no representaban la pantalla del dueño.

La **1.15.0** (2026-08-16) agrega el **buscador de descargas de la TUI**
(`ctrl+g`): escribir una consulta, elegir entre los resultados de YouTube y
descargar el elegido, sin salir de la interfaz. Cierra una carencia real de
`maly get`, que baja **el primer resultado a ciegas** (`ytsearch1:`): si sale
el equivocado, el único recurso es borrar el archivo y reformular. Salió de
un análisis de factibilidad pedido por el dueño ANTES de tocar código, y ese
análisis refutó tres premisas de la idea original — vale la pena que queden
escritas, porque cada una habría llevado a construir mal:

- **No existía búsqueda remota en maly, y agregarla mueve una frontera.** El
  proyecto había esquivado TRES veces el parseo de la salida de yt-dlp (el
  diff de directorio, `%(playlist_index)02d` en el nombre,
  `%(playlist_title)s` como subdirectorio). La línea que se mantiene es "no
  parsear la salida HUMANA" —progreso, avisos, errores—, que es frágil y
  cambia sin aviso; `--dump-json` es su interfaz de MÁQUINA y consumirla es
  lo mismo que se hace con ffprobe en `internal/probe`. Lo que la filosofía
  sí prohíbe —que maly hable con YouTube por su cuenta— sigue sin pasar.
- **El comportamiento fzf en tiempo real es IMPOSIBLE acá.** fzf filtra un
  conjunto local ya materializado; cada consulta acá cuesta ~0,95 s de red y
  un proceso nuevo (medido, `ytsearch8`), o sea un yt-dlp por pulsación. De
  ahí que la pantalla sea de DOS FASES, y de ahí que el fuzzy sobre los
  resultados sea casi decorativo: si no está lo que buscas, reformulas la
  consulta, no filtras diez líneas que acabas de pedir.
- **La descarga NO pertenece al demonio.** La idea pedía "que la maneje el
  daemon", pero hoy no le pertenece: `getter` arma el comando y el CLIENTE
  lo corre (`cmd.Run` en la CLI, `tea.ExecProcess` en la TUI). Meterla ahí
  habría exigido inventar trabajos asíncronos, progreso por push,
  cancelación y errores parciales — un subsistema entero en un demonio cuyo
  `dispatch` es a propósito un switch plano bajo un solo mutex, y cuya única
  op larga (scan) ya costó tres mecanismos especiales. Habría sido el peor
  error posible del cambio.

Con eso, la implementación resultó pequeña: **cero cambios en el demonio,
cero en el protocolo IPC, cero dependencias nuevas** (`sahilm/fuzzy` y el
`textinput` de bubbles ya estaban; el parseo es `encoding/json`). Ocho de
los nueve pasos del flujo ya existían y solo hubo que conectarlos.

**`getter.Search` (`internal/getter/search.go`)** es la única pieza
genuinamente nueva. Tres detalles que costarían un rato redescubrir:

- **`--dump-json`, NO `--print` con separador.** La variante barata está
  ROTA de entrada: un resultado real de buscar "aurora runaway" es `AURORA -
  Runaway | Sub Español - Lyrics + (VIDEO OFICIAL) HD`, con el separador
  obvio dentro del título. `%(title)j` escapa el contenido pero el `|` sigue
  literal entre comillas. Lo encoda `TestSearchTitleConSeparadores`,
  verificado en ambas direcciones (con parseo por `|`, el título vuelve
  cortado). Se decodifican CUATRO campos de los ~50 que trae cada entrada:
  toda la superficie de acoplamiento con el formato de yt-dlp.
- **`cmd.WaitDelay` es lo que hace que el timeout SIRVA**, y no es evidente.
  `exec.CommandContext` mata al proceso lanzado, pero `cmd.Output()` sigue
  esperando a que se cierre el pipe de stdout, y cualquier hijo vivo lo
  mantiene abierto: proceso muerto y llamada colgada igual. Lo destapó el
  test, que sin esto tardaba los 5 s enteros del proceso falso PESE a la
  cancelación — o sea que el timeout no servía para nada.
- **Quién venció importa.** `Search` conserva el `parent` context: si el
  llamador canceló (cerrar la pantalla) o su deadline era más corto, vuelve
  SU error y no el mensaje "tras 20 s" de maly, que ahí sería mentira. Sin
  esa distinción la pantalla no podría separar "esc del usuario" de "se cayó
  la red". La URL se toma de yt-dlp (`url` del JSON) en vez de armarse desde
  el `id`: maly no tiene por qué conocer la forma de una URL de YouTube, y
  el prefijo `https://` de regalo garantiza que no empiece con guion.

Los títulos y canales son la **SEGUNDA frontera de ingesta de texto ajeno**
del proyecto (la primera fue el título de playlist de `get playlist`,
1.11.0) y la más expuesta: acá el texto llega con solo BUSCAR, sin descargar
nada. Van por `safetext.Clean`, y sin él el PoC de siempre pasa entero.

**La pantalla (`internal/tui/get.go`)** clona el tríptico de ctrl+o
(`openGet`/`handleGetKey`/`getView` + rama en las dos cadenas de `View()` y
`handleKey`, cerrando `showHelp` al abrir por el hallazgo T1). Dos
decisiones que aparecieron implementando y no estaban en el plan:

- **`enter` tenía dos significados en el mismo estado.** Con resultados en
  pantalla, editar la consulta y pulsar enter debería BUSCAR, pero enter era
  "descargar": corregir una palabra disparaba una descarga que nadie pidió.
  Se resuelve con `getStale()`, DERIVADO (`texto ≠ consulta buscada`) en vez
  de un flag que habría que acordarse de bajar en cada camino, y el hint del
  pie dice cuál de los dos significados está activo. El revert lo confirmó:
  sin ese chequeo, editar + enter cierra la pantalla descargando.
- **Dos knobs genéricos en `picker`**, ambos con valor cero = comportamiento
  de siempre: `noFilter` (el input es caja de CONSULTA, no filtro — filtrar
  localmente chocaría con que enter re-busque) y `emptyText` (`sel.none` y
  `sel.none_empty` hablan de la biblioteca y acá no aplican).

`esc` no solo descarta la respuesta por generación: también cancela el
contexto y mata el yt-dlp en vuelo. El contador `getGen` cubre la única
carrera real —dos enter seguidos pueden resolver en orden inverso— con el
mismo patrón que `loadGen` en `internal/player`. El reloj del spinner se
autocancela fuera del estado "buscando", en la línea de la 1.7.2.

**Lección de verificación, y es nueva:** un test mío NO servía y hubo que
rehacerlo dos veces. El del salto de línea en el cuerpo del panel (los
errores de `getter.Tools()` traen la instrucción de instalación en una
SEGUNDA línea, y el cuerpo es UNA fila) pasaba con y sin el arreglo. Primero
porque usé un error con primera línea larguísima, que `clip` corta ANTES de
llegar al salto — el error real tiene la primera línea corta. Y aun con el
caso correcto seguía pasando, porque las aserciones medían "no se pasa del
ancho" y la corrupción real es una fila MÁS ANGOSTA: el `\n` parte la fila y
deja la primera mitad sin borde derecho. La aserción buena es que la caja
sea un RECTÁNGULO — toda fila mide exactamente lo mismo (`boxLinesSameWidth`,
complementaria a `boxLinesFitWithin`, que solo ve el desborde). Es la
tercera vez que este defecto aparece (consola 1.12.1, paneles vacíos
1.14.0), y la primera en que el chequeo queda con la forma correcta.

Probado en vivo bajo tmux con sandbox XDG: búsqueda ~1 s / 10 resultados,
spinner animando, el hint cambiando al editar, descarga del resultado
SELECCIONADO (verificado porque el `.webp` intermedio llevaba el título del
2.º resultado, que era el elegido con un ↓) y la cadena cerrando hasta
`Biblioteca (1) · 1/1` con la pista en el árbol. La alineación con CJK se
midió con una implementación de ancho INDEPENDIENTE de la de lipgloss
(`east_asian_width` sobre lo que capturó tmux): las 15 filas de la caja,
exactamente 80 celdas, incluidas `supercell / 君の知らない物語` y un canal que
es un carácter combinante.

No se hizo, y es deliberado: entrada en `ConsoleCommands` (es una PANTALLA,
no un comando — ctrl+o y ctrl+t tampoco figuran ahí, y la consola nunca fue
espejo estricto) y mención en el pie de la TUI (que ya perdía `? ayuda` /
`q salir` primero en español por ser más largo — hallazgo D10.3/T23; la
tecla vive en la ayuda `?`).

Sobre la 1.15.0, el **criterio de éxito de una descarga deja de ser el
código de salida de yt-dlp** (`internal/getter/diff.go`, con los helpers de
diff que estaban duplicados entre `cmd/maly` y `internal/tui`). Y acá hay una
CORRECCIÓN que importa más que el arreglo: durante la prueba en vivo de la
1.15.0 se concluyó que "yt-dlp sale con código 0 aunque la descarga falle", y
**eso es falso**. La medición que lo sostenía estaba mal hecha dos veces — se
leyó el código de salida de una corrida que en realidad HABÍA bajado el mp3
(nunca se listaron los archivos resultantes), y al intentar confirmarlo se
midió `$?` después de un pipe, o sea el estado de `tail` y no el de yt-dlp.
El cuadro real, medido caso por caso y sin pipes:

	descarga correcta       → 0, con el mp3
	HTTP 403                → 1, dejando SOLO la miniatura .webp
	video inexistente       → 1, sin dejar nada
	búsqueda sin resultados → 0, SIN DEJAR NADA

O sea que el código de salida acierta en los fallos de DESCARGA; el agujero
real es más estrecho y sigue siendo real: una consulta que no encuentra nada
—un typo basta— sale 0, y `maly get "algo mal escrito"` anunciaba "Descarga
lista — actualizando la biblioteca" y cerraba con "Listo: 0 nuevas" y "La
biblioteca está vacía", sin decir en ningún momento que no había encontrado
la canción. Verificado con un A/B contra el binario de HEAD.

Del 403 salió el segundo arreglo: yt-dlp baja la miniatura ANTES del audio,
así que un fallo deja un `.webp` huérfano en `music_dir` con cada intento —
y ese camino (código ≠ 0) volvía sin tocar nada. `getter.Cleanup` lo limpia
en los DOS caminos, y solo toca archivos que cumplen las tres condiciones:
no existían antes de esta descarga, están en el primer nivel del destino, y
su extensión es de intermedio conocido (`.webp`, `.part`…). Sin las tres,
sería un borrador de archivos del usuario con pasos extra.

La lección de método, que es lo que de verdad deja este ciclo: **medir el
código de salida sin verificar el efecto observable no es medir**. Las dos
veces que falló el diagnóstico fue por dar por supuesto lo que no se miró —
los archivos que quedaron, y de qué proceso venía el `$?`.



La **1.16.0** (2026-08-16) cierra los tres candidatos que la 1.15.0 había
dejado anotados como opcionales, más la corrección de unos comentarios que
afirmaban lo que la propia 1.15.0 ya había refutado (el doc de `NewAudio`
conservaba "yt-dlp sale con código 0 pase lo que pase" y remitía a un `Count`
inexistente). Ninguna toca el demonio ni el protocolo.

**Lo que ya tienes sale marcado** (`✓` en `getItems`, `internal/tui/get.go`).
La decisión que importa es la CLAVE de comparación: título limpio y plegado
**sin el artista**. El uploader de YouTube casi nunca es el artista real —una
descarga de supercell puede quedar acreditada a "LumenAster23"—, así que
incluirlo daría falsos negativos prácticamente siempre; `cleanTitle` además
iguala el ruido de yt-dlp, con lo que una pista ya bajada y su propio
resultado remoto producen la MISMA cadena. Verificado en ambas direcciones
INCLUIDA la variante con artista, que es el error fácil. Es una PISTA y no un
bloqueo: un cover legítimo con el mismo título saldrá marcado y descargarlo
sigue estando a un enter — falso positivo barato, falso negativo caro. El
marcador va delante con hueco equivalente en los no marcados, porque el ancho
escasea (los títulos ya se recortan). El conjunto sale del árbol que la TUI
ya tiene cargado —ni una consulta más a la base— y se recalcula por
respuesta, no se cachea, para que una descarga hecha entre dos búsquedas de
la misma sesión aparezca marcada.

**Lives y estrenos fuera** (`notLive`, `internal/getter/search.go`).
`live_status` llega gratis en el `--dump-json` que ya se decodifica, pero
solo se descartan DOS de sus cinco valores: `is_live` e `is_upcoming`.
`was_live` y `post_live` son la GRABACIÓN de un directo que ya terminó y eso
es audio perfectamente descargable —muchísimos conciertos viven así—, así que
la variante ingenua ("todo lo que no sea `not_live`") se llevaría por delante
material real; el test la ejercita explícitamente. Un valor desconocido se
conserva: la lista dice qué se tira, no qué se admite.

**`maly get pick <búsqueda>`** lleva el ctrl+g a la línea de comandos.
Subcomando y no flag porque la CLI no tiene parser de flags —`-v`/`-l`/`-h`
son COMANDOS—, siguiendo el precedente de `get playlist`. Lo barato viene de
partir `runGet` en resolver-el-spec y **`downloadOne`**, que es el camino
común de las dos formas y ya traía todo lo de la 1.15.0 (snapshot,
verificación por diff, limpieza de intermedios en los dos caminos de fallo,
re-escaneo y el nombre de lo bajado): la rama `pick` solo aporta la URL
elegida. `tui.RunGetPick` clona el patrón de `RunSelect` con una diferencia
deliberada: **NO exige demonio**. `RunSelect` lo pide porque reproduce; acá
descargar no pasa por él salvo el re-escaneo final, que ya degrada solo a
escribir directo en la DB, y copiar ese chequeo le negaría la descarga a
quien no tenga el servicio levantado. La biblioteca se abre solo si existe
(nada de fabricar una base vacía, misma regla que `info`/`doctor`) y un fallo
al leerla no es fatal: sin ella simplemente no se marca nada.

Dos redes de seguridad del propio proyecto saltaron en esta tanda y las dos
tenían razón: `TestUsageCabeEnColumna` rechazó el usage largo que había
escrito para `get` (la columna del help es de 28, y los subcomandos de `get`
nunca estuvieron ahí — `playlist` tampoco), y el test fijo de completions
obligó a declarar los dos subcomandos a mano, que es exactamente el aviso que
se busca. Quedan compartidos entre las dos pantallas `pickedResult` (los
ítems guardan el ÍNDICE, no la URL) y `oneLine`.

Probado en vivo: `maly get pick` bajo tmux con búsqueda real, elección con ↓,
descarga, y el `✓` apareciendo al repetir la misma búsqueda con la pista ya
en la biblioteca.

No se hizo, y sigue siendo deliberado: entrada de `pick` en
`ConsoleCommands` (la consola ya tiene ctrl+g, que es mejor que escribir un
comando) y bloquear la descarga de algo ya marcado.


La **1.16.1** (2026-08-16) contesta en el buscador la pregunta que ni el
título ni el canal contestaban: **cuál es la subida canónica**. `view_count`
ya venía en el `--dump-json` que se decodifica, así que no hay red nueva ni
proceso extra — sexto campo de los ~50. El caso apareció en la PRIMERA
búsqueda real de la prueba en vivo: `AURORA — Runaway [04:10 · 808M]` y
`AURORA — Runaway [04:10 · 423K]`, mismo canal, mismo título, misma
duración; sin el dato son indistinguibles.

Tres decisiones que conviene no re-descubrir:

- **`ViewCount` se decodifica como `float64`, no como `int64`**, igual que
  `Duration`. Con `int64`, una entrada en notación exponencial rompe el
  `Decode` y —como `decodeResults` corta el bucle ante un objeto ilegible— se
  pierden TODOS los resultados posteriores. Verificado revirtiéndolo: 1 de 4.
- **Sin decimales y sin la palabra "visitas"**, y las dos por el mismo
  motivo: un decimal obligaría a elegir separador (`6,6M` o `6.6M`) y la
  palabra obligaría a i18n, y ambas se comerían celdas de título en una fila
  que ya se recorta. Se trunca en vez de redondear para que 999999 dé `999K`
  y no `1000K`. Duración y visitas comparten UN corchete: dos costarían tres
  celdas más por fila.
- **`channel_is_verified` se descartó**, aunque estaba sobre la mesa. No hay
  glifo libre —el `✓` ya es el de "ya lo tienes" y `pickerItem.label` es una
  sola cadena que el picker pinta con un solo estilo, así que no se pueden
  colorear trozos— y en la práctica es redundante con las visitas y con el
  nombre del canal, que ya se muestra.

El **ancho** se resuelve partiendo `pickerWidth` en `pickerWidthMax(termW,
max)`: solo las dos pantallas de búsqueda piden 140, por ser los únicos
pickers cuyos ítems son texto AJENO y largo. Los otros tres no cambian una
celda, y por debajo de 150 columnas tampoco cambia nada porque manda la regla
de los dos tercios — o sea que nadie que hoy esté cómodo nota la diferencia.
Medido a 190 columnas: la caja pasa de 100 a 126 celdas y ninguno de los diez
títulos queda truncado, con la caja comprobada como rectángulo por una
implementación de ancho independiente de la de lipgloss.

Las miniaturas y la pantalla completa quedan **descartadas**, no aplazadas
(el dueño lo decidió el mismo día, con las visitas ya en uso). Bajar una
miniatura sería la primera petición de red propia de la historia de maly, y
de ahí salió el invariante que ahora encabeza las decisiones transversales:
yt-dlp es la única frontera con lo online. La pantalla completa caía con
ellas — solo tenía sentido para dar sitio a una imagen. Si la duda vuelve, lo
que hay que releer es el invariante, no esta entrada.


La **1.16.2** (2026-08-17) tiene una pieza con sustancia y tres arreglos
chicos. Todos salen de repasar los P3 que la auditoría de UX de la 1.12.0
dejó marcados para un ciclo aparte (ver "Post-1.0"), más un reporte del
dueño sobre la ayuda de la CLI.

**La sección de atajos de `maly -h` sale ahora de `tui.HelpRows`**, la MISMA
lista que pinta el modal `?` (`internal/tui/helprows.go`). Era una copia a
mano, con las teclas literales, y falló de las dos maneras en que una copia
falla: se quedó atrás —`ctrl+g`, el buscador de descargas de la 1.15.0, nunca
llegó a aparecer, y con él tampoco volumen, seek, mover en la cola ni
shuffle/repeat: mostraba 11 filas de 22— y mentía, porque anunciaba los
defaults aunque el usuario tuviera un preset de `controls` o teclas propias
en `[keys]`. La lista compartida es el mismo patrón que la tabla de comandos
(fuente única de dispatch, help y completions) y la red de paridad
consola↔CLI: un atajo nuevo aparece en los dos lados o en ninguno.

Tres detalles que conviene no re-descubrir. `helpKeys()` resuelve las teclas
con `config.Load()`, y eso NO agrega ningún efecto sobre el disco: `main()`
ya la llama en cada invocación para fijar el idioma, y su retorno con nombre
+ defer garantiza teclas resueltas incluso si el config falla. `keyLabel`
sustituye el espacio por su nombre POR TECLA y no por fila ya compuesta,
así que también cubre el espacio como segunda mitad de un par (la versión
vieja, dentro de `helpView`, solo miraba el prefijo `" / "`). Y el ancho de
la columna se calcula con `lipgloss.Width`, no con `%-14s`: el fijo contaba
BYTES y descuadraba con cualquier tecla no ASCII, además de quedarse corto
con `pgup/pgdn home/end`, que es la fila que ya había mordido al modal.

La red que impide la próxima desincronización es
`TestHelpRowsCubreTodasLasTeclas`: toda acción de `config.DefaultKeys()`
—salvo `help`, que es el modal mostrándose a sí mismo— tiene que tener fila
en `HelpRows`. Mapea cada acción a un valor ÚNICO (`<<accion>>`) en vez de a
su tecla real, porque con las de verdad una acción sin fila puede "pasar" de
casualidad: hay teclas compartidas (`K`/`J`, `+`/`-`) y letras sueltas que
aparecen dentro de otras cadenas.

Los tres chicos, cada uno un P3 con impacto real:

- **C24** — `maly search` abría con `openLibrary`, que CREA la base: una
  consulta sin haber escaneado nunca dejaba en disco una biblioteca vacía
  que nadie pidió. Pasa a `openLibraryIfExists` y, sin base, responde con
  `cli.search_none`, el mensaje que ya existía y que además remite a `maly
  scan` — no hizo falta clave nueva.
- **G7** — con nombre explícito, `get playlist` crea el destino ANTES de
  invocar a yt-dlp (es su `-o`), así que cada intento fallido dejaba un
  directorio vacío huérfano en `music_dir`. Se limpia solo el que creó ESA
  corrida (`os.Stat` antes del `MkdirAll`) y con `os.Remove`, que se niega si
  no está vacío: no puede llevarse música del usuario por delante. El espejo
  de la TUI necesitó llevar el flag en `getPlaylistDoneMsg`, porque
  `ExecProcess` parte el flujo en dos funciones.
- **C26** — agregar a una playlist inexistente decía solo "no existe". El
  remedio va ahora en el mismo mensaje, y la detección es por TIPO
  (`library.ErrPlaylistNotFound` + `PlaylistNotFound`, cuyo `Error()` se
  traduce al imprimirlo) y no por el texto, que sale de i18n y cambia con el
  idioma — la trampa que la 1.5.0 ya había dejado anotada con el retry de
  `player.seek`. Va en UNA sola línea a propósito: el espejo de la consola lo
  pinta dentro de un panel, y un salto de línea ahí parte la caja (el defecto
  que costó dos intentos de test en la 1.15.0).

Cerrados sin código, verificando contra HEAD que ya no aplicaban: **C23**
(`maly config` sí está en ambos README desde D13.1), **D10.5** (las claves
`cli.logo*` ya las usa un comando CLI real desde C21) y **D13.5** (la
detección de idioma de D10.1 hace que la primera salida salga en el idioma
del sistema). **C25**, **G8** y **D7.6** siguen como "no cambiar", que es lo
que la propia auditoría recomendaba.

Cada arreglo con lógica nueva se verificó en ambas direcciones, y dos de las
reversiones fallaron primero por NO COMPILAR (variable sin usar, import sin
usar) — señal débil, la lección de la 1.6.1: hubo que rehacerlas quitando
también lo que sobraba para que el test fallara por la razón correcta. De
paso, un `git checkout` usado para restaurar una de esas reversiones se
llevó por delante el fix que ya estaba en el archivo; el arnés de
verificación conviene que sea `cp` de una copia, no `git checkout`, mientras
haya cambios sin commitear.

La **1.16.3** (2026-08-20) cierra **una carrera real entre `advance()` y los
mutadores de la cola** que perdía una pista entera. Salió de una pregunta del
dueño —"¿puede `advance()` avanzar usando una promesa ya obsoleta?"— y la
respuesta era que sí.

`resolveEnd` (`player.go`) captura `chained` bajo `p.mu` y despacha el
callback con `go` **sin sostener ningún lock**; `advance` recién toma `d.mu`
después. En esa ventana cualquier `dispatch` puede ganar el mutex y emitir un
`loadfile replace`. La guarda que había —`chained == PeekNext().Path`— compara
la promesa vieja contra la **cola**, no contra lo que mpv reproduce, así que un
`jump` al índice que YA era el actual (recarga mpv pero deja `PeekNext`
idéntico) matcheaba por coincidencia y confirmaba un encadenado que la recarga
ya había anulado.

La intercalación, con cola `[a b c]` sonando a y b anexada: a termina, mpv
encadena a b, `resolveEnd` devuelve `chained="b"` y despacha el callback;
antes de que corra, llega `jump 0`, que recarga a y rearma la ventana con b;
entonces corre el callback, `PeekNext` sigue siendo b, matchea, y `q.Next(true)`
mueve `Index` a b. Resultado medido con mpv real: **mpv reproduce a mientras la
cola dice b**, y el `syncWindowLocked` del final saca a b de la playlist de
mpv — al terminar a, mpv encadena a c y **b no suena nunca**. La desincronía
además llega a disco: `notify()` llama a `learnDuration()` de entrada, que
cruza `q.Current()` (=b) con `d.pl.State()` (=la duración de a) y termina en
`d.lib.SetDuration(b, duraciónDeA)`, fuera de `d.mu`. Los tres efectos
—desincronía, pista perdida, duración corrupta en SQLite— se reprodujeron por
separado antes de tocar nada.

El arreglo usa el mecanismo que el proyecto ya tenía, extendido al tramo que le
faltaba: `loadGen` existía justamente para esto (*"los desenlaces que siguen son
de esta carga"*) pero se leía en UN solo lugar, `resolveEnd`, que corre ANTES
del `go` — cubría `[end-file → start-file/idle]` y no `[resolveEnd → advance
toma d.mu]`. Ahora `resolveEnd` **devuelve** la generación, el callback la
lleva (`onEnd(reason, next, gen)`, `advance(reason, chained, gen)`) y `advance`
la revalida contra `LoadGen()` como primer chequeo bajo `d.mu`, saliendo con
`skipNotify` igual que el eco tras stop (el mutador que subió la generación ya
realineó y notificó por su propio `handle()`). Las dos guardas quedan vivas y
son complementarias: la de generación ataja lo que RECARGA (jump, play, next,
stop) y la de ruta lo que muta SIN recargar (move, shuffle, remove).

Se descartó la alternativa de comparar contra `p.CurrentPath()` —que existe y
se documenta como "la verdad viva"— porque es un round-trip IPC a mpv con
`d.mu` tomado, justo lo que las tres excepciones de "antes de `d.mu`" existen
para evitar. De paso quedó anotado que esa función **no tiene ningún llamador
de producción**: su único uso en el repo es un test.

Dos cosas que este ciclo deja como lección de método:

- **`go test -race` es CIEGO a esta clase de defecto.** No hay ningún acceso a
  memoria sin sincronizar: la cola siempre bajo `d.mu`, el espejo siempre bajo
  `p.mu`. Lo que viaja mal es un VALOR obsoleto cruzando entre dos dominios de
  lock, y el detector no puede verlo. Confirmado corriendo `-race` sobre los
  tests que fallaban: ni un `DATA RACE`. O sea que el job `race` de CI no lo
  habría encontrado ni con `daemon` en su lista.
- **Una medición negativa sin verificar su precondición no es una medición.**
  El primer intento de reproducir la corrupción de duración dio negativo y casi
  se descarta la hipótesis; la causa era que el test llamaba a `advance` antes
  de que mpv publicara `duration`, y `learnDuration` corta con
  `st.Duration <= 0`. Con un `waitStatus` esperando el dato, reproduce. Es la
  misma trampa de la 1.15.0, esta vez del lado del arnés.

Los seis tests viven en `internal/daemon/daemon_race_test.go`: tres fijan el
defecto (jump, pista perdida, duración corrupta), `TestAdvanceObsoletoTrasNext`
y `TestAdvanceObsoletoTrasMove` ejercitan una guarda cada una —el de move es
el que impide que la de ruta quede sin nadie que la pruebe, porque `move` no
mueve `loadGen`— y `TestAdvanceGeneracionVigenteAvanza` fija el camino feliz.
La reversión para verificar se hizo neutralizando **solo el cuerpo del
chequeo** y dejando la firma: un revert que quite el parámetro no compila, y
eso es señal DÉBIL (la lección de la 1.6.1, que ya mordió dos veces en la
1.16.2). `TestGaplessChain` y `TestGaplessRepeatOne` siguen en verde: son la red
del camino feliz con mpv real.

Fuera de alcance a propósito: el **append duplicado** (el `end-file` baja
`nextKnown` y desarma el guard de no-op de `SetNext`, así que un mutador en esa
ventana re-anexa la pista que mpv ya está encadenando) se auto-repara en el
`syncWindowLocked` siguiente y solo sería audible si esa pista falla al
instante, donde `errStreak` corta igual; y el **reordenamiento de dos `advance`
concurrentes**, que capturan la MISMA generación y por tanto el chequeo nuevo
no los separa — peor caso medido, una recarga de más.

La **1.16.4** (2026-09-05) es la **Phase 0** de una auditoría técnica y
arquitectónica completa sobre la 1.16.3 (35 hallazgos: 1 CRITICAL, 5 HIGH,
11 MEDIUM, 11 LOW y 8 oportunidades; informe aparte en `~/Audits/MalyAu/`,
con una sección **KEEP** de 48 puntos y una **Explicitly Don't Do** de 18).
Tres ítems, los únicos marcados "fix now". El resto queda repartido en
Phase 1/2/3 y **no** entra acá.

**A-01 (CRITICAL) — un escaneo con el `music_dir` vacío borraba la
biblioteca y VACIABA TODAS LAS PLAYLISTS.** `Scan` construía `seen` y
purgaba de la base toda entrada bajo `root` que no estuviera ahí, sin
ninguna guarda sobre el TAMAÑO del borrado. Un `root` que existe pero está
vacío —el punto de montaje de un disco externo sin montar, un NFS/SSHFS
caído, un `music_dir` recién cambiado (A-03), o el `os.MkdirAll` que
`maly get` hace antes de bajar nada— purgaba las 4.000 filas de una. Y
como `playlist_tracks` tiene `ON DELETE CASCADE` sobre `tracks(id)` con
las claves foráneas activas, el borrado se llevaba el CONTENIDO de todas
las playlists: lo único que el usuario arma a mano y que maly no puede
reconstruir de ningún sitio — el mismo argumento con el que C15/T26 le
puso confirmación a `playlist delete` y a `ctrl+x`. Era la única pérdida
de datos irreversible del proyecto.

La guarda es la mínima de las tres que proponía la auditoría (decisión del
dueño): si el walk no vio NI UN archivo de audio y `countUnderRoot` dice
que la base sí tiene pistas ahí, no se purga y se devuelve `*ScanEmpty`.
Tres cosas que conviene no re-descubrir:

- **Va DESPUÉS del walk, no antes.** Un `root` que existe pero está vacío
  es indistinguible de uno montado hasta haberlo recorrido: el `os.Stat`
  del principio de `Scan` dice que sí, que es un directorio.
- **`countUnderRoot` reusa `underRoot`**, el mismo criterio con el que
  `purgeGone` decide a quién borra. Contar con otro criterio dejaría un
  hueco justo donde importa.
- **El error va por TIPO** (`ErrScanEmpty` + `ScanEmpty`, cuyo `Error()`
  se traduce al imprimirlo), no por texto, así que el demonio lo reconoce
  con `errors.As` y lo re-traduce con `TLf` al idioma del CLIENTE —
  `Error()` ya lo formó, pero con el idioma global del proceso, que es el
  del demonio y no el de quien preguntó. Mismo patrón que
  `ErrPlaylistNotFound` (1.16.2, C26). El mensaje va en UNA línea: el
  espejo de la consola lo dibuja dentro de un panel.

**Sin escape, a propósito** (decisión del dueño, y la que la auditoría
recomendaba): ni subcomando `scan force` ni clave de config. maly no tiene
parser de flags, un subcomando costaría tabla de comandos + completions +
ayuda + espejo de la consola + i18n, y una clave se pone una vez y se
olvida — justo lo contrario de una confirmación puntual. Quien de verdad
quiera vaciar la biblioteca borra `library.db`, y el propio mensaje se lo
dice.

Un test EXISTENTE cazó el borde de la guarda antes que ningún test nuevo:
`TestScanPurgeDotDotDir` quitaba la única pista bajo `root` para comprobar
el filtro `..covers/` de la purga, que bajo la guarda nueva es exactamente
el caso de refusal. El fixture ganó una pista que sobrevive, así que ahora
mide el FILTRO con el árbol presente, que es lo que quería medir. Es la
segunda vez que la suite completa (y no el test dirigido) encuentra el
efecto colateral — la primera fue `TestLoadLogoSane` en la 1.13.0.

**A-02 (HIGH) — la TUI se negaba a abrir mientras el demonio arrancaba.**
`runTUI` preguntaba con `ipc.Ping` y, ante `ErrAlreadyRunning`, devolvía el
error. Pero el ping es justo la heurística que el proyecto declaró
insuficiente al introducir el flock: *"un demonio ocupado arrancando
(esperando hasta 5 s a mpv) tampoco contesta al ping aunque esté vivo"*
(comentario de `acquireLock`). O sea que `daemon.New` ya tenía la respuesta
buena —el kernel dice que hay otro— y `runTUI` la tiraba. Escenarios
cotidianos: `maly.service` compitiendo con el usuario en los primeros
segundos del login, y dos `maly` lanzados a la vez. Y el mensaje engañaba:
"ya hay otro demonio corriendo" describe bien el estado del sistema y mal
el del usuario, para quien es exactamente la razón por la que la TUI NO
debería fallar.

`startOrAttach` (extraído de `runTUI` para poder testearlo sin pasar por
bubbletea, mismo motivo por el que `embeddedStartErr` ya vivía aparte) cae
a modo cliente: `waitForDaemon` sondea el socket hasta
`daemonStartupBudget` (8 s = el techo de 5 s de `player.Start` esperando a
mpv, más margen para abrir la base, reponer la sesión y registrar MPRIS),
cada 200 ms. Silencio si contesta rápido —el caso normal— y una línea al
stderr pasado un segundo, para que varios segundos de terminal mudo no
parezcan un cuelgue; si vence, el mensaje dice lo que de verdad pasó
("arrancando y no respondió en 8 s"), no "ya hay otro demonio".

**A-03 (mitad barata, HIGH) — `maly scan` anunciaba una ruta y escaneaba
otra.** `daemon.New(cfg)` guarda una copia del config y no vuelve a
mirarlo jamás, así que `maly scan` sin argumentos mandaba `Query: ""` y el
demonio resolvía con `d.cfg.ScanTarget("")`, o sea con el `music_dir` que
tenía al ARRANCAR — mientras el cliente imprimía `cli.scan_start` con el
que acababa de leer del disco. Con `maly.service` habilitado el demonio
vive semanas, así que la ventana es el caso normal y no uno raro. Ahora el
cliente manda siempre la ruta ya RESUELTA (`runScan` y `conScan`), como ya
hacía `get playlist`, y el demonio deja de tener voz. La mitad estructural
—que el demonio relea el config— es Phase 2 y no entra acá.

La consecuencia que no es obvia: con la ruta siempre explícita, el demonio
ya no puede formar el mensaje de "esa ruta no existe" (no sabe de dónde
salió ni si era explícita), así que lo forma el cliente ANTES de dialar.
Para no duplicarlo entre los dos espejos vive en `config.ScanNoExistErr`,
junto a `ScanTarget` — que ya devolvía `originKey` SOLO para ese mensaje.
El `!explicit` de `daemon_scan.go` se conserva igual: un cliente de una
versión anterior (binarios desparejados a mitad de una actualización)
sigue mandando `Query: ""`.

Verificación. Cada arreglo con lógica nueva lleva test en ambas
direcciones, con la reversión hecha por `cp` de una copia y no por `git
checkout` (la trampa de la 1.16.2). Los tres fallaron por la razón
correcta, COMPILANDO — la lección de la 1.6.1 —, y en A-02 hizo falta un
segundo intento: la primera reversión dejaba el `case` de
`ErrAlreadyRunning` en pie con la llamada anulada, y el test fallaba con el
mensaje de timeout NUEVO en vez del de HEAD; recién quitando el `case`
entero reprodujo `another maly daemon is already running (socket: …)`, que
es lo que la auditoría transcribió. Además, la reversión de A-01 se
completó con un test desechable que midió el efecto observable y no solo el
error: biblioteca 4→0 y playlist 4→0, exactamente la reproducción del
informe. Y A-03 se comprobó con un A/B contra un binario compilado de HEAD
(`git archive`), porque su síntoma es silencioso: HEAD anuncia `musicB` e
indexa `musicA`. En vivo, bajo tmux con sandbox XDG, la TUI esperó al
demonio que aparecía a mitad de la espera y abrió en modo CLIENTE
(verificado porque el demonio siguió vivo tras cerrarla, no porque la TUI
se dibujara). `go build`, `go vet`, `gofmt -l .`, `go test ./...` y
`go test -race ./...` limpios.

Se agregó de más, y a propósito, un test que la auditoría no pedía:
`TestConScanMandaLaRutaResuelta`, el espejo de `TestScanMandaLaRutaResuelta`
en la consola. Es la clase exacta de A-04 —tres arreglos publicados que
sobrevivieron en un solo lado por no tener test del otro—, y el arreglo de
A-03 toca los dos espejos.

### Post-1.0 (candidatos)

La lista, que la 1.5.0 había dejado vacía, la reabrió la auditoría del
2026-07-21.

El hallazgo **#4** (IO no acotado dentro de `d.mu`) se **midió** antes de
tocarlo, y la medición REFUTÓ la hipótesis del informe. Números con 40.000
pistas: un `search` de la biblioteca entera retiene `d.mu` **96 ms** (lineal:
5k→10 ms, 20k→48 ms) y bloquea otro tanto a un `status` concurrente. La
hipótesis era que la contención por la ÚNICA conexión SQLite —`d.mu` →
conexión, con el scan ocupándola— disparase eso a segundos: **con un scan
reescribiendo las 40k filas a la vez, el peor `search` fue 112 ms**, o sea
+16 ms. Los lotes de 500 del scan hacen justo lo que promete su comentario.
La severidad baja de Media a **Baja**, y se decidió NO sacar `search` ni
`playlist_play` de `d.mu`: la consulta vacía (la única que recorre la
biblioteca entera) no es alcanzable POR EL DEMONIO —`maly search` y el
`search` de la consola exigen argumentos—, `play`/`add` ya resuelven fuera
del lock desde la 1.1.5, y `playlist_play` opera sobre listas curadas a mano.
Reestructurar `dispatch` otra vez no compensa por ~100 ms en un caso que el
protocolo no expone.

Corrección de la 1.7.3: esa consulta vacía SÍ era alcanzable, por otro lado
—el completado del shell, que no pasa por el demonio y por tanto nunca tocó
`d.mu`—, y ahí se pagaba en cada TAB. Arreglado con `SearchLimit` (ver la
1.7.3); el razonamiento de arriba sobre `d.mu` no cambia.

Lo que sí se cerró es la única pieza que era una ESCRITURA y se disparaba
sola: `learnDuration` hacía su `SetDuration` con `d.mu` tomado, en cada
cambio de pista. Ahora captura ruta y duración, suelta el mutex y escribe
fuera. La guarda contra escrituras repetidas sigue siendo la copia en
memoria de la cola, que se actualiza bajo el lock.

Los menores se cerraron en una cuarta tanda: **#5** (`main` valida el runtime
dir con `EnsureRuntimeDir` antes del dispatch — un solo punto cubre los
catorce sitios que usan `SocketPath`, porque todos cuelgan de ahí), **#8**
(la barra de progreso estaba duplicada en `nowPanel` y `npMeta` y le faltaba
la guarda INFERIOR: con `Duration` diminuta el cociente desborda a `+Inf` y
`int(+Inf)` da el mínimo de int64, que llegaba a `strings.Repeat` y lo hacía
entrar en pánico; ahora es `Model.progressBar`, fuente única), **#10**
(`ExportM3U` con `O_NOFOLLOW` y 0600), **#11** (`saveKey` por tmp+rename),
**#12** (cota en `ParseLRC` y en `loadLogoArt`) y **#13** (`doClose` espera
al scan en vuelo, acotado a 5 s).

**Cerrado por documentación, sin código** —mismo criterio que con #4—: el
tope de conexiones concurrentes y la cota de `req.Paths` (#12), y el rollback
de las mutaciones de `dispatch` ante fallo del player (#13). El atacante de
esos vectores es del mismo UID, o sea que ya tiene la cuenta, y el rollback
exigiría tocar `dispatch` por tercera vez. Tampoco se toca el "lost update"
de dos TUIs guardando config a la vez.

Dos cambios de comportamiento que conviene recordar: con un runtime dir no
fiable **fallan TODOS los comandos**, incluidos `help` y el `__complete` de
cada TAB (es lo buscado: solo pasa si algo va mal de verdad); y `playlist
export` ya **no escribe a través de un symlink** (solo afecta al componente
final de la ruta; un directorio enlazado sigue valiendo).

El ratón en la TUI sigue descartado.


**Latencia del aviso de `update`: ARREGLADO** (anotado y cerrado el
2026-07-21). Nunca fue un fallo —`maly update` funcionaba y el aviso salía—,
sino latencia: `updateCheckCmd` consultaba `update.Cached()` primero y, con
el cache fresco, ni preguntaba a la red, con un TTL de 24 h. Publicabas un
tag, mirabas al minuto y no estaba, mientras el resto de la TUI da feedback
en vivo (push, `libGen`, progreso de scan).

Ahora el cache se honra **solo cuando ya anuncia algo más nuevo**: si dice
que estás al día se pregunta igualmente, porque ese es justo el caso "el tag
se publicó después de guardar el cache". Verificado con un A/B: con la misma
`update.json` fresca, el código viejo no mostraba nada y el nuevo anuncia al
instante. El coste es un `ls-remote` por arranque estando al día, en
goroutine y mudo si falla.

**Confirmado en producción** al publicar la 1.6.1 (2026-07-22): el dueño abrió
la TUI antes de recompilar, con su cache real a 20,5 h de antigüedad —fresco
bajo el TTL de 24 h— diciendo `v1.6.0`, o sea "al día". Con el código anterior
no habría visto nada hasta que expirase el cache; con este, el aviso de
`v1.6.1` salió al momento. No es la prueba de un sandbox: es el escenario
exacto que motivó el arreglo, en la máquina del dueño.

La otra mitad, cerrada después: el chequeo ocurría **solo en `Init`**, así que
una TUI abierta días no volvía a mirar nunca. Ahora `updTickCmd` lo repite
cada `updRecheckEvery` (1 hora) y se re-arma, respetando la clave
`update_check`. Repetir es barato: cuando el cache ya anuncia algo,
`updateCheckCmd` resuelve sin tocar la red.

Sigue SIN tocarse, porque el dueño confirmó que no era la causa de lo que le
chirriaba: `verMismatch` tiene prioridad sobre `updAvail` en el `switch` del
footer (`view.go`), y cualquier flash, `connErr` o el progreso de scan también
lo pisan.

**P3 pendientes de la auditoría de UX de la 1.12.0**, elegidos por el dueño
para revisar en un ciclo aparte (no implementados; el resto de los 13 P3
originales quedan solo en el informe, sin marcar para revisión explícita).
La auditoría original agrupó estos diez en una sola fila resumen ("ver
secciones respectivas"), sin desarrollo individual — el detalle real vive
repartido en las secciones 04/05/07/10/13 del informe
(`~/Documents/maly-ux-audit.html`):

Los diez quedaron revisados el 2026-08-17: tres con código en la 1.16.2,
tres cerrados por estar ya resueltos de rebote y tres confirmados como "no
cambiar" (más D14.4, que se había cerrado antes). La lista completa, con el
desenlace de cada uno:

- **C23** — CERRADO sin código: `maly config` sí está en ambos README
  (`README.md`, `README.en.md`), lo cerró D13.1 en la 1.12.0. El hallazgo
  ya no aplicaba.
- **C24** — CERRADO con código en la 1.16.2 (ver su entrada).
- **C25** — NO CAMBIAR, confirmado: la cola tiene `move` y las playlists
  no, y la auditoría ya lo marcaba como asimetría consciente. Revisar era
  confirmar la decisión, no implementar `playlist move`.
- **C26** — CERRADO con código en la 1.16.2 (ver su entrada).
- **G7** — CERRADO con código en la 1.16.2 (ver su entrada).
- **G8** — NO CAMBIAR, confirmado: `yt-dlp failed: exit status 1 (see its
  output above)` es técnico, pero el paréntesis manda a la salida real de
  yt-dlp, que es donde está la causa. La auditoría ya decía "sin acción".
- **D7.6** — NO CAMBIAR, confirmado: `runDaemon` agrega `(socket: %s)` al
  error de "ya corriendo", y es el único contexto donde esa ruta es
  accionable.
- **D10.5** — CERRADO sin código: las claves `cli.logo*` ya las usa un
  comando CLI real (`runLogo`, registrado en la tabla de `commands.go`)
  desde que C21 se cerró en la 1.12.0. El namespace `cli.` es el correcto.
- **D13.5** — CERRADO sin código: la detección de idioma de D10.1 (1.12.0,
  `envLangHint`) hace que la primera salida ya salga en el idioma del
  sistema, así que el orden que sugiere el README no la deja en inglés.
- **D14.4** — CERRADO (2026-08-16, sin bump: solo instalador). `--uninstall`
  ya no dice "no encontré nada que quitar" cuando existe la copia del gestor
  de paquetes: la señala y remite al gestor. Informa y NO borra —`/usr/bin`
  sigue siendo territorio del gestor y el script nunca instala ni desinstala
  ahí—, así que el comentario de `PKG_BIN` que decía que la omisión era
  deliberada quedó corregido: lo deliberado es no tocar esa copia, no callarla.
  En la misma tanda, `inst_comp` dejó de tragarse un binario roto: si el maly
  recién compilado no puede emitir sus completions —o las emite VACÍAS, el
  mismo agujero que el PKGBUILD cerró en la 1.11.1— avisa en vez de instalar
  un archivo de 0 bytes en silencio. Verificado con binarios falsos en las dos
  direcciones (el código viejo instalaba los 0 bytes) y con una instalación
  completa de punta a punta en una sandbox XDG.
