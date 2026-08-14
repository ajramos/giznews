import { Pencil, Trash2, Plus, Rss, Mail, BookOpen, MessageSquare, type LucideIcon } from "lucide-react";
import type { SourceDTO } from "../types";

const TYPE_ICON: Record<string, LucideIcon> = {
  rss: Rss,
  hackernews: MessageSquare,
  arxiv: BookOpen,
  gmail: Mail,
};

function typeIcon(t: string): LucideIcon {
  return TYPE_ICON[t] ?? Rss;
}

interface Props {
  sources: SourceDTO[];
  activeId: number | null;
  onSelect: (id: number | null) => void;
  onToggle: (id: number, enabled: boolean) => void;
  onAdd: () => void;
  onEdit: (s: SourceDTO) => void;
  onDelete: (s: SourceDTO) => void;
}

export function SourceList({ sources, activeId, onSelect, onToggle, onAdd, onEdit, onDelete }: Props) {
  const groups = new Map<string, SourceDTO[]>();
  for (const s of sources) {
    const g = s.group || "general";
    if (!groups.has(g)) groups.set(g, []);
    groups.get(g)!.push(s);
  }

  return (
    <div className="sources-pane">
      <div className="pane-head">
        <span className="pane-title">Fuentes</span>
        <button className="icon-btn" onClick={onAdd} title="Añadir fuente">
          <Plus size={14} /> Añadir
        </button>
      </div>
      {sources.length === 0 ? (
        <div className="empty with-icon">
          <Rss size={30} />
          <span>No hay fuentes todavía.</span>
          <button onClick={onAdd}>Añadir tu primera fuente</button>
        </div>
      ) : (
        <div className="source-list">
          {[...groups.entries()].map(([group, items]) => (
            <div key={group}>
              <div className="source-group-title">{group}</div>
              {items.map((s) => {
                const Icon = typeIcon(s.type);
                return (
                  <div
                    key={s.id}
                    className={`source-item ${activeId === s.id ? "active" : ""}`}
                    onClick={() => onSelect(activeId === s.id ? null : s.id)}
                    title={`${s.name} (${s.type})`}
                  >
                    <span className={`dot ${s.enabled ? "on" : "off"}`} />
                    <span className="source-name">{s.name}</span>
                    <span className="source-type"><Icon size={12} /></span>
                    <button
                      className={s.enabled ? "switch on" : "switch"}
                      data-on={s.enabled}
                      aria-pressed={s.enabled}
                      title={s.enabled ? "Desactivar" : "Activar"}
                      onClick={(e) => {
                        e.stopPropagation();
                        onToggle(s.id, !s.enabled);
                      }}
                    />
                    <button className="icon-btn source-act" title="Editar" onClick={(e) => { e.stopPropagation(); onEdit(s); }}>
                      <Pencil size={13} />
                    </button>
                    <button className="icon-btn source-act" title="Eliminar de la lista (los artículos se conservan)" onClick={(e) => { e.stopPropagation(); onDelete(s); }}>
                      <Trash2 size={13} />
                    </button>
                  </div>
                );
              })}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
