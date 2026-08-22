# R10 — LLM chatbots as practice partners

**Confidence: Medium-Low.** Effects are consistently positive but the primary studies are short
and heterogeneous. This is the weakest evidence base in the project — treat accordingly.

## Claim

Conversational agents improve L2 outcomes, reduce speaking anxiety and raise willingness to
communicate. **Voice input is a significant positive moderator** — spoken interaction beats
typed interaction.

## Evidence

| Source | k | Effect |
|---|---|---|
| Lyu et al. (2025), *Int. J. Applied Linguistics* | comparative studies | **g = 0.61** |
| Wang, Cheung, Neitzel & Chai (2025), *Review of Educational Research* | 28 studies, 70 ES | **g ≈ 0.48** |
| Li (2025), *J. Computer Assisted Learning* | meta-analysis | GenAI chatbots > rule-based |

**Moderators that matter to us:**
- **GenAI-powered chatbots produce significantly larger effects than rule-based / pattern-matching ones.**
- **Chatbots supporting voice input produce significantly larger effects than those that do not.**
- Consistent reductions in **speaking anxiety** and gains in **willingness to communicate**, via
  a non-judgemental practice space.
- Larger effects for **non-English L2** and in **informal learning** — i.e. precisely our case.

Links:
- Lyu et al. 2025 — https://onlinelibrary.wiley.com/doi/full/10.1111/ijal.12668
- Wang et al. 2025 — https://journals.sagepub.com/doi/abs/10.3102/00346543241255621
- Li 2025 — https://onlinelibrary.wiley.com/doi/10.1111/jcal.70060
- Systematic review — https://www.sciencedirect.com/science/article/pii/S2215039025000086
- Oral proficiency & WTC — https://www.sciencedirect.com/science/article/abs/pii/S0346251X2500329X

## What this changes in our code

1. **Voice is core to the roleplay trainer, not a later nicety.** The moderator finding is the
   reason `services/asr` + `services/tts` are wired into roleplay in the same release as the
   text path. A text-only tutor would be building the weaker version on purpose.
2. **The roleplay partner is instructed to be imperfect.** `.claude/prompts/roleplay-*.md`
   tells the model to speak at natural pace, use colloquial contractions, and ask the learner
   to repeat — per source plan §6.1 #4. A maximally accommodating interlocutor removes the
   signal the learner needs.
3. **Scenarios are Budapest-specific and practical**: NAV (`adókártya`), orvos, bank,
   `ügyintézés`, szerviz, lakás, posta. Stored as `grammar_topic`-linked scenario cards so a
   roleplay can target the feature you are currently weak at.
4. **Every LLM-produced Hungarian string passes the attestation guardrail** before it can become
   study material — see `internal/corpus/attest.go` and the caveat below.
5. **`llm_log` records prompt, response and `attested` flag** so the learner can audit what the
   model told them, and so a bad generation can be traced.

## ⚠️ The caveat that drives an entire subsystem

Source plan §6.2 states the rule we build on:

> **LLM генерирует объём, человек валидирует эталон.**

LLM quality in Hungarian is materially below its English quality — which is why Hungarian-specific
benchmarks exist at all: **HuLU** (understanding), **OpenHuEval** (Hungarian-specific knowledge),
**HuGME** (hallucination, relevance, bias). Models readily produce grammatical but
**unidiomatic** Hungarian, and a learner will memorise it as a model sentence.

Therefore `internal/corpus/attest.go` checks 2–4-grams of any generated Hungarian against the
language pack's `ngram` index (built from Hungarian Webcorpus 2.0). Unattested spans are marked
**«не подтверждено корпусом» / "not attested in corpus"** and **cannot be added as a card
without an explicit override**. This is a gate, not a suggestion.

## Additional caveats

- Interventions in these meta-analyses are often **1–7 days**. There are no 12-month RCTs.
- High heterogeneity; publication bias plausible.
- The anxiety-reduction finding cuts both ways: a partner with **no social stakes** also
  provides no social encoding. This is exactly why the source plan's Loop 4 is
  un-substitutable, and why our Today view refuses to let the app tick it off.

## Related

[[R9]](09-corrective-feedback.md) · [[R8]](08-capt-asr.md) · [[R14 motivation]](14-motivation.md) · [[R17]](17-hungarian-nlp-stack.md)
