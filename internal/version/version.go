// Package version guarda la versión de maly en un punto importable por
// cualquier paquete: la CLI la imprime, el demonio la adjunta en cada
// respuesta IPC y la TUI la compara para detectar un servicio desparejado.
package version

// Version es la versión del binario (sin la "v").
const Version = "1.2.0"
