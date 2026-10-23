# Roadmap de ingeniería — v1.17.0 en adelante

Índice: [`README.md`](README.md).

## v1.22.12 (2026-10-23) — captura PCM del visualizador

**Estado:** commit y tag anotado locales; publicación pendiente.
**1.22.11 → 1.22.12 es un cambio pequeño:** se explicita el formato de la
captura existente, conservando FFT, suavizado, configuración, reproducción e
IPC. No se cambia el modelo de interacción ni se introduce estado compartido.

Corregido el contrato de `pw-record`: se pide `--raw` porque stdout entregaba
AU con PCM de 16 bits big-endian y `readLoop` lo leía como s16le, convirtiendo
la señal en ruido para la FFT. Capturas reales simultáneas del binario
instalado y el corregido, con el mismo Go 1.27.3, confirmaron la diferencia.
Prueba de muestras/espectro en ambas direcciones, suite completa sin CGo,
race, vet y build correctos. La release incorpora bump, changelog y tag anotado.
[Diagnóstico, capturas y límites](../planning/evidence/viz-pcm-2026-10-23/README.md).
La misma revisión reparó Super+M en la configuración privada del dueño;
ese cambio de Hyprland no forma parte del código distribuido de maly.

## Sobre v1.22.11, sin bump de versión (2026-10-19) — revisión de README

Se contrastaron los README español e inglés con el código y el workflow de CI.
Se completaron los resúmenes de funciones, Ctrl+N, el mínimo 40×12 y los paquetes
nuevos; se corrigieron las instrucciones de race, completions y reintentos del
visualizador. Se aclaró que las capturas son anteriores y que el código de una
rama puede incluir cambios pendientes de publicación. Sin bump: solo documentación,
sin cambios del binario. Verificación: enlaces locales de ambos README y
`git diff --check` correctos; `dbus-run-session -- go test ./...` pasó completo
fuera del sandbox, después de que este bloqueara los sockets Unix de los tests.

## v1.22.11 (2026-10-19) — ayuda por tareas y detalles de escritorio

**Estado:** implementada y verificada; commit local autorizado; tag pendiente. Encargo explícito
tras la [revisión de escritorio](../planning/evidence/desktop-1.22.10/README.md)
y una captura del dueño mostrando la ayuda demasiado densa.

**Base y alcance.** Los arreglos se verificaron sobre el árbol 1.22.10 de
UX-09, con HEAD entonces en `7d68623`. UX-09 quedó después en su commit
separado `6d5b1f6`. Respaldo completo del trabajo previo en
`/tmp/maly-polish-before`. **1.22.10 → 1.22.11 es un cambio pequeño:** presentación
y diagnóstico locales, sin modificar reproducción, IPC, configuración ni datos.
El número central y los contratos de UX-09 se conservan.

**Cambios.** `HelpRows` sigue siendo la fuente única de TUI/CLI, ahora con seis
secciones. La TUI las presenta como tarjetas con cabeceras contrastadas y teclas
en negrita; geometría pura integrada en `computeLayout`, dos columnas si caben,
una con scroll en estrecho. Se separaron las instrucciones de la paleta y las
condiciones de deshacer en líneas legibles. `maly -h` indenta las continuaciones.

D1: la consola envuelve antes de paginar, conserva los estilos y cuenta filas
visuales en PgUp/PgDn/Ctrl+Home/End. Redimensionar vuelve a envolver el historial
original y acota el desplazamiento. D2: `tea.Exec` mantiene el proceso interactivo
y copia stdout/stderr a un buffer con mutex de 16 KiB; al fallar conserva hasta
40 líneas finales saneadas en el historial de la paleta. No interpreta mensajes
de yt-dlp, no decide el éxito a partir de ellos ni escribe logs persistentes.
Funciona en pista y playlist; la CLI conserva su salida directa y ambos clientes
retiran la indicación ambigua «revisa arriba». D3: el mínimo 40×12 va primero y
el aviso se ajusta al ancho disponible.

**Verificación.** Build sin CGo, vet, suite y race: 19 paquetes y 916 casos/subcasos,
sin fallos ni saltos, con mpv y D-Bus privados. Seis contrapruebas compilaron,
fallaron por el defecto neutralizado y pasaron al restaurar mediante `cp`.
Kitty/Hyprland reales: es/en, default/Vim/atajo personalizado, color/NO_COLOR,
ayuda ancha y mínima, error largo, 404 real con yt-dlp y descarga correcta.
La prueba remota de este cierre se limita a HTTP local, sin cookies ni escucha.

[Evidencia y capturas](../planning/evidence/desktop-fixes-1.22.11/README.md).
No se cambia el tema personal ni el binario instalado; no hay tag, push,
empaquetado o reinicio del servicio real.

## v1.22.10 (2026-10-19) — UX-09: deshacer una eliminación

**Estado:** implementada y verificada; commit local autorizado; tag pendiente.
[Contrato y aceptación](../planning/roadmap-ui-ux.md#ux-09--deshacer-una-eliminación-de-cola)
y [evidencia reproducible](../planning/evidence/ux-09-1.22.10/README.md).

**Base y decisión.** `main` limpio en `7d68623` (UX-08, 1.21.10), UX-07 en
`5806445`; último tag local v1.19.5. Sin consultar remoto. Copia exacta en
`/tmp/maly-ux09-backup`; el binario previo rechaza `undo` como subcomando
desconocido. El dueño aprobó una sola recuperación compartida: conservar
pausa/volumen/reloj, caducar por cambios de cola/pista o reinicio. Se concretan
stop y ajustes de modos como invalidaciones, sin temporizador arbitrario.

**Antes → después.** RemoveAt conserva una entrada y sus posiciones, no una
foto de la cola. `undo` en CLI/consola la reinserta; `u` configurable en TUI
manda el token que observó. Un token viejo falla sin consumir una eliminación
nueva. No depende de QueueGen: aprender duraciones no invalida, y un EOF sí,
incluso al agotar la cola. No se guardan tokens en sesión. La entrada recupera
su lugar en shuffle y prioridad FIFO; la antigua actual ya quedó consumida
antes de la nueva actual. No se carga audio por recuperar contenido, tampoco
si la cola quedó vacía. El pie anuncia la acción disponible a 40 columnas.
Contrato completo y arquitectura en las fichas de cola, daemon y TUI.

**Versión 1.21.10 → 1.22.10, cambio grande.** Introduce estado compartido con
validez entre clientes y reproducción; conserva el último componente. No hay
migración de SQLite/sesión, nuevas dependencias ni red. Se descartó un historial
por cliente y restaurar instantáneas completas: complicaban propiedad/validez
y podían sobrescribir modificaciones ajenas. El identificador aleatorio por
eliminación evita reutilización tras reinicio; no pretende ser autenticación.

**Verificación.** Build sin CGo, vet, suite completa sin CGo y race correctos:
19 paquetes, 910 casos/subcasos por suite, cero fallos/saltos, mpv `ao=null`
y bus privado. Pruebas de cola con shuffle on/off, todas las posiciones de
actual/eliminada, duplicados por ruta distinguidos por ID, conservación de
duraciones aprendidas, prioridad FIFO, cola única y caducidad por mutador.
Pruebas de daemon con pausa/seek/volumen y LoadCount intacto, dos clientes IPC
con un solo ganador, token nuevo tras reinicio, final natural a otra pista y
fin de cola. CLI/consola validan argumentos; TUI prueba token observado, falta
de disponibilidad y ayuda resuelta a 40 columnas.

Diez neutralizaciones compilables fallaron por la aserción esperada y pasaron
tras restaurar con `cp`: registro, token, identidad actual, FIFO, slot shuffle,
caducidad al añadir, fin de cola, audio recargado por undo, token TUI y gramática
CLI. Las suites se ejecutaron después de terminar las neutralizaciones.

Matriz tmux: es/default/color, en/vim/NO_COLOR y es/Ctrl+Y/NO_COLOR;
140×42 → 100×28 → 80×24 → 40×12 → 140×42. Dos TUIs, daemon embebido y servicio
privado separado. Recuperación compartida desde la segunda TUI, filtro por
ruta, consola/CLI, rechazo de token viejo sin pisar el nuevo, quitar actual y
restaurar sin cambiar la ruta efectiva de mpv, caducidad tras mutación y reinicio.
Capturas de texto/ANSI; no se acredita audio audible ni rasterizado Kitty.

**Trampas del arnés.** newTestDaemon prepara el socket pero no ejecuta Run:
los clientes IPC reales requieren arrancar ese bucle; omitirlo causó un timeout,
no un fallo de la función. Stop responde antes de asentarse el espejo de mpv:
el caso de última entrada debe esperar Idle antes de afirmar que undo arrancó
audio. Ambos arneses se corrigieron; no fueron defectos de producción. Los
sockets bloqueados por el sandbox se probaron con permiso fuera de él.

**Riesgo y reversión.** La acción es global y CLI/consola `undo` actúa sobre la
última eliminación vigente de cualquier cliente; TUI exige la observada.
Caducar es deliberadamente conservador y no pretende reconstruir cambios
intermedios. El orden con EOF se decide bajo el mutex del demonio, conservando
sus guardas de generación y ruta. No recupera archivos borrados del disco.
Para volver a UX-08, restaurar las fuentes afectadas de `7d68623` o de la copia
con `cp` y retirar los cinco archivos Go nuevos de UX-09, preservando cambios
posteriores; no hay datos persistentes nuevos
que migrar ni revertir. No se ejecutó esa reversión de entrega. Sin commit,
tag, push, PKGBUILD, sustitución de `~/.local/bin/maly` ni reinicio real.
El ciclo UX-00–UX-09 queda implementado, con publicación/despliegue pendientes.

## v1.21.10 (2026-10-19) — UX-08: reproducir después

**Estado:** implementada y verificada; commit autorizado; tag pendiente.
[Contrato y aceptación](../planning/roadmap-ui-ux.md#ux-08--reproducir-después)
y [evidencia reproducible](../planning/evidence/ux-08-1.21.10/README.md).

**Base y decisión.** HEAD `0121e52` (UX-06) con UX-07 completa sin commit,
const 1.20.10; último tag local v1.19.5. Copia exacta de ese árbol, incluidos
archivos nuevos, en `/tmp/maly-ux08-backup`. Su binario rechaza `playnext` como
subcomando desconocido. El dueño autorizó UX-08 y eligió explícitamente FIFO:
B y después C deben sonar B → C. No se consultó remoto.

**Resultado.** `playnext` es una operación atómica del demonio, compartida por
CLI/consola y la acción TUI `play_next` (Ctrl+N configurable). Crea entradas
prioritarias persistentes, conservando pista actual, posición y pausa; no
arranca una cola vacía/detenida. El contrato completo de modos, duplicados,
mutaciones y EOF vive en el roadmap UI/UX; implementación en las fichas de
[cola/player](../architecture/playback.md), [demonio](../architecture/daemon.md)
y [TUI](../architecture/tui.md). La primera pendiente tiene un marcador textual
compartido, independiente del color y de la selección.

**Versión 1.20.10 → 1.21.10, cambio grande.** Cambia el orden efectivo y la
promesa gapless, añade prioridad persistente y campos de Status. Se conserva
el último componente, según la regla de releases. No cambia SQLite, no añade
red propia ni dependencias. Sesión v1 compatible con un campo opcional.

**Verificación.** Build sin CGo, vet, suite completa sin CGo y race pasaron:
19 paquetes, 842 casos/subcasos en cada suite, sin fallos ni saltos; mpv real,
`ao=null`, bus privado. La matriz de cola cruza shuffle on/off y repeat
off/all/one; comprueba consumo por entrada y continuidad del ciclo. Los tests
de demonio verifican pausa/posición, lotes, duplicados y sesión con archivos
eliminados. La transición natural A → B → C usa WAV cortos y comprueba la ruta
efectiva de mpv y LoadCount sin cargas adicionales, con shuffle on/off y
repeat-one. Otro test fuerza la ventana resolveEnd → d.mu de una promesa
obsoleta y verifica reparación por ruta y rechazo de generación vieja.
Dos clientes IPC compiten con el callback EOF bajo el mismo mutex y sus
elecciones no se pierden ni duplican; esa carrera está controlada por el test,
no depende de acertar unos milisegundos de audio real.

Once neutralizaciones compilaron, fallaron por la aserción prevista y pasaron
tras restaurar cada archivo con `cp`: prioridad, consumo FIFO, move, remove,
replace, sesión, promesa anterior, dispatch del demonio, gramática CLI, ruta
TUI y marcador. La aserción del marcador construye su expectativa independiente
del helper compartido; utilizar el mismo helper habría ocultado la regresión.

Matriz tmux en español/default/color, inglés/vim/NO_COLOR y español/Ctrl+Y/
NO_COLOR; dos TUIs, demonio embebido y separado privado. Redimensionado
140×42 → 100×28 → 80×24 → 40×12 → 140×42 con actual pausada intacta.
CLI, consola, Ctrl+O, cola filtrada y `maly select` generan seis elecciones
comprobadas por rutas exactas contra mpv. Reinicio completo conserva FIFO,
pausa y modos. Tests TUI cubren además biblioteca y archivos descargados con
metadatos iguales. A 40 columnas se priorizan ayuda/salida, y el marcador
sigue legible; las acciones adicionales están en la ayuda.

**Trampas del arnés.** Tras redimensionar, un buffer viejo aún podía contener
el filtro buscado: esperar solo una subcadena daba un falso fallo posterior.
El arnés espera ahora el panel y ancho nuevos, contador filtrado y tecla
resuelta. No hubo un arreglo de producción por ese síntoma. Las pruebas de
cola filtrada fijan el foco real y entran por handleKey; un filtro con foco en
biblioteca no acredita selección filtrada de cola. Los sockets bloqueados por
el sandbox se probaron con permiso fuera de él.

**Riesgo, reversión y límites.** Una prioridad aceptada ante una promesa ya
encadenada puede requerir recargar en el límite; no se promete continuidad
audible en esa carrera. Se conservan las dos guardas contra callbacks viejos.
Restaurar fuentes desde la copia pre-UX08 con `cp`, preservando cualquier cambio
posterior, permite volver a UX-07. Tras separar los commits, `5806445` conserva
esa entrega sin depender de /tmp. El binario anterior conserva las entradas de la sesión pero
ignora sus prioridades. No se ejecutó una reversión de entrega. Los buffers
no acreditan rasterizado Kitty, contraste del escritorio ni audio audible.
No se hicieron descargas remotas ni se modificó la biblioteca/servicio real.
El dueño autorizó después commits separados: UX-07 en `5806445` y UX-08
en el siguiente commit. Sin tag, push, PKGBUILD ni copia a `~/.local/bin/maly`.
UX-09 no iniciada.

## v1.20.10 (2026-10-19) — UX-07: continuidad después de descargar

**Estado:** implementada y verificada; commit autorizado; tag pendiente.
[Contrato y aceptación](../planning/roadmap-ui-ux.md#ux-07--continuidad-después-de-descargar)
y [evidencia reproducible](../planning/evidence/ux-07-1.20.10/README.md).

**Base y reproducción.** `main` limpio en `0121e52` (UX-06, 1.20.9), último tag
local v1.19.5; sin consultar remoto. Copias exactas en `/tmp/maly-ux07-backup`
y binario previo antes de editar. Con yt-dlp simulado, la CLI indexaba una
nueva descarga, pero no un archivo existente fuera de la DB ni el audio
conservado con exit 1. Vacío y fallo total ya eran errores. En la TUI el éxito
se anunciaba antes del scan. No se hizo una descarga remota real.

**Antes → después.** `getter.Collect` comparte la clasificación entre CLI y
TUI. Comprueba archivos reales y rutas reportadas; el diff queda como respaldo.
Nuevos, existentes y parciales pasan por scan del destino original y verificación
de presencia en SQLite. La CLI ofrece comandos por ruta con comillas de shell.
La TUI reutiliza el selector para filtrar/ver archivos: Tab añade y Enter
reproduce la ruta elegida. Un scan fallido deja archivos conservados, acciones
deshabilitadas y Ctrl+R para reintentar. Esc cierra, y la generación evita que
respuestas tardías invadan otra búsqueda o descarga. `get playlist` ofrece las
rutas de la playlist recién creada conservando su tratamiento previo de parcial.
Errores al descargar quedan en consola; un parcial mantiene además su resumen.

**Versión 1.20.9 → 1.20.10, cambio pequeño.** Composición de operaciones y del
selector existentes, sin protocolo, esquema, trabajos persistentes ni cambios
al orden de reproducción. Se estudió un panel de detalle remoto para títulos
largos y se mantuvo fuera de este alcance: el selector adaptativo conserva sus
etiquetas planas. En los archivos locales se antepone el nombre de archivo para
distinguir tomas con metadatos iguales. No se certifica identidad con ✓.

**Verificación.** Build sin CGo, vet, suite completa sin CGo y race: 19 paquetes,
810 casos/subcasos en cada suite, sin fallos ni saltos, mpv real con `ao=null`
y bus privado. Once neutralizaciones compilables fallaron por la causa prevista
y pasaron tras restaurar con `cp`: parcial, límite de rutas, presencia en DB,
ruta relativa, acción por ruta, fallo de scan, generaciones de scan y playlist,
aviso obsoleto, escaneo del archivo existente en CLI e invalidación de la
playlist pendiente al iniciar otra descarga. Los logs conservan las
aserciones concretas; no se contaron errores de compilación como regresiones.

Matriz tmux: es/en, default/vim/override, color/NO_COLOR, 140×42, 100×28, 80×24,
40×12 y regreso al grande; demonio embebido y servicio privado. Dos archivos
con metadatos iguales permiten comprobar en IPC que se añade/reproduce el
segundo, no el que coincida por título. Se prueba cola intacta al terminar,
existente sin indexar, parcial, vacío, fallo total, bloqueo de SQLite, reintento
y playlist parcial. Cancelar búsqueda/descarga y cerrar durante búsqueda deja
muerto el PID simulado antes de cerrar el pane; su shell permanece vivo para
no ocultar huérfanos con la limpieza del arnés.

**Hallazgos y ajustes.** La inspección de buffers detectó avisos viejos de
biblioteca vacía/reproducción bajo los archivos; la transición ahora los retira.
El aviso de archivo existente pone el estado antes del nombre para que un
nombre largo no lo oculte a 40 columnas. El directorio relativo necesitaba normalización antes de contrastar rutas
absolutas. El bloqueo de SQLite mostró que scan puede devolver OK con archivos
omitidos: comprobar cada ruta en DB es necesario. El arnés usa Bash explícito
para el PID del shell y elimina el archivo de PID anterior antes de esperar una
búsqueda lenta; esperar solo que existiera podía verificar el proceso equivocado.
La revisión final también protegió el inicio de otra descarga mientras termina
el postproceso de una playlist. El test anterior de archivo existente buscaba
una frase inglesa literal; se actualizó para comprobar el mensaje localizado
tras anteponer el estado al nombre. Los sockets bloqueados por el sandbox se volvieron a probar fuera de él.

**Riesgo, reversión y límites.** Sin migración, las pistas y playlists siguen
siendo compatibles con la base. Restaurar solo las fuentes afectadas desde la
copia exacta o una extracción de `0121e52`, con `cp`, preservando cambios
posteriores; no se ejecutó una reversión de entrega. La biblioteca real no se
tocó. El respaldo por diff no atribuye escrituras simultáneas de otros procesos,
y el reporte de una ruta por línea no admite nombres con saltos de línea.
Cerrar el selector no cancela un scan ya enviado. No hay cobertura del servicio
remoto, autenticación ni subprocessos reales de yt-dlp/ffmpeg; los buffers no
acreditan rasterizado Kitty, escritorio o audio audible. Sin push, tag, PKGBUILD,
copia a `~/.local/bin/maly` ni reinicio del servicio real. UX-08 no iniciada.

## v1.20.9 (2026-10-18) — UX-06: guardar la cola

**Estado:** implementada y verificada; commit autorizado por el dueño al cerrar
UX-06. Tag, publicación y despliegue pendientes.
[Contrato y aceptación](../planning/roadmap-ui-ux.md#ux-06--guardar-la-cola-como-playlist)
y [evidencia](../planning/evidence/ux-06-1.20.9/README.md).

**Base y alcance.** `main` limpio en `4b6d9ef`, const 1.20.8 y último tag local
v1.19.5. El dueño autorizó continuar con UX-06. Antes de editar se guardaron
copias y un binario de la base: `playlist save` fallaba como desconocido mientras
`create/list` funcionaban. Se fijaron nombre, atajo, semántica de instantánea y
rechazo de rutas sin indexar antes de implementar.

**Resultado.** CLI y consola usan `internal/playlist.SaveQueue`: una consulta
`queue`, la transacción de `CreatePlaylistFromPaths` y el aviso `refresh` tras
confirmar. La transacción crea nombre y entradas, resuelve rutas contra la DB
actual y conserva orden/repeticiones. No usa IDs viejos de la cola. Ctrl+S
(configurable) prepara la paleta; Enter con el nombre ejecuta. El filtro y
shuffle no intervienen. Las ayudas salen de `commandmeta`, `HelpRows` e i18n.
El remedio de escaneo tiene su propia línea para no perderse tras una ruta larga.

**Versión 1.20.8 → 1.20.9, cambio pequeño.** Compone dos operaciones existentes
sin cambiar protocolo IPC, esquema, reproducción, layout ni formato persistente.
El paquete pequeño de composición mantiene IPC fuera de library y SQLite fuera
del protocolo; CLI y consola comparten comportamiento sin acoplarse entre sí.

**Verificación.** Build/vet, suite sin CGo y race completos: 19 paquetes,
789 casos/subcasos por suite, sin saltos ni fallos; mpv real con `ao=null` y bus
privado. Seis neutralizaciones compilables fallaron en la aserción prevista:
rollback, orden, instantánea, atajo y dispatch de consola/CLI. Restauradas con
`cp`, todas pasaron. Una prueba corre 25 guardados mientras otra goroutine mueve
la cola del demonio real; cada playlist coincide con uno de los estados completos.

La matriz tmux cubre es/en, cuatro tamaños y regreso al grande, default/vim/override,
color/NO_COLOR, embebido/servicio privado y dos TUIs. Consulta SQLite y el IPC real:
`B,A,B` se conserva aun con filtro sin resultados y shuffle; el guardado no cambia
estado en pausa y durante reproducción mantiene pista y avance natural. Se
comprueban errores y refresco al segundo cliente, además de buffers texto/ANSI.

**Ajustes del arnés.** El test de sugerencias de UX-05 contaba diez Tabs de forma
fija; ahora busca la sugerencia concreta con un límite. La primera prueba de
concurrencia omitía `Run` del daemon, y falló por timeout; se corrigió el arranque.
En vivo, `play <ruta>` reemplazaba legítimamente la cola de prueba: se cambió a
`jump 2`. Se espera la duración cargada por mpv antes de comparar estado, para
no confundir ese refresco asincrónico con una mutación del guardado. La
contraprueba de CLI debía esperar el error en inglés, idioma de ese proceso.
No fueron fallos del producto ni se contaron como aceptación.

**Riesgo, vuelta atrás y límites.** Las transacciones y las pruebas cubren mezcla
de estados y escrituras parciales. No hay migración; las playlists creadas son
compatibles con la base. Se puede restaurar únicamente las fuentes afectadas
desde las copias previas o `4b6d9ef`, con `cp`, conservando trabajo posterior.
No se ejecutó un rollback de entrega ni se tocó la biblioteca real. Como en otras
mutaciones, el aviso `refresh` es best-effort: perder la conexión después del
commit no deshace el guardado. Los buffers no acreditan rasterizado Kitty,
contraste del escritorio ni audio audible. Sin push, tag, PKGBUILD, sustitución
del binario instalado ni reinicio del servicio real. UX-07 no iniciada.

## v1.20.8 (2026-10-18) — UX-05: paleta con sugerencias

**Estado:** implementada y verificada; commit `4b6d9ef` autorizado después
del cierre. Sin tag, push, PKGBUILD ni despliegue local.
[Aceptación](../planning/roadmap-ui-ux.md#ux-05--paleta-con-sugerencias-seleccionables)
y [evidencia reproducible](../planning/evidence/ux-05-1.20.8/README.md).

**Base y reproducción.** `main` limpio en `c8c5cf0` (cierre documental posterior
a UX-04 `fb0db1b`), const 1.19.8, último tag local v1.19.5. No se consultó el
remoto. En tmux 80×24/es, `pl` + Tab no ofrecía ni rellenaba sugerencias; `help`
e historial sí funcionaban. Copias exactas antes de editar y XDG de prueba.

**Antes → después.** La consola permite descubrir acciones por nombre o
descripción localizada, incluidas operaciones de playlists y `get`. Comparte
metadatos mínimos con ayuda y completado de shell en `internal/commandmeta`;
se retiraron las listas manuales de `ConsoleCommands` y ayuda de consola,
derivándolas del catálogo. El dispatch conserva sus responsabilidades en cada
cliente. Los tests comprueban sintaxis esperada y ejecución real además de paridad.

Tab/Shift+Tab seleccionan, Enter rellena sin ejecutar ni insertar argumentos
ficticios; el siguiente Enter usa las validaciones existentes. ↑/↓ mantienen
historial/borrador; Escape retira selección, cancela confirmación o cierra.
Cerrar la paleta y Enter vacío descartan confirmaciones. A 40×12 se muestra
una sugerencia; a tamaños mayores hasta tres, con descripción, marcador textual
y presupuesto de salida/ayudas. Los argumentos se escriben manualmente. No se
extrajo el dispatch, añadió red propia ni adelantó UX-06. `computeLayout` sigue
puro y no cambia; tampoco VS15, metadatos/carátulas ni filtros de UX-04.

**Hallazgo al verificar, no al comparar listas.** `get pick` existía en la CLI
pero la consola lo enviaba como consulta de descarga «pick …». Se corrigió la
paridad reutilizando el buscador de Ctrl+G. El test ejecuta yt-dlp simulado,
observa `--dump-json`/`ytsearch10:Otra dimensión`, recibe el resultado y comprueba
que no se preparó una descarga. No acredita búsqueda ni descarga real.

**Versión 1.19.8 → 1.20.8, cambio grande.** Coordina un recorrido nuevo de
selección, edición, historial y confirmaciones. Se incrementa el central y se
conserva el último componente según releases; la corrección de paridad queda
dentro del alcance de los comandos sugeridos, sin iniciar UX-07.

**Verificación.** Tres variantes compilables fallan por el defecto real:
console.go de la base, metadatos sin argumentos en ambos consumidores y búsqueda
solo por nombres. Restauración siempre con `cp`, pruebas verdes. Las pruebas
capturan IPC y verifican playlists en SQLite. Build sin CGo, vet, suite sin CGo
y race pasan: 18 paquetes y 777 casos/subcasos por suite, cero fallos y cero
saltos; mpv disponible, audio nulo y bus privado. gofmt y diff limpios.

La matriz real cubre es/en, cuatro tamaños con regreso al grande,
default/vim/override Ctrl+Y, color/NO_COLOR, demonio embebido y servicio privado.
Comprueba rutas/orden de cola, reproducción desde otra pista, pausa, historial,
borrador, scroll, confirmaciones y refresco de playlist hacia un segundo cliente.
Se conservan buffers texto/ANSI, sin presentarlos como capturas de escritorio.
El primer arnés esperaba «conservada» donde se imprime «se mantiene»; también se
corrigió una espera que confundía el eco histórico con la entrada actual.

**Riesgo y vuelta atrás.** El riesgo principal era que selección o confirmación
obsoleta enviaran otra acción; lo cubren peticiones y cambios de DB observados.
No hay migración ni persistencia nueva. Las copias originales y el contenido de
`c8c5cf0` permiten restaurar solo los archivos afectados con `cp`, preservando
trabajo posterior; no se toca la biblioteca del usuario. No se ejecutó rollback.

**Límites.** Sin revalidación del rasterizado/arte Kitty, contraste del escritorio,
audio audible ni descarga real. El binario de prueba no sustituye el instalado.
Commit/tag esperan autorización; publicación, empaquetado y despliegue siguen
pendientes. UX-06 no iniciada.

## Sobre 1.19.8 (2026-10-18) — cierre de sesión y pausa

El dueño autoriza commitear UX-04 y **pausa el trabajo por hoy**. Queda en
`fb0db1b` (`fix(tui): unificar consultas locales en la v1.19.8`). Este cierre
posterior solo actualiza documentación y continuidad: **sin bump**, porque
no cambia el binario ni añade una entrega. No se inició UX-05.

La sesión avanzó desde UX-01 hasta UX-04, siguiendo la aceptación del roadmap:

| Entrega | Resultado | Commit / versión local |
| --- | --- | --- |
| UX-01 | Distingue carga, errores, colección vacía y filtros sin coincidencias; guía correcta para crear playlists | `b8ebe5a` / 1.19.5, tag anotado `v1.19.5` |
| UX-02 | Ayudas contextuales con teclas resueltas, mensajes envueltos y ayuda por tareas | `9135bcd` / 1.19.6, sin tag |
| UX-03 | Título de hasta dos filas, selección visible sin color y contraste medido; conserva configs existentes | `2363318` / 1.19.7, sin tag |
| UX-04 | Lenguaje local común para `/`, Ctrl+O y `select`, campos reales, negación y selección por ruta | `fb0db1b` / 1.19.8, sin tag |

Los cierres de cada versión más abajo conservan reproducción, decisiones,
regresiones en ambas direcciones, matriz y límites. La verificación final de
UX-04 pasó build sin CGo, vet, suite completa y race: 17 paquetes, 749
casos/subcasos, sin fallos ni saltos; mpv disponible y bus privado. El cierre
documental comprueba enlaces, hashes y `git diff --check`; no repite suites
correctas ni atribuye la evidencia gráfica de UX-03 a una prueba nueva.

**Publicación y despliegue:** último tag local `v1.19.5`; no existen todavía
`v1.19.6`, `v1.19.7` ni `v1.19.8`. No se hizo push ni se consultó el remoto
para afirmar la publicación de estas entregas. El PKGBUILD sigue aplazado
por instrucción del dueño hasta terminar de probar las subidas. No se sustituyó
`~/.local/bin/maly` ni se reinició el servicio real: el despliegue local sigue
pendiente. No se debe interpretar un commit o un binario de prueba como release
publicada o instalación actualizada.

**Reanudación:** UX-05, paleta con sugerencias seleccionables. Su alcance y
aceptación siguen teniendo una sola fuente en el
[roadmap UI/UX](../planning/roadmap-ui-ux.md#ux-05--paleta-con-sugerencias-seleccionables).
Destino previsto desde esta base: **1.20.8**, cambio grande que conserva el
último componente. El [prompt de continuidad](../planning/PROMPT-CONTINUIDAD-UI-UX.md)
recoge el encargo para otra sesión, sin autorizar avanzar durante esta pausa.

## v1.19.8 (2026-10-18) — UX-04: consultas locales coherentes

**Estado:** implementada y verificada en el commit `fb0db1b`; pendiente de tag,
publicación y despliegue local. UX-03 quedó en `2363318` por autorización del dueño, sin tag
ni push. PKGBUILD aplazado. Siguiente entrega: UX-05.
[Aceptación](../planning/roadmap-ui-ux.md#ux-04--consultas-locales-coherentes) y
[evidencia reproducible](../planning/evidence/ux-04-1.19.8/README.md).

**Base y reproducción.** `main` limpio después de commitear UX-03, const
`1.19.7`, último tag local `v1.19.5`; no se consultó el remoto. Se reprodujo
E09 con datos nuevos: cuatro pistas de Áurea encontradas por `/`, cero por
Ctrl+O y `maly select` con `a:Áurea`. Consulta simple `cafe` y colección completa
como controles positivos. No fue necesario reabrir la auditoría.

**Antes → después.** `songItems` entrega campos plegados separados de la etiqueta
visible. El picker cachea esos campos al reemplazar los ítems y llama a
`parseFilter`/`filterRank`, el mismo motor de biblioteca y cola: AND, prefijos,
negación por subcadena, fuzzy desde tres runas dentro de un campo y ranking
estable. No se interpretan prefijos sobre artista/título/álbum concatenados.
Los ejemplos compactos caben en la entrada a 40×12 y HelpRows explica los campos
en es/en; ambos README describen el cambio de resultados y orden.

Se comprobaron todos los consumidores del picker: playlists conservan fuzzy por
etiqueta; Ctrl+G y `get pick` conservan `noFilter` y su consulta remota. La búsqueda
SQL no cambia. Enter reproduce y Tab agrega sin cerrar. `setItemsKeeping` conserva
la identidad por ruta al refrescar; `select` conserva su foto inicial de biblioteca.
Se evita recalcular al mover el cursor de escritura sin cambiar el texto; render
sigue limitado a dibujar filas. No se añadieron comandos, persistencia, red ni
contratos IPC; `computeLayout` permanece puro e intacto.

**Versión `1.19.7` → `1.19.8`, cambio pequeño.** Amplía dos selectores existentes
con el lenguaje local ya disponible, sin un nuevo recorrido ni modelo de reproducción.
Se incrementa el último componente según releases; no corresponde subir el central.

**Verificación.** Regresiones de respuestas esperadas y paridad en biblioteca,
cola y ambos selectores, con acentos, NFD, Unicode, campos vacíos, filename,
negación y exacta antes de difusa. Tras insertar arriba y renombrar la elegida,
las acciones enviaron su ruta original por IPC. Producción anterior copiada con
`cp`: compila y falla por consultas incorrectas; restauración desde copia, verde.
Build sin CGo, vet, suite completa sin CGo y race: 17 paquetes, 749 casos/subcasos,
cero fallos y cero saltos; mpv presente, bus privado. Los logs y hashes están en
la evidencia. Se preservaron las regresiones UX-00–UX-03.

La matriz tmux cubre es/en, 140×42, 100×28, 80×24, 40×12 y vuelta al tamaño grande;
default/vim/override Ctrl+Y, color y NO_COLOR. XDG nuevo, audio nulo, updates y
viz desactivados. TUI embebida y `select` conectado a su demonio. Tab comprueba
la cola completa con una ruta más; Enter cambia desde otra pista y verifica la
ruta final. Los fallos iniciales del arnés fueron expectativas sobre el título
del selector, contadores envueltos y el nombre CLI `play` (no el IPC `playnow`),
no defectos del producto. Capturas de buffer, no aceptación gráfica Kitty nueva.

**Coste medido.** 40.000 pistas, tres corridas por consulta: `horizonte` pasa de
25,44 a 1,853 ms y ~15,72 a ~0,98 MB asignados; render con las mismas 14 filas,
26,95 → 27,93 µs. Mover izquierda sin editar pasa de 24,79 ms a 0,582 µs.
Prefijos/negación: 0,68–1,45 ms después (antes no tenían la semántica correcta).
No se mide aquí el coste de abrir/refrescar, que incluye plegar los campos.

**Vuelta atrás y límites.** Revertir esta entrega o recuperar picker/songs de
las copias previas; no hay migración de datos. No se modificaron binario instalado,
servicio de música, configuración real ni PKGBUILD. El commit de UX-04 fue autorizado y creado; queda pendiente el tag.
Publicar y comprobar el tag antes de empaquetar. Despliegue local
pendiente de copiar el binario y reiniciar el servicio por decisión del dueño.

## v1.19.7 (2026-10-18) — UX-03: títulos, contraste y selección

**Estado:** implementada y verificada en el commit `2363318`; pendiente de tag, publicación y
despliegue local. UX-02 se cerró previamente en el commit `9135bcd`, autorizado
por el dueño; no se creó `v1.19.6` ni se hizo push. El PKGBUILD sigue aplazado.
Siguiente entrega: UX-04. [Aceptación](../planning/roadmap-ui-ux.md#ux-03--títulos-contraste-y-selección)
y [evidencia](../planning/evidence/ux-03-1.19.7/README.md).

**Base y reproducción.** `main` limpio tras el commit `9135bcd`, const
`1.19.6`, último tag local `v1.19.5`; remoto sin consultar. Se reconstruyó esa
base y se reprodujo con cinco FLAC sintéticos: título largo, NFD y CJK, títulos
repetidos y metadatos ausentes; la primera pista tenía carátula y `.lrc`.
En la columna, el título seguía en una sola fila aunque sobraba alto. Sin
color, el cursor de biblioteca/cola/picker no tenía señal propia. El control
positivo conservaba pistas y posición actual, y el título corto se leía entero.
Además, a 40×12 una ficha con artista, álbum y género consumía la fila de
letras y la ayuda de la capa. Se reprodujo ese caso antes de corregir el
presupuesto; no se atribuye a la nueva segunda línea.

**Antes → después.** `npMetaTextRows` envuelve el título en hasta dos filas.
La columna reserva siete filas para ficha/márgenes, frente a seis; el arte
cede una fila cuando el alto limita, conservando su proporción. La capa pasa
un presupuesto explícito a `npMetaBlock`: margen y sombra ceden primero,
segunda fila de título después; tiempos, progreso, metadatos y una fila de
letra más la salida permanecen. Si la carátula es más baja que la ficha,
el bloque toma el alto de la ficha y ya no descarta sus últimas filas.
`computeLayout` sigue puro y sin cambios; las listas no pierden filas.

Biblioteca y cola reservan tres celdas de prefijo: `>` cursor enfocado y `▶`
pista actual, juntos cuando coinciden. Biblioteca compara rutas; cola,
índices reales incluso al filtrar. El coste horizontal es tres celdas en
biblioteca y una adicional en cola. Los pickers reutilizan sus dos celdas de
margen para `> `. Las flechas de expansión siguen aparte; los offsets del
resaltado se ajustan al prefijo. No se alteran selección lógica ni acciones.
La leyenda vive en `HelpRows`, compartida con la CLI y traducida en es/en.

**Contraste y temas.** Los defaults oscuros cambian solo `dim` de `#6b7a7e`
a `#849296` y `playing` de `#b85c50` a `#cc8072`. Frente a `#171e22`, pasan de
3,78 a 5,25:1 y de 3,76 a 5,54:1. Texto, acento y error ya superaban 4,5:1.
El logo, bordes y gradientes conservan su paleta. La misma paleta oscura **no**
cumple el objetivo sobre `#f5f5f5`; no se inventó una detección de fondo ni un
sistema nuevo de temas. Ambos README ofrecen una combinación clara explícita
(texto `#253035`, dim `#506369`, acento `#286565`, pista/error `#9b4337`), con
ratios de 12,40 / 5,78 / 6,14 / 5,92:1 sobre ese fondo. La tabla completa mide
ambas paletas contra ambos fondos para que el límite sea visible.

La prueba carga un config nuevo y contrasta template/defaults; después carga
uno previo con los colores antiguos y un texto personalizado, comprobando
bytes idénticos y valores conservados. Actualizar no reescribe el archivo:
para adoptar los colores nuevos hace falta editarlos explícitamente. La
medición es luminancia sRGB sobre pares sólidos, no certificación del escritorio
ni promesa para transparencia, colores personalizados o contexto lejano de letras.

**Versión: `1.19.6` → `1.19.7`, cambio pequeño.** Presentación de información
existente dentro de las mismas pantallas y acciones; sin estado persistente,
IPC ni modelo de reproducción nuevos. Se incrementa solo el último componente
según [releases](../development/releases.md). El template cambia defaults,
no introduce migración ni claves de configuración nuevas.

**Verificación.** Los cuatro grupos `TestUX03*` prueban título largo/corto,
posiciones de artista/álbum/detalle, letra y salida, con/sin arte, selección
coincidente y separada, texto sin ANSI, contraste y config previo/nuevo.
Cubren es/en, 140×42, 100×28, 80×24, 40×12 y dos alturas/anchos intermedios.
Las aserciones de contenido evitan que `panel()` esconda un desborde de alto.
Se copiaron los archivos de producción anteriores con `cp`: compiló y los
cuatro grupos fallaron por títulos truncados, letras/salida perdidas,
marcadores ausentes y contraste insuficiente. Restaurados con `cp`, pasaron.

Build sin CGo, vet, suite completa sin CGo y `-race` correctos: **17 paquetes,
cero fallos y cero saltos** en ambas suites; mpv disponible y bus privado.
Los paquetes afectados también pasaron antes del recorrido real. Gofmt y
`git diff --check` correctos. No se repitió la suite por cambios documentales.

TUI real: es/en × oscuro/claro/NO_COLOR × cuatro tamaños, regresando a
140×42: **24 combinaciones, 30 puntos**. Filtro, biblioteca, cola con cursor
y pista iguales/distintos, canciones y ahora suena. XDG propio, audio nulo,
updates/viz apagados; acciones sobre música sintética. Kitty 0.49.2 real,
sin tmux: carátula gráfica, regreso entre columna/capa y los cuatro tamaños;
referencia clara a 140×42. Se verificaron píxeles de la imagen sintética en
140/100/80; a 40×12 el arte cede como estaba previsto. La revisión gráfica
conserva los efectos del compositor: no sustituye la medición de colores.

**Trampas del arnés.** Las primeras capturas de cola y modales podían recoger
el frame previo por esperar texto común a ambos estados: se cambiaron a
condiciones de posición/modal y se recapturó la matriz. En Kitty hubo que
usar la API Lua local de Hyprland y su resize; la petición de resize de Kitty
no bastaba. Enviar `/Una` como una sola entrada no activa el filtro: se
separó `/` del texto. La carga gráfica puede llegar después del texto: se
esperaron píxeles, no solo placeholders. Una pasada adicional se detuvo al
perder el foco, como exigía la guarda que impide actuar sobre otra ventana.
Los avisos de portales/systemd en el bus privado no son defectos de Maly.

**Riesgo, vuelta atrás y límites.** Riesgo principal: empujar metadatos o
confundir cursor con reproducción; se midieron ambos. No hay migración ni
escaneos automáticos. Se guardó copia previa para la regresión; tras un cierre,
la vuelta atrás es revertir la entrega en un commit nuevo y reconstruir la
versión anterior. No se reemplazó `~/.local/bin/maly`, no se reinició el servicio
real y no se tocó el PKGBUILD. Los PNG de escritorio no se incorporan porque
traslucen ventanas ajenas a la prueba; quedan buffers Kitty y observaciones.
No se repite servicio/dos clientes ni todos los remapeos de UX-02: aquí no
cambian contratos compartidos ni dispatch de teclas. No se probó red real.

## v1.19.6 (2026-10-18) — UX-02: ayudas visibles y contextuales

**Estado:** implementada y verificada; commit `9135bcd` creado el 2026-10-18
para continuar con UX-03. Sin tag ni publicación de esta entrega. El cierre
anterior con tag fue
`v1.19.5`. PKGBUILD aplazado por el dueño hasta probar las próximas subidas;
instalación y servicio reales sin modificar. Siguiente entrega: UX-03.
Alcance y aceptación: [roadmap UI/UX](../planning/roadmap-ui-ux.md#ux-02--ayudas-visibles-y-contextuales).

**Base y reproducción.** Árbol limpio en `main`, commit
`b8ebe5a0cae4c72b0a019e869ccbfef0abdeeeba`, const `1.19.5` y tag anotado
`v1.19.5`. No se verificó el remoto. Con XDG aislado, audio nulo y D-Bus privado,
la TUI anterior anunció `a agrega` teniendo `add=f7`; a 80×24 desaparecía el
filtro del pie. El selector de canciones vacío cortaba el remedio y Esc a
40×12. Los controles positivos fueron el mismo selector a tamaño grande y,
después, una biblioteca poblada con navegación y acciones efectivas. Se
conservan [buffers, arnés y resultados](../planning/evidence/ux-02-1.19.6/README.md).

**Antes → después.** El pie fijo se sustituye por acciones completas ordenadas
por prioridad, foco y ancho, con las teclas resueltas. Descubre el panel oculto
en columna única y distingue expandir, plegar y reproducir. Espacio tiene un
nombre visible; una acción que no cabe no anuncia una tecla cortada. Los
mensajes vacíos y errores se envuelven dentro del espacio existente. La
biblioteca ofrece `maly scan` en una línea propia; los selectores muestran Esc
primero y reparten sus instrucciones en hasta tres filas. La ayuda conserva
`HelpRows` compartido con CLI, ahora con encabezados por tareas, descripciones
envueltas y scroll. Ahora suena usa las teclas reales de pausa y ayuda. El
primer flash vacío también resuelve la tecla de consola; los flashes de
modales se acotan al ancho de pantalla.

La prioridad flash → scan → pie normal **ya funcionaba** y se conserva; su
regresión es un control positivo que pasa también con producción anterior.
No se cambian interacción, comandos, filtros, reproducción, IPC ni datos.
No se adelanta el contraste/marcador de selección de UX-03. No se reserva otra
fila permanente: `computeLayout`, `libraryPageH`, `queuePageH` y `maxRows` de
resultados siguen iguales. El picker puede crecer dos filas de instrucciones
dentro del margen que ya reservaban sus llamadores; a 40×12 caben también
flash y tres resultados mínimos. La consola cabe con su mínimo de cuatro
filas de historial y progreso de scan. La ayuda necesita más desplazamiento,
a cambio de conservar las descripciones completas.

**Versión: `1.19.5` → `1.19.6`, cambio pequeño.** Mejora la presentación y el
descubrimiento de acciones existentes dentro del modelo de interacción actual,
sin contratos nuevos de reproducción, IPC o persistencia. Conforme a
[releases](../development/releases.md), aumenta solo el último componente.
El tamaño del diff y las nuevas claves i18n no justifican un salto central.

**Verificación.** Cuatro grupos `TestUX02*` verifican contenido presente y
incorrecto ausente, posición final del pie, estados vacíos/poblados, controles
reales de expansión, retorno tras flash/scan, ayuda larga y límites del frame.
Cubren es/en, los cuatro tamaños y los límites 89/90 y 119/120 del pie.
Para la dirección negativa se copiaron con `cp` los archivos de producción
originales, manteniendo únicamente las claves nuevas adicionales para que los
tests comparasen textos reales: compiló y fallaron tres grupos por atajos
fijos, contenido perdido y falta de grupos. Se restauró producción con `cp` y
todos pasaron. También se corrigieron fixtures antiguos sin ancho/teclas,
que no representaban un modelo inicializado; no se debilitó la expectativa.

Build sin CGo, vet, suite completa sin CGo y suite completa con `-race`:
**17 paquetes correctos, cero fallos y cero saltos** en ambas suites. mpv
estuvo disponible; ejecución bajo bus privado y mediante el mecanismo de
permisos para sockets/cache. `gofmt` y `git diff --check` correctos.

La TUI real recorrió es/en × default/vim/overrides × normal/NO_COLOR ×
140×42, 100×28, 80×24 y 40×12, con regreso a 140×42: **48 combinaciones y
60 puntos de tamaño**. Se ejercitaron filtro, cola, ayuda hasta el final,
canciones, playlists, consola y ahora suena; Space remapeado agregó la pista.
`maly -h` se leyó con las mismas configuraciones. Música, carátula y `.lrc`
sintéticos locales; updates y visualizador desactivados. Los errores de
lectura y las transiciones breves de scan/flash se cubren determinísticamente
en tests, sin depender de capturar un instante real de escaneo.

**Riesgo, vuelta atrás y límites.** El riesgo era perder instrucciones o filas
al envolver; se comprobaron contenido y posiciones, además de dimensiones.
No hay migración de datos. Antes del commit, se puede restaurar la copia de
producción previa; después, revertir la entrega en un commit nuevo y reconstruir
el binario anterior. Se preservan siempre cambios ajenos. No se desplegó nada.
La aceptación visual aquí es el buffer real de tmux; no demuestra contraste
sobre fondos del escritorio, fuentes ni el protocolo de carátulas Kitty. No
se repitió la matriz de dos clientes/servicio: esta entrega no altera estado
compartido. No se probó red ni descargas reales; los consumidores remotos del
picker conservan sus fases y quedan cubiertos por la suite existente.

## v1.19.5 (2026-10-17) — UX-01: estados vacíos veraces

**Estado:** implementada y verificada; cierre local autorizado por el dueño
con commit y tag anotado `v1.19.5`. Publicación y despliegue local pendientes.
El PKGBUILD se aplaza hasta terminar de probar las próximas subidas, por
instrucción del dueño.
Siguiente entrega: UX-02. Alcance y aceptación viven en el
[roadmap UI/UX](../planning/roadmap-ui-ux.md#ux-01--estados-vacíos-veraces).

**Base y reproducción.** Se comprobó `main` en `2031b730b2c9575fe678a60fc76dbcdeb9f03ab1`,
const `1.19.4` y tag anotado `v1.19.4`. Había cambios documentales previos en
el índice y el roadmap, y un prompt de continuidad sin seguimiento; se
conservaron. No se consultaron remotos ni se reabrió la auditoría. La TUI real,
bajo tmux con XDG y D-Bus privados, reprodujo E01–E03 con una canción como
control positivo: biblioteca/cola pobladas decían vacío al filtrar; el selector
sin playlists pedía escanear. Se conservan [buffers antes/después y resultados](../planning/evidence/ux-01-1.19.5/README.md).

**Antes → después.** Los paneles comparaban las filas filtradas con cero;
ahora distinguen la colección original y muestran «sin coincidencias» más
«esc: limpiar filtro». El selector de playlists tiene estado de carga, error
persistente y vacío propio («Escribe un nombre + ctrl+n»). Con playlists
existentes y cero coincidencias permite editar el filtro; canciones y `select`
usan el mismo consejo. Son dos filas breves, sin redistribuir el layout ni
crear un sistema general de hints: eso sigue siendo UX-02.

La protección inicial y el error persistente de **biblioteca ya existían**:
se conservaron, se hizo explícita la carga y se transmitió la misma distinción
al selector de canciones. La cola espera el primer Status y muestra el aviso
de desconexión si lo hay. El flash de primer arranque sigue reservado a una
lectura exitosa sin canciones; no se sustituyó por un error.

**Hallazgo dentro del alcance.** `loadLibrary` descartaba los errores parciales
de playlists y refrescaba el picker como si hubiera leído una lista vacía.
Se reprodujo con SQLite real desechable: renombrar `playlists.name` rompe esa
consulta sin impedir `All()`. `libraryMsg.listsErr` conserva la diferencia,
sin descartar canciones ni modificar el protocolo IPC. Solo una recarga válida
realimenta el picker y limpia su error. Si había ítems anteriores se conservan;
el cuerpo de estado se muestra cuando no hay filas visibles. El buscador remoto
mantiene sus fases y `emptyText` mediante `noFilter`.

**Versión `1.19.4` → `1.19.5`: pequeño.** Corrige la presentación de estados
existentes y añade únicamente estado transitorio de lectura en la TUI; no hay
comandos, interacción nueva, persistencia, IPC ni contratos de reproducción.
Por la regla de releases sube solo el último componente. El cierre local incluye commit y tag anotado;
no acredita publicación.

**Verificación.** `TestUX01FilteredPanels`, `TestUX01LocalPickers`,
`TestUX01LoadingErrorsAndRecovery` y `TestUX01PlaylistReadFailure` comprueban el
mensaje correcto y la ausencia del falso, con controles poblados, recuperación
de filtro y de DB. Con los cinco archivos de producción TUI originales los
cuatro grupos fallaron **compilando y por el estado mostrado**; restaurados con
`cp` pasaron. Se conservaron las traducciones nuevas durante la reversión para
exigir el texto real. La prueba de fallo parcial falló además antes de añadir
`listsErr`. El arnés inicialmente intentó Ctrl+K para borrar; el picker lo usa
para navegar, por lo que se corrigió a Ctrl+E/Ctrl+W.

Pasaron gofmt, build sin CGo, vet, suite completa sin CGo y `-race ./...`,
con bus privado: los 17 paquetes, mpv real disponible, **cero tests saltados**.
La ampliación final del test de carga/error a los cuatro tamaños volvió a pasar
la regresión dirigida y `-race` dirigido; no cambió la producción validada. Las restricciones del
sandbox sobre sockets y caché de Go se resolvieron mediante permisos, no como
supuestos bugs del reproductor.

**Aceptación.** es/en × default/vim/override (F6 para filtro, Ctrl+Y para
playlists) × color/NO_COLOR. Recorrido 140×42 → 100×28 → 80×24 → 40×12 →
140×42: 48 combinaciones únicas y 60 puntos, conservando foco y recuperación.
Se probaron los paneles, canciones, creación de playlist, filtro de playlists,
refresco desde CLI en dos TUIs (embebida y adjunta) y apertura de Ctrl+G sin red.
Carga/errores y `select` se cubren además por mensajes deterministas del modelo;
los consumidores remotos por sus tests existentes. Los buffers confirman texto
útil, incluido Ctrl+N a 40×12; no acreditan fuentes ni contraste del escritorio.
Los recortes de ayudas generales quedan para UX-02. Sin prueba de audio audible,
carátula Kitty ni descarga remota, fuera del cambio de estados.

**Datos y vuelta atrás.** Copias exactas previas en `/tmp/maly-ux01/before` y
copias del arreglo para restaurarlo con `cp`. No hay migración ni datos de usuario
a revertir. Las pruebas usan DB/config/sockets propios, `ao=null`, visualizador
y updates apagados; cierran por Ctrl+C, sin matar procesos por patrones. Para
deshacer esta entrega antes del commit, reponer solo sus archivos desde las
copias, preservando las ediciones documentales que ya estaban al empezar.
El binario validado es temporal: `~/.local/bin/maly` y la sesión musical real
siguen sin actualizarse. El dueño autorizó commit/tag; el push lo hace él.
El PKGBUILD queda aplazado hasta que termine de probar las próximas subidas.

## v1.19.4 (2026-10-17) — bordes y presentación Unicode

**Estado:** implementada, verificada y compilada para la entrega local
`v1.19.4`. La publicación remota queda a cargo del dueño. Durante el arreglo
no se sustituyó el binario instalado.

**Problema reproducido.** La captura del dueño mostraba una fila desplazada
desde `Kareto☕︎`, incluidos los bordes de cola y carátula. La lectura de la
biblioteca confirmó `U+2615 U+FE0E` (café + selector de presentación texto),
sin cambiar sus datos. Las dependencias actuales de la TUI cuentan ese
nombre como ocho celdas; `Screen.draw` de Kitty 0.49.2 ocupa siete. En una
fila de 270 columnas con una aparición, la TUI medía 270 y Kitty dibujaba
269. Quitar solamente VS15 devolvió las 270. El arte no era la causa.

**Cambio:** las tres entradas de render de Bubble Tea normalizan su salida
final. El contrato y el costo tipográfico están en la
[ficha de TUI](../architecture/tui.md): no se altera la biblioteca, los
valores de selección ni las consultas. Abarca los overlays de la TUI y los
selectores independientes `select` y `get pick`.

**Por qué esta solución:** al quitar VS15 solo al final se conserva el
ancho ya reservado sin alterar índices de resaltado. No se agrega un espacio
al final de la fila (dejaría desplazados los bordes interiores), no se cambia
el layout ni el render de carátulas (medían lo esperado), y no se modifica
`safetext` ni se reescriben etiquetas. Cambiar únicamente `padTo` para contar
VS15 de otra forma dejaría otras mediciones divergentes dentro de Lip Gloss,
recortes y selectores. Un protocolo de ancho exclusivo de Kitty o reemplazar
las bibliotecas de medición tendría un alcance mayor que este arreglo.
Se acepta perder la preferencia de presentación texto en pantalla.

**Justificación de versión:** `1.19.3` → `1.19.4`, cambio pequeño: corrige la
presentación de un símbolo sin cambiar interacción, IPC, persistencia ni
reproducción. El hotfix pasa a ser UX-00; los números de UX-01–UX-09 se
recalculan desde esta base en el [plan](../planning/roadmap-ui-ux.md).

**Verificación:**

- Regresión portable sobre la vista completa a 40, 100, 140 y 270 columnas:
  ancho total y posición del borde interior, uno/dos VS15 y controles sin
  selector/con VS16. El ancho de estas fixtures se fija con una fuente
  independiente del cálculo defectuoso; también se cubren ambos selectores.
- Se conserva byte por byte el arte Kitty generado, los escapes ANSI, VS16,
  ZWJ, acentos combinantes y una variante CJK; los datos originales no mutan.
- Al neutralizar la normalización, las tres pruebas nuevas fallaron
  **compilando**, por ancho o salida incorrectos; al restaurarla con `cp`
  pasaron. El primer intento del arnés desechable tenía un campo mal nombrado
  y no compilaba: se corrigió antes de aceptar la comprobación roja.
- Contraste con `Screen.draw` de Kitty 0.49.2, sin ventana: frames exportados
  a 40×12, 100×30, 140×42 y 270×58, con/sin renderer gráfico (visible donde
  cabe la tercera columna). Se quitaron los escapes ANSI para medir el
  avance del cursor, conservando los placeholders Unicode. Antes: filas
  cortas por una o dos celdas; después: **cero desajustes** en los ocho
  frames. No se presenta esto como captura de la sesión gráfica del dueño.
- Suite completa con `CGO_ENABLED=0`, suite completa con `-race`, ambas bajo
  `dbus-run-session` con mpv real; `go vet ./...`; compilación sin CGo a
  `./maly`, que informa `v1.19.4`. Los bloqueos iniciales del sandbox eran
  de sockets/caché; las comprobaciones se ejecutaron fuera de él.

## Sobre v1.19.3, sin bump (2026-10-17) — revisión y plan UI/UX

Codex revisó el código y controló una TUI compilada desde `16bed5c`, con 14
pistas sintéticas en un sandbox XDG, audio nulo y tamaños entre 140×42 y
40×12. Se registraron estados vacíos incorrectos, instrucciones recortadas y
diferencias de consulta entre `/` y Ctrl+O. El inventario completo, las
capturas y los límites viven en la
[evidencia](../planning/evidence/ui-ux-2026-10-17/README.md).

El [roadmap](../planning/roadmap-ui-ux.md) separa arreglos acotados de nuevas
capacidades; la [guía de implementación](../development/implementacion-ui-ux.md)
define cómo reproducir, verificar y cerrar cada entrega. El dueño fijó la
regla de numeración por alcance, documentada en
[releases.md](../development/releases.md): cada entrega justifica su número.

**Justificación de versión:** se conserva `1.19.3` porque este trabajo solo
documenta evidencia y preparación. Ninguna propuesta está implementada; no
cambia el comportamiento del binario ni se crea tag o entrada pública de
release. La validación de este cambio es documental; compilar y recorrer la
TUI durante la revisión no equivale a probar futuras implementaciones.
Se comprobaron los enlaces locales de los nueve Markdown afectados, los hashes
de los veinte archivos de evidencia y las nueve transiciones previstas de
versión. `git diff --check` y el chequeo de whitespace de los Markdown nuevos
no encontraron problemas. La const de versión y `CHANGELOG.md` no cambiaron;
`AGENTS.md` y `CLAUDE.md` conservan el mismo contrato operativo.

## v1.19.3 (2026-10-15) — consultas en lote

`play`, `add` y `playlist add` pasan de unir los argumentos a resolverlos por
separado. La regla, el orden, los duplicados, los mensajes y la compatibilidad
de una sola consulta viven en `docs/architecture/cli.md`; la decisión de no
adivinar entre frase y lista, en `docs/decisions/no-hacer.md`. Las comillas de
ctrl+p y sus diferencias con la shell se describen en la ficha de la TUI.
El número 1.19.3 continúa la serie actual por elección del dueño.

**Lo que encontró la revisión previa a publicar.** `resolveAll` confundía
cualquier error de `resolveTracks` con una consulta sin resultados. Con una
base cerrada y una ruta válida, el lote devolvía éxito parcial y ocultaba
`sql: database is closed`; con dos búsquedas, reemplazaba ese error por «sin
resultados». `noTracksError` permite saltar solo las búsquedas sin
coincidencias y los directorios sin audio, conservando los errores reales y
cancelando antes de tocar la cola. `SearchEach`, usado por las playlists, ya
conservaba los errores de SQLite.

La ayuda de ctrl+p también conservaba la sintaxis de un solo argumento; ahora
anuncia los lotes. Los README en ambos idiomas y el changelog explican la
ruptura visible: una frase de varias palabras requiere comillas. El bump
permite distinguir el cliente nuevo de un demonio anterior, que no entiende
`Queries`; actualizar exige reiniciar el servicio.

### Verificación

`TestResolveAllPreservaErrorBiblioteca` reproduce el error con SQLite real
cerrado, una ruta válida y ambos órdenes del lote, además del caso en que
fallan todas las búsquedas. Se vio fallar compilando antes de la corrección
y al restaurar temporalmente el código anterior; el arreglo se recuperó con
`cp` de una copia.
`TestResolveAllSaltaSoloSinResultados` comprueba el éxito parcial legítimo,
los directorios vacíos, el fallo sin ninguna pista y los mensajes individuales
en inglés y español.

Pasan la suite completa con `CGO_ENABLED=0` y `go test -race -count=1 ./...`,
ambas con mpv real y un bus D-Bus privado (`dbus-run-session`); también
`go vet ./...` y `git diff --check`. El binario local se compiló sin CGo y
`./maly help` confirma la versión 1.19.3 y la sintaxis de consultas en lote.

## v1.19.2 (2026-10-14) — la CLI legible en fondo claro

Un solo arreglo, señalado por el dueño con una captura de `maly help` en un
terminal claro: el verde de los comandos (`#a6e3a1`) y el gris de las notas
(`#6c7086`) casi no se distinguían del fondo. La CLI fijaba colores de
Catppuccin Mocha, pensados solo para fondo oscuro, y los repetía en seis
sitios (help, info, config, doctor, controls y logo).

La paleta pasa a `cmd/maly/style.go` como `lipgloss.AdaptiveColor`: Mocha
para fondo oscuro y Latte para claro. lipgloss resuelve el lado con una
consulta OSC 11, y solo si stdout es un terminal. Bajo tmux termenv ni
siquiera pregunta y asume oscuro, que es el comportamiento anterior, así que
nadie pierde nada.

La TUI no cambia: sus colores salen del tema del config (`[theme]`), que el
usuario ya controla. Esto solo afectaba a la salida de la CLI.

Sin test dedicado: es un cambio de presentación, como los de solo texto. Se
comprobó forzando `SetHasDarkBackground` en las dos direcciones (dan
`38;2;166;227;161` y `38;2;64;160;43` respectivamente) y en vivo, en el
terminal del dueño.

## v1.19.1 (2026-10-05) — el filtro `/` flota, y el arreglo de MPRIS llega a todos

Dos piezas. La primera es el **filtro `/` rehecho**: era una fila dentro del
panel que empujaba la lista hacia abajo. Ahora es una caja flotante al pie del
panel activo (estilo fzf, decisión del dueño frente a «arriba, estilo yazi» y
«centrada»). Trae navegación sin salir del input, contador, resaltado y un
lenguaje de consulta: prefijos `a:`/`al:`/`t:`/`f:`, `-negación`, nombre de
archivo y coincidencia difusa con orden por puntuación. El diseño y sus
porqués están en la ficha (`docs/architecture/tui.md`, párrafo del filtro).
Aquí va solo lo que costó o se midió.

La segunda es `a24a2fc`, la posición MPRIS que se arrastraba de la pista
anterior. Era posterior al tag v1.19.0 y ya se distribuía por el AUR como
parche del PKGBUILD (1.19.0-2). Con este tag el parche sale del PKGBUILD,
como anunciaba su propio comentario.

**El número.** Es una feature y por semver le tocaría 1.20.0. Sale como
1.19.1 por decisión del dueño: el número lo elige él, y ya hay precedentes de
numeración elegida a propósito en `docs/development/releases.md`.

### Lo que costó

- **`x/ansi.Truncate` no sirve para componer capas.** Guarda TODOS los escapes
  de la línea, también los de después del corte. Con la carátula de kitty en
  la misma fila, la transmisión APC habría salido dos veces por frame. Además,
  un carácter ancho partido lo descarta (`Truncate`) o lo cuela entero
  (`TruncateLeft`), y la fila deja de medir el ancho del terminal. Por eso el
  corte de `compose.go` es propio, sobre `ansi.DecodeSequence`. Lo fijan
  `TestSpliceLineAnchoExacto` y `TestSpliceLineNoDuplicaEscapes`; con las dos
  funciones de x/ansi en su lugar fallan los dos, con «mide 31, quería 30» y
  «aparece 2 veces».
- **El primer test del resaltado no probaba nada.** Al revertir el plegado
  runa a runa por un `Fold` de la cadena entera, el test seguía pasando:
  «Café» en NFC no cambia de longitud al plegarse. Hizo falta un caso NFD
  (`Cafe\u0301`, lo habitual en archivos venidos de macOS), y con él falla por
  la razón correcta: las marcas corridas una runa.
- **El tope de la tercera columna no tiene mutación que lo dispare** con las
  constantes de hoy. La caja mide `max(panel, 44)`, y en el modo de tres
  columnas eso nunca pasa de `libW+queueW`. El barrido de
  `TestFilterBoxRect` lo comprobó subiendo `filterBoxMinW` a 200: con el tope,
  0 errores; sin él, «la caja llega a 120, el tope es 92». Queda como red para
  el día en que alguien ensanche la caja.
- **Dos defectos que solo aparecieron en vivo** (tmux a 160×42, 100×30 y
  60×20):
  - Un `…` fijo en el borde derecho del input. `textinput.View()` mide
    prompt + Width + 1, y al ras `clip()` lo daba por desbordado.
  - El contador decía `5/6` con el filtro vacío, porque contaba nodos de
    artista y no pistas.

### Verificación

- Hay test nuevo para cada pieza con estado: parseo, coincidencia, orden,
  resaltado, composición, `filterBoxRect` barrido con la tabla de layout,
  navegación mientras se escribe y el borrado correcto con la cola reordenada.
- Cada uno se vio fallar revirtiendo producción (restaurado con `cp`, nunca
  con `git checkout`).
- Pasan `go vet ./...` y `go test ./...` completos.

## v1.19.0 (2026-09-06) — Phase 3 de la auditoría del 2026-09: siete hechos, cinco cerrados

La **1.19.0** cierra la **Phase 3**, que era la fase **explícitamente no
obligatoria**: doce oportunidades de las que el propio informe dice que
«ninguna resuelve un problema que alguien esté sufriendo hoy». Con esto la
auditoría del 2026-09 queda **entera**: 27 de 27 hallazgos y 8 de 8
oportunidades, o cerrados con código o cerrados como decisión.

**El trabajo de esta tanda fue TRIAR, no implementar.** Se hicieron **siete**
y **cinco se cerraron como decisiones tomadas** en
`docs/decisions/no-hacer.md`, con su razón. Implementar los doce porque
estaban en una lista habría sido el mal resultado.

### Lo que de verdad deja esta tanda

**La lección se repitió por tercera fase seguida, y esta vez el riesgo era
mayor**: estas fichas son las menos reproducidas del informe (casi todas LOW y
OPPORTUNITY, la parte que más se razonó y menos se midió) y describen la
1.16.3, tres releases atrás. De los doce ítems, **cinco tenían la premisa o la
justificación movida**, y en dos casos eso decidió el triaje:

1. **O-07 daba el estado por MUDO y no lo es.** «El demonio queda vivo y mudo
   sin reproductor» — pero cuando mpv muere, `command` corta al instante por
   `p.done` y **cada comando contesta «mpv terminó»**. Lo que estaba mudo era
   otra cosa: `status`, que responde con normalidad porque el espejo del
   player conserva la última foto, y `doctor`, que preguntaba por el socket
   del DEMONIO. El arreglo se escribió sobre eso y no sobre la ficha.
2. **A-09 hablaba de seis sitios y quedaban dos.** A-10 ya había cambiado la
   forma de `View()`. Y al mirar los dos que quedan, el remedio resultó PEOR
   que el problema: `helpScroll` se clampa contra el contenido que arma
   `helpView` y `npLyricsLines` se llama con DOS alturas distintas, así que
   moverlos a `Update` significa duplicar cálculos que pueden divergir para
   arreglar cero fallos observables. Cerrado.
3. **A-18 decía dos sitios y es uno.** El otro `go cmd.Wait()` viene después
   de un `Kill()` y ahí nadie lee el pipe: es un reaper correcto.
4. **O-08 se quedó sin justificación.** «Desactiva A-06 de raíz» dejó de ser
   cierto cuando la Phase 1 cerró A-06. Rejustificado, lo que quedaba era
   ahorro de bytes en una petición puntual. Cerrado.
5. **A-08 pedía medir antes que código, y la medición no lo refutó: le retiró
   el riesgo.** El temor de la ficha era romper un `SetNext` legítimo en una
   máquina lenta. Con mpv REAL, 200 `SetNext` alternando ruta dan **29,7 µs de
   media y 79 µs el peor caso** (el no-op por espejo, 3 ns), así que el tope
   nuevo de 1 s es ~12.000 veces el peor caso sano. La medición dijo que el
   arreglo era barato, no que sobrara.

Y una de proceso: **la red de O-03 no cubría dos de los ítems** (A-26 y A-23
son de paridad CLI↔consola y compara solo `get playlist`), así que en los dos
casos la paridad se resolvió moviendo la DECISIÓN a un solo sitio —el deadline
al protocolo, el mensaje a `ipc/display.go`— en vez de arreglar dos espejos a
mano. Es la lección de A-04 aplicada antes de que divergieran.

### Los siete

**1. A-22 — el sesgo de `loadGen`, documentado.** `Load`/`LoadPaused`/`Stop`
suben la generación ANTES del `loadfile`, así que un comando fallido la deja
incrementada sin que mpv haya recargado nada. Se documenta y no se cambia,
que es lo que la ficha prefería: el error va en la dirección segura (se pierde
un avance automático en vez de avanzar sobre una premisa falsa, el bug de la
1.16.3) y moverlo abriría la ventana contraria.

**2. A-11 — los colores del tema, por tabla.** `hexFields()` es ahora la
fuente única y `Load` la recorre. Pero **la tabla sola no cierra la clase**:
una fila también se olvida, que es literalmente CFG-1. Quien la cierra es un
test por REFLEXIÓN que recorre `Theme` y `Visualizer` y exige que todo campo
string cuyo default sea `#rrggbb` esté en la tabla —así `banner = "splash"` y
`backend = "auto"` quedan fuera solos— con los derivados exceptuados POR
NOMBRE. Lleva control positivo por conteo, porque una reflexión que dejara de
ver los campos pasaría vacua. `TestLoadInvalidColorsClamp` se conserva
enumerado a mano a propósito: el test nuevo arma su TOML DESDE la tabla, así
que no puede cazar una fila que falte, y el viejo sí.

**3. A-18 — `cmd.Wait()` después del `readLoop`.** El efecto en maly es
benigno, como decía la ficha, pero **no es teórico**: reproducido fuera del
árbol con la precondición puesta (el escritor sale con datos en el buffer del
pipe y el lector no es instantáneo) da **20 de 20 corridas con pérdida**, y en
las 20 el lector no llega a ver ni un byte de los 64 KiB que quedaban. En maly
eso es el último parpadeo del visualizador de un backend que ya murió. El test
cubre el RIESGO del cambio —el reapeo pasa a depender de que el readLoop
termine— y se registra que no cubre el orden: eso lo sostiene la medición. El
reapeo se le pregunta al sistema con señal 0 y no a `cmd.ProcessState`, que lo
escribe `Wait` desde otra goroutine (`-race` lo caza, y con razón).

**4. A-26 — el scan sin timeout artificial.** Eran 10 min fijados A MANO en
tres sitios de dos espejos. La decisión se mueve al PROTOCOLO (`reqTimeout` en
`internal/ipc`): lo que el llamador fije manda, y si no fija nada son 30 s
salvo `scan`, que no lleva deadline. Comprobado que el lado servidor aguanta:
`serve` pone su read-deadline ANTES de leer, y el escaneo ocurre después.
El test del deadline mira el valor REAL que `Do` le pone a la conexión, con
una conexión que los anota, porque un test cronometrado no puede distinguir
«sin deadline» de «con el default de 30 s» sin durar 30 s — sería la aserción
laxa que pasa con el defecto puesto.

**5. A-08 — la ventana gapless deja de esperar como un comando normal.** Ver
arriba la medición. Dos topes: `cmdTimeout` (5 s) para un comando cuyo fallo
ve el usuario, `windowTimeout` (1 s) para los de `SetNext`, que son
best-effort por diseño y son los únicos que se pagan en CADA mutador bajo
`d.mu`. `dispatch` no se tocó. Va con la mitad documental que el hallazgo
pedía: la regla no es «estas tres operaciones están fuera del lock» sino
**«ninguna espera a mpv bajo `d.mu` pasa de su tope»**.

**6. A-23 — `playlist add` dice cuántas ya estaban.** `AddToPlaylist` devuelve
`(added, dup)` contando dentro de la transacción que ya se abría, y no bloquea
nada. El mensaje se arma en `ipc.PlaylistAdded` y no en cada llamador: los
TRES caminos que agregan a una playlist tenían el texto por separado. La red
que lo mantiene así es nueva y estática: `callsites_test.go` fija que una
clave con dueño único solo se use desde su archivo.

**7. O-07 — el demonio dice cuándo se quedó sin mpv.** Versión que solo
REPORTA (decisión del dueño): relanzar pide decidir si la música vuelve sola y
en qué posición, y eso es producto, no arreglo. `player.Dead()` mira `p.done`
—que es exactamente lo que hace fallar a `command`, y cubre también el proceso
vivo con el socket roto—, viaja como `Status.PlayerDead` (omitempty, y a
diferencia de `Notice` NO caduca: no hay carga siguiente posible) y lo muestran
el pie de la TUI, el stderr de `maly status` y `maly doctor`, que es `fail`
porque el criterio del comando es salir 1 solo si algo impide reproducir.
Probado en vivo en un sandbox aparte, matando SOLO el pid exacto de su mpv:
`doctor` pasa a fail y sale 1, y las filas siguen alineadas con la etiqueta
más larga en español.

### Los cinco que se cerraron

A-09 (los clamps de `View()`), O-04 (`context` en `Scan`/`doClose`), O-08
(`add` por id), **A-13** (seguir enlaces de directorio) y A-21 (la carátula
fuera de los mutex de MPRIS). El desarrollo de cada uno está en
`docs/decisions/no-hacer.md`; el más importante es A-13, el único ítem de la
fase que puede PERDER DATOS: `underRoot` sostiene hoy la purga **y** el conteo
de la guarda de A-01, y con pistas detrás de un enlace cualquiera de las dos
identidades posibles rompe una de las dos. La mitad barata ya salió en la
1.17.0 y el usuario está avisado con un remedio que funciona.

### Trampa de arnés que costó un ciclo

Una, y de las viejas conocidas: **restaurar una reversión con `git checkout`
se llevó por delante el fix sin commitear** del mismo archivo (el espejo de
A-23 en la consola). Es exactamente la trampa 6 de
`docs/development/verificacion.md`, escrita desde la 1.16.2, y volvió a
morder. El resto de reversiones de la tanda se hicieron con `cp` de una copia.

### Verificación

Cada arreglo con lógica o estado nuevo lleva test verificado en ambas
direcciones, siempre COMPILANDO y fallando por la razón documentada; A-22 no
lleva test por ser documental y el de A-18 cubre el riesgo del cambio y no la
carrera, que se sostiene con la medición. `go build`, `go vet`, `gofmt -l .`,
`go test ./...` y `go test -race ./...` limpios en todo el árbol.

**Un intermitente observado y NO atribuido**, anotado acá para que exista si
vuelve: en una de 15 corridas de `go test -race ./...` sobre el árbol entero,
`TestAdvanceObsoletoTrasJump` falló con «la cola dice a (Index=0) y mpv
reproduce ""». No se reprodujo en 20 corridas aisladas de ese test —ni sobre
esta tanda ni sobre la 1.18.0 sacada con `git archive`— ni en las otras 14
corridas del árbol, así que no hay con qué atribuirlo al cambio de A-08 (el
único que toca tiempos de mpv, y solo los de `SetNext`: `CurrentPath` sigue
con sus 5 s). La forma es la de la intermitencia que el proyecto ya tiene
documentada: las esperas del test miran el ESPEJO del demonio y la aserción
le pregunta a mpv EN VIVO, así que bajo carga —el árbol entero con `-race`
en paralelo— pueden desalinearse. Si reaparece, el arreglo es que `checkSync`
polleé en vez de leer una sola vez, no relajar la aserción.

---

## v1.18.0 (2026-09-06) — Phase 2 de la auditoría del 2026-09

La **1.18.0** cierra la **Phase 2** completa (8 ítems). Con esto la auditoría
queda en **21 de 27 hallazgos y 5 de 8 oportunidades** cerrados; Phase 3 (12
oportunidades, explícitamente no obligatorias) sigue en el informe
(`~/Audits/MalyAu/`).

Se hicieron en el orden que el informe marcaba como no negociable —el
benchmark ANTES de la optimización, la extracción antes de la red de
paridad— y uno por commit, cada uno con sus tests verificados en ambas
direcciones antes de pasar al siguiente.

### Lo que de verdad deja esta tanda

**La lección de la 1.17.0 se repitió, y con la misma proporción.** De los ocho
ítems, **cuatro fichas no sobrevivieron intactas al contacto con el código**.
Las cuatro se descubrieron reproduciendo la premisa en vez de transcribirla, y
las cuatro habrían producido código peor si se hubieran obedecido:

1. **A-05 daba una lista de mutadores mal por los dos lados.** Proponía subir
   la generación de la cola en `Add, Replace, Clear, RemoveAt, Move, JumpTo,
   Next, Prev, SetShuffle`. Cuatro de esos **no tocan `Items`** —mueven
   `Index`/`order`/`pos`, estado que ya viaja aparte en `Status`— así que
   habrían re-mandado la cola entera en CADA canción, o sea casi nada de lo
   que se quería ahorrar. Y faltaban los dos que sí importan: `learnDuration`
   y `refreshQueueDurations` escriben `Items[i].Duration` **en sitio**, sin
   pasar por ningún mutador de `internal/queue`. Sin cubrirlos, la cola del
   cliente se quedaría con las duraciones en cero **para siempre** — y no es
   una hipótesis: `refreshQueueDurations` ya llamaba a `wakeSubs()` justamente
   para que esas duraciones llegaran a la pantalla. De ahí `Queue.Touch()`.
2. **A-10 contaba tres listas de modales; son cuatro.** Además de
   `npColumnVisible`, que la ficha señala, `logoVisible` (`logo.go:179`)
   enumeraba las capas a mano **y se olvidaba de `getOpen` igual que la otra**.
   O sea que con `ctrl+g` abierto la TUI hacía dos trabajos para nada: decodificar
   la carátula de una columna que no se dibujaba, y animar la onda de un
   banner invisible — esto último justo contra la línea de la 1.7.2, que bajó
   el reposo de la TUI de 3,5 % a 0,7 % autocancelando esta clase de relojes.
3. **A-03 describía un problema que la 1.16.4 ya había cerrado.** La ficha
   habla de `music_dir`, pero `d.cfg` se usa hoy en EXACTAMENTE dos sitios, los
   dos en `daemon_scan.go`, y desde que el cliente manda la ruta ya resuelta el
   único efecto observable del config rancio era **`scan_durations`**. El test
   se escribió sobre esa clave; sobre `music_dir` no habría reproducido nada.
4. **A-12 proponía el remedio correcto, y se callaba lo que lo hizo barato.**
   El reparto lee→`Open` / escribe→`Create` es el mismo criterio en los tres
   sitios donde aparece, y en uno de ellos ya estaba escrito sin saberlo: el
   flag `reload` de `conLib` significa literalmente "esto muta playlists".

Y una quinta lección, de proceso y no de código: **la red de paridad de O-03
pagó dentro de la propia fase en la que se escribió.** Cazó que la CLI toleraba
la biblioteca ausente al planificar `get playlist` y la consola no — una
divergencia CLI↔consola que la implementación de A-12 acababa de introducir
horas antes, en este mismo trabajo. Es exactamente la clase de A-04, y esta vez
duró minutos en vez de tres releases.

### Los ocho ítems

**1. O-05.2 — `BenchmarkState`, antes que nada (`internal/daemon`).** Línea
base del push de suscripción por tamaño de cola. Mide `state` + `json.Marshal`
juntos, porque es la unidad que el suscriptor paga cada 250 ms, y reporta
**B/push**, que es la métrica que A-05 baja y la única comparable entre
máquinas. Fuera de CI, como `BenchmarkScan`. Medido en el equipo del dueño:

    cola=100      64,5 µs/push      33,8 kB/push
    cola=1.000     642 µs/push       338 kB/push
    cola=10.000   6,00 ms/push     3,40 MB/push

Perfectamente lineal, que es el problema: con 10.000 pistas y cuatro pushes por
segundo son 13,6 MB/s por cliente sin que cambie nada en pantalla.

**2. A-05 / O-01 — `Status.QueueGen`.** El patrón de `libGen` aplicado a la
cola, con el contador en `internal/queue` (ver arriba dónde sube y dónde no) y
el push armado por suscriptor en `stateFor` —que ya era por suscriptor desde
A-25—. Tras el arreglo, con el mismo benchmark:

    cola=100      1,24 µs/push      201 B/push
    cola=1.000    0,98 µs/push      202 B/push
    cola=10.000   0,74 µs/push      203 B/push

El tamaño del push en régimen **deja de depender del tamaño de la cola**. A/B
en vivo, mismo sandbox, misma cola de 1.000, misma música, suscriptor externo
contando bytes 10 s: **4,28 MB → 0,11 MB**; CPU del demonio con un suscriptor
pasivo, **4 ticks/10 s → 0**.

Compatibilidad comprobada en el código y no supuesta: los dos suscriptores que
existen aguantan un push sin `Queue` —la TUI conserva su copia, y con ella la
caché `queueFolded` que se destruía cuatro veces por segundo— y el polling sin
suscripción usa el comando `queue`, que sigue mandando la cola siempre.

**3. A-04 (extraer) — `internal/getter/playlist.go`.** `PlanPlaylist`,
`PlaylistPlan.Resolve` y `TrackIDs`: las decisiones de `get playlist` en un solo
sitio. La ficha listaba dos funciones; hicieron falta tres, porque `TrackIDs`
(el filtro de G3) también estaba escrito dos veces. `newDirEntry` estaba
duplicada **byte a byte** y se absorbe en `Resolve`. Lo que NO se movió es la
ejecución: la CLI corre yt-dlp con `cmd.Run()` y la consola con
`tea.ExecProcess`, que solo devuelve el error — esa diferencia es real.

La reversión que prueba que sirvió: quitar el filtro de G3 de `getter.TrackIDs`
hace fallar a la vez dos tests de la CLI y uno de la consola. Un cambio, los dos
espejos; antes había que romper dos copias.

**4. O-03 — paridad de COMPORTAMIENTO (`cmd/maly/parity_test.go`).** Siete
escenarios corridos contra los dos planificadores, comparando el plan. Dos cosas
que el informe no anticipaba: cada escenario necesita **dos sandboxes frescos**
(planificar CREA el destino, así que la segunda pasada vería lo que hizo la
primera y `CreatedDir` saldría distinto por culpa del arnés); y **comparar los
dos lados no basta**, porque dos espejos igual de rotos coinciden — de ahí que
cada caso compruebe además lo que se espera de verdad, verificado rompiendo los
dos lados a la vez.

**5. A-12 / O-06 — `Create` crea, `Open` no.** Las cinco variantes de "abrir si
existe" (`library.OpenIfExists`, `openLibraryIfExists`, `openLibIfExists`, y los
dos `os.Stat` propios de `select.go` y `get_pick.go`) se van a cero. `Open`
devuelve `fs.ErrNotExist` envuelto, que es lo que permite a `RunSelect`
distinguir "no hay biblioteca" de "la hay pero no abre" sin envoltorio propio.
Cierra el caso concreto de la ficha: `maly playlist list` sobre una instalación
nueva ya no deja un `library.db` vacío.

La reversión vale por sí sola como recordatorio de por qué se corre la suite
entera: devolverle a `Open` la capacidad de crear hace fallar el test nuevo **y
tres que ya existían** (`TestCompleteNoDB`, `TestCheckLibraryNoDB`,
`TestLibraryStatsNoDB`).

**6. A-10 — `activeOverlay()`.** Un enum y una función en vez de siete
booleanos en cuatro listas, con `showHelp` aparte porque la ayuda se dibuja
ENCIMA de otra capa. El test va **por tabla sobre el enum**, hasta un `ovCount`
de cierre: escribir a mano una quinta lista habría sido repetir el error que el
cambio cierra. Lleva control positivo, porque una implementación que devolviera
`false` siempre pasaría igual.

**7. A-28 — tests de contrato de `cmd/maly`.** Los tres que pedía el informe más
dos que salieron con ellos. Para el parseo hizo falta extraer `buildRequest(cmd,
args)`, como la ficha anticipaba. La gramática de `playlist` (C14) se fija por
EFECTO sobre la base —qué pista se borra— y no por parseo. Un ajuste de arnés
que conviene registrar: la primera versión del test de `doctor` exigía además
que no escribiera el config, y eso estaba MAL — el template se escribe cuando no
existe, y ese es el mecanismo de migración del proyecto.

**8. A-03 (mitad estructural) — el demonio relee el config.** `reloadConfig()`
al empezar cada scan; si el TOML dejó de parsear se conserva el anterior y se
anota el error, porque hacerlo en silencio es lo que el hallazgo reprocha.
**Releer le quitó a `d.cfg` la inmutabilidad de la que dependía leerlo sin
sincronizar** —el scan corre fuera de `d.mu` y su comentario decía "solo toca
lib (thread-safe) y cfg (inmutable)"—, así que `reloadConfig` escribe bajo
`d.mu` y el escaneo trabaja con una copia local; `TestConfigConcurrenteConScan`
existe para que `-race` mire ese tramo, y quitarle el lock produce un `DATA
RACE` de verdad.

Del lado del diagnóstico, una op de solo lectura `config` expone lo que el
servicio tiene en efecto (`Response.Config` es `omitempty`: no cuesta nada en el
push) y `checkDaemonConfig` lo compara con el disco. Relectura fallida = `warn`;
"el config cambió y aún no hubo scan" = `info`, porque no hay nada roto.

### Trampas del arnés que costaron un ciclo

Las tres salieron de esta tanda y están destiladas en
`docs/development/verificacion.md`:

- **Un `add` sobre una cola vacía arranca la reproducción**, y con la pista
  sonando `learnDuration` sube la generación de la cola por su cuenta. El test
  del relleno de duraciones **pasaba sin el arreglo** hasta que se paró el
  player antes de medir. Variante exacta de "una medición negativa sin verificar
  su precondición no es una medición".
- **`TestScanFillsDurations` no podía cazar el defecto** aunque parecía el test
  natural para hacerlo: lee por el comando `queue`, que manda la cola SIEMPRE.
- **`newTestDaemon` apagaba `ScanDurations` en el struct**, y desde que el
  demonio relee el config manda el ARCHIVO: un sandbox sin `config.toml` recibe
  el template, que la trae activa. La guarda dejaba de guardar sin que nada lo
  dijera. Es la misma familia que SYSD-1: el arnés dejó de reproducir la
  condición real cuando la condición real cambió.

Y una de la prueba en vivo: con el config del CLIENTE roto, `maly doctor` cae a
los defaults, así que comparar su `music_dir` con el del servicio decía "el
config cambió" cuando lo que pasa es que no se puede leer. La fila se omite en
ese caso.

### Lo que NO se hizo, a propósito

- **Nada de Phase 3.** Sigue siendo explícitamente no obligatoria.
- **`QueueGen` no se generalizó** a un mecanismo de generaciones para todo
  `Status`: la cola es la única parte no acotada del Status (el resto son ~200
  bytes), así que la abstracción no tendría un segundo usuario.
- **No se añadió opt-in de protocolo** para la cola omitida. Se comprobó en el
  código que los dos suscriptores existentes conservan su copia ante un push
  sin `Queue`, así que un negociado habría sido superficie de protocolo para
  cubrir un caso que no se rompe.
- **La rama de relectura fallida de `reloadConfig` es hoy casi inalcanzable**
  —todos los clientes que piden un scan cargan el config antes y fallan ellos
  primero— y se conserva igual, anotada en el código para que nadie la borre:
  es la única respuesta correcta a un config ilegible.

---

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
