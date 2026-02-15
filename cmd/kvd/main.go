package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/shinerio/gopher-kv/internal/config"
	"github.com/shinerio/gopher-kv/internal/core"
	"github.com/shinerio/gopher-kv/internal/server"
	"github.com/shinerio/gopher-kv/internal/storage"
)

func main() {
	configPath := flag.String("config", "configs/config.yaml", "config path")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config failed: %v\n", err)
		os.Exit(1)
	}

	logger := newLogger(cfg.Log.Level)

	store := storage.NewEngine(storage.Options{
		ShardCount:   cfg.Storage.ShardCount,
		MaxKeySize:   cfg.Storage.MaxKeySize,
		MaxValueSize: cfg.Storage.MaxValueSize,
		MaxMemory:    cfg.Storage.MaxMemory,
	})

	var aof *storage.AOF
	if cfg.AOF.Enabled {
		aof = storage.NewAOF(cfg.AOF.FilePath, cfg.AOF.RewriteThreshold, logger)
		if err := aof.OpenAndReplay(store.Restore); err != nil {
			logger.Error("aof restore failed", "error", err)
			os.Exit(1)
		}
	}

	var rdb *storage.RDB
	if cfg.RDB.Enabled {
		rdb = storage.NewRDB(cfg.RDB.FilePath, logger)
		if aof == nil {
			if records, path, err := rdb.LoadLatest(); err == nil && len(records) > 0 {
				if err := store.Restore(records); err != nil {
					logger.Error("rdb restore failed", "path", path, "error", err)
					os.Exit(1)
				}
				logger.Info("rdb restored", "path", path, "records", len(records))
			}
		}
	}

	svc := core.NewService(cfg, store, aof, rdb)
	h := server.NewHTTPHandler(svc, logger)
	mux := http.NewServeMux()
	h.Register(mux)

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      withMiddleware(mux),
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	go func() {
		logger.Info("kvd started", "addr", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("http server failed", "error", err)
			os.Exit(1)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	logger.Info("shutdown signal received")

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Warn("http shutdown failed", "error", err)
	}
	if err := svc.Close(ctx); err != nil {
		logger.Warn("service close failed", "error", err)
	}
	logger.Info("kvd stopped")
}

func newLogger(level string) *slog.Logger {
	var lv slog.Level
	switch level {
	case "warn":
		lv = slog.LevelWarn
	case "error":
		lv = slog.LevelError
	default:
		lv = slog.LevelInfo
	}
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: lv}))
}

func withMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}
