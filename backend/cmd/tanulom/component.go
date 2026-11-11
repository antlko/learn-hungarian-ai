package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"tanulom/internal/components"
	"tanulom/internal/state"
)

const componentUsage = `usage: tanulom component run <mt> [--port N] [--host IP] [--data-dir DIR]

Installs (first start only) and runs one sidecar in the foreground, so a second
container from the same image can host it:

  command: ["component", "run", "mt"]

--host defaults to 0.0.0.0 and --port to the service's own (mt 5003).
The sidecar has no authentication: publish it on the compose network only.`

// componentArgs finds the "component" subcommand. A leading `--data-dir X` (the
// server's flag) is accepted in front of it and carried over.
func componentArgs(args []string) (rest []string, dataDir string, ok bool) {
	for len(args) > 0 {
		a := args[0]
		switch {
		case a == "component":
			return args[1:], dataDir, true
		case a == "--data-dir" || a == "-data-dir":
			if len(args) < 2 {
				return nil, "", false
			}
			dataDir, args = args[1], args[2:]
		case strings.HasPrefix(a, "--data-dir=") || strings.HasPrefix(a, "-data-dir="):
			dataDir, args = a[strings.Index(a, "=")+1:], args[1:]
		default:
			return nil, "", false
		}
	}
	return nil, "", false
}

// runComponent implements `tanulom component run <id>` and returns the exit code.
func runComponent(args []string, lead string) int {
	return runComponentTo(args, lead, os.Stdout, os.Stderr, nil)
}

// runComponentTo is runComponent with its seams: output streams and an optional
// Manager for tests (nil = production defaults on the resolved data dir).
func runComponentTo(args []string, lead string, stdout, stderr io.Writer, mk func(dir string) *components.Manager) int {
	if len(args) == 0 || args[0] != "run" {
		fmt.Fprintln(stderr, componentUsage)
		return 2
	}
	args = args[1:]
	// The id is the one positional; flags may come before or after it.
	var id string
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		id, args = args[0], args[1:]
	}
	fs := flag.NewFlagSet("component run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var host, dataDir string
	var port int
	fs.StringVar(&host, "host", "", "bind address: 0.0.0.0 (default) or 127.0.0.1")
	fs.IntVar(&port, "port", 0, "port, 1024-65535 (default: the service's own)")
	fs.StringVar(&dataDir, "data-dir", lead, "data directory (default: $TANULOM_DATA_DIR or ./data)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if id == "" && fs.NArg() == 1 {
		id = fs.Arg(0)
	} else if fs.NArg() != 0 {
		fmt.Fprintln(stderr, componentUsage)
		return 2
	}
	host, port, err := components.ValidateRun(id, host, port)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n\n%s\n", err, componentUsage)
		return 2
	}

	dir := dataDir
	if dir == "" {
		dir = os.Getenv("TANULOM_DATA_DIR")
	}
	if dir == "" {
		dir = state.Load().DataDir // read-only: a sidecar never rewrites the app's saved choice
	}
	if dir == "" {
		dir = "./data"
	}

	out := func(l string) { fmt.Fprintln(stdout, l) }
	var mgr *components.Manager
	if mk != nil {
		mgr = mk(dir)
	} else {
		mgr = components.New(components.Options{DataDir: dir, OnLog: func(id, l string) { out("[" + id + "] " + l) }})
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		mgr.Shutdown(ctx)
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	slog.Info("component run", "component", id, "host", host, "port", port, "data_dir", dir)
	code, err := mgr.RunForeground(ctx, id, components.RunOptions{Host: host, Port: port,
		Out: func(l string) { out("[" + id + "] " + l) }})
	if err != nil {
		var ce *components.Error
		if errors.As(err, &ce) {
			fmt.Fprintln(stderr, humanReason(ce))
		} else {
			fmt.Fprintln(stderr, err)
		}
		if errors.Is(err, components.ErrUsage) {
			return 2
		}
		return max(code, 1)
	}
	return code
}

// humanReason is the CLI's rendering of the keys the app's UI translates.
func humanReason(e *components.Error) string {
	var msg string
	switch e.Key {
	case components.KeyNoPythonSlim:
		msg = "This image has no Python. Use the :latest image (the same volume and data work)."
	case components.KeyNoPython:
		msg = "Python 3.10 or newer with venv and pip was not found."
	case components.KeyReadOnly:
		msg = "The data directory is not writable."
	case components.KeyDiskFull:
		msg = "Not enough free disk space."
	case components.KeyNetwork:
		msg = "The download failed: no network, or the host is unreachable."
	default:
		msg = "The component could not be installed or started (" + e.Key + ")."
	}
	if e.Detail != "" {
		msg += "\n" + e.Detail
	}
	return msg
}
