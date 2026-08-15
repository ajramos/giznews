import { useEffect, useState } from "react";
import { FileText, GitBranch, FlaskConical, NotebookText, type LucideIcon } from "lucide-react";
import { api } from "../api";
import type { NoteDTO } from "../types";

const TYPE_META: Record<string, { label: string; icon: LucideIcon }> = {
  atom: { label: "atom", icon: FileText },
  electron: { label: "electron", icon: GitBranch },
  molecule: { label: "molecule", icon: FlaskConical },
  inbox: { label: "inbox", icon: NotebookText },
};

const FILTERS: { key: string; type: string; label: string }[] = [
  { key: "*", type: "", label: "all" },
  { key: "a", type: "atom", label: "atoms" },
  { key: "e", type: "electron", label: "electrons" },
  { key: "m", type: "molecule", label: "molecules" },
];

interface Props {
  onOpen: (id: number) => void;
  onClose: () => void;
  notify: (msg: string) => void;
}

// NotesPicker: browse the knowledge-graph notes (atoms/electrons/molecules).
//   j/k or ↑/↓  navigate · Enter  open · a/e/m/*  filter by type · Esc  close
export function NotesPicker({ onOpen, onClose, notify }: Props) {
  const [notes, setNotes] = useState<NoteDTO[]>([]);
  const [filter, setFilter] = useState("");
  const [focus, setFocus] = useState(0);

  useEffect(() => {
    api.listNotes(filter).then(setNotes).catch((e) => notify(String(e)));
  }, [filter, notify]);

  useEffect(() => {
    setFocus((f) => Math.min(f, Math.max(0, notes.length - 1)));
  }, [notes.length]);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const n = notes.length;
      if (e.key === "j" || e.key === "ArrowDown") { e.preventDefault(); e.stopPropagation(); setFocus((f) => Math.min(f + 1, n - 1)); }
      else if (e.key === "k" || e.key === "ArrowUp") { e.preventDefault(); e.stopPropagation(); setFocus((f) => Math.max(f - 1, 0)); }
      else if (e.key === "Enter") { e.preventDefault(); e.stopPropagation(); const s = notes[focus]; if (s) onOpen(s.id); }
      else if (e.key === "a") { e.preventDefault(); e.stopPropagation(); setFilter("atom"); }
      else if (e.key === "e") { e.preventDefault(); e.stopPropagation(); setFilter("electron"); }
      else if (e.key === "m") { e.preventDefault(); e.stopPropagation(); setFilter("molecule"); }
      else if (e.key === "*") { e.preventDefault(); e.stopPropagation(); setFilter(""); }
      else if (e.key === "Escape") { e.stopPropagation(); onClose(); }
    };
    window.addEventListener("keydown", onKey, true);
    return () => window.removeEventListener("keydown", onKey, true);
  }, [notes, focus, filter, onOpen, onClose]);

  return (
    <div className="overlay" onClick={onClose}>
      <div className="palette notes-picker" onClick={(e) => e.stopPropagation()}>
        <div className="palette-head">
          <NotebookText size={14} /> Knowledge-graph notes ({notes.length})
          <span className="muted" style={{ marginLeft: "auto", fontSize: 12 }}>
            {FILTERS.find((f) => f.type === filter)?.label ?? "all"}
          </span>
        </div>
        <div className="source-picker-list">
          {notes.length === 0 && <div className="palette-empty">No notes. Run <kbd>:process</kbd> (or <kbd>:kb build</kbd>) to generate them.</div>}
          {notes.map((n, i) => {
            const meta = TYPE_META[n.type] ?? TYPE_META.atom;
            const Icon = meta.icon;
            return (
              <button
                key={n.id}
                className={`source-picker-item ${i === focus ? "selected" : ""}`}
                onMouseEnter={() => setFocus(i)}
                onClick={() => onOpen(n.id)}
              >
                <span className="sp-type"><Icon size={13} /></span>
                <span className="sp-name">{n.title}</span>
                <span className="sp-meta">{n.type}</span>
              </button>
            );
          })}
        </div>
        <div className="palette-head" style={{ borderTop: "1px solid var(--border)", borderBottom: "none", flexWrap: "wrap", gap: "4px 14px" }}>
          <span><kbd>j/k</kbd> navigate</span>
          <span><kbd>Enter</kbd> open</span>
          <span><kbd>a/e/m</kbd> atom/electron/molecule</span>
          <span><kbd>*</kbd> all</span>
          <span><kbd>Esc</kbd> close</span>
        </div>
      </div>
    </div>
  );
}
