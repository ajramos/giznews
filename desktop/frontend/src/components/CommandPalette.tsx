import { useState, type ReactNode } from "react";
import { Command, CornerDownLeft } from "lucide-react";

export interface PaletteCommand {
  name: string;
  hint: string;
  icon?: ReactNode;
  run: () => void;
}

interface Props {
  commands: PaletteCommand[];
  onClose: () => void;
}

export function CommandPalette({ commands, onClose }: Props) {
  const [value, setValue] = useState("");
  const [sel, setSel] = useState(0);
  const filtered = commands.filter((c) => c.name.includes(value.toLowerCase()));
  const q = value.trim().toLowerCase();

  // Allow exact-ish commands to run on Enter even when no fuzzy match.
  const run = (i: number) => {
    if (filtered[i]) { filtered[i].run(); onClose(); return; }
    // fallback: first command whose name starts with the typed text
    const m = commands.find((c) => c.name.toLowerCase().startsWith(q));
    if (m && q) { m.run(); onClose(); }
  };

  return (
    <div className="overlay" onClick={onClose}>
      <div className="palette" onClick={(e) => e.stopPropagation()}>
        <div className="palette-head"><Command size={14} /> type a command</div>
        <input
          autoFocus
          value={value}
          onChange={(e) => { setValue(e.target.value); setSel(0); }}
          onKeyDown={(e) => {
            if (e.key === "ArrowDown") { e.preventDefault(); setSel((s) => Math.min(s + 1, Math.max(0, filtered.length - 1))); }
            else if (e.key === "ArrowUp") { e.preventDefault(); setSel((s) => Math.max(s - 1, 0)); }
            else if (e.key === "Enter") { e.preventDefault(); run(sel); }
            else if (e.key === "Escape") { e.preventDefault(); onClose(); }
          }}
          placeholder="fetch · classify · kb build · search index · digest · theme…"
        />
        <div className="palette-list">
          {filtered.length === 0 && <div className="palette-empty">No commands</div>}
          {filtered.map((c, i) => (
            <div key={c.name} className={`palette-item ${i === sel ? "selected" : ""}`} onMouseEnter={() => setSel(i)} onClick={() => run(i)}>
              <span className="cmd-left">
                {c.icon}
                <span className="cmd-name">{c.name}</span>
              </span>
              <span className="cmd-hint">{c.hint}</span>
            </div>
          ))}
        </div>
        <div className="palette-head" style={{ borderTop: "1px solid var(--border)", borderBottom: "none" }}>
          <CornerDownLeft size={12} /> Enter to run · Esc to close
        </div>
      </div>
    </div>
  );
}
