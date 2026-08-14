import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import rehypeRaw from "rehype-raw";
import { api } from "../api";

export function Markdown({ content }: { content: string }) {
  if (!content) return <div className="empty">Sin contenido</div>;
  return (
    <div className="markdown">
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        rehypePlugins={[rehypeRaw]}
        components={{
          a: ({ href, children }) => (
            <a
              href={href}
              onClick={(e) => {
                e.preventDefault();
                if (href) void api.openURL(href);
              }}
            >
              {children}
            </a>
          ),
        }}
      >
        {content}
      </ReactMarkdown>
    </div>
  );
}

export function stars(importance: number): string {
  const n = Math.max(0, Math.min(3, importance));
  return "★".repeat(n) + "☆".repeat(3 - n);
}

// CSS class for a category chip ("" when unknown).
export function catClass(category: string): string {
  const allowed = ["models", "research", "industry", "funding", "regulation", "tools", "open-source", "opinion", "general"];
  return allowed.includes(category) ? category : "general";
}

export function timeAgo(iso?: string): string {
  if (!iso) return "";
  const d = new Date(iso);
  if (isNaN(d.getTime())) return "";
  const diff = Date.now() - d.getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return "ahora";
  if (mins < 60) return `${mins}m`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h`;
  const days = Math.floor(hours / 24);
  if (days < 30) return `${days}d`;
  return d.toLocaleDateString("es-ES", { day: "numeric", month: "short" });
}
