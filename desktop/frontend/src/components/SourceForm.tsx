import { useState } from "react";
import { Plus, Pencil, X } from "lucide-react";
import type { SourceDTO } from "../types";

const TYPES = ["rss", "hackernews", "arxiv", "gmail"] as const;

interface Props {
  initial: SourceDTO | null;
  onSave: (data: { name: string; type: string; url: string; group: string }) => void;
  onCancel: () => void;
}

export function SourceForm({ initial, onSave, onCancel }: Props) {
  const [name, setName] = useState(initial?.name ?? "");
  const [type, setType] = useState<string>(initial?.type ?? "rss");
  const [url, setUrl] = useState(initial?.url ?? "");
  const [group, setGroup] = useState(initial?.group ?? "general");
  const [error, setError] = useState<string | null>(null);

  const valid = name.trim().length > 0 && url.trim().length > 0;

  const submit = () => {
    if (!valid) {
      setError("Nombre y URL son obligatorios.");
      return;
    }
    onSave({ name: name.trim(), type, url: url.trim(), group: group.trim() || "general" });
  };

  return (
    <div className="modal-overlay" onClick={onCancel}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-head">
          <h2>{initial ? <><Pencil size={15} /> Editar fuente</> : <><Plus size={15} /> Añadir fuente</>}</h2>
          <button className="icon-btn" onClick={onCancel}><X size={15} /></button>
        </div>
        <div className="modal-body">
          <div className="field">
            <label>Nombre</label>
            <input autoFocus value={name} onChange={(e) => setName(e.target.value)} placeholder="p. ej. DeepMind Blog" />
          </div>
          <div className="field">
            <label>Tipo</label>
            <div className="type-seg">
              {TYPES.map((t) => (
                <button key={t} className={type === t ? "active" : ""} onClick={() => setType(t)}>{t}</button>
              ))}
            </div>
          </div>
          <div className="field">
            <label>URL del feed</label>
            <input value={url} onChange={(e) => setUrl(e.target.value)} placeholder={type === "gmail" ? 'query gmail (opcional) o deja el query global' : "https://…/rss.xml"} />
          </div>
          <div className="field">
            <label>Grupo</label>
            <input value={group} onChange={(e) => setGroup(e.target.value)} placeholder="general" />
          </div>
          {error && <div className="muted" style={{ color: "var(--bad)" }}>{error}</div>}
        </div>
        <div className="modal-foot">
          <button onClick={onCancel}>Cancelar</button>
          <button disabled={!valid} onClick={submit} style={{ background: "var(--accent-dim)", borderColor: "var(--accent-dim)", color: "#fff" }}>
            {initial ? "Guardar" : "Añadir"}
          </button>
        </div>
      </div>
    </div>
  );
}
