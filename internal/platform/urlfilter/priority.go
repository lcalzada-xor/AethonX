package urlfilter

import (
	"net/url"
	"path/filepath"
	"sort"
	"strings"

	"aethonx/internal/platform/logx"
)

// PriorityScorer assigns priority scores to URLs based on their characteristics.
// Higher scores = more valuable URLs for reconnaissance.
type PriorityScorer struct {
	weights ScoreWeights
	logger  logx.Logger
}

// ScoreWeights defines scoring weights for various URL characteristics.
type ScoreWeights struct {
	// High-value targets (positive weights)
	SensitiveFile   int // .env, config.php, credentials.json, etc.
	Repository      int // .git, .svn, .hg exposed
	BackupFile      int // .bak, .old, .sql, .dump
	AdminPath       int // /admin, /dashboard, /console
	APIEndpoint     int // /api/, /rest/, /graphql
	AuthPath        int // /login, /auth, /oauth
	DatabasePath    int // /phpmyadmin, /adminer
	ConfigPath      int // /config/, /settings/
	UploadPath      int // /upload/, /uploads/, /files/
	HasParameters   int // Query parameters (potential injection points)
	DeepPath        int // Many path segments (might be interesting)

	// Low-value targets (negative weights)
	StaticAsset     int // .jpg, .png, .css, .js
	TrackingParam   int // utm_, ga_, fb_ parameters
	PaginationParam int // page=, offset=, limit=
	LongPath        int // Very long paths (likely noise)
	CommonAssetDir  int // /assets/, /static/, /images/
}

// DefaultScoreWeights returns balanced default weights.
func DefaultScoreWeights() ScoreWeights {
	return ScoreWeights{
		// High-value (positive)
		SensitiveFile:   1000,
		Repository:      800,
		BackupFile:      600,
		AdminPath:       400,
		APIEndpoint:     300,
		AuthPath:        350,
		DatabasePath:    450,
		ConfigPath:      300,
		UploadPath:      250,
		HasParameters:   100,
		DeepPath:        50,

		// Low-value (negative)
		StaticAsset:     -200,
		TrackingParam:   -100,
		PaginationParam: -50,
		LongPath:        -150,
		CommonAssetDir:  -100,
	}
}

// NewPriorityScorer creates a new priority scorer.
func NewPriorityScorer(weights ScoreWeights, logger logx.Logger) *PriorityScorer {
	return &PriorityScorer{
		weights: weights,
		logger:  logger.With("component", "priority"),
	}
}

// ScoredURL represents a URL with its priority score.
type ScoredURL struct {
	URL      string                 // Original URL
	Score    int                    // Total priority score
	Reasons  []string               // Explanation of score components
	Metadata map[string]interface{} // Additional metadata
}

// Score calculates priority score for a URL.
func (p *PriorityScorer) Score(rawURL string) ScoredURL {
	scored := ScoredURL{
		URL:      rawURL,
		Score:    0,
		Reasons:  make([]string, 0, 5),
		Metadata: make(map[string]interface{}),
	}

	// Parse URL
	parsed, err := url.Parse(rawURL)
	if err != nil {
		p.logger.Debug("failed to parse URL for scoring", "url", rawURL, "error", err.Error())
		scored.Score = -1000 // Invalid URL gets very low score
		scored.Reasons = append(scored.Reasons, "invalid_url")
		return scored
	}

	// Score based on path characteristics
	p.scorePath(parsed, &scored)

	// Score based on query parameters
	p.scoreParameters(parsed, &scored)

	// Score based on file characteristics
	p.scoreFile(parsed, &scored)

	return scored
}

// scorePath scores URL based on path characteristics.
func (p *PriorityScorer) scorePath(parsed *url.URL, scored *ScoredURL) {
	path := strings.ToLower(parsed.Path)

	// Sensitive files (CRITICAL)
	if p.detectSensitiveFile(path) {
		scored.Score += p.weights.SensitiveFile
		scored.Reasons = append(scored.Reasons, "sensitive_file")
		scored.Metadata["category"] = "critical"
	}

	// Exposed repositories (CRITICAL)
	if p.detectRepository(path) {
		scored.Score += p.weights.Repository
		scored.Reasons = append(scored.Reasons, "repository")
		scored.Metadata["category"] = "critical"
	}

	// Backup files (HIGH)
	if p.detectBackupFile(path) {
		scored.Score += p.weights.BackupFile
		scored.Reasons = append(scored.Reasons, "backup_file")
		scored.Metadata["category"] = "high"
	}

	// Admin paths (HIGH)
	if p.detectAdminPath(path) {
		scored.Score += p.weights.AdminPath
		scored.Reasons = append(scored.Reasons, "admin_path")
		scored.Metadata["category"] = "high"
	}

	// API endpoints (MEDIUM-HIGH)
	if p.detectAPIPath(path) {
		scored.Score += p.weights.APIEndpoint
		scored.Reasons = append(scored.Reasons, "api_endpoint")
		scored.Metadata["category"] = "medium"
	}

	// Auth paths (MEDIUM-HIGH)
	if p.detectAuthPath(path) {
		scored.Score += p.weights.AuthPath
		scored.Reasons = append(scored.Reasons, "auth_path")
		scored.Metadata["category"] = "medium"
	}

	// Database admin interfaces (HIGH)
	if p.detectDatabasePath(path) {
		scored.Score += p.weights.DatabasePath
		scored.Reasons = append(scored.Reasons, "database_path")
		scored.Metadata["category"] = "high"
	}

	// Config paths (MEDIUM)
	if p.detectConfigPath(path) {
		scored.Score += p.weights.ConfigPath
		scored.Reasons = append(scored.Reasons, "config_path")
		scored.Metadata["category"] = "medium"
	}

	// Upload paths (MEDIUM)
	if p.detectUploadPath(path) {
		scored.Score += p.weights.UploadPath
		scored.Reasons = append(scored.Reasons, "upload_path")
		scored.Metadata["category"] = "medium"
	}

	// Common asset directories (LOW)
	if p.detectCommonAssetDir(path) {
		scored.Score += p.weights.CommonAssetDir
		scored.Reasons = append(scored.Reasons, "common_asset_dir")
	}

	// Path depth scoring
	segments := strings.Split(strings.Trim(path, "/"), "/")
	segmentCount := len(segments)

	if segmentCount > 8 {
		// Very deep paths might be noise
		scored.Score += p.weights.LongPath
		scored.Reasons = append(scored.Reasons, "very_deep_path")
	} else if segmentCount >= 4 && segmentCount <= 7 {
		// Moderately deep paths might be interesting
		scored.Score += p.weights.DeepPath
		scored.Reasons = append(scored.Reasons, "deep_path")
	}
}

// scoreParameters scores URL based on query parameters.
func (p *PriorityScorer) scoreParameters(parsed *url.URL, scored *ScoredURL) {
	query := parsed.Query()
	if len(query) == 0 {
		return
	}

	hasTracking := false
	hasPagination := false
	interestingParams := 0

	for key := range query {
		keyLower := strings.ToLower(key)

		// Tracking parameters (negative)
		if p.isTrackingParam(keyLower) {
			hasTracking = true
			continue
		}

		// Pagination parameters (negative)
		if p.isPaginationParam(keyLower) {
			hasPagination = true
			continue
		}

		// Other parameters are potentially interesting
		interestingParams++
	}

	if hasTracking {
		scored.Score += p.weights.TrackingParam
		scored.Reasons = append(scored.Reasons, "tracking_params")
	}

	if hasPagination {
		scored.Score += p.weights.PaginationParam
		scored.Reasons = append(scored.Reasons, "pagination_params")
	}

	if interestingParams > 0 {
		scored.Score += p.weights.HasParameters * interestingParams
		scored.Reasons = append(scored.Reasons, "has_parameters")
		scored.Metadata["param_count"] = interestingParams
	}
}

// scoreFile scores URL based on file characteristics.
func (p *PriorityScorer) scoreFile(parsed *url.URL, scored *ScoredURL) {
	base := filepath.Base(parsed.Path)
	ext := strings.ToLower(filepath.Ext(base))

	if ext == "" {
		return
	}

	// Static assets (negative)
	if p.isStaticAsset(ext) {
		scored.Score += p.weights.StaticAsset
		scored.Reasons = append(scored.Reasons, "static_asset")
		scored.Metadata["extension"] = ext
	}
}

// Detection helpers

func (p *PriorityScorer) detectSensitiveFile(path string) bool {
	sensitivePatterns := []string{
		".env", ".env.local", ".env.production",
		"config.php", "config.yml", "config.yaml", "config.json",
		"database.yml", "database.yaml",
		"credentials.json", "credentials.yml",
		"web.config", "app.config",
		".htpasswd", ".htaccess",
		"id_rsa", "id_dsa", "id_ecdsa", "id_ed25519",
		"authorized_keys", "known_hosts",
		"secrets.yml", "secrets.yaml", "secrets.json",
		"settings.py", "settings.php",
		"application.properties",
		"privatekey", "private.key",
		"password", "passwd",
	}

	for _, pattern := range sensitivePatterns {
		if strings.Contains(path, pattern) {
			return true
		}
	}
	return false
}

func (p *PriorityScorer) detectRepository(path string) bool {
	repoPatterns := []string{
		"/.git/", "/.svn/", "/.hg/", "/.bzr/", "/cvs/",
		".git/config", ".git/head",
	}

	for _, pattern := range repoPatterns {
		if strings.Contains(path, pattern) {
			return true
		}
	}
	return false
}

func (p *PriorityScorer) detectBackupFile(path string) bool {
	backupExts := []string{
		".bak", ".old", ".backup", ".orig", ".save", ".copy",
		".sql", ".sql.gz", ".sql.bz2", ".sql.zip",
		".tar.gz", ".tar.bz2", ".zip", ".rar", ".7z",
		".dump", ".dmp",
		"~", // Unix backup suffix
	}

	for _, ext := range backupExts {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}
	return false
}

func (p *PriorityScorer) detectAdminPath(path string) bool {
	adminPatterns := []string{
		"/admin", "/administrator", "/administrador",
		"/dashboard", "/panel", "/console",
		"/cpanel", "/plesk", "/whm",
		"/wp-admin", "/wp-login",
		"/manager", "/management",
	}

	for _, pattern := range adminPatterns {
		if strings.Contains(path, pattern) {
			return true
		}
	}
	return false
}

func (p *PriorityScorer) detectAPIPath(path string) bool {
	apiPatterns := []string{
		"/api/", "/api-", "/api.",
		"/rest/", "/restapi/",
		"/graphql", "/v1/", "/v2/", "/v3/", "/v4/",
		"/webapi/", "/webservice/",
	}

	for _, pattern := range apiPatterns {
		if strings.Contains(path, pattern) {
			return true
		}
	}
	return false
}

func (p *PriorityScorer) detectAuthPath(path string) bool {
	authPatterns := []string{
		"/login", "/signin", "/sign-in",
		"/auth", "/oauth", "/sso",
		"/logout", "/signout",
		"/register", "/signup", "/sign-up",
		"/forgot", "/reset", "/password",
	}

	for _, pattern := range authPatterns {
		if strings.Contains(path, pattern) {
			return true
		}
	}
	return false
}

func (p *PriorityScorer) detectDatabasePath(path string) bool {
	dbPatterns := []string{
		"/phpmyadmin", "/pma",
		"/adminer", "/adminer.php",
		"/mysql", "/mssql", "/postgres",
		"/database", "/db",
	}

	for _, pattern := range dbPatterns {
		if strings.Contains(path, pattern) {
			return true
		}
	}
	return false
}

func (p *PriorityScorer) detectConfigPath(path string) bool {
	configPatterns := []string{
		"/config/", "/configuration/",
		"/settings/", "/setup/",
		"/install/", "/installer/",
	}

	for _, pattern := range configPatterns {
		if strings.Contains(path, pattern) {
			return true
		}
	}
	return false
}

func (p *PriorityScorer) detectUploadPath(path string) bool {
	uploadPatterns := []string{
		"/upload/", "/uploads/",
		"/files/", "/documents/",
		"/media/", "/attachments/",
	}

	for _, pattern := range uploadPatterns {
		if strings.Contains(path, pattern) {
			return true
		}
	}
	return false
}

func (p *PriorityScorer) detectCommonAssetDir(path string) bool {
	assetDirs := []string{
		"/assets/", "/static/", "/public/",
		"/images/", "/img/", "/pics/",
		"/css/", "/styles/", "/stylesheets/",
		"/js/", "/javascript/", "/scripts/",
		"/fonts/", "/font/",
		"/icons/", "/icon/",
	}

	for _, dir := range assetDirs {
		if strings.Contains(path, dir) {
			return true
		}
	}
	return false
}

func (p *PriorityScorer) isTrackingParam(param string) bool {
	trackingParams := []string{
		"utm_source", "utm_medium", "utm_campaign", "utm_term", "utm_content",
		"gclid", "gclsrc", "fbclid",
		"_ga", "_gid", "_gac",
		"fb_action_ids", "fb_action_types", "fb_source", "fb_ref",
		"mc_cid", "mc_eid", // Mailchimp
		"ref", "referer", "referrer",
	}

	for _, tracking := range trackingParams {
		if param == tracking {
			return true
		}
	}
	return false
}

func (p *PriorityScorer) isPaginationParam(param string) bool {
	paginationParams := []string{
		"page", "p", "pg", "pagenum",
		"offset", "start", "from",
		"limit", "count", "per_page", "perpage",
		"skip", "take",
	}

	for _, pagination := range paginationParams {
		if param == pagination {
			return true
		}
	}
	return false
}

func (p *PriorityScorer) isStaticAsset(ext string) bool {
	staticExts := map[string]bool{
		// Images
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
		".svg": true, ".webp": true, ".ico": true, ".bmp": true,
		".tiff": true, ".tif": true,

		// Stylesheets
		".css": true, ".scss": true, ".sass": true, ".less": true,

		// Fonts
		".woff": true, ".woff2": true, ".ttf": true, ".eot": true,
		".otf": true,

		// Media
		".mp4": true, ".webm": true, ".ogg": true, ".avi": true,
		".mov": true, ".wmv": true, ".flv": true,
		".mp3": true, ".wav": true, ".flac": true,

		// Documents (common)
		".pdf": true, ".doc": true, ".docx": true,
		".xls": true, ".xlsx": true,
		".ppt": true, ".pptx": true,

		// Archives
		".zip": true, ".tar": true, ".gz": true, ".bz2": true,
		".rar": true, ".7z": true,
	}

	return staticExts[ext]
}

// ScoreBatch scores multiple URLs and sorts by priority (descending).
func (p *PriorityScorer) ScoreBatch(urls []string) []ScoredURL {
	scored := make([]ScoredURL, 0, len(urls))

	for _, rawURL := range urls {
		scored = append(scored, p.Score(rawURL))
	}

	// Sort by score (descending)
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})

	return scored
}
