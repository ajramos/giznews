import { useState } from "react";

export interface PaletteCommand {
  name: string;
  hint: string;
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

  const run = (i: number) => {
    if (filtered[i]) {
      filtered[i].run();
      onClose();
    }
  };

  return (
    <div className="palette-overlay" onClick={onClose}>
      <div className="palette" onClick={(e) => e.stopPropagation()}>
        <input
          autoFocus
          value={value}
          onChange={(e) => { setValue(e.target.value); setSel(0); }}
          onKeyDown={(e) => {
            if (e.key === "ArrowDown") { e.preventDefault(); setSel((s) => Math.min(s + 1, filtered.length - 1)); }
            else if (e.key === "ArrowUp") { e.preventDefault(); setSel((s) => Math.max(s - 1, 0)); }
            else if (e.key === "Enter") run(sel);
            else if (e.key === "Escape") onClose();
          }}
          placeholder=": comando…"
        />
        <div className="palette-list">
          {filtered.length === 0 && <div className="empty">Sin comandos</div>}
          {filtered.map((c, i) => (
            <div key={c.name} className={`palette-item ${i === sel ? "selected" : ""}`} onMouseEnter={() => setSel(i)} onClick={() => run(i)}>
              <span className="cmd-name">{c.name}</span>
              <span className="cmd-hint">{c.hint}</span>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
