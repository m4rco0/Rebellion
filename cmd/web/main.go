package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/m4rco0/Rebellion/internal/database"
	"github.com/m4rco0/Rebellion/internal/web"
)

func main() {
	log.SetFlags(log.LstdFlags)

	// Carrega o .env antes de ler qualquer variável (WEB_ADDR, WEB_USER, DB_*).
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("aviso: não foi possível carregar o .env: %v", err)
	}

	addr := flag.String("addr", envOrDefault("WEB_ADDR", "127.0.0.1:8080"), "endereço HTTP de escuta (ex.: 127.0.0.1:8080)")
	flag.Parse()

	dsn, err := database.DSNFromEnv()
	if err != nil {
		log.Fatalf("configuração do banco inválida: %v", err)
	}

	db, err := database.InitDB(dsn)
	if err != nil {
		log.Fatalf("inicializar banco: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("obter conexão com o banco: %v", err)
	}
	defer sqlDB.Close()

	pingCtx, cancelPing := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelPing()
	if err := sqlDB.PingContext(pingCtx); err != nil {
		log.Fatalf("conectar ao banco: %v", err)
	}

	server, err := web.New(db)
	if err != nil {
		log.Fatalf("montar interface web: %v", err)
	}

	handler := web.BasicAuthAllowHealth(os.Getenv("WEB_USER"), os.Getenv("WEB_PASSW"), server.Handler())

	httpServer := &http.Server{
		Addr:              *addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       90 * time.Second,
	}

	stopCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("Rebellion web disponível em http://%s", displayAddr(*addr))
		if os.Getenv("WEB_USER") == "" {
			log.Printf("autenticação desativada (defina WEB_USER e WEB_PASSW para proteger o painel)")
		}
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("servidor HTTP: %v", err)
		}
	}()

	<-stopCtx.Done()
	log.Println("encerrando servidor...")

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("encerramento: %v", err)
	}
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func displayAddr(addr string) string {
	if strings.HasPrefix(addr, ":") {
		return "localhost" + addr
	}
	return addr
}
