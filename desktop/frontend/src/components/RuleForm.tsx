import { useState } from "react";
import { Pencil, X, Zap } from "lucide-react";
import type { RuleActionDTO, RuleDTO } from "../types";
import { CATEGORIES } from "./CategoryPicker";
import { Select, type SelectOption } from "./Select";

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
  const [importance, setImportance] = useState(
    initial?.actions.find((a) => a.type === "importance" || a.type === "boost")?.value ?? "",
  );
  const [boost, setBoost] = useState(initial?.actions.some((a) => a.type === "boost") ?? false);
  const [tags, setTags] = useState((initial?.actions.filter((a) => a.type === "tag").map((a) => a.value) ?? []).join(", "));
  const [archive, setArchive] = useState(initial?.actions.some((a) => a.type === "archive") ?? false);
  const [keep, setKeep] = useState(initial?.actions.some((a) => a.type === "keep") ?? false);
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
    if (importance) actions.push({ type: boost ? "boost" : "importance", value: importance });
    for (const t of tags.split(",").map((s) => s.trim()).filter(Boolean)) actions.push({ type: "tag", value: t });
    if (archive) actions.push({ type: "archive", value: "" });
    if (keep) actions.push({ type: "keep", value: "" });
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
            <Select
              value={category}
              onChange={setCategory}
              options={[{ value: "", label: "(none)" }, ...CATEGORIES.map((c): SelectOption => ({ value: c, label: c }))]}
            />
          </div>
          <div className="field">
            <label>Importance</label>
            <label style={{ display: "inline-flex", alignItems: "center", gap: 6, fontWeight: 400 }}
              title="Applied after the LLM rather than instead of it: the article keeps its summary and entities, and is only raised if the model rated it lower.">
              <input type="checkbox" checked={boost} disabled={!importance}
                onChange={(e) => { setBoost(e.target.checked); if (e.target.checked) setArchive(false); }} />
              as a floor after the LLM (boost)
            </label>
            <Select
              value={importance}
              onChange={setImportance}
              options={[
                { value: "", label: "(none)" },
                { value: "0", label: "0 · noise" },
                { value: "1", label: "1 · background" },
                { value: "2", label: "2 · relevant" },
                { value: "3", label: "3 · key" },
              ]}
            />
          </div>
          <div className="field">
            <label>Tags (comma-separated)</label>
            <input value={tags} onChange={(e) => setTags(e.target.value)} placeholder="agents, safety" />
          </div>
          <div className="field" style={{ display: "flex", gap: 16, flexWrap: "wrap" }}>
            <label style={{ display: "inline-flex", alignItems: "center", gap: 6 }}>
              <input type="checkbox" checked={archive} onChange={(e) => { setArchive(e.target.checked); if (e.target.checked) { setKeep(false); setBoost(false); } }} /> Archive on match
            </label>
            <label style={{ display: "inline-flex", alignItems: "center", gap: 6 }} title="Protect: the rule fires, nothing is applied, and the article still goes to the LLM. Put it above the rules it protects from.">
              <input type="checkbox" checked={keep} onChange={(e) => { setKeep(e.target.checked); if (e.target.checked) setArchive(false); }} /> Keep (protect from later rules)
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
