// internal/platform/ui/colors.go
package ui

import "github.com/pterm/pterm"

// Paleta de colores "Infierno" - Inspirada en las capas del infierno dantesco
// y el caballo de fuego Aethon que ilumina la oscuridad digital

// Colores primarios
var (
	// EmberOrange - Llamas del infierno, elementos principales, iluminación
	EmberOrange = pterm.NewRGB(255, 107, 53)

	// InfernoRed - Fuego infernal, errores críticos
	InfernoRed = pterm.NewRGB(215, 38, 56)

	// CrimsonBlood - Río Estigia, errores graves
	CrimsonBlood = pterm.NewRGB(139, 0, 0)

	// MoltenGold - Metal fundido, warnings, descubrimientos importantes
	MoltenGold = pterm.NewRGB(255, 182, 39)

	// AshGray - Cenizas, texto secundario, elementos pendientes
	AshGray = pterm.NewRGB(61, 61, 61)

	// ObsidianBlack - Oscuridad profunda, fondos, separadores
	ObsidianBlack = pterm.NewRGB(26, 26, 26)

	// SmokeWhite - Humo, texto principal
	SmokeWhite = pterm.NewRGB(232, 232, 232)

	// LavaFlow - Lava en movimiento, running/active, animaciones
	LavaFlow = pterm.NewRGB(255, 69, 0)

	// DeepPurple - Sombras del Hades, información secundaria
	DeepPurple = pterm.NewRGB(75, 0, 130)

	// GhostCyan - Almas perdidas, acentos fríos, contraste
	GhostCyan = pterm.NewRGB(0, 206, 209)
)

// Estilos preconfigurados para diferentes contextos
var (
	// StylePrimary - Estilo principal para headers y elementos destacados
	StylePrimary = EmberOrange.ToRGBStyle()

	// StyleSuccess - Estilo para operaciones exitosas
	StyleSuccess = GhostCyan.ToRGBStyle()

	// StyleWarning - Estilo para advertencias
	StyleWarning = MoltenGold.ToRGBStyle()

	// StyleError - Estilo para errores
	StyleError = InfernoRed.ToRGBStyle()

	// StyleCritical - Estilo para errores críticos
	StyleCritical = CrimsonBlood.ToRGBStyle()

	// StyleSecondary - Estilo para texto secundario
	StyleSecondary = AshGray.ToRGBStyle()

	// StyleText - Estilo para texto principal
	StyleText = SmokeWhite.ToRGBStyle()

	// StyleActive - Estilo para elementos activos/running
	StyleActive = LavaFlow.ToRGBStyle()

	// StyleInfo - Estilo para información adicional
	StyleInfo = DeepPurple.ToRGBStyle()

	// StyleAccent - Estilo para acentos y highlights
	StyleAccent = GhostCyan.ToRGBStyle()
)

// Estilos de fondo para headers
var (
	// BgPrimary - Fondo principal (texto negro sobre fondo ember)
	BgPrimary = pterm.NewStyle(pterm.FgBlack).Sprint

	// BgSuccess - Fondo para éxito
	BgSuccess = pterm.NewStyle(pterm.FgBlack).Sprint

	// BgError - Fondo para error
	BgError = pterm.NewStyle(pterm.FgBlack).Sprint
)
