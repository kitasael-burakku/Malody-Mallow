# Barras uniformes del visualizador

Trabajo sobre `a881792` (v1.22.12), autorizado el 2026-10-23 e incorporado
después a v1.22.13. Las capturas corresponden al binario preparatorio 1.22.12,
previo al bump; su hash está en [binary.json](binary.json).

Posteriormente se añadió un selector entre estas barras y la franja anterior;
el análisis fijo de 96 bandas descrito aquí corresponde al estilo `bars`.
[Contrato y evidencia del selector](../viz-styles-2026-10-23/README.md).

## Comportamiento

La FFT entrega 96 bandas logarítmicas estables. La pantalla agrupa intervalos
contiguos conservando su pico, desde los graves hasta los agudos. Cada barra
tiene el mismo ancho; los huecos miden una o dos celdas y absorben el sobrante
para ocupar toda la franja. Hay hasta 96 barras; en 240 columnas son 80 y en
40 columnas son 13. No se fuerzan 48 barras en ventanas donde no caben.

La franja principal y Ctrl+T usan el mismo render, con geometrías resueltas
por `computeLayout` para sus respectivos anchos exterior e interior. El
capturador conserva sus 96 bandas al redimensionar: su suavizado no se
reinicia y el agrupado no descarta el extremo agudo. La captura `--raw`,
FFT, autoganancia común, curva perceptual, gravedad, alturas y gradiente
configurable conservan sus reglas. No se añade ganancia por barra.

## Capturas reales

- [Antes](antes.png) y [después](despues.png): ventanas simultáneas con audio
  real de PipeWire, ambas en «Amanecer (Remastered)», 03:07. Son capturas
  sucesivas, no del mismo bloque PCM. Estas dos ventanas heredaron
  `NO_COLOR=1` del entorno de pruebas; sirven para comparar la geometría.
- [240×42 con los colores del config](color-240x42.png).
- [Ctrl+T a 140×42](color-140x42-ctrl-t.png).
- [80×24](color-80x24.png) y [40×12](color-40x12.png): la franja se sacrifica
  por altura en el mínimo, según la regla existente.
- [140×42 sin color](sin-color-140x42.png).

Las capturas de tamaños incluyen JSON con las dimensiones efectivas de
Kitty/Hyprland; se exigió que coincidieran con las solicitadas. La primera
tanda de redimensionado no coincidió y se descartó; no forma parte de esta
evidencia. Para las capturas de color se quitó `NO_COLOR` únicamente del
entorno de las ventanas de prueba, sin modificar el config personal.

Además se comprobaron visualmente tonos PCM de aproximadamente 86 Hz,
1,034 kHz y 7,989 kHz mediante un capturador simulado en un PATH temporal.
No se emitió sonido. Esa prueba complementa las capturas con PipeWire real;
no se presenta como captura de audio real.

El aviso visible de versión corresponde al demonio real 1.22.11 frente al
cliente 1.22.12 y ya aparecía antes de este cambio. El servicio conservó
el PID 27456. No se reinició ni recibió comandos de reproducción; las
ventanas de prueba se cerraron por su propio socket Kitty y se restauró
el escritorio 1.

## Verificación

`visualizer_test.go` comprueba anchos uniformes, separaciones, ocupación del
ancho completo, todos los picos posibles entre las 96 bandas, silencio,
extremo agudo visible en ambas vistas completas y conservación de la caída
al redimensionar. Incluye tamaños hasta 1000 columnas.

Tres contrapruebas retiraron por separado el dibujo uniforme, el agrupado
del espectro completo y las bandas de análisis estables. Todas compilaron
y fallaron por la causa esperada: [dibujo](red-dibujo.txt),
[espectro](red-espectro.txt), [caída](red-caida.txt). Producción se restauró
mediante `cp`; los tests nuevos [pasaron después](green.txt).

Pasaron `dbus-run-session -- env CGO_ENABLED=0 go test -count=1 ./...`,
`dbus-run-session -- go test -race -count=1 ./...`, `go vet ./...` y build sin
CGo. Los tests completos se ejecutaron fuera del sandbox porque este
bloquea los sockets Unix de las pruebas existentes.

## Aplicación local

Se instaló el binario probado en `./maly` y `~/.local/bin/maly` con sustitución
atómica, verificando el hash de ambos. El servicio conservó su PID y no se
reinició. Las nuevas TUI usan las barras uniformes; el demonio 1.22.11 seguirá
avisando de su versión hasta que el dueño lo reinicie.

El binario instalado anterior quedó en
`/tmp/maly-uniform-bars/maly-installed.before`; el de la raíz del repo, en
`/tmp/maly-uniform-bars/maly.before`. Son respaldos temporales para volver al
dibujo anterior. Estas pruebas precedieron a la preparación de v1.22.13.
