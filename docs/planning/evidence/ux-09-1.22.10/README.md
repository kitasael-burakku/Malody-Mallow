# UX-09 — evidencia local de 1.22.10

Fecha: 2026-10-19. Base limpia `7d68623` / 1.21.10, último tag local v1.19.5.
[Contrato](../../roadmap-ui-ux.md#ux-09--deshacer-una-eliminación-de-cola) y
[cierre de ingeniería](../../../history/roadmap-1.17.md#v12210-2026-10-19--ux-09-deshacer-una-eliminación).
Sin consultar remoto ni tocar instalación, servicio o biblioteca reales.

## Reproducir

Desde la raíz del repo, con Go, mpv, Python, D-Bus y tmux disponibles:

```sh
CGO_ENABLED=0 go build -o /tmp/maly-ux09-after ./cmd/maly
dbus-run-session -- python3 docs/planning/evidence/ux-09-1.22.10/live.py /tmp/maly-ux09-after
dbus-run-session -- python3 docs/planning/evidence/ux-09-1.22.10/reverse.py
dbus-run-session -- python3 docs/planning/evidence/ux-09-1.22.10/checks.py
```

`reverse.py` modifica producción temporalmente y restaura con `cp` incluso
si falla. Ejecutar sin ediciones ni pruebas concurrentes sobre esas fuentes.
Los otros arneses aíslan XDG bajo `/tmp`. Live usa bus y socket tmux privados,
mpv `ao=null`, visualizador y chequeo de actualización apagados, WAV sintéticos.
El detector de carreras necesita CGo; el build y la suite normal no lo usan.
Si el sandbox bloquea sockets/caché Go hace falta permiso fuera de él.

## Resultados

- `base.json` identifica HEAD y respaldo de fuentes. `baseline.json` registra
  el rechazo de `undo` por el binario UX-08 (exit 1, subcomando desconocido).
  Sus fuentes coinciden con las de la base; fue compilado antes de su commit.
- `checks.json` y logs build/vet/suite/race: correctos. **19 paquetes,
  910 casos/subcasos por suite**, sin saltos ni fallos, con mpv real.
- `reverse.json`/`reverse.log`: diez fallos compilables por la aserción prevista
  y diez pases tras restaurar. Registro, token, identidad actual, prioridad
  FIFO, slot shuffle, caducidad al añadir y al terminar la cola, ausencia de
  recarga de audio, token TUI y gramática CLI.
- `matrix.json`: español/default/color, inglés/vim/NO_COLOR y español/Ctrl+Y/
  NO_COLOR; este último con demonio privado separado, los otros embebido.
  140×42 → 100×28 → 80×24 → 40×12 → 140×42. El pie mantiene la acción visible.
- `*-removed`, `*-second-available`, `*-restored`, `*-second-restored`:
  dos TUIs comparten una eliminación y la segunda la recupera sin cambiar
  la pista pausada ni su posición.
- `*-filtered-removed`, `*-console-restored`: borrar la entrada visible D
  por ruta exacta y recuperar desde consola. Los nombres incluyen espacios,
  título largo, Unicode ancho y NFD; la biblioteca de prueba no tiene álbum.
- `*-stale.json`: un token anterior falla sin consumir el nuevo ni pisar cola.
- `*-old-current-restored`: recuperar la antigua actual conserva la nueva
  actual y pausa/posición. El arnés consulta además la ruta efectiva de mpv.
- Cada perfil verifica caducidad al añadir y al reiniciar el demonio completo.

Los tests de cola cubren posiciones, shuffle, duplicados de ruta distinguidos
por ID, conservación de duraciones aprendidas, FIFO, cola única e invalidación
por mutador. Los del daemon verifican LoadCount intacto, token distinto tras
reinicio, un solo ganador entre dos clientes IPC y caducidad por EOF real,
incluido fin de cola. La gramática CLI/consola y la tecla/ayuda personalizada
se comprueban sin depender de traducir un mensaje en el propio test.

Las capturas conservan los renglones vacíos del terminal: son buffers de texto
y ANSI, no imágenes de Kitty. No acreditan fuentes, contraste del escritorio,
carátulas ni audio audible. No hay descargas remotas en este arnés. Undo
recupera una entrada de cola, no archivos borrados ni el tiempo de reproducción.

`build-identity.json` registra los binarios de prueba. `SOURCE-SHA256SUMS`
identifica las fuentes Go y dependencias finales; tras las suites solo se
actualizaron documentación y comentarios. `SHA256SUMS` identifica arneses y
evidencias, excluyéndose a sí mismo. Ambos se verifican desde la raíz del repo.
Commit/tag, publicación, empaquetado y despliegue pendientes.
