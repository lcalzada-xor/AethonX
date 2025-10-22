// internal/platform/ui/ascii.go
package ui

// ASCII art y banners temáticos para AethonX
// Inspirado en el caballo de fuego Aethon y la mitología del infierno

// AethonBannerCompact - Banner compacto y profesional para el header principal
const AethonBannerCompact = `
╔════════════════════════════════════════════════════════════╗
║                                                            ║
║     █████╗ ███████╗████████╗██╗  ██╗ ██████╗ ███╗   ██╗  ║
║    ██╔══██╗██╔════╝╚══██╔══╝██║  ██║██╔═══██╗████╗  ██║  ║
║    ███████║█████╗     ██║   ███████║██║   ██║██╔██╗ ██║  ║
║    ██╔══██║██╔══╝     ██║   ██╔══██║██║   ██║██║╚██╗██║  ║
║    ██║  ██║███████╗   ██║   ██║  ██║╚██████╔╝██║ ╚████║  ║
║    ╚═╝  ╚═╝╚══════╝   ╚═╝   ╚═╝  ╚═╝ ╚═════╝ ╚═╝  ╚═══╝  ║
║                                                            ║
║            Illuminating the Digital Underworld            ║
║                                                            ║
╚════════════════════════════════════════════════════════════╝
`

// AethonBannerMinimal - Banner minimalista para terminales pequeñas
const AethonBannerMinimal = `
╔═══════════════════════════════════════╗
║                                       ║
║    AETHONX                       🔥   ║
║    Reconnaissance Engine              ║
║    ════════════════════               ║
║    Illuminating Digital Assets        ║
║                                       ║
╚═══════════════════════════════════════╝
`

// AethonHorse - ASCII art del caballo de fuego (decorativo, opcional)
const AethonHorse = `
              ▲
             ▲▲▲
            ▲▲ ▲▲          🔥
           ▲▲   ▲▲        🔥🔥
          ▲▲     ▲▲      🔥 🔥
         ▲▲       ▲▲    🔥   🔥
        ▲▲    ◉    ▲▲  ════════
       ▲▲           ▲▲    ║║
      ▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲▲   ║║
     ▲▲              ▲▲  ║ ║
    ▲▲                ▲▲ ║  ║
   ▲▲                  ▲▲╚══╝

     AETHON - The Horse of Helios
`

// FlameFrameTop - Marco superior con efecto de llama
const FlameFrameTop = `
▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰
`

// FlameFrameBottom - Marco inferior con efecto de llama
const FlameFrameBottom = `
▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰▰
`

// ScanCompleteArt - Arte ASCII para completar escaneo
const ScanCompleteArt = `
    ⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡
    ⚡                        ⚡
    ⚡    SCAN COMPLETE       ⚡
    ⚡                        ⚡
    ⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡⚡
`

// ErrorFrameTop - Marco superior para errores críticos
const ErrorFrameTop = `
╔════════════════════════════════════════════════════════════╗
║ 💀 CRITICAL ERROR                                          ║
╠════════════════════════════════════════════════════════════╣
`

// ErrorFrameBottom - Marco inferior para errores críticos
const ErrorFrameBottom = `
╚════════════════════════════════════════════════════════════╝
`

// WarningFrameTop - Marco superior para warnings
const WarningFrameTop = `
╔════════════════════════════════════════════════════════════╗
║ 🔥 WARNING                                                 ║
╠════════════════════════════════════════════════════════════╣
`

// WarningFrameBottom - Marco inferior para warnings
const WarningFrameBottom = `
╚════════════════════════════════════════════════════════════╝
`

// InfoBox - Caja decorativa para información
type InfoBox struct {
	Title   string
	Content []string
}

// Render renderiza una caja de información con bordes Unicode
func (box InfoBox) Render() string {
	width := 60
	result := BorderTopLeft

	// Top border con título
	titleLen := len(box.Title)
	leftPad := (width - titleLen - 2) / 2
	rightPad := width - titleLen - leftPad - 2

	for i := 0; i < leftPad; i++ {
		result += BorderHorizontal
	}
	result += " " + box.Title + " "
	for i := 0; i < rightPad; i++ {
		result += BorderHorizontal
	}
	result += BorderTopRight + "\n"

	// Content lines
	for _, line := range box.Content {
		result += BorderVertical + " " + line
		// Pad to width
		padding := width - len(line) - 2
		for i := 0; i < padding; i++ {
			result += " "
		}
		result += " " + BorderVertical + "\n"
	}

	// Bottom border
	result += BorderBottomLeft
	for i := 0; i < width; i++ {
		result += BorderHorizontal
	}
	result += BorderBottomRight

	return result
}

// GetBanner retorna el banner apropiado según el ancho del terminal
func GetBanner(terminalWidth int) string {
	if terminalWidth < 80 {
		return AethonBannerMinimal
	}
	return AethonBannerCompact
}
