# Roadmap — v1.14.0 … v1.16.4 (2026-08-15 … 2026-09-05)

Parte del roadmap de ingeniería de maly, movido aquí desde `CLAUDE.md`
sin cambiar una palabra. Índice de todas las versiones y de qué explica
cada una: `docs/history/README.md`.

El rediseño de la pantalla principal (1.14.0/1.14.1), el buscador de
descargas (1.15.0/1.16.0/1.16.1), los P3 de la auditoría de UX (1.16.2),
la carrera de la promesa obsoleta (1.16.3) y la Phase 0 de la auditoría
técnica del 2026-09 (1.16.4).

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
