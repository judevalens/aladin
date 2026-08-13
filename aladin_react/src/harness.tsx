/**
 * Visual harness for the document reader — dev only, not part of the app bundle.
 *
 * The app sits behind a sign-in, so the reader can't be eyeballed through it. `DocumentReader`
 * is presentational on purpose (no app context, no fetching), which lets this file mount it
 * against a staged PDF and real tokens and get a true picture of the chrome.
 *
 * Run: npm run dev, then open /harness.html
 */
import { createRoot } from "react-dom/client";

import "./index.css";
import { DocumentReader } from "@/modules/documents/ui/document-viewer-ui";
import type { OutlineEntry } from "@/modules/documents/hooks/use-document";

// Shaped like what segmentation actually recovers from this thesis: mostly unnumbered
// headings (depth 0) with a few numbered ones, which is the layout case worth looking at.
const OUTLINE: OutlineEntry[] = [
  { title: "Abstract", depth: 0, page: 3 },
  { title: "Acknowledgments", depth: 0, page: 5 },
  { title: "Chapter 1 Introduction", depth: 0, page: 31 },
  { title: "1.1 Motivation", depth: 1, page: 31 },
  { title: "1.2 Objective of the Study", depth: 1, page: 33 },
  { title: "Chapter 2 Literature Review", depth: 0, page: 35 },
  { title: "Chapter 3 Quantitative Trading Strategies", depth: 0, page: 39 },
  { title: "3.1 Definition", depth: 1, page: 39 },
  { title: "3.2 Trend-Based Regression", depth: 1, page: 47 },
  { title: "3.2.1 Model Specification", depth: 2, page: 47 },
  { title: "3.2.2 Parameter Selection", depth: 2, page: 51 },
  { title: "3.3 Pattern-Based Approaches", depth: 1, page: 55 },
  { title: "Chapter 4 Data and Methodology", depth: 0, page: 59 },
  { title: "Chapter 5 Empirical Analysis", depth: 0, page: 65 },
  { title: "5.1 Back-Test Results", depth: 1, page: 65 },
  { title: "5.2 Sources of Profitability", depth: 1, page: 88 },
  { title: "Chapter 6 Conclusion", depth: 0, page: 121 },
  { title: "Appendix A Detailed Correlation Analysis", depth: 0, page: 123 },
  { title: "Appendix B Detailed Performances of Strategy Back-Tests", depth: 0, page: 199 },
  { title: "Bibliography", depth: 0, page: 277 },
];

createRoot(document.getElementById("harness")!).render(
  <div className="h-screen bg-bg">
    <DocumentReader
      title="An Empirical Analysis of Quantitative Trading Strategies"
      pageCount={280}
      outline={OUTLINE}
      outlineRecovered
      url="/__harness.pdf"
      resourceLoading={false}
    />
  </div>,
);
