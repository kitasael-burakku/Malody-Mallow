# UX-02 — evidencia de 1.19.6, 2026-10-18

Base: `b8ebe5a0cae4c72b0a019e869ccbfef0abdeeeba` (`v1.19.5`).
Implementación y verificación locales; commit autorizado; sin tag de 1.19.6 ni publicación.
[Alcance y aceptación](../../roadmap-ui-ux.md#ux-02--ayudas-visibles-y-contextuales)
y [cierre de ingeniería](../../../history/roadmap-1.17.md#v1196-2026-10-18--ux-02-ayudas-visibles-y-contextuales).

## Qué son estos archivos

Son **buffers reales de tmux** (`capture-pane -p` y `-p -e`), texto y ANSI.
No son capturas de escritorio ni imágenes renderizadas. Espacios y filas
vacías se conservan. `SHA256SUMS` permite comprobar su integridad.

- `before-*`: reproducción anterior. El pie a 80×24 conserva `a agrega`
  pese a configurar `add=f7`, y pierde el filtro. Ese buffer fue tomado durante
  el redimensionado y conserva restos en los paneles superiores: solo se usa
  como evidencia del pie. `songs-40x12` muestra el remedio y Esc recortados.
- `es-*`/`en-*`: selección de resultados nuevos en los cuatro tamaños,
  instrucciones de canciones y pie principal. Los buffers de 140×42 son el
  regreso desde tamaño pequeño, no solo la primera apertura.
- `*override*`: teclas propias, incluida Space para agregar; `cli-help` permite
  contrastar la fuente de atajos compartida. `*no-color*`: pasada sin color.
- `matrix-results.json`: 60 puntos aprobados, 48 combinaciones distintas.
- `checks.json`: build/vet, paquetes aprobados y ausencia de saltos/fallos,
  más resultados de regresión en ambas direcciones. Resumen extraído de los
  logs JSON de Go; no es una copia completa de esos logs.

## Recorrido y condiciones

XDG nuevo por ejecución, socket tmux propio, `ao=null`, updates y visualizador
apagados y D-Bus privado. es/en × default/vim/overrides × color/NO_COLOR, con
140×42 → 100×28 → 80×24 → 40×12 → 140×42. El recorrido espera contenido
observable y prueba expandir/plegar, filtro, agregar, cambiar de panel, ayuda
hasta el final, canciones, playlists, consola y ahora suena; cierra cada modal
y vuelve. Genera una pista FLAC de silencio con acento, carátula PNG embebida
y letra LRC. No usa la biblioteca, el servicio ni el bus reales del usuario.

Se ejecutó el arnés en `/tmp/maly-ux02`; el ejemplar conservado
[`tmux-matrix.py`](tmux-matrix.py) está adaptado para recibir el binario y
crear un directorio temporal nuevo. La adaptación se comprobó sintácticamente;
la matriz medida corresponde al mismo recorrido antes de esa adaptación.
Requiere mpv, ffmpeg, tmux y dbus-run-session:

```sh
go build -o /tmp/maly-ux02-build ./cmd/maly
dbus-run-session -- python3 docs/planning/evidence/ux-02-1.19.6/tmux-matrix.py /tmp/maly-ux02-build
```

El script imprime el directorio de resultados. Cada TUI embebida termina con
Ctrl+C y se comprueba que desaparezca su socket. No usa `pkill`. La advertencia
al activar `org.freedesktop.systemd1` en el bus privado no impidió la ejecución;
no se sustituyó por el bus de la sesión real.

## Pruebas y límites

`TestUX02*` verifica el mensaje correcto, ausencia de atajos antiguos, Enter
según selección, ancho/alto, posición del pie y retorno tras flash/scan. La
ayuda recorre claves largas y espacio a 40×12. Se conserva la suite UX-01 para
carga, error persistente, vacío y filtro sin coincidencias. Con producción
anterior copiada mediante `cp`, tres grupos nuevos fallan compilando por el
defecto; al restaurar pasan. El control de prioridad pasa en ambas versiones.
Build, vet, suite sin CGo y `-race` pasaron; mpv disponible, ningún test saltado.

No acredita contraste sobre fondos claros/oscuros reales, fuentes ni arte
Kitty. El arte bajo tmux usa half-blocks. No es una auditoría de metadatos
Unicode ni una nueva validación de UX-00. Los errores y scans transitorios se
prueban con estados deterministas, no con fallos reales de disco ni capturas
cronometradas. No se alteraron IPC/cola compartida y no se repitieron servicio
externo/dos clientes. El buscador de red no hizo descargas: el picker compartido
se revisó en todos sus consumidores y se verificó con la suite local.
