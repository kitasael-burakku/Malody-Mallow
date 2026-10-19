# Changelog

Un resumen público, versión por versión. Sin categorías Added/Changed/Fixed
— la mayoría de releases de maly son una o dos piezas bien acotadas, y
separarlas en categorías no aportaría claridad extra. El razonamiento
completo detrás de cada decisión (qué se midió, qué se descartó y por qué)
vive en `CLAUDE.md`, pensado para quien vaya a tocar el código, no para
quien solo quiere saber qué cambió.

## v1.20.9 — 2026-10-18 (commit local autorizado; tag y publicación pendientes)

`playlist save <nombre>` guarda toda la cola como una playlist nueva, tanto
en CLI como en la consola. Conserva el orden normal y las repeticiones, aunque
haya filtro o shuffle, sin alterar la reproducción. Ctrl+S prepara la entrada
para escribir el nombre; el atajo es configurable. Cola vacía, nombre ocupado
o pistas sin indexar cancelan el guardado completo; el error indica cómo
escanear las pistas que faltan. Las otras TUIs reciben el refresco de playlists.

## v1.20.8 — 2026-10-18 (commit local; tag y publicación pendientes)

La paleta Ctrl+P permite encontrar comandos por nombre o descripción localizada,
ver sus argumentos y elegir con Tab/Shift+Tab. Enter copia la sugerencia sin
ejecutarla; tras completar sus argumentos, otro Enter la ejecuta. Conserva
entrada directa, historial con borrador, comillas y scroll de salida. Escape
regresa a edición, cancela una confirmación o cierra la paleta; cerrar y Enter
vacío descartan el borrado pendiente. Las sugerencias y ayudas se adaptan a
ventanas pequeñas. `get pick` también abre desde la consola el buscador para
elegir antes de descargar, igual que en la CLI.

## v1.19.8 — 2026-10-18 (pendiente de publicación)

- **Consultas locales coherentes.** Ctrl+O y `maly select` comparten con `/`
  los campos `a:`, `al:`, `t:`, `f:` y la negación `-live`, con ejemplos en
  la entrada y la ayuda. Buscan sobre los campos reales de la pista, incluido
  el nombre de archivo; las coincidencias difusas ya no cruzan campos y su
  orden puede cambiar. Enter y Tab conservan sus acciones. Playlists,
  búsqueda SQL de la CLI y consultas remotas mantienen su comportamiento.

## v1.19.7 — 2026-10-18 (pendiente de publicación)

- **Títulos, contraste y selección.** La ficha de la pista muestra hasta dos
  líneas de título según el espacio disponible. En ventanas pequeñas conserva
  metadatos, progreso, letras y salida. Biblioteca, cola y selectores tienen
  un marcador de selección visible sin color, distinto de la pista actual.
  Los defaults oscuros aclaran ayudas y pista actual; los configs existentes
  conservan sus colores, con instrucciones para adoptar los nuevos valores
  o una combinación medida para fondo claro.

## v1.19.6 — 2026-10-18 (pendiente de publicación)

- **Ayudas visibles y contextuales.** El pie anuncia acciones según el panel,
  la selección y las teclas configuradas; conserva ayuda y salida en ventanas
  pequeñas. Los mensajes vacíos y las instrucciones de los selectores se
  distribuyen en varias líneas, con Esc visible. La ayuda se agrupa por tareas,
  comparte sus teclas con `maly -h` y permite leer descripciones largas al
  desplazarse. Los errores y el progreso de escaneo conservan su prioridad.

## v1.19.5 — 2026-10-17 (pendiente de publicación)

- **Estados vacíos veraces.** Una biblioteca o cola poblada con un filtro sin
  resultados indica «sin coincidencias» y cómo limpiarlo. El selector sin
  playlists explica cómo crear una, también al escribir su nombre. La carga
  pendiente y los errores de lectura se distinguen de una colección vacía;
  los selectores de canciones y playlists conservan esa distinción, en inglés
  y español, sin escaneos automáticos ni cambios en los datos.

## v1.19.4 — 2026-10-17

- **Bordes alineados al mostrar símbolos con presentación de texto.** Nombres
  como `Kareto☕︎` desplazaban los paneles y la carátula una celda en Kitty.
  La TUI y los selectores `select`/`get pick` omiten el selector invisible
  VS15 solo al dibujar: se conserva el símbolo con su presentación
  predeterminada, y los metadatos, rutas y consultas mantienen el original.

## v1.19.3 — 2026-10-15

- **Varias canciones en un comando.** `play`, `add` y `playlist add` aceptan
  una consulta por argumento: `maly add Secreto "Otra dimension"`. Las
  coincidencias se agregan en el orden indicado, sin repetir una pista dentro
  del lote; las consultas sin resultados se nombran sin cancelar las demás.
  Si ninguna encuentra pistas, el comando falla. Los errores reales de la
  biblioteca cancelan el lote antes de modificar la cola o la playlist.
- **Las frases ahora requieren comillas:** `maly add daft punk` busca «daft»
  y «punk» por separado; `maly add "daft punk"` conserva la búsqueda de la
  frase. `search`, `select` y `get` mantienen su comportamiento anterior.
- La consola ctrl+p respeta comillas simples o dobles y avisa si quedan sin
  cerrar. La ayuda y el completado acompañan las consultas en lote. Tras
  actualizar, reinicia el servicio para que también entienda los lotes.

## v1.19.2 — 2026-10-14

- **La salida de la CLI se lee en terminales de fondo claro.** En `maly help`
  el verde de los comandos y el gris de las notas apenas se veían sobre un
  fondo claro. Ahora maly le pregunta al terminal de qué color es el fondo y
  usa colores oscuros cuando es claro. Lo mismo vale para `info`, `config`,
  `doctor`, `controls` y `logo`. Si el terminal no contesta (dentro de tmux,
  por ejemplo), se queda con los colores de siempre.

## v1.19.1 — 2026-10-05

- **Un filtro `/` nuevo.** Antes era una fila que empujaba la lista hacia
  abajo. Ahora es una caja flotante al pie del panel, y la lista se queda
  quieta. Puedes moverte por los resultados con `↑`/`↓` sin salir de la caja,
  ves cuántas pistas pasan de cuántas hay y las coincidencias salen
  subrayadas. Al confirmar con `enter`, el filtro pasa al borde del panel.
- **Filtrar es más potente:**
  - prefijos de campo: `a:` artista, `al:` álbum, `t:` título y `f:` nombre de
    archivo, para las pistas sin etiquetas;
  - `-palabra` excluye;
  - coincidencia difusa (`htsm` → "Hot Summer");
  - los mejores resultados van primero.
- **La posición en tu barra de medios ya no se arrastra de la canción
  anterior.** Con un cliente MPRIS abierto (la tarjeta de medios del
  escritorio), al pasar de canción el tiempo seguía contando desde la pista
  anterior durante toda la canción. Quien instaló desde el AUR ya tenía este
  arreglo desde el paquete 1.19.0-2; ahora llega a todos.

## v1.19.0 — 2026-09-06

La Phase 3 de la auditoría del 2026-09, que era la parte **no obligatoria**:
doce ideas con fundamento que no resolvían ningún problema que alguien
estuviera sufriendo. Se hicieron siete y **cinco se cerraron como decisiones
tomadas**, con su razón escrita, en vez de dejarlas en una lista para siempre.

Lo que vas a notar:

- **Si mpv se muere, maly te lo dice.** Antes, si el reproductor se caía por
  su cuenta, el servicio seguía en pie sin poder reproducir nada y no había
  dónde enterarse: `maly status` contestaba con normalidad y `maly doctor`
  daba todo en verde. Ahora la interfaz lo muestra en el pie, `maly status` lo
  avisa y `maly doctor` lo marca como fallo, con el remedio. maly no relanza
  mpv por su cuenta: eso sería decidir por vos si la música vuelve sola.
- **Escanear una biblioteca grande ya no "falla" a los diez minutos.** El
  cliente esperaba como mucho ese rato y después imprimía un error y salía
  con código 1 — mientras el servicio seguía escaneando y terminaba bien. Con
  80.000 pistas eso pasaba siempre. Ya no hay tope: el escaneo dura lo que
  dure tu biblioteca.
- **`maly playlist add` te dice cuántas pistas ya estaban.** Repetir el mismo
  comando con una consulta amplia duplicaba decenas de entradas diciendo
  exactamente lo mismo que la primera vez. Se sigue permitiendo repetir una
  pista a propósito; lo que cambia es que ahora lo dice.

Y por dentro: los comandos de la ventana gapless dejan de poder retener el
servicio cinco segundos si mpv se traba (medido: en condiciones normales
tardan 30 microsegundos), los colores del tema se validan por tabla en vez de
uno por uno, y el visualizador espera a su capturador como manda la
documentación de Go.

## v1.18.0 — 2026-09-06

Los ocho ítems de la Phase 2 de la misma auditoría. Esta tanda es sobre todo
estructura: casi nada de lo que cambia se ve, y lo poco que se ve, se nota.

Lo que sí se nota es el consumo. Mientras suena música, maly le manda a cada
ventana abierta una foto del estado cuatro veces por segundo, y esa foto
incluía **la cola entera** aunque la cola no hubiera cambiado. Con mil pistas
encoladas eran cuatro megas cada diez segundos, por ventana, para mandar
siempre lo mismo. Ahora la cola solo viaja cuando cambia de verdad: medido en
el mismo equipo y con la misma cola, de 4,28 MB a 0,11 MB en diez segundos. En
un portátil eso es batería.

El servicio también relee tu configuración al escanear, en vez de quedarse
para siempre con la que leyó al arrancar — que con el servicio activado
pueden ser semanas. Cambiar `music_dir` o `scan_durations` ya no exige
reiniciarlo, y `maly doctor` tiene una fila nueva que te dice si el servicio
está usando el mismo config que tú.

Un cambio de comportamiento pequeño y a propósito: los comandos que solo
CONSULTAN la biblioteca ya no la crean. `maly playlist list` sobre una
instalación recién hecha dejaba un archivo de biblioteca vacío en el disco
solo por preguntar; ahora te dice que todavía no hay biblioteca y que
escanees. Los comandos que sí escriben (crear una playlist, importar un m3u)
la siguen creando cuando hace falta.

Por dentro, tres piezas que existían dos veces pasan a existir una: las
decisiones de `maly get playlist`, que la CLI y la paleta de comandos
reimplementaban cada una por su lado y ya habían divergido; las cinco maneras
distintas de "abrir la biblioteca solo si existe"; y las cuatro listas de
ventanas modales de la interfaz que había que mantener sincronizadas a mano
—dos de ellas se olvidaban del buscador de descargas, y por eso con `ctrl+g`
abierto maly seguía decodificando carátulas y animando un banner que nadie
estaba viendo—. Y hay una red de tests nueva que compara el COMPORTAMIENTO de
la CLI contra el de la paleta, no solo sus nombres de comando: es la que
faltaba para que un arreglo no vuelva a aplicarse en una sola mitad de la
aplicación.

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

> **Esta versión no se publicó por separado.** Fue un puente: se quedó sin tag
> a propósito y todo lo de abajo llegó a los usuarios dentro de la **1.17.0**,
> al día siguiente. Si buscás un release `v1.16.4`, no existe.

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
