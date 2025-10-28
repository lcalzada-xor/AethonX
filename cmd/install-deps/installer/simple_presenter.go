package installer

import (
	"fmt"
	"strings"
	"time"
)

// SimplePresenter provides clean, focused output for dependency installation.
type SimplePresenter struct {
	quiet bool
}

// NewSimplePresenter creates a new simple presenter.
func NewSimplePresenter(quiet bool) *SimplePresenter {
	return &SimplePresenter{
		quiet: quiet,
	}
}

// ShowHeader displays the installation header.
func (s *SimplePresenter) ShowHeader() {
	if s.quiet {
		return
	}
	fmt.Println()
	fmt.Println("════════════════════════════════════════════════════════════")
	fmt.Println("    🔧 AethonX Dependency Installer")
	fmt.Println("════════════════════════════════════════════════════════════")
	fmt.Println()
}

// ShowPreCheck displays pre-installation check results.
func (s *SimplePresenter) ShowPreCheck(results []InstallationResult, force bool) {
	if s.quiet {
		return
	}

	fmt.Println("📦 DEPENDENCY CHECK")
	fmt.Println()

	toInstall := []InstallationResult{}
	alreadyInstalled := []InstallationResult{}

	for _, result := range results {
		if result.Status == StatusAlreadyInstalled {
			alreadyInstalled = append(alreadyInstalled, result)
		} else {
			toInstall = append(toInstall, result)
		}
	}

	if len(toInstall) > 0 || force {
		if force {
			fmt.Println("Dependencies to reinstall:")
		} else {
			fmt.Println("Dependencies to install:")
		}
		for _, result := range results {
			if force || result.Status != StatusAlreadyInstalled {
				fmt.Printf("  ○ %-15s (not installed)\n", result.Dependency.Name)
			}
		}
		fmt.Println()
	}

	if len(alreadyInstalled) > 0 && !force {
		fmt.Println("Already installed:")
		for _, result := range alreadyInstalled {
			// Clean version (truncate long/multiline output)
			version := result.Version
			if idx := strings.Index(version, "\n"); idx > 0 {
				version = version[:idx]
			}
			if len(version) > 10 {
				version = version[:7] + "..."
			}
			fmt.Printf("  ✓ %-15s (%s)\n", result.Dependency.Name, version)
		}
		fmt.Println()
	}

	if force {
		fmt.Printf("Total: %d to reinstall\n", len(results))
	} else {
		fmt.Printf("Total: %d to install, %d already installed\n", len(toInstall), len(alreadyInstalled))
	}
	fmt.Println()
	fmt.Println("────────────────────────────────────────────────────────────")
	fmt.Println()
}

// ShowCheckResults displays check-only mode results.
func (s *SimplePresenter) ShowCheckResults(results []InstallationResult) {
	fmt.Println()
	fmt.Println("📦 DEPENDENCY STATUS")
	fmt.Println()

	installed := 0
	missing := 0

	for _, result := range results {
		// Clean version (truncate long/multiline output)
		version := result.Version
		if idx := strings.Index(version, "\n"); idx > 0 {
			version = version[:idx]
		}
		if len(version) > 10 {
			version = version[:7] + "..."
		}

		switch result.Status {
		case StatusAlreadyInstalled:
			fmt.Printf("  ✓ %-15s v%-10s (installed)\n", result.Dependency.Name, version)
			installed++
		case StatusPending:
			fmt.Printf("  ✗ %-15s %-10s   (missing)\n", result.Dependency.Name, "-")
			missing++
		default:
			fmt.Printf("  ⚠ %-15s %-10s   (check failed)\n", result.Dependency.Name, "-")
			missing++
		}
	}

	fmt.Println()
	if missing > 0 {
		fmt.Printf("To install %d missing dependencies, run: ./install-deps\n", missing)
	} else {
		fmt.Println("All dependencies are installed ✓")
	}
	fmt.Println()
}

// StartInstallation shows installation start message.
func (s *SimplePresenter) StartInstallation(count int) {
	if s.quiet {
		return
	}
	fmt.Printf("Installing %d dependencies...\n\n", count)
}

// ShowProgress displays real-time installation progress.
func (s *SimplePresenter) ShowProgress(toolName string, phase InstallationPhase, message string) {
	if s.quiet {
		return
	}
	phaseIcon := map[InstallationPhase]string{
		PhaseChecking:    "🔍",
		PhaseDownloading: "⬇",
		PhaseExtracting:  "📦",
		PhaseInstalling:  "🔧",
		PhaseValidating:  "✓",
	}
	icon := phaseIcon[phase]
	if icon == "" {
		icon = "•"
	}
	fmt.Printf("  %s %-15s %s\n", icon, toolName, message)
}

// ShowResult displays a single installation result.
func (s *SimplePresenter) ShowResult(result InstallationResult) {
	// Clean up version string (truncate multi-line output)
	version := result.Version
	if idx := strings.Index(version, "\n"); idx > 0 {
		version = version[:idx] // Take only first line
	}
	if len(version) > 50 {
		version = version[:47] + "..."
	}

	switch result.Status {
	case StatusSuccess:
		if s.quiet {
			fmt.Printf("✓ %s v%s\n", result.Dependency.Name, version)
		} else {
			fmt.Printf("  ✓ %-15s v%-10s (%.1fs)\n", result.Dependency.Name, version, result.Duration.Seconds())
		}

	case StatusAlreadyInstalled:
		if !s.quiet {
			fmt.Printf("  ✓ %-15s v%-10s (already installed)\n", result.Dependency.Name, version)
		}

	case StatusFailed:
		fmt.Printf("  ✗ %-15s FAILED\n", result.Dependency.Name)
		if !s.quiet && result.ErrorContext != nil {
			fmt.Println()
			s.showErrorContext(result.ErrorContext)
		}

	case StatusSkipped:
		if !s.quiet {
			fmt.Printf("  ⊘ %-15s skipped\n", result.Dependency.Name)
		}
	}
}

// showErrorContext displays detailed error information.
func (s *SimplePresenter) showErrorContext(ec *ErrorContext) {
	indent := "      "

	fmt.Printf("%sERROR: %s\n", indent, ec.Error)

	if ec.Reason != "" {
		fmt.Printf("%sREASON: %s\n", indent, ec.Reason)
	}

	if len(ec.Solutions) > 0 {
		fmt.Println()
		fmt.Printf("%sSOLUTIONS:\n", indent)
		for i, solution := range ec.Solutions {
			// Wrap long lines
			wrapped := wrapText(solution, 60)
			lines := strings.Split(wrapped, "\n")
			fmt.Printf("%s%d) %s\n", indent, i+1, lines[0])
			for j := 1; j < len(lines); j++ {
				fmt.Printf("%s   %s\n", indent, lines[j])
			}
		}
	}

	if ec.DocsURL != "" {
		fmt.Println()
		fmt.Printf("%sDOCS: %s\n", indent, ec.DocsURL)
	}
	fmt.Println()
}

// ShowSummary displays the final installation summary.
func (s *SimplePresenter) ShowSummary(results []InstallationResult, duration time.Duration, pathWarning string) {
	fmt.Println()
	fmt.Println("════════════════════════════════════════════════════════════")
	fmt.Println("⚡ INSTALLATION SUMMARY")
	fmt.Println()

	succeeded := []InstallationResult{}
	failed := []InstallationResult{}

	for _, result := range results {
		if result.Status == StatusSuccess || result.Status == StatusAlreadyInstalled {
			succeeded = append(succeeded, result)
		} else if result.Status == StatusFailed {
			failed = append(failed, result)
		}
	}

	// Success section
	if len(succeeded) > 0 {
		fmt.Printf("✓ INSTALLED (%d)\n", len(succeeded))
		for _, result := range succeeded {
			// Clean version (truncate long/multiline output)
			version := result.Version
			if idx := strings.Index(version, "\n"); idx > 0 {
				version = version[:idx]
			}
			if len(version) > 20 {
				version = version[:17] + "..."
			}

			if result.Status == StatusSuccess {
				location := "installed"
				if result.InstallPath != "" {
					location = result.InstallPath
				}
				fmt.Printf("  ✓ %-15s v%-10s → %s\n", result.Dependency.Name, version, location)
			} else {
				fmt.Printf("  ✓ %-15s v%-10s (already installed)\n", result.Dependency.Name, version)
			}
		}
		fmt.Println()
	}

	// Failure section
	if len(failed) > 0 {
		fmt.Printf("✗ FAILED (%d)\n", len(failed))
		for _, result := range failed {
			reason := "unknown error"
			if result.ErrorContext != nil && result.ErrorContext.Reason != "" {
				reason = result.ErrorContext.Reason
			} else if result.Error != nil {
				reason = result.Error.Error()
				if len(reason) > 50 {
					reason = reason[:47] + "..."
				}
			}
			fmt.Printf("  ✗ %-15s → %s\n", result.Dependency.Name, reason)
		}
		fmt.Println()
	}

	fmt.Println("────────────────────────────────────────────────────────────")

	// Duration
	fmt.Printf("⏱  Duration: %.1fs\n", duration.Seconds())

	// PATH warning
	if pathWarning != "" {
		fmt.Println()
		fmt.Println("⚠  PATH WARNING")
		fmt.Println()
		fmt.Println("   ~/go/bin is not in your PATH.")
		fmt.Println("   Add it with:")
		fmt.Println()
		fmt.Println("     export PATH=\"$HOME/go/bin:$PATH\"")
		fmt.Println()
		fmt.Println("   Or permanently:")
		fmt.Println("     echo 'export PATH=\"$HOME/go/bin:$PATH\"' >> ~/.bashrc")
	}

	// Next steps for failures
	if len(failed) > 0 {
		fmt.Println()
		fmt.Println("📋 NEXT STEPS")
		fmt.Println()
		fmt.Println("   • Review error messages above for specific solutions")
		fmt.Println("   • Run with --verbose for detailed logs")
		fmt.Println("   • Retry with --force to reinstall")
		fmt.Println("   • Install manually if issues persist")
	}

	fmt.Println()
	fmt.Println("════════════════════════════════════════════════════════════")
	fmt.Println()
}

// wrapText wraps text at the specified width.
func wrapText(text string, width int) string {
	if len(text) <= width {
		return text
	}

	var result strings.Builder
	words := strings.Fields(text)
	line := ""

	for _, word := range words {
		if len(line)+len(word)+1 > width {
			if result.Len() > 0 {
				result.WriteString("\n")
			}
			result.WriteString(line)
			line = word
		} else {
			if line != "" {
				line += " "
			}
			line += word
		}
	}

	if line != "" {
		if result.Len() > 0 {
			result.WriteString("\n")
		}
		result.WriteString(line)
	}

	return result.String()
}
