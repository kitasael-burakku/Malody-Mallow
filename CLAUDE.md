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
Cada entrega justifica su número; la regla de incremento por alcance vive en
`docs/development/releases.md` (cambio grande conserva el último componente).

## Mapa de la documentación

| Necesitas… | Archivo |
| --- | --- |
| Ficha completa de un paquete | `docs/architecture/*.md` (tabla en [Arquitectura](#arquitectura)) |
| Lo que ya se evaluó y se descartó, con su razón | `docs/decisions/no-hacer.md` |
| Por qué una release hizo lo que hizo, qué se midió | `docs/history/README.md` → `docs/history/roadmap-*.md` |
| Próximas mejoras UI/UX y aceptación por entrega | `docs/planning/roadmap-ui-ux.md` |
| Ejecutar y cerrar esas entregas con evidencia | `docs/development/implementacion-ui-ux.md` |
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
| `cmd/maly` | La CLI. `commands.go` es la **tabla de comandos**: fuente única de dispatch, help y completions. `tui.go` decide en `startOrAttach` si embeber el demonio o entrar como cliente (ante `ErrAlreadyRunning` espera hasta 8 s, no se rinde). `runScan` manda la ruta ya RESUELTA. `get.go` es el wrapper de yt-dlp. `info` lista HECHOS y sale 0; `doctor` emite VEREDICTOS y sale 1 solo si algo impide reproducir; ninguno necesita demonio ni red. Dos chequeos de `doctor` le PREGUNTAN al demonio, porque no hay otra forma de saberlo: su config en efecto y si su mpv sigue vivo. | `docs/architecture/cli.md` |
| `internal/ipc` | Protocolo (Request/Response/Status/TrackInfo), cliente, y `display.go` con los helpers de presentación compartidos — no re-armar "Artista — Título" a mano, ni un mensaje que salga por los tres clientes. El **deadline** de una petición lo decide el protocolo (`reqTimeout`), no el llamador: 30 s por defecto y **ninguno para `scan`**. | `docs/architecture/daemon.md` |
| `internal/daemon` | El **orden de arranque de `New` no es negociable** (`EnsureRuntimeDir` → `ipc.Ping` → **flock** → socket → biblioteca → player → sesión → MPRIS): solo CON el lock son seguras las dos operaciones destructivas. `serve → handle → dispatch`, dispatch bajo `d.mu`, con **tres excepciones** que resuelven fuera del lock (play/add/playnow, seek, scan). `advance` tiene DOS guardas contra una promesa obsoleta y no se solapan. El push de suscripción se arma **por suscriptor** (`stateFor`) y lleva la cola SOLO si `queue.Gen()` cambió. `d.cfg` ya NO es inmutable: se relee en cada scan bajo `d.mu`. | `docs/architecture/daemon.md` |
| `internal/player` | Wrapper de mpv. Gapless por ventana de dos entradas con `playlist-clear + append` (**NUNCA podar por índices**). Callbacks **siempre async con `go`** — en línea deadlockean readLoop —, y por eso `loadGen` se valida en DOS puntos. Toda espera a mpv está ACOTADA y con dos topes: 5 s un comando normal, **1 s los de la ventana gapless**, que son best-effort y se pagan bajo `d.mu` en cada mutador. | `docs/architecture/playback.md` |
| `internal/queue` | Cola con shuffle por **PERMUTACIÓN** (`order`/`pos`/`staged`): nada se repite hasta agotar el ciclo. `PeekNext()` promete el avance natural; los mutadores mantienen la permutación con cirugía incremental. `gen` sube SOLO con lo que altera `Items` (Add/Replace/Clear/RemoveAt/Move) más `Touch()`, que usa el demonio para las duraciones que escribe en sitio. | `docs/architecture/playback.md` |
| `internal/library` | SQLite (modernc, sin CGo, `SetMaxOpenConns(1)`, WAL). Scan por **LOTES de 500**, nunca una transacción única. La purga tiene una **GUARDA**: un walk sin ni un archivo de audio no borra nada (`*ScanEmpty`) — era la única pérdida de datos irreversible del proyecto. `scanTrack` es el punto único de salida. **`Create` crea, `Open` NO** (devuelve `fs.ErrNotExist` envuelto): consultar la biblioteca no puede fabricarla. | `docs/architecture/library.md` |
| `internal/probe` | ffprobe para las duraciones. A diferencia de `getter.Tools`, **la ausencia NO es error**: se salta la fase en silencio. Se INYECTA en `FillDurations` (library no lo importa). | `docs/architecture/library.md` |
| `internal/mpris` | MPRIS2 (godbus). `props.go` es una implementación **PROPIA** de Properties — no volver a `godbus/prop`. Caché de carátulas acotado a 32 MB con evicción FIFO (el runtime dir es tmpfs). | `docs/architecture/mpris-media.md` |
| `internal/media` | Extracción compartida de lo embebido (carátula, letras USLT, `.lrc`, escalado). Lo consumen mpris y la TUI. | `docs/architecture/mpris-media.md` |
| `internal/safetext` | `Clean` descarta los caracteres de control del texto que maly NO controla. **Es un requisito de seguridad, no cosmética**: el recorte de la TUI es ANSI-aware y conserva los escapes. Filtra RUNAS, no bytes. | `docs/architecture/mpris-media.md` |
| `internal/tui` | Bubble Tea, estado por **suscripción push**. **Todo el reparto de pantalla vive en `computeLayout`, función PURA** — innegociable. `panel()` rellena pero **NO acorta**: cada llamador es responsable de su ancho. `case libraryMsg` es el punto único de refresco. El estado modal es **un** enum (`activeOverlay()`), no banderas sueltas; `showHelp` va aparte porque se dibuja ENCIMA. Los relojes de animación se autocancelan. | `docs/architecture/tui.md` |
| `internal/i18n` | `T/Tf` (idioma global) y `TL/TLf` (por petición). `callsites_test.go` valida los sitios de llamada parseando el árbol con `go/ast`. | `docs/architecture/soporte.md` |
| `internal/update` | Chequeo de releases con `git ls-remote --tags`, cache 24 h. Nada de HTTP propio. | `docs/architecture/soporte.md` |
| `internal/config` | Mezcla teclas: defaults ← preset (`controls`) ← `[keys]` del usuario, vía un defer con retorno con nombre — **mantener ese orden**. `ScanTarget`/`ScanNoExistErr` son el punto único de la ruta de scan. Los colores se validan por **tabla** (`hexFields`) y un test por reflexión exige que no falte ninguno. | [Invariantes](#invariantes-transversales) |
| `internal/getter` | Wrapper de yt-dlp: comando, búsqueda (`--dump-json`), diff de directorio y limpieza de intermedios. El CLIENTE lo corre, no el demonio; el éxito de una descarga se mide por diff de directorio, no por el exit code. | `docs/architecture/cli.md` + `docs/decisions/no-hacer.md` |
| `internal/viz` | Captura de audio del sistema (pw-record/parec) y FFT. Reintenta cada 15 s tras perder la captura, solo si alguna vez la tuvo. | `docs/architecture/tui.md` |

## Invariantes transversales

Estos ocho no se negocian y no viven en ningún otro archivo: esta sección es
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
- **El demonio relee el config en cada scan** (`reloadConfig`, A-03), así que
  `daemon.New(cfg)` recibe el config INICIAL y no el definitivo: a partir del
  primer escaneo manda el archivo. Un test que quiera fijar una clave tiene que
  escribirla en el `config.toml` del sandbox, no solo en el struct. Si la
  relectura falla se conserva el anterior y se anota el error, que expone la op
  de solo lectura `config` y mira `maly doctor`. Releer le quitó a `d.cfg` la
  inmutabilidad de la que dependía leerlo sin lock: ahora se escribe bajo
  `d.mu` y el scan trabaja con una copia local.
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
  vacío, sin `playlist move`. Varios argumentos solo en `play`/`add`/
  `playlist add` (una consulta por argumento); sin `remove` en lote y sin
  heurística que adivine frase o lista.
- **TUI**: sin ratón; sin marquesina en las letras; `computeLayout` no vuelve
  dentro de `View()`; la barra de progreso no vuelve a la columna.
- **Temas**: Matugen está revertido entero y no vuelve; sin `#rrggbbaa`; el
  arte del banner no es clave TOML.
- **Phase 3 de la auditoría del 2026-09**: cinco ítems cerrados con su razón —
  los clamps de `View()` (A-09), `context` en `Scan`/`doClose` (O-04),
  `add` por id (O-08), **seguir enlaces de directorio** (A-13, el único que
  podía perder datos) y la carátula fuera de los mutex de MPRIS (A-21).
- **Distribución**: sin firma de releases; el PKGBUILD no vive en este repo;
  `--uninstall` no toca `/usr/bin`.

## Estado actual

**v1.19.8 — implementada y verificada; pendiente de commit, tag y publicación**
(`internal/version/version.go`): UX-04 unifica las consultas locales de canciones;
aceptación y límites en el roadmap UI/UX y la historia de ingeniería. La
1.19.7 (commit `2363318`, sin tag) mejoró títulos, contraste y selección. La
1.19.6 (commit `9135bcd`, sin tag) presentó ayudas contextuales y teclas resueltas. La 1.19.5
distinguió carga, error, vacío y filtro sin coincidencias (UX-01). La 1.19.4 corrigió los bordes
por VS15 normalizando solo la salida de pantalla (ver la ficha de TUI). La 1.19.3 añadió consultas en lote en `play`,
`add` y `playlist add`, también en la consola ctrl+p. La 1.19.2 hizo legible
la CLI en terminales de fondo claro. La 1.19.1 trajo el filtro `/` flotante y el
arreglo de la posición MPRIS, y es una feature con número de parche por
decisión del dueño. Ojo con los tags: la **1.16.4 se
dejó sin tag A PROPÓSITO** (decisión del dueño) — fue un puente a la 1.17.0,
que salió el día siguiente con la Phase 1 entera. Tiene su commit de bump y su
entrada de CHANGELOG, pero no existe `v1.16.4` y por tanto nunca se
distribuyó: los tres canales compilan el último tag, así que su contenido
llegó a los usuarios recién dentro de la 1.17.0. Es el precedente de qué hacer
cuando una versión se queda por el camino: no se borra su entrada, se anota
dónde acabó saliendo. Comprobar con `git tag` antes de asumir que una versión
salió.

La **auditoría técnica y arquitectónica del 2026-09** sobre la 1.16.3 (35
hallazgos: 1 CRITICAL, 5 HIGH, 11 MEDIUM, 11 LOW, 8 oportunidades; informe
aparte en `~/Audits/MalyAu/`, con una sección KEEP de 48 puntos y una
*Explicitly Don't Do* de 18) está aplicada hasta la Phase 2:

- **Phase 0** — cerrada en la 1.16.4: A-01 (un scan vacío ya no purga la
  biblioteca ni vacía las playlists), A-02 (la TUI espera al demonio que
  arranca en vez de negarse), A-03 (`maly scan` manda la ruta ya resuelta).
  Desarrollo en `docs/history/roadmap-1.14-1.16.md`.
- **Phase 1** — cerrada COMPLETA (14/14) en la 1.17.0.
- **Phase 2** — cerrada COMPLETA (8/8) en la 1.18.0. Desarrollo de las dos en
  `docs/history/roadmap-1.17.md`.
- **Phase 3** — cerrada en la 1.19.0, y no haciéndola entera: era la fase
  **explícitamente no obligatoria** (12 oportunidades, ninguna resolvía un
  problema que alguien sufriera). Se hicieron **siete** (A-22, A-11, A-18,
  A-26, A-08, A-23, O-07) y **cinco se cerraron como decisiones tomadas** en
  `docs/decisions/no-hacer.md` (A-09, O-04, O-08, A-13, A-21). Con esto la
  auditoría queda entera: 27/27 hallazgos y 8/8 oportunidades. Desarrollo en
  `docs/history/roadmap-1.17.md`.

Lo que hay que llevarse de las dos fases aplicadas, más allá de los arreglos:
**ocho afirmaciones del informe no sobrevivieron al contacto con el código**
—cuatro en cada fase— y las ocho se descubrieron por reproducir la premisa en
vez de transcribirla. **Un informe se cita, no se obedece.** Ejemplos: A-05
daba una lista de mutadores mal por los dos lados (sobraban cuatro, faltaban
dos que habrían dejado las duraciones en cero para siempre); A-10 contaba tres
listas de modales y son cuatro; A-03 describía un problema de `music_dir` que
la 1.16.4 ya había cerrado.

El triaje volvió a pagar: **cinco de los doce ítems tenían la premisa o la
justificación movida**, y en dos casos eso decidió no hacerlos. O-07 daba el
estado por «mudo» y no lo es (cada comando contesta «mpv terminó»; lo mudo
eran `status` y `doctor`); A-09 hablaba de seis sitios y quedaban dos, con un
remedio peor que el problema; A-18 decía dos sitios y es uno; O-08 se quedó
sin justificación al cerrarse A-06; y la medición que A-08 exigía no refutó el
ítem sino que le retiró el riesgo.

La lista de candidatos post-1.0 está **cerrada**
(`docs/history/candidatos-post-1.0.md`): conserva las mediciones que refutaron
hipótesis, que siguen siendo la razón de varias decisiones vigentes.
