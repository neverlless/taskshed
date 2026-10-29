package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/neverlless/taskshed/internal/api"
	"github.com/neverlless/taskshed/internal/database"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	var (
		port        = flag.String("port", getEnv("PORT", "8080"), "Port to run the server on")
		dbType      = flag.String("db-type", getEnv("DB_TYPE", "sqlite"), "Database type (sqlite or postgres)")
		dbPath      = flag.String("db-path", getEnv("DB_PATH", "./taskshed.db"), "SQLite database file")
		dbHost      = flag.String("db-host", getEnv("DB_HOST", "localhost"), "Database host")
		dbPort      = flag.String("db-port", getEnv("DB_PORT", "5432"), "Database port")
		dbUser      = flag.String("db-user", getEnv("DB_USER", "postgres"), "Database user")
		dbPassword  = flag.String("db-password", getEnv("DB_PASSWORD", ""), "Database password (prefer DB_PASSWORD)")
		dbName      = flag.String("db-name", getEnv("DB_NAME", "taskshed"), "Database name")
		dbSSLMode   = flag.String("db-sslmode", getEnv("DB_SSLMODE", "disable"), "PostgreSQL sslmode")
		logLevel    = flag.String("log-level", getEnv("LOG_LEVEL", "info"), "Log level: debug, info, warn, error")
		healthcheck = flag.Bool("healthcheck", false, "Probe /healthz of a running server on -port and exit 0 if healthy")
		showVersion = flag.Bool("version", false, "Print version and exit")
	)
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}
	if *healthcheck {
		os.Exit(probe(*port))
	}

	var level slog.Level
	levelErr := level.UnmarshalText([]byte(*logLevel))
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(log)
	if levelErr != nil {
		log.Warn("invalid log level, using info", "log_level", *logLevel)
	}

	var (
		store *database.Store
		err   error
	)
	switch *dbType {
	case "postgres":
		log.Info("opening database", "type", "postgres", "host", *dbHost, "port", *dbPort, "user", *dbUser, "name", *dbName, "sslmode", *dbSSLMode)
		store, err = database.OpenPostgres(database.PostgresConfig{
			Host: *dbHost, Port: *dbPort, User: *dbUser, Password: *dbPassword, DBName: *dbName, SSLMode: *dbSSLMode,
		})
	case "sqlite":
		log.Info("opening database", "type", "sqlite", "path", *dbPath)
		store, err = database.OpenSQLite(*dbPath)
	default:
		err = fmt.Errorf("unknown DB_TYPE %q (want sqlite or postgres)", *dbType)
	}
	if err != nil {
		log.Error("database init failed", "err", err)
		os.Exit(1)
	}
	if err := serve(log, store, *port, os.Getenv("API_TOKEN")); err != nil {
		log.Error("server failed", "err", err)
		os.Exit(1)
	}
}

func serve(log *slog.Logger, store *database.Store, port, token string) error {
	defer store.Close()
	if token == "" {
		log.Warn("API_TOKEN is not set: anyone who can reach the server can create, change and delete tasks")
	}

	srv := &http.Server{
		Addr:              net.JoinHostPort("", port),
		Handler:           api.NewServer(store, api.Options{APIToken: token, Logger: log}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	log.Info("server started", "version", version, "addr", srv.Addr, "auth", token != "")

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	if err := <-errc; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// probe is the Docker HEALTHCHECK for images without curl.
func probe(port string) int {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+net.JoinHostPort("127.0.0.1", port)+"/healthz", nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, "unhealthy:", resp.Status)
		return 1
	}
	return 0
}

func getEnv(key, defaultValue string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return defaultValue
}
