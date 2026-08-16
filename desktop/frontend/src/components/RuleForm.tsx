import { useState } from "react";
import { Pencil, X, Zap } from "lucide-react";
import type { RuleActionDTO, RuleDTO } from "../types";
import { CATEGORIES } from "./CategoryPicker";

interface Props {
  initial: RuleDTO | null;
  onSave: (data: { name: string; query: string; actions: RuleActionDTO[]; enabled: boolean }) => void;
  onCancel: () => void;
}

// RuleForm: create/edit a deterministic classification rule (⚡ prefilter).
export function RuleForm({ initial, onSave, onCancel }: Props) {
  const [name, setName] = useState(initial?.name ?? "");
  const [query, setQuery] = useState(initial?.query ?? "");
  const [category, setCategory] = useState(initial?.actions.find((a) => a.type === "category")?.value ?? "");
  const [importance, setImportance] = useState(initial?.actions.find((a) => a.type === "importance")?.value ?? "");
  const [tags, setTags] = useState((initial?.actions.filter((a) => a.type === "tag").map((a) => a.value) ?? []).join(", "));
  const [archive, setArchive] = useState(initial?.actions.some((a) => a.type === "archive") ?? false);
  const [enabled, setEnabled] = useState(initial?.enabled ?? true);
  const [error, setError] = useState<string | null>(null);

  const valid = name.trim().length > 0 && query.trim().length > 0;

  const submit = () => {
    if (!valid) {
      setError("Name and query (regex) are required.");
      return;
    }
    const actions: RuleActionDTO[] = [];
    if (category) actions.push({ type: "category", value: category });
    if (importance) actions.push({ type: "importance", value: importance });
    for (const t of tags.split(",").map((s) => s.trim()).filter(Boolean)) actions.push({ type: "tag", value: t });
    if (archive) actions.push({ type: "archive", value: "" });
    onSave({ name: name.trim(), query: query.trim(), actions, enabled });
  };

  return (
    <div className="modal-overlay" onClick={onCancel}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-head">
          <h2>{initial ? <><Pencil size={15} /> Edit rule</> : <><Zap size={15} /> Add rule</>}</h2>
          <button className="icon-btn" onClick={onCancel}><X size={15} /></button>
        </div>
        <div className="modal-body">
          <div className="field">
            <label>Name</label>
            <input autoFocus value={name} onChange={(e) => setName(e.target.value)} placeholder="e.g. openai" />
          </div>
          <div className="field">
            <label>Query (regex)</label>
            <input value={query} onChange={(e) => setQuery(e.target.value)} placeholder="openai|gpt|chatgpt" />
            <span className="muted" style={{ fontSize: 11 }}>Matched against title + author + URL (case-insensitive).</span>
          </div>
          <div className="field">
            <label>Category</label>
            <select value={category} onChange={(e) => setCategory(e.target.value)}>
              <option value="">(none)</option>
              {CATEGORIES.map((c) => <option key={c} value={c}>{c}</option>)}
            </select>
          </div>
          <div className="field">
            <label>Importance</label>
            <select value={importance} onChange={(e) => setImportance(e.target.value)}>
              <option value="">(none)</option>
              <option value="0">0 · noise</option>
              <option value="1">1 · background</option>
              <option value="2">2 · relevant</option>
              <option value="3">3 · key</option>
            </select>
          </div>
          <div className="field">
            <label>Tags (comma-separated)</label>
            <input value={tags} onChange={(e) => setTags(e.target.value)} placeholder="agents, safety" />
          </div>
          <div className="field" style={{ display: "flex", gap: 16 }}>
            <label style={{ display: "inline-flex", alignItems: "center", gap: 6 }}>
              <input type="checkbox" checked={archive} onChange={(e) => setArchive(e.target.checked)} /> Archive on match
            </label>
            <label style={{ display: "inline-flex", alignItems: "center", gap: 6 }}>
              <input type="checkbox" checked={enabled} onChange={(e) => setEnabled(e.target.checked)} /> Enabled
            </label>
          </div>
          {error && <div className="muted" style={{ color: "var(--bad)" }}>{error}</div>}
        </div>
        <div className="modal-foot">
          <button onClick={onCancel}>Cancel</button>
          <button disabled={!valid} onClick={submit} style={{ background: "var(--accent-dim)", borderColor: "var(--accent-dim)", color: "#fff" }}>
            {initial ? "Save" : "Add"}
          </button>
        </div>
      </div>
    </div>
  );
}
