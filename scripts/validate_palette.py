#!/usr/bin/env python3
"""WCAG 2.1 contrast check for the design tokens in web-ui/src/assets/tailwind.css.

Reads the tokens straight out of the `:root` (light) and `.dark` blocks, so the
numbers measure what ships rather than a copy. Values may be #hex or oklch().
Text pairs need 4.5:1; graphics / UI boundaries need 3:1. Exits 1 on failure.
Pure standard library. See docs/UX.md §14.1.
"""
import math
import os
import re
import sys

CSS = os.path.join(os.path.dirname(__file__), "..", "web-ui", "src", "assets", "tailwind.css")


def parse_blocks(src):
    out = {}
    for name, sel in (("light", r":root"), ("dark", r"\.dark")):
        m = re.search(r"(?m)^" + sel + r"\s*\{", src)
        if not m:
            sys.exit(f"validate_palette: no {sel} block")
        depth, i = 1, m.end()
        while depth:
            depth += {"{": 1, "}": -1}.get(src[i], 0)
            i += 1
        body = re.sub(r"/\*.*?\*/", "", src[m.end():i - 1], flags=re.S)
        out[name] = dict(re.findall(r"(--[\w-]+)\s*:\s*([^;]+);", body))
    return out


def to_rgb(v):
    v = v.strip()
    if v.startswith("#"):
        h = v[1:]
        if len(h) == 3:
            h = "".join(c * 2 for c in h)
        return tuple(int(h[i:i + 2], 16) / 255 for i in (0, 2, 4))
    m = re.fullmatch(r"oklch\(\s*([\d.]+)\s+([\d.]+)\s+([\d.]+)\s*\)", v)
    if not m:
        raise ValueError(f"unsupported colour {v!r}")
    L, C, h = float(m[1]), float(m[2]), math.radians(float(m[3]))
    a, b = C * math.cos(h), C * math.sin(h)
    l_ = (L + 0.3963377774 * a + 0.2158037573 * b) ** 3
    m_ = (L - 0.1055613458 * a - 0.0638541728 * b) ** 3
    s_ = (L - 0.0894841775 * a - 1.2914855480 * b) ** 3
    lin = (
        4.0767416621 * l_ - 3.3077115913 * m_ + 0.2309699292 * s_,
        -1.2684380046 * l_ + 2.6097574011 * m_ - 0.3413193965 * s_,
        -0.0041960863 * l_ - 0.7034186147 * m_ + 1.7076147010 * s_,
    )
    enc = lambda x: 12.92 * x if x <= 0.0031308 else 1.055 * x ** (1 / 2.4) - 0.055
    return tuple(min(1.0, max(0.0, enc(x))) for x in lin)


def lum(rgb):
    f = lambda c: c / 12.92 if c <= 0.04045 else ((c + 0.055) / 1.055) ** 2.4
    r, g, b = (f(c) for c in rgb)
    return 0.2126 * r + 0.7152 * g + 0.0722 * b


def ratio(a, b):
    la, lb = lum(to_rgb(a)), lum(to_rgb(b))
    hi, lo = max(la, lb), min(la, lb)
    return (hi + 0.05) / (lo + 0.05)


TINTS = ["--tint-cream", "--tint-mist", "--tint-blush", "--tint-sage"]
SURF = ["--background", "--card", "--muted"]
RAMP = ["--ring-start", "--ring-mid", "--ring-end"]

# (class, minimum, foreground, [backgrounds])
PAIRS = [
    ("text", 4.5, "--foreground", SURF + TINTS),
    ("text", 4.5, "--muted-foreground", SURF + TINTS),
    ("text", 4.5, "--primary", SURF + ["--tint-cream", "--tint-mist", "--tint-sage"]),
    ("text", 4.5, "--success", SURF + ["--tint-cream", "--tint-mist", "--tint-sage"]),
    ("text", 4.5, "--warn", SURF + ["--tint-cream", "--tint-mist"]),
    ("text", 4.5, "--destructive", TINTS),
    ("text", 4.5, "--destructive", SURF),
    ("text", 4.5, "--primary-foreground", ["--primary"]),
    ("text", 4.5, "--destructive-foreground", ["--destructive"]),
    ("text", 4.5, "--accent-foreground", ["--accent"]),
    ("text", 4.5, "--foreground", ["--accent"]),
    ("text", 4.5, "--kn-known", SURF),
    ("text", 4.5, "--kn-learning", SURF),
    ("text", 4.5, "--kn-new", SURF),
    ("text", 4.5, "--kn-ignored", SURF),
    ("text", 4.5, "--kn-uncertain", SURF),
    ("text", 4.5, "--kn-error", SURF),
    ("graphic", 3.0, "--paprika", ["--background", "--card"] + TINTS),
    ("graphic", 3.0, "--primary", TINTS),
    ("graphic", 3.0, "--ring", ["--background", "--card"] + TINTS),
    ("graphic", 3.0, "--border", ["--background", "--card"]),
    ("graphic", 3.0, "--input", ["--background", "--card"]),
    ("graphic", 3.0, "--ch-cards", ["--card"]),
    ("graphic", 3.0, "--ch-reading", ["--card"]),
    ("graphic", 3.0, "--ch-listening", ["--card"]),
] + [("graphic", 3.0, r, ["--background", "--card"] + TINTS) for r in RAMP]


def main():
    toks = parse_blocks(open(CSS, encoding="utf-8").read())
    fails, mins = 0, {}
    for theme in ("light", "dark"):
        t = dict(toks["light"]) if theme == "light" else {**toks["light"], **toks["dark"]}
        print(f"\n== {theme} ==")
        print(f"{'pair':<46}{'ratio':>7}{'min':>6}")
        for cls, need, fg, bgs in PAIRS:
            for bg in bgs:
                if fg not in t or bg not in t:
                    print(f"MISSING token: {fg} or {bg}")
                    fails += 1
                    continue
                r = ratio(t[fg], t[bg])
                ok = r >= need
                fails += not ok
                key = (theme, cls)
                mins[key] = min(mins.get(key, 99), r)
                if not ok or "-v" in sys.argv:
                    print(f"{fg[2:] + ' on ' + bg[2:]:<46}{r:>7.2f}{need:>6}  {'ok' if ok else 'FAIL'}")
    print("\nminimum measured ratio per class:")
    for (theme, cls), r in sorted(mins.items()):
        print(f"  {theme:<6}{cls:<9}{r:.2f}")
    print(f"\n{'FAILED: %d pair(s)' % fails if fails else 'all pairs pass'}")
    return 1 if fails else 0


if __name__ == "__main__":
    sys.exit(main())
