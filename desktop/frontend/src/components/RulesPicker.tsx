import { useEffect, useState } from "react";
import { Zap, Check } from "lucide-react";
import { api } from "../api";
import type { RuleDTO } from "../types";

interface Props {
  onEdit: (r: RuleDTO) => void;
  onAdd: () => void;
  onClose: () => void;
  notify: (msg: string) => void;
}

// RulesPicker: browse/manage the deterministic classification rules.
//   j/k navigate · Enter toggle enabled · a add · e edit · d delete · Esc close
export function RulesPicker({ onEdit, onAdd, onClose, notify }: Props) {
  const [rules, setRules] = useState<RuleDTO[]>([]);
  const [focus, setFocus] = useState(0);

  const load = () => api.listRules().then(setRules).catch((e) => notify(String(e)));
  useEffect(() => { void load(); }, [notify]);

  useEffect(() => {
    setFocus((f) => Math.min(f, Math.max(0, rules.length - 1)));
  }, [rules.length]);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const n = rules.length;
      if (e.key === "j" || e.key === "ArrowDown") { e.preventDefault(); e.stopPropagation(); setFocus((f) => Math.min(f + 1, n - 1)); }
      else if (e.key === "k" || e.key === "ArrowUp") { e.preventDefault(); e.stopPropagation(); setFocus((f) => Math.max(f - 1, 0)); }
      else if (e.key === "Enter") { e.preventDefault(); e.stopPropagation(); const r = rules[focus]; if (r) void api.setRuleEnabled(r.id, !r.enabled).then(load).catch((err) => notify(String(err))); }
      else if (e.key === "a") { e.preventDefault(); e.stopPropagation(); onAdd(); }
      else if (e.key === "e") { e.preventDefault(); e.stopPropagation(); const r = rules[focus]; if (r) onEdit(r); }
      else if (e.key === "d") { e.preventDefault(); e.stopPropagation(); const r = rules[focus]; if (r) void api.deleteRule(r.id).then(load).catch((err) => notify(String(err))); }
      else if (e.key === "Escape") { e.stopPropagation(); onClose(); }
    };
    window.addEventListener("keydown", onKey, true);
    return () => window.removeEventListener("keydown", onKey, true);
  }, [rules, focus, onEdit, onAdd, onClose, notify]);

  return (
    <div className="overlay" onClick={onClose}>
      <div className="palette source-picker" onClick={(e) => e.stopPropagation()}>
        <div className="palette-head"><Zap size={14} /> Classification rules ({rules.length})</div>
        <div className="source-picker-list">
          {rules.length === 0 && <div className="palette-empty">No rules yet — press <kbd>a</kbd> to add one.</div>}
          {rules.map((r, i) => (
            <div
              key={r.id}
              className={`source-picker-item ${i === focus ? "selected" : ""}`}
              onMouseEnter={() => setFocus(i)}
              onClick={() => void api.setRuleEnabled(r.id, !r.enabled).then(load).catch((e) => notify(String(e)))}
            >
              <span className="sp-dot" data-on={r.enabled} />
              <span className="sp-type">{r.enabled ? <Check size={13} /> : <Zap size={13} />}</span>
              <span className="sp-name">{r.name}</span>
              <span className="sp-meta">{r.query}</span>
              <span className="sp-state">{r.enabled ? "on" : "off"}</span>
            </div>
          ))}
        </div>
        <div className="palette-head" style={{ borderTop: "1px solid var(--border)", borderBottom: "none", flexWrap: "wrap", gap: "4px 14px" }}>
          <span><kbd>j/k</kbd> navigate</span>
          <span><kbd>Enter</kbd> toggle</span>
          <span><kbd>a</kbd> add</span>
          <span><kbd>e</kbd> edit</span>
          <span><kbd>d</kbd> delete</span>
          <span><kbd>Esc</kbd> close</span>
        </div>
      </div>
    </div>
  );
}
