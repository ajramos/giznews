import { useEffect, useState } from "react";
import { Filter } from "lucide-react";

export const CATEGORIES = [
  "models", "research", "industry", "funding", "regulation", "tools", "open-source", "opinion", "general",
];

interface Props {
  current: string | null;
  unclassified: boolean;
  onPick: (cat: string | null) => void;
  onUnclassified: (v: boolean) => void;
  onClose: () => void;
}

interface Item {
  key: string;
  label: string;
  cat: string | null;
  unclassified?: boolean;
}

// CategoryPicker is the keyboard overlay for the classification filters.
//   j/k navigate · Enter apply · Esc close
export function CategoryPicker({ current, unclassified, onPick, onUnclassified, onClose }: Props) {
  const items: Item[] = [
    { key: "all", label: "All", cat: null },
    { key: "unclassified", label: "Unclassified", cat: null, unclassified: true },
    ...CATEGORIES.map((c) => ({ key: c, label: c, cat: c })),
  ];
  const [focus, setFocus] = useState(() => {
    const i = items.findIndex((it) => (it.unclassified ? unclassified : it.cat === current));
    return i >= 0 ? i : 0;
  });

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const n = items.length;
      if (e.key === "j" || e.key === "ArrowDown") { e.preventDefault(); e.stopPropagation(); setFocus((f) => Math.min(f + 1, n - 1)); }
      else if (e.key === "k" || e.key === "ArrowUp") { e.preventDefault(); e.stopPropagation(); setFocus((f) => Math.max(f - 1, 0)); }
      else if (e.key === "Enter") {
        e.preventDefault(); e.stopPropagation();
        const it = items[focus];
        if (it) {
          if (it.unclassified) onUnclassified(!unclassified);
          else onPick(it.cat);
          onClose();
        }
      } else if (e.key === "Escape") { e.preventDefault(); e.stopPropagation(); onClose(); }
    };
    window.addEventListener("keydown", onKey, true);
    return () => window.removeEventListener("keydown", onKey, true);
  }, [items, focus, current, unclassified, onPick, onUnclassified, onClose]);

  return (
    <div className="overlay" onClick={onClose}>
      <div className="palette category-picker" onClick={(e) => e.stopPropagation()}>
        <div className="palette-head"><Filter size={14} /> Filter by classification</div>
        <div className="palette-list">
          {items.map((it, i) => {
            const active = it.unclassified ? unclassified : it.cat === current;
            return (
              <div
                key={it.key}
                className={`palette-item ${i === focus ? "selected" : ""} ${active ? "active" : ""}`}
                onMouseEnter={() => setFocus(i)}
                onClick={() => {
                  if (it.unclassified) onUnclassified(!unclassified);
                  else onPick(it.cat);
                  onClose();
                }}
              >
                <span className="cmd-name">{it.label}</span>
                {active && <span className="cmd-hint">✓</span>}
              </div>
            );
          })}
        </div>
        <div className="palette-head" style={{ borderTop: "1px solid var(--border)", borderBottom: "none" }}>
          <span className="muted" style={{ fontSize: 12 }}><kbd>j/k</kbd> navigate · <kbd>Enter</kbd> apply · <kbd>Esc</kbd> close</span>
        </div>
      </div>
    </div>
  );
}
