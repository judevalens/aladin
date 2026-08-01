"""
Tests for the fragile parts: the hand-editable format's parser, and the printed->PDF
page arithmetic. Everything else is I/O around these two.

Run: .venv/bin/python test_pdftoc.py
"""

import tempfile
from pathlib import Path

from pdftoc import (
    TocDoc,
    TocEntry,
    parse_page_token,
    parse_ranges,
    parse_roman,
    read_toc_file,
    resolve_indices,
    write_toc_file,
)

failures: list[str] = []


def check(name: str, got, want):
    if got != want:
        failures.append(f"{name}\n    got  {got!r}\n    want {want!r}")


def parse(text: str) -> TocDoc:
    with tempfile.NamedTemporaryFile("w", suffix=".toc", delete=False) as handle:
        handle.write(text)
        path = Path(handle.name)
    try:
        return read_toc_file(path)
    finally:
        path.unlink()


# ── roman numerals ────────────────────────────────────────────────────────────
check("roman iv", parse_roman("iv"), 4)
check("roman xiv", parse_roman("xiv"), 14)
check("roman XLII", parse_roman("XLII"), 42)
check("roman rejects word", parse_roman("chapter"), None)
# "mix" is all-roman-letters and would parse as 1009 — a real hazard for titles that
# lose their page number. Documented, not fixed: the page token is positional.
check("roman empty", parse_roman(""), None)

check("page token arabic", parse_page_token("21"), ("arabic", 21))
check("page token roman", parse_page_token("vii"), ("roman", 7))
check("page token dotted", parse_page_token("...12"), ("arabic", 12))
check("page token junk", parse_page_token("hello"), None)

check("ranges", parse_ranges("5-7,9"), [5, 6, 7, 9])
check("ranges single", parse_ranges("3"), [3])


# ── the editable format ───────────────────────────────────────────────────────
doc = parse(
    """
# offset: 4
# roman-offset: -3
Preface                vii
1 Beginnings           1
    1.1 First light    2
        1.1.1 Deeper   2
2 Middles              4
"""
)
check("entry count", len(doc.entries), 5)
check("offset directive", doc.offset, 4)
check("roman directive", doc.roman_offset, -3)
check("levels", [e.level for e in doc.entries], [0, 0, 1, 2, 0])
check("titles", [e.title for e in doc.entries][:2], ["Preface", "1 Beginnings"])
check("printed pages", [e.printed for e in doc.entries], ["vii", "1", "2", "2", "4"])

# Dot leaders are how printed TOCs actually look; the page must still be found.
doc = parse("Chapter 1 . . . . . . . 12\nChapter 2 ............. 30\n")
check("dot leaders (spaced)", doc.entries[0].printed, "12")
check("dot leaders (solid)", doc.entries[1].printed, "30")
check("dot leaders keep title", doc.entries[0].title, "Chapter 1")

# A human editing this will leave blank lines and stray comments.
doc = parse("\n\n# a note\nOnly entry   7\n\n")
check("tolerates noise", len(doc.entries), 1)

# An entry with no page number must survive parsing and be reported later, not dropped
# silently — losing a heading without telling anyone is the worst failure mode.
doc = parse("Front matter\nChapter 1   1\n")
check("keeps page-less entry", len(doc.entries), 2)
check("page-less printed is None", doc.entries[0].printed, None)

# Tabs are what you get from copy-paste; treat one as a level.
doc = parse("Top   1\n\tChild   2\n")
check("tab indent", [e.level for e in doc.entries], [0, 1])


# ── page arithmetic ───────────────────────────────────────────────────────────
doc = TocDoc(
    entries=[
        TocEntry("Preface", 0, "vii"),
        TocEntry("Chapter 1", 0, "1"),
        TocEntry("Chapter 3", 0, "6"),
    ],
    offset=4,
    roman_offset=-3,
)
problems = resolve_indices(doc, page_count=10)
check("no problems", problems, [])
check("roman resolves", doc.entries[0].pdf_index, 3)  # printed vii -> PDF page 4
check("arabic resolves", doc.entries[1].pdf_index, 4)  # printed 1  -> PDF page 5
check("last resolves", doc.entries[2].pdf_index, 9)  # printed 6  -> PDF page 10

# The whole point of `verify`: a wrong offset must be reported, not written.
doc = TocDoc(entries=[TocEntry("Preface", 0, "vii")], offset=4)  # no roman offset
problems = resolve_indices(doc, page_count=10)
check("out-of-range is reported", len(problems), 1)
check("out-of-range leaves index unset", doc.entries[0].pdf_index, None)
check("problem names the page", "PDF page 11" in problems[0], True)

doc = TocDoc(entries=[TocEntry("Nameless", 0, None)], offset=0)
problems = resolve_indices(doc, page_count=10)
check("missing page reported", len(problems), 1)


# ── write -> read round trip ──────────────────────────────────────────────────
original = TocDoc(
    entries=[
        TocEntry("Preface", 0, "vii"),
        TocEntry("1 Beginnings", 0, "1"),
        TocEntry("1.1 First light", 1, "2"),
        TocEntry("1.1.1 Deeper still", 2, "2"),
    ],
    offset=4,
    roman_offset=-3,
)
with tempfile.NamedTemporaryFile("w", suffix=".toc", delete=False) as handle:
    round_path = Path(handle.name)
write_toc_file(round_path, original)
reparsed = read_toc_file(round_path)
round_path.unlink()

check("round-trip count", len(reparsed.entries), 4)
check("round-trip levels", [e.level for e in reparsed.entries], [0, 0, 1, 2])
check("round-trip titles", [e.title for e in reparsed.entries], [e.title for e in original.entries])
check("round-trip pages", [e.printed for e in reparsed.entries], ["vii", "1", "2", "2"])
check("round-trip offset", reparsed.offset, 4)
check("round-trip roman offset", reparsed.roman_offset, -3)


if failures:
    print(f"{len(failures)} FAILED\n")
    for failure in failures:
        print(f"  {failure}\n")
    raise SystemExit(1)
print("all passed")
