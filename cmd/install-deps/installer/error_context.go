package installer

import (
	"fmt"
	"strings"
)

// ErrorContext provides detailed context and solutions for installation errors.
type ErrorContext struct {
	ToolName  string
	Phase     string // "download", "extract", "install", "validate"
	Error     error
	Reason    string
	Solutions []string
	DocsURL   string
}

// String formats the error context for display.
func (ec *ErrorContext) String() string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("\n    ERROR: %s\n", ec.Error))

	if ec.Reason != "" {
		b.WriteString(fmt.Sprintf("    REASON: %s\n", ec.Reason))
	}

	if len(ec.Solutions) > 0 {
		b.WriteString("\n    SOLUTIONS:\n")
		for i, solution := range ec.Solutions {
			b.WriteString(fmt.Sprintf("    %d) %s\n", i+1, solution))
		}
	}

	if ec.DocsURL != "" {
		b.WriteString(fmt.Sprintf("\n    For more help: %s\n", ec.DocsURL))
	}

	return b.String()
}

// AnalyzeError creates an ErrorContext from a raw error.
func AnalyzeError(toolName string, phase string, err error, docsURL string) *ErrorContext {
	if err == nil {
		return nil
	}

	ctx := &ErrorContext{
		ToolName: toolName,
		Phase:    phase,
		Error:    err,
		DocsURL:  docsURL,
	}

	errMsg := strings.ToLower(err.Error())

	// Analyze error type and provide solutions
	switch {
	case strings.Contains(errMsg, "rate limit") || strings.Contains(errMsg, "403"):
		ctx.Reason = "GitHub API rate limit exceeded (60 requests/hour for unauthenticated requests)"
		ctx.Solutions = []string{
			"Wait 1 hour and try again",
			"Set GITHUB_TOKEN environment variable for higher rate limit (5000/hour):\n       export GITHUB_TOKEN=ghp_your_token_here\n       Then run: ./install-deps",
			"Create a GitHub token at: https://github.com/settings/tokens (no scopes needed)",
			fmt.Sprintf("Install manually: Check installation docs at %s", docsURL),
		}

	case strings.Contains(errMsg, "timeout") || strings.Contains(errMsg, "deadline exceeded"):
		ctx.Reason = "Network request timeout - slow or unstable connection"
		ctx.Solutions = []string{
			"Check your internet connection and retry",
			"Try again with increased timeout: ./install-deps (will retry automatically)",
			"Use a VPN if GitHub is blocked in your region",
			"Install manually if network issues persist",
		}

	case strings.Contains(errMsg, "no such host") || strings.Contains(errMsg, "dns"):
		ctx.Reason = "DNS resolution failed - cannot reach GitHub servers"
		ctx.Solutions = []string{
			"Check your internet connection",
			"Verify DNS settings: try 'ping github.com'",
			"Try using Google DNS (8.8.8.8) or Cloudflare DNS (1.1.1.1)",
			"Check if GitHub is accessible: https://www.githubstatus.com/",
		}

	case strings.Contains(errMsg, "connection refused") || strings.Contains(errMsg, "connection reset"):
		ctx.Reason = "Network connection was refused or reset"
		ctx.Solutions = []string{
			"Check if a firewall is blocking outbound connections",
			"Verify proxy settings if behind corporate proxy",
			"Try again in a few moments (temporary network issue)",
		}

	case strings.Contains(errMsg, "no asset pattern"):
		ctx.Reason = "No compatible release found for your platform"
		ctx.Solutions = []string{
			fmt.Sprintf("Your platform may not be supported by %s", toolName),
			"Check supported platforms in the project documentation",
			"Try installing from source code instead",
		}

	case strings.Contains(errMsg, "not found in archive") || strings.Contains(errMsg, "binary") && strings.Contains(errMsg, "not found"):
		ctx.Reason = "Downloaded archive doesn't contain expected binary"
		ctx.Solutions = []string{
			"The release archive structure may have changed",
			"Try forcing reinstall: ./install-deps --force",
			"Install manually from the GitHub releases page",
		}

	case strings.Contains(errMsg, "permission denied"):
		ctx.Reason = "Insufficient permissions to write to installation directory"
		ctx.Solutions = []string{
			"Run with sudo: sudo ./install-deps",
			"Or install to a user-writable directory: ./install-deps --dir ~/bin",
			"Check directory permissions: ls -la ~/go/bin",
		}

	case strings.Contains(errMsg, "no space left"):
		ctx.Reason = "Insufficient disk space"
		ctx.Solutions = []string{
			"Free up disk space and try again",
			"Check available space: df -h",
		}

	case strings.Contains(errMsg, "health check failed"):
		ctx.Reason = "Installation succeeded but tool validation failed"
		ctx.Solutions = []string{
			"The tool may be installed but not working correctly",
			"Try reinstalling: ./install-deps --force",
			"Check if dependencies are missing: ldd $(which " + toolName + ")",
			"Verify PATH is set correctly",
		}

	case strings.Contains(errMsg, "already exists"):
		ctx.Reason = "Tool appears to be already installed"
		ctx.Solutions = []string{
			"Use --force to reinstall: ./install-deps --force",
			"Or check installation: which " + toolName,
		}

	default:
		// Generic error
		ctx.Reason = "An unexpected error occurred during installation"
		ctx.Solutions = []string{
			"Try running with verbose mode: ./install-deps --verbose",
			"Check the installation logs for details",
			"Try forcing reinstall: ./install-deps --force",
			"Install manually if the issue persists",
		}
	}

	return ctx
}

// GetDocumentationURL returns the documentation URL for a tool.
func GetDocumentationURL(toolName string) string {
	urls := map[string]string{
		"subfinder":    "https://github.com/projectdiscovery/subfinder",
		"httpx":        "https://github.com/projectdiscovery/httpx",
		"amass":        "https://github.com/owasp-amass/amass",
		"waybackurls":  "https://github.com/tomnomnom/waybackurls",
		"go-modules":   "https://golang.org/doc/install",
	}

	if url, ok := urls[strings.ToLower(toolName)]; ok {
		return url
	}

	return "https://github.com/search?q=" + toolName
}
