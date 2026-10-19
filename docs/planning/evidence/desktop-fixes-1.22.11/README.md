# Ayuda y detalles de escritorio — 1.22.11

2026-10-19. Encargo explícito posterior a la revisión visual: corregir sus
hallazgos D1/D2/D3 y mejorar la ayuda de la captura del dueño. Base: árbol
1.22.10 con UX-09 sin commit, HEAD `7d68623`. Respaldo anterior a los cambios:
`/tmp/maly-polish-before`. Commit/tag pendientes; no hubo push, empaquetado,
reemplazo del binario instalado ni reinicio del servicio personal.

## Resultado visual

- [Ayuda ancha en español](01-help-wide-es.png).
- [Ayuda a 140×42](02-help-140-es.png): seis tarjetas, cabeceras contrastadas,
  teclas en negrita y dos columnas. En este tamaño cabe todo el contenido.
- [Ayuda inglesa, Vim y Ctrl+Y personalizado](11-help-wide-en-vim.png).
- [Una columna a 100×28](03-help-100-es.png), [40×12](04-help-40-es.png) y
  [final desplazado](10-help-bottom.png): contador de filas y cierre siempre visible.
- [Sin color sobre fondo claro](15-help-no-color-light.png): la organización
  sigue visible gracias a títulos, bordes, espacios y negrita.

Los colores derivan del tema del usuario. No se tocaron sus archivos de
configuración. La captura oscura usa su configuración de Kitty y transparencia;
el perfil claro cambia únicamente la instancia temporal y desaparece al cerrarla.

## Hallazgos cerrados

| Hallazgo de 1.22.10 | Comportamiento comprobado en 1.22.11 |
| --- | --- |
| D1: la paleta corta respuestas | [El error de undo completo a 100×28](06-error-wrapped.png). Se envuelve antes de paginar; el estilo se conserva en cada continuación, y Ctrl+Home/End llegan al principio/final de filas visuales. |
| D2: se pierde el diagnóstico al volver de yt-dlp | [404 real de una pista](07-download-404-retained.png) y [404 real de playlist](13-playlist-error-retained.png). La paleta conserva la salida final de la herramienta, saneada, sin interpretar sus mensajes. |
| D3: el aviso pequeño oculta el mínimo | [39×11 muestra 40×12 primero](05-minimum-es.png). El resto del texto se ajusta al ancho. |

La [descarga correcta](08-download-success.png) sigue llevando al selector de
archivos; no se alteraron reproducción, cola, indexación ni recuperación UX-09.
La CLI mantiene su salida directa: [cli.jsonl](cli.jsonl). El mensaje genérico
compartido ya no remite ambiguamente a una salida «arriba».

## Diseño e implementación

- `HelpRows` sigue siendo la fuente única CLI/TUI. Seis tareas: reproducción,
  biblioteca/navegación, cola/playlists, paleta, descargas, vistas/salida.
  `maly -h` indenta las continuaciones de las descripciones.
- `computeLayout` incluye la geometría pura de ayuda. Dos columnas desde
  116 celdas útiles de modal, ancho máximo 180; debajo usa una columna.
- `consoleOutput` conserva el historial original y vuelve a envolver al cambiar
  el ancho. El scroll representa filas desde el final; se acota al redimensionar.
  Se conserva ANSI propio y se normaliza VS15 únicamente en la copia de pantalla.
- `downloadExec` conserva el proceso interactivo mediante `tea.Exec`. Copia
  stdout/stderr a un buffer de 16 KiB con mutex y retiene hasta 40 líneas finales
  al fallar. No interpreta errores/progreso ni escribe logs persistentes.
  `safetext.Clean` protege la salida incorporada a la paleta; `getter.Collect`
  y el plan de playlist siguen decidiendo el resultado por sus contratos previos.

**Versión 1.22.10 → 1.22.11:** alcance pequeño de presentación y diagnóstico,
sin nuevo estado compartido entre clientes/demonio ni cambio de reproducción.
[Cierre de ingeniería](../../../history/roadmap-1.17.md#v12211-2026-10-19--ayuda-por-tareas-y-detalles-de-escritorio).

## Verificación

[checks.json](checks.json): build sin CGo y vet correctos; suite completa sin
CGo y suite con detector de carreras: **19 paquetes, 916 casos/subcasos cada
una, cero fallos y cero saltos**. mpv real con `ao=null`, rutas XDG y D-Bus privados.
La última ejecución comprueba la geometría integrada en `computeLayout`.

[reverse.json](reverse.json): seis neutralizaciones compilan y fallan por la
razón prevista; todas pasan después de restaurar con `cp`:

1. Salida envuelta y conservación del texto/estilo.
2. Aviso mínimo ajustado al ancho.
3. Ayuda ancha dividida en tarjetas/columnas.
4. Captura del diagnóstico de la herramienta.
5. Presentación del diagnóstico al volver a la TUI (pista y playlist).
6. Límite del buffer ante escrituras largas/concurrentes.

Las nuevas pruebas también cubren Ctrl+Home con una sola entrada que ocupa
muchas filas, redimensionado, inglés/español, Unicode, controles maliciosos y
restauración de estilos después del borde. Se mantuvo la paridad de `maly -h`;
la prueba existente recorre cada línea de las descripciones multilínea. La
prueba previa de ayuda pequeña amplía su recorrido para alcanzar los seis grupos.

```sh
# Desde la raíz del repositorio; requieren sockets, mpv, Go y D-Bus.
dbus-run-session -- python3 docs/planning/evidence/desktop-fixes-1.22.11/checks.py
python3 docs/planning/evidence/desktop-fixes-1.22.11/reverse.py
```

`reverse.py` modifica producción temporalmente: ejecutar sin otros cambios o
checks concurrentes. Restaura siempre sus copias, incluso si falla. El cuerpo
de ayuda anterior usado por la contraprueba se conserva en `help-baseline.txt`.

## Prueba en el escritorio y límites

Quince recortes reales de `grim`, cada uno con buffer de Kitty y geometría JSON.
[visual-final.json](visual-final.json) identifica el binario exacto de la última
pasada: el mismo build sin CGo de los checks finales. Perfiles español/default,
inglés/Vim/Ctrl+Y e inglés/Vim/Ctrl+Y/NO_COLOR. Tamaños 240×52, 140×42, 100×28,
40×12 y 39×11. Las capturas se inspeccionaron visualmente.

Descargas con yt-dlp/ffmpeg instalados y servidor HTTP temporal en
127.0.0.1:18779: error 404, archivo existente/correcto y error de playlist. La
biblioteca, música de prueba, sesión y bus se aislaron; no se modificó la música
personal ni su cola. Se cerraron las instancias y el servidor temporales.
No se acredita escucha audible, descarga de YouTube ni cookies. La revisión
anterior conserva sus pruebas y límites en su propia carpeta.

Los controladores de escritorio se guardan como referencia de esta sesión;
dependen de fixtures/rutas temporales y del compositor del equipo. No son un
instalador ni deben apuntarse al runtime personal. Los checks y contrapruebas
anteriores sí reconstruyen sus entornos de prueba.

`identity.json` registra los binarios; `SOURCE-SHA256SUMS` identifica Go y sus
dependencias finales, `SOURCE-BEFORE-SHA256SUMS` el árbol 1.22.10 anterior.
`SHA256SUMS` identifica las evidencias de esta carpeta.
