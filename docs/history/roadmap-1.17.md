# Roadmap de ingeniería — v1.17.0 en adelante

Índice: [`README.md`](README.md).

## v1.17.0 (2026-09-06) — Phase 1 de la auditoría del 2026-09

La **1.17.0** cierra la **Phase 1** completa (14 ítems) de la auditoría
técnica y arquitectónica del 2026-09 sobre la 1.16.3, cuya Phase 0 se hizo en
la 1.16.4 — que **se dejó sin tag a propósito**, como puente: nunca se
distribuyó por separado (los tres canales compilan el último tag) y su
contenido, incluida la guarda de A-01, llegó a los usuarios recién acá. Se hicieron en el orden de la lista del informe, uno por commit, cada
uno con sus tests verificados en ambas direcciones antes de pasar al
siguiente.

Con esto la auditoría queda en **16 de 27 hallazgos y 2 de 8 oportunidades**
cerrados. Phase 2 y Phase 3 siguen en el informe (`~/Audits/MalyAu/`).

### Lo que de verdad deja esta tanda

No son los catorce arreglos: es que **cuatro afirmaciones del informe no
sobrevivieron al contacto con el código**, y las cuatro se descubrieron por
reproducir la premisa en vez de transcribirla. Un informe se cita, no se
obedece; si un hallazgo dice «esto ya funciona así», hay que verlo funcionar.

1. **A-06** daba por hecho que «el cliente ya sabe imprimir `Response.Error`».
   No: con una petición pasada de tope el demonio deja de leer, el cliente ve
   EPIPE a mitad de su `Write`, y `Do` volvía con el error de I/O crudo **sin
   llegar a leer nada**. El mensaje nuevo habría sido inalcanzable desde maly
   — justo el «diagnóstico imposible desde la UI» que el hallazgo describe.
2. **A-25** daba por hecho que «`TL` lo traduce al idioma del cliente». No:
   `statusLocked` no recibía idioma y el push de suscripción se arma UNA vez
   para todos los suscriptores. Guardar el texto ya armado lo habría dejado en
   el idioma del demonio, que es lo que `TL` existe para evitar.
3. **A-13** proponía un remedio que **no funciona**: «`maly scan <destino>`
   los indexa igual, porque el `root` explícito sí se sigue». Apuntar
   `maly scan` al ENLACE devuelve «0 new» y no indexa nada, con barra final o
   sin ella — `filepath.WalkDir` hace `lstat` de su propia raíz. Documentarlo
   tal cual habría mandado al usuario a un callejón sin salida.
4. **A-15** proponía una receta insuficiente: «cancelar en el camino de
   salida, igual que `abortGetSearch`». Medido: eso deja el yt-dlp huérfano en
   **12 de 20** corridas. `cancel()` no mata — marca el contexto, y el kill lo
   ejecuta la goroutine vigía que `os/exec` arranca en `Start()`, que se va con
   el proceso si este sale enseguida.

### Los catorce, en orden

**A-04 — portar G1, G2, G3 y la confirmación de `delete` a la consola.**
`internal/tui` no puede importar `cmd/maly`, así que `get playlist` y
`playlist` están escritos dos veces, y las copias habían divergido: tres
hallazgos que la auditoría de UX de la 1.12.0 dio por cerrados seguían vivos
solo del lado CLI. G1 (un ítem privado tiraba la descarga entera), G2 (el
choque de nombre se detectaba tras bajar todo), G3 (la playlist se llevaba
todo el audio del directorio) y C15/T26 (`playlist delete` sin confirmar, el
único de los tres caminos de la TUI que borraba de una).

`planGetPlaylist` y `newTrackIDs` se extraen por el mismo motivo por el que el
hallazgo existía: con todo dentro de `conGetPlaylist`/`conGetPlaylistFinish`
**no hay forma de testear esta mitad** (`tea.ExecProcess` solo se resuelve
dentro del runtime de bubbletea, y al resto solo se llega pasando por el
demonio). El primer test de G3 quedó en `t.Skip`, y un Skip en una suite que
no tiene ninguno es un test que no prueba nada.

**A-16 — `mpris.setVolume` rechaza no finitos.** `NaN` es false en toda
comparación, así que atravesaba los dos clamps; `int(NaN*100+0.5)` da el
mínimo de int64, que viaja como `"-9223372036854775808"`, y `parseAdjust` lo
lee como ajuste relativo, lo ve finito y lo clampa: **el reproductor se muteaba
en silencio, de 70 a 0**, medido de punta a punta. La frontera MPRIS es
ANTERIOR a `finite()` y a la última barrera de `player.SetVolume`, así que
ninguna lo salvaba. Corrección al hallazgo: habla de «NaN e Inf», pero los
infinitos NO estaban rotos —`Inf > 1` y `-Inf < 0` son true— y se rechazan
igual solo porque la barrera se escribe «no finito».

**A-20 — `maly search ""` es error de uso.** La guarda contaba argumentos y no
contenido. Se comprobó, en vez de darla por buena, la premisa con la que la
1.6.1 dejó `search` DENTRO de `d.mu`: sigue en pie, porque esta ruta lee
SQLite directo y el único cliente que manda la op por IPC es la consola, cuyos
argumentos salen de `strings.Fields`. Como esa garantía la da `strings.Fields`
y no una guarda explícita, se fijó con un test: si la consola aprendiera a
respetar comillas —se discutió en C14— la premisa caería en silencio.

**A-19 + O-05.1 — el ruido de i18n y la red que lo mantiene.** Se borra la
clave muerta `help.show` y los cuatro `fmt.Sprintf(i18n.T(k), …)` pasan a
`Tf`. Van en el mismo commit porque la limpieza sin la red se vuelve a
ensuciar: `internal/i18n/callsites_test.go` deja persistente el analizador que
la auditoría escribió como desechable, con cuatro chequeos sobre el árbol
parseado con `go/ast`. Hace falta porque **`go vet` no puede ayudar**:
reconoce printf-wrappers cuando el formato es un PARÁMETRO, y en `Tf` sale de
buscar la clave en la tabla.

Dos trampas que costaron un ciclo cada una. El chequeo de claves muertas
**nacía vacuo**: la tabla vive en el mismo paquete y sus claves son literales,
así que recogerlas contaba a cada clave como uso de sí misma —una clave
inventada pasaba el test—; las cadenas se recogen ahora de todo el árbol MENOS
del paquete `i18n`. Y la primera reversión de «clave inexistente» no probó
nada: el sitio elegido resultó ser un helper local y no una llamada a i18n, o
sea que el `sed` no cambió nada y el test «pasó» correctamente.

**A-17 — la frase del despacho D-Bus era un fósil.** `CLAUDE.md` decía que los
métodos D-Bus despachan en goroutine «porque en línea deadlockea vía
SetMust». Falso dos veces: solo los TRES setters de propiedades llevan `go`, y
los OCHO métodos del Player corren en línea desde siempre. Pero la afirmación
**fue cierta**, y eso explica por qué se escribió: con `godbus/prop`,
`Properties.Set` llama al callback CON el mutex tomado y `SetMust` toma el
mismo mutex. `props.go` —el reemplazo propio— suelta `p.mu` ANTES de llamar al
setter, así que el motivo desapareció al escribirlo y el `go` sobrevivió. Hoy
compra LATENCIA. Que los ocho en línea sean seguros es propiedad de **godbus**
y no de maly (`go conn.handleCall(msg)`, v5.2.2 `conn.go:435`), y el texto
nuevo lo dice como tal para que se re-compruebe si se cambia de librería.

**A-15 — el yt-dlp huérfano al salir.** Además de la receta insuficiente (ver
arriba), apareció un **segundo agujero de la misma familia que el hallazgo no
menciona**: `ctrl+c` sale ANTES de todo modal (primera línea de `handleKey`),
así que con el buscador abierto nunca se llega a `closeGet`. `stopGetSearch`
cancela **y espera**, con tope, y lo usan `RunGetPick` y `tui.Run`.

Trampa de arnés, y de las buenas: la primera medición en vivo dio **0 de 5
huérfanos en HEAD**, o sea que «refutaba» el hallazgo. Era tmux: al terminar
el comando de un pane se derriba su grupo de procesos, que se lleva al hijo
con arreglo o sin él. Con el pane vivo después (`maly …; sleep 60`), HEAD da
**5 de 5** y el arreglo **0 de 5**.

**A-14 — `-race` sobre todo el árbol en CI.** El job corría solo `library` y
`mpris`. El motivo de la lista corta —la race entre tests que causaba
`idleTimeout` como var de paquete— se cerró en la 1.11.0, y se **comprobó**
antes de tocar nada: no queda ninguna var de paquete que los tests pisen, y
`go test -race ./...` pasa entero en TRES corridas seguidas (una sola verde no
basta para un job que corre en cada push: lo que se mete es un CI
intermitente). La condición de CI también se midió: con un PATH realmente
vacío, `internal/daemon` salta **36 de sus 42** tests y reporta `ok` — de
paso, el «23 de 24» que la documentación arrastraba desde la 1.8.0 estaba
desactualizado.

**A-24 — el flash de error llega a las cuatro capas.** El flash del pie es el
único canal de error de la TUI y los modales de pantalla completa lo tapan;
estaba resuelto en dos de los cuatro sitios, con el bloque copiado. `withFlash`
lo extrae, y `npView` —pantalla completa con panel propio— toma prestada la
fila del HINT en vez de agregar una.

La trampa: el test que protege ese riesgo hubo que rehacerlo. Medir que **el
alto no cambie NO sirve**, porque `panel()` trunca a `innerH` en silencio y la
variante ingenua (agregar una fila) pasa igual; comparar la salida entera
tampoco, porque sin audio las filas del viz son idénticas entre sí y el
desplazamiento no se ve. La aserción buena es POSICIONAL: el flash cae en el
mismo índice de fila que ocupaba el hint.

**A-06 — una petición grande ya no se pierde en silencio.** `serve` usaba un
`bufio.Scanner` con tope de 1 MiB y una petición más larga cerraba la conexión
sin responder nada — alcanzable desde la UI, porque `add`/`playnow` mandan
rutas EXACTAS y el techo caía en ~15.000 pistas. `readLine` se mueve a
`internal/ipc` como `ReadLine` compartida, el tope pasa a `MaxReqLine`
(16 MiB) y el exceso se distingue por TIPO para contestar. Con la
reproducción del informe (20.000 rutas, 1.460.027 bytes): antes
`BrokenPipeError` y ninguna respuesta, ahora `{"ok":true}`.

**A-25 — los avisos de reproducción llegan al usuario.** `advance` contaba las
pistas saltadas y la cola detenida SOLO al stderr, o sea al journal bajo
systemd. `ipc.Status.Notice` con `omitempty`; el stderr se conserva para el
postmortem. En la TUI el flash se arma solo al CAMBIAR: los pushes llegan
varias veces por segundo y el aviso persiste hasta la carga sana siguiente,
así que rearmarlo en cada foto lo dejaría fijo sin caducar nunca.

De paso apareció un **test intermitente PREEXISTENTE**: `TestSubscribe`
fallaba 3 de 40 corridas leyendo el volumen viejo. La causa es general y quedó
anotada: un comando de reproducción vuelve cuando mpv lo acepta, pero
`statusLocked` lee el ESPEJO del player, que se refresca con el
property-change asíncrono posterior — leer el estado enseguida es intermitente
por construcción. Con la espera, 0 de 40. Es el candidato más probable a
explicar el fallo suelto que A-14 dejó anotado sin explicación, aunque no
quedó registrado qué test había fallado entonces.

**A-27 — el núcleo del modelo de la TUI.** El layout estaba barrido por tabla
y la máquina de estados que decide QUÉ PISTA SE TOCA no tenía ni un test.
Cuatro tests que capturan el `ipc.Request` real con un demonio falso. El
primero falla, con el defecto puesto, diciendo exactamente lo que el informe
temía: `Index = 1, quería 3 (la posición REAL): se borra la pista equivocada`.
`handleQueueKey` queda en 35,6 % y no en 100 a propósito: apuntan a los
caminos que pueden tocar la pista equivocada, no a subir un número.

**A-07 + O-02 — el contrato de descarga.** Desde la 1.15.0 el criterio de
éxito es el diff del directorio, y tiene un agujero que aquella no consideró:
yt-dlp no rebaja lo que ya existe, así que el diff queda vacío igual que
cuando no encontró nada — dos casos con remedios OPUESTOS colapsados en el
mismo error, con código de salida 1. Y no es raro: `ctrl+g` marca con `✓` lo
ya descargado justamente porque re-descargar es algo que la gente intenta.

Se implementó la vía buena: `--print-to-file after_move:filepath`, otra
interfaz de MÁQUINA de yt-dlp del mismo tipo que `--dump-json`. El diff **no**
se retira —sigue siendo lo que caza la búsqueda sin resultados— y las dos
señales se complementan. La premisa se verificó dos veces: en la FUENTE de
yt-dlp (las dos ramas de `report_file_already_downloaded` no retornan
temprano, así que se llega a `run_all_pps("after_move")`) y EN VIVO contra
yt-dlp 2026.08.19, con una descarga real repetida.

Un riesgo que el hallazgo subestima: dice que conviene «degradar en silencio
si el archivo sale vacío (yt-dlp antiguo)», pero un yt-dlp que no conociera la
opción no la ignoraría — fallaría con error de uso y NINGUNA descarga
funcionaría. En la práctica no aplica (existe desde 2021), y el degradado que
sí está cubre lo que puede: archivo ausente, ilegible o vacío vuelve al diff.

**A-13 (mitad barata) — los enlaces simbólicos, documentados y
diagnosticados.** Ver arriba el remedio que el hallazgo proponía y que no
funciona. `checkLinkedDirs` en `maly doctor` lista los enlaces a directorio de
primer nivel bajo `music_dir` con el destino de cada uno, en `lvlInfo` (no
cambia el código de salida): no seguir enlaces es una decisión defendible
—ciclos, pistas duplicadas por dos rutas, y `underRoot` dejaría de tener
sentido para la purga, que desde la 1.16.4 sostiene además la guarda de
A-01—; lo que no lo era es que estuviera tomada en silencio. Seguirlos en el
primer nivel sigue siendo Phase 3.

### Sobre la 1.17.0, sin bump: la documentación reorganizada

En medio de la Phase 1, el dueño reorganizó la documentación: `CLAUDE.md` pasa
de ~2.300 líneas a ~250 y se queda como **contrato operativo** (filosofía,
invariantes, arquitectura de alto nivel, reglas de trabajo), y todo lo demás
—ficha por paquete, este roadmap, las decisiones cerradas, cómo se prueba y
cómo se publica— vive en `docs/`. La regla que lo sostiene: **cada hecho tiene
un dueño**; si algo está desarrollado en `docs/`, `CLAUDE.md` lleva la regla y
remite, no una segunda copia. El commit es `842bcdc`.

### Verificación

Cada arreglo con lógica o estado nuevo lleva test verificado en ambas
direcciones, revirtiendo con `cp` de una copia y no con `git checkout`.
`go build`, `go vet`, `gofmt -l .`, `go test ./...` y —desde A-14, en CI
también— `go test -race ./...` limpios en todo el árbol.
