import React from "react";
import ReactDOM from "react-dom/client";
import App from "./App";
import "./styles.css";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App pageId="preview-page" initialMarkdown="Start writing here." isEditable={true} bridge={undefined} />
  </React.StrictMode>,
);
