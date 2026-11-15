# Authored reading texts (ADR-075)

One JSON file per basic-tier course unit: `<unit code>.json`, up to three texts
(`short`, `medium`, `long`) in different genres. Format, rules, the genre list and
how to regenerate a text with a neural net: `docs/LANGPACK.md` "Reading sets".

    python -m pipeline.texts PACK --brief m1-u1-koszones --tier long   # the author's brief
    python -m pipeline.texts PACK [--require-all]                      # validate (read-only)
