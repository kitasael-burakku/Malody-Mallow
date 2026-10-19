# Revisión real de escritorio — Maly 1.22.10

2026-10-19. Revisión exploratoria autorizada por el dueño: interacción real con
Kitty/Hyprland, cambios de workspace, tamaños, capturas del compositor y pruebas
CLI/TUI. **No se modificó producción durante esta revisión.** UX-09 sigue sin
commit; esta carpeta añade el informe y la evidencia de escritorio.

## Resultado

No se observaron fallos funcionales en los recorridos de las funciones nuevas.
Se encontraron **tres detalles reproducibles de presentación/diagnóstico** y
una limitación del tema personalizado. Esto no prueba ausencia de otros fallos:
la cobertura concreta y sus límites se enumeran abajo.

### D1 — La paleta recorta mensajes que podrían leerse completos

Prioridad media. A 100×28, ejecutar `undo` sin recuperación disponible desde
Ctrl+P. La respuesta termina en «caducó o f…», aunque sobra altura. El scroll
vertical mueve entradas, pero no revela la parte cortada de una entrada.

- [Captura a 100×28](27-expired-undo-error.png) y [buffer](27-expired-undo-error.txt).
- [La misma respuesta completa a 140 columnas](36-download-failure.png).
- Fuente: `internal/tui/console.go`, `consoleView`: aplica `clip(l, innerW)`
  a cada línea de salida. Las ayudas del modal sí se envuelven.
- Propuesta: envolver la salida al ancho útil y calcular el desplazamiento
  sobre filas visuales; conservar estilos y posición al redimensionar.
- La recuperación funciona: el defecto afecta a la lectura de la explicación.

### D2 — El error de descarga remite a una salida que la paleta no conserva

Prioridad media. Desde Ctrl+P, descargar una URL local que responda 404.
`yt-dlp` muestra el diagnóstico durante su ejecución; cuando regresa la TUI,
la paleta conserva «yt-dlp falló: exit status 1 (revisa su salida arriba)»,
sin el 404 ni la explicación de la herramienta. No hay esa salida «arriba»
dentro del historial de la paleta.

- [Captura](36-download-failure.png). En [cli.jsonl](cli.jsonl), la misma URL
  mediante CLI conserva `HTTP Error 404: File not found`.
- Propuesta: conservar el log bruto del proceso y ofrecer acceso a él, o
  reformular el mensaje y señalar dónde consultarlo. **Sin parsear la salida
  humana de yt-dlp**, conforme a `docs/decisions/no-hacer.md`.
- Es un problema de diagnóstico; no se añadió nada a la cola al fallar.

### D3 — El aviso de tamaño mínimo pierde justo las dimensiones

Prioridad baja. Redimensionar a 39×11. Aparece «terminal muy pequeña para maly
(necesit…», pero no llega a verse `40×12`.

- [Captura](45-below-minimum.png) y [dimensiones medidas](45-below-minimum.json).
- Fuente: `internal/tui/view.go`, retorno anticipado de `view()`; devuelve
  `tui.too_small` en una sola línea sin ajustarla al ancho disponible.
- Propuesta: poner el mínimo al principio o envolver el mensaje. A 40×12
  la interfaz normal sí funciona, incluyendo deshacer y ayuda desplazable.

No se hizo una comparación ejecutable contra una versión anterior para fechar
estos tres hallazgos. No se atribuyen como regresiones específicas de UX-09.

### Observación del tema del usuario

El perfil real tiene transparencia de Kitty cercana a 0,70 y colores explícitos
para texto y texto secundario. Los textos secundarios se ven apagados sobre
partes del fondo; al imponer un fondo claro, el texto principal claro pierde
mucho contraste. La fila seleccionada conserva legibilidad.

- [Aspecto real oscuro](02-real-color-wide.png).
- [Tema personalizado sobre fondo claro](50-custom-theme-light-attempt.png).
- [NO_COLOR sobre fondo claro](49-no-color-light.png): texto y selección
  identificables sin depender de la paleta de color de Maly.

La prueba clara cambió colores únicamente en la instancia temporal de Kitty,
y los restauró. No se alteró el tema del usuario. Esta observación corresponde
a la combinación de colores explícitos y fondo; no demuestra un defecto nuevo
de los valores predeterminados. No se midió contraste WCAG por píxel.

## Recorridos comprobados

| Área | Comprobación y evidencia |
| --- | --- |
| Identidad | Binario instalado 1.22.10, idéntico byte a byte a `./maly`; 163 archivos de fuentes/dependencias coinciden con el manifiesto de UX-09. [identity.json](identity.json) |
| Escritorio | Kitty 0.49.2, Hyprland 0.56.2, monitor 1920×1080 a escala 1. Ventana movida al workspace 9, flotante y redimensionada; foco original restaurado. |
| Configuración real | Biblioteca real de 133 pistas, cola de 18, inglés/Vim, transparencia, carátula Kitty y vista ampliada. [Principal](02-real-color-wide.png), [ampliada](03-real-now-playing.png) |
| Tamaños | 140×42, 100×28, 80×24, 40×12 y límites 89/90, 119/120; 39×11 para el aviso. Cada captura incluye JSON con píxeles y celdas realmente obtenidos. Las pestañas adicionales ocupan una fila: sus capturas son 140×41. |
| Ayuda | Descripciones envueltas y scroll hasta el último atajo a 40×12. [Inicio](10-help-40x12.png), [final](51-help-bottom-40x12.png). `End` cierra la ayuda y se procesa en la pantalla principal por el contrato vigente; para desplazar se usan las teclas indicadas. |
| Unicode y títulos | Carátula real; títulos largos, 東京, café, acento combinante y ☕ con VS15 en fixtures. No se observaron bordes desplazados en las capturas. Elipsis en las listas y título de dos filas en la ficha. |
| UX-01/02/03 | Cola vacía con biblioteca poblada, filtro sin coincidencias, ayudas contextuales, selección y reproducción diferenciadas. [Vacío](16-empty-queue-es.png), [sin coincidencias](35-no-matches.png) |
| UX-04 | Ctrl+O `a:lorax` sobre biblioteca real; `/t:Elegida` en cola; `maly select` con `a:aurea -t:Elegida`, sin distinguir acento y excluyendo B/C. [Selector CLI](37-cli-select-filter.png) |
| UX-05 | Tab selecciona sugerencia; primer Enter rellena, segundo ejecuta. Respuesta de error visible, historial de operaciones y comando preparado. [Selección](25-palette-undo.png), [relleno](26-palette-filled-no-execution.png) |
| UX-06 | Ctrl+S desde una cola filtrada a dos resultados guarda sus cuatro entradas; CLI verifica el contenido. Repetir el nombre falla sin sobrescribir. [Filtro](21-filtered-queue.png), [guardado](23-save-result.png), [CLI](cli.jsonl) |
| UX-07 | `yt-dlp` real 2026.08.19 y ffmpeg real n9.0.2: descarga HTTP local nueva, existente, conversión FLAC→MP3 e indexación, 404. La cola no cambia al acabar la descarga; Tab añade, Ctrl+N prioriza y Enter reproduce explícitamente. [Resultado](29-downloaded-real-tools.png), [mínimo](33-download-40x12.png) |
| Búsqueda remota | Ctrl+G devolvió diez resultados reales para «Claude Debussy Clair de lune», con título, canal, duración y vistas. No se descargó ningún resultado remoto. [Captura](40-real-search-result.png) |
| UX-08 | B mediante CLI y C mediante Ctrl+N en canciones: orden A→B→C→D y marca «después». Dos `next` comprueban B y C por ruta mediante IPC. También probado Ctrl+N en archivos descargados y en `maly select`. [Cola](18-fifo-cli-tui.png), [CLI](cli.jsonl) |
| UX-09 | Quitar C y `u` la restaura conservando pausa y posición 9 s; indicador disponible a 40×12; error cuando no existe recuperación. Segundo cliente en inglés/Vim/NO_COLOR ve y consume la recuperación compartida. [Mínimo](24-undo-40x12.png), [segundo cliente](47-shared-undo-en.png) |

## Medición y preservación

[metrics.json](metrics.json) registra una muestra de **6,00 s**, con **25,43 MiB
RSS** y **0,83 % de un núcleo** para el PID exacto de Maly: TUI y demonio
embebido, pausa, `ao=null`, visualizador apagado, 140×42. No incluye Kitty,
mpv ni compositor; es una muestra puntual, no un benchmark de carga. Se
seleccionó el proceso por ejecutable y runtime aislado, no el lanzador D-Bus.

[real-session-preserved.json](real-session-preserved.json) compara la sesión
personal antes y después: misma pista, pausa, posición **18,261312 s**, volumen
100, shuffle, repeat y longitud de cola 18. Tras limpiar quedaron solo el
servicio original de Maly, PID 137855; se cerraron las TUIs y el servidor HTTP
temporales. No se reemplazó el binario instalado ni se reinició ese servicio.
No hubo commit, tag, push ni cambio de PKGBUILD durante esta revisión.

## Límites y reproducción

Las 48 imágenes PNG son recortes reales tomados con `grim` de la ventana de
prueba, no renderizados de buffers ANSI. Sus `.txt` son lecturas separadas de
Kitty; los `.json` registran geometría y tamaño. Puede haber diferencias
mínimas entre captura de texto e imagen por animación/tiempo de reproducción.
Se enfocó la ventana antes de capturar para evitar registrar el workspace de
la conversación. Se descartaron capturas de preparación inválidas.

Las mutaciones usaron XDG y D-Bus privados, audio nulo y archivos en
`/tmp/maly-desktop-review/sandbox`. El origen HTTP fue
`127.0.0.1:18769`, con audio sintético; **no un yt-dlp simulado**. Además se
copió una pista real al sandbox para verificar su carátula, sin modificar el
original. La búsqueda de YouTube sí utilizó red externa.

No se acredita escucha audible, descarga desde YouTube, cookies, cancelación
de un árbol remoto de procesos, estrés prolongado, gapless auditivo, ni todos
los fallos de disco/indexación. Los tests anteriores de UX-09 cubren otros
contratos; esta ronda amplía la evidencia visual, no sustituye aquella suite.
No se volvió a ejecutar la suite Go completa porque no se cambió código.

Para reproducir los hallazgos basta el binario 1.22.10 y estos recorridos:

1. Terminal 100×28 → Ctrl+P → `undo` sin recuperación disponible → Enter.
2. Servidor HTTP local con un MP3 → Ctrl+P → `get http://127.0.0.1:PUERTO/ausente.mp3`.
   Comparar el mensaje final con el mismo comando CLI.
3. Ventana flotante → 39 columnas por 11 filas; observar el aviso inicial.

Los controladores de esta sesión quedan en `/tmp/maly-desktop-review` para
inspección local. Dependen de su socket de Kitty, PIDs y geometría específicos;
no son un arnés portable ni deben relanzarse contra la sesión del usuario.
`SHA256SUMS` identifica los artefactos de esta carpeta.
