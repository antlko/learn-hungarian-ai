/**
 * What the full language pack adds over the starter one.
 *
 * NOT measured at runtime. `starter` was counted by decompressing the embedded
 * starter pack (5,725 lemmas, 587,898 forms, 12,858 sentences, 2,598 Russian
 * glosses) and must be re-counted after `make starter`; `full` is per the docs
 * (docs/LANGPACK.md). The Components card labels both for exactly that reason.
 * They live here, once, so a pack rebuild changes one file and not four strings.
 */
export const PACK_FACTS = {
  starter: { lemmas: 5725, sentences: 12858 },
  full: { lemmas: 35311, sentences: 60000 },
} as const
