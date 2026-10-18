# UX-03 — evidencia de 1.19.7, 2026-10-18

Base: commit `9135bcd`, versión `1.19.6`, sin tag de esa versión. UX-03
implementada y verificada; commit autorizado; sin tag de 1.19.7 ni publicación.
[Alcance](../../roadmap-ui-ux.md#ux-03--títulos-contraste-y-selección) y
[cierre de ingeniería](../../../history/roadmap-1.17.md#v1197-2026-10-18--ux-03-títulos-contraste-y-selección).

## Evidencia conservada

- `before-*`: **texto/ANSI real de tmux** sobre la base. Título de una línea,
  selección invisible sin color y ficha completa que a 40×12 oculta letra/salida.
- `es-*` / `en-*`: selección del recorrido final; biblioteca, cola con cursor
  y pista actual coincidentes/separados, y ahora suena en los cuatro tamaños.
  Los archivos sin nombre de paleta se capturaron con `NO_COLOR=1`.
- `matrix-results.json`: 24 combinaciones y 30 puntos de tamaño, todos correctos.
  es/en × oscuro/claro/sin color × 140×42 → 100×28 → 80×24 → 40×12 → 140×42.
  La segunda captura a 140×42 sobrescribe la inicial: acredita el regreso.
- `checks.json`: resumen extraído de logs JSON de Go, build/vet y regresión.
  17 paquetes aprobados en suite sin CGo y race, sin fallos ni saltos; mpv
  disponible. No contiene todos los logs de cada test.
- `contrast.json`: luminancia sRGB y ratios sobre `#171e22` y `#f5f5f5`.
- `kitty/`: **buffers del Kitty real**, no de tmux. ANSI incluye placeholders
  gráficos; un visor de texto no reconstruye la imagen embebida. Los JSON
  conservan solo dimensiones y pantalla alternativa, no variables de entorno.
- `kitty-observations.json`: condiciones y observaciones gráficas. Se vieron
  PNG reales de la región de la ventana, capturados por grim; no se guardan
  aquí porque los efectos del compositor traslucen el escritorio. No son
  imágenes sintetizadas a partir del buffer.

## Qué se comprobó

Regresión compilable en ambas direcciones, con producción anterior y restaurada
mediante `cp`. Fallan cuatro grupos `TestUX03*` antes, pasan después. Se verifica
el contenido correcto y la posición de la salida, no solo el alto que `panel()`
puede rellenar. Controles: título corto, ausencia/presencia de carátula, Unicode,
selección movida a otra pista y config anterior con colores personalizados.

El arnés real usa XDG nuevos, bus privado, audio nulo y updates/viz apagados.
Genera cinco FLAC de silencio: título largo, NFD/CJK, títulos repetidos y datos
vacíos, con carátula y LRC en la primera. Recorre filtro, cola, canciones y
ahora suena, cierra vistas y regresa. Las capturas de cola esperan el cursor
correcto; las primeras esperas laxas se corrigieron antes de guardar esta matriz.

Kitty 0.49.2: carátula gráfica inspeccionada en columna y capa, bordes alineados,
redimensionado y regreso, es, cuatro tamaños oscuros y 140×42 claro. En una
pasada se confirmaron los píxeles del PNG sintético para 140/100/80; a 40×12
el arte se oculta y se conserva letra/salida. La opacidad pedida a la ventana
no elimina todos los efectos del compositor: **el contraste numérico no se
infiere de esos PNG**. Una pasada adicional se detuvo al perder el foco; las
guardas solo permitían modificar la ventana de prueba. No se cambiaron reglas
persistentes de Hyprland ni se controló la sesión musical real.

## Contraste y límites

| Rol | Antes sobre oscuro | Default nuevo sobre oscuro | Paleta clara sobre claro |
| --- | --- | --- | --- |
| Texto | 11,93 | 11,93 (`#d4dadb`) | 12,40 (`#253035`) |
| Secundario | 3,78 | 5,25 (`#849296`) | 5,78 (`#506369`) |
| Pista actual | 3,76 | 5,54 (`#cc8072`) | 5,92 (`#9b4337`) |
| Acento | 7,53 | 7,53 (`#7ab8b8`) | 6,14 (`#286565`) |
| Error | 4,76 | 4,76 (`#c96f60`) | 5,92 (`#9b4337`) |

Fondos: oscuro `#171e22`, claro `#f5f5f5`. La paleta oscura sobre claro falla
el objetivo; se documenta una combinación manual clara, no se añade detección
automática. Ambos README indican cómo adoptar colores sin sobrescribir configs.
La prueba de config verifica bytes intactos al cargar el previo y coincidencia
entre template y defaults al crear uno nuevo. Ratios de referencia, no garantía
para transparencia, temas propios, fuentes ni letras de contexto atenuadas.

No se repitió la matriz de servicio/dos clientes ni todos los presets de UX-02:
no se cambian IPC, persistencia ni dispatch. No hubo red ni descargas reales.
El código comparte el marcador entre consumidores del picker y la suite de
sus consumidores pasó. El binario instalado y el servicio real no se actualizaron.

## Repetir el recorrido

El script conservado se adaptó del arnés medido para recibir binario y crear un
temporal nuevo; esa adaptación se validó sintácticamente. Requiere mpv, ffmpeg,
tmux y dbus-run-session; imprime el directorio de resultados.

```sh
go build -o /tmp/maly-ux03-build ./cmd/maly
dbus-run-session -- python3 docs/planning/evidence/ux-03-1.19.7/tmux-matrix.py /tmp/maly-ux03-build
```

Cada proceso termina con Ctrl+C y se espera la desaparición de su socket.
No usa `pkill`. Los buffers no dependen de los temporales de sesiones anteriores.
`SHA256SUMS` cubre texto, ANSI, JSON y el arnés conservado.
