#!/usr/bin/env python3
"""Illustration limits (ADR-059, UX.md §14.9): every web-ui/src/components/art/*.vue
compiles to an SVG under 2048 bytes once whitespace and comments are stripped,
carries aria-hidden + focusable=false, and contains no <text>, gradient or filter.
The tram strip is the one wide piece; it is held to the same 2 KB."""
import glob
import os
import re
import sys

root = os.path.join(os.path.dirname(__file__), "..", "web-ui", "src", "components", "art")
bad = 0
for f in sorted(glob.glob(os.path.join(root, "*.vue"))):
    name = os.path.basename(f)
    if name == "Illustration.vue":
        continue
    src = re.sub(r"<!--.*?-->", "", open(f, encoding="utf-8").read(), flags=re.S)
    m = re.search(r"<svg.*</svg>", src, flags=re.S)
    if not m:
        print(f"FAIL {name}: no <svg>")
        bad += 1
        continue
    svg = re.sub(r">\s+<", "><", re.sub(r"\s+", " ", m.group(0)))
    problems = []
    if len(svg.encode()) >= 2048:
        problems.append(f"{len(svg.encode())} bytes >= 2048")
    for banned in ("<text", "Gradient", "<filter", "<image"):
        if banned in svg:
            problems.append(f"contains {banned}")
    if 'aria-hidden="true"' not in svg or 'focusable="false"' not in svg:
        problems.append("missing aria-hidden/focusable")
    print(f"{'FAIL' if problems else 'ok  '} {name:<24}{len(svg.encode()):>5} B  {'; '.join(problems)}")
    bad += bool(problems)
sys.exit(1 if bad else 0)
