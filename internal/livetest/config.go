package livetest

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config holds the runtime configuration for the live test suite.
type Config struct {
	Token       string `json:"token"`
	BaseURL     string `json:"base_url"`
	TestUserID  int64  `json:"test_user_id"`
	TestChatID  int64  `json:"test_chat_id"`
	Interactive bool   `json:"interactive"`
	Cleanup     bool   `json:"cleanup"`
	RenderHTML  bool   `json:"render_html"`
	ReportDir   string `json:"report_dir"`
}

// DefaultConfig returns reasonable defaults.
func DefaultConfig() *Config {
	return &Config{
		BaseURL:     "https://platform-api2.max.ru",
		Interactive: false, // Default off in non-interactive / automated environments
		Cleanup:     true,  // Guaranteed cleanup by default
		RenderHTML:  true,  // Generate HTML transcript
		ReportDir:   "reports",
	}
}

// LoadConfig attempts to parse .env from the working directory or ancestors,
// then layers on environment variables.
func LoadConfig() *Config {
	cfg := DefaultConfig()

	// Try loading .env and determine project root
	rootDir := loadDotEnv()
	if rootDir != "" {
		cfg.ReportDir = filepath.Join(rootDir, "reports")
	}

	if token := os.Getenv("MAX_BOT_TOKEN"); token != "" {
		cfg.Token = token
	}
	if baseURL := os.Getenv("MAX_API_BASE_URL"); baseURL != "" {
		cfg.BaseURL = baseURL
	}
	if uidStr := os.Getenv("TEST_USER_ID"); uidStr != "" {
		if uid, err := strconv.ParseInt(uidStr, 10, 64); err == nil {
			cfg.TestUserID = uid
		}
	}
	if cidStr := os.Getenv("TEST_CHAT_ID"); cidStr != "" {
		if cid, err := strconv.ParseInt(cidStr, 10, 64); err == nil {
			cfg.TestChatID = cid
		}
	}
	if v := os.Getenv("LIVETEST_INTERACTIVE"); v != "" {
		cfg.Interactive = strings.ToLower(v) == "true" || v == "1"
	}
	if v := os.Getenv("LIVETEST_CLEANUP"); v != "" {
		cfg.Cleanup = strings.ToLower(v) == "true" || v == "1"
	}
	if v := os.Getenv("LIVETEST_RENDER_HTML"); v != "" {
		cfg.RenderHTML = strings.ToLower(v) == "true" || v == "1"
	}
	if v := os.Getenv("LIVETEST_REPORT_DIR"); v != "" {
		cfg.ReportDir = v
	}

	return cfg
}

func loadDotEnv() string {
	// Look up to 4 directory levels up for .env
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}

	for i := 0; i < 4; i++ {
		envPath := filepath.Join(dir, ".env")
		if info, err := os.Stat(envPath); err == nil && !info.IsDir() {
			parseEnvFile(envPath)
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

func parseEnvFile(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			k := strings.TrimSpace(parts[0])
			v := strings.TrimSpace(parts[1])
			v = strings.Trim(v, `"'`)
			if os.Getenv(k) == "" {
				os.Setenv(k, v)
			}
		}
	}
}
