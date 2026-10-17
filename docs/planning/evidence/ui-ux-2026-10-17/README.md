# Evidencia de la revisión UI/UX — 2026-10-17

Observación de Codex sobre **v1.19.3 / `16bed5c`**. Este archivo conserva lo
observado; las propuestas y su aceptación viven en el
[roadmap](../../roadmap-ui-ux.md). No es evidencia de arreglos implementados.

## Condiciones y límites

- Binario compilado del checkout limpio con `go build`, sin sustituir el
  instalado. Biblioteca sintética de 14 FLAC silenciosos de 180 segundos,
  agrupados en artistas/álbumes, con un título largo, caracteres japoneses,
  títulos similares y una pista sin artista ni álbum. La primera tenía `.lrc`.
- XDG bajo `/tmp/maly-ux-review`, `ao=null`, `update_check=false`, idioma español,
  teclas default y visualizador desactivado. Bus D-Bus deliberadamente inaccesible
  para no registrar el MPRIS de prueba en la sesión real.
- TUI real controlada mediante tmux: biblioteca, reproducción/pausa, filtro,
  canciones, playlists, consola, ayuda, descarga inicial y ahora suena. Tamaños
  140×42, 100×28, 80×24 y 40×12.
- La primera tanda heredaba `NO_COLOR=1`; se repitieron las observaciones
  relevantes con esa variable retirada. Se obtuvieron 37 capturas; aquí se
  conserva una selección suficiente para documentar los hallazgos.
- `.txt`: buffer visible de `tmux capture-pane -p`. `.ansi`: el mismo tipo de
  captura con estilos (`-e`). `.png`: render de ese buffer mediante pyte/Pillow,
  fondo de referencia **#171e22** y fuentes de la máquina de revisión. Los PNG
  **no son capturas del escritorio**: no evalúan transparencia, rasterización
  real del terminal ni protocolo gráfico de Kitty. Los archivos fuente permiten
  distinguir un recorte real del producto de un detalle del render de evidencia.
- Las filas vacías finales de `.txt` y `.ansi` conservan la altura capturada.
  `.gitattributes` desactiva solo el aviso `blank-at-eof` para estos archivos;
  no se recortan las capturas ni se modifican sus hashes por estilo.
- No se probó audio audible, carátula Kitty, descarga remota, MPRIS, biblioteca
  masiva ni toda la matriz en inglés/presets. Abrir Ctrl+G no prueba yt-dlp.
- El cierre fue normal y se comprobó que no quedaban maly/mpv de prueba. El árbol
  de código quedó limpio al acabar la revisión. La suite completa no se ejecutó
  para esta inspección; no había cambios de implementación.

## Observaciones y reproducción

Los IDs E son evidencia, los IDs UX son propuestas. Las rutas temporales
originales no son necesarias: reconstruir el sandbox siguiendo
[probar-en-vivo.md](../../../development/probar-en-vivo.md).

| ID | Observación y reproducción | Evidencia | Entrega |
| --- | --- | --- | --- |
| E01 | Con 14 canciones, `/` → `zzzzsincoincidencias`: contador 0/14 pero «biblioteca vacía» | [Texto](25-color-no-matches.txt), [PNG](25-color-no-matches.png) | UX-01 |
| E02 | Con una pista en cola, Tab → `/` → consulta imposible: contador 0/1 pero «cola vacía» | [Texto](32-color-queue-zero.txt) | UX-01 |
| E03 | Biblioteca poblada sin playlists, Ctrl+L: aparecen «biblioteca vacía… scan» y «no hay playlists» a la vez | [Texto](35-color-empty-playlists.txt), [PNG](35-color-empty-playlists.png) | UX-01 |
| E04 | Primer arranque en 140×42, después del flash inicial: «biblioteca vacía — ejec…» y aviso de servicio integrado recortado | [Texto](01-empty-140x42.txt) | UX-02 |
| E05 | Ventana 80×24: el pie principal pierde atajos; Ctrl+O pierde el final del hint. En 40×12 se recorta aún más | [Principal](17-main-80x24.txt), [selector PNG](31-color-songs-80x24.png), [selector mínimo](23-songs-40x12.txt) | UX-02 |
| E06 | Reproducir el título largo y pausar: la ficha lateral conserva una sola línea pese a disponer de altura | [Texto](36-color-title-and-lyrics.txt), [PNG](36-color-title-and-lyrics.png) | UX-03 |
| E07 | Medición de colores predeterminados contra fondo de referencia, no contra el escritorio | Tabla siguiente y `internal/config/config.go` | UX-03 |
| E08 | Bajo `NO_COLOR=1`, estilos de selección desaparecen; la fila no tiene marcador propio que los sustituya | [Selector sin color](06-songs-filter.txt), `internal/tui/picker.go`, `styles.go` | UX-03 |
| E09 | `a:Áurea` con `/`: 4 resultados; cerrar y escribir lo mismo en Ctrl+O: 0 | [Filtro](33-field-filter.txt), [selector](34-field-song-picker.txt) | UX-04 |
| E10 | Ctrl+P → `pl`: texto de entrada sin sugerencias; el historial sí está implementado | [Texto](29-color-console.txt), `internal/tui/console.go` | UX-05 |
| E11 | Inspección estática de `getDoneMsg`: mensaje y scan tras éxito; `ownedTitles` compara títulos normalizados | `internal/tui/tui.go`, `get.go`; no hubo descarga real | UX-07 |

La ausencia de acciones dedicadas para guardar cola, reproducir después y
deshacer se comprobó en los comandos y handlers de la base. Su utilidad es
una propuesta de producto; no se afirma que sean defectos reproducidos.

## Datos mínimos para repetir E06 y E09

Crear cuatro pistas con artista `Áurea`, álbum `Horizontes` y estos títulos:

1. `Una canción con un título muy largo que necesita respirar — versión acústica`
2. `Luz de octubre`
3. `Luz de octubre (En vivo)`
4. `Volver al principio`

Las otras diez pistas pertenecían a Atlas, Niebla, 北の空 y una sin artista.
No son necesarias para reproducir la diferencia de prefijos; sí proporcionan
controles negativos y permiten observar que la biblioteca tiene contenido.
Con cuatro pistas, el contador esperado es 4/4 en vez de 4/14.

## Contraste calculado

Se utilizó luminancia relativa sRGB y `(Lclara + 0,05)/(Loscura + 0,05)`.
Estas cifras describen combinaciones explícitas, no el fondo real del usuario.

| Texto | Fondo | Relación aproximada |
| --- | --- | --- |
| Dim `#6b7a7e` | `#171e22` | 3,78:1 |
| Text `#d4dadb` | `#171e22` | 11,93:1 |
| Playing `#b85c50` | `#171e22` | 3,76:1 |
| Error `#c96f60` | `#171e22` | 4,76:1 |

La referencia de [W3C sobre contraste mínimo](https://www.w3.org/WAI/WCAG22/Understanding/contrast-minimum.html)
propone 4,5:1 para texto normal; se usa como objetivo de diseño, no como
certificación WCAG de una TUI. El criterio de ayudas visibles y acciones
descubribles se apoya en las
[heurísticas de Nielsen Norman Group](https://www.nngroup.com/articles/ten-usability-heuristics/).
Estas referencias fundamentan propuestas; no sustituyen la observación local.

## Conservación

Se copiaron los archivos seleccionados del entorno de revisión sin modificar
su contenido. [SHA256SUMS](SHA256SUMS) registra sus hashes para poder detectar
ediciones posteriores. Los PNG y sus buffers ANSI son documentación, no
goldens de tests ni una promesa de comparación píxel a píxel futura.
