package main

import "github.com/charmbracelet/lipgloss"

// Paleta de la salida de la CLI (help, info, config, doctor, controls, logo).
// Cada color lleva su par Catppuccin: Mocha para fondo oscuro y Latte para
// fondo claro. Con colores fijos de Mocha, en un terminal claro el verde de
// los comandos y el gris de las notas apenas se leían. lipgloss decide el
// lado preguntándole el fondo al terminal (OSC 11) solo si stdout es un
// terminal; si no contesta (tmux, por ejemplo) asume oscuro, que es lo que
// había antes.
var (
	clrAccent = lipgloss.AdaptiveColor{Dark: "#89b4fa", Light: "#1e66f5"}
	clrCmd    = lipgloss.AdaptiveColor{Dark: "#a6e3a1", Light: "#40a02b"}
	clrDim    = lipgloss.AdaptiveColor{Dark: "#6c7086", Light: "#6c6f85"}
	clrWarn   = lipgloss.AdaptiveColor{Dark: "#f9e2af", Light: "#df8e1d"}
	clrFail   = lipgloss.AdaptiveColor{Dark: "#f38ba8", Light: "#d20f39"}
)
