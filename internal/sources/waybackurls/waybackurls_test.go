package waybackurls

import (
	"context"
	"testing"
	"time"

	"aethonx/internal/core/domain"
	"aethonx/internal/platform/logx"
)

func TestParser_ParseLine(t *testing.T) {
	logger := logx.New()
	parser := NewParser(logger, "waybackurls")
	target := domain.Target{Root: "example.com"}

	tests := []struct {
		name              string
		line              string
		expectedURLCount  int
		expectedSubdomain bool
		expectedEndpoint  bool
		expectedParam     bool
	}{
		{
			name:              "simple URL",
			line:              "https://example.com/path",
			expectedURLCount:  2, // URL + endpoint
			expectedSubdomain: false,
			expectedEndpoint:  true,
			expectedParam:     false,
		},
		{
			name:              "URL with timestamp",
			line:              "2020-09-30 22:51:11 https://example.com/api/v1/users",
			expectedURLCount:  3, // URL + endpoint + API
			expectedSubdomain: false,
			expectedEndpoint:  true,
			expectedParam:     false,
		},
		{
			name:              "subdomain URL",
			line:              "https://api.example.com/graphql",
			expectedURLCount:  4, // URL + subdomain + endpoint + API
			expectedSubdomain: true,
			expectedEndpoint:  true,
			expectedParam:     false,
		},
		{
			name:              "URL with parameters",
			line:              "https://example.com/search?q=test&page=1",
			expectedURLCount:  4, // URL + endpoint + 2 params
			expectedSubdomain: false,
			expectedEndpoint:  true,
			expectedParam:     true,
		},
		{
			name:              "JavaScript file",
			line:              "https://example.com/js/app.min.js",
			expectedURLCount:  3, // URL + endpoint + JS
			expectedSubdomain: false,
			expectedEndpoint:  true,
			expectedParam:     false,
		},
		{
			name:              "backup file",
			line:              "https://example.com/backup.sql",
			expectedURLCount:  3, // URL + endpoint + backup
			expectedSubdomain: false,
			expectedEndpoint:  true,
			expectedParam:     false,
		},
		{
			name:              "git repository",
			line:              "https://example.com/.git/config",
			expectedURLCount:  3, // URL + endpoint + repository
			expectedSubdomain: false,
			expectedEndpoint:  true,
			expectedParam:     false,
		},
		{
			name:              "out of scope domain",
			line:              "https://other.com/path",
			expectedURLCount:  0,
			expectedSubdomain: false,
			expectedEndpoint:  false,
			expectedParam:     false,
		},
		{
			name:              "empty line",
			line:              "",
			expectedURLCount:  0,
			expectedSubdomain: false,
			expectedEndpoint:  false,
			expectedParam:     false,
		},
		{
			name:              "invalid URL",
			line:              "not-a-url",
			expectedURLCount:  0,
			expectedSubdomain: false,
			expectedEndpoint:  false,
			expectedParam:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			artifacts := parser.ParseLine(tt.line, target)

			if len(artifacts) != tt.expectedURLCount {
				t.Errorf("expected %d artifacts, got %d", tt.expectedURLCount, len(artifacts))
			}

			if tt.expectedSubdomain {
				found := false
				for _, a := range artifacts {
					if a.Type == domain.ArtifactTypeSubdomain {
						found = true
						break
					}
				}
				if !found {
					t.Error("expected subdomain artifact but not found")
				}
			}

			if tt.expectedEndpoint {
				found := false
				for _, a := range artifacts {
					if a.Type == domain.ArtifactTypeEndpoint {
						found = true
						break
					}
				}
				if !found {
					t.Error("expected endpoint artifact but not found")
				}
			}

			if tt.expectedParam {
				found := false
				for _, a := range artifacts {
					if a.Type == domain.ArtifactTypeParameter {
						found = true
						break
					}
				}
				if !found {
					t.Error("expected parameter artifact but not found")
				}
			}
		})
	}
}

func TestParser_ExtractURLAndTimestamp(t *testing.T) {
	logger := logx.New()
	parser := NewParser(logger, "waybackurls")

	tests := []struct {
		name              string
		line              string
		expectedURL       string
		expectedTimestamp string
	}{
		{
			name:              "URL with timestamp",
			line:              "2020-09-30 22:51:11 https://example.com/path",
			expectedURL:       "https://example.com/path",
			expectedTimestamp: "2020-09-30 22:51:11",
		},
		{
			name:              "URL without timestamp",
			line:              "https://example.com/path",
			expectedURL:       "https://example.com/path",
			expectedTimestamp: "",
		},
		{
			name:              "http URL with timestamp",
			line:              "2020-09-30 22:51:11 http://example.com/path",
			expectedURL:       "http://example.com/path",
			expectedTimestamp: "2020-09-30 22:51:11",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, timestamp := parser.extractURLAndTimestamp(tt.line)

			if url != tt.expectedURL {
				t.Errorf("expected URL %q, got %q", tt.expectedURL, url)
			}

			if timestamp != tt.expectedTimestamp {
				t.Errorf("expected timestamp %q, got %q", tt.expectedTimestamp, timestamp)
			}
		})
	}
}

func TestURLAnalyzer_DetectSensitiveFile(t *testing.T) {
	logger := logx.New()
	analyzer := NewURLAnalyzer(logger)

	tests := []struct {
		name     string
		path     string
		url      string
		expected bool
	}{
		{"env file", "/.env", "https://example.com/.env", true},
		{"config php", "/config.php", "https://example.com/config.php", true},
		{"database yml", "/config/database.yml", "https://example.com/config/database.yml", true},
		{"normal file", "/index.html", "https://example.com/index.html", false},
		{"normal path", "/about", "https://example.com/about", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			artifact := analyzer.detectSensitiveFile(tt.path, tt.url)
			if tt.expected && artifact == nil {
				t.Error("expected sensitive file detection but got nil")
			}
			if !tt.expected && artifact != nil {
				t.Error("expected no detection but got artifact")
			}
		})
	}
}

func TestURLAnalyzer_DetectBackupFile(t *testing.T) {
	logger := logx.New()
	analyzer := NewURLAnalyzer(logger)

	tests := []struct {
		name     string
		path     string
		url      string
		expected bool
	}{
		{"bak file", "/index.php.bak", "https://example.com/index.php.bak", true},
		{"old file", "/backup.old", "https://example.com/backup.old", true},
		{"sql file", "/database.sql", "https://example.com/database.sql", true},
		{"sql gz file", "/backup.sql.gz", "https://example.com/backup.sql.gz", true},
		{"normal file", "/index.html", "https://example.com/index.html", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			artifact := analyzer.detectBackupFile(tt.path, tt.url)
			if tt.expected && artifact == nil {
				t.Error("expected backup file detection but got nil")
			}
			if !tt.expected && artifact != nil {
				t.Error("expected no detection but got artifact")
			}
		})
	}
}

func TestURLAnalyzer_DetectRepository(t *testing.T) {
	logger := logx.New()
	analyzer := NewURLAnalyzer(logger)

	tests := []struct {
		name     string
		path     string
		url      string
		expected bool
	}{
		{"git config", "/.git/config", "https://example.com/.git/config", true},
		{"git head", "/.git/HEAD", "https://example.com/.git/HEAD", true},
		{"svn entries", "/.svn/entries", "https://example.com/.svn/entries", true},
		{"normal path", "/git/", "https://example.com/git/", false},
		{"normal file", "/index.html", "https://example.com/index.html", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			artifact := analyzer.detectRepository(tt.path, tt.url)
			if tt.expected && artifact == nil {
				t.Error("expected repository detection but got nil")
			}
			if !tt.expected && artifact != nil {
				t.Error("expected no detection but got artifact")
			}
		})
	}
}

func TestURLAnalyzer_DetectAPI(t *testing.T) {
	logger := logx.New()
	analyzer := NewURLAnalyzer(logger)

	tests := []struct {
		name     string
		path     string
		url      string
		expected bool
	}{
		{"api v1", "/api/v1/users", "https://example.com/api/v1/users", true},
		{"rest api", "/rest/products", "https://example.com/rest/products", true},
		{"graphql", "/graphql", "https://example.com/graphql", true},
		{"versioned api", "/v2/posts", "https://example.com/v2/posts", true},
		{"normal path", "/about", "https://example.com/about", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			artifact := analyzer.detectAPI(tt.path, tt.url)
			if tt.expected && artifact == nil {
				t.Error("expected API detection but got nil")
			}
			if !tt.expected && artifact != nil {
				t.Error("expected no detection but got artifact")
			}
		})
	}
}

func TestWaybackurlsSource_BuildCommand(t *testing.T) {
	logger := logx.New()
	target := domain.Target{Root: "example.com"}

	tests := []struct {
		name      string
		withDates bool
		noSubs    bool
		wantArgs  int
	}{
		{"no flags", false, false, 0},
		{"with dates", true, false, 1},
		{"no subs", false, true, 1},
		{"both flags", true, true, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := NewWithConfig(logger, "waybackurls", 60*time.Second, tt.withDates, tt.noSubs)
			cmd := source.buildCommand(context.Background(), target)

			if len(cmd.Args)-1 != tt.wantArgs { // -1 because Args[0] is the command itself
				t.Errorf("expected %d args, got %d", tt.wantArgs, len(cmd.Args)-1)
			}
		})
	}
}

func TestWaybackurlsSource_Validate(t *testing.T) {
	logger := logx.New()

	tests := []struct {
		name      string
		execPath  string
		timeout   time.Duration
		wantError bool
	}{
		{"valid config", "waybackurls", 60 * time.Second, false},
		{"empty exec path", "", 60 * time.Second, true},
		{"zero timeout", "waybackurls", 0, true},
		{"negative timeout", "waybackurls", -1 * time.Second, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := NewWithConfig(logger, tt.execPath, tt.timeout, false, false)
			err := source.Validate()

			if tt.wantError && err == nil {
				t.Error("expected error but got nil")
			}
			if !tt.wantError && err != nil {
				t.Errorf("expected no error but got: %v", err)
			}
		})
	}
}

func TestWaybackurlsSource_Interfaces(t *testing.T) {
	logger := logx.New()
	source := New(logger)

	// Check basic methods
	if source.Name() != "waybackurls" {
		t.Errorf("expected name 'waybackurls', got %q", source.Name())
	}

	if source.Mode() != domain.SourceModePassive {
		t.Errorf("expected passive mode, got %v", source.Mode())
	}

	if source.Type() != domain.SourceTypeCLI {
		t.Errorf("expected CLI type, got %v", source.Type())
	}
}
