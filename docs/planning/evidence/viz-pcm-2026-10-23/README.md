# Visualizador: PCM de PipeWire interpretado como ruido

Revisión del 2026-10-23 sobre `cbe32d8` (v1.22.11), con la sesión real del
dueño: Go `go1.27.3-X:nodwarf5`, PipeWire 1.6.10 y Kitty 0.49.2. El cambio de
producción consiste únicamente en pedir `--raw` a `pw-record`.

## Observado

El candidato PipeWire pedía `--format s16 --rate 44100 --channels 1 -`, pero
no PCM crudo. Su stdout empezaba por `2e736e64` (`.snd`): cabecera AU de 24
bytes, encoding 3 (PCM de 16 bits big-endian), 44100 Hz y un canal. `readLoop`
interpreta todos los pares de bytes como s16le. No se trata solo de una
cabecera visible durante un frame: el orden de bytes incorrecto afecta toda
la captura y produce energía falsa de banda ancha.

En los mismos datos capturados, descartando la cabecera, la amplitud RMS era
0,0484 al decodificar correctamente big-endian y 0,5732 al interpretarlos como
little-endian. Con `--raw`, la salida no llevaba cabecera y las muestras
little-endian eran coherentes con audio. El manual oficial explica tanto el
[AU por stdout como la opción `--raw`](https://docs.pipewire.org/page_man_pw-cat_1.html).

Las capturas son recortes reales de ventanas Kitty en Hyprland, de dos TUI
simultáneas conectadas al mismo demonio. Ambas muestran «Nightmares Again» a
04:54/05:34, con idéntica geometría. La captura sucesiva de las ventanas no
garantiza el mismo bloque PCM, pero evita comparar canciones o pasajes
diferentes:

- [Antes](antes.png): binario instalado, sin `--raw`.
- [Después](despues.png): binario de prueba, con `--raw`, compilado con el
  mismo Go. La baja energía real de ese pasaje permite ver con claridad el
  ruido que inventaba la lectura anterior.

El visualizador analiza el monitor de salida del sistema, no el texto de las
letras ni únicamente la voz. No se encontró evidencia de un fallo del
compilador: cambiar el contrato de captura basta para corregir este caso.
No se determinó en qué actualización empezó a resultar perceptible; no se
compararon versiones anteriores de PipeWire o Go.

## Verificación

`TestPipeWireCapturaPCMCrudo` arranca un capturador de prueba que reproduce el
contrato observado: AU big-endian por defecto, PCM crudo con `--raw`. Verifica
las muestras que llegan al ring y la banda dominante de un seno de 440 Hz.
Sin el argumento, compila y falla por la muestra mal decodificada; después
de restaurar producción mediante `cp`, pasa. Resultado de la contraprueba:

```text
muestra 1: 0.007874, quería PCM s16le 0.015656;
posible contenedor o endianness incorrecto
```

Pasaron la suite completa sin CGo con D-Bus privado, la suite completa con
`-race`, `go vet ./...` y el build del binario de prueba con el Go actual.
Los comandos Go se ejecutaron fuera del sandbox: dentro, Go rechazaba
paquetes estándar presentes en disco. La prueba real no modificó la cola,
la posición, el volumen ni el servicio del dueño.

## Super+M, configuración personal

El bind existía y su comando podía abrir maly. El dueño confirmó que con la
tecla la ventana aparecía y se cerraba. Usaba una búsqueda textual en
`hyprctl clients` y `pkill -f 'kitty --title maly'`. Se comprobó con `pgrep`,
sin enviar señales, que el patrón también encuentra un shell que contiene
esa cadena. Ese riesgo está demostrado; no se atribuye con certeza cada
cierre observado a esa coincidencia.

Se reemplazó únicamente el módulo privado personal por un callback Lua:
abrir Kitty con clase `maly`, consultar `hl.get_windows({ class = "maly" })`
y cerrar sus objetos de ventana con `hl.dsp.window.close`. La API de consulta
usa igualdad exacta: el primer intento con `^maly$` no encontraba ventanas;
se corrigió y se verificó con eventos del compositor.

El callback real pasó abrir → permanecer → cerrar, conservando las demás
ventanas y el demonio (PID 1055). El dueño confirmó después que Super+M abre
y cierra correctamente desde su teclado. Hyprland recargó sin errores.
El original quedó en `/tmp/maly-viz-review/private.lua.before`; no se copia
la configuración privada al repositorio. Las ventanas comparativas se
cerraron por dirección exacta y se restauró el escritorio 1.

## Aplicación local y vuelta atrás

Tras la verificación se instaló el mismo binario probado en `./maly` y
`~/.local/bin/maly`, con sustitución atómica del instalado. SHA-256 de ambos:
`378397f712cabb10a1060cb1dcd2cec543084e5f7ec41787fccb2c86f4828e9a`.
El demonio conservó el PID 1055 y su reproducción: el arreglo corresponde a
la captura que arranca cada TUI nueva. Basta cerrar y reabrir la TUI.

Los binarios previos quedaron como respaldos temporales en
`/tmp/maly-viz-review/maly-installed.before` y `maly-repo.before`; la
configuración original está en `private.lua.before` del mismo directorio.
Restaurar el módulo exige recargar Hyprland. En ese punto del diagnóstico
todavía no se había preparado una release.

Después, por encargo del dueño, el arreglo se preparó como v1.22.12 con bump,
changelog, commit y tag anotado locales. Las capturas y el SHA anterior
corresponden al binario de diagnóstico v1.22.11 con el arreglo; no se atribuyen
al binario recompilado de la release. Publicación pendiente del push del dueño.
