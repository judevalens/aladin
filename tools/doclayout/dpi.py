"""We render at 144dpi then the model resizes to 1024. Is the oversample paying for itself?"""
import time, statistics
import fitz
from huggingface_hub import hf_hub_download
from doclayout_yolo import YOLOv10

PDF = "../../backend_v2/uploads/file-0b3c37dc-cdd2-46fe-9744-f46beacad4f3.pdf"
doc = fitz.open(PDF)
weights = hf_hub_download("juliozhao/DocLayout-YOLO-DocStructBench",
                          "doclayout_yolo_docstructbench_imgsz1024.pt")
model = YOLOv10(weights)
model.predict("/tmp/bench0.png", imgsz=1024, conf=0.3, device="mps", verbose=False)

for dpi in (96, 110, 144, 200):
    rt, it, boxes = [], [], []
    for i in range(8):
        page = doc[i + 20]
        t = time.time()
        pix = page.get_pixmap(dpi=dpi)
        p = f"/tmp/dpi{dpi}_{i}.png"
        pix.save(p)
        rt.append((time.time() - t) * 1000)
        t = time.time()
        res = model.predict(p, imgsz=1024, conf=0.3, device="mps", verbose=False)[0]
        it.append((time.time() - t) * 1000)
        boxes.append(len(res.boxes))
    r, inf = statistics.median(rt), statistics.median(it)
    print(f"{dpi:>4}dpi  {pix.width:>5}x{pix.height:<5}  render {r:6.1f}  infer {inf:6.1f}  "
          f"total {r+inf:6.1f} ms/page   regions/page {statistics.median(boxes):.1f}   "
          f"→ 280pp in {(r+inf)*280/1000:5.1f}s")
