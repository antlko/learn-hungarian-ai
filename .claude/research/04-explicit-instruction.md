# R4 — Explicit grammar instruction beats implicit

**Confidence: High.** Three independent meta-analyses spanning 25 years agree on direction.

## Claim

Explicitly stating a grammatical rule and drawing attention to it produces larger and more
durable gains than exposure-only ("just immerse") approaches — including on measures of
spontaneous production, not merely on discrete-point tests.

## Evidence

| Source | k | Finding |
|---|---|---|
| Norris & Ortega (2000), *Language Learning* 50(3) | 49 studies (1980–1998) | Explicit **g ≈ 1.29** vs implicit **g ≈ 0.77**; explicit effects more durable |
| Goo, Granena, Yilmaz & Novella (2015), in *Implicit and Explicit Learning of Languages* | 34 studies (11 from N&O + 23 new, 1999–2011) | Replicates and updates: explicit > implicit overall |
| Spada & Tomita (2010), *Language Learning* 60(2) | 41 studies | Explicit wins for **both simple and complex** features, and **transfers to free production** |

- Norris & Ortega 2000 — https://onlinelibrary.wiley.com/doi/abs/10.1111/0023-8333.00136
- Goo et al. 2015 — https://benjamins.com/catalog/sibil.48.18goo
- Spada & Tomita 2010 — https://onlinelibrary.wiley.com/doi/10.1111/j.1467-9922.2010.00562.x
- Recent extension: Li (2024), *Foreign Language Annals* — https://onlinelibrary.wiley.com/doi/10.1111/flan.12726

## What this changes in our code

1. **Every grammar topic opens with a rule card, not with examples.** `grammar_topic.explain_md`
   in the language pack is a required field — a topic without an explanation cannot be seeded.
   Enforced in `langpack/pipeline/`.
2. **Vowel harmony and definite/indefinite conjugation are taught as stated rules first**, then
   automated by volume. This is exactly the source plan's §1.1 point: you cannot pick these up
   from podcasts.
3. **The morpheme breakdown shown after every answer is explicit instruction in miniature** —
   it names the feature (`Case=Ine`), names the allomorph rule (back harmony → `-ban`), and
   shows why. `internal/morph/explain.go`.
4. **We do not build a "pure comprehensible input" mode.** Reading ([R11](11-extensive-reading.md))
   exists, but it is framed as input volume that *complements* explicit instruction, never as a
   substitute for it.

## Caveats

- "Explicit" in these meta-analyses covers a wide range of treatments; the construct is fuzzier
  than the effect size suggests.
- Nearly all primary studies are short (weeks). Durability claims rest on delayed post-tests
  measured in weeks, not years.
- Publication bias in this literature is real and under-corrected.

## Related

[[R5 interleaving]](05-interleaving.md) · [[R9 corrective feedback]](09-corrective-feedback.md) · [[R15 Hungarian difficulty]](15-hungarian-difficulty.md)
