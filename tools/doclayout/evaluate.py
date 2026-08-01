"""Do the layout classes hold up across a real corpus, or only on one thesis?

Reports per document: whether it's born-digital or an OCR'd scan, the class mix the
model finds, and the actual `title` regions — because a heading detector that finds
nothing, or finds everything, is equally useless.
"""
import sys, time, collections, statistics
import fitz
from huggingface_hub import hf_hub_download
from doclayout_yolo import YOLOv10

weights = hf_hub_download("juliozhao/DocLayout-YOLO-DocStructBench",
                          "doclayout_yolo_docstructbench_imgsz1024.pt")
model = YOLOv10(weights)
SAMPLE = 12

for path in sys.argv[1:]:
    try:
        doc = fitz.open(path)
    except Exception as e:
        print(f"\n{path.split('/')[-1]}: cannot open — {e}\n"); continue

    prod = (doc.metadata.get("producer") or "").strip()
    n = len(doc)
    # A scan = the page is one big image with an OCR'd text layer over it.
    p0 = doc[min(3, n - 1)]
    def area(r):
        return abs((r.x1 - r.x0) * (r.y1 - r.y0))
    big_image = any(area(fitz.Rect(p0.get_image_bbox(i))) > area(p0.rect) * 0.6
                    for i in p0.get_images(full=True)[:3]) if p0.get_images() else False
    kind = "OCR'd scan" if big_image else "born-digital"

    step = max(1, n // SAMPLE)
    idxs = list(range(0, n, step))[:SAMPLE]
    classes, titles, times = collections.Counter(), [], []
    for i in idxs:
        page = doc[i]
        pix = page.get_pixmap(dpi=144)
        img = "/tmp/eval.png"; pix.save(img)
        t = time.time()
        res = model.predict(img, imgsz=1024, conf=0.3, device="mps", verbose=False)[0]
        times.append((time.time() - t) * 1000)
        scale = page.rect.width / pix.width
        for b in res.boxes:
            label = res.names[int(b.cls)]
            classes[label] += 1
            if label == "title":
                x0, y0, x1, y1 = [float(v) * scale for v in b.xyxy[0]]
                txt = page.get_textbox(fitz.Rect(x0, y0, x1, y1)).strip().replace("\n", " ")
                if txt:
                    titles.append((i + 1, txt[:62]))

    total = sum(classes.values())
    print(f"\n{'─'*78}\n{path.split('/')[-1]}")
    print(f"  {n} pages · {kind} · producer: {prod[:44] or '(none)'}")
    print(f"  {statistics.median(times):.0f} ms/page · {total} regions over {len(idxs)} sampled pages "
          f"({total/len(idxs):.1f}/page)")
    mix = "  ".join(f"{k}:{v}" for k, v in classes.most_common(6))
    print(f"  classes: {mix}")
    ratio = 100 * classes['title'] / total if total else 0
    print(f"  titles: {classes['title']} ({ratio:.0f}% of regions)")
    for page, txt in titles[:6]:
        print(f"     p{page:<4} {txt}")
    if not titles:
        print("     (no title regions in the sample)")
