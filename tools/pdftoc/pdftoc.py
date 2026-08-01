#!/usr/bin/env python3
"""
pdftoc — give a PDF the bookmarks it never had.

The problem: plenty of PDFs (scanned books, exported reports, old papers) have no
outline at all, so nothing can navigate them and nothing can chunk them by section.
Many of them DO print a table of contents on a page or two. This turns that printed
page into a real PDF outline.

The pipeline is four explicit steps, and the middle one is a plain text file you edit
by hand. That is the point: heading extraction is a guess, page offsets are a guess,
and a tool that writes guesses straight into your PDF is worse than useless.

    pdftoc probe   book.pdf                       # where's the TOC? what's the offset?
    pdftoc draft   book.pdf --pages 5-7 -o book.toc
    $EDITOR book.toc                              # <- fix it here
    pdftoc verify  book.pdf book.toc              # does entry N land on the right page?
    pdftoc apply   book.pdf book.toc -o out.pdf

Nothing before `apply` writes a PDF.
"""

from __future__ import annotations

import argparse
import json
import os
import re
import sys
from dataclasses import dataclass, field
from pathlib import Path

import pdfplumber
from pypdf import PdfReader, PdfWriter

INDENT_UNIT = 4  # spaces per nesting level in the .toc file


# ── the editable format ───────────────────────────────────────────────────────
#
# Indentation is the hierarchy. The last whitespace-separated token is the printed
# page (arabic or roman). Everything else is the title. `#` starts a comment.
#
#   # offset: 24
#   Preface                    vii
#   Chapter 1  Introduction      1
#       1.1  Motivation          3
#
# Chosen over JSON deliberately: this file exists to be corrected by a human at 1am,
# and nesting is far easier to see (and fix) as indentation than as brackets.


@dataclass
class TocEntry:
    title: str
    level: int  # 0 = top level
    printed: str | None  # page as printed in the book ("21", "vii"), None if absent
    source_line: int = 0  # line number in the .toc file, for error messages
    pdf_index: int | None = field(default=None)  # 0-based, resolved at verify/apply


@dataclass
class TocDoc:
    entries: list[TocEntry]
    offset: int = 0  # printed arabic page 1 -> pdf_index (offset + 0)
    roman_offset: int | None = None  # printed roman i -> pdf_index (roman_offset + 0)


ROMAN_VALUES = {"i": 1, "v": 5, "x": 10, "l": 50, "c": 100, "d": 500, "m": 1000}


def parse_roman(token: str) -> int | None:
    s = token.lower().strip()
    if not s or any(c not in ROMAN_VALUES for c in s):
        return None
    total, prev = 0, 0
    for char in reversed(s):
        value = ROMAN_VALUES[char]
        total = total - value if value < prev else total + value
        prev = max(prev, value)
    return total or None


def parse_page_token(token: str) -> tuple[str, int] | None:
    """Returns (kind, number) where kind is 'arabic' or 'roman'."""
    cleaned = token.strip().strip(".·—–-")
    if not cleaned:
        return None
    if cleaned.isdigit():
        return ("arabic", int(cleaned))
    roman = parse_roman(cleaned)
    return ("roman", roman) if roman else None


def parse_ranges(spec: str) -> list[int]:
    """'5-7,9' -> [5,6,7,9] as 1-based page numbers."""
    out: list[int] = []
    for chunk in spec.split(","):
        chunk = chunk.strip()
        if not chunk:
            continue
        if "-" in chunk:
            lo, hi = chunk.split("-", 1)
            out.extend(range(int(lo), int(hi) + 1))
        else:
            out.append(int(chunk))
    return out


def read_toc_file(path: Path) -> TocDoc:
    doc = TocDoc(entries=[])
    indent_unit = INDENT_UNIT

    for lineno, raw in enumerate(path.read_text().splitlines(), start=1):
        if not raw.strip():
            continue
        if raw.lstrip().startswith("#"):
            directive = raw.lstrip()[1:].strip()
            if match := re.match(r"offset\s*:\s*(-?\d+)", directive, re.I):
                doc.offset = int(match.group(1))
            elif match := re.match(r"roman[-_]offset\s*:\s*(-?\d+)", directive, re.I):
                doc.roman_offset = int(match.group(1))
            elif match := re.match(r"indent\s*:\s*(\d+)", directive, re.I):
                indent_unit = max(1, int(match.group(1)))
            continue

        expanded = raw.replace("\t", " " * indent_unit)
        indent = len(expanded) - len(expanded.lstrip(" "))
        level = indent // indent_unit
        body = expanded.strip()

        # Dot leaders are how printed contents pages look, and they come in both
        # "Chapter 1 ....... 12" and "Chapter 1 . . . . . 12" flavours — the spaced
        # form is what pdfplumber usually gives you. Require a run of 3+ so ordinary
        # punctuation ("1.1", "e.g.", "Ph.D.") is left alone.
        body = re.sub(r"(?:[.…·]\s*){3,}", " ", body).strip(" .·…")

        printed = None
        parts = body.rsplit(None, 1)
        if len(parts) == 2 and parse_page_token(parts[1]):
            body, printed = parts[0].strip(), parts[1].strip()

        if not body:
            continue
        doc.entries.append(TocEntry(title=body, level=level, printed=printed, source_line=lineno))

    return doc


def write_toc_file(path: Path, doc: TocDoc, note: str = "") -> None:
    lines = [
        "# pdftoc — edit this file, then run `pdftoc verify` and `pdftoc apply`.",
        "# Indentation is the hierarchy (4 spaces per level). Last token is the",
        "# page as PRINTED in the book. Lines starting with # are ignored.",
        f"# offset: {doc.offset}    # printed arabic page 1 -> this many PDF pages in",
    ]
    if doc.roman_offset is not None:
        lines.append(f"# roman-offset: {doc.roman_offset}    # printed page i -> PDF page {doc.roman_offset + 1}")
    if note:
        lines.append(f"# {note}")
    lines.append("")

    width = max((len(" " * (e.level * INDENT_UNIT) + e.title) for e in doc.entries), default=0)
    for entry in doc.entries:
        prefix = " " * (entry.level * INDENT_UNIT) + entry.title
        lines.append(f"{prefix.ljust(width + 2)}{entry.printed or ''}".rstrip())
    path.write_text("\n".join(lines) + "\n")


# ── page-number resolution ────────────────────────────────────────────────────


def resolve_indices(doc: TocDoc, page_count: int) -> list[str]:
    """Fills entry.pdf_index. Returns human-readable problems (never raises)."""
    problems: list[str] = []
    for entry in doc.entries:
        if entry.printed is None:
            problems.append(f"line {entry.source_line}: {entry.title!r} has no page number")
            continue
        parsed = parse_page_token(entry.printed)
        if not parsed:
            problems.append(f"line {entry.source_line}: can't read page {entry.printed!r}")
            continue
        kind, number = parsed
        base = doc.roman_offset if kind == "roman" and doc.roman_offset is not None else doc.offset
        index = number + base - 1
        if not 0 <= index < page_count:
            problems.append(
                f"line {entry.source_line}: {entry.title!r} -> PDF page {index + 1}, "
                f"outside 1..{page_count} (offset wrong?)"
            )
            continue
        entry.pdf_index = index
    return problems


def guess_offset(pdf_path: Path, sample: int = 40) -> tuple[int | None, list[str]]:
    """
    Infer the printed->PDF offset by finding page folios.

    Looks at the top and bottom of each sampled page for a bare number and takes the
    most common (pdf_index - printed) difference. Crude, but it turns "guess and check"
    into "check", and `verify` is there for when it's wrong.
    """
    votes: dict[int, int] = {}
    notes: list[str] = []
    with pdfplumber.open(str(pdf_path)) as pdf:
        pages = pdf.pages[:sample]
        for idx, page in enumerate(pages):
            text = page.extract_text() or ""
            lines = [ln.strip() for ln in text.splitlines() if ln.strip()]
            if not lines:
                continue
            for candidate in (lines[0], lines[-1]):
                token = candidate.strip()
                if token.isdigit() and len(token) <= 4:
                    votes[idx - int(token) + 1] = votes.get(idx - int(token) + 1, 0) + 1
    if not votes:
        notes.append("no page folios found — set `# offset:` by hand")
        return None, notes
    best = max(votes.items(), key=lambda kv: kv[1])
    notes.append(f"offset {best[0]} seen on {best[1]} page(s)")
    return best[0], notes


def find_toc_pages(pdf_path: Path, sample: int = 30) -> list[tuple[int, int]]:
    """Candidate TOC pages as (1-based page, score) — lines that look like entries."""
    hits: list[tuple[int, int]] = []
    entry_re = re.compile(r"(\.{3,}|…)\s*\d+\s*$|\s{3,}\d+\s*$")
    with pdfplumber.open(str(pdf_path)) as pdf:
        for idx, page in enumerate(pdf.pages[:sample]):
            text = page.extract_text() or ""
            score = sum(1 for line in text.splitlines() if entry_re.search(line))
            header = text[:200].lower()
            if any(word in header for word in ("contents", "table of contents", "index")):
                score += 5
            if score >= 3:
                hits.append((idx + 1, score))
    return sorted(hits, key=lambda h: -h[1])


def extract_pages_text(pdf_path: Path, pages: list[int]) -> str:
    chunks = []
    with pdfplumber.open(str(pdf_path)) as pdf:
        for page_no in pages:
            if not 1 <= page_no <= len(pdf.pages):
                continue
            text = pdf.pages[page_no - 1].extract_text() or ""
            chunks.append(f"--- PDF page {page_no} ---\n{text}")
    return "\n\n".join(chunks)


# ── the LLM step ──────────────────────────────────────────────────────────────


def load_api_key(explicit: str | None) -> str | None:
    if explicit:
        return explicit
    if key := os.environ.get("ANTHROPIC_API_KEY"):
        return key
    # Convenience for this repo: keys live in backend_v2/.env, not the shell.
    for candidate in (Path("backend_v2/.env"), Path(__file__).resolve().parents[2] / "backend_v2/.env"):
        if candidate.is_file():
            for line in candidate.read_text().splitlines():
                if line.startswith("ANTHROPIC_API_KEY="):
                    return line.split("=", 1)[1].strip().strip("'\"")
    return None


PROMPT = """\
Below is the raw text of the table-of-contents page(s) of a book, extracted from a PDF.
Turn it into a structured outline.

Rules:
- One object per contents entry, in the order they appear.
- `title`: the heading text, cleaned of dot leaders and stray whitespace. Keep the
  chapter/section numbering that's part of the heading (e.g. "1.2 Prior work").
- `level`: 0 for top-level entries (parts, chapters), 1 for their children, and so on.
  Infer from numbering depth and indentation.
- `page`: the page number exactly as PRINTED (so "vii" stays "vii", "21" stays "21").
  Use null if an entry genuinely has no page number.
- Do NOT invent entries, and do NOT include the words "Contents"/"Table of Contents"
  themselves as an entry.
- If the text is garbled, prefer omitting an entry over guessing at it.

Return ONLY a JSON array, no prose:
[{"title": "...", "level": 0, "page": "1"}]

TEXT:
"""


def llm_draft(text: str, api_key: str, model: str) -> list[dict]:
    import anthropic

    client = anthropic.Anthropic(api_key=api_key)
    message = client.messages.create(
        model=model,
        max_tokens=8000,
        messages=[{"role": "user", "content": PROMPT + text}],
    )
    body = "".join(block.text for block in message.content if block.type == "text").strip()
    if body.startswith("```"):
        body = re.sub(r"^```[a-z]*\n?|```$", "", body, flags=re.M).strip()
    start, end = body.find("["), body.rfind("]")
    if start == -1 or end == -1:
        raise SystemExit(f"model did not return JSON:\n{body[:500]}")
    return json.loads(body[start : end + 1])


# ── commands ──────────────────────────────────────────────────────────────────


def cmd_probe(args) -> int:
    pdf_path = Path(args.pdf)
    reader = PdfReader(str(pdf_path))
    print(f"{pdf_path.name}: {len(reader.pages)} pages")

    try:
        existing = reader.outline
    except Exception:
        existing = []
    if existing:
        print(f"\nAlready has an outline ({_count_outline(existing)} entries).")
        print("  → `pdftoc dump` would give you it as an editable .toc file.")
    else:
        print("\nNo outline. That's what this tool is for.")

    print("\nCandidate contents pages:")
    candidates = find_toc_pages(pdf_path, sample=args.sample)
    if candidates:
        for page_no, score in candidates[:8]:
            print(f"  page {page_no:>4}   (score {score})")
        best = ",".join(str(p) for p, _ in candidates[:3])
        print(f"\n  → pdftoc draft {pdf_path.name} --pages {best} -o {pdf_path.stem}.toc")
    else:
        print("  none found — the contents page may be scanned images (needs OCR),")
        print("  or laid out without dot leaders. Pass --pages by hand.")

    offset, notes = guess_offset(pdf_path)
    print("\nPage offset:")
    for note in notes:
        print(f"  {note}")
    if offset is not None:
        print(f"  → printed page 1 looks like PDF page {offset + 1}")
    return 0


def _count_outline(items) -> int:
    total = 0
    for item in items:
        if isinstance(item, list):
            total += _count_outline(item)
        else:
            total += 1
    return total


def cmd_draft(args) -> int:
    pdf_path = Path(args.pdf)
    pages = parse_ranges(args.pages)
    text = extract_pages_text(pdf_path, pages)
    if not text.strip():
        print(f"No extractable text on page(s) {args.pages}.", file=sys.stderr)
        print("If the page is a scan, OCR it first (see README).", file=sys.stderr)
        return 1

    out_path = Path(args.output or f"{pdf_path.stem}.toc")

    if args.no_llm:
        out_path.with_suffix(".txt").write_text(text)
        print(f"Wrote raw text to {out_path.with_suffix('.txt')} (--no-llm).")
        return 0

    api_key = load_api_key(args.api_key)
    if not api_key:
        print("No ANTHROPIC_API_KEY (env, --api-key, or backend_v2/.env).", file=sys.stderr)
        print("Re-run with --no-llm to dump the raw text instead.", file=sys.stderr)
        return 1

    raw = llm_draft(text, api_key, args.model)
    offset, _ = guess_offset(pdf_path)
    doc = TocDoc(
        entries=[
            TocEntry(
                title=str(item.get("title", "")).strip(),
                level=max(0, int(item.get("level", 0) or 0)),
                printed=(str(item["page"]).strip() if item.get("page") not in (None, "") else None),
            )
            for item in raw
            if str(item.get("title", "")).strip()
        ],
        offset=offset if offset is not None else 0,
    )
    write_toc_file(out_path, doc, note=f"drafted from PDF page(s) {args.pages} by {args.model}")
    print(f"Wrote {len(doc.entries)} entries to {out_path}")
    print(f"  → review it, then: pdftoc verify {pdf_path.name} {out_path}")
    return 0


def cmd_dump(args) -> int:
    """Existing outline -> editable .toc (round-trip, and a starting point)."""
    pdf_path = Path(args.pdf)
    reader = PdfReader(str(pdf_path))
    entries: list[TocEntry] = []

    def walk(items, level: int):
        for item in items:
            if isinstance(item, list):
                walk(item, level + 1)
                continue
            try:
                index = reader.get_destination_page_number(item)
            except Exception:
                index = None
            entries.append(
                TocEntry(title=str(item.title).strip(), level=level, printed=str(index + 1) if index is not None else None)
            )

    walk(reader.outline, 0)
    if not entries:
        print("No outline to dump.", file=sys.stderr)
        return 1
    out_path = Path(args.output or f"{pdf_path.stem}.toc")
    # These pages are already PDF indices, so the offset is zero by construction.
    write_toc_file(out_path, TocDoc(entries=entries, offset=1), note="dumped from the existing outline (pages are PDF pages)")
    print(f"Wrote {len(entries)} entries to {out_path}")
    return 0


def cmd_verify(args) -> int:
    pdf_path, toc_path = Path(args.pdf), Path(args.toc)
    doc = read_toc_file(toc_path)
    reader = PdfReader(str(pdf_path))
    problems = resolve_indices(doc, len(reader.pages))

    print(f"{len(doc.entries)} entries, offset {doc.offset}\n")
    matched = 0
    with pdfplumber.open(str(pdf_path)) as pdf:
        for entry in doc.entries:
            if entry.pdf_index is None:
                print(f"  {'?':>5}  {'  ' * entry.level}{entry.title}   ← unresolved")
                continue
            text = pdf.pages[entry.pdf_index].extract_text() or ""
            first = next((ln.strip() for ln in text.splitlines() if ln.strip()), "(no text — scanned page?)")
            # The check that matters: does the heading actually appear where we say?
            hit = _looks_like(entry.title, text)
            matched += hit
            print(f"  {'✓' if hit else '·'} p{entry.pdf_index + 1:<4} {'  ' * entry.level}{entry.title}")
            if not hit:
                print(f"          {'  ' * entry.level}lands on: {first[:70]}")

    print(f"\n{matched}/{len(doc.entries)} entries found their heading on the target page.")
    if matched < len(doc.entries) * 0.6:
        print("Low match rate — the offset is probably wrong. Adjust `# offset:` and re-run.")
    for problem in problems:
        print(f"  ! {problem}")
    return 0


def _looks_like(title: str, page_text: str) -> bool:
    """Fuzzy: are most of the title's distinctive words on this page?"""
    words = [w for w in re.findall(r"[A-Za-z]{4,}", title.lower())]
    if not words:
        return False
    haystack = page_text.lower()
    return sum(1 for w in words if w in haystack) >= max(1, len(words) // 2)


def cmd_apply(args) -> int:
    pdf_path, toc_path = Path(args.pdf), Path(args.toc)
    doc = read_toc_file(toc_path)
    reader = PdfReader(str(pdf_path))
    problems = resolve_indices(doc, len(reader.pages))

    usable = [e for e in doc.entries if e.pdf_index is not None]
    if not usable:
        print("Nothing to write — no entry resolved to a page.", file=sys.stderr)
        for problem in problems:
            print(f"  ! {problem}", file=sys.stderr)
        return 1
    if problems and not args.force:
        print(f"{len(problems)} unresolved entr{'y' if len(problems) == 1 else 'ies'}:", file=sys.stderr)
        for problem in problems[:10]:
            print(f"  ! {problem}", file=sys.stderr)
        print("Fix them, or pass --force to write the rest anyway.", file=sys.stderr)
        return 1

    writer = PdfWriter()
    writer.append(reader)  # carries pages + existing metadata

    parents: dict[int, object] = {}
    for entry in usable:
        parent = parents.get(entry.level - 1) if entry.level > 0 else None
        ref = writer.add_outline_item(entry.title, entry.pdf_index, parent=parent)
        parents[entry.level] = ref
        # A deeper level from a previous branch must not adopt this entry's children.
        for deeper in [lvl for lvl in parents if lvl > entry.level]:
            parents.pop(deeper, None)

    out_path = Path(args.output or pdf_path.with_name(f"{pdf_path.stem}-toc.pdf"))
    with out_path.open("wb") as handle:
        writer.write(handle)
    print(f"Wrote {len(usable)} bookmarks -> {out_path}")
    if problems:
        print(f"  ({len(problems)} entries skipped)")
    return 0


def main() -> int:
    parser = argparse.ArgumentParser(prog="pdftoc", description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = parser.add_subparsers(dest="command", required=True)

    p = sub.add_parser("probe", help="find the contents page and the page offset")
    p.add_argument("pdf")
    p.add_argument("--sample", type=int, default=30, help="pages to scan (default 30)")
    p.set_defaults(func=cmd_probe)

    p = sub.add_parser("draft", help="contents page -> editable .toc via an LLM")
    p.add_argument("pdf")
    p.add_argument("--pages", required=True, help="contents pages, e.g. 5-7 or 5,6,9")
    p.add_argument("-o", "--output")
    p.add_argument("--model", default="claude-sonnet-5")
    p.add_argument("--api-key")
    p.add_argument("--no-llm", action="store_true", help="just dump the raw text")
    p.set_defaults(func=cmd_draft)

    p = sub.add_parser("dump", help="existing outline -> editable .toc")
    p.add_argument("pdf")
    p.add_argument("-o", "--output")
    p.set_defaults(func=cmd_dump)

    p = sub.add_parser("verify", help="show where each entry actually lands")
    p.add_argument("pdf")
    p.add_argument("toc")
    p.set_defaults(func=cmd_verify)

    p = sub.add_parser("apply", help="write the bookmarks into a new PDF")
    p.add_argument("pdf")
    p.add_argument("toc")
    p.add_argument("-o", "--output")
    p.add_argument("--force", action="store_true", help="write even with unresolved entries")
    p.set_defaults(func=cmd_apply)

    args = parser.parse_args()
    return args.func(args)


if __name__ == "__main__":
    sys.exit(main())
