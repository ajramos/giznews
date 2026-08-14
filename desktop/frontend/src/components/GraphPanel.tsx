import type { NoteDTO } from "../types";

interface Props {
  note: NoteDTO | null;
  neighbors: NoteDTO[];
  loading: boolean;
  onOpenNote: (id: number) => void;
}

export function GraphPanel({ note, neighbors, loading, onOpenNote }: Props) {
  return (
    <div className="panel graph-panel">
      <div className="panel-title">g g · Knowledge graph</div>
      {loading && <div className="empty">Cargando grafo…</div>}
      {!loading && !note && <div className="empty">Este artículo aún no tiene nota (ejecuta kb build).</div>}
      {note && (
        <>
          <div className="graph-current">
            <span className="note-type">{note.type}</span>
            <h2>{note.title}</h2>
            {note.tags.map((t) => <span key={t} className="tag">#{t}</span>)}
          </div>
          <div className="graph-links">
            <div className="pane-title">Conexiones ({neighbors.length})</div>
            {neighbors.length === 0 && <div className="empty">Sin conexiones todavía.</div>}
            {neighbors.map((n) => (
              <div key={n.id} className="result-row" onClick={() => onOpenNote(n.id)}>
                <span className="note-type">{n.type}</span>
                <span className="result-title">{n.title}</span>
              </div>
            ))}
          </div>
        </>
      )}
    </div>
  );
}
