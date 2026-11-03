package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"geoentity/backend/internal/auth"
	"geoentity/backend/internal/handler"
	"geoentity/backend/internal/maintenance"
	"geoentity/backend/internal/repository"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	addr := env("GEOENTITY_ADDR", "127.0.0.1:8080")
	dbPath := env("GEOENTITY_DB_PATH", "./data/geoentity.db")
	publicOrigin := env("GEOENTITY_PUBLIC_ORIGIN", "https://localhost:5173")
	insecureDev := os.Getenv("GEOENTITY_DEV_COOKIE_INSECURE") == "true"
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o750); err != nil {
		logger.Error("create database directory", "error", err)
		os.Exit(1)
	}
	apiLock, err := maintenance.Acquire(dbPath+".api.lock", true, true)
	if err != nil {
		logger.Error("another API process owns this database", "error", err)
		os.Exit(1)
	}
	defer maintenance.Release(apiLock)
	lock, err := maintenance.Acquire(dbPath+".lock", false, false)
	if err != nil {
		logger.Error("lock database", "error", err)
		os.Exit(1)
	}
	defer maintenance.Release(lock)

	startupCtx, cancelStartup := context.WithTimeout(context.Background(), 10*time.Second)
	store, err := repository.Open(startupCtx, dbPath)
	cancelStartup()
	if err != nil {
		logger.Error("initialize database", "error", err)
		os.Exit(1)
	}
	defer store.Close()
	secureCookie := !insecureDev
	if insecureDev && !isLoopbackAddress(addr) {
		logger.Error("insecure development cookies require loopback bind")
		os.Exit(1)
	}
	if secureCookie && !strings.HasPrefix(publicOrigin, "https://") {
		logger.Error("GEOENTITY_PUBLIC_ORIGIN must use HTTPS when secure cookies are enabled")
		os.Exit(1)
	}
	var trustedProxies []string
	for _, cidr := range strings.Split(os.Getenv("GEOENTITY_TRUSTED_PROXY_CIDRS"), ",") {
		if v := strings.TrimSpace(cidr); v != "" {
			trustedProxies = append(trustedProxies, v)
		}
	}
	if !isLoopbackAddress(addr) && len(trustedProxies) == 0 {
		logger.Error("non-loopback bind requires explicit trusted proxy CIDRs and a TLS-terminating proxy")
		os.Exit(1)
	}
	authService, err := auth.New(store.DB(), auth.Config{PublicOrigin: publicOrigin, CookieSecure: secureCookie, TrustedProxyCIDRs: trustedProxies})
	if err != nil {
		logger.Error("configure authentication", "error", err)
		os.Exit(1)
	}
	provisioned, err := authService.HasAdmin(context.Background())
	if err != nil || !provisioned {
		logger.Error("administrator is not provisioned; run geoentity admin provision")
		os.Exit(1)
	}

	api := handler.New(store, logger)
	api.SetTrustedProxyCIDRs(trustedProxies)
	server := &http.Server{
		Addr:              addr,
		Handler:           authService.Middleware(api),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("GeoEntity API listening", "address", addr, "database", dbPath)
		serverErrors <- server.ListenAndServe()
	}()

	signals, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("HTTP server stopped", "error", err)
			os.Exit(1)
		}
	case <-signals.Done():
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			logger.Error("shutdown HTTP server", "error", err)
		}
	}
}

func isLoopbackAddress(address string) bool {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
