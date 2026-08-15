package config

import "fmt"

// Aritmética de color compartida. Vive acá porque config es el dueño de la
// paleta: la necesita para derivar accent_dim, surface y los colores de
// progreso a partir del accent (Theme.resolveDerived). internal/tui la reusa
// tal cual (ver internal/tui/color.go) en vez de mantener su propia copia:
// antes había DOS —la de logoRamp y la de blendHex/vizGradient— haciendo la
// misma cuenta por canal con distinta forma.

// ParseHex descompone "#rrggbb" en sus tres canales. Un valor mal formado
// devuelve negro en vez de fallar: los llamadores pintan, no validan (para
// validar está ValidHex, y Load ya lo aplica a todo lo que viene del TOML).
func ParseHex(s string) [3]int {
	var r, g, b int
	if len(s) == 7 && s[0] == '#' {
		fmt.Sscanf(s[1:], "%02x%02x%02x", &r, &g, &b)
	}
	return [3]int{r, g, b}
}

// BlendHex interpola linealmente entre dos colores en t∈[0,1] (0 = a, 1 = b),
// canal por canal, truncando como siempre lo hizo el gradiente del banner.
func BlendHex(a, b string, t float64) string {
	ca, cb := ParseHex(a), ParseHex(b)
	var c [3]int
	for i := range c {
		c[i] = ca[i] + int(t*float64(cb[i]-ca[i]))
	}
	return fmt.Sprintf("#%02x%02x%02x", c[0], c[1], c[2])
}

// scaleHex multiplica los tres canales por f (mezclar hacia el negro), que es
// como se oscurece un color conservando su tono.
func scaleHex(hex string, f float64) string { return BlendHex(hex, "#000000", 1-f) }
