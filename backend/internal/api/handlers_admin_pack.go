package api

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"

	"tanulom/internal/lang"
	"tanulom/internal/mt"
)

// The full language pack, the restart that loads it, and the components
// overview — ADR-049 (amends ADR-010).
//
// Every route here is admin-only and none of them accepts a URL: where the pack
// comes from and what it must hash to are config.yaml (`langpack.url`,
// `langpack.sha256`), changed only through PUT /api/settings. A request body
// that could name a URL would turn an admin session into a server-side fetch.
//
// The pack is opened once at boot, so a finished install is "ready, restart
// required" — the app cannot swap a pack under running requests and does not
// pretend it can. Restart is the process exiting cleanly (SIGTERM → the same
// graceful shutdown as `docker stop`); whether something brings it back is the
// operator's supervisor, which the image declares with TANULOM_SUPERVISED=1.

// supervisedEnv is set to "1" by the Dockerfile: the image is meant to run
// under a restart policy, so exiting is a restart rather than an outage.
const supervisedEnv = "TANULOM_SUPERVISED"

// restartDelay lets the 202 reach the client before the process stops.
const restartDelay = 400 * time.Millisecond

// signalSelfTerm asks this process to shut down the way `docker stop` does.
func signalSelfTerm() {
	p, err := os.FindProcess(os.Getpid())
	if err == nil {
		err = p.Signal(syscall.SIGTERM)
	}
	if err != nil {
		slog.Error("could not signal this process to stop", "error", err)
	}
}

// PackInstallStatusDTO is GET /api/admin/pack/install.
type PackInstallStatusDTO struct {
	State           string `json:"state"` // idle | downloading | verifying | ready | failed
	Bytes           int64  `json:"bytes"`
	Total           int64  `json:"total"`
	ErrorKey        string `json:"errorKey"`
	RestartRequired bool   `json:"restartRequired"`
}

// RestartResponse is POST /api/admin/restart.
type RestartResponse struct {
	// Supervised is false when nothing is known to bring the process back, and
	// the screen then says "start it again yourself".
	Supervised bool `json:"supervised"`
}

// ComponentDTO is one row of GET /api/admin/components.
type ComponentDTO struct {
	ID             string  `json:"id"` // pack | mt | nlp | tts | asr | llm
	Configured     bool    `json:"configured"`
	Reachable      bool    `json:"reachable"`
	State          string  `json:"state"`
	Progress       float64 `json:"progress"` // 0..1; 0 when nothing is in flight
	Model          string  `json:"model"`
	RAMHint        string  `json:"ramHint"`
	ComposeProfile string  `json:"composeProfile"`
	// Built is false for a sidecar this repository does not yet ship.
	Built bool `json:"built"`
	// Managed is present on mt and nlp: the binary can install and run them itself.
	Managed *ManagedDTO `json:"managed,omitempty"`
}

// componentTable is the static part of each row. Nothing here is measured at
// runtime; ramHint is only ever a figure someone documented, and blank where
// nobody did.
var componentTable = []struct {
	id, profile, ramHint string
	built                bool
}{
	{"pack", "", "", true},
	{"mt", "mt", "≈2.4 GB model download (RAM not measured)", true},
	{"nlp", "nlp", "≈2.4 GB (HuSpaCy hu_core_news_md, not measured in a container)", true},
	{"tts", "speech", "", false},
	{"asr", "speech", "", false},
	{"llm", "llm", "", true},
}

func statusDTO(st lang.InstallStatus) PackInstallStatusDTO {
	return PackInstallStatusDTO{State: st.State, Bytes: st.Bytes, Total: st.Total,
		ErrorKey: st.ErrorKey, RestartRequired: st.RestartRequired}
}

// handlePackInstallStart begins the download. 202, 409 while one runs, 422
// when nothing is published to download.
func (s *Server) handlePackInstallStart(c fiber.Ctx) error {
	cfg := s.Config()
	err := s.packInstaller.Start(s.bgCtx, lang.InstallSpec{
		URL:    cfg.LangPack.URL,
		SHA256: cfg.LangPack.SHA256,
		Compat: func(ctx context.Context, next *lang.Pack) error {
			return lang.CheckIDCompat(ctx, s.db.DB, s.pack, next)
		},
	})
	switch {
	case errors.Is(err, lang.ErrBusy):
		return fiber.NewError(fiber.StatusConflict, "errors.packInstallBusy")
	case errors.Is(err, lang.ErrNotPublished):
		return fiber.NewError(fiber.StatusUnprocessableEntity, lang.ErrKeyNotPublished)
	case err != nil:
		return err
	}
	return c.Status(fiber.StatusAccepted).JSON(statusDTO(s.packInstaller.Status()))
}

func (s *Server) handlePackInstallStatus(c fiber.Ctx) error {
	return c.JSON(statusDTO(s.packInstaller.Status()))
}

// handleAdminRestart stops the process gracefully after answering 202.
func (s *Server) handleAdminRestart(c fiber.Ctx) error {
	switch s.packInstaller.Status().State {
	case lang.InstallDownloading, lang.InstallVerifying:
		return fiber.NewError(fiber.StatusConflict, "errors.packInstallBusy")
	}
	supervised := os.Getenv(supervisedEnv) == "1"
	stop := s.stopProcess
	time.AfterFunc(restartDelay, func() {
		// An install may have been started in the delay window; stopping now
		// would throw its download away.
		switch s.packInstaller.Status().State {
		case lang.InstallDownloading, lang.InstallVerifying:
			slog.Warn("restart skipped: a pack install started after it was requested")
			return
		}
		slog.Info("restart requested by an administrator", "supervised", supervised)
		stop()
	})
	return c.Status(fiber.StatusAccepted).JSON(RestartResponse{Supervised: supervised})
}

// handleAdminComponents lists the pack and the optional sidecars in one shape.
// Reachability comes from the capability probe the rest of the app uses — the
// two screens cannot disagree — and mt's own state words are passed through.
func (s *Server) handleAdminComponents(c fiber.Ctx) error {
	caps := map[string]CapabilityDTO{}
	for _, d := range s.capabilities(c.Context()) {
		caps[d.Name] = d
	}
	cfg := s.Config()
	out := make([]ComponentDTO, 0, len(componentTable))
	for _, row := range componentTable {
		d := ComponentDTO{ID: row.id, ComposeProfile: row.profile, RAMHint: row.ramHint, Built: row.built}
		if row.id == "pack" {
			s.fillPackComponent(&d)
			out = append(out, d)
			continue
		}
		cp := caps[row.id]
		d.Configured, d.Reachable = cp.Configured, cp.Reachable
		switch {
		case !cp.Configured:
			d.State = "off"
		case cp.Provider == "device":
			d.State = "device"
		case cp.Reachable:
			d.State = "ready"
		default:
			d.State = "unreachable"
		}
		switch row.id {
		case "mt":
			d.Managed = managedDTO(s.components.Status("mt"))
			if client := mt.New(s.service("mt")); client != nil && cp.Configured {
				if st, err := client.Status(c.Context()); err == nil {
					d.Reachable, d.State, d.Model = true, st.State, st.Model
					if st.TotalBytes > 0 {
						d.Progress = min(1, float64(st.DownloadedBytes)/float64(st.TotalBytes))
					}
				}
			}
		case "nlp":
			d.Managed = managedDTO(s.components.Status("nlp"))
			s.fillNLPComponent(c.Context(), &d, s.service("nlp"), cp.Configured)
		case "llm":
			d.Model = cfg.Services.LLM.Model
		}
		out = append(out, d)
	}
	return c.JSON(out)
}

func (s *Server) fillPackComponent(d *ComponentDTO) {
	d.Configured = true
	d.Reachable = s.pack != nil
	st := s.packInstaller.Status()
	switch {
	case st.State == lang.InstallDownloading || st.State == lang.InstallVerifying:
		d.State = "installing"
		if st.Total > 0 {
			d.Progress = min(1, float64(st.Bytes)/float64(st.Total))
		}
	case st.State == lang.InstallReady && st.RestartRequired:
		d.State = "restartRequired"
	case s.pack == nil:
		d.State = "missing"
	default:
		d.State = s.pack.Meta.Edition // starter | full
	}
}
