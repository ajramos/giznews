import type { SourceDTO } from "../types";

interface Props {
  sources: SourceDTO[];
  activeId: number | null;
  onSelect: (id: number | null) => void;
  onToggle: (id: number, enabled: boolean) => void;
}

export function SourceList({ sources, activeId, onSelect, onToggle }: Props) {
  return (
    <div className="source-list">
      <div className="pane-title">Fuentes</div>
      {sources.length === 0 && <div className="empty">Añade fuentes con :add-source</div>}
      {sources.map((s) => (
        <div
          key={s.id}
          className={`source-item ${activeId === s.id ? "active" : ""}`}
          onClick={() => onSelect(activeId === s.id ? null : s.id)}
        >
          <span className="dot" data-state={s.enabled ? "on" : "off"} />
          <span className="source-name">{s.name}</span>
          <button
            className="mini-btn"
            title="Desactivar/activar"
            onClick={(e) => {
              e.stopPropagation();
              onToggle(s.id, !s.enabled);
            }}
          >
            {s.enabled ? "off" : "on"}
          </button>
        </div>
      ))}
    </div>
  );
}
