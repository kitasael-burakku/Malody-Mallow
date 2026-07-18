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
  progreso de yt-dlp pasa directo al terminal, cero parsing. mp3 a propósito:
  dhowden lee sus ID3 en el scan y la miniatura APIC es justo lo que
  `mpris:artUrl` extrae. Tests sin red con un yt-dlp falso en el PATH
  (`get_test.go`, mismo patrón que el mpv falso de `player_test.go`).
- `internal/ipc` — protocolo (Request/Response/Status/TrackInfo), cliente, y
  `display.go` con los helpers de presentación compartidos (`TrackInfo.String`,
  `FmtTime`, `OnOff`) — no re-armar "Artista — Título" a mano.
- `internal/daemon` — `serve → handle → dispatch` (dispatch bajo `d.mu`; `handle`
  refleja mutadores a MPRIS/suscriptores y realinea la ventana gapless).
  play/add/playnow resuelven sus pistas (directorios, tags, rutas) ANTES de
  tomar `d.mu` — un `add <carpeta-grande>` bajo el lock congelaba
  status/TUI/MPRIS, la misma lección que sacó a scan del mutex.
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
  `maly get` reutiliza ese mismo camino. Sesión en
  `session.go` (JSON atómico en XDG_DATA_HOME, guardado cada 15 s si dirty y en
  Close; restaura la pista actual EN PAUSA).
- `internal/player` — wrapper de mpv. Gapless: `SetNext` mantiene una ventana de
  dos entradas con `playlist-clear + append` (NUNCA podar por índices: van
  rezagados tras end-file). Un end-file queda `pendingEnd` y se resuelve con el
  evento siguiente (start-file = encadenó, idle = no había nada); `loadGen`
  descarta desenlaces pisados por cargas propias. Callbacks (`onEnd`,
  `onChange`) SIEMPRE async con `go` — en línea deadlockean readLoop.
- `internal/queue` — cola con shuffle/repeat; `PeekNext()` promete el avance
  natural (la promesa que SetNext anexa); las mutaciones la invalidan.
- `internal/library` — SQLite (modernc, sin CGo, `SetMaxOpenConns(1)`, WAL).
  Búsqueda por columna `search_text` (minúsculas sin diacríticos vía `Fold`,
  que usa un `sync.Pool` porque los transformers tienen estado). Scan por LOTES
  de 500 (`flush` = Begin→N Exec→Commit); NUNCA una transacción única: fija la
  conexión y bloquearía Search/ByPath todo el escaneo. La columna `duration` se
  aprende perezosamente de mpv (`learnDuration` en el demonio); el upsert del
  scan no la toca. `IsAudio` es el filtro único de extensiones.
- `internal/mpris` — MPRIS2 (godbus). `props.go` es una implementación PROPIA de
  org.freedesktop.DBus.Properties porque godbus/prop tiene una data race con
  propiedades mapa y nunca borra claves — no volver a prop. Los métodos D-Bus
  despachan `ctrl.Do` en goroutine (en línea deadlockea vía SetMust).
  `metadataOf` es pura; el wrapper `Service.metadata` añade `artUrl` (carátula
  embebida → cache SHA-1 en runtime dir, `art.go`; la extracción vive en
  `internal/media`).
- `internal/media` — extracción compartida de lo embebido en las pistas:
  `ReadEmbedded` (carátula + letras USLT en una pasada de dhowden; OJO:
  ffmpeg escribe `-metadata lyrics=` como TXXX, no como USLT real — dhowden
  no lo ve), `DecodeImage`/`ScaleBox` (stdlib, box average) y `ParseLRC`/
  `LyricsFor` (sidecar `.lrc` con prioridad sobre las embebidas; `At < 0` =
  sin sincronía). Lo consumen mpris (artUrl) y la capa ctrl+t de la TUI.
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
  consola `conMsg.reload`) recarga el árbol; las hechas por CLI desde otra
  terminal no se reflejan en vivo (van directo a SQLite, sin demonio de
  por medio — limitación conocida). La capa "Ahora suena" (ctrl+t,
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
- El demonio adjunta `Response.Version` en toda respuesta; CLI y TUI avisan si
  difiere del binario.
- `config.Load()` mezcla teclas: defaults ← preset (`controls`) ← `[keys]` del
  usuario, vía un defer con retorno con nombre — mantener ese orden si se toca.
  `ScanTarget` resuelve el directorio a escanear (query explícita o music_dir
  con origen para mensajes de error).
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
en la TUI (`queueFolded`). Diferido a propósito: el retry de `player.seek`
duerme 250 ms bajo `d.mu` (aceptable hasta que moleste).

Trampas que dejaron estos ciclos:

- Tests de `internal/viz`: construyen el `Viz` a mano (`newTestViz`) porque
  `New()` arranca un pw-record/parec REAL en la máquina de desarrollo.
- El instalador sondea /dev/tty EN SUBSHELL: `:` es un special builtin y
  POSIX manda que su redirección fallida termine el shell entero — sin
  subshell, el modo no interactivo moría mudo.
- Probar el instalador bajo tmux con HOME alterno: pasar GOMODCACHE/GOCACHE
  reales al `go build` o deja un mod-cache de solo lectura en el sandbox
  (`chmod -R u+w` antes de borrar).

### Post-1.0 (candidatos)

- **Rediseño visual del instalador**: la funcionalidad interactiva ya está;
  falta lo vistoso que quiere el dueño (hoy conserva el banner/heartbeat
  sobrios de siempre).
- **`maly move <de> <a>`** + reorden en la TUI (J/K en cola): `queue.Move`,
  campo `To` en `ipc.Request`; la ventana gapless ya se realinea en `handle`.
- Progreso de scan (fácil en CLI directa; por IPC requiere diseño).
- Opcionales viejos: shuffle-permutación, ratón en la TUI, duración masiva vía
  `ffprobe` opcional.
