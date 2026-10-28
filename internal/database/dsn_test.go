package database

import (
	neturl "net/url"
	"strings"
	"testing"
)

func TestDSNFromEnv(t *testing.T) {
	t.Setenv("DB_HOST", "db.local")
	t.Setenv("DB_PORT", "6543")
	t.Setenv("DB_USER", "admin")
	t.Setenv("DB_PASSW", "s3nh@/com:caracteres")
	t.Setenv("DB_NAME", "rebellion")
	t.Setenv("DB_SSLMODE", "require")

	dsn, err := DSNFromEnv()
	if err != nil {
		t.Fatalf("DSNFromEnv retornou erro: %v", err)
	}

	parsed, err := neturl.Parse(dsn)
	if err != nil {
		t.Fatalf("DSN gerado não é uma URL válida (%q): %v", dsn, err)
	}

	if parsed.Scheme != "postgres" {
		t.Errorf("scheme esperado postgres, obteve %q", parsed.Scheme)
	}
	if parsed.Host != "db.local:6543" {
		t.Errorf("host esperado db.local:6543, obteve %q", parsed.Host)
	}
	if parsed.User.Username() != "admin" {
		t.Errorf("usuário esperado admin, obteve %q", parsed.User.Username())
	}
	if password, _ := parsed.User.Password(); password != "s3nh@/com:caracteres" {
		t.Errorf("senha não foi preservada: %q", password)
	}
	if parsed.Path != "/rebellion" {
		t.Errorf("banco esperado /rebellion, obteve %q", parsed.Path)
	}
	if parsed.Query().Get("sslmode") != "require" {
		t.Errorf("sslmode esperado require, obteve %q", parsed.Query().Get("sslmode"))
	}
}

func TestDSNFromEnvDefaults(t *testing.T) {
	for _, key := range []string{"DB_HOST", "DB_PORT", "DB_USER", "DB_NAME", "DB_SSLMODE"} {
		t.Setenv(key, "")
	}
	t.Setenv("DB_PASSW", "")

	dsn, err := DSNFromEnv()
	if err != nil {
		t.Fatalf("DSNFromEnv retornou erro: %v", err)
	}

	expectedParts := []string{"postgres://", "@localhost:5432/", "sslmode=disable"}
	for _, part := range expectedParts {
		if !strings.Contains(dsn, part) {
			t.Errorf("DSN %q deveria conter %q", dsn, part)
		}
	}
}

func TestDSNFromEnvInvalidPort(t *testing.T) {
	t.Setenv("DB_PORT", "abc")

	if _, err := DSNFromEnv(); err == nil {
		t.Fatal("esperado erro para DB_PORT inválida")
	}
}
