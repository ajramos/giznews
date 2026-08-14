import { useEffect, useState } from "react";
import { Rss, Mail, BookOpen, MessageSquare, List, type LucideIcon } from "lucide-react";
import type { SourceDTO } from "../types";

const TYPE_ICON: Record<string, LucideIcon> = {
  rss: Rss,
  hackernews: MessageSquare,
  arxiv: BookOpen,
  gmail: Mail,
};

interface Props {
  sources: SourceDTO[];
  onToggle: (id: number, enabled: boolean) => void;
  onAdd: () => void;
  onEdit: (s: SourceDTO) => void;
  onDelete: (s: SourceDTO) => void;
  onFilter: (s: SourceDTO) => void;
  onClose: () => void;
}

// SourcePicker: a giztui-style keyboard picker for managing sources.
//   j/k or ↑/↓  navigate      · Enter  toggle enabled
//   a  add · e  edit · d  delete · f  filter by source · Esc  close
export function SourcePicker({ sources, onToggle, onAdd, onEdit, onDelete, onFilter, onClose }: Props) {
  const [focus, setFocus] = useState(0);

  useEffect(() => {
    setFocus((f) => Math.min(f, Math.max(0, sources.length - 1)));
  }, [sources.length]);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const n = sources.length;
      if (e.key === "j" || e.key === "ArrowDown") { e.preventDefault(); e.stopPropagation(); setFocus((f) => Math.min(f + 1, n - 1)); }
      else if (e.key === "k" || e.key === "ArrowUp") { e.preventDefault(); e.stopPropagation(); setFocus((f) => Math.max(f - 1, 0)); }
      else if (e.key === "Enter") { e.preventDefault(); e.stopPropagation(); const s = sources[focus]; if (s) onToggle(s.id, !s.enabled); }
      else if (e.key === "a") { e.preventDefault(); e.stopPropagation(); onAdd(); }
      else if (e.key === "e") { e.preventDefault(); e.stopPropagation(); const s = sources[focus]; if (s) onEdit(s); }
      else if (e.key === "d") { e.preventDefault(); e.stopPropagation(); const s = sources[focus]; if (s) onDelete(s); }
      else if (e.key === "f") { e.preventDefault(); e.stopPropagation(); const s = sources[focus]; if (s) onFilter(s); }
      else if (e.key === "Escape") { e.stopPropagation(); onClose(); }
    };
    window.addEventListener("keydown", onKey, true);
    return () => window.removeEventListener("keydown", onKey, true);
  }, [sources, focus, onToggle, onAdd, onEdit, onDelete, onFilter, onClose]);

  return (
    <div className="overlay" onClick={onClose}>
      <div className="palette source-picker" onClick={(e) => e.stopPropagation()}>
        <div className="palette-head"><List size={14} /> Fuentes ({sources.length})</div>
        <div className="source-picker-list">
          {sources.length === 0 && <div className="palette-empty">Sin fuentes — pulsa <kbd>a</kbd> para añadir.</div>}
          {sources.map((s, i) => {
            const Icon = TYPE_ICON[s.type] ?? Rss;
            return (
              <button
                key={s.id}
                className={`source-picker-item ${i === focus ? "selected" : ""}`}
                onMouseEnter={() => setFocus(i)}
                onClick={() => onToggle(s.id, !s.enabled)}
              >
                <span className="sp-dot" data-on={s.enabled} />
                <span className="sp-type"><Icon size={13} /></span>
                <span className="sp-name">{s.name}</span>
                <span className="sp-meta">{s.type}</span>
                <span className="sp-state">{s.enabled ? "on" : "off"}</span>
              </button>
            );
          })}
        </div>
        <div className="palette-head" style={{ borderTop: "1px solid var(--border)", borderBottom: "none", flexWrap: "wrap", gap: "4px 14px" }}>
          <span><kbd>j/k</kbd> navegar</span>
          <span><kbd>Enter</kbd> activar/desactivar</span>
          <span><kbd>a</kbd> añadir</span>
          <span><kbd>e</kbd> editar</span>
          <span><kbd>d</kbd> eliminar</span>
          <span><kbd>f</kbd> filtrar</span>
          <span><kbd>Esc</kbd> cerrar</span>
        </div>
      </div>
    </div>
  );
}
