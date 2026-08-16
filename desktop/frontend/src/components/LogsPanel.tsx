import { useEffect, useRef, useState } from "react";
import { RefreshCw, ScrollText, Loader2 } from "lucide-react";
import { api } from "../api";

interface Props {
  onClose: () => void;
}

// LogsPanel shows the tail of giznews.log so the user can follow what the
// pipeline has been deciding (fetch/classify/kb decisions, per batch).
export function LogsPanel({ onClose }: Props) {
  const [text, setText] = useState<string | null>(null);
  const bodyRef = useRef<HTMLDivElement>(null);

  const load = () => api.logs(300).then(setText).catch(() => setText(""));
  useEffect(() => {
    load();
    const iv = window.setInterval(load, 4000);
    bodyRef.current?.focus();
    return () => window.clearInterval(iv);
  }, []);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const el = bodyRef.current;
      if (!el) return;
      if (e.key === "j" || e.key === "ArrowDown") { e.preventDefault(); e.stopPropagation(); el.scrollBy({ top: 40 }); }
      else if (e.key === "k" || e.key === "ArrowUp") { e.preventDefault(); e.stopPropagation(); el.scrollBy({ top: -40 }); }
      else if (e.key === "PageDown") { e.preventDefault(); e.stopPropagation(); el.scrollBy({ top: el.clientHeight * 0.9 }); }
      else if (e.key === "PageUp") { e.preventDefault(); e.stopPropagation(); el.scrollBy({ top: -el.clientHeight * 0.9 }); }
      else if (e.key === "Escape") { e.stopPropagation(); onClose(); }
    };
    window.addEventListener("keydown", onKey, true);
    return () => window.removeEventListener("keydown", onKey, true);
  }, [onClose]);

  return (
    <div className="overlay" onClick={onClose}>
      <div className="palette logs-panel" onClick={(e) => e.stopPropagation()}>
        <div className="palette-head">
          <ScrollText size={14} /> Pipeline log
          <span className="muted" style={{ marginLeft: "auto", fontSize: 12 }}>~/.config/giznews/giznews.log</span>
          <button className="icon-btn" onClick={load} title="Refresh"><RefreshCw size={13} /></button>
        </div>
        <div className="logs-body" ref={bodyRef} tabIndex={0}>
          {text === null ? (
            <div className="empty with-icon"><Loader2 size={20} className="spin" /> Loading…</div>
          ) : text === "" ? (
            <div className="empty">No log entries yet.</div>
          ) : (
            <pre className="logs-pre">{text}</pre>
          )}
        </div>
        <div className="palette-head" style={{ borderTop: "1px solid var(--border)", borderBottom: "none", justifyContent: "flex-end" }}>
          <span className="muted" style={{ fontSize: 12 }}><kbd>j/k</kbd> scroll · <kbd>Esc</kbd> close</span>
        </div>
      </div>
    </div>
  );
}
