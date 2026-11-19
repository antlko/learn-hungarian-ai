package api

// Data-transfer objects. These are the wire contract; web-ui/src/types mirrors
// them in camelCase. Change both together, and update docs/API.md in the same
// change.
//
// Invariant: no DTO ever carries a secret value. Secrets are exposed only as a
// presence map (see SettingsDTO.Secrets).

// BootstrapDTO drives the first-run wizard. Unauthenticated.
type BootstrapDTO struct {
	NeedsSetup  bool   `json:"needsSetup"`
	PackPresent bool   `json:"packPresent"`
	PackVersion string `json:"packVersion,omitempty"`
	// PackEdition is "starter" (the pack embedded in the binary) or "full".
	// The setup wizard and Settings both say which one is installed, and
	// [ADR-010] promises they can — a promise that needs the field to be here
	// rather than only in the pack.
	PackEdition string   `json:"packEdition,omitempty"`
	Locales     []string `json:"locales"`
	Version     string   `json:"version"`
	// MinPassword is auth.MinPasswordLength, so the setup and users screens
	// print the floor the server enforces rather than a copy of it.
	MinPassword int `json:"minPassword"`
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
	Curriculum CurriculumDTO         `json:"curriculum"`
	Services   map[string]ServiceDTO `json:"services"`
	// LLMProfiles are the saved tutor presets; LLMActive is the id the active
	// `services.llm` was last taken from, "" when it differs (ADR-082).
	LLMProfiles []LLMProfileDTO `json:"llmProfiles"`
	LLMActive   string          `json:"llmActive"`
	LangPack    LangPackDTO     `json:"langpack"`
	Secrets     map[string]bool `json:"secrets"`
}

// LangPackDTO is where the optional full pack is downloaded from. Admin-edited
// only (PUT /api/settings is admin-only); the install endpoint never takes a
// URL from its request. An empty sha256 means nothing is published. ADR-049.
type LangPackDTO struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
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
	AccentTolerance  bool                    `json:"accentTolerance"`
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

type CurriculumDTO struct {
	FocusUnlockListeningHours int `json:"focusUnlockListeningHours"`
	// ExamDate is the booked exam (ISO date), the [R14] countdown anchor.
	ExamDate string `json:"examDate"`
}

type ServiceDTO struct {
	Provider string   `json:"provider"`
	Endpoint string   `json:"endpoint"`
	Model    string   `json:"model,omitempty"`
	Voices   []string `json:"voices,omitempty"`
	// Paused is output only: PUT /api/settings never reads it (ADR-068).
	Paused bool `json:"paused,omitempty"`
}

// LLMProfileDTO is one saved tutor preset. No key: that is one global secret.
type LLMProfileDTO struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Provider string `json:"provider"`
	Endpoint string `json:"endpoint"`
	Model    string `json:"model,omitempty"`
}

// LLMProfileRequest is the body of POST/PUT /api/admin/llm-profiles.
type LLMProfileRequest struct {
	Name     string `json:"name"`
	Provider string `json:"provider"`
	Endpoint string `json:"endpoint"`
	Model    string `json:"model"`
}

// UpdateSettingsRequest is a partial update. Nil fields are left unchanged.
type UpdateSettingsRequest struct {
	Locale     *LocaleDTO            `json:"locale,omitempty"`
	Review     *ReviewDTO            `json:"review,omitempty"`
	Mining     *MiningDTO            `json:"mining,omitempty"`
	Drill      *DrillDTO             `json:"drill,omitempty"`
	Curriculum *CurriculumDTO        `json:"curriculum,omitempty"`
	Services   map[string]ServiceDTO `json:"services,omitempty"`
	LangPack   *LangPackDTO          `json:"langpack,omitempty"`
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

// TranslateRequest asks for a batch of sentences in one call. Batched because
// the reader's natural unit is a paragraph, and a model that takes a second per
// sentence must not be woken up eight times to answer one screen.
type TranslateRequest struct {
	Texts []string `json:"texts"`
	// Target is a UI locale ("ru" / "en"); blank means the configured gloss
	// language. The FLORES code the model wants is mapped in internal/mt.
	Target string `json:"target"`
}

// TranslationDTO is one machine translation.
//
// Model travels with the text on purpose. The learner is shown who wrote this
// line, every time, because it is the only line in the app that no human and no
// corpus stands behind — ADR-014. Cached says the answer came from the local
// cache, which is why it arrived with the sidecar stopped.
type TranslationDTO struct {
	Source string `json:"source"`
	Text   string `json:"text"`
	Model  string `json:"model,omitempty"`
	Cached bool   `json:"cached"`
}

// TeacherTranslateRequest asks the AI Teacher to translate one sentence.
type TeacherTranslateRequest struct {
	HU     string `json:"hu"`
	Target string `json:"target"` // "ru" | "en"
}

// SaveSentenceTranslationRequest keeps a translation as the sentence's own.
type SaveSentenceTranslationRequest struct {
	HU     string `json:"hu"`
	Target string `json:"target"`
	Text   string `json:"text"`
	Model  string `json:"model"`
}

// SentenceTranslationDTO is a translation with the model that wrote it: the
// Teacher's answer, or the kept line echoed back. Model is never empty.
type SentenceTranslationDTO struct {
	Text  string `json:"text"`
	Model string `json:"model"`
}

// OriginalSentenceDTO is the crib after a kept translation is reset: the
// translation the card would show without it. Fields are empty when there is none.
type OriginalSentenceDTO struct {
	SentenceRU    string `json:"sentenceRu"`
	SentenceEN    string `json:"sentenceEn"`
	SentenceModel string `json:"sentenceModel"`
}

type TranslateResponse struct {
	Translations []TranslationDTO `json:"translations"`
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
	// Reason says why a configured service could not be reached: one of
	// tlsToPlainHttp, refused, timeout, dns, tlsCert, unreachable. A code, never
	// the raw error, URL or address; absent when reachable or not attempted.
	Reason string `json:"reason,omitempty"`
	// Models is what an llm endpoint says it serves (GET /v1/models), for
	// the settings screen to offer; absent for every other service and for an
	// endpoint that does not answer the route.
	Models []string `json:"models,omitempty"`
	// Paused: the admin switched this service off (ADR-068). The row reports
	// configured=false, reachable=false, detail "paused"; provider is the stored one.
	Paused bool `json:"paused,omitempty"`
}

// LemmaMemory is the learner's memory of one word, for the word inspector.
// It rides on GET /api/lemma/:id as `memory`, and is absent when the word has
// no card in the learner's deck. Every figure is the BEST card's (highest
// retrievability, then reps — the Words list's rule), built by
// srs.Store.CardMemory; Progress is srs.Memory.Progress (ADR-070).
type LemmaMemory struct {
	Progress       int     `json:"progress"`
	Stage          string  `json:"stage"`
	NextDue        string  `json:"nextDue,omitempty"`
	IntervalDays   int     `json:"intervalDays"`
	Reps           int     `json:"reps"`
	Lapses         int     `json:"lapses"`
	Retrievability float64 `json:"retrievability"`
	// History is the best card's last outcomes, most recent LAST, at most 8.
	History []bool `json:"history"`
	// Cards lists every card of the word, one row per direction.
	Cards []LemmaCardMemory `json:"cards"`
}

// LemmaCardMemory is one card's place on the progress scale.
type LemmaCardMemory struct {
	Direction string `json:"direction"`
	Progress  int    `json:"progress"`
	Stage     string `json:"stage"`
}

// SpeechVoiceDTO is one Piper persona the learner can pick (ADR-079). Name and
// gender are chrome for the picker; they come from a static table, not the model.
type SpeechVoiceDTO struct {
	ID     string `json:"id"`     // anna | berta | imre
	Name   string `json:"name"`   // Anna | Berta | Imre
	Gender string `json:"gender"` // female | male
}

// SpeechVoicesDTO is GET /api/speech/voices. Unavailable is a normal answer,
// not an error: available=false, voices=[] and the client uses device speech.
type SpeechVoicesDTO struct {
	Available bool             `json:"available"`
	Voices    []SpeechVoiceDTO `json:"voices"`
}

// SpeechTTSRequest is POST /api/speech/tts.
type SpeechTTSRequest struct {
	Text  string `json:"text"`
	Voice string `json:"voice"`
	// Style is "sentence" (default when empty) or "word" (rendered as a slow
	// one-word sentence, because Piper renders bare words badly).
	Style string `json:"style"`
}

// SpeechTTSResponse is the rendering: a cache URL and its length in seconds.
type SpeechTTSResponse struct {
	URL       string  `json:"url"` // /api/speech/audio/<64 hex>.wav
	DurationS float64 `json:"durationS"`
}

// ── «Письмо» (Writing, ADR-085) ───────────────────────────────────────────
// Hungarian in a check (quote, fix, example, corrected) is model-written and
// display-only; nothing here is a card. The browser never handles text
// offsets: it renders `segments` by interpolation.

// WritingTopicDTO is one authored task from the embedded catalogue. Titles and
// briefs come in both locales; the client picks one. The word range is advice.
type WritingTopicDTO struct {
	Code     string `json:"code"`
	Level    string `json:"level"` // A1 | A2 | B1 | B2
	Genre    string `json:"genre"` // email | description | narrative | opinion
	TitleRU  string `json:"titleRu"`
	TitleEN  string `json:"titleEn"`
	BriefRU  string `json:"briefRu"`
	BriefEN  string `json:"briefEn"`
	MinWords int    `json:"minWords"`
	MaxWords int    `json:"maxWords"`
}

// WritingTopicsResponse is GET /api/writing/topics.
type WritingTopicsResponse struct {
	Topics []WritingTopicDTO `json:"topics"`
}

// WritingLastCheckDTO is the newest check on a piece, for the list.
type WritingLastCheckDTO struct {
	Source    string `json:"source"` // local | external
	Model     string `json:"model"`
	CreatedAt string `json:"createdAt"`
}

// WritingListItemDTO is one row of GET /api/writing (newest edit first).
type WritingListItemDTO struct {
	ID        int64                `json:"id"`
	Title     string               `json:"title"`
	TopicCode string               `json:"topicCode"`
	Level     string               `json:"level"`
	Words     int                  `json:"words"`
	UpdatedAt string               `json:"updatedAt"`
	Checks    int                  `json:"checks"`
	LastCheck *WritingLastCheckDTO `json:"lastCheck,omitempty"`
}

// WritingListResponse is GET /api/writing.
type WritingListResponse struct {
	Pieces []WritingListItemDTO `json:"pieces"`
}

// WritingCreateRequest is POST /api/writing. All fields optional; an empty
// topicCode is a free topic, an empty title becomes the topic's title.
type WritingCreateRequest struct {
	TopicCode string `json:"topicCode"`
	Level     string `json:"level"`
	Title     string `json:"title"`
}

// WritingUpdateRequest is PUT /api/writing/:id (autosave).
type WritingUpdateRequest struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

// WritingCheckSummaryDTO is a check without its text: the history list.
type WritingCheckSummaryDTO struct {
	ID          int64  `json:"id"`
	Source      string `json:"source"` // local | external
	Model       string `json:"model"`
	Lang        string `json:"lang"`
	CreatedAt   string `json:"createdAt"`
	Status      string `json:"status"` // ok | partial | failed
	Truncated   bool   `json:"truncated"`
	Interrupted bool   `json:"interrupted"`
	Level       string `json:"level"`      // CEFR level the model assigned, or ""
	ErrorCount  int    `json:"errorCount"` // parsed errors
}

// WritingPieceDTO is GET/PUT /api/writing/:id and the answer to create. Checks
// is the history newest first; Latest is the newest one in full (nil when none).
type WritingPieceDTO struct {
	ID        int64                    `json:"id"`
	Title     string                   `json:"title"`
	TopicCode string                   `json:"topicCode"`
	Level     string                   `json:"level"`
	Body      string                   `json:"body"`
	Words     int                      `json:"words"`
	CreatedAt string                   `json:"createdAt"`
	UpdatedAt string                   `json:"updatedAt"`
	Checks    []WritingCheckSummaryDTO `json:"checks"`
	Latest    *WritingCheckDTO         `json:"latest"`
}

// WritingPromptRequest is POST /api/writing/:id/prompt.
type WritingPromptRequest struct {
	Lang string `json:"lang"` // ru | en; anything else falls back to the UI locale
}

// WritingPromptResponse carries the ready-to-copy prompt. Text is the essay
// that was embedded in it; the client returns it with the pasted answer.
type WritingPromptResponse struct {
	Prompt        string `json:"prompt"`
	Text          string `json:"text"`
	PromptVersion int    `json:"promptVersion"`
}

// WritingExternalRequest is POST /api/writing/:id/checks/external.
type WritingExternalRequest struct {
	Lang  string `json:"lang"`
	Text  string `json:"text"`  // the essay the prompt embedded; empty = current body
	Raw   string `json:"raw"`   // the chat's answer, as pasted
	Model string `json:"model"` // free label ("ChatGPT"); empty = "external"
}

// WritingLocalRequest is POST /api/writing/:id/checks/local.
type WritingLocalRequest struct {
	Lang string `json:"lang"`
}

// WritingErrorDTO is one marked error. N is 1-based and is the number printed
// on the text. Unattested lists Hungarian words of FIX/EXAMPLE the dictionary
// does not carry. TopicCode/TopicTitle name a grammar lesson that exists in
// the installed pack (open it at /grammar?topic=<code>); omitted otherwise.
type WritingErrorDTO struct {
	N          int      `json:"n"`
	Quote      string   `json:"quote"`
	Fix        string   `json:"fix"`
	Type       string   `json:"type"` // closed list, see docs/API.md
	Hint       string   `json:"hint"`
	Why        string   `json:"why"`
	Rule       string   `json:"rule"`
	Example    string   `json:"example"`
	Anchored   bool     `json:"anchored"`
	Unattested []string `json:"unattested"`
	TopicCode  string   `json:"topicCode,omitempty"`
	TopicTitle string   `json:"topicTitle,omitempty"`
}

// WritingSegmentDTO is a piece of the checked text. Errors are 0-based indices
// into the check's errors array (error number = index + 1) of every error whose
// quote covers the segment; empty for plain text. Segment texts concatenate to
// the checked text.
type WritingSegmentDTO struct {
	Text   string `json:"text"`
	Errors []int  `json:"errors"`
}

// WritingCheckDTO is a whole check, rebuilt on read from the stored raw answer
// and the checked text.
type WritingCheckDTO struct {
	ID          int64               `json:"id"`
	Source      string              `json:"source"` // local | external
	Model       string              `json:"model"`
	Lang        string              `json:"lang"`
	CreatedAt   string              `json:"createdAt"`
	Status      string              `json:"status"` // ok | partial | failed
	Truncated   bool                `json:"truncated"`
	Interrupted bool                `json:"interrupted"`
	Dropped     int                 `json:"dropped"`
	Level       string              `json:"level"`
	Summary     string              `json:"summary"`
	Strengths   []string            `json:"strengths"`
	Vocab       []string            `json:"vocab"`
	Corrected   string              `json:"corrected"`
	Raw         string              `json:"raw"`
	Errors      []WritingErrorDTO   `json:"errors"`
	Segments    []WritingSegmentDTO `json:"segments"`
}
