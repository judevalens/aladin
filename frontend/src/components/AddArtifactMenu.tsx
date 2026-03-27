import { useRef } from "react";
import { ingestUrl } from "../lib/api";
import type { ArtifactType } from "../types";

interface Props {
  onAdd: (type: ArtifactType, label: string, content: string, sourceUrl?: string) => void;
  onMic: () => void;
  onClose: () => void;
  style?: React.CSSProperties;
}

export default function AddArtifactMenu({ onAdd, onMic, onClose, style }: Props) {
  const fileInputRef = useRef<HTMLInputElement>(null);

  const handleLink = async () => {
    const url = prompt("Paste a URL:");
    if (!url) { onClose(); return; }
    onClose();
    try {
      const result = await ingestUrl(url);
      if (result.post) {
        const { authorName, authorHandle, content, platform } = result.post;
        onAdd("link", `${authorName} (@${authorHandle}) · ${platform}`, content, url);
        return;
      }
    } catch { /* fall through */ }
    onAdd("link", url.replace(/^https?:\/\//, "").split("/")[0], url, url);
  };

  const handlePaste = async () => {
    const text = await navigator.clipboard.readText();
    if (text) onAdd("text", text.slice(0, 40) + (text.length > 40 ? "…" : ""), text);
    onClose();
  };

  const handleFile = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (file) onAdd("file", file.name, file.name);
    onClose();
  };

  const items = [
    { icon: "🎙️", label: "Record audio", onClick: () => { onMic(); onClose(); } },
    { icon: "🔗", label: "Add link",      onClick: handleLink },
    { icon: "📋", label: "Paste text",    onClick: handlePaste },
    { icon: "📎", label: "Upload file",   onClick: () => fileInputRef.current?.click() },
  ];

  return (
    <>
      <input ref={fileInputRef} type="file" className="hidden" onChange={handleFile} />
      <div
        className="z-50 min-w-[160px] rounded-lg border border-gray-200 bg-white py-1 shadow-lg"
        style={style}
      >
        {items.map((item) => (
          <button
            key={item.label}
            onClick={item.onClick}
            className="flex w-full items-center gap-2 px-3 py-2 text-sm text-gray-700 hover:bg-gray-50"
          >
            <span>{item.icon}</span>
            {item.label}
          </button>
        ))}
      </div>
    </>
  );
}
