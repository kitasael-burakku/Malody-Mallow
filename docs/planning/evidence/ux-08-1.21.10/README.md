# UX-08 — evidencia local de 1.21.10

Fecha: 2026-10-19. Base HEAD `0121e52` con UX-07 sin commit, const 1.20.10;
último tag local v1.19.5. [Contrato](../../roadmap-ui-ux.md#ux-08--reproducir-después)
y [cierre de ingeniería](../../../history/roadmap-1.17.md#v12110-2026-10-19--ux-08-reproducir-después).
No se consultó remoto ni se tocó la instalación del usuario.

## Reproducir

Desde la raíz del repo, con Go, mpv, Python, D-Bus y tmux disponibles:

```sh
CGO_ENABLED=0 go build -o /tmp/maly-ux08-after ./cmd/maly
dbus-run-session -- python3 docs/planning/evidence/ux-08-1.21.10/live.py /tmp/maly-ux08-after
dbus-run-session -- python3 docs/planning/evidence/ux-08-1.21.10/reverse.py
dbus-run-session -- python3 docs/planning/evidence/ux-08-1.21.10/checks.py
```

`reverse.py` modifica producción temporalmente, una neutralización por vez,
y restaura con `cp` incluso si falla. Ejecutarlo con árbol estable, sin otra
edición concurrente. Los arneses aíslan XDG en `/tmp`; live desactiva red de
actualización y visualizador, usa `ao=null` y un socket privado de tmux.
El detector de carreras necesita CGo; el build y la suite normal no lo usan.
Si el sandbox bloquea sockets o la caché Go, hace falta permiso fuera de él.

La base no es simplemente HEAD: incluye UX-07 sin commit. `base.json` identifica
esas fuentes, respaldadas en `/tmp/maly-ux08-backup`; `baseline.json` registra
el hash del binario UX-07 y el rechazo de `playnext` (exit 1, subcomando
desconocido). Para repetir esa observación se necesita compilar aquel árbol,
no mezclar los tests UX-08 con producción antigua. Las neutralizaciones sí
son reproducibles sobre esta entrega sin el respaldo temporal.

## Resultados

- `checks.json` y logs build/vet/suite/race: correctos. **19 paquetes,
  842 casos/subcasos**, sin saltos ni fallos en las dos suites. mpv real y bus
  privado, sin audio al dispositivo real.
- `reverse.json`/`reverse.log`: once fallos compilables por la causa prevista
  y once pases tras restaurar. Prioridad, consumo FIFO, move/remove/replace,
  sesión, promesa obsoleta, daemon, CLI, ruta TUI y marcador.
- `matrix.json`: 3 perfiles × 140×42 → 100×28 → 80×24 → 40×12 → 140×42.
  Español/default/color, inglés/vim/NO_COLOR, español/Ctrl+Y/NO_COLOR;
  este último usa un demonio privado separado, los otros uno embebido.
- `*-scheduled`, `*-second-client`, `*-second-refreshed`: dos TUIs, marcador
  y refresco; pausa, actual y posición conservadas al agendar y redimensionar.
- `*-songs-next`, `*-console-next`, `*-queue-filter-next`, `*-select-next`:
  acciones reales y selección filtrada por ruta. `*-filter-ready` prueba que
  el arnés espera el filtro del frame actual, no texto previo al resize.
- `*-queue-cli`: presentación compartida de la prioridad en CLI.
- Cada perfil comprueba seis transiciones efectivas B → C → C → D → D → B
  contra el socket de mpv; luego reinicia el demonio y verifica FIFO C → D,
  pausa y modos restaurados. Nombres con espacios, Unicode, NFD y título largo.

Los tests de cola cruzan shuffle on/off × repeat off/all/one, mutaciones y
restauración. Los de daemon comprueban A → B → C natural con WAV cortos y
LoadCount sin cargas adicionales, con shuffle on/off y repeat-one. También
fuerzan el callback de una promesa antigua y la carrera con dos clientes IPC:
ese orden de callbacks se controla desde el test, separado de la transición
natural real. La prueba de sesión elimina un archivo antes de restaurar y
conserva las entradas repetidas restantes. Tests TUI cubren biblioteca y
archivos descargados, además de selectores/cola/consola, con etiquetas iguales
y teclas predeterminadas/personalizadas; la matriz tmux no descarga audio.

Cada captura está en texto y ANSI: son buffers de terminal, no imágenes de
Kitty. No acreditan escritorio, carátulas, audio audible ni descarga remota.
La corrección excepcional de una promesa que mpv ya encadenó puede recargar
la pista; solo el camino natural sano acredita ausencia de cargas extra.

`build-identity.json` registra hashes de los binarios usados; el build final coincide
byte por byte con el ejecutado en la matriz tmux. `SOURCE-SHA256SUMS`
identifica las fuentes Go finales; tras las suites solo se corrigieron
comentarios de producción y documentación. `SHA256SUMS` identifica arneses y
evidencias (excluye ese propio manifiesto). Los manifiestos se verifican desde
la raíz del repo. Cierre posterior autorizado en commits separados: UX-07 en `5806445`, UX-08
en el siguiente commit. Tags, publicación y despliegue pendientes.
