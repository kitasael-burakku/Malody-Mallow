# Roadmap — v1.8.0 … v1.11.1 (2026-07-29 … 2026-07-31)

Parte del roadmap de ingeniería de maly, movido aquí desde `CLAUDE.md`
sin cambiar una palabra. Índice de todas las versiones y de qué explica
cada una: `docs/history/README.md`.

La auditoría técnica exhaustiva repartida en tres tandas por prioridad
(1.8.0 alta, 1.9.0 media, 1.10.0 baja), la reversión de Matugen (1.10.2),
el paquete de AUR, la auditoría de seguridad desde cero (1.11.0) y el canal
de paquete (1.11.1).

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
