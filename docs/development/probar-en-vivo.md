# Cómo probar en vivo — trampas conocidas

Fuente única: esta ficha. `CLAUDE.md` remite aquí antes de montar cualquier
sandbox o arnés de prueba. La disciplina de VERIFICACIÓN (cómo comprobar que
un test de verdad prueba lo que dice) vive aparte, en
`docs/development/verificacion.md`.


- Sandbox: `XDG_CONFIG_HOME/XDG_DATA_HOME/XDG_RUNTIME_DIR` apuntando a un dir de
  prueba. `XDG_RUNTIME_DIR` debe ser CORTO (p. ej. `/tmp/claude-1000/mt`): el
  path del socket de mpv revienta el límite (~108 chars) de sockets Unix.
  Poner `ao=null` en `$XDG_CONFIG_HOME/mpv/mpv.conf` del sandbox.
- TUI: probar bajo tmux (`new-session -d`, `send-keys`, `capture-pane -p`);
  bajo `script -qec` el init espera ~5 s por OSC 11. El pane NO es fish aunque
  el shell del usuario lo sea: usar `env VAR=... cmd`, no `set -x`.
- **tmux mata el grupo de procesos del pane** cuando su comando termina, así
  que un pane que corre `maly …` a secas se lleva por delante cualquier hijo
  que maly hubiera dejado — con lo cual NO sirve para comprobar si algo queda
  huérfano (medido: el binario con el defecto de A-15 daba 0 de 5). Para eso,
  que el pane siga vivo después: `maly …; sleep 60`.
- `tmux send-keys` TOKENIZA por espacios y busca nombres de tecla en cada
  token, **incluso con `-l`**: escribir `playlist delete favs` manda un
  `playlist`, la tecla **Delete** y un `favs`, así que a la TUI le llega
  `playlist  favs` y el comando falla por una razón que no tiene nada que ver
  con el código. Solo muerde con la palabra SUELTA (`AAdeleteBB` pasa entera).
  Se esquiva partiendo el token entre dos llamadas (`-l "playlist de"` +
  `-l "lete favs"`). Vale para cualquier token que sea nombre de tecla:
  `delete`, `up`, `space`, `enter`, `bspace`…
- Matar procesos de prueba SOLO por PID exacto (`pgrep -a -x maly`) y el mpv por
  su socket (`pkill -f "input-ipc-server=<runtime>/maly/mpv.sock"`). NUNCA
  `pkill -f` con cadenas que aparezcan en la propia línea de comandos del shell.
  El dueño corre `mpvpaper` permanente (parece mpv en pgrep).
- La DB real está en WAL: copiarla requiere los 3 archivos (`library.db`,
  `-wal`, `-shm`).
- Los pushes del demonio son FOTOS de estado, no eventos: los tests deben
  pollear hasta el estado final, nunca leer una sola vez.
- mpv con `--no-terminal` es totalmente mudo; para diagnosticar una muerte
  temprana se usa `--input-terminal=no` y se captura **stdout** (mpv escribe ahí).
