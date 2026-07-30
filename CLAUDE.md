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
  `__complete` oculto). Al agregar un subcomando de playlist, actualizar la lista
  fija de `TestCompletePlaylistSubs`. El completado de pistas pide con TOPE
  (`library.SearchLimit`, `completeFetch` filas) y no con `Search`: la palabra
  parcial vacía es la consulta que recorre la biblioteca entera, y un TAB no
  puede materializarla para mostrar treinta líneas — ver la 1.7.3. `get.go` es el wrapper de yt-dlp
  (filosofía "como lazygit usa git": maly coordina herramientas externas, no
  las reimplementa): descarga MP3 con metadata/carátula embebidas a `music_dir`
  y re-escanea (vía IPC si el demonio responde, directo a la DB si no);
  yt-dlp/ffmpeg opcionales vía `exec.LookPath` con mensaje de instalación; el
  progreso de yt-dlp pasa directo al terminal, cero parsing.
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
  `advance(reason, chained)` es la política de avance y salto de pistas
  irreproducibles (guarda `errStreak`, silencio deliberado `stopped`).
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
  `onChange`) SIEMPRE async con `go` — en línea deadlockean readLoop.
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
  todos los consumidores prueban `> 0`). `IsAudio` es el filtro único de extensiones.
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
  (`subscribe`; fallback a polling de 500 ms con reintento). Paneles biblioteca/
  cola + consola ctrl+p (tabla propia de comandos en `console.go`, con paridad
  CLI completa: `playlist` en `console_playlist.go`, `get` vía
  `tea.ExecProcess` + `internal/getter` compartido con la CLI, `controls`
  aplica el preset en vivo recargando `m.keys`) + picker
  fuzzy genérico (`picker.go`, usado por ctrl+o canciones, ctrl+l playlists y
  `maly select`). Los modales tapan el footer: los flashes no se ven con un
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
  con origen para mensajes de error). Una clave booleana que deba venir
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
`mallow-install.sh` — el dueño descartó hacer PKGBUILD para AUR.

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

**Integración con Matugen — `maly theme sync`.** El diseño cambió a mejor
tras leer la config real de Matugen del dueño (con permiso explícito):
Matugen no escribe ningún JSON central por defecto — todo sale por
`[templates.*]` con sintaxis Tera y `post_hook`, el mismo mecanismo que ya
usa para kitty/waybar/hyprlock. `maly theme sync` no parsea el formato
interno de Matugen (inestable entre versiones): lee un TOML chico en una
ruta fija que el usuario genera agregando `[templates.maly]` a su propio
config, con `post_hook = 'maly theme sync'`. Sin flag `--from`: la CLI de
maly no tiene parser de flags a propósito. Los cuatro campos (accent,
color_low, color_high, logo) son opcionales — parcial es válido, salvo
`color_low`/`color_high` que van juntos (un gradiente a medias no tiene
sentido). Persiste con `saveKey` vía dos wrappers nuevos
(`SaveThemeAccent`, `SaveVisualizerColors`), mismo patrón que
`SaveThemeLogo`. Verificado end-to-end con el binario real de Matugen
(v4.1.0) sin tocar la config real del dueño (sandbox aislado).

De paso, `theme reload` en la consola ctrl+p: relee `config.toml` y aplica
el tema completo en vivo (`m.st = newStyles(...)` + `m.logo.ramp`), mismo
patrón que `controls` ya usaba para el merge de teclas — antes solo el
logo se recalculaba en caliente (`conLogo`), el resto del tema esperaba a
reiniciar la TUI.

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

La **1.10.1** (2026-07-29) salió de dar de alta la integración con Matugen
de verdad en la config real del dueño (fuera del repo, en su home) — al
probarla con un wallpaper real, una TUI que ya estaba abierta no recogía
el tema nuevo hasta reiniciarla. Esperable (documentado desde la 1.9.0:
"no hay señal de reload en caliente hacia una TUI ya abierta"), pero el
propio dueño señaló que kitty y waybar en su mismo `matugen/config.toml`
sí se refrescan solos porque sus `post_hook` mandan una señal
(`SIGUSR1`/`SIGUSR2`) — mismo mecanismo, aplicado a maly.

La TUI ahora atiende `SIGUSR1` en `Run` (`internal/tui/tui.go`, junto al
handler de `SIGHUP` que ya existía) y dispara `reloadTheme()` — la misma
función que ya usaba `theme reload` en la consola, extraída de `conTheme`
para que la señal y el comando compartan un solo camino. Como la señal
puede llegar muchas veces en la vida de una TUI (un `SIGHUP` es una vez y
se acabó; un cambio de wallpaper no), el goroutine que la atiende queda en
loop en vez del `select` de una sola pasada que usa `SIGHUP`.

**Trampa real, encontrada probando en vivo con `tmux` y la señal de
verdad, no supuesta:** el demonio y la TUI comparten el mismo nombre de
proceso (`maly`), así que `pkill -SIGUSR1 -x maly` —la forma obvia de
mandarla desde el `post_hook`— alcanza a los dos. La acción por defecto de
`SIGUSR1` es terminar el proceso, así que sin nada del lado del demonio
esa señal mataría el servicio de reproducción real cada vez que cambia el
wallpaper. `runDaemon` (`cmd/maly/client.go`) ahora también registra
`SIGUSR1`, pero solo para drenarla y no hacer nada — nunca se suma al
canal que dispara `d.Close()`. Verificado con un demonio y una TUI reales
en sandbox: la señal cambia el acento de la TUI en vivo (confirmado
grepeando el código ANSI truecolor renderizado antes/después en la salida
de `tmux capture-pane`) y dejó al demonio con el mismo PID y estado tras
recibirla.

El `post_hook` recomendado queda
`'maly theme sync && pkill -SIGUSR1 -x maly'` — documentado en el README
de ambos idiomas, y ya dado de alta en la config real de Matugen del
dueño junto con la plantilla y el archivo estático
(`~/.config/maly/matugen-colors.static.toml`, con los mismos valores que
`config.Default()` — la paleta "Kitasan Glass" del dueño y los defaults
de maly resultaron ser literalmente los mismos colores).

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
