package database

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
)

// DSNFromEnv monta a string de conexão do PostgreSQL a partir das variáveis
// DB_* do .env, aplicando defaults para host, porta, usuário e banco.
//
// Usa o formato URL (postgres://...) em vez de chave=valor para lidar
// corretamente com senhas que contêm caracteres especiais e para não quebrar
// quando alguma variável está vazia.
func DSNFromEnv() (string, error) {
	host := envOrDefault("DB_HOST", "localhost")
	port := envOrDefault("DB_PORT", "5432")
	user := envOrDefault("DB_USER", "root")
	password := os.Getenv("DB_PASSW")
	name := envOrDefault("DB_NAME", "postgres")
	sslmode := envOrDefault("DB_SSLMODE", "disable")

	if _, err := strconv.ParseUint(port, 10, 16); err != nil {
		return "", fmt.Errorf("DB_PORT inválida (%q): %w", port, err)
	}

	dsn := &url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(user, password),
		Host:     net.JoinHostPort(host, port),
		Path:     "/" + name,
		RawQuery: url.Values{"sslmode": {sslmode}}.Encode(),
	}

	return dsn.String(), nil
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
