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
  fija de `TestCompletePlaylistSubs`. `get.go` es el wrapper de yt-dlp
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
  `TestFillDurationsConcurrentSearch`. Escribe en lotes de `fillBatchSize`
  (50, no 500: cada elemento cuesta un ffprobe) y lo que falla queda en 0
  para que el próximo scan reintente (nada de centinelas: todos los
  consumidores prueban `> 0`). `IsAudio` es el filtro único de extensiones.
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
  entre la vista principal y la capa. El comando `logo` de la consola aplica
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

### Post-1.0 (candidatos)

La lista, que la 1.5.0 había dejado vacía, la reabrió la auditoría del
2026-07-21. Pendiente el hallazgo **#4**: `search`, `playlist_play` y
`learnDuration`→`SetDuration` siguen haciendo IO no acotado DENTRO de
`d.mu`, justo lo que sacó de ahí a scan, a la resolución de pistas y a
seek — el invariante está aplicado a medias. Sin planificar quedan los
menores (#5 los clientes no validan el runtime dir, #8, #10-#13). El ratón
en la TUI sigue descartado.

**El aviso de `update` no aparece cuando debería** (anotado 2026-07-21 a
raíz de la 1.6.0; el dueño lo arrastra desde hace tiempo). Al publicar el
tag v1.6.0 no salió el aviso, pero ESE caso concreto era correcto: el
binario ya estaba en 1.6.0 y `Newer` compara contra `version.Version`.
Diagnóstico de esa sesión: el repo responde a `ls-remote` anónimo por
HTTPS y el cache se refrescó bien. O sea que el problema no es la red.
Los cuatro sospechosos reales, por orden:
1. **El cache de 24 h se prefiere a la red** (`updateCheckCmd` en `tui.go`:
   si `Cached()` dice fresco, ni se pregunta). Un tag publicado hace diez
   minutos no se ve hasta 24 h después — que es EXACTAMENTE el momento en
   que uno mira si salió el aviso.
2. **Solo se chequea en `Init`**: una TUI abierta durante días no vuelve a
   mirar nunca.
3. **`verMismatch` tiene prioridad sobre `updAvail`** en el `switch` del
   footer (`view.go`). Justo tras actualizar el binario sin reiniciar el
   servicio —el escenario más común— el aviso de versión tapa el de
   update: los dos colisionan precisamente cuando ambos son relevantes.
   Y cualquier flash, `connErr` o el progreso de scan también lo pisan.
4. **Los fallos son mudos a propósito** (sin git, sin red): "no salió el
   aviso" y "el chequeo se rompió" son indistinguibles para el usuario.
Impacto real bajo (el repo es de una sola persona), pero conviene decidir
si el aviso merece su propia línea en vez de competir por el footer.
