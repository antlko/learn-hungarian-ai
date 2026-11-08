package tutor

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"tanulom/internal/db"
)

// The thread store. A thread is a conversation (ADR-029): it has a title from
// its first question, may be pinned to one curriculum topic, and is kept so an
// explanation can be re-read after a reload and so a second question has the
// first in front of the model. Nothing here is read by any other package:
// there is no path from tutor_message to a note, a card or either state
// space, and that absence is the design (ADR-027).

// Store persists tutor threads.
type Store struct{ db *sql.DB }

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// ErrNoThread is returned for a thread that does not exist or belongs to
// another learner — the same answer for both, on purpose.
var ErrNoThread = errors.New("no such tutor thread")

// ErrNoMessage is returned for a turn that is not in the thread, or is not a
// question of the learner's own.
var ErrNoMessage = errors.New("no such tutor message")

// Thread is one conversation.
type Thread struct {
	ID     int64 `json:"id"`
	UserID int64 `json:"-"`
	// Title is the first question, cut to TitleLimit; blank until asked.
	Title string `json:"title"`
	// TopicCode pins every question in the thread to one lesson; blank
	// means the app infers a lesson per question.
	TopicCode string `json:"topic,omitempty"`
	// Kind is blank for an ordinary chat and KindReview for a review chat
	// (ADR-034): one per WORD per local day, found or created when the review
	// card first asks about that note.
	Kind string `json:"kind,omitempty"`
	// Word is the surface form an inspector chat is about, lowercased; blank
	// for every other kind. Migration 00021.
	Word string `json:"word,omitempty"`
	// NoteID is the note a review chat is about; zero for every other kind,
	// and for review threads created before migration 00019. A pointer the
	// tutor reads — nothing on the review path reads it back.
	NoteID    int64     `json:"noteId,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// KindReview marks a review chat — one per word per day.
const KindReview = "review"

// KindWord marks a chat about one word, opened from the word inspector.
//
// Separate from KindReview because it is keyed differently: a review chat
// hangs off `note_id` (the card being answered), while an inspector chat hangs
// off the FORM the learner clicked, which frequently has no note and sometimes
// no lemma — the words most worth asking about are the ones the pack lacks.
// Migration 00021.
const KindWord = "word"

// Message is one turn of a thread.
type Message struct {
	ID      int64  `json:"id"`
	Role    string `json:"role"`
	Content string `json:"content"`
	Lang    string `json:"lang,omitempty"`
	// Model names what wrote an assistant turn — the label the honesty rules
	// require, printed beside the text.
	Model         string `json:"model,omitempty"`
	PromptVersion int    `json:"promptVersion,omitempty"`
	// Topic is the lesson an assistant turn was grounded in — pinned or
	// inferred — and blank when the answer came from the reference alone.
	// The panel prints it; a choice the learner can disagree with is the
	// honesty of an inferred one. TopicTitle is resolved by the handler in
	// the answer language and never stored.
	Topic      string `json:"topic,omitempty"`
	TopicTitle string `json:"topicTitle,omitempty"`
	// Unattested lists the backticked Hungarian words the pack does not carry.
	Unattested []string `json:"unattested"`
	// Interrupted marks an answer whose stream was cut off — a timeout, a
	// stop, a lost connection or the answer budget; the text is partial and
	// the UI says so. CutReason says which: CutBudget, CutTimeout, CutStop or
	// CutConnection; blank on a complete answer (and on rows older than the
	// column, which the UI labels without a cause).
	Interrupted bool   `json:"interrupted"`
	CutReason   string `json:"cutReason,omitempty"`
	// CardID, CardPhase and Template say which review card a question was
	// about, in which phase, from which prepared template (ADR-034); zero
	// and blank for an ordinary question. Recorded on the tutor's side only —
	// nothing on the review path reads them.
	CardID    int64  `json:"cardId,omitempty"`
	CardPhase string `json:"cardPhase,omitempty"`
	Template  string `json:"template,omitempty"`
	// PromptTokens and AnswerTokens are what the server reported spending on
	// this answer, in its own tokens; ContextTokens is the window the model
	// was running with (an Ollama server says, others do not). Zero means
	// "not reported", and the UI says that instead of printing 0. Kept with
	// the answer and printed under the composer, because a prompt longer
	// than the window is cut from the head without a word (ADR-031).
	PromptTokens  int       `json:"promptTokens,omitempty"`
	AnswerTokens  int       `json:"answerTokens,omitempty"`
	ContextTokens int       `json:"contextTokens,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
}

// Why an answer stopped early, for the label under it (ADR-031).
const (
	CutBudget     = "budget"     // max_tokens ran out (llm.ErrTruncated)
	CutTimeout    = "timeout"    // the stream timeout
	CutStop       = "stop"       // the learner pressed stop
	CutConnection = "connection" // the model or the client went away
	CutEcho       = "echo"       // the model began reciting the prompt's own headings
)

const (
	// ThreadKeep is how many messages a thread retains; older ones are
	// pruned after each append. Long enough to scroll back a session, short
	// enough that a table nobody reads does not grow without bound.
	ThreadKeep = 200
	// TitleLimit is the length of a thread's title, in runes.
	TitleLimit = 60
)

// TitleFrom makes a thread title out of its first question: the first line,
// cut to TitleLimit runes.
func TitleFrom(question string) string {
	q := strings.TrimSpace(question)
	if i := strings.IndexAny(q, "\r\n"); i >= 0 {
		q = strings.TrimSpace(q[:i])
	}
	if utf8.RuneCountInString(q) <= TitleLimit {
		return q
	}
	r := []rune(q)[:TitleLimit]
	return strings.TrimRight(string(r), " ,;:") + "…"
}

// NewThread is what CreateThread opens: pinned to Topic when it is non-blank,
// of Kind (blank for an ordinary chat), about Note when it is a review chat,
// and named Title when the caller already knows the name — a review chat is
// named by its word at creation, an ordinary one by its first question.
type NewThread struct {
	Topic  string
	Kind   string
	Title  string
	NoteID int64
	Word   string
}

// CreateThread opens an empty thread.
func (s *Store) CreateThread(userID int64, n NewThread, now time.Time) (Thread, error) {
	stamp := db.FmtTime(now)
	res, err := s.db.Exec(`
		INSERT INTO tutor_thread (user_id, topic_code, kind, title, note_id, word, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?)`,
		userID, n.Topic, n.Kind, n.Title, n.NoteID, n.Word, stamp, stamp)
	if err != nil {
		return Thread{}, fmt.Errorf("create tutor thread: %w", err)
	}
	id, _ := res.LastInsertId()
	return Thread{
		ID: id, UserID: userID, Title: n.Title, TopicCode: n.Topic, Kind: n.Kind, NoteID: n.NoteID,
		CreatedAt: now, UpdatedAt: now,
	}, nil
}

// LatestThreadOfKind finds the learner's newest thread of a kind created at
// or after `since`. With a note id it is that WORD's review chat for the day;
// with zero, the newest review chat of the day whatever it is about — which
// is what a client older than migration 00019 asks for, and what threads
// created before it have.
func (s *Store) LatestThreadOfKind(userID int64, kind string, noteID int64, since time.Time) (Thread, bool, error) {
	q := `SELECT ` + threadCols + ` FROM tutor_thread
		 WHERE user_id = ? AND kind = ? AND created_at >= ?`
	args := []any{userID, kind, db.FmtTime(since)}
	if noteID > 0 {
		q += ` AND note_id = ?`
		args = append(args, noteID)
	}
	q += ` ORDER BY created_at DESC, id DESC LIMIT 1`
	t, err := scanThread(s.db.QueryRow(q, args...))
	if errors.Is(err, ErrNoThread) {
		return Thread{}, false, nil
	}
	return t, err == nil, err
}

// LatestWordThread finds today's chat about one form, if there is one.
//
// Mirrors LatestThreadOfKind's contract — one per key per local day, found or
// created — but keyed on the form rather than on a note id, for the reason
// KindWord exists.
func (s *Store) LatestWordThread(userID int64, word string, since time.Time) (Thread, bool, error) {
	t, err := scanThread(s.db.QueryRow(
		`SELECT `+threadCols+` FROM tutor_thread
		  WHERE user_id = ? AND kind = ? AND word = ? AND created_at >= ?
		  ORDER BY created_at DESC, id DESC LIMIT 1`,
		userID, KindWord, word, db.FmtTime(since)))
	if errors.Is(err, ErrNoThread) {
		return Thread{}, false, nil
	}
	return t, err == nil, err
}

func scanThread(row interface{ Scan(...any) error }) (Thread, error) {
	var t Thread
	var created, updated string
	if err := row.Scan(&t.ID, &t.UserID, &t.Title, &t.TopicCode, &t.Kind, &t.NoteID, &t.Word,
		&created, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Thread{}, ErrNoThread
		}
		return Thread{}, err
	}
	t.CreatedAt = db.ParseTime(sql.NullString{String: created, Valid: true})
	t.UpdatedAt = db.ParseTime(sql.NullString{String: updated, Valid: true})
	return t, nil
}

const threadCols = `id, user_id, title, topic_code, kind, note_id, COALESCE(word, ''), created_at, updated_at`

// GetThread loads one of the learner's threads.
func (s *Store) GetThread(userID, id int64) (Thread, error) {
	return scanThread(s.db.QueryRow(`
		SELECT `+threadCols+` FROM tutor_thread WHERE id = ? AND user_id = ?`, id, userID))
}

// LatestThreadForTopic finds the learner's most recently active thread pinned
// to a topic — what the panel under that lesson opens.
func (s *Store) LatestThreadForTopic(userID int64, topic string) (Thread, bool, error) {
	t, err := scanThread(s.db.QueryRow(`
		SELECT `+threadCols+` FROM tutor_thread
		 WHERE user_id = ? AND topic_code = ? AND topic_code != ''
		 ORDER BY updated_at DESC, id DESC LIMIT 1`, userID, topic))
	if errors.Is(err, ErrNoThread) {
		return Thread{}, false, nil
	}
	return t, err == nil, err
}

// Messages loads a thread's turns, oldest first.
func (s *Store) Messages(threadID int64) ([]Message, error) {
	rows, err := s.db.Query(`
		SELECT id, role, content, lang, model, prompt_version, topic_code, unattested, interrupted,
		       cut_reason, prompt_tokens, answer_tokens, context_tokens, card_id, card_phase, template, created_at
		  FROM tutor_message
		 WHERE thread_id = ?
		 ORDER BY id ASC`, threadID)
	if err != nil {
		return nil, fmt.Errorf("load tutor thread: %w", err)
	}
	defer rows.Close()
	out := []Message{}
	for rows.Next() {
		var m Message
		var unatt, created string
		var interrupted int
		if err := rows.Scan(&m.ID, &m.Role, &m.Content, &m.Lang, &m.Model, &m.PromptVersion,
			&m.Topic, &unatt, &interrupted, &m.CutReason, &m.PromptTokens, &m.AnswerTokens, &m.ContextTokens,
			&m.CardID, &m.CardPhase, &m.Template, &created); err != nil {
			return nil, err
		}
		m.Unattested = []string{}
		if unatt != "" {
			_ = json.Unmarshal([]byte(unatt), &m.Unattested)
		}
		m.Interrupted = interrupted == 1
		m.CreatedAt = db.ParseTime(sql.NullString{String: created, Valid: true})
		out = append(out, m)
	}
	return out, rows.Err()
}

// Append stores one turn, titles the thread from its first question, bumps
// its activity, and prunes it to ThreadKeep.
func (s *Store) Append(threadID int64, m Message, now time.Time) (int64, error) {
	unatt, err := json.Marshal(orEmpty(m.Unattested))
	if err != nil {
		return 0, err
	}
	interrupted := 0
	if m.Interrupted {
		interrupted = 1
	}
	var userID int64
	var kind string
	if err := s.db.QueryRow(`SELECT user_id, kind FROM tutor_thread WHERE id = ?`, threadID).Scan(&userID, &kind); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrNoThread
		}
		return 0, err
	}
	res, err := s.db.Exec(`
		INSERT INTO tutor_message
		    (thread_id, user_id, topic_code, role, content, lang, model, prompt_version, unattested, interrupted,
		     cut_reason, prompt_tokens, answer_tokens, context_tokens, card_id, card_phase, template, created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		threadID, userID, m.Topic, m.Role, m.Content, m.Lang, m.Model, m.PromptVersion, string(unatt), interrupted,
		m.CutReason, m.PromptTokens, m.AnswerTokens, m.ContextTokens, m.CardID, m.CardPhase, m.Template, db.FmtTime(now))
	if err != nil {
		return 0, fmt.Errorf("append tutor message: %w", err)
	}
	id, _ := res.LastInsertId()
	if m.Role == "user" && kind == "" {
		// The first question names the thread; later ones do not rename it.
		// A review chat is named by its word, at creation (ADR-034).
		if _, err := s.db.Exec(`UPDATE tutor_thread SET title = ?, updated_at = ? WHERE id = ? AND title = ''`,
			TitleFrom(m.Content), db.FmtTime(now), threadID); err != nil {
			return id, err
		}
	}
	if _, err := s.db.Exec(`UPDATE tutor_thread SET updated_at = ? WHERE id = ?`, db.FmtTime(now), threadID); err != nil {
		return id, err
	}
	// Prune the oldest beyond the keep. One statement, cheap, and it keeps
	// the table bounded without a maintenance job.
	if _, err := s.db.Exec(`
		DELETE FROM tutor_message
		 WHERE thread_id = ?
		   AND id NOT IN (
		       SELECT id FROM tutor_message WHERE thread_id = ?
		        ORDER BY id DESC LIMIT ?)`,
		threadID, threadID, ThreadKeep); err != nil {
		return id, fmt.Errorf("prune tutor thread: %w", err)
	}
	return id, nil
}

// Summary is one thread at a glance — the learner's chat list, most recently
// active first.
type Summary struct {
	ID        int64
	Title     string
	TopicCode string
	Kind      string
	// NoteID is the word a review chat is about; zero for every other kind.
	NoteID int64
	// LastTopic is the lesson the latest answer was grounded in, for a free
	// thread's subtitle.
	LastTopic   string
	Messages    int
	LastRole    string
	LastContent string
	UpdatedAt   time.Time
}

// Threads lists the learner's threads, most recently active first. One query
// — a per-thread window over the log — rather than a round trip per thread.
func (s *Store) Threads(userID int64) ([]Summary, error) {
	rows, err := s.db.Query(`
		SELECT t.id, t.title, t.topic_code, t.kind, t.note_id, t.updated_at,
		       COALESCE(m.role, ''), COALESCE(m.content, ''), COALESCE(m.total, 0),
		       COALESCE((SELECT a.topic_code FROM tutor_message a
		                  WHERE a.thread_id = t.id AND a.role = 'assistant'
		                  ORDER BY a.id DESC LIMIT 1), '')
		  FROM tutor_thread t
		  LEFT JOIN (
		        SELECT thread_id, role, content,
		               ROW_NUMBER() OVER (PARTITION BY thread_id ORDER BY id DESC) AS rn,
		               COUNT(*) OVER (PARTITION BY thread_id) AS total
		          FROM tutor_message
		       ) m ON m.thread_id = t.id AND m.rn = 1
		 WHERE t.user_id = ?
		 ORDER BY t.updated_at DESC, t.id DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("load tutor threads: %w", err)
	}
	defer rows.Close()
	out := []Summary{}
	for rows.Next() {
		var sm Summary
		var updated string
		if err := rows.Scan(&sm.ID, &sm.Title, &sm.TopicCode, &sm.Kind, &sm.NoteID, &updated,
			&sm.LastRole, &sm.LastContent, &sm.Messages, &sm.LastTopic); err != nil {
			return nil, err
		}
		sm.UpdatedAt = db.ParseTime(sql.NullString{String: updated, Valid: true})
		out = append(out, sm)
	}
	return out, rows.Err()
}

// TruncateFrom deletes one of the learner's own questions and everything
// after it — the edit-and-resend of a chat (ADR-031). A thread is not a
// branching tree: what came after the edited question was an answer to the
// old wording, and keeping it would leave the model arguing with a question
// that no longer exists. A thread whose first question was edited loses its
// title, so the new wording names it. ErrNoMessage for a turn that is not in
// this thread or is not a question.
func (s *Store) TruncateFrom(threadID, messageID int64) error {
	var role string
	err := s.db.QueryRow(`SELECT role FROM tutor_message WHERE id = ? AND thread_id = ?`, messageID, threadID).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && role != "user") {
		return ErrNoMessage
	}
	if err != nil {
		return err
	}
	if _, err := s.db.Exec(`DELETE FROM tutor_message WHERE thread_id = ? AND id >= ?`, threadID, messageID); err != nil {
		return fmt.Errorf("truncate tutor thread: %w", err)
	}
	var left int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM tutor_message WHERE thread_id = ?`, threadID).Scan(&left); err != nil {
		return err
	}
	if left == 0 {
		if _, err := s.db.Exec(`UPDATE tutor_thread SET title = '' WHERE id = ?`, threadID); err != nil {
			return err
		}
	}
	return nil
}

// DeleteThread removes one of the learner's threads and its turns.
func (s *Store) DeleteThread(userID, id int64) error {
	if _, err := s.GetThread(userID, id); err != nil {
		return err
	}
	if _, err := s.db.Exec(`DELETE FROM tutor_message WHERE thread_id = ?`, id); err != nil {
		return fmt.Errorf("delete tutor thread: %w", err)
	}
	if _, err := s.db.Exec(`DELETE FROM tutor_thread WHERE id = ? AND user_id = ?`, id, userID); err != nil {
		return fmt.Errorf("delete tutor thread: %w", err)
	}
	return nil
}

// DeleteThreads removes every thread this learner has, or every thread of one
// KIND. It returns how many went.
//
// A per-thread delete was the only way to clear anything, and a review thread
// is created once per word per day — so a month of reviewing leaves hundreds
// of them and «удалить чат» one at a time is not a feature, it is a punishment.
//
// `kind` is matched exactly and "" means all kinds. The messages go first, by
// the same two statements DeleteThread uses: `tutor_message.thread_id` is a
// plain INTEGER with no foreign key (migration 00016), so nothing cascades and
// leaving them would orphan every row.
func (s *Store) DeleteThreads(userID int64, kind string) (int64, error) {
	where := `user_id = ?`
	args := []any{userID}
	if kind != "" {
		where += ` AND kind = ?`
		args = append(args, kind)
	}
	if _, err := s.db.Exec(
		`DELETE FROM tutor_message WHERE thread_id IN (SELECT id FROM tutor_thread WHERE `+where+`)`,
		args...,
	); err != nil {
		return 0, fmt.Errorf("delete tutor messages: %w", err)
	}
	res, err := s.db.Exec(`DELETE FROM tutor_thread WHERE `+where, args...)
	if err != nil {
		return 0, fmt.Errorf("delete tutor threads: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// DeleteThreadsOfChatKind removes the ordinary chats — the ones whose stored
// `kind` is ”.
//
// A separate method rather than `DeleteThreads(userID, "")`, because that call
// means "every kind" and the two must not be one code path: a caller that
// confused them would clear a learner's entire tutor history while meaning to
// clear one tab.
func (s *Store) DeleteThreadsOfChatKind(userID int64) (int64, error) {
	if _, err := s.db.Exec(`
		DELETE FROM tutor_message
		 WHERE thread_id IN (SELECT id FROM tutor_thread WHERE user_id = ? AND kind = '')`,
		userID); err != nil {
		return 0, fmt.Errorf("delete tutor messages: %w", err)
	}
	res, err := s.db.Exec(`DELETE FROM tutor_thread WHERE user_id = ? AND kind = ''`, userID)
	if err != nil {
		return 0, fmt.Errorf("delete tutor threads: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// CountThreadsByKind is what the tabs need: how many chats of each kind, so a
// tab can carry its number and the delete-all confirm can name what it will
// take. The empty kind — a free or topic-pinned chat — keys as "chat".
func (s *Store) CountThreadsByKind(userID int64) (map[string]int, error) {
	rows, err := s.db.Query(`
		SELECT CASE WHEN kind = '' THEN 'chat' ELSE kind END AS k, COUNT(*)
		  FROM tutor_thread WHERE user_id = ? GROUP BY k`, userID)
	if err != nil {
		return nil, fmt.Errorf("count tutor threads: %w", err)
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var k string
		var n int
		if err := rows.Scan(&k, &n); err != nil {
			return nil, err
		}
		out[k] = n
	}
	return out, rows.Err()
}

func orEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
