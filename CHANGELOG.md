# Changelog

Un resumen público, versión por versión. Sin categorías Added/Changed/Fixed
— la mayoría de releases de maly son una o dos piezas bien acotadas, y
separarlas en categorías no aportaría claridad extra. El razonamiento
completo detrás de cada decisión (qué se midió, qué se descartó y por qué)
vive en `CLAUDE.md`, pensado para quien vaya a tocar el código, no para
quien solo quiere saber qué cambió.

## v1.17.0 — 2026-09-06

Catorce arreglos de la misma auditoría que dio la 1.16.4, en su mayoría cosas
que fallaban en silencio.

Lo que más se nota a diario: `maly get` de una canción que ya tienes deja de
reportarse como un fallo —yt-dlp no la vuelve a bajar, y maly lo tomaba por
"no encontré nada", que es un problema distinto y con otra solución—; y
cuando el reproductor salta una pista que no puede sonar, o se detiene porque
ninguna de la cola se pudo reproducir, ahora lo dice en pantalla en vez de
dejarlo en el registro del sistema, donde nadie lo mira. Ese segundo caso es
justo el del disco de música que se desmonta mientras suena algo: antes todo
se paraba sin ninguna explicación.

Cosas que dejaban de funcionar sin avisar: agregar una carpeta muy grande a
la cola (a partir de unas quince mil pistas, la orden se perdía entera sin
mensaje); descargar una playlist entera desde la paleta de comandos, que se
quedaba en cero pistas si un solo vídeo era privado; los errores dentro de
"Ahora suena" y del buscador de descargas, que no se veían en ninguna parte y
hacían parecer que la tecla no había hecho nada; y borrar una playlist desde
la paleta, que no pedía confirmación como sí hacen los otros dos caminos.

`maly doctor` avisa ahora de los directorios enlazados dentro de tu carpeta
de música: no se indexan, y ambos README explican por qué y qué hacer (los
archivos enlazados sí se indexan; para un directorio hay que escanear el
destino). Además, el volumen ya no se puede poner en un valor imposible desde
un cliente MPRIS, `maly search ""` deja de volcar la biblioteca entera, y
salir de `maly get pick` mientras busca ya no deja un proceso corriendo solo
por detrás.

Por dentro: la detección de condiciones de carrera pasa a correr sobre todo
el proyecto en cada cambio, y hay tests nuevos sobre la parte que decide qué
pista se toca al pulsar una tecla — donde un error de una posición borraría
la canción equivocada sin decir nada.

## v1.16.4 — 2026-09-05

Un escaneo ya no puede vaciar la biblioteca cuando la música no está. Si
`maly scan` recorre el directorio de música y no encuentra ni un archivo de
audio —el caso típico es un disco externo sin montar, un directorio de red
caído o un `music_dir` recién cambiado, donde el punto de montaje sigue
existiendo como carpeta vacía—, hasta ahora concluía que toda la música se
había borrado y limpiaba la biblioteca entera. Y como las playlists apuntan
a las pistas indexadas, se quedaban vacías con ella: lo único que uno arma a
mano y que maly no puede reconstruir de ningún sitio. Ahora se niega y lo
dice, contando cuántas pistas acaba de proteger.

La interfaz también abre en un caso en el que antes se negaba: si el
servicio está arrancando —los primeros segundos tras iniciar sesión, con
`maly.service` habilitado, o dos `maly` lanzados a la vez—, escribir `maly`
mostraba un error en vez del reproductor. Ahora espera a que el servicio
termine de arrancar (hasta 8 s, avisando si tarda) y se conecta a él.

Y `maly scan` deja de anunciar una ruta y escanear otra. El servicio lee la
configuración al arrancar y no vuelve a mirarla, así que tras cambiar
`music_dir` con el servicio ya corriendo se seguía escaneando el directorio
viejo mientras el mensaje mostraba el nuevo. Ahora la ruta la resuelve quien
ejecuta el comando, que es el único que tiene la configuración al día — lo
que además desactiva el disparador más silencioso del problema anterior.

## v1.16.3 — 2026-08-20

Cerrada una condición de carrera que podía saltarse una canción entera. Al
terminar una pista, el demonio recibe el aviso de mpv en segundo plano; si
justo en ese instante llegaba una orden que recargaba la reproducción —un
salto a otra pista, por ejemplo— el aviso viejo se aplicaba igual sobre una
situación que ya había cambiado. El resultado era que la cola avanzaba sin
que la canción prometida llegara a sonar: la interfaz mostraba una pista
mientras se oía otra, y la siguiente desaparecía de la reproducción. De
paso, la duración aprendida en ese momento se guardaba en la fila
equivocada de la biblioteca.

El arreglo hace que el demonio compruebe, antes de hacerle caso al aviso,
que la reproducción no cambió mientras el aviso viajaba. Es una ventana de
milisegundos y hacía falta caer justo dentro, pero cuando pasaba se perdía
una canción sin ninguna señal de que algo hubiera fallado. Seis tests
nuevos la fijan, tres reproduciendo el fallo y tres cuidando que el camino
normal siga intacto.

## v1.16.2 — 2026-08-17

La sección de atajos de `maly -h` ya no es una copia a mano: sale de la misma
lista que pinta el modal `?` dentro de la TUI. Estaba desfasada —`ctrl+g`, el
buscador de descargas, nunca llegó a aparecer, y con él tampoco volumen, seek,
mover en la cola ni shuffle/repeat— y encima anunciaba siempre los defaults,
así que le mentía a cualquiera con un preset de `controls` o unas teclas
propias en `[keys]`. Ahora muestra las teclas EFECTIVAS y un atajo nuevo sale
en los dos lados o en ninguno.

Con eso, tres arreglos chicos: `maly search` ya no crea una biblioteca vacía
en disco solo por consultarla sin haber escaneado (la regla que info, doctor y
las completions ya respetaban); `maly get playlist <url> <nombre>` limpia el
directorio vacío que dejaba en `music_dir` cada intento fallido —solo el que
creó esa corrida, y solo si quedó vacío—; y agregar a una playlist que no
existe dice ahora cómo crearla.

## v1.16.1 — 2026-08-16

Los resultados de búsqueda muestran ahora las visitas junto a la duración
(`[04:10 · 808M]`), que es lo que separa la subida buena de un re-upload
cualquiera — y esa pregunta no la contestaban ni el título ni el canal.
Buscando "aurora runaway" aparecen dos resultados con el mismo canal, el
mismo título y la misma duración: uno con 808M visitas y otro con 423K. El
dato ya venía en lo que yt-dlp devuelve, así que no hay ninguna consulta
nueva.

Va sin decimales y sin la palabra "visitas" a propósito: lo que discrimina es
el orden de magnitud, y tanto el separador decimal como la palabra se
comerían espacio del título.

Y esa pantalla —y solo esa— gana ancho: es la única cuyos textos vienen de
fuera y son largos. En terminales anchas la caja pasa de 100 a 126 columnas y
los títulos dejan de cortarse; por debajo de 150 columnas no cambia nada, y
el resto de los paneles tampoco.

## v1.16.0 — 2026-08-16

Tres refinamientos sobre el buscador de descargas de la 1.15.0.

Los resultados que **ya están en tu biblioteca** salen marcados con `✓`, con
el hueco equivalente en los demás para que la columna de títulos siga
alineada. La comparación es por título limpio, sin el artista: el canal de
YouTube casi nunca es el artista real —una descarga de supercell puede
quedar acreditada al canal que la subió—, así que incluirlo daría falsos
negativos casi siempre. Es una pista y no un bloqueo: un cover legítimo con
el mismo título saldrá marcado y descargarlo sigue estando a un enter.

Los **directos en curso y los estrenos programados** ya no aparecen entre los
resultados: no son descargables como pista. La grabación de un directo que ya
terminó sí se conserva — muchos conciertos viven así, y descartarlos se
llevaría por delante material real.

Y llega **`maly get pick <búsqueda>`**: el mismo elegir-antes-de-bajar desde
la terminal, sin abrir la TUI. Es un subcomando y no un flag porque la CLI de
maly no tiene parser de flags, siguiendo el precedente de `maly get playlist`.
No necesita el servicio corriendo: descargar no pasa por él salvo el
re-escaneo final, que ya funciona igual sin demonio.

## v1.15.0 — 2026-08-16

Buscador de descargas dentro de la TUI: `ctrl+g` abre una pantalla donde
escribes qué quieres, `enter` busca en YouTube y la lista muestra canal y
duración de cada resultado; `enter` sobre el que elijas lo descarga y la
biblioteca se actualiza sola. Corrige una carencia real de `maly get`, que
bajaba **el primer resultado a ciegas**: si salía el equivocado, el único
recurso era borrar el archivo y reformular la consulta.

Es de dos fases —escribir, `enter` para buscar, elegir, `enter` para
descargar— y no un filtro en vivo tipo fzf, porque cada búsqueda cuesta
alrededor de un segundo de red y un proceso de yt-dlp: filtrar por pulsación
sería un proceso por tecla. Como los resultados dejan de corresponder en
cuanto editas la consulta, `enter` cambia de significado solo y el pie de la
pantalla dice cuál tiene en cada momento, para que corregir una palabra
nunca dispare una descarga que no pediste.

La descarga no cambia de dueño: la pantalla solo elige una URL y se la pasa
al mismo camino de siempre (yt-dlp toma el terminal, su progreso pasa
directo, y al volver se reescanea la biblioteca), así que el demonio no se
tocó y el protocolo IPC tampoco. Por dentro, `getter.Search` le pide a
yt-dlp su salida JSON —su interfaz de máquina, no el texto de progreso, que
maly sigue sin parsear— y se queda con cuatro campos de los cincuenta que
trae cada entrada. Los títulos y los canales pasan por el mismo saneado que
las etiquetas de las pistas: son texto ajeno que ahora llega con solo
buscar, sin descargar nada.

Y una descarga deja de darse por buena solo porque yt-dlp salga con código
0: ahora se comprueba mirando la carpeta de música. El caso que lo motiva es
una búsqueda que no encuentra nada —un typo basta—, que salía 0 sin bajar
nada y hacía que maly anunciara "Descarga lista" para cerrar con "La
biblioteca está vacía", sin decir en ningún momento que no había encontrado
la canción. De paso, la miniatura que yt-dlp baja antes del audio ya no se
queda huérfana en tu carpeta cuando la descarga falla.

## v1.14.1 — 2026-08-15

Cierra el hueco que dejaba la columna "Ahora suena" en pantallas altas. La
carátula es cuadrada, así que su alto lo manda el ancho de la columna y no
puede estirarse para llenar un panel de 38 filas: con la ficha anclada al
fondo quedaban ~20 filas muertas en medio, visibles en la captura del README
de la 1.14.0. Ahora el bloque (carátula + ficha) va centrado en vertical,
con el aire repartido arriba y abajo, y la columna puede crecer hasta 40
celdas en pantallas anchas — ocho celdas más de columna son cuatro filas más
de carátula, y la cola, que es la elástica, ni las nota.

## v1.14.0 — 2026-08-15

Rediseño completo de la pantalla principal de la TUI, en cinco fases.

El **tema** deja Catppuccin y pasa a la paleta del propio logo (teal
`#7ab8b8` y terracota `#b85c50`), con cinco claves nuevas en `[theme]` —
`accent_dim`, `surface` y los tres colores de la barra de progreso— que se
**derivan de `accent`** mientras no las escribas: cambiás un color y bordes,
selección y barra lo acompañan. La fila seleccionada pasa de una barra de
accent sólido a un fondo sutil (`surface`) que ya no pesa más que la pista
sonando, y el panel enfocado se distingue del resto por saturación del
borde, no por un gris apenas distinto.

La **barra de reproducción** gana resolución de sub-carácter: la cabeza
avanza en octavos de celda, con una estela que disuelve el corte contra la
pista y una sombra bajo el tramo reproducido (en la vista "Ahora suena", que
es donde hay altura para dibujarla).

El **layout** pasa a tres columnas —biblioteca de ancho fijo, cola elástica
y "Ahora suena" con carátula y letras sincronizadas—, con breakpoints: por
debajo de 120 columnas "Ahora suena" se colapsa en la barra del pie, y por
debajo de 90 queda una sola columna donde `tab` cicla entre biblioteca y
cola. El visualizador pasa a ser una franja sin panel propio y el banner
ASCII sale de la vista principal: la clave nueva `[theme] banner` elige
entre `splash` (pantalla de bienvenida al arrancar, por defecto), `titlebar`
(una sola fila con el gradiente animado) y `off`. Todo el reparto vive en
una función pura con tests de tabla sobre 13 anchos × 10 altos.

Las **letras** salen de `ctrl+t` y llegan a la pantalla principal, en un
panel propio bajo la carátula: la línea vigente se envuelve en hasta tres
filas sin cortar palabras (más de un tercio de las líneas reales no entra en
una columna angosta) mientras el contexto se mantiene a una fila.

Y los **títulos se muestran limpios**: los sufijos que deja yt-dlp
(`(Official Video)`, `[Lyric Video]`, `(Video Oficial)`…) y el artista
repetido dentro del título se ocultan solo al dibujarlos. La etiqueta del
archivo no se toca, así que `maly search`, la CLI y cualquier script que
parsee su salida siguen viendo el título original.

Config viejo sigue funcionando sin cambios: las claves nuevas se derivan y
`banner` toma su default, así que un `config.toml` de una versión anterior
no necesita ni una línea nueva.

## v1.13.1 — 2026-08-14

Dos refinamientos sobre la Lyrics View de la capa "Ahora suena" (ctrl+t).
La línea activa respira con un pulso de brillo sutil mientras suena música
(se congela en pausa, sin ticker nuevo) y el resto de las líneas se
atenúa según su distancia a la activa, mezclando los colores del tema ya
existentes en vez de introducir paletas nuevas. Además, más allá de 4
líneas de distancia el contexto ya no sigue apagándose hasta casi
ilegible: directamente desaparece. En la altura de terminal dominante
(~7 filas de letras) esto casi nunca se nota; en terminales altas evita
que el panel se llene de texto casi invisible.

## v1.13.0 — 2026-08-12

Cierra el roadmap completo (fases 1-5) de la segunda auditoría técnica
integral del proyecto (informe aparte, sin hallazgos por encima de MEDIUM).
Seguridad y estabilidad: un panic de programación en el demonio ya no
tumba el proceso entero para todos los clientes (`recover()` en dispatch,
con los mutex internos hechos a prueba de panic), el cliente IPC deja de
crecer sin límite si el demonio nunca contesta, y un tope de conexiones
evita agotar el demonio con suscripciones sin cerrar. Calidad: la
configuración valida los 7 colores del tema (antes solo el logo se
autocorregía), tests de stress con ~40 conexiones reales bajo `-race`,
fuzzing del parser de letras (`.lrc`) y benchmarks persistentes. UX: la
consola siempre vuelve al fondo al ejecutar un comando, ctrl+home/ctrl+end
saltan al principio/final de su historial, el color de error ya es
configurable y la fila seleccionada garantiza contraste sin depender del
terminal. El visualizador de espectro reintenta solo tras perder
PipeWire/el dispositivo (antes quedaba en animación falsa por el resto de
la sesión), y la unit de systemd `--user` suma hardening
(`NoNewPrivileges`, `ProtectSystem=strict` con `ReadWritePaths`,
`RestrictAddressFamilies=AF_UNIX` y más), verificado en vivo con una unit
sandboxeada antes de aplicarlo. Detalle completo, hallazgo por hallazgo,
en `CLAUDE.md`.

## v1.12.1 — 2026-08-03

Reportado por el dueño tras usar la 1.12.0: la Command Palette y otros
modales de la TUI rompían el borde con contenido ancho — el textinput de
bubbles nunca recibía `Width`, así que un comando largo (una URL de `get`,
por ejemplo) desbordaba la caja en vivo mientras se escribía. Junto con eso,
tres agujeros más del mismo patrón (una fila de ayuda de la consola, la
línea de tiempo de "Ahora suena" y el aviso de biblioteca vacía del
selector), la columna de teclas de la Ayuda (`?`) ahora se calcula al ancho
real en vez de un valor fijo, el tope de ancho de la consola subió de 80 a
100 columnas, y un hint de la consola que se había sobrecargado en la 1.12.0
volvió a un tamaño razonable.

## v1.12.0 — 2026-08-01

Cierra P0+P1+P2 (63 de 76 hallazgos) de la primera auditoría de UX completa
del proyecto — hasta ahora todas habían sido de seguridad. Entre lo más
visible: la ayuda (`?`) ya no tapa otros modales ni se traga la tecla que
la cierra, `ctrl+x` pide confirmación antes de borrar una playlist, rutas
relativas vuelven a funcionar en `add`/`play`, una descarga de playlist con
un solo video caído ya no tira todo a cero pistas, `maly update` sin red
deja de imprimir "exit status 128", el instalador reconoce una instalación
por gestor de paquetes (AUR) y dos comandos nuevos: `maly remove <pos>` y
`maly logo`. El detalle completo, por tandas y con las 4 decisiones de "no
cambiar" documentadas, está en `CLAUDE.md`. Quedan 10 hallazgos P3 de baja
prioridad marcados para revisar en un ciclo aparte.

## v1.11.1 — 2026-07-31

Cierra el último ítem diferido de la auditoría de la 1.11.0: canal de
paquete. `internal/version.Channel` (fijado por el PKGBUILD vía
`-ldflags -X`, con un fallback de ruta bajo `/usr/` para packagers que se
olviden del flag) hace que `maly update` deje de intentar instalar una
segunda copia por detrás de pacman — en un binario empaquetado, remite al
gestor en vez de bajar `mallow-install.sh`. El pie de la TUI y `maly info`
reflejan el canal.

## v1.11.0 — 2026-07-30

Auditoría de seguridad completa desde cero (sin críticos ni altos): el
instalador de `maly update` pasa a bajarse del mismo tag que instala en vez
de siempre `main`, `maly.sock` queda en 0600, `serve` gana deadlines de
lectura/escritura (evita fugas de goroutine/fd con un cliente que se cuelga)
y la guarda anti-bomba de carátulas ya no desborda en plataformas de 32
bits. También cierra el hallazgo de más impacto en UX: `maly get` ahora
siempre lleva `--no-playlist`, así que un enlace con `&list=` nunca baja de
más — y para cuando SÍ se quiere la playlist completa, **`maly get playlist
<url> [nombre]`** la descarga a un subdirectorio y crea una playlist de
maly con esas pistas, en orden.

## v1.10.2 — 2026-07-29

Saca entera la integración con Matugen (`maly theme sync` de la v1.9.0 y
la recarga en caliente por `SIGUSR1` de la v1.10.1), código y sistema
real. Decisión del dueño tras probarla: demasiada superficie orientada a
un escritorio para lo que maly quiere ser, una herramienta local. Ver
`CLAUDE.md` para el detalle.

## v1.10.1 — 2026-07-29

`maly theme sync` recarga en caliente por señal: la TUI atiende
`SIGUSR1` y aplica el tema sin reiniciarse, mismo mecanismo que kitty y
waybar. El `post_hook` recomendado pasa a ser
`maly theme sync && pkill -SIGUSR1 -x maly` — seguro de mandar aunque
también tengas `maly daemon` corriendo aparte, que ignora la señal a
propósito en vez de terminar. **Revertido en la v1.10.2.**

## v1.10.0 — 2026-07-29

Los siete ítems de prioridad baja de la auditoría: elimina dos índices
SQLite sin ningún consumidor, Makefile mínimo, este CHANGELOG,
`[visualizer] backend` forzable (`auto`/`pipewire`/`pulse`), el modal de
ayuda de la TUI ya no se desborda en terminales chicas, `daemon.go`
dividido en archivos por categoría, e interfaz `Controller` propia para
MPRIS (ya no depende de `ipc.Request`/`Response`).

## v1.9.0 — 2026-07-29

`maly config` (muestra la configuración efectiva: defaults ← preset ←
overrides del usuario), tests para `doctor.go`/`info.go`, unit de systemd
`--user` empaquetada en el instalador. También agregó integración con
Matugen vía `maly theme sync`, **revertida en la v1.10.2**.

## v1.8.0 — 2026-07-29

Primer CI del proyecto (GitHub Actions: build, vet, test y `-race`).
Corrige un bug real de sincronización en el gapless (`player.SetNext`
podía dejar el espejo de estado mintiendo tras un fallo de red) y el
filtro de la Biblioteca en la TUI ahora cachea el plegado Unicode, igual
que ya lo hacía la Cola.

## v1.7.3 — 2026-07-28

El completado de shell (`maly play <TAB>`) dejó de leer la biblioteca
entera con la palabra parcial vacía: 92 ms y 36 MB → 14 ms y 16 MB con
40.000 pistas.

## v1.7.2 — 2026-07-28

Los relojes de animación de la TUI se autocancelan en reposo: 3,5 % →
0,7 % de un núcleo con el visualizador activo y nada sonando.

## v1.7.1 — 2026-07-28

La fase de duraciones del scan (ffprobe) ahora sondea en paralelo:
28,5 s → 7,7 s por 1.000 pistas.

## v1.7.0 — 2026-07-22

`maly info` y `maly doctor`, los primeros comandos de diagnóstico —
funcionan sin demonio y sin red.

## v1.6.2 — 2026-07-22

El aviso de actualización disponible se revisa cada hora en vez de solo
al abrir la TUI.

## v1.6.1 — 2026-07-22

Cierre de la auditoría de seguridad de la 1.6.0: seis hallazgos menores
más, entre ellos un pánico real en la barra de progreso de "Ahora suena".

## v1.6.0 — 2026-07-21

Segunda auditoría de seguridad completa. Inyección ANSI/OSC desde tags
ID3 (paquete nuevo `internal/safetext`), `NaN`/`Inf` congelando el
demonio 5 s, mpv quedaba huérfano tras un `SIGKILL`, y la identidad del
demonio pasa a reclamarse con `flock` en vez de una heurística de socket.

## v1.5.1 — 2026-07-20

El panel de playlists (`ctrl+l`) se entera en vivo de las mutaciones que
hagan otros clientes conectados.

## v1.5.0 — 2026-07-20

Duraciones masivas con `ffprobe` como segunda fase del scan. El `seek`
sale del mutex del demonio.

## v1.4.0 — 2026-07-20

Shuffle por permutación: nada se repite hasta agotar el ciclo completo.

## v1.3.1 — 2026-07-18

Las mutaciones de playlists se reflejan en vivo en todos los clientes
conectados.

## v1.3.0 — 2026-07-18

`maly move`, progreso de scan visible en la TUI y en la CLI, ayuda `?`
con ancho dinámico.

## v1.2.1 — 2026-07-18

`[ytdlp] cookies_from_browser` para descargas que piden cuenta
(restricción de edad).

## v1.2.0 — 2026-07-17

Carátula como imagen real en kitty (su protocolo gráfico). Rediseño
visual completo del instalador.

## v1.1.5 — 2026-07-17

Primera auditoría de seguridad completa del proyecto.

## v1.1.0 — 2026-07-17

Pantalla "Ahora suena" (`ctrl+t`: carátula, letras sincronizadas,
visualizador), `maly kill`, colores del banner configurables.

## v1.0.2 — 2026-07-12

Instalador interactivo por pantallas (wizard con checklist de
dependencias).

## v1.0.1 — 2026-07-12

Primera revisión de seguridad: `EnsureRuntimeDir`, purga de carátulas
sin falsos `..`, `AddToPlaylist` transaccional.

## v1.0.0 — 2026-07-11

Primer release: reproducción gapless, biblioteca SQLite, playlists, TUI
con paneles, MPRIS.
