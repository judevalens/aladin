"""Regression net for the segmentation pass.

INGESTION_PRD §13c recorded region counts across a real corpus. A model swap, a filter
change, or a bad refactor all show up here as a class-mix that moved — which is otherwise
invisible until someone notices the chunks got worse weeks later.
"""
import json, subprocess, sys, collections, time, os

DOCS = [
    ("thesis (OCR'd scan)", "../../backend_v2/uploads/file-0b3c37dc-cdd2-46fe-9744-f46beacad4f3.pdf", 12),
    ("arXiv",               os.path.expanduser("~/Downloads/2603.00495v1.pdf"), 12),
    ("SSRN",                os.path.expanduser("~/Downloads/ssrn-6163327.pdf"), 12),
    ("ACM",                 os.path.expanduser("~/Downloads/3447772.pdf"), 12),
    ("deep hedging",        "/private/tmp/claude-501/-Users-judepaulemon-Documents-aladin/e44f0ba5-4e1c-4901-a37a-2ade930799dc/scratchpad/deep-hedging.pdf", 12),
]

print(f"{'document':<22} {'pages':>6} {'regions':>8} {'title%':>7} {'abandon%':>9}  {'ms/pg':>6}  top classes")
print("─" * 108)
for label, path, limit in DOCS:
    if not os.path.isfile(path):
        print(f"{label:<22} (missing)")
        continue
    t = time.time()
    out = subprocess.run([".venv/bin/python", "segment.py", "--pdf", path, "--max-pages", str(limit)],
                         capture_output=True, text=True)
    if out.returncode != 0:
        print(f"{label:<22} FAILED: {out.stderr.strip().splitlines()[-1][:50]}")
        continue
    d = json.loads(out.stdout)
    elapsed = (time.time() - t) * 1000
    classes = collections.Counter()
    for pg in d["pages"]:
        for r in pg["regions"]:
            classes[r["class"]] += 1
    total = sum(classes.values()) or 1
    mix = " ".join(f"{k}:{v}" for k, v in classes.most_common(4))
    print(f"{label:<22} {d['page_count']:>6} {total:>8} {100*classes['title']/total:>6.0f}% "
          f"{100*classes['abandon']/total:>8.0f}%  {elapsed/max(1,d['pages_processed']):>5.0f}  {mix}")
