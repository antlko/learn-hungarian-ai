package api

// Data-transfer objects. These are the wire contract; web-ui/src/types mirrors
// them in camelCase. Change both together, and update docs/API.md in the same
// change.
//
// Invariant: no DTO ever carries a secret value. Secrets are exposed only as a
// presence map (see SettingsDTO.Secrets).

// BootstrapDTO drives the first-run wizard. Unauthenticated.
type BootstrapDTO struct {
	NeedsSetup  bool     `json:"needsSetup"`
	PackPresent bool     `json:"packPresent"`
	PackVersion string   `json:"packVersion,omitempty"`
	Locales     []string `json:"locales"`
	Version     string   `json:"version"`
}

// UserDTO is a rendered account. Never carries the password hash.
type UserDTO struct {
	ID        int64  `json:"id"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	CreatedAt string `json:"createdAt"`
}

// MeDTO is the signed-in user plus the settings the UI needs at boot.
type MeDTO struct {
	User     UserDTO     `json:"user"`
	Settings SettingsDTO `json:"settings"`
}

// SetupRequest creates the first admin account.
type SetupRequest struct {
	Email     string `json:"email"`
	Password  string `json:"password"`
	UILocale  string `json:"uiLocale"`
	GlossLang string `json:"glossLang"`
}

// LoginRequest authenticates.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// SettingsDTO renders config.yaml for the UI.
//
// Secrets holds only presence flags — {"llmKey": true} means a key is stored,
// never what it is. Sending a blank or omitted secret on PUT preserves the
// stored value; sending a new value replaces it.
type SettingsDTO struct {
	Locale     LocaleDTO             `json:"locale"`
	Review     ReviewDTO             `json:"review"`
	Mining     MiningDTO             `json:"mining"`
	Drill      DrillDTO              `json:"drill"`
	HVPT       HVPTDTO               `json:"hvpt"`
	Curriculum CurriculumDTO         `json:"curriculum"`
	Services   map[string]ServiceDTO `json:"services"`
	Secrets    map[string]bool       `json:"secrets"`
}

type LocaleDTO struct {
	UI            string `json:"ui"`
	Gloss         string `json:"gloss"`
	GlossFallback bool   `json:"glossFallback"`
}

type ReviewDTO struct {
	DesiredRetention float64                 `json:"desiredRetention"`
	NewPerDay        int                     `json:"newPerDay"`
	MaxReviewsPerDay int                     `json:"maxReviewsPerDay"`
	DailyMinutes     int                     `json:"dailyMinutes"`
	Goal             string                  `json:"goal"`
	Directions       map[string]DirectionDTO `json:"directions"`
	BurySiblings     bool                    `json:"burySiblings"`
}

type DirectionDTO struct {
	Enabled bool    `json:"enabled"`
	Weight  float64 `json:"weight"`
}

type MiningDTO struct {
	KnownThreshold float64 `json:"knownThreshold"`
	FeatureWeight  float64 `json:"featureWeight"`
	TargetUnknown  float64 `json:"targetUnknown"`
	UnknownSigma   float64 `json:"unknownSigma"`
	MinCoverage    float64 `json:"minCoverage"`
	TargetLength   [2]int  `json:"targetLength"`
}

// DrillDTO exposes the blocked -> interleaved gate. See [R5] —
// .claude/research/05-interleaving.md. Surfaced because a learner who does not
// know why interleaving is being held back will assume the app is broken.
type DrillDTO struct {
	GateThreshold     float64 `json:"gateThreshold"`
	GateWindow        int     `json:"gateWindow"`
	MasteryThreshold  float64 `json:"masteryThreshold"`
	MasteryWindow     int     `json:"masteryWindow"`
	Hysteresis        float64 `json:"hysteresis"`
	DefiniteObjectPct int     `json:"definiteObjectPct"`
	DemonstrativePct  int     `json:"demonstrativePct"`
	HarmonyLatencyMs  int     `json:"harmonyLatencyMs"`
}

type HVPTDTO struct {
	MinTalkers       int     `json:"minTalkers"`
	TrialsPerSession int     `json:"trialsPerSession"`
	RateJitter       float64 `json:"rateJitter"`
}

type CurriculumDTO struct {
	FocusUnlockListeningHours int `json:"focusUnlockListeningHours"`
}

type ServiceDTO struct {
	Provider string   `json:"provider"`
	Endpoint string   `json:"endpoint"`
	Model    string   `json:"model,omitempty"`
	Voices   []string `json:"voices,omitempty"`
}

// UpdateSettingsRequest is a partial update. Nil fields are left unchanged.
type UpdateSettingsRequest struct {
	Locale     *LocaleDTO            `json:"locale,omitempty"`
	Review     *ReviewDTO            `json:"review,omitempty"`
	Mining     *MiningDTO            `json:"mining,omitempty"`
	Drill      *DrillDTO             `json:"drill,omitempty"`
	HVPT       *HVPTDTO              `json:"hvpt,omitempty"`
	Curriculum *CurriculumDTO        `json:"curriculum,omitempty"`
	Services   map[string]ServiceDTO `json:"services,omitempty"`
	Secrets    map[string]string     `json:"secrets,omitempty"` // blank value = keep stored
}

// UpdateSettingsResponse returns the saved settings plus any clamps applied, so
// a silent adjustment is never invisible to the user.
type UpdateSettingsResponse struct {
	Settings SettingsDTO `json:"settings"`
	Clamped  []ClampDTO  `json:"clamped,omitempty"`
}

type ClampDTO struct {
	Field string  `json:"field"`
	From  float64 `json:"from"`
	To    float64 `json:"to"`
}

// CapabilityDTO reports an optional service. The UI renders these as the
// capability dots and disables dependent features with a visible reason —
// never silently. See ADR-005.
type CapabilityDTO struct {
	Name       string `json:"name"`
	Provider   string `json:"provider"`
	Configured bool   `json:"configured"`
	Reachable  bool   `json:"reachable"`
	LatencyMs  int64  `json:"latencyMs,omitempty"`
	Detail     string `json:"detail,omitempty"`
}
