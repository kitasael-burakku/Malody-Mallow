# UX-06 — guardar la cola (2026-10-18)

Base real: `main` limpio en `4b6d9ef`, const 1.20.8, último tag local v1.19.5.
Resultado: **1.20.9**, implementado y verificado; commit de cierre autorizado,
tag pendiente. Sin push, PKGBUILD, sustitución del binario instalado ni reinicio
del servicio real. No se verificó el remoto.

## Contrato y reproducción

El contrato vive en el [roadmap](../../roadmap-ui-ux.md#ux-06--guardar-la-cola-como-playlist).
`baseline.json` conserva versión y salidas de la base compilada antes de editar:
`playlist create/list` funciona y `playlist save` falla por subcomando desconocido.
Es una oportunidad nueva, no un defecto preexistente. Se guardaron copias exactas
antes de editar en `/tmp/maly-ux06-backup`; no son necesarias para repetir las
pruebas del resultado.

## Evidencia

- `checks.json`, `build.log`, `vet.log`, `suite.log`, `race.log`: build y vet
  correctos; 19 paquetes y 789 casos/subcasos por suite, cero saltos/fallos.
  mpv real, audio nulo, XDG aislado y bus privado. Los logs vacíos de build/vet
  significan éxito silencioso; sus códigos están en el JSON.
- `reverse.json` y `reverse.log`: seis neutralizaciones compilables fallan por
  la aserción prevista (atomicidad, orden, instantánea, atajo, consola, CLI).
  Todas restauradas con `cp` y verificadas en verde.
- `matrix.json`: es/en; 140×42, 100×28, 80×24, 40×12 y regreso a 140×42;
  default/vim/override Ctrl+Y, color/NO_COLOR, embebido/servicio privado.
  El arnés consulta SQLite e IPC: cola B,A,B conservada con filtro a cero y
  shuffle; mismo estado en pausa, misma pista y avance natural al reproducir;
  errores sin escrituras parciales, nombres con espacios y refresco a otra TUI.
- `*-saved.txt/.ansi`: mensajes de éxito en los cuatro tamaños.
  `*-empty`, `*-occupied`, `*-unindexed-40x12`: errores observados; el remedio
  `maly scan` queda visible incluso a 40×12. `*-entry`, `*-now-entry`, `*-help`:
  entrada preparada, acceso desde Ahora suena y ayuda con tecla resuelta.
  `override-second-client`: playlist nueva visible en el segundo cliente.
- `SOURCE-SHA256SUMS`: fuentes Go modificadas/nuevas del resultado.
  `SHA256SUMS`: integridad de esta carpeta, salvo el propio manifiesto.

Las pruebas Go incluyen rollback después de una inserción válida, competencia
por un mismo nombre desde dos conexiones, IDs de cola obsoletos, ausencia de
biblioteca/demonio y 25 guardados mientras el demonio mueve la cola. No se usan
índices de snapshots distintos. El test antiguo de scroll de sugerencias dejó
de asumir un número fijo de comandos.

## Repetir

Desde la raíz del repositorio, con Go, mpv, ffmpeg, tmux, Python y dbus-run-session:

```sh
CGO_ENABLED=0 go build -o /tmp/maly-ux06-after ./cmd/maly
dbus-run-session -- python3 docs/planning/evidence/ux-06-1.20.9/live.py /tmp/maly-ux06-after
python3 docs/planning/evidence/ux-06-1.20.9/reverse.py
dbus-run-session -- python3 docs/planning/evidence/ux-06-1.20.9/checks.py
```

`reverse.py` modifica una fuente por vez y restaura con `cp` incluso si falla.
No ejecutarlo a la vez que otra compilación o edición de esas fuentes. Los
arneses actualizan sus logs/capturas y crean XDG temporales. Tras repetirlos,
regenerar manifiestos si se quieren conservar los resultados nuevos.

El sandbox bloqueó la caché de Go/sockets: se ejecutó mediante el mecanismo de
permisos, sin atribuir ese bloqueo al producto. El bus privado imprimió intentos
fallidos de activar su servicio systemd; las conexiones IPC, mpv y MPRIS de la
prueba funcionaron. Los ajustes del arnés y sus causas están en el
[cierre de ingeniería](../../../history/roadmap-1.17.md#v1209-2026-10-18--ux-06-guardar-la-cola).
Después de los checks solo se corrigió un comentario de Ahora suena y se
actualizó documentación; no cambió la lógica probada.

## Límites

Son buffers de tmux, no capturas del escritorio. No validan fuentes,
transparencia, carátulas Kitty ni audio audible. La música es local y sintética;
no se necesitó red ni descarga. El aviso de refresco es best-effort, como en las
otras mutaciones de playlists: desconectar tras el commit no deshace el guardado.
No hay migración ni formato nuevo; las playlists son compatibles con la base.
