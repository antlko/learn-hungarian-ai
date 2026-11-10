package api

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"

	"tanulom/internal/components"
	"tanulom/internal/config"
)

// Managed components — the binary installs and runs the mt and nlp sidecars
// itself (ADR-061). Settings → Components is the only entry point.
//
// Every route is admin-only and takes nothing but an id from the URL: no URL, no
// path, no argument ever reaches a command line. Install / start / stop /
// uninstall answer 202 and the card polls GET /api/admin/components, whose
// `managed` object is the state machine.

// ManagedDTO is `managed` on the mt and nlp rows of GET /api/admin/components.
type ManagedDTO struct {
	Available      bool     `json:"available"`
	UnavailableKey string   `json:"unavailableKey,omitempty"`
	PythonVersion  string   `json:"pythonVersion,omitempty"`
	State          string   `json:"state"`
	Step           string   `json:"step,omitempty"`
	Progress       float64  `json:"progress"`
	Port           int      `json:"port,omitempty"`
	ErrorKey       string   `json:"errorKey,omitempty"`
	ErrorDetail    string   `json:"errorDetail,omitempty"`
	Log            []string `json:"log,omitempty"`
	DiskNeed       string   `json:"diskNeed"`
	RAMNeed        string   `json:"ramNeed"`
}

func managedDTO(st components.Status) *ManagedDTO {
	return &ManagedDTO{
		Available: st.Available, UnavailableKey: st.UnavailableKey, PythonVersion: st.PythonVersion,
		State: string(st.State), Step: st.Step, Progress: st.Progress, Port: st.Port,
		ErrorKey: st.ErrorKey, ErrorDetail: st.ErrorDetail, Log: st.Log,
		DiskNeed: st.DiskNeed, RAMNeed: st.RAMNeed,
	}
}

// newComponents builds the manager and its two hooks. The hooks are the only
// place the manager reaches back into the server.
func (s *Server) newComponents(mods ...func(*components.Options)) *components.Manager {
	o := components.Options{
		DataDir: s.dataDir,
		// A finished install selects itself: provider=managed, through the one
		// sanctioned config path. No endpoint is stored — see resolveService.
		OnInstalled: func(id string) error {
			_, err := s.updateConfig(func(cfg *config.Config) {
				if f := serviceField(cfg, id); f != nil {
					*f = config.Service{Provider: config.ProviderManaged}
				}
			})
			s.invalidateCapabilityCache()
			return err
		},
		// Ports appear and vanish; never show a 30 s old probe over them.
		OnChange: func(string) { s.invalidateCapabilityCache() },
	}
	for _, mod := range mods { // tests swap the runner, never the hooks
		mod(&o)
	}
	return components.New(o)
}

// resolveService is the ONE place a service's endpoint is decided. A managed
// service has no stored endpoint: while its child runs it resolves to
// `http://127.0.0.1:<port>` as an ordinary `local` service, so every client
// (mt, nlp, the capability probe, the Components rows) goes through here and none
// of them learns about ports. Not running resolves to provider `managed` with no
// endpoint, which every client treats as "not configured right now".
func (s *Server) resolveService(name string, svc config.Service) config.Service {
	if svc.Provider != config.ProviderManaged {
		return svc
	}
	if ep := s.components.Endpoint(name); ep != "" {
		return config.Service{Provider: config.ProviderLocal, Endpoint: ep}
	}
	return config.Service{Provider: config.ProviderManaged}
}

// service returns the named service from the live config, endpoint resolved.
func (s *Server) service(name string) config.Service {
	cfg := s.Config()
	f := serviceField(&cfg, name)
	if f == nil {
		return config.Service{}
	}
	return s.resolveService(name, *f)
}

// reconcileManaged follows a provider change made in Settings: leaving
// `managed` stops the child (nothing else will, and it holds 2+ GB of RAM);
// choosing it starts an installed one.
func (s *Server) reconcileManaged(prev, next config.Config) {
	for _, id := range components.IDs() {
		a, b := serviceField(&prev, id).Provider, serviceField(&next, id).Provider
		switch {
		case a == config.ProviderManaged && b != config.ProviderManaged:
			if err := s.components.Stop(id); err != nil {
				slog.Warn("components: could not stop after provider change", "component", id, "error", err)
			}
		case a != config.ProviderManaged && b == config.ProviderManaged:
			if err := s.components.Start(id); err != nil && !errors.Is(err, components.ErrNotInstalled) {
				slog.Warn("components: could not start after provider change", "component", id, "error", err)
			}
		}
	}
}

// autoStartComponents runs at boot, off the boot path: a component that will not
// start is logged and the app carries on without it.
func (s *Server) autoStartComponents() {
	s.bg.Add(1)
	go func() {
		defer s.bg.Done()
		s.components.AutoStart(func(id string) bool {
			cfg := s.Config()
			return serviceField(&cfg, id).Provider == config.ProviderManaged
		})
	}()
}

// componentError maps a manager error to an HTTP error carrying an i18n key.
func componentError(err error) error {
	var ce *components.Error
	if !errors.As(err, &ce) {
		return err
	}
	code := fiber.StatusUnprocessableEntity
	switch ce.Key {
	case components.KeyUnknown:
		code = fiber.StatusNotFound
	case components.KeyBusy, components.KeyInstalled, components.KeyNotInstalled:
		code = fiber.StatusConflict
	case components.KeyDiskFull:
		code = fiber.StatusInsufficientStorage
	}
	return fiber.NewError(code, ce.Key)
}

func (s *Server) componentResponse(c fiber.Ctx, id string) error {
	return c.Status(fiber.StatusAccepted).JSON(managedDTO(s.components.Status(id)))
}

func (s *Server) handleComponentInstall(c fiber.Ctx) error {
	id := c.Params("id")
	if err := s.components.Install(c.Context(), id); err != nil {
		return componentError(err)
	}
	return s.componentResponse(c, id)
}

func (s *Server) handleComponentStart(c fiber.Ctx) error {
	id := c.Params("id")
	if err := s.components.Start(id); err != nil {
		return componentError(err)
	}
	return s.componentResponse(c, id)
}

func (s *Server) handleComponentStop(c fiber.Ctx) error {
	id := c.Params("id")
	if err := s.components.Stop(id); err != nil {
		return componentError(err)
	}
	return s.componentResponse(c, id)
}

// handleComponentUninstall deletes `<data>/components/<id>/` and, when the
// service was managed, switches it off — leaving provider=managed over nothing
// would be a setting that points at a deleted thing.
func (s *Server) handleComponentUninstall(c fiber.Ctx) error {
	id := c.Params("id")
	if err := s.components.Uninstall(id); err != nil {
		return componentError(err)
	}
	if cfg := s.Config(); serviceField(&cfg, id).Provider == config.ProviderManaged {
		if _, err := s.updateConfig(func(cfg *config.Config) {
			*serviceField(cfg, id) = config.Service{Provider: config.ProviderOff}
		}); err != nil {
			return err
		}
	}
	s.invalidateCapabilityCache()
	return s.componentResponse(c, id)
}

// shutdownComponents stops every child; bounded so a wedged one cannot hold
// the process hostage.
func (s *Server) shutdownComponents() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	s.components.Shutdown(ctx)
}
