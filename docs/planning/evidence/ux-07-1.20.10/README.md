# UX-07 — evidencia local de 1.20.10

Fecha: 2026-10-19. Base limpia `0121e52` / 1.20.9; último tag local v1.19.5.
[Contrato](../../roadmap-ui-ux.md#ux-07--continuidad-después-de-descargar) y
[cierre de ingeniería](../../../history/roadmap-1.17.md#v12010-2026-10-19--ux-07-continuidad-después-de-descargar).
No se consultó el remoto ni se tocó la instalación del usuario.

## Reproducir

Desde la raíz del repo, con Go, mpv, ffmpeg, Python, D-Bus y tmux disponibles:

```sh
# baseline.py espera un binario de 0121e52 en /tmp/maly-ux07-before.
python3 docs/planning/evidence/ux-07-1.20.10/baseline.py
CGO_ENABLED=0 go build -o /tmp/maly-ux07-after ./cmd/maly
dbus-run-session -- python3 docs/planning/evidence/ux-07-1.20.10/live.py /tmp/maly-ux07-after
python3 docs/planning/evidence/ux-07-1.20.10/reverse.py
dbus-run-session -- python3 docs/planning/evidence/ux-07-1.20.10/checks.py
```

`reverse.py` modifica producción temporalmente, una neutralización por vez, y
restaura cada archivo con `cp` incluso si falla. Ejecutarlo con árbol estable,
sin otra edición concurrente. Los demás arneses aíslan XDG en `/tmp`; live usa
bus privado, `ao=null`, sin visualizador ni chequeo de actualización. Los
comandos requieren permiso para sockets si el sandbox los bloquea.

## Resultados

- `baseline.json`: nuevo indexado; existente y parcial con audio en disco pero
  sin indexar; vacío/fallo total ya fallaban. El binario actual queda cubierto
  por los tests de CLI y por la matriz en vivo.
- `checks.json`, `build.log`, `vet.log`, `suite.log`, `race.log`: build/vet OK;
  **19 paquetes, 810 casos/subcasos**, cero saltos/fallos, tanto sin CGo como
  con detector de carreras. mpv disponible, sin audio al dispositivo real.
- `reverse.json`/`reverse.log`: once fallos compilables por la causa prevista
  y once pases tras restaurar. Incluye CLI/TUI, rutas y generaciones.
- `matrix.json`: 3 perfiles × redimensionado 140×42 → 100×28 → 80×24 → 40×12
  → 140×42. Español/default/color, inglés/vim/NO_COLOR, español/override
  Ctrl+Y/NO_COLOR; este último usa un demonio privado separado de la TUI.
- `*-search`, `*-owned-hint`: búsqueda y significado aproximado de ✓.
  `*-new`, `*-files`: selector local, nombres y acciones útiles.
- `*-existing`, `*-partial`, `*-empty`, `*-failure`: desenlaces distintos.
  `*-scan-failed*`, `*-scan-retried`: bloqueo de SQLite y recuperación con
  Ctrl+R, sin mutación de cola. `*-playlist-partial`: playlist y archivos.
- `*-search-cancelled`, `*-download-cancelled`, `*-exit-search`: PID simulado
  muerto antes de limpiar tmux, con shell persistente para detectar huérfanos.

Cada captura está en texto y ANSI; son **buffers de terminal**, no imágenes
de Kitty. IPC verifica la segunda ruta entre archivos con metadatos iguales;
SQLite verifica archivos indexados, y la cola no cambia al terminar descargas.
No se acreditan descarga remota real, cookies, conversión/descendientes reales
de yt-dlp/ffmpeg, contraste del escritorio, carátulas Kitty ni audio audible.
El sustituto de yt-dlp no tiene hijos: el chequeo de PID cubre ese proceso.

`build-identity.json` registra base y hashes de los binarios de prueba.
`SOURCE-SHA256SUMS` identifica las fuentes Go de la entrega comprobada;
`SHA256SUMS` identifica los arneses y sus evidencias (excluye este último
manifiesto). Commit/tag, publicación, empaquetado y despliegue pendientes.
