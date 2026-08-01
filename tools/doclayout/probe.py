"""Does the PDF's own text layer carry enough typography to find headings?

Go's ledongthuc/pdf declares FontSize and never sets it. PyMuPDF wraps MuPDF and
exposes span-level size/flags/bbox. If that's populated, born-digital PDFs need no
ML model at all — the boundaries are already in the file.
"""
import sys, collections
import fitz

path = sys.argv[1]
limit = int(sys.argv[2]) if len(sys.argv) > 2 else 40

doc = fitz.open(path)
pages = min(len(doc), limit)
chars = collections.Counter()
lines = []

for i in range(pages):
    for block in doc[i].get_text("dict")["blocks"]:
        if block.get("type") != 0:      # 0 = text
            continue
        for line in block["lines"]:
            text = "".join(s["text"] for s in line["spans"]).strip()
            if not text:
                continue
            span = max(line["spans"], key=lambda s: s["size"])
            size = round(span["size"], 1)
            bold = bool(span["flags"] & 2 ** 4)
            chars[(size, bold)] += len(text)
            lines.append((size, bold, span["font"], text, i + 1))

print(f"{path} — {len(doc)} pages (sampled {pages})")
if not chars:
    print("  NO TEXT LAYER — this is a scan; only a visual model can help.")
    raise SystemExit

total = sum(chars.values())
(body_size, body_bold), _ = chars.most_common(1)[0]
print(f"\n  size/bold distribution (body = {body_size}pt bold={body_bold}):")
for (size, bold), n in sorted(chars.items(), key=lambda kv: -kv[1])[:8]:
    tag = "  <- body" if (size, bold) == (body_size, body_bold) else ""
    print(f"    {size:6.1f}pt {'bold' if bold else '    '}  {n:7d} chars  {100*n/total:5.1f}%{tag}")

heads = [l for l in lines if (l[0] > body_size + 0.5 or (l[1] and not body_bold)) and len(l[3]) < 100]
print(f"\n  heading candidates: {len(heads)} of {len(lines)} lines")
for size, bold, font, text, page in heads[:12]:
    print(f"    p{page:<4} {size:5.1f}pt {'bold' if bold else '    '}  {text[:66]}")
if not heads:
    print("    (none — typographically flat, needs drift or a visual model)")
