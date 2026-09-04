# R19 — A grammar tutor on a small model: grounding, placement, and what it may not do

**Confidence: Medium-Low overall.** One strong RCT on AI tutoring design, robust engineering
results on retrieval grounding and context position, and a benchmark literature that says
Hungarian is exactly where models are weakest. The pedagogy of *explanation on demand* rests on
[R4](04-explicit-instruction.md), which is High.

## Claims and evidence

### 1. An AI tutor built on learning-science principles can beat good teaching — **Medium (one RCT)**

Kestin, Miller et al. (2025, *Scientific Reports*): 194 Harvard undergraduates, crossover RCT,
an AI tutor built to the same principles as the in-class active-learning lesson (timely
feedback, proactive engagement, managed cognitive load, growth-mindset framing). Students
learned **more than twice as much** with the tutor, in less time, and reported higher engagement
and motivation. One course, one model (GPT-4), physics — and the authors stress the *design*, not
the model, as the active ingredient.

- Kestin et al. 2025 — https://www.nature.com/articles/s41598-025-97652-6

What transfers here: the tutor is scaffolded by the app's own lesson, it answers the learner's
question rather than lecturing, and it is asked to end with a check the learner can act on.
What does not transfer: a 7–8B local model is not GPT-4, and Hungarian is not physics (§4).

### 2. Retrieval grounding reduces hallucination — **High (engineering)**

Lewis et al. (2020, NeurIPS) introduced retrieval-augmented generation; Shuster et al. (2021,
EMNLP Findings) showed retrieval augmentation **reduces hallucination in conversation**
specifically. The mechanism is mundane and reliable: a model asked to explain from a passage in
front of it invents less than a model asked to recall.

- Lewis et al. 2020 — https://arxiv.org/abs/2005.11401
- Shuster et al. 2021 — https://aclanthology.org/2021.findings-emnlp.320.pdf

### 3. Where a fact sits in the prompt decides whether a model uses it — **High (engineering)**

Liu et al. (2023, "Lost in the Middle"): performance is highest when the relevant passage is at
the **beginning or end** of the context and degrades markedly in the middle, and degrades as
the context grows — for long-context models too. For a small model the consequences are
sharper still: keep the prompt short, put the rules first and the question last, and do not
bury the lesson under a reference dump.

- Liu et al. 2023 — https://arxiv.org/abs/2307.03172

### 4. Hungarian is where models are weakest — **Medium**

OpenHuEval (2025, ACL Findings): the first Hungarian-specific LLM benchmark, 3,953 questions
over eight dimensions; models that look fluent still fail on Hungarian specifics. Hallucination
rates are consistently higher in lower-resource languages, and higher again for smaller open
models. This is [R10](10-llm-tutors.md)'s caveat with numbers behind it, and it is the reason
the tutor is grounded rather than trusted.

- OpenHuEval — https://aclanthology.org/2025.findings-acl.390/
- Hallucination in low-resource conversation (2025) — https://arxiv.org/pdf/2507.22720

### 5. Guardrails and structure beat an open chat — **Medium-Low**

The 2024–25 tutoring literature (AutoTutor-with-LLM at Learning@Scale 2024; MWPTutor;
pedagogically-aligned tutors) converges on one design: the model fills **constrained slots** in
a pedagogy the system owns, with explicit checks, rather than free-running. Evidence is mostly
maths tutoring and short studies.

- AutoTutor meets LLMs — https://dl.acm.org/doi/10.1145/3657604.3662041
- Training LLM tutors for outcomes — https://arxiv.org/pdf/2503.06424

### 6. Explanation on demand is explicit instruction — **High (via R4)**

[R4]: stating the rule beats inferring it (g ≈ 1.29 vs 0.77), and the effect holds on free
production. A learner who asks "why -ban and not -ben?" and gets the rule, with the harmony
class named and two contrasting examples, is receiving explicit instruction at the moment of
need. That is the tutor's whole job; it is not a conversation partner ([R10] wants voice for
that, and Loop 4 stays human — [R16 §7](16-debunked.md)).

## What this changes in our code

1. **The tutor explains; it does not generate material.** Nothing it writes becomes a card, a
   gloss or a graded answer — the same line [ADR-014](../DECISIONS.md#adr-014) draws for
   machine translation. `internal/corpus` attestation is therefore not on its path; what is on
   it is **labelling** (§4): every answer names the model, and every Hungarian word the model
   puts in backticks is looked up in the pack's `wordform` table — the ones the dictionary does
   not carry are listed under the answer as *not found in the dictionary*.
2. **Grounded from three sources, in this order** (§2, §3): the pack's own lesson for the open
   topic (`grammar_topic.explain_md_*`, the app's authority), the pack's feature labels and
   allomorph tables, and a small embedded reference corpus (`internal/tutor/kb`) assembled from
   openly licensed grammars. Retrieval is keyword-and-topic scoring over titled sections — no
   embedding model, because the corpus is a few hundred sections and the deployment is a single
   static binary ([ADR-001](../DECISIONS.md#adr-001)).
3. **Built for a weak model** (§3, §4): a fixed context budget (~4,500 tokens), the system rules
   at the top, the learner's question and the answer-language reminder at the very end, the
   lesson before the reference, and the reference cut to the best three sections. The model is
   told it may say it does not know, and that a wrong Hungarian form is worse than no form.
4. **It answers in the learner's interface language** — the client sends its rendered locale,
   the server accepts `ru` or `en` and falls back to `locale.ui`. Grammar terms are the pack's
   own labels in that language, so «инессив» on the card and in the answer are the same word.
5. **It knows the learner** (§1): the accuracy and observation count on the topic's features
   travel in the prompt as a two-line "learner state", so "you get the inessive right 62% of
   the time" is a fact the tutor has and not one it invents.
6. **It is placed after the lesson, never before it.** [R4] is the order of the grammar screen
   and the tutor does not reorder it: the rule is read first, the question is asked about it.
7. **Streaming, persisted per topic, one flight per learner.** A local model on a CPU takes
   seconds per sentence; the answer appears as it is written, the thread survives a reload,
   and a second question waits for the first.

## Caveats

- §1 is one trial with a frontier model; nothing here has been tested with a 7B model on
  Hungarian grammar questions from Russian speakers. Expect errors, and expect the labelling to
  be doing real work.
- Keyword retrieval misses paraphrase. A question with no Hungarian word and no grammar term
  in it retrieves only the open topic's own section — which is the right fallback, and the
  reason the topic is always in the prompt.
- The reference corpus is English-only text from open sources. A Russian-locale learner gets a
  Russian answer produced from English references plus the pack's Russian lesson; that is a
  known asymmetry, stated in `docs/TUTOR.md`.

## Related

[[R4 explicit instruction]](04-explicit-instruction.md) · [[R9 corrective feedback]](09-corrective-feedback.md) · [[R10 LLM tutors]](10-llm-tutors.md) · [[R16 debunked]](16-debunked.md) · [ADR-027](../DECISIONS.md#adr-027)
