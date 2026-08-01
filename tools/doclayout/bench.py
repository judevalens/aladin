"""CPU vs MPS vs batched, on real pages from the real document."""
import sys, time, statistics
import fitz
from huggingface_hub import hf_hub_download
from doclayout_yolo import YOLOv10

PDF = "../../backend_v2/uploads/file-0b3c37dc-cdd2-46fe-9744-f46beacad4f3.pdf"
N = 12

doc = fitz.open(PDF)
paths = []
t0 = time.time()
for i in range(N):
    p = f"/tmp/bench{i}.png"
    doc[i + 20].get_pixmap(dpi=144).save(p)
    paths.append(p)
render_ms = (time.time() - t0) * 1000 / N
print(f"render (PyMuPDF, 144dpi):     {render_ms:6.1f} ms/page\n")

weights = hf_hub_download("juliozhao/DocLayout-YOLO-DocStructBench",
                          "doclayout_yolo_docstructbench_imgsz1024.pt")

def bench(device, batch):
    model = YOLOv10(weights)
    # warm up — first call pays lazy init and, on MPS, shader compilation
    model.predict(paths[0], imgsz=1024, conf=0.3, device=device, verbose=False)
    times = []
    if batch == 1:
        for p in paths:
            t = time.time()
            model.predict(p, imgsz=1024, conf=0.3, device=device, verbose=False)
            times.append((time.time() - t) * 1000)
    else:
        for i in range(0, len(paths), batch):
            chunk = paths[i:i + batch]
            t = time.time()
            model.predict(chunk, imgsz=1024, conf=0.3, device=device, verbose=False)
            times.append((time.time() - t) * 1000 / len(chunk))
    return statistics.median(times)

for device, batch, label in [("cpu", 1, "cpu   single"),
                             ("mps", 1, "mps   single"),
                             ("mps", 4, "mps   batch 4"),
                             ("mps", 8, "mps   batch 8"),
                             ("cpu", 8, "cpu   batch 8")]:
    try:
        ms = bench(device, batch)
        pages = 280
        print(f"{label:<14} {ms:6.1f} ms/page   →  {pages} pages in {(ms+render_ms)*pages/1000:5.1f}s")
    except Exception as e:
        print(f"{label:<14} failed: {str(e)[:70]}")
