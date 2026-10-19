# Roadmap UI/UX — revisión de Codex del 2026-10-17

**Estado: UX-00 verificada y publicada en GitHub como `1.19.4`;
UX-01 con release local `1.19.5`; UX-02 verificada en el commit `9135bcd`
(`1.19.6`, sin tag); UX-03 verificada en el commit `2363318` (`1.19.7`, sin tag);
UX-04 verificada en el commit `fb0db1b` (`1.19.8`, sin tag).
UX-05 verificada en `4b6d9ef` (`1.20.8`, sin tag). UX-06 verificada en
`0121e52` (`1.20.9`, sin tag). UX-07 implementada y verificada (`1.20.10`),
en `5806445`, sin tag. UX-08 implementada y verificada (`1.21.10`), con commit autorizado y tag pendiente. UX-09 implementada y verificada (`1.22.10`), commit local autorizado; tag pendiente. No se ha verificado publicación de estas entregas.**
Base de la revisión original: `v1.19.3`, commit `16bed5c`. El hotfix posterior
adelanta la corrección de los bordes; la secuencia futura parte de `1.19.4`.
Implementado no significa publicado: el cierre de release requiere commit
y tag anotado según [releases](../development/releases.md).

## Propósito y fuentes

Mejorar la claridad de los estados, el descubrimiento de acciones y el manejo
cotidiano de la música. Conservar la identidad visual y la arquitectura local
del reproductor. El objetivo no es completar todas las ideas por obligación.

- Este archivo es dueño del alcance, orden, estado y aceptación de cada entrega.
- [Guía de implementación](../development/implementacion-ui-ux.md): procedimiento,
  matriz de pruebas y ficha de cierre.
- [Evidencia de la revisión](evidence/ui-ux-2026-10-17/README.md): observaciones,
  capturas, condiciones y límites; no sustituye la reproducción actual.
- [Releases](../development/releases.md): fuente única de la regla de versión.
- [Decisiones cerradas](../decisions/no-hacer.md): restricciones que este plan
  no reabre.

## Secuencia prevista y justificación de versión

Secuencia si se ejecuta en este orden y no entran otras releases. Los IDs son
estables; los números se recalculan si cambia la base o el alcance. Las filas
condicionadas son candidatas, no compromisos de publicación.

| ID | Base → destino previsto | Entrega | Tamaño y justificación | Estado |
| --- | --- | --- | --- | --- |
| UX-00 | `1.19.3` → `1.19.4` | Bordes con VS15 | Pequeño: normalización de salida de la TUI, sin alterar datos ni interacción | Verificada; publicada en GitHub |
| UX-01 | `1.19.4` → `1.19.5` | Estados vacíos veraces | Pequeño: corrige la interpretación visual de datos existentes, sin nuevos comandos ni estado persistente | Verificada; release local v1.19.5, publicación pendiente |
| UX-02 | `1.19.5` → `1.19.6` | Ayudas visibles y contextuales | Pequeño: presenta acciones existentes según foco, teclas y espacio; conserva su comportamiento | Verificada; commit 9135bcd, tag pendiente |
| UX-03 | `1.19.6` → `1.19.7` | Títulos, contraste y selección | Pequeño: legibilidad dentro del reparto actual, sin rediseñar pantallas ni reproducción | Verificada; commit 2363318, tag pendiente |
| UX-04 | `1.19.7` → `1.19.8` | Consultas locales coherentes | Pequeño: amplía el selector de canciones con el lenguaje local existente; conserva sus acciones y no cambia búsqueda SQL ni red | Verificada; commit fb0db1b, tag pendiente |
| UX-05 | `1.19.8` → `1.20.8` | Paleta con sugerencias seleccionables | Grande: nuevo recorrido para descubrir y ejecutar acciones, con navegación, argumentos e historial coordinados | Verificada; commit 4b6d9ef, tag pendiente |
| UX-06 | `1.20.8` → `1.20.9` | Guardar la cola como playlist | Pequeño: compone una foto de la cola con la persistencia de playlists existente | Verificada; commit autorizado, tag pendiente |
| UX-07 | `1.20.9` → `1.20.10` | Continuidad después de descargar | Pequeño: reutiliza el selector y acciones existentes sobre rutas obtenidas, sin gestor de trabajos nuevo | Verificada con yt-dlp simulado; commit autorizado; tag pendiente |
| UX-08 | `1.20.10` → `1.21.10` | Reproducir después | Grande: modifica el orden efectivo de reproducción, su interacción con shuffle y la promesa gapless | Implementada y verificada; commit autorizado; tag pendiente |
| UX-09 | `1.21.10` → `1.22.10` | Deshacer una eliminación de cola | Grande: introduce recuperación con validez compartida entre clientes y demonio | Implementada y verificada; commit local autorizado; tag pendiente |

La primera tanda recomendada es UX-01–UX-03. Cada fila es una entrega coherente,
no un bump por archivo o commit. Un cambio que desborde la justificación se
divide o reclasifica antes de publicar. Si una candidata se descarta, se anota
la razón y las siguientes se recalculan sin dejar saltos artificiales.

## UX-00 — bordes con VS15

Hotfix incorporado después de la revisión inicial, a partir de la captura
del dueño y la reproducción de `Kareto☕︎` en el motor real de Kitty.
El [cierre de ingeniería de v1.19.4](../history/roadmap-1.17.md#v1194-2026-10-17--bordes-y-presentación-unicode)
contiene la causa, el costo tipográfico, las alternativas y la verificación.
El contrato de presentación pertenece a la ficha de arquitectura de la TUI.

**Aceptación:**

- [x] Alinear el borde de biblioteca y la fila completa con uno o dos VS15.
- [x] Mantener los originales en metadatos, rutas y valores del selector.
- [x] Cubrir la TUI principal, `select` y `get pick`.
- [x] Preservar VS16, ZWJ, acentos y datos gráficos de Kitty.
- [x] Regresión roja compilando al retirar el arreglo, verde al restaurarlo.
- [x] Contrastar cuatro tamaños, con/sin arte, con `Screen.draw` de Kitty.

## UX-01 — estados vacíos veraces

**Evidencia E01, E02 y E03.** Un filtro sin resultados muestra «biblioteca
vacía» o «cola vacía» aunque haya contenido; sin playlists, el picker propone
escanear una biblioteca que ya tiene canciones.

**Alcance:** distinguir carga pendiente, error de lectura, colección realmente
vacía y filtro sin coincidencias. Playlists debe usar su propio estado vacío.
Mantener los errores persistentes de lectura y el flash de primer arranque.
No convertir un error en un consejo genérico de escaneo.

**Puntos de entrada:** `libraryPanel` y `queuePanel` en
[`view.go`](../../internal/tui/view.go), `picker.render` en
[`picker.go`](../../internal/tui/picker.go), `openPlaylists`/`plView` en
[`playlists.go`](../../internal/tui/playlists.go), tabla de i18n.

**Por qué sí:** corrige mensajes falsos y evita acciones de recuperación
innecesarias. **Límite:** no añadir un asistente de instalación ni ejecutar
escaneos automáticamente.

**Aceptación:**

- [x] Con biblioteca/cola pobladas y cero coincidencias, mostrar «sin
  coincidencias» y una salida correcta para limpiar o editar el filtro.
- [x] Sin playlists, mostrar cómo crear una; no decir que faltan canciones.
- [x] Distinguir primera carga y error de DB de los vacíos legítimos.
- [x] Los mensajes funcionan en inglés y español; cubrir positivos y negativos
  con tests de comportamiento verificados en ambas direcciones.

**Cierre:** [ingeniería de 1.19.5](../history/roadmap-1.17.md#v1195-2026-10-17--ux-01-estados-vacíos-veraces) y [evidencia](evidence/ux-01-1.19.5/README.md). Cierre local autorizado con commit y tag anotado `v1.19.5`; publicación pendiente.

## UX-02 — ayudas visibles y contextuales

**Evidencia E04 y E05.** La acción útil del primer arranque se corta en
«biblioteca vacía — ejec…». En 80×24 el pie pierde atajos, y los selectores
recortan su instrucción de salida. El pie fijo tampoco deriva de `m.keys`.

**Alcance:** envolver mensajes de estado en el espacio disponible; componer
el pie por foco y ancho con las teclas resueltas. Sobre artistas/álbumes,
Enter anuncia expandir; sobre pistas, reproducir. En columna única debe
descubrirse el otro panel. Agrupar la ayuda por tareas conservando `HelpRows`
como fuente compartida con la CLI; no crear otra lista de atajos.

**Puntos de entrada:** [`view.go`](../../internal/tui/view.go),
[`helprows.go`](../../internal/tui/helprows.go), hints de pickers e i18n.
Respetar el presupuesto de filas de [`layout.go`](../../internal/tui/layout.go).

**Por qué sí:** las acciones existentes se pueden descubrir sin memorizar.
**Límite:** el footer no es la ayuda completa; errores, scan y avisos conservan
su prioridad. No reservar filas de más sin medir cuánto pierde la lista.

**Aceptación:**

- [x] En 80×24 se descubre cómo buscar, añadir, cambiar de panel, pedir ayuda
  y cerrar la superficie actual; puede repartirse entre pie y ayuda visible.
- [x] En 40×12 se conservan salida y acceso a ayuda; el mínimo no obliga a
  meter todos los atajos en una fila.
- [x] Presets y remapeos aparecen con sus teclas reales, incluyendo espacio
  y nombres largos. CLI y ayuda TUI mantienen paridad.
- [x] Un flash o un scan no provoca desbordes ni oculta permanentemente las
  instrucciones. Se comprueba contenido y posición, no solo ancho/alto.

**Cierre:** [ingeniería de 1.19.6](../history/roadmap-1.17.md#v1196-2026-10-18--ux-02-ayudas-visibles-y-contextuales) y [evidencia](evidence/ux-02-1.19.6/README.md). Sin filas permanentes adicionales; commit `9135bcd`; tag, publicación y despliegue pendientes. Siguiente entrega: UX-03.

## UX-03 — títulos, contraste y selección

**Evidencia E06–E08.** La ficha recorta el título a una línea aun con altura
libre; el texto secundario requiere más contraste sobre el fondo de referencia;
con `NO_COLOR` la selección depende de estilos que desaparecen.

**Alcance:** hasta dos líneas de título donde quepan, con degradación explícita
en ventanas pequeñas; marcador no cromático de selección distinto del de pista
actual; ajustar legibilidad de ayudas y metadatos manteniendo la paleta.

**Puntos de entrada:** `npMetaText` en
[`nowplaying.go`](../../internal/tui/nowplaying.go), presupuesto de ficha en
[`nowcolumn.go`](../../internal/tui/nowcolumn.go), [`styles.go`](../../internal/tui/styles.go),
[`config.go`](../../internal/config/config.go), render de filas y pickers.

**Por qué sí:** facilita identificar música y navegar. **Límite:** no ensanchar
columnas automáticamente, no agregar marquesinas, no hacer que toda la pantalla
tenga el mismo énfasis. No sobrescribir temas existentes: cambiar defaults no
actualiza configs ya creados; documentar ese límite al cerrar la entrega.

**Aceptación:**

- [x] Título largo legible en dos líneas cuando hay presupuesto; artista,
  álbum, carátula, letras y progreso no se desplazan fuera de sus áreas.
- [x] Selección y pista actual se distinguen cuando coinciden y cuando no,
  también sin color y en modo de una columna.
- [x] Medir texto informativo contra fondos de referencia oscuro y claro;
  objetivo orientativo de contraste ≥4,5:1. Registrar los colores medidos,
  sin prometer ese resultado para cualquier transparencia o tema personalizado.
- [x] Validar un config previo sin modificarlo y uno nuevo; revisar Kitty
  real con carátula antes de afirmar aceptación gráfica completa.

**Cierre:** [ingeniería de 1.19.7](../history/roadmap-1.17.md#v1197-2026-10-18--ux-03-títulos-contraste-y-selección) y [evidencia](evidence/ux-03-1.19.7/README.md). Verificación gráfica en Kitty real; ratios sobre fondos de referencia sólidos, sin garantía para efectos del compositor. Commit `2363318`; tag, publicación y despliegue pendientes.

## UX-04 — consultas locales coherentes

**Evidencia E09.** `a:Áurea` encuentra cuatro pistas con `/` y ninguna con
Ctrl+O: son motores diferentes, no una biblioteca que pierde canciones.

**Alcance:** reutilizar la semántica de campos y negación del filtro local en
el selector de canciones y en `maly select`, conservando Enter/Tab. Ofrecer
ejemplos descubribles de `a:`, `al:`, `t:`, `f:` y negación. Usar campos de la
pista, no interpretar prefijos sobre una etiqueta concatenada.

**Puntos de entrada:** [`filter.go`](../../internal/tui/filter.go),
[`songs.go`](../../internal/tui/songs.go), [`select.go`](../../internal/tui/select.go)
y picker compartido. No cambiar el filtrado de playlists ni los resultados
remotos al adaptar el picker.

**Por qué sí:** se aprende un lenguaje local reutilizable. **Coste:** puede
cambiar el ranking que conoce el usuario. No basta con que dos buscadores
coincidan: cada resultado tiene que ser correcto. Se conserva la búsqueda SQL
de la CLI y la consulta remota de yt-dlp.

**Aceptación:**

- [x] Mismos resultados esperados para campos/negación sobre la misma lista
  de pistas; probar también acentos, NFD, Unicode, nombres de archivo y vacíos.
- [x] Coincidencias exactas por encima de difusas; comprobar que no cambia la
  pista seleccionada después de refrescos o de acciones con filtro.
- [x] Medición antes/después con una biblioteca grande y consultas repetidas;
  rechazar recomputación innecesaria por frame. Registrar datos, no estimarlos.
- [x] Describir el cambio de semántica en la documentación de uso en ambos
  idiomas, sin presentarlo como una nueva búsqueda online.

**Cierre:** [ingeniería de 1.19.8](../history/roadmap-1.17.md#v1198-2026-10-18--ux-04-consultas-locales-coherentes) y [evidencia](evidence/ux-04-1.19.8/README.md). Campos reales y selección por ruta; benchmark de 40.000 pistas y matriz tmux es/en. Commit `fb0db1b`; pendientes tag, publicación y despliegue.

## UX-05 — paleta con sugerencias seleccionables

**Evidencia E10.** Ctrl+P tiene historial y ejecución directa; al escribir
`pl` no ofrece sugerencias. La capacidad existe, pero exige conocer su sintaxis.

**Alcance:** selección por teclado de comando, descripción y argumentos
esperados; conservar entrada directa, historial y salida de consola. Empezar
por comandos existentes. La paleta puede rellenar una plantilla para una
acción incompleta; no debe ejecutarla hasta tener argumentos válidos.

**Puntos de entrada:** [`console.go`](../../internal/tui/console.go),
[`commands.go`](../../cmd/maly/commands.go), completado y claves de ayuda.
Extraer solo los metadatos compartidos necesarios: la TUI no puede importar
`package main`, y no se reestructura el dispatch del demonio por esta función.

**Por qué sí:** nuevo recorrido para usuarios que no recuerdan comandos.
**Coste que justifica el salto central:** conviven selección, texto, historial
y confirmaciones. Resolver sus teclas antes de escribir el código; no introducir
una tercera lista manual ni ejecutar un borrado por seleccionar una sugerencia.

**Aceptación:**

- [x] Un usuario encuentra una acción por nombre o descripción localizada,
  completa sus argumentos y la ejecuta sin consultar el README.
- [x] Historial, borrador, comillas, salida con scroll y confirmación de
  borrado de playlists conservan su comportamiento.
- [x] Escape tiene un resultado definido en cada nivel; sugerencias y salida
  son legibles en 80×24 y permiten salir en 40×12.
- [x] Metadatos sin deriva: comandos reales, ayuda y completado se validan
  contra expectativas, además de comparar listas entre sí.

**Resultado (2026-10-18):** nombre o descripción localizada → Tab/Shift+Tab →
Enter rellena → argumentos manuales → Enter ejecuta. ↑/↓ mantienen historial y
borrador. Escape retira la selección, cancela confirmación o cierra; Enter vacío
cancela el borrado. Metadatos compartidos de comandos/subcomandos y paridad de
`get pick` corregida al contrastar la sugerencia con la ejecución real.
[Cierre de ingeniería](../history/roadmap-1.17.md#v1208-2026-10-18--ux-05-paleta-con-sugerencias) y
[evidencia](evidence/ux-05-1.20.8/README.md). Commit autorizado: `4b6d9ef`; sin tag, push ni despliegue.

## UX-06 — guardar la cola como playlist

**Implementada y verificada en 1.20.9; commit autorizado, tag pendiente.**
Base limpia `4b6d9ef` (1.20.8). La reproducción previa confirmó que `save` no
existía; era una oportunidad, no un defecto. El control `playlist create/list`
funcionaba en el mismo sandbox.

**Contrato fijado antes de implementar:** `playlist save <nombre>` guarda una
foto de **toda la cola en su orden normal**, con duplicados, en una playlist
nueva. La foto se toma al ejecutar, no al abrir el acceso. Filtro y shuffle no
cambian lo guardado. No sobrescribe. Cola vacía, nombre ocupado o ruta sin indexar
fallan sin playlist parcial; en el último caso indica cómo escanear y reintentar.
Ctrl+S (`save_queue`, configurable) abre la paleta con el comando preparado.

**Alcance:** lectura coherente de cola por IPC existente, transacción en
`internal/library`, composición compartida en `internal/playlist`, CLI, consola
y acceso TUI. Sin nueva operación IPC, esquema de DB, formato, cola pendiente
ni exportación de sesión. El bump **1.20.8 → 1.20.9** sigue siendo pequeño: una
operación acotada sobre cola y persistencia existentes.

**Aceptación:**

- [x] Orden y repeticiones intactos; pausa/posición y reproducción conservadas.
- [x] Vacío, nombre ocupado y pista sin indexar no dejan playlists parciales;
  también se revierte un fallo SQL después de insertar la primera entrada.
- [x] Mutaciones concurrentes producen fotos coherentes; dos conexiones que
  compiten por un nombre no mezclan entradas. Otro cliente recibe el refresco.
- [x] CLI/consola comparten la operación, con atajo configurable, ayuda y
  completado. Matriz es/en, cuatro tamaños y redimensionado, default/vim/override,
  color/NO_COLOR, servicio privado y demonio embebido.

[Cierre de ingeniería](../history/roadmap-1.17.md#v1209-2026-10-18--ux-06-guardar-la-cola)
y [evidencia reproducible](evidence/ux-06-1.20.9/README.md).

## UX-07 — continuidad después de descargar

**Implementada y verificada en 1.20.10; commit autorizado; tag pendiente.**
La evidencia E11 era estática. La reproducción contra `0121e52` confirmó que un
archivo existente podía quedar sin indexar, y que un fallo parcial ignoraba el
audio conservado. La TUI anunciaba la descarga antes de comprobar el scan.

**Contrato implementado:** tras scan, el selector existente ofrece los archivos
obtenidos: filtro local, Tab añade, Enter reproduce, Esc cierra. Las acciones
usan rutas exactas; terminar una descarga no reproduce. Nuevos, existentes y
parciales se escanean. Fallo de indexación mantiene las acciones bloqueadas y
ofrece Ctrl+R; los errores se conservan en consola. CLI ofrece comandos por
ruta, y `get playlist` presenta los archivos de la playlist creada. `✓` anuncia
similitud de título, nunca identidad certificada.

**Alcance pequeño, 1.20.9 → 1.20.10:** compone scan, biblioteca, picker y acciones
existentes; no cambia protocolo, esquema ni reproducción. Se estudió ampliar el
detalle del resultado remoto: se conserva el selector de ancho adaptativo y sus
etiquetas planas, sin panel adicional ni colores incrustados. La continuidad local
prioriza nombre de archivo e identidad exacta. No se promete mostrar completo un
título arbitrariamente largo a 40 columnas. Sin miniaturas, progreso parseado,
gestor de trabajos ni sistema de duplicados.

**Aceptación:**

- [x] Rutas correctas con metadatos iguales, añadir/reproducir por elección y
  cola intacta al terminar descargas.
- [x] Nueva, existente, vacía, fallo y parcial diferenciados; scan fallido sin
  acciones y recuperación comprobada.
- [x] Cancelar búsqueda/descarga y cerrar durante búsqueda no deja vivo el
  proceso simulado. Los errores permanecen visibles. No se probó descarga real
  remota ni el árbol de subprocessos de yt-dlp/ffmpeg real.
- [x] Reutilización acotada del selector; no hizo falta una pantalla nueva amplia.

[Cierre de ingeniería](../history/roadmap-1.17.md#v12010-2026-10-19--ux-07-continuidad-después-de-descargar)
y [evidencia reproducible](evidence/ux-07-1.20.10/README.md).

## UX-08 — reproducir después

**Implementada y verificada en 1.21.10 sobre UX-07 (1.20.10, `5806445`).
Commit autorizado; tag pendiente.**
El dueño autorizó UX-08 y eligió FIFO: pedir B y después C significa B → C.
`playnext` es una operación del demonio; no una secuencia add/move del cliente.

| Situación | Contrato |
| --- | --- |
| Reproducción / pausa | Conservar pista, posición y pausa; encolar sin cargar audio |
| Cola vacía / detenida | Preparar entradas sin arrancar sonido; play/next es explícito |
| Shuffle off/on | Las pendientes FIFO tienen prioridad; las demás conservan su orden/ciclo |
| Repeat off/all | Tras agotar pendientes, seguir el modo normal |
| Repeat one | Las elecciones explícitas pasan primero; después se repite la última actual |
| Varias llamadas | FIFO por orden de aceptación bajo el mutex; cada lote es indivisible |
| Duplicados | Cada petición crea entradas nuevas, aunque la ruta ya esté en cola; dentro de un lote de consultas se deduplican solapamientos, como add |
| Next / jump / prev | Next consume la primera pendiente; seleccionar una pendiente por jump/prev consume esa entrada, sin borrar las demás |
| Remove / move / clear / play con consulta | Remove cancela esa entrada; move conserva su identidad prioritaria; clear/reemplazo borran prioridades |
| Fin natural concurrente | Orden serial bajo d.mu. Una prioridad aceptada antes del callback gana; si mpv ya encadenó la promesa antigua se recarga la elegida, sin saltarse una pendiente |
| Reinicio | Guardar prioridades por entrada; restaurar en pausa, remapear archivos desaparecidos; ciclo shuffle ordinario se regenera como antes |

CLI/consola: `playnext <consulta|ruta> [más…]`. TUI: Ctrl+N (`play_next`,
configurable) sobre biblioteca, cola, canciones y archivos descargados; en la
cola crea una nueva entrada, no mueve la original. En playlists Ctrl+N mantiene
su acción local de crear playlist. `maly select` comparte el acceso. La primera
pendiente se marca como «después»/«next» en las tres representaciones de cola.

**Alcance grande:** prioridad persistente compartida entre clientes, interacción
con shuffle/repeat y reparación de la transición gapless. **1.20.10 → 1.21.10**
conserva el último componente. Sin cambios de esquema de biblioteca ni red.

**Aceptación antes de cerrar:**

- [x] Tabla de comportamiento acordada para shuffle on/off, repeat
  off/all/one, cola vacía, pausa, final de pista y llamadas consecutivas.
- [x] Pruebas de la transición efectiva a la próxima pista, no solo de
  `Items`; conservar la no repetición del ciclo y actualizar gapless.
- [x] Carrera entre final natural y petición de dos clientes cubierta;
  paridad CLI/consola/TUI y persistencia definida al reiniciar el servicio.

Cierre y límites en [ingeniería](../history/roadmap-1.17.md#v12110-2026-10-19--ux-08-reproducir-después)
y [evidencia reproducible](evidence/ux-08-1.21.10/README.md).

## UX-09 — deshacer una eliminación de cola

**Implementada y verificada en 1.22.10 sobre 7d68623 (1.21.10).
Commit local autorizado; tag pendiente.**
El dueño aprobó una sola recuperación compartida. El demonio conserva la
última entrada quitada; no una foto completa de la cola ni un historial.

| Situación | Contrato |
| --- | --- |
| Undo | Reinsertar la entrada en su posición normal, sin cargar audio, cambiar pausa ni retroceder posición |
| Actual eliminada | Conservar la nueva actual; restaurar contenido no vuelve a la canción anterior |
| Última entrada | Recuperar la cola sin arrancarla; play es explícito |
| Shuffle | Recuperar su lugar en el ciclo; la antigua actual queda ya consumida antes de la nueva actual |
| Prioridad UX-08 | Recuperar el lugar FIFO de la entrada quitada; no resucitar prioridades ya consumidas al quitar la actual |
| Duplicados | Recuperar exactamente esa entrada, aunque comparta ruta o metadatos |
| Otra eliminación | Sustituye la recuperación anterior; una TUI con token anterior falla sin tocar la nueva |
| Caducidad | Add/playnext/move efectivo/clear/reemplazo, salto/avance/prev, stop, ajustar repeat/cambiar shuffle o reinicio |
| Sin caducidad | Pausa/reanudar la misma pista, seek, volumen, ticks, lectura, filtros, scan/duraciones; no hay temporizador |
| Dos clientes | Un solo ganador bajo d.mu; token opaco por eliminación, distinto incluso tras reinicio |
| CLI/consola | `undo` recupera la última eliminación vigente, con explicación si no existe |
| TUI | `u` configurable (`undo_remove`), en la pantalla principal; usa el token observado y muestra disponibilidad |

No persiste la posibilidad de deshacer. El archivo de sesión y SQLite no
cambian. Una petición caducada no borra una recuperación más reciente.
**Alcance grande, 1.21.10 → 1.22.10:** nuevo estado compartido y contrato de
validez entre clientes y demonio; conserva el último componente.

**Aceptación antes de cerrar:**

- [x] Definir conducta al quitar la pista actual, con shuffle, duplicados,
  salto de canción, reinicio y mutaciones desde otro cliente.
- [x] Una acción caducada falla con explicación y no pisa la cola nueva.
- [x] Volver a insertar no reinicia audio ni posición por accidente; UX y
  pruebas distinguen restaurar una entrada de restaurar reproducción.

Cierre y límites en [ingeniería](../history/roadmap-1.17.md#v12210-2026-10-19--ux-09-deshacer-una-eliminación)
y [evidencia reproducible](evidence/ux-09-1.22.10/README.md).

## Ajustes tras la revisión de escritorio — 1.22.11

Encargo adicional explícito del dueño, posterior a UX-09: corregir D1/D2/D3 de
la revisión y hacer más clara la ayuda mostrada en su captura. Implementado y
verificado; no añade otra entrega UX numerada. Ayuda en seis tarjetas, dos
columnas adaptables, salida de paleta completa, diagnóstico de descarga retenido
y aviso mínimo visible. [Cierre y evidencia](evidence/desktop-fixes-1.22.11/README.md).
Commit/tag pendientes, junto con UX-09; no se autorizó publicación ni despliegue.

## Fuera de alcance y cierre de decisiones

No se reabren ratón, Matugen, miniaturas remotas, marquesinas, `playlist move`,
red propia ni traslado de descargas al demonio. Streaming, cuentas y
recomendación automática quedan fuera de este ciclo. Favoritos no necesita un
modelo separado mientras una playlist cubra la necesidad.

Si una propuesta deja de tener fundamento, cerrarla aquí con su evidencia.
Si cambia una decisión general del proyecto, actualizar su dueño en
`docs/decisions/`; no convertir los pendientes en deuda perpetua.

## Registro de ejecución

2026-10-17, revisión inicial: documentación terminada, todavía sin entregas
implementadas en ese momento.

2026-10-17, cierre de UX-00: **1 entrega implementada y verificada**, versión
`1.19.4`, commit `2031b73`. Se comprobó por HTTPS que `main` y el tag anotado
`v1.19.4` públicos de GitHub apuntan al mismo commit. El paquete `1.19.4-1`
pasó `makepkg`; su commit local es `769eef9` en el repositorio independiente
del PKGBUILD. El dueño informó el push al AUR, sin comprobación independiente
posterior de ese remoto en esta sesión. El razonamiento del arreglo está en
la [entrada histórica](../history/roadmap-1.17.md#v1194-2026-10-17--bordes-y-presentación-unicode).

2026-10-17, cierre de implementación de UX-01: **2 entregas implementadas y
verificadas**, const local `1.19.5`. [Cierre de ingeniería](../history/roadmap-1.17.md#v1195-2026-10-17--ux-01-estados-vacíos-veraces).
Cierre local autorizado con commit y tag anotado `v1.19.5`; publicación y
despliegue pendientes. El PKGBUILD se aplaza hasta terminar de probar las
próximas subidas, por instrucción del dueño.

Siguiente entrega tras UX-01 en ese momento: UX-02. El historial técnico
permanece en `docs/history/`; el [prompt de continuidad](PROMPT-CONTINUIDAD-UI-UX.md)
se actualiza con el punto de reanudación vigente.

2026-10-18: UX-02 quedó en el commit `9135bcd` (`1.19.6`, sin tag). UX-03
implementada y verificada en `1.19.7`; cuatro entregas UI/UX realizadas,
publicación y despliegue de las últimas pendientes. Siguiente entrega: UX-04.


2026-10-18, pausa de cierre solicitada por el dueño: UX-03 commiteada en
`2363318` y UX-04 en `fb0db1b` (`1.19.8`), **cinco entregas implementadas y
verificadas contando UX-00**. Sin tags nuevos, push, PKGBUILD ni despliegue.
UX-05 no iniciada; será la siguiente entrega al reanudar. Resumen de sesión y
límites en el [cierre histórico](../history/roadmap-1.17.md#sobre-1198-2026-10-18--cierre-de-sesión-y-pausa).

2026-10-18, reanudación: UX-05 implementada y verificada en `1.20.8`. Seis entregas contando
UX-00; commit/tag esperan autorización de cierre. UX-06 sigue condicionada y no
iniciada. Publicación, PKGBUILD y despliegue continúan pendientes.

2026-10-18, continuación autorizada por el dueño: UX-05 quedó en `4b6d9ef`
(solo commit, sin tag). UX-06 implementada y verificada en `1.20.9`: siete
entregas contando UX-00. El dueño autorizó su commit de cierre; tag pendiente. UX-07 no iniciada.
Publicación, PKGBUILD y despliegue continúan pendientes.


2026-10-19, continuación autorizada por el dueño: UX-06 quedó en `0121e52`
(solo commit, sin tag). UX-07 implementada y verificada en `1.20.10`, ocho
entregas contando UX-00. Commit/tag esperan autorización; UX-08 no iniciada.
Aceptación local con yt-dlp simulado; publicación y despliegue pendientes.


**2026-10-19 — UX-08 verificada.** Base HEAD `0121e52` con UX-07 sin commit.
El dueño eligió FIFO B → C. Const 1.21.10; nueve entregas contando UX-00.
Build/vet/suite/race correctos, once contrapruebas y matriz de terminal con
mpv real en entorno privado. UX-07 y UX-08 quedaron pendientes de commit/tag al terminar la verificación;
UX-09 no iniciada. No se publica ni despliega automáticamente.


**2026-10-19 — cierre autorizado en commits separados.** UX-07 quedó en
`5806445` (1.20.10); UX-08 se registra en el commit siguiente (1.21.10).
Se contrastaron ambas instantáneas con las fuentes verificadas, conservando
las evidencias originales. Tags, publicación y despliegue pendientes; UX-09 no iniciada.


**2026-10-19 — UX-09 verificada.** Base limpia `7d68623` / 1.21.10;
const 1.22.10. El dueño aprobó una recuperación compartida con caducidad.
Build/vet/suite/race correctos (910 casos/subcasos por suite, sin saltos), diez
contrapruebas y matriz TUI con dos clientes y mpv privado. Quedan implementadas
y verificadas las diez entregas UX-00–UX-09, con los límites particulares
registrados (incluido yt-dlp simulado en UX-07). Commit/tag de UX-09, publicación
y despliegue pendientes; no se inicia trabajo fuera de este ciclo.
