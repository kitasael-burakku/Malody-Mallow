package tui

import "maly/internal/config"

// La aritmética de color de la TUI es la de internal/config (dueño de la
// paleta: la usa para derivar accent_dim, surface y los colores de progreso).
// Estos dos alias existen para no repetirla acá y para no reescribir los ~10
// llamadores del paquete: hasta este cambio convivían dos copias de la misma
// cuenta por canal — la de logoRamp y la de blendHex —, y el interpolador que
// pedía el rediseño ya existía en nowplaying.go bajo otro nombre.
func parseHex(s string) [3]int { return config.ParseHex(s) }

func blendHex(a, b string, t float64) string { return config.BlendHex(a, b, t) }
