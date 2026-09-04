"""Build the grammar tutor's reference corpus from openly licensed sources.

The tutor (`backend/internal/tutor`, ADR-027) explains grammar from three things:
the language pack's own lesson for the open topic, the pack's feature tables, and
a small embedded reference corpus. This module builds the third one.

    cd langpack && python -m pipeline.tutor_kb --out ../backend/internal/tutor/kb

What it fetches, and why each is admissible:

* Wikipedia's Hungarian grammar articles and Wiktionary's Hungarian appendices —
  CC BY-SA 4.0, the same licence the language pack already carries (see
  ../LICENSES.md). Converted from wikitext to markdown here; every output file
  names its source, URL and licence in its front matter.
* The grammar notes of the US Foreign Service Institute's *Hungarian Basic
  Course* (1962) — a US government work, public domain. The archive.org scan is
  OCR'd WITHOUT diacritics (`Kerek egy belyeget`), which for a weak model is
  worse than no example at all: it would copy the accentless forms. So every
  Hungarian-looking token in the extracted notes is looked up in the pack, and
  restored when exactly one attested form strips to it and the token is not an
  English word. Ambiguous tokens are left as they are.

Sections are titled with `##`/`###` headings and carry `<!-- topics: … -->`
lines naming the curriculum topics they serve, assigned from heading keywords in
TOPIC_HINTS below. The Go loader splits on headings and reads those comments.

Excluded on purpose: Ignácz Singer's 1882 *Simplified Grammar* on Wikisource —
public domain and a real book, but its orthography predates the reforms (`cz`),
and a small model would learn it.

Hand-written reference files (`ref-*.md`) in the same directory are NOT
produced here; they are authored and reviewed by hand, and this script never
touches them.
"""

from __future__ import annotations

import argparse
import html
import re
import sqlite3
import sys
import unicodedata
import urllib.parse
import urllib.request
from pathlib import Path

USER_AGENT = "tanulom-tutor-kb/1.0 (grammar reference build; https://github.com/antlko/learn-hungarian-ai)"

WIKIPEDIA = [
    ("Hungarian_grammar", "Hungarian grammar"),
    ("Hungarian_verbs", "Hungarian verbs"),
    ("Hungarian_noun_phrase", "Hungarian noun phrase"),
    ("Hungarian_phonology", "Hungarian phonology"),
    ("Hungarian_orthography", "Hungarian orthography"),
]

WIKTIONARY = [
    "Appendix:Hungarian suffixes",
    "Appendix:Hungarian possessive suffixes",
    "Appendix:Hungarian postpositions",
    "Appendix:Hungarian pronouns",
    "Appendix:Hungarian verbal prefixes",
    "Appendix:Hungarian pronunciation",
    "Appendix:Hungarian case usage - inessive (-ban/-ben)",
    "Appendix:Hungarian antiharmonic words",
    "Appendix:Hungarian low-vowel nouns",
    "Appendix:Hungarian v-stem words",
    "Appendix:Hungarian fleeting-vowel words",
    "Appendix:Hungarian third-person possessive forms",
    "Appendix:Hungarian words of time",
    "Appendix:Hungarian compound verbs",
    "Appendix:Hungarian verbs used only with a prefix",
]

FSI = [
    ("Volume1", "https://archive.org/download/Fsi-HungarianBasicCourse-StudentText/"
                "Fsi-HungarianBasicCourse-Volume1-StudentText_djvu.txt"),
    ("Volume2", "https://archive.org/download/Fsi-HungarianBasicCourse-StudentText/"
                "Fsi-HungarianBasicCourse-Volume2-StudentText_djvu.txt"),
]

# Heading keyword -> curriculum topic codes (langpack/pipeline/curriculum.py).
# Matched case-insensitively against the section's own heading and its parents.
TOPIC_HINTS: list[tuple[str, tuple[str, ...]]] = [
    ("vowel harmony", ("vowel-harmony",)),
    ("harmony", ("vowel-harmony",)),
    ("alphabet", ("orthography",)),
    ("orthograph", ("orthography",)),
    ("pronunciation", ("orthography",)),
    ("phonolog", ("orthography",)),
    ("vowel length", ("orthography",)),
    ("stress", ("orthography",)),
    ("digraph", ("orthography",)),
    ("assimilation", ("orthography", "instrumental")),
    ("article", ("van-nincs",)),
    ("copula", ("van-nincs",)),
    ("existential", ("van-nincs",)),
    ("negation", ("van-nincs",)),
    ("negative", ("van-nincs",)),
    ("word order", ("focus-word-order",)),
    ("focus", ("focus-word-order",)),
    ("topic", ("focus-word-order",)),
    ("plural", ("plural",)),
    ("accusative", ("accusative",)),
    ("direct object", ("accusative",)),
    ("object", ("accusative", "definite-conjugation")),
    ("inessive", ("locative-grid",)),
    ("illative", ("locative-grid",)),
    ("elative", ("locative-grid",)),
    ("superessive", ("locative-grid",)),
    ("sublative", ("locative-grid",)),
    ("delative", ("locative-grid",)),
    ("adessive", ("locative-grid",)),
    ("allative", ("locative-grid",)),
    ("ablative", ("locative-grid",)),
    ("-ban", ("locative-grid",)),
    ("locative", ("locative-grid", "remaining-cases")),
    ("place", ("locative-grid",)),
    ("dative", ("dative",)),
    ("indirect object", ("dative",)),
    ("possess", ("possessive", "dative")),
    ("genitive", ("possessive", "dative")),
    ("past", ("past-tense",)),
    ("definite", ("definite-conjugation",)),
    ("indefinite", ("present-indefinite", "definite-conjugation")),
    ("conjugation", ("present-indefinite", "definite-conjugation")),
    ("present tense", ("present-indefinite",)),
    ("present", ("present-indefinite",)),
    ("-lak", ("lak-lek",)),
    ("lak/-lek", ("lak-lek",)),
    ("infinitive", ("infinitive",)),
    ("potential", ("potential",)),
    ("-hat", ("potential",)),
    ("instrumental", ("instrumental",)),
    ("comitative", ("instrumental",)),
    ("comparison", ("comparison",)),
    ("comparative", ("comparison",)),
    ("superlative", ("comparison",)),
    ("prefix", ("preverbs", "focus-word-order")),
    ("preverb", ("preverbs", "focus-word-order")),
    ("coverb", ("preverbs", "focus-word-order")),
    ("conditional", ("conditional",)),
    ("imperative", ("imperative",)),
    ("subjunctive", ("imperative",)),
    ("participle", ("participles",)),
    ("causal", ("remaining-cases",)),
    ("terminative", ("remaining-cases",)),
    ("translative", ("remaining-cases",)),
    ("temporal", ("remaining-cases",)),
    ("essive", ("remaining-cases",)),
    ("distributive", ("remaining-cases",)),
    ("formal", ("remaining-cases",)),
    ("case", ("locative-grid", "remaining-cases")),
    ("postposition", ("remaining-cases",)),
    ("causative", ("causative",)),
    ("converb", ("converb",)),
    ("adverbial participle", ("converb",)),
    ("frequentative", ("iterative",)),
    ("iterative", ("iterative",)),
    ("stem", ("stem-alternation",)),
    ("fleeting", ("stem-alternation",)),
    ("v-stem", ("stem-alternation",)),
    ("low-vowel", ("stem-alternation", "plural")),
    ("register", ("register",)),
    ("formal address", ("politeness",)),
    ("polite", ("politeness",)),
    ("ön", ("politeness",)),
    ("pronoun", ("politeness", "possessive")),
    ("ik verb", ("present-indefinite",)),
    ("'ik'", ("present-indefinite",)),
    ("-ik", ("present-indefinite",)),
    ("number", ("plural",)),
    ("adjective", ("comparison",)),
    ("time", ("remaining-cases",)),
]


# ---------------------------------------------------------------------------
# Fetching


def fetch(url: str, cache: Path) -> str:
    cache.parent.mkdir(parents=True, exist_ok=True)
    if cache.exists() and cache.stat().st_size > 0:
        return cache.read_text(encoding="utf-8", errors="replace")
    req = urllib.request.Request(url, headers={"User-Agent": USER_AGENT})
    with urllib.request.urlopen(req, timeout=120) as res:  # noqa: S310 — pinned public sources
        data = res.read()
    cache.write_bytes(data)
    return data.decode("utf-8", errors="replace")


def wiki_raw(host: str, title: str, cache_dir: Path) -> str:
    url = f"https://{host}/w/index.php?" + urllib.parse.urlencode({"title": title, "action": "raw"})
    fn = re.sub(r"[^A-Za-z0-9]+", "_", title).strip("_") + ".wiki"
    return fetch(url, cache_dir / host / fn)


# ---------------------------------------------------------------------------
# Wikitext -> markdown. Pragmatic, not a parser: the target reader is a language
# model, which forgives an imperfect table far more readily than it forgives an
# invented suffix.

_TEMPLATE_KEEP_INNER = {"nowrap", "small", "sc", "smallcaps", "lang", "IPA", "IPAchar", "angbr",
                        "transl", "respell", "w", "wikt", "nobr", "big", "unicode", "script"}


def _split_args(s: str) -> list[str]:
    """Split template arguments on top-level pipes."""
    out, depth, cur = [], 0, []
    i = 0
    while i < len(s):
        ch = s[i]
        if s.startswith("{{", i) or s.startswith("[[", i):
            depth += 1
            cur.append(s[i:i + 2])
            i += 2
            continue
        if s.startswith("}}", i) or s.startswith("]]", i):
            depth -= 1
            cur.append(s[i:i + 2])
            i += 2
            continue
        if ch == "|" and depth == 0:
            out.append("".join(cur))
            cur = []
        else:
            cur.append(ch)
        i += 1
    out.append("".join(cur))
    return out


def _render_template(inner: str) -> str:
    parts = _split_args(inner)
    name = parts[0].strip().lower()
    args = parts[1:]
    positional = [a for a in args if "=" not in a.split("|", 1)[0] or a.strip().startswith("[[")]
    named = {}
    for a in args:
        k, sep, v = a.partition("=")
        if sep and re.fullmatch(r"\s*[a-z0-9_]+\s*", k):
            named[k.strip()] = v.strip()
    if name in ("m", "l", "m+", "l+", "mention", "link", "ll"):
        # {{m|hu|word|display|gloss}} or {{m|hu|word||gloss}} / t= gloss
        word = positional[1].strip() if len(positional) > 1 else ""
        display = positional[2].strip() if len(positional) > 2 and positional[2].strip() else word
        gloss = named.get("t") or named.get("gloss") or (positional[3].strip() if len(positional) > 3 else "")
        display = _inline_markup(display)
        return f"{display} (“{gloss}”)" if gloss else display
    if name in ("lang", "lang-hu"):
        return positional[-1].strip() if positional else ""
    if name in _TEMPLATE_KEEP_INNER:
        return positional[-1].strip() if positional else ""
    if name in ("gloss", "gl", "glossary", "q", "qual", "qualifier", "i"):
        return "(" + ", ".join(p.strip() for p in positional if p.strip()) + ")" if positional else ""
    if name in ("ipa", "ipachar"):
        return positional[-1].strip() if positional else ""
    if name in ("sic",):
        return positional[0].strip() if positional else ""
    if name in ("ndash", "endash"):
        return "–"
    if name in ("mdash", "emdash"):
        return "—"
    if name == "·":
        return "·"
    if name in ("nbsp",):
        return " "
    # Everything else — citations, infoboxes, navigation, maintenance — is
    # dropped whole. Leaking `{{cite web…}}` into the corpus would be noise the
    # model might echo.
    return ""


def _strip_templates(text: str) -> str:
    """Replace every {{…}} recursively, innermost first."""
    pattern = re.compile(r"\{\{([^{}]*)\}\}", re.S)
    prev = None
    while prev != text:
        prev = text
        text = pattern.sub(lambda m: _render_template(m.group(1)), text)
    return text


def _inline_markup(s: str) -> str:
    s = re.sub(r"\[\[(?:[^|\]]*\|)?([^\]]*)\]\]", r"\1", s)  # [[a|b]] -> b, [[a]] -> a
    s = re.sub(r"\[(?:https?|ftp)://\S+\s+([^\]]*)\]", r"\1", s)  # [url text] -> text
    s = re.sub(r"\[(?:https?|ftp)://\S+\]", "", s)
    s = s.replace("'''", "**").replace("''", "*")
    return s


def _table_to_md(block: str) -> str:
    """A rough wikitable -> markdown table. Header cells become the first row."""
    rows: list[list[str]] = []
    cur: list[str] = []
    for raw in block.split("\n"):
        line = raw.strip()
        if not line or line.startswith("{|") or line.startswith("|}"):
            continue
        if line.startswith("|+"):
            continue  # caption
        if line.startswith("|-"):
            if cur:
                rows.append(cur)
            cur = []
            continue
        if line.startswith("!") or line.startswith("|"):
            # Links first, so a pipe inside `[[-a#Hungarian|-a]]` is not read
            # as a cell boundary or an attribute separator.
            body = _inline_markup(line[1:])
            cells = re.split(r"\|\||!!", body)
            for c in cells:
                # drop cell attributes: `style="…" | text`, `colspan=2 | text`
                c = re.sub(r'^\s*(?:[a-z]+\s*=\s*(?:"[^"]*"|\S+)\s*)+\|', "", c)
                cur.append(c.strip())
        else:
            if cur:
                cur[-1] = (cur[-1] + " " + line).strip()
    if cur:
        rows.append(cur)
    rows = [r for r in rows if any(c for c in r)]
    if not rows:
        return ""
    # Wiktionary appendices carry a "Categories" column of maintenance links;
    # it is noise to a reader and to a model.
    width = max(len(r) for r in rows)
    drop = set()
    for col in range(width):
        head = rows[0][col] if col < len(rows[0]) else ""
        cells = [r[col] for r in rows if col < len(r)]
        if head.strip().lower() == "categories" or all(
            (not c) or c.startswith(":Category:") for c in cells):
            drop.add(col)
    rows = [[c for i, c in enumerate(r) if i not in drop] for r in rows]
    rows = [r for r in rows if any(c for c in r)]
    if not rows:
        return ""
    width = max(len(r) for r in rows)
    out = []
    for i, r in enumerate(rows):
        r = r + [""] * (width - len(r))
        out.append("| " + " | ".join(c.replace("|", "/") for c in r) + " |")
        if i == 0:
            out.append("|" + " --- |" * width)
    return "\n".join(out)


def wikitext_to_md(text: str) -> str:
    text = re.sub(r"<!--.*?-->", "", text, flags=re.S)
    text = re.sub(r"<ref[^>/]*/>", "", text)
    text = re.sub(r"<ref[^>]*>.*?</ref>", "", text, flags=re.S)
    text = re.sub(r"<(?:gallery|imagemap|math|score|timeline)[^>]*>.*?</(?:gallery|imagemap|math|score|timeline)>", "", text, flags=re.S | re.I)
    text = re.sub(r"\[\[(?:File|Image|Category|kép|Kép):[^\]]*\]\]", "", text, flags=re.I)
    text = _strip_templates(text)
    # Tables, before inline markup mangles the pipes.
    text = re.sub(r"\{\|.*?\|\}", lambda m: "\n" + _table_to_md(m.group(0)) + "\n", text, flags=re.S)
    text = re.sub(r"<br\s*/?>", "\n", text, flags=re.I)
    text = re.sub(r"</?(?:small|big|sup|sub|span|div|center|code|tt|kbd|abbr|u|s|i|b|em|strong|font|poem|blockquote|section|nowiki|includeonly|onlyinclude|noinclude|table|tr|td|th|p)[^>]*>", "", text, flags=re.I)
    text = html.unescape(text)

    out = []
    for line in text.split("\n"):
        m = re.match(r"^(={2,6})\s*(.*?)\s*=+\s*$", line)
        if m:
            level = len(m.group(1))
            out.append("#" * level + " " + _inline_markup(m.group(2).strip()))
            continue
        if line.startswith("|") and line.count("|") >= 2:
            out.append(line.rstrip())  # an already converted table row
            continue
        if line.startswith("*") or line.startswith("#"):
            depth = len(re.match(r"^[*#:]+", line).group(0))
            body = _inline_markup(line[depth:].strip())
            if not body:
                continue
            out.append("  " * (depth - 1) + ("- " if line[0] == "*" else "1. ") + body)
            continue
        if line.startswith(":"):
            body = _inline_markup(line.lstrip(":").strip())
            if body:
                out.append("    " + body)
            continue
        if line.startswith(";"):
            out.append("**" + _inline_markup(line[1:].strip()) + "**")
            continue
        out.append(_inline_markup(line.rstrip()))
    md = "\n".join(out)
    md = re.sub(r"\n{3,}", "\n\n", md).strip() + "\n"
    return md


# ---------------------------------------------------------------------------
# FSI grammar notes


FSI_END_HEADINGS = re.compile(
    r"^(SUBSTITUTION DRILL|TRANSFORMATION DRILL|VARIATION DRILL|VOCABULARY DRILL|"
    r"TRANSLATION DRILL|RESPONSE DRILL|CONVERSATION PRACTICE|SITUATIONS|NARRATIVE|"
    r"EXPANSION DRILL|COMPREHENSION|Basic Sentences|BASIC SENTENCES|UNIT \d+)\s*$"
)
FSI_NOISE = re.compile(
    r"^(SPOKEN HUNGARIAN|UNIT \d+|[A-ZÁÉÍÓÖŐÚÜŰ]{3,}|\d{1,3}|[ivxlc]+)\s*$"
)


def fsi_notes(text: str) -> list[tuple[str, str]]:
    """Return (title, body) for every 'Notes on Grammar' block in a volume."""
    lines = text.split("\n")
    blocks = []
    i = 0
    unit = 0
    while i < len(lines):
        if re.match(r"^UNIT\s+(\d+)\s*$", lines[i].strip()):
            unit = int(re.match(r"^UNIT\s+(\d+)", lines[i].strip()).group(1))
        if lines[i].strip() == "Notes on Grammar" and i + 2 < len(lines) and "Home Study" in lines[i + 1]:
            j = i + 2
            body = []
            while j < len(lines) and not FSI_END_HEADINGS.match(lines[j].strip()):
                body.append(lines[j])
                j += 1
            blocks.append((unit, body))
            i = j
            continue
        i += 1
    out = []
    for unit, body in blocks:
        cleaned = []
        for raw in body:
            line = raw.rstrip()
            if FSI_NOISE.match(line.strip()):
                continue
            cleaned.append(line)
        md = _fsi_block_to_md(cleaned)
        if len(md) > 400:
            out.append((f"Unit {unit}", md))
    return out


def _fsi_block_to_md(lines: list[str]) -> str:
    out = []
    for line in lines:
        s = line.strip()
        m = re.match(r"^([A-H])[.\-]\s+(.+)$", s)
        if m and len(m.group(2)) < 90 and not s.endswith("."):
            out.append("")
            out.append("### " + m.group(2).strip())
            out.append("")
            continue
        out.append(s)
    text = "\n".join(out)
    # OCR leaves single-line breaks inside paragraphs and 2–4 blank lines
    # between them; collapse to paragraphs.
    text = re.sub(r"\n{2,}", "\n\n", text)
    text = re.sub(r"(?<!\n)\n(?!\n)", " ", text)
    text = re.sub(r"[ \t]{2,}", " ", text)
    # A few of the scan's tics, where the fix is unambiguous: a caret glued to
    # a word, `'the 1 .` for `'the'.`, and `1 1` for an opening quote.
    text = text.replace("^", "")
    text = re.sub(r"'(\w+) 1 \.", r"'\1'.", text)
    text = re.sub(r"(?<![\w'])1 1 (?=\w)", "'", text)
    text = re.sub(r"\s+([,.;:])", r"\1", text)
    # The scan turns every paradigm table into a column of one-word
    # paragraphs, which no reader can reassemble; keep the prose only.
    paras = [p for p in text.split("\n\n") if p.startswith("### ") or len(p.split()) >= 4]
    text = "\n\n".join(paras)
    return text.strip() + "\n"


HU_LETTERS = "áéíóöőúüűÁÉÍÓÖŐÚÜŰ"


def _strip_accents(s: str) -> str:
    return "".join(c for c in unicodedata.normalize("NFD", s) if unicodedata.category(c) != "Mn")


def build_restorer(pack: Path, english: Path | None):
    """Map an accentless token to its unique attested form, when there is one."""
    con = sqlite3.connect(f"file:{pack}?mode=ro", uri=True)
    candidates: dict[str, set[str]] = {}
    for (form,) in con.execute("SELECT DISTINCT form FROM wordform WHERE source != 'generated'"):
        if not form or any(ch in HU_LETTERS for ch in form) is False:
            continue
        # Case-folded: `Beszélek` and `beszélek` are one form, not two rivals.
        candidates.setdefault(_strip_accents(form).lower(), set()).add(form.lower())
    con.close()
    english_words: set[str] = set()
    if english and english.exists():
        english_words = {w.strip().lower() for w in english.read_text(errors="replace").split("\n") if w.strip()}

    def restore(token: str) -> str:
        low = token.lower()
        if len(low) < 3 or low in english_words:
            return token
        forms = candidates.get(low)
        if not forms or len(forms) != 1:
            return token
        form = next(iter(forms))
        if token[0].isupper():
            form = form[0].upper() + form[1:]
        return form

    return restore


def restore_accents(text: str, restore) -> str:
    return re.sub(r"[A-Za-z]{3,}", lambda m: restore(m.group(0)), text)


# ---------------------------------------------------------------------------
# Topic tagging and output


def topics_for(heading_chain: list[str]) -> list[str]:
    hay = " ".join(heading_chain).lower()
    found: list[str] = []
    for key, codes in TOPIC_HINTS:
        if key in hay:
            for c in codes:
                if c not in found:
                    found.append(c)
    return found[:4]


def tag_sections(md: str) -> str:
    """Insert `<!-- topics: … -->` after every heading, from the heading chain."""
    out = []
    chain: list[str] = []
    for line in md.split("\n"):
        m = re.match(r"^(#{2,4})\s+(.*)$", line)
        if m:
            level = len(m.group(1))
            chain = chain[: level - 2] + [m.group(2)]
            out.append(line)
            codes = topics_for(chain)
            if codes:
                out.append("<!-- topics: " + ", ".join(codes) + " -->")
            continue
        out.append(line)
    return "\n".join(out)


def front_matter(title: str, source: str, url: str, license_: str, note: str = "") -> str:
    fm = [
        "---",
        f"title: {title}",
        f"source: {source}",
        f"url: {url}",
        f"license: {license_}",
    ]
    if note:
        fm.append(f"note: {note}")
    fm.append("---")
    return "\n".join(fm) + "\n\n"


def build(out_dir: Path, cache_dir: Path, pack: Path | None, english: Path | None) -> None:
    out_dir.mkdir(parents=True, exist_ok=True)
    written = []

    for page, title in WIKIPEDIA:
        raw = wiki_raw("en.wikipedia.org", page, cache_dir)
        md = tag_sections(wikitext_to_md(raw))
        fn = out_dir / f"src-wikipedia-{page.lower()}.md"
        fn.write_text(
            front_matter(title, f"Wikipedia, “{title}”", f"https://en.wikipedia.org/wiki/{page}",
                         "CC BY-SA 4.0", "converted from wikitext by langpack/pipeline/tutor_kb.py")
            + "# " + title + "\n\n" + md, encoding="utf-8")
        written.append(fn)

    for title in WIKTIONARY:
        raw = wiki_raw("en.wiktionary.org", title, cache_dir)
        if raw.lstrip().upper().startswith("#REDIRECT") or len(raw) < 1000:
            print(f"  skipped {title!r}: redirect or empty")
            continue
        md = tag_sections(wikitext_to_md(raw))
        short = title.replace("Appendix:Hungarian ", "")
        slug = re.sub(r"[^a-z0-9]+", "-", short.lower()).strip("-")
        fn = out_dir / f"src-wiktionary-{slug}.md"
        url = "https://en.wiktionary.org/wiki/" + urllib.parse.quote(title.replace(" ", "_"))
        fn.write_text(
            front_matter(f"Hungarian {short}", f"Wiktionary, “{title}”", url, "CC BY-SA 4.0",
                         "converted from wikitext by langpack/pipeline/tutor_kb.py")
            + "# Hungarian " + short + "\n\n" + md, encoding="utf-8")
        written.append(fn)

    restore = build_restorer(pack, english) if pack and pack.exists() else None
    for vol, url in FSI:
        raw = fetch(url, cache_dir / "fsi" / f"{vol}.txt")
        notes = fsi_notes(raw)
        parts = []
        for unit, body in notes:
            if restore:
                body = restore_accents(body, restore)
            parts.append(f"## {unit} — grammar notes\n\n{body}")
        md = tag_sections("\n\n".join(parts))
        fn = out_dir / f"src-fsi-{vol.lower()}.md"
        fn.write_text(
            front_matter(f"FSI Hungarian Basic Course, {vol}: notes on grammar",
                         "US Foreign Service Institute, Hungarian Basic Course (1962)",
                         "https://archive.org/details/Fsi-HungarianBasicCourse-StudentText",
                         "public domain (US government work)",
                         "OCR text from archive.org; diacritics restored from the language pack where "
                         "exactly one attested form matches — other tokens are as scanned")
            + f"# FSI Hungarian Basic Course — {vol}\n\n" + md, encoding="utf-8")
        written.append(fn)

    for fn in written:
        print(f"  {fn.name}  {fn.stat().st_size:,} bytes")


def main(argv: list[str] | None = None) -> int:
    ap = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    ap.add_argument("--out", required=True, type=Path, help="backend/internal/tutor/kb")
    ap.add_argument("--cache", type=Path, default=Path("cache/tutor"), help="download cache")
    ap.add_argument("--pack", type=Path, default=None, help="language pack, for accent restoration")
    ap.add_argument("--english-words", type=Path, default=Path("/usr/share/dict/words"),
                    help="an English word list; tokens in it are never re-accented")
    args = ap.parse_args(argv)
    build(args.out, args.cache, args.pack, args.english_words)
    return 0


if __name__ == "__main__":
    sys.exit(main())
