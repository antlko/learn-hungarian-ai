package api

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"

	"tanulom/internal/srs"
)

// maxExcludedExamples bounds the `exclude` list: «Ещё примеры» sends the ids
// already shown, a few per press.
const maxExcludedExamples = 200

// handleTopicExamples serves attested example sentences for a lesson's
// features. The sentences come from the pack, the focus token is split on the
// server (the client never slices with offsets), and a feature with no surface
// ending returns an empty list.
func (s *Server) handleTopicExamples(c fiber.Ctx) error {
	u, err := mustUser(c)
	if err != nil {
		return err
	}
	if err := s.requireCurriculum(); err != nil {
		return err
	}
	topic, ok := s.dag.Topic(c.Params("code"))
	if !ok {
		return fiber.NewError(fiber.StatusNotFound, "no such topic")
	}

	// Repeated ids are collapsed and the list is capped at the topic's own
	// feature count: each id costs pack queries, and a topic cannot own more
	// distinct features than it has.
	var featureIDs []string
	seenFeature := map[string]bool{}
	for _, raw := range c.RequestCtx().QueryArgs().PeekMulti("feature") {
		for _, id := range strings.Split(string(raw), ",") {
			id = strings.TrimSpace(id)
			if id == "" || seenFeature[id] {
				continue
			}
			if len(featureIDs) >= len(topic.FeatureIDs) {
				return badRequest("feature takes at most %d ids for this topic", len(topic.FeatureIDs))
			}
			seenFeature[id] = true
			featureIDs = append(featureIDs, id)
		}
	}
	if len(featureIDs) == 0 {
		featureIDs = topic.FeatureIDs
	}

	exclude := map[int64]bool{}
	if raw := c.Query("exclude"); raw != "" {
		parts := strings.Split(raw, ",")
		if len(parts) > maxExcludedExamples {
			return badRequest("exclude takes at most %d ids", maxExcludedExamples)
		}
		for _, p := range parts {
			id, err := strconv.ParseInt(strings.TrimSpace(p), 10, 64)
			if err != nil {
				return badRequest("exclude must be a comma list of sentence ids")
			}
			exclude[id] = true
		}
	}

	known, err := srs.NewStore(s.db.DB).KnownLemmaIDs(u.ID)
	if err != nil {
		return err
	}
	knownIDs := make(map[int64]bool, len(known))
	for id, status := range known {
		if status != "ignored" {
			knownIDs[id] = true
		}
	}

	out := TopicExamplesResponse{Topic: topic.Code, Features: []FeatureExamplesDTO{}}
	for _, id := range featureIDs {
		exs, owned, err := s.drills.Examples(topic, id, knownIDs, exclude)
		if err != nil {
			return err
		}
		if !owned {
			return badRequest("feature %q is not taught by this topic", id)
		}
		out.Features = append(out.Features, FeatureExamplesDTO{FeatureID: id, Examples: exs})
	}
	return c.JSON(out)
}
