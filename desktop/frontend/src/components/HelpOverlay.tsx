import { useEffect, useRef } from "react";
import { CircleHelp } from "lucide-react";
import { HELP } from "../keys";

export function HelpOverlay({ onClose }: { onClose: () => void }) {
  const listRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    listRef.current?.focus();
    const onKey = (e: KeyboardEvent) => {
      const el = listRef.current;
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
      <div className="palette help" onClick={(e) => e.stopPropagation()}>
        <div className="palette-head"><CircleHelp size={14} /> Keyboard shortcuts</div>
        <div className="help-list" ref={listRef} tabIndex={0}>
          {HELP.map((cat) => (
            <div key={cat.title}>
              <div className="help-cat">{cat.title}</div>
              {cat.rows.map((r) => (
                <div key={r.keys} className="help-row">
                  <span className="keys"><kbd>{r.keys}</kbd></span>
                  <span>{r.label}</span>
                </div>
              ))}
            </div>
          ))}
        </div>
        <div className="help-note">
          <strong>Scroll:</strong> j/k · ↑/↓ · PageUp/PageDown. <strong>Legend:</strong> ★ = importance (0-3, ★★☆/★★★ = relevant or key) · ● = unread · <span className="star-badge">★</span> = starred · 🗄/strikethrough = archived.
          <br /><strong>Archiving is logical:</strong> articles are never physically deleted; everything is recoverable from the archived view or the undo toast.
        </div>
      </div>
    </div>
  );
}
