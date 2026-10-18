# Roadmap UI/UX — revisión de Codex del 2026-10-17

**Estado: UX-00 verificada y publicada en GitHub como `1.19.4`;
UX-01 con release local `1.19.5`; UX-02 verificada en el commit `9135bcd`
(`1.19.6`, sin tag); UX-03 implementada y verificada en `1.19.7`, commit autorizado; pendiente de tag.
UX-04–UX-09 pendientes. No se ha verificado publicación de estas entregas.**
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
| UX-03 | `1.19.6` → `1.19.7` | Títulos, contraste y selección | Pequeño: legibilidad dentro del reparto actual, sin rediseñar pantallas ni reproducción | Implementada y verificada; commit autorizado; tag pendiente |
| UX-04 | `1.19.7` → `1.19.8` | Consultas locales coherentes | Pequeño: amplía el selector de canciones con el lenguaje local existente; conserva sus acciones y no cambia CLI ni red | Pendiente |
| UX-05 | `1.19.8` → `1.20.8` | Paleta con sugerencias seleccionables | Grande: nuevo recorrido para descubrir y ejecutar acciones, con navegación, argumentos e historial coordinados | Pendiente |
| UX-06 | `1.20.8` → `1.20.9` | Guardar la cola como playlist | Pequeño si se mantiene acotado: compone una foto de la cola con la persistencia de playlists existente | Condicionada a diseño acotado |
| UX-07 | `1.20.9` → `1.20.10` | Continuidad después de descargar | Pequeño si se mantiene acotado: ofrece acciones existentes sobre los archivos obtenidos, sin gestor de trabajos nuevo | Condicionada a verificar el flujo |
| UX-08 | `1.20.10` → `1.21.10` | Reproducir después | Grande: modifica el orden efectivo de reproducción, su interacción con shuffle y la promesa gapless | Candidata; diseño pendiente |
| UX-09 | `1.21.10` → `1.22.10` | Deshacer una eliminación de cola | Grande: introduce recuperación con validez compartida entre clientes y demonio | Candidata; diseño pendiente |

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

**Cierre:** [ingeniería de 1.19.7](../history/roadmap-1.17.md#v1197-2026-10-18--ux-03-títulos-contraste-y-selección) y [evidencia](evidence/ux-03-1.19.7/README.md). Verificación gráfica en Kitty real; ratios sobre fondos de referencia sólidos, sin garantía para efectos del compositor. Commit autorizado; tag, publicación y despliegue pendientes.

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

- [ ] Mismos resultados esperados para campos/negación sobre la misma lista
  de pistas; probar también acentos, NFD, Unicode, nombres de archivo y vacíos.
- [ ] Coincidencias exactas por encima de difusas; comprobar que no cambia la
  pista seleccionada después de refrescos o de acciones con filtro.
- [ ] Medición antes/después con una biblioteca grande y consultas repetidas;
  rechazar recomputación innecesaria por frame. Registrar datos, no estimarlos.
- [ ] Describir el cambio de semántica en la documentación de uso en ambos
  idiomas, sin presentarlo como una nueva búsqueda online.

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

- [ ] Un usuario encuentra una acción por nombre o descripción localizada,
  completa sus argumentos y la ejecuta sin consultar el README.
- [ ] Historial, borrador, comillas, salida con scroll y confirmación de
  borrado de playlists conservan su comportamiento.
- [ ] Escape tiene un resultado definido en cada nivel; sugerencias y salida
  son legibles en 80×24 y permiten salir en 40×12.
- [ ] Metadatos sin deriva: comandos reales, ayuda y completado se validan
  contra expectativas, además de comparar listas entre sí.

## UX-06 — guardar la cola como playlist

**Oportunidad, no defecto reproducido.** No existe una acción dedicada en la
tabla revisada. Antes de implementar, fijar el contrato: propuesta inicial,
guardar **toda la cola en su orden visible normal**, incluidos duplicados,
en una playlist nueva; no guardar el ranking del filtro ni la permutación
shuffle. No sobrescribir otra playlist de forma implícita.

**Alcance y puntos de entrada:** lectura coherente de cola vía IPC,
[`playlist.go`](../../internal/library/playlist.go), CLI, consola y entrada TUI.
La biblioteca ya permite duplicados; no cambiar ese contrato ni deduplicar
silenciosamente. Definir qué hacer con rutas no indexadas antes de crear la
playlist: error accionable sin escritura parcial es el punto de partida.

**Por qué sí:** conserva una selección ya construida. **Límite:** no incorporar
cola pendiente, exportación de sesión ni un nuevo formato en esta entrega.
Si requiere nuevo modelo persistente o una API extensa, reclasificar el bump.

**Aceptación:**

- [ ] Preserva orden y repeticiones; no modifica cola, reproducción ni posición.
- [ ] Cola vacía, nombre ocupado y pista no indexada no dejan playlists parciales.
- [ ] Mutación concurrente: se guarda una foto coherente, nunca una mezcla de
  índices de dos estados. Los demás clientes reciben el refresco pertinente.
- [ ] Paridad de la operación en CLI/consola y acceso TUI; nombre de comando
  y atajo se fijan en diseño, no se dan por existentes en esta documentación.

## UX-07 — continuidad después de descargar

**Evidencia E11, estática.** El código informa y escanea tras obtener archivos;
la revisión no descargó contenido remoto. Reproducir éxito, archivo existente,
fallo y éxito parcial antes de confirmar el diseño.

**Alcance:** ofrecer ver/añadir/reproducir los archivos obtenidos después del
scan, sin arrancarlos automáticamente. Mantener referencias por ruta exacta,
no volver a resolver por título. Explicar que `✓` es coincidencia aproximada
de título, no identidad certificada de una grabación. En resultados largos,
estudiar detalle de la selección sin colorear trozos del `pickerItem.label`.

**Puntos de entrada:** [`get.go`](../../internal/tui/get.go), finalización de
`getDoneMsg` en [`tui.go`](../../internal/tui/tui.go), consola y `internal/getter`.

**Por qué sí:** evita buscar de nuevo lo recién obtenido. **Límite:** yt-dlp
sigue en el cliente, con sus interfaces de máquina; no parsear progreso humano,
no miniaturas de red, no gestor de trabajos ni subsistema de duplicados.

**Aceptación:**

- [ ] Acciones posteriores apuntan a los archivos correctos, incluso con
  títulos iguales; nunca disparan reproducción sin elección del usuario.
- [ ] Distinguir descarga nueva, archivo existente, ninguna descarga y fallo;
  un scan fallido no se anuncia como indexación terminada.
- [ ] Cancelación y cierre siguen sin dejar hijos; los errores permanecen
  visibles. Un yt-dlp falso valida el flujo, no prueba el servicio remoto.
- [ ] Si el detalle o la continuidad exigen una pantalla nueva amplia,
  revisar el alcance y su versión antes de seguir.

## UX-08 — reproducir después

**Candidata funcional.** Beneficio: elegir la próxima pista sin interrumpir
la actual ni mandarla al final. No existe esa operación dedicada en la base
revisada. No se implementa como «add + move» desde el cliente: dos peticiones
permiten intercalaciones y no definen el siguiente efectivo con shuffle.

**Diseño necesario:** una operación coherente del demonio, regla para varias
peticiones consecutivas (FIFO o LIFO), duplicados y repeat one. Conservar la
permutación y el compromiso de `PeekNext`; estudiar `queue`,
`daemon_playback.go`, `player` y la [ficha de reproducción](../architecture/playback.md).

**Por qué sí:** resuelve una intención frecuente. **Por qué puede esperar:**
afecta garantías delicadas ya comprobadas. El salto central reconoce ese
cambio de reproducción, aunque la interfaz sea una sola acción.

**Aceptación antes de cerrar:**

- [ ] Tabla de comportamiento acordada para shuffle on/off, repeat
  off/all/one, cola vacía, pausa, final de pista y llamadas consecutivas.
- [ ] Pruebas de la transición efectiva a la próxima pista, no solo de
  `Items`; conservar la no repetición del ciclo y actualizar gapless.
- [ ] Carrera entre final natural y petición de dos clientes cubierta;
  paridad CLI/consola/TUI y persistencia definida al reiniciar el servicio.

## UX-09 — deshacer una eliminación de cola

**Candidata funcional.** Beneficio: recuperar una pulsación accidental sin
confirmaciones en cada borrado. No convertirla en historial general de la app.

**Diseño necesario:** una sola eliminación recuperable, ámbito de la acción,
propietario del estado y condición explícita de caducidad. Evitar restaurar una
foto entera que pise cambios de otro cliente. `QueueGen` por sí sola no es un
contador de todas las mutaciones: también cambia por duraciones y no por cada
transición de reproducción; no asumir que sirve como token de validez.

**Por qué sí:** recuperación rápida. **Por qué puede esperar:** requiere un
contrato nuevo de estado compartido; deshacer contenido no implica volver atrás
en el tiempo de audio. Rechazarla si no se obtiene una regla sencilla de explicar.

**Aceptación antes de cerrar:**

- [ ] Definir conducta al quitar la pista actual, con shuffle, duplicados,
  salto de canción, reinicio y mutaciones desde otro cliente.
- [ ] Una acción caducada falla con explicación y no pisa la cola nueva.
- [ ] Volver a insertar no reinicia audio ni posición por accidente; UX y
  pruebas distinguen restaurar una entrada de restaurar reproducción.

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

Siguiente entrega tras UX-01: UX-02. El [prompt de continuidad](PROMPT-CONTINUIDAD-UI-UX.md)
se conserva como el encargo histórico de UX-01; el estado vigente es este
roadmap. El historial técnico permanece en `docs/history/`.

2026-10-18: UX-02 quedó en el commit `9135bcd` (`1.19.6`, sin tag). UX-03
implementada y verificada en `1.19.7`; cuatro entregas UI/UX realizadas,
publicación y despliegue de las últimas pendientes. Siguiente entrega: UX-04.
