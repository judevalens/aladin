"""Layout segmentation on the pages a PDF already contains.

For an OCR'd scan the page image is IN the file, so there's nothing to rasterize —
and the OCR text layer already has the words. The model's only job is to say where
the regions are; text is mapped in afterwards by bounding box.
"""
import sys, time
import fitz
from huggingface_hub import hf_hub_download
from doclayout_yolo import YOLOv10

path, pages = sys.argv[1], [int(p) for p in sys.argv[2].split(",")]

t0 = time.time()
weights = hf_hub_download("juliozhao/DocLayout-YOLO-DocStructBench",
                          "doclayout_yolo_docstructbench_imgsz1024.pt")
model = YOLOv10(weights)
print(f"model loaded in {time.time()-t0:.1f}s  (paid ONCE per document, not per page)\n")

doc = fitz.open(path)
for pno in pages:
    page = doc[pno - 1]
    # 144 dpi is plenty for layout; we are finding boxes, not reading glyphs.
    pix = page.get_pixmap(dpi=144)
    img = f"/tmp/page{pno}.png"
    pix.save(img)

    t = time.time()
    res = model.predict(img, imgsz=1024, conf=0.25, device="cpu", verbose=False)[0]
    elapsed = (time.time() - t) * 1000

    scale = page.rect.width / pix.width
    print(f"page {pno}: {len(res.boxes)} regions in {elapsed:.0f}ms")
    rows = []
    for b in res.boxes:
        label = res.names[int(b.cls)]
        x0, y0, x1, y1 = [float(v) * scale for v in b.xyxy[0]]
        # Pull the OCR text that already lives inside this box.
        text = page.get_textbox(fitz.Rect(x0, y0, x1, y1)).strip().replace("\n", " ")
        rows.append((y0, label, float(b.conf), text))
    for y0, label, conf, text in sorted(rows):
        print(f"   {label:<18} {conf:.2f}  {text[:64]}")
    print()
