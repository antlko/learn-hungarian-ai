// Command tanulom is the single-binary server: it serves the embedded Vue SPA and
// the JSON API, and owns the learner database and language pack.
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"

	"tanulom/internal/api"
	"tanulom/internal/config"
	"tanulom/internal/logger"
	"tanulom/internal/state"
)

func main() {
	logger.Init()

	var (
		addr        string
		dataDir     string
		showVersion bool
	)
	flag.StringVar(&addr, "addr", envOr("TANULOM_ADDR", config.DefaultAddr), "HTTP listen address")
	flag.StringVar(&dataDir, "data-dir", "", "data directory (default: $TANULOM_DATA_DIR or ./data)")
	flag.BoolVar(&showVersion, "version", false, "print version and exit")
	flag.Parse()

	if showVersion {
		os.Stdout.WriteString("tanulom " + api.Version + "\n")
		return
	}

	dir := resolveDataDir(dataDir)
	slog.Info("starting tanulom", "version", api.Version, "data_dir", dir, "addr", addr)

	srv, err := api.New(dir)
	if err != nil {
		slog.Error("startup failed", "error", err)
		os.Exit(1)
	}
	defer srv.Close()

	app := srv.App()
	go func() {
		if err := app.Listen(addr, fiber.ListenConfig{DisableStartupMessage: true}); err != nil {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()
	slog.Info("listening", "addr", addr)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		slog.Error("shutdown error", "error", err)
	}
}

// resolveDataDir picks the data directory (flag > env > saved state > default) and
// persists an explicit choice, so a restart without the flag stays consistent.
func resolveDataDir(flagDir string) string {
	if flagDir != "" {
		_ = state.Save(state.State{DataDir: flagDir})
		return flagDir
	}
	if env := os.Getenv("TANULOM_DATA_DIR"); env != "" {
		_ = state.Save(state.State{DataDir: env})
		return env
	}
	if st := state.Load(); st.DataDir != "" {
		return st.DataDir
	}
	return "./data"
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
