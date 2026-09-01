import jbig2 from "pdfjs-dist/wasm/jbig2.wasm?url";
import openjpeg from "pdfjs-dist/wasm/openjpeg.wasm?url";
import qcms from "pdfjs-dist/wasm/qcms_bg.wasm?url";

const decoders: Record<string, string> = {
  "jbig2.wasm": jbig2,
  "openjpeg.wasm": openjpeg,
  "qcms_bg.wasm": qcms,
};
const fonts = import.meta.glob<string>("/node_modules/pdfjs-dist/standard_fonts/*.{pfb,ttf}", { eager: true, query: "?url", import: "default" });
const cmaps = import.meta.glob<string>("/node_modules/pdfjs-dist/cmaps/*.bcmap", { eager: true, query: "?url", import: "default" });

/** Vite owns these URLs (including inline data URLs in the iPad build). A hardcoded
 * /node_modules or CDN path silently loses scanned pages in production/offline. */
export class BoardPdfBinaryDataFactory {
  async fetch({ kind, filename }: { kind: string; filename: string }): Promise<Uint8Array> {
    const url = kind === "wasmUrl" ? decoders[filename]
      : kind === "standardFontDataUrl" ? fonts[`/node_modules/pdfjs-dist/standard_fonts/${filename}`]
        : kind === "cMapUrl" ? cmaps[`/node_modules/pdfjs-dist/cmaps/${filename}`] : undefined;
    if (!url) throw new Error(`Unsupported PDF resource: ${kind}/${filename}`);
    const response = await fetch(url);
    if (!response.ok) throw new Error(`PDF decoder unavailable: ${filename}`);
    return new Uint8Array(await response.arrayBuffer());
  }
}
