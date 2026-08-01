# pdftoc

Give a PDF the bookmarks it never had.

Plenty of PDFs — scanned books, exported reports, old papers — ship with no outline, so
nothing can navigate them and nothing can chunk them by section. Many of them *do* print
a table of contents on a page or two. This turns that printed page into a real PDF
outline.

**The structure is editable, on purpose.** Heading extraction is a guess and page offsets
are a guess, so the pipeline stops at a plain text file you correct by hand. Nothing
writes a PDF until you run `apply`.

```
pdftoc probe   book.pdf                        # where's the contents page? what's the offset?
pdftoc draft   book.pdf --pages 5-7 -o book.toc
$EDITOR book.toc                               # ← fix it here
pdftoc verify  book.pdf book.toc               # does each entry land on the right page?
pdftoc apply   book.pdf book.toc -o out.pdf
```

## Setup

```bash
python3 -m venv .venv
.venv/bin/pip install -r requirements.txt
```

The `draft` step calls Claude. It reads `ANTHROPIC_API_KEY` from the environment,
`--api-key`, or `backend_v2/.env`. Everything else runs offline — `--no-llm` on `draft`
just dumps the contents-page text if you'd rather structure it yourself.

## The .toc format

Indentation is the hierarchy. The last token on a line is the page **as printed in the
book**. `#` starts a comment.

```
# offset: 4          # printed arabic page 1 is PDF page 5
# roman-offset: -3   # printed page vii is PDF page 4
Preface                vii
1 Beginnings             1
    1.1 First light      2
    1.2 Second wind      3
2 Middles                4
```

Plain text rather than JSON because this file exists to be corrected by a human, and
nesting is easier to see — and fix — as indentation than as brackets.

## Page offsets are the whole difficulty

Printed page 1 is almost never PDF page 1. Books have unnumbered front matter, then a
roman-numbered preface, then an arabic-numbered body — **two different offsets in one
document**, which is why there are two directives.

- `probe` guesses the arabic offset by looking for page folios and reports its confidence.
- `verify` is the real check: it resolves every entry and tells you whether the heading
  text is actually on the page you're pointing at. A low match rate means the offset is
  wrong.

```
  ✓ p4    Preface
  ✓ p5    1 Beginnings
  ✓ p6      1.1 First light
  7/7 entries found their heading on the target page.
```

An entry that resolves outside the document is reported and skipped, never written —
`apply` refuses to run with unresolved entries unless you pass `--force`.

## Commands

| | |
|---|---|
| `probe` | Page count, existing outline, candidate contents pages, guessed offset. |
| `draft` | Contents-page text → LLM → editable `.toc`. `--no-llm` dumps raw text. |
| `dump` | An existing outline → `.toc`, for editing or re-nesting what's already there. |
| `verify` | Resolve every entry and show what's actually on that page. Writes nothing. |
| `apply` | Write the bookmarks into a new PDF. Leaves the original untouched. |

## Limits

- **Scanned contents pages need OCR first.** If `draft` finds no text, the page is an
  image — run it through `ocrmypdf` or similar, then retry.
- Titles ending in an ellipsis lose it (indistinguishable from a dot leader).
- Only bookmarks. It does not render a visible contents page into the document.

## Tests

```bash
.venv/bin/python test_pdftoc.py     # parser + page arithmetic
.venv/bin/python make_fixture.py    # regenerate sample-book.pdf
```

The fixture is a book with unnumbered front matter, a roman preface, a printed contents
page, and an arabic body — the dual-offset case, which is where naive tools go wrong.
