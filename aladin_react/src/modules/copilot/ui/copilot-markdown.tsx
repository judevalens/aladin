import { memo } from "react";
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { cn } from "@/lib/utils";

/**
 * Renders assistant markdown with Aladin tokens. react-markdown does not render raw HTML by
 * default (no rehype-raw), so this is safe against injected markup. Kept compact for the dock.
 */
export const CopilotMarkdown = memo(function CopilotMarkdown({ text }: { text: string }) {
  return (
    <div className="text-body leading-relaxed text-ink-2">
      <Markdown
        remarkPlugins={[remarkGfm]}
        components={{
          p: ({ children }) => <p className="mb-2 last:mb-0">{children}</p>,
          strong: ({ children }) => <strong className="font-semibold text-ink">{children}</strong>,
          em: ({ children }) => <em className="italic">{children}</em>,
          a: ({ href, children }) => (
            <a
              href={href}
              target="_blank"
              rel="noopener noreferrer"
              className="text-amber underline decoration-amber-line underline-offset-2 hover:opacity-80"
            >
              {children}
            </a>
          ),
          ul: ({ children }) => <ul className="mb-2 ml-4 list-disc space-y-1 last:mb-0">{children}</ul>,
          ol: ({ children }) => <ol className="mb-2 ml-4 list-decimal space-y-1 last:mb-0">{children}</ol>,
          li: ({ children }) => <li className="marker:text-ink-4">{children}</li>,
          h1: ({ children }) => <h1 className="mb-1.5 mt-1 font-display text-lead font-semibold text-ink">{children}</h1>,
          h2: ({ children }) => <h2 className="mb-1.5 mt-1 font-display text-lead font-semibold text-ink">{children}</h2>,
          h3: ({ children }) => <h3 className="mb-1 mt-1 text-body font-semibold text-ink">{children}</h3>,
          blockquote: ({ children }) => (
            <blockquote className="my-2 border-l-2 border-amber-line pl-2.5 text-ink-3">{children}</blockquote>
          ),
          hr: () => <hr className="my-3 border-line-2" />,
          code: ({ className, children }) => {
            const isBlock = /language-/.test(className ?? "");
            if (isBlock) {
              return (
                <code className="font-mono text-small text-ink">{children}</code>
              );
            }
            return (
              <code className="rounded-tap bg-raise px-1 py-0.5 font-mono text-small text-ink">{children}</code>
            );
          },
          pre: ({ children }) => (
            <pre className="my-2 overflow-x-auto rounded-card border border-line bg-field p-2.5">{children}</pre>
          ),
          table: ({ children }) => (
            <div className="my-2 overflow-x-auto">
              <table className="w-full border-collapse text-small">{children}</table>
            </div>
          ),
          th: ({ children }) => <th className="border border-line px-2 py-1 text-left font-semibold text-ink">{children}</th>,
          td: ({ children }) => <td className="border border-line px-2 py-1 text-ink-2">{children}</td>,
        }}
      >
        {text}
      </Markdown>
    </div>
  );
});

/** A subtle blinking caret appended to streaming text. */
export function StreamCaret({ className }: { className?: string }) {
  return <span className={cn("ml-0.5 inline-block h-3.5 w-[2px] animate-pulse bg-amber align-middle", className)} />;
}
