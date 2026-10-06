package config

import "testing"

func TestFromEnvRequired(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "")
	t.Setenv("OPENROUTER_API_KEY", "k")
	t.Setenv("AI_MODEL", "m")
	t.Setenv("BOT_BUILDER_URL", "https://example.com")
	if _, err := FromEnv(); err == nil {
		t.Fatal("ожидалась ошибка без токена бота")
	}
}

func TestFromEnvDefaultsAndAllowlist(t *testing.T) {
	t.Setenv("TELEGRAM_BOT_TOKEN", "tg")
	t.Setenv("OPENROUTER_API_KEY", "or")
	t.Setenv("AI_MODEL", "openai/gpt-4o-mini")
	t.Setenv("BOT_BUILDER_URL", "https://example.com/")
	t.Setenv("AI_MAX_TOKENS", "")
	t.Setenv("AI_TEMPERATURE", "")
	t.Setenv("AGENT_MAX_ROUNDS", "")
	t.Setenv("AGENT_MAX_RESULT_CHARS", "")
	t.Setenv("AGENT_HISTORY_MESSAGES", "")
	t.Setenv("TELEGRAM_ALLOWED_IDS", "")
	t.Setenv("DATABASE_URL", "")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BuilderURL != "https://example.com/mcp" {
		t.Fatalf("url: %s", cfg.BuilderURL)
	}
	if cfg.AIMaxTokens != 4000 || cfg.MaxRounds != 16 || cfg.MaxResultChars != 12000 || cfg.HistoryMessages != 40 {
		t.Fatalf("дефолты: %+v", cfg)
	}
	if cfg.AITemperature != 0.2 {
		t.Fatalf("temperature: %v", cfg.AITemperature)
	}
	if cfg.DatabaseURL != "sqlite://data/agent.db" {
		t.Fatalf("db: %s", cfg.DatabaseURL)
	}
	if !cfg.Allowed(42) {
		t.Fatal("пустой allowlist должен пускать всех")
	}
}

func TestAllowlist(t *testing.T) {
	base(t)
	t.Setenv("TELEGRAM_ALLOWED_IDS", "10, 20")
	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Allowed(10) || cfg.Allowed(11) {
		t.Fatalf("ids: %+v", cfg.AllowedIDs)
	}

	t.Setenv("TELEGRAM_ALLOWED_IDS", "1,nope")
	if _, err := FromEnv(); err == nil {
		t.Fatal("битый id должен давать ошибку")
	}
}

func TestDefaultBuilderURL(t *testing.T) {
	base(t)
	t.Setenv("BOT_BUILDER_URL", "")
	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BuilderURL != DefaultBuilderURL+"/mcp" {
		t.Fatalf("url: %s", cfg.BuilderURL)
	}
}

func TestBuilderURLRejectsScheme(t *testing.T) {
	base(t)
	t.Setenv("BOT_BUILDER_URL", "ftp://example.com")
	if _, err := FromEnv(); err == nil {
		t.Fatal("ждали ошибку схемы")
	}
}

func base(t *testing.T) {
	t.Helper()
	t.Setenv("TELEGRAM_BOT_TOKEN", "tg")
	t.Setenv("OPENROUTER_API_KEY", "or")
	t.Setenv("AI_MODEL", "m")
	t.Setenv("BOT_BUILDER_URL", "https://builder.example/mcp")
	t.Setenv("TELEGRAM_ALLOWED_IDS", "")
}
