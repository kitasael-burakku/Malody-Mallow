# CLAUDE.md

Guía para Claude Code (claude.ai/code) al trabajar en este repositorio.

Este archivo es el **contrato operativo** del proyecto, no su enciclopedia:
filosofía, invariantes, arquitectura de alto nivel y reglas de trabajo. El
conocimiento profundo —ficha por paquete, el roadmap de ingeniería completo,
las decisiones cerradas con su razonamiento— vive en `docs/` y se consulta
cuando hace falta. **Cada sección dice a qué archivo ir.**

## Qué es

**Malody Mallow** (`maly`) es un reproductor de música local para terminal, en
Go. El branding visible es "Malody Mallow", pero el comando, el módulo Go, las
rutas XDG y el socket se llaman `maly` **a propósito** — no "corregir" eso.

El proyecto está en español: comentarios, mensajes de commit y documentación se
escriben en español. Todo texto visible para el usuario sale de `internal/i18n`
(tabla clave→[en, es]); nunca hardcodear cadenas en un solo idioma.

Versión actual: la const en `internal/version/version.go`. Los git tags empiezan
en v1.0.0; cada release nueva lleva bump + tag anotado. La meta sigue siendo
código limpio y entendible, no acumular features.

## Mapa de la documentación

| Necesitas… | Archivo |
| --- | --- |
| Ficha completa de un paquete | `docs/architecture/*.md` (tabla en [Arquitectura](#arquitectura)) |
| Lo que ya se evaluó y se descartó, con su razón | `docs/decisions/no-hacer.md` |
| Por qué una release hizo lo que hizo, qué se midió | `docs/history/README.md` → `docs/history/roadmap-*.md` |
| Montar un sandbox o probar la TUI en vivo | `docs/development/probar-en-vivo.md` |
| Cómo se verifica un arreglo (y las trampas del arnés) | `docs/development/verificacion.md` |
| Publicar, empaquetar, tocar la unit de systemd | `docs/development/releases.md` |
| Qué cambió en cada versión, en público | `CHANGELOG.md` |
| Cómo se usa maly | `README.md` / `README.en.md` |

Regla de mantenimiento: cada hecho tiene **un** dueño. Si algo de aquí se
desarrolla en `docs/`, aquí queda la regla y allí el desarrollo — no las dos
versiones completas.

## Comandos de desarrollo

```sh
go build -o maly ./cmd/maly   # ojo: `go build ./...` NO regenera ./maly
go test ./...                 # daemon/player usan mpv real; hacen t.Skip sin mpv
go test -race ./internal/library/ -run TestScanConcurrentSearch
go vet ./...
```

El `Makefile` envuelve los mismos (`build`, `vet`, `test`, `install`, `clean`).

El binario que usa el dueño del repo es `~/.local/bin/maly` (copia manual tras
compilar, no symlink). Tras un cambio, recordarle recompilar/copiar y reiniciar
el servicio (la TUI avisa si el demonio corre un binario viejo).

## Workflow esperado

1. **Antes de tocar una decisión que parezca rara, buscar su razón.** Casi todo
   lo que se ve extraño en este código se midió o se probó en vivo antes de
   quedarse así. Los dos sitios: `docs/decisions/no-hacer.md` y el roadmap.
2. **Todo arreglo con lógica o estado nuevo se verifica en AMBAS direcciones**
   — revertir producción, ver el test fallar **compilando y por la razón
   correcta**, restaurar (con `cp` de una copia, no con `git checkout`). Los
   cambios de solo texto/i18n no llevan test dedicado. Las seis trampas del
   arnés, en `docs/development/verificacion.md`.
3. **Correr la suite completa, no solo el test nuevo.** Dos regresiones reales
   las cazó un test que ya existía (`TestLoadLogoSane`, `TestScanPurgeDotDotDir`).
4. **Paridad CLI ↔ consola ctrl+p.** La consola no es espejo estricto, pero un
   comando que exista en los dos lados se arregla en los dos. Es exactamente lo
   que dejó pasar A-04. Lo cuida `ConsoleCommands` + sus dos tests.
5. **Texto nuevo = clave de i18n en en/es.** `callsites_test.go` valida los
   sitios de llamada (clave existe, `Tf` pasa tantos argumentos como verbos,
   ningún `T` con verbos, sin claves muertas); `go vet` **no** puede ayudar ahí.
6. **Al agregar un subcomando o un atajo**, actualizar su fuente única: la tabla
   de `commands.go` (dispatch + help + completions), `TestCompletePlaylistSubs`
   si es de playlist, y `tui.HelpRows` si es una tecla (de ahí sale también la
   ayuda de `maly -h`).
7. **Antes de una release**, leer `docs/development/releases.md`: bump + tag
   anotado + párrafo en `CHANGELOG.md` + entrada en el roadmap.
8. Al probar en vivo, **matar procesos solo por PID exacto** (`pgrep -a -x maly`)
   y el mpv por su socket. NUNCA `pkill -f` con cadenas que aparezcan en la
   propia línea de comandos del shell: el dueño corre un `mpvpaper` permanente
   que parece mpv en `pgrep`. El resto de trampas del sandbox, en
   `docs/development/probar-en-vivo.md`.

## Arquitectura

Demonio + clientes sobre un socket Unix con JSON por línea
(`$XDG_RUNTIME_DIR/maly/maly.sock`). El demonio posee mpv (IPC JSON por otro
socket), la cola y la biblioteca; CLI y TUI son clientes. Si no hay demonio, la
TUI lo **embebe** en su proceso (`cmd/maly/tui.go`) y muere con ella.

Resumen por paquete: lo mínimo para saber qué toca y qué no se puede romper.
**La ficha completa de cada uno —con el porqué de cada invariante— está en el
archivo de la última columna, y es la fuente única.**

| Paquete | Qué es y qué no se toca | Ficha |
| --- | --- | --- |
| `cmd/maly` | La CLI. `commands.go` es la **tabla de comandos**: fuente única de dispatch, help y completions. `tui.go` decide en `startOrAttach` si embeber el demonio o entrar como cliente (ante `ErrAlreadyRunning` espera hasta 8 s, no se rinde). `runScan` manda la ruta ya RESUELTA. `get.go` es el wrapper de yt-dlp. `info` lista HECHOS y sale 0; `doctor` emite VEREDICTOS y sale 1 solo si algo impide reproducir; ninguno necesita demonio ni red. | `docs/architecture/cli.md` |
| `internal/ipc` | Protocolo (Request/Response/Status/TrackInfo), cliente, y `display.go` con los helpers de presentación compartidos — no re-armar "Artista — Título" a mano. | `docs/architecture/daemon.md` |
| `internal/daemon` | El **orden de arranque de `New` no es negociable** (`EnsureRuntimeDir` → `ipc.Ping` → **flock** → socket → biblioteca → player → sesión → MPRIS): solo CON el lock son seguras las dos operaciones destructivas. `serve → handle → dispatch`, dispatch bajo `d.mu`, con **tres excepciones** que resuelven fuera del lock (play/add/playnow, seek, scan). `advance` tiene DOS guardas contra una promesa obsoleta y no se solapan. | `docs/architecture/daemon.md` |
| `internal/player` | Wrapper de mpv. Gapless por ventana de dos entradas con `playlist-clear + append` (**NUNCA podar por índices**). Callbacks **siempre async con `go`** — en línea deadlockean readLoop —, y por eso `loadGen` se valida en DOS puntos. | `docs/architecture/playback.md` |
| `internal/queue` | Cola con shuffle por **PERMUTACIÓN** (`order`/`pos`/`staged`): nada se repite hasta agotar el ciclo. `PeekNext()` promete el avance natural; los mutadores mantienen la permutación con cirugía incremental. | `docs/architecture/playback.md` |
| `internal/library` | SQLite (modernc, sin CGo, `SetMaxOpenConns(1)`, WAL). Scan por **LOTES de 500**, nunca una transacción única. La purga tiene una **GUARDA**: un walk sin ni un archivo de audio no borra nada (`*ScanEmpty`) — era la única pérdida de datos irreversible del proyecto. `scanTrack` es el punto único de salida. | `docs/architecture/library.md` |
| `internal/probe` | ffprobe para las duraciones. A diferencia de `getter.Tools`, **la ausencia NO es error**: se salta la fase en silencio. Se INYECTA en `FillDurations` (library no lo importa). | `docs/architecture/library.md` |
| `internal/mpris` | MPRIS2 (godbus). `props.go` es una implementación **PROPIA** de Properties — no volver a `godbus/prop`. Caché de carátulas acotado a 32 MB con evicción FIFO (el runtime dir es tmpfs). | `docs/architecture/mpris-media.md` |
| `internal/media` | Extracción compartida de lo embebido (carátula, letras USLT, `.lrc`, escalado). Lo consumen mpris y la TUI. | `docs/architecture/mpris-media.md` |
| `internal/safetext` | `Clean` descarta los caracteres de control del texto que maly NO controla. **Es un requisito de seguridad, no cosmética**: el recorte de la TUI es ANSI-aware y conserva los escapes. Filtra RUNAS, no bytes. | `docs/architecture/mpris-media.md` |
| `internal/tui` | Bubble Tea, estado por **suscripción push**. **Todo el reparto de pantalla vive en `computeLayout`, función PURA** — innegociable. `panel()` rellena pero **NO acorta**: cada llamador es responsable de su ancho. `case libraryMsg` es el punto único de refresco. Los relojes de animación se autocancelan. | `docs/architecture/tui.md` |
| `internal/i18n` | `T/Tf` (idioma global) y `TL/TLf` (por petición). `callsites_test.go` valida los sitios de llamada parseando el árbol con `go/ast`. | `docs/architecture/soporte.md` |
| `internal/update` | Chequeo de releases con `git ls-remote --tags`, cache 24 h. Nada de HTTP propio. | `docs/architecture/soporte.md` |
| `internal/config` | Mezcla teclas: defaults ← preset (`controls`) ← `[keys]` del usuario, vía un defer con retorno con nombre — **mantener ese orden**. `ScanTarget`/`ScanNoExistErr` son el punto único de la ruta de scan. | [Invariantes](#invariantes-transversales) |
| `internal/getter` | Wrapper de yt-dlp: comando, búsqueda (`--dump-json`), diff de directorio y limpieza de intermedios. El CLIENTE lo corre, no el demonio; el éxito de una descarga se mide por diff de directorio, no por el exit code. | `docs/architecture/cli.md` + `docs/decisions/no-hacer.md` |
| `internal/viz` | Captura de audio del sistema (pw-record/parec) y FFT. Reintenta cada 15 s tras perder la captura, solo si alguna vez la tuvo. | `docs/architecture/tui.md` |

## Invariantes transversales

Estos siete no se negocian y no viven en ningún otro archivo: esta sección es
su fuente única.

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
  `Response.Msg`/`Error`. Las otras dos fronteras de texto ajeno son el título
  de playlist de `get playlist` y los títulos/canales de `getter.Search`.
  `Track.Path` NO se sanea NUNCA: tiene que seguir abriendo el archivo.
- Ningún valor no finito llega a mpv: `NaN` sobrevive a TODA comparación
  (`NaN < 0` y `NaN > 100` son ambos false), así que se colaba por las
  validaciones de rango de `parseAdjust` y `d.seek` hasta `json.Marshal`, que
  lo rechaza — y con aquel error descartado el comando se perdía y costaba 5 s
  de timeout con `d.mu` tomado (un `maly vol NaN` congelaba el demonio entero).
  Lo cortan `finite()` en daemon y, como última barrera, `player.command` y
  `SetVolume`. La frontera MPRIS es ANTERIOR a las dos y quedó fuera hasta la
  auditoría del 2026-09-04 (A-16): `mpris.setVolume` clampaba `[0,1]` y un
  `Volume = NaN` atravesaba los dos clamps, `int(NaN*100+0.5)` daba el mínimo
  de int64 y llegaba como la cadena `"-9223372036854775808"`, que `parseAdjust`
  lee como ajuste relativo, ve finita y clampa — muteando el reproductor en
  silencio (medido: de 70 a 0). Ahora rechaza no finitos con `ErrInvalidArg`,
  como pide la spec. Los infinitos NO estaban rotos ahí (los clamps sí los
  recortan); se rechazan igual porque la barrera se escribe "no finito".
- El demonio adjunta `Response.Version` en toda respuesta; CLI y TUI avisan si
  difiere del binario.
- `config.Load()` mezcla teclas: defaults ← preset (`controls`) ← `[keys]` del
  usuario, vía un defer con retorno con nombre — mantener ese orden si se toca.
  `ScanTarget` resuelve el directorio a escanear (query explícita o music_dir
  con origen para mensajes de error) y `ScanNoExistErr` forma el mensaje de
  "esa ruta no existe": vive ahí porque desde la 1.16.4 lo produce el CLIENTE
  —el demonio recibe la ruta ya resuelta y ya no sabe de dónde salió— y así
  los dos espejos, CLI y consola, comparten un solo punto. Una clave booleana
  que deba venir ACTIVA por defecto se puebla en `Default()` (`update_check`,
  `scan_durations`): `toml.Decode` corre sobre el struct ya inicializado, así
  que un config viejo que no la menciona conserva el default. El zero-value
  solo sirve para las que nacen apagadas (`[ytdlp]`). El template únicamente
  se escribe cuando el config NO existe: una clave nueva jamás aparece en
  configs existentes y tiene que funcionar sin tocarlos.
- bubbletea fusiona teclas rápidas: dos `g` llegan como UN KeyMsg `"gg"` — los
  paneles manejan ambos casos.

## Lo que NO se hace

Lista corta para reconocer la casilla; el desarrollo completo de cada una, con
la medición o el experimento que la cerró, está en
**`docs/decisions/no-hacer.md`**. No son huecos por hacer: son decisiones
tomadas.

- **Red**: ninguna petición HTTP propia. Sin miniaturas en el buscador, sin
  pantalla completa para ellas, sin API de GitHub.
- **yt-dlp**: no se parsea su salida humana. La única excepción es
  `--dump-json`, su interfaz de máquina.
- **Demonio**: `search`/`playlist_play` se quedan dentro de `d.mu` (medido: 96
  ms, no segundos); la descarga NO le pertenece; `dispatch` sigue siendo un
  switch plano; sin auth ni `SO_PEERCRED` en el socket; sin `Pdeathsig`.
- **Dependencias**: no vuelve `godbus/prop` (data race); no se delega
  `safetext` en `x/ansi.Strip`; `gonum` se queda (65 KB reales en el binario).
- **CLI**: sin parser de flags (todo es subcomando), sin salida JSON, sin
  `maly report`, sin `status --verbose`, sin escape para la guarda de scan
  vacío, sin `playlist move`.
- **TUI**: sin ratón; sin marquesina en las letras; `computeLayout` no vuelve
  dentro de `View()`; la barra de progreso no vuelve a la columna.
- **Temas**: Matugen está revertido entero y no vuelve; sin `#rrggbbaa`; el
  arte del banner no es clave TOML.
- **Distribución**: sin firma de releases; el PKGBUILD no vive en este repo;
  `--uninstall` no toca `/usr/bin`.

## Estado actual

**v1.17.0** (`internal/version/version.go`). Ojo con los tags: la **1.16.4
nunca llegó a tagearse** —tiene su commit de bump y su entrada de CHANGELOG,
pero no existe `v1.16.4`, así que nunca se distribuyó: los tres canales
compilan el último tag—, y su contenido va dentro de la 1.17.0. Comprobar con
`git tag` antes de asumir que una versión salió.

En curso: la **auditoría técnica y arquitectónica del 2026-09** sobre la 1.16.3
(35 hallazgos: 1 CRITICAL, 5 HIGH, 11 MEDIUM, 11 LOW, 8 oportunidades; informe
aparte en `~/Audits/MalyAu/`, con una sección KEEP de 48 puntos y una
*Explicitly Don't Do* de 18).

- **Phase 0** — cerrada en la 1.16.4: A-01 (un scan vacío ya no purga la
  biblioteca ni vacía las playlists), A-02 (la TUI espera al demonio que
  arranca en vez de negarse), A-03 (`maly scan` manda la ruta ya resuelta).
  Desarrollo en `docs/history/roadmap-1.14-1.16.md`.
- **Phase 1** — cerrada COMPLETA (14/14) en la 1.17.0. Desarrollo en
  `docs/history/roadmap-1.17.md`. Lo que conviene llevarse de ahí, más allá de
  los arreglos: **cuatro afirmaciones del informe no sobrevivieron al contacto
  con el código** (A-06, A-25 y A-13 daban por resuelto algo que no lo estaba;
  A-15 proponía una receta insuficiente), y las cuatro se descubrieron por
  reproducir la premisa en vez de transcribirla. Un informe se cita, no se
  obedece.
- **Phases 2 y 3** — pendientes, en el informe. La mitad estructural de A-03
  (que el demonio relea el config) es Phase 2, y el orden importa: **O-05.2**
  (`BenchmarkState` por tamaño de cola) va ANTES de **A-05/O-01**
  (`Status.QueueGen`).

La lista de candidatos post-1.0 está **cerrada**
(`docs/history/candidatos-post-1.0.md`): conserva las mediciones que refutaron
hipótesis, que siguen siendo la razón de varias decisiones vigentes.
