import { useEffect, useRef, useState } from "react";
import { Search, Link2, CornerDownLeft, FileText, GitBranch, FlaskConical, Inbox, ExternalLink, type LucideIcon } from "lucide-react";

export interface LinkItem {
  id: number; // note id (0 for external links)
  title: string;
  type: string; // note type, or "external"
  dir: "out" | "in"; // out = "enlaza a", in = "enlazado por"
  url?: string; // when set, this is an external link
}

const TYPE_ICON: Record<string, LucideIcon> = {
  atom: FileText,
  electron: GitBranch,
  molecule: FlaskConical,
  inbox: Inbox,
  external: ExternalLink,
};

interface Props {
  links: LinkItem[];
  onPick: (item: LinkItem) => void;
  onClose: () => void;
}

// LinksPicker: a giztui-style picker for a note's connections.
//   type to filter · Tab input↔list · ↑/↓ navigate · Enter/1-9 open · Esc close
export function LinksPicker({ links, onPick, onClose }: Props) {
  const [query, setQuery] = useState("");
  const [sel, setSel] = useState(0);
  const [focus, setFocus] = useState<"input" | "list">("input");
  const inputRef = useRef<HTMLInputElement>(null);

  const q = query.trim().toLowerCase();
  const filtered = links.filter(
    (l) => !q || l.title.toLowerCase().includes(q) || l.type.includes(q)
  );

  useEffect(() => {
    inputRef.current?.focus();
  }, []);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") { e.preventDefault(); e.stopPropagation(); onClose(); return; }
      if (e.key === "Tab") { e.preventDefault(); e.stopPropagation(); setFocus((f) => (f === "input" ? "list" : "input")); return; }
      if (e.key === "ArrowDown") {
        e.preventDefault(); e.stopPropagation();
        if (focus === "input") setFocus("list");
        else setSel((s) => Math.min(s + 1, Math.max(0, filtered.length - 1)));
        return;
      }
      if (e.key === "ArrowUp") {
        e.preventDefault(); e.stopPropagation();
        if (focus === "list" && sel === 0) setFocus("input");
        else setSel((s) => Math.max(s - 1, 0));
        return;
      }
      if (e.key === "Enter") {
        e.preventDefault(); e.stopPropagation();
        if (filtered.length) { onPick(filtered[focus === "list" ? sel : 0]); onClose(); }
        return;
      }
      if (e.key >= "1" && e.key <= "9") {
        e.preventDefault(); e.stopPropagation();
        const i = Number(e.key) - 1;
        if (filtered[i]) { onPick(filtered[i]); onClose(); }
      }
    };
    window.addEventListener("keydown", onKey, true);
    return () => window.removeEventListener("keydown", onKey, true);
  }, [focus, sel, filtered, onPick, onClose]);

  return (
    <div className="overlay" onClick={onClose}>
      <div className="palette links-picker" onClick={(e) => e.stopPropagation()}>
        <div className="palette-head"><Link2 size={14} /> Links ({filtered.length})</div>
        <div className="search-box">
          <Search size={15} />
          <input
            ref={inputRef}
            value={query}
            onChange={(e) => { setQuery(e.target.value); setFocus("input"); setSel(0); }}
            placeholder="filter by title or type…"
          />
        </div>
        <div className="palette-list">
          {filtered.length === 0 && <div className="palette-empty">No links.</div>}
          {filtered.map((l, i) => {
            const Icon = TYPE_ICON[l.type] ?? FileText;
            return (
              <div
                key={(l.url ?? String(l.id)) + l.dir}
                className={`palette-item ${focus === "list" && i === sel ? "selected" : ""}`}
                onMouseEnter={() => { setFocus("list"); setSel(i); }}
                onClick={() => { onPick(l); onClose(); }}
              >
                <span className="cmd-left">
                  <span className="sp-type"><Icon size={13} /></span>
                  <span className="cmd-name">{i + 1}. {l.title}</span>
                </span>
                <span className="cmd-hint">{l.url ? l.url : `${l.dir === "out" ? "→ links to" : "← linked by"} · ${l.type}`}</span>
              </div>
            );
          })}
        </div>
        <div className="palette-head" style={{ borderTop: "1px solid var(--border)", borderBottom: "none", flexWrap: "wrap", gap: "4px 12px" }}>
          <span className="muted" style={{ fontSize: 12, display: "inline-flex", alignItems: "center", gap: 6 }}>
            <CornerDownLeft size={12} /> Enter / 1-9 open · Tab list · Esc close
          </span>
        </div>
      </div>
    </div>
  );
}
