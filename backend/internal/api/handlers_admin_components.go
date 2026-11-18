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

// Managed components — the binary installs and runs the mt sidecar
// itself (ADR-061). Settings → Components is the only entry point.
//
// Every route is admin-only and takes nothing but an id from the URL: no URL, no
// path, no argument ever reaches a command line. Install / start / stop /
// uninstall answer 202 and the card polls GET /api/admin/components, whose
// `managed` object is the state machine.

// ManagedDTO is `managed` on the mt row of GET /api/admin/components.
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
	// Voices (tts only): the Piper personas whose model files the server found,
	// in display order (anna, berta, imre). Absent for other components and when
	// none were found — the card then says so and install is refused.
	Voices []string `json:"voices,omitempty"`
	// Bundled (tts): the runtime ships with the binary and works offline
	// (ADR-080). The card then says Включить / Выключить instead of Установить,
	// and `unavailableKey` is errors.componentNoTtsRuntime when this platform or
	// build has no runtime (device speech remains the floor).
	Bundled bool `json:"bundled"`
}

func managedDTO(st components.Status) *ManagedDTO {
	return &ManagedDTO{
		Available: st.Available, UnavailableKey: st.UnavailableKey, PythonVersion: st.PythonVersion,
		State: string(st.State), Step: st.Step, Progress: st.Progress, Port: st.Port,
		ErrorKey: st.ErrorKey, ErrorDetail: st.ErrorDetail, Log: st.Log,
		DiskNeed: st.DiskNeed, RAMNeed: st.RAMNeed, Voices: st.Voices, Bundled: st.Bundled,
	}
}

// newComponents builds the manager and its two hooks. The hooks are the only
// place the manager reaches back into the server.
func (s *Server) newComponents(mods ...func(*components.Options)) *components.Manager {
	o := components.Options{
		DataDir: s.dataDir, VoicesDir: s.voicesDir, TTSRuntimeDir: s.ttsRuntimeDir,
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
// (mt, the capability probe, the Components rows) goes through here and none
// of them learns about ports. Not running resolves to provider `managed` with no
// endpoint, which every client treats as "not configured right now".
func (s *Server) resolveService(name string, svc config.Service) config.Service {
	if svc.Paused {
		return config.Service{Provider: config.ProviderOff, Paused: true} // ADR-068
	}
	if svc.Provider != config.ProviderManaged {
		return svc
	}
	if ep := s.components.Endpoint(name); ep != "" {
		return config.Service{Provider: config.ProviderLocal, Endpoint: ep}
	}
	return config.Service{Provider: config.ProviderManaged}
}

// managedWanted: the child should run only for a managed provider that is not paused.
func managedWanted(svc config.Service) bool {
	return svc.Provider == config.ProviderManaged && !svc.Paused
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
		a, b := managedWanted(*serviceField(&prev, id)), managedWanted(*serviceField(&next, id))
		switch {
		case a && !b:
			if err := s.components.Stop(id); err != nil {
				slog.Warn("components: could not stop after provider change", "component", id, "error", err)
			}
		case !a && b:
			if err := s.components.Start(id); err != nil && !errors.Is(err, components.ErrNotInstalled) {
				slog.Warn("components: could not start after provider change", "component", id, "error", err)
			}
		}
	}
}

// autoStartComponents runs at boot, off the boot path: a component that will not
// start is logged and the app carries on without it.
func (s *Server) autoStartComponents() {
	// Capture the manager here, on the boot path: reading s.components inside the
	// goroutine raced with a test (and any future caller) that swaps the field.
	comps := s.components
	s.bg.Add(1)
	go func() {
		defer s.bg.Done()
		comps.AutoStart(func(id string) bool {
			cfg := s.Config()
			return managedWanted(*serviceField(&cfg, id))
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
	if components.IsBundled(id) {
		// Enable: unpack when needed, start, hand over provider=managed.
		return s.handleComponentInstall(c)
	}
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
	if components.IsBundled(id) {
		// Disable: the child is gone (its memory with it) and the provider goes
		// back to the device voice, so a restart does not switch it on again.
		if err := s.resetManaged(id, config.ProviderDevice); err != nil {
			return err
		}
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
	after := config.ProviderOff
	if components.IsBundled(id) {
		after = config.ProviderDevice
	}
	if err := s.resetManaged(id, after); err != nil {
		return err
	}
	s.invalidateCapabilityCache()
	return s.componentResponse(c, id)
}

// resetManaged moves a service off provider=managed (and only then: a service
// the owner pointed elsewhere is left alone) to `to`.
func (s *Server) resetManaged(id, to string) error {
	cfg := s.Config()
	if serviceField(&cfg, id).Provider != config.ProviderManaged {
		return nil
	}
	_, err := s.updateConfig(func(cfg *config.Config) {
		*serviceField(cfg, id) = config.Service{Provider: to}
	})
	s.invalidateCapabilityCache()
	return err
}

// shutdownComponents stops every child; bounded so a wedged one cannot hold
// the process hostage.
func (s *Server) shutdownComponents() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	s.components.Shutdown(ctx)
}
