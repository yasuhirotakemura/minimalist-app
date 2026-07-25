// Package config は環境変数からapplication設定を読み込む。
//
// 設定項目は設計書 26.3 に対応する。
// secretはGitへcommitせず、環境変数または secret manager から供給する。
package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/YasuhiroTakemura/minimalist-app/apps/api/internal/platform/logging"
)

// APP_ENVの許容値。
const (
	EnvLocal      = "local"
	EnvStaging    = "staging"
	EnvProduction = "production"
)

// MinSecretLength は PASSWORD_PEPPER と CSRF_SECRET の最小長。
const MinSecretLength = 16

// Config はapplication設定。
type Config struct {
	AppEnv             string
	WebBaseURL         string
	APIPort            int
	DatabaseURL        string
	SessionCookieName  string
	SessionTTL         time.Duration
	PasswordPepper     string
	CSRFSecret         string
	CORSAllowedOrigins []string
	LogLevel           slog.Level
	MaxImportSizeMB    int
	ExportTTL          time.Duration
	MigrationsDir      string
	SeedsDir           string
}

// ValidationMode は必須とする設定項目の範囲を表す。
//
// migration jobはCSRF secretやpassword pepperを必要としない。
// deploy pipelineで不要なsecretの配布を強制しないよう、用途ごとに検証範囲を変える。
type ValidationMode int

const (
	// ValidateAll はAPI serverが必要とする全項目を検証する。
	ValidateAll ValidationMode = iota
	// ValidateDatabaseOnly はDB接続に必要な項目のみ検証する。migration用。
	ValidateDatabaseOnly
	// ValidateSeed はDB接続とpassword hash化に必要な項目を検証する。seed用。
	ValidateSeed
)

// Load はAPI server向けにConfigを構築し、全項目を検証する。
func Load() (Config, error) {
	return LoadWithMode(ValidateAll)
}

// LoadWithMode は環境変数からConfigを構築し、指定範囲で検証する。
func LoadWithMode(mode ValidationMode) (Config, error) {
	sessionTTLHours, err := lookupInt("SESSION_TTL_HOURS", 720)
	if err != nil {
		return Config{}, err
	}
	apiPort, err := lookupInt("API_PORT", 8081)
	if err != nil {
		return Config{}, err
	}
	maxImportSizeMB, err := lookupInt("MAX_IMPORT_SIZE_MB", 10)
	if err != nil {
		return Config{}, err
	}
	exportTTLMinutes, err := lookupInt("EXPORT_TTL_MINUTES", 15)
	if err != nil {
		return Config{}, err
	}

	config := Config{
		AppEnv:             lookupString("APP_ENV", EnvLocal),
		WebBaseURL:         lookupString("WEB_BASE_URL", "http://localhost:8080"),
		APIPort:            apiPort,
		DatabaseURL:        lookupString("DATABASE_URL", ""),
		SessionCookieName:  lookupString("SESSION_COOKIE_NAME", "less_session"),
		SessionTTL:         time.Duration(sessionTTLHours) * time.Hour,
		PasswordPepper:     lookupString("PASSWORD_PEPPER", ""),
		CSRFSecret:         lookupString("CSRF_SECRET", ""),
		CORSAllowedOrigins: splitAndTrim(lookupString("CORS_ALLOWED_ORIGINS", "")),
		LogLevel:           logging.ParseLevel(lookupString("LOG_LEVEL", "info")),
		MaxImportSizeMB:    maxImportSizeMB,
		ExportTTL:          time.Duration(exportTTLMinutes) * time.Minute,
		MigrationsDir:      lookupString("MIGRATIONS_DIR", "../../db/migrations"),
		SeedsDir:           lookupString("SEEDS_DIR", "../../db/seeds"),
	}

	if err := config.validate(mode); err != nil {
		return Config{}, err
	}
	return config, nil
}

func (c Config) validate(mode ValidationMode) error {
	var problems []string

	// 全modeで必須となる項目。
	switch c.AppEnv {
	case EnvLocal, EnvStaging, EnvProduction:
	default:
		problems = append(problems, fmt.Sprintf(
			"APP_ENV must be one of %s/%s/%s (got %q)", EnvLocal, EnvStaging, EnvProduction, c.AppEnv))
	}
	if c.DatabaseURL == "" {
		problems = append(problems, "DATABASE_URL is required")
	}

	if mode == ValidateAll || mode == ValidateSeed {
		if len(c.PasswordPepper) < MinSecretLength {
			problems = append(problems, fmt.Sprintf(
				"PASSWORD_PEPPER must be at least %d characters", MinSecretLength))
		}
	}

	if mode == ValidateAll {
		if c.SessionCookieName == "" {
			problems = append(problems, "SESSION_COOKIE_NAME is required")
		}
		if c.SessionTTL <= 0 {
			problems = append(problems, "SESSION_TTL_HOURS must be positive")
		}
		if len(c.CSRFSecret) < MinSecretLength {
			problems = append(problems, fmt.Sprintf(
				"CSRF_SECRET must be at least %d characters", MinSecretLength))
		}
		if c.APIPort <= 0 || c.APIPort > 65535 {
			problems = append(problems, "API_PORT must be between 1 and 65535")
		}
		if c.MaxImportSizeMB <= 0 {
			problems = append(problems, "MAX_IMPORT_SIZE_MB must be positive")
		}
		if c.ExportTTL <= 0 {
			problems = append(problems, "EXPORT_TTL_MINUTES must be positive")
		}
	}

	if len(problems) > 0 {
		return fmt.Errorf("invalid configuration: %s", strings.Join(problems, "; "))
	}
	return nil
}

// IsLocal はlocal開発環境かどうかを返す。
func (c Config) IsLocal() bool {
	return c.AppEnv == EnvLocal
}

// UseSecureCookies はCookieへSecure属性を付与すべきかを返す。
// local以外ではTLSを必須とする (設計書 18.2 / 24.1)。
func (c Config) UseSecureCookies() bool {
	return !c.IsLocal()
}

// ListenAddress はHTTP serverのlisten addressを返す。
func (c Config) ListenAddress() string {
	return ":" + strconv.Itoa(c.APIPort)
}

func lookupString(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return fallback
}

func lookupInt(key string, fallback int) (int, error) {
	raw := lookupString(key, "")
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid configuration: %s must be an integer (got %q): %w", key, raw, err)
	}
	return value, nil
}

func splitAndTrim(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			values = append(values, trimmed)
		}
	}
	if len(values) == 0 {
		return nil
	}
	return values
}
