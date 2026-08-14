import { useEffect, useState } from "react";
import { Palette, Check } from "lucide-react";
import { THEMES, type ThemeName } from "../theme";

interface Props {
  value: ThemeName;
  onChange: (t: ThemeName) => void;
  onClose: () => void;
}

// ThemeModal is the :theme picker: a centred list of themes with colour
// swatches, fully keyboard-driven (↑/↓/Enter/Esc).
export function ThemeModal({ value, onChange, onClose }: Props) {
  const start = THEMES.findIndex((t) => t.name === value);
  const [focus, setFocus] = useState(start >= 0 ? start : 0);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "ArrowDown") { e.preventDefault(); e.stopPropagation(); setFocus((f) => Math.min(f + 1, THEMES.length - 1)); }
      else if (e.key === "ArrowUp") { e.preventDefault(); e.stopPropagation(); setFocus((f) => Math.max(f - 1, 0)); }
      else if (e.key === "Enter") { e.preventDefault(); e.stopPropagation(); onChange(THEMES[focus].name); onClose(); }
      else if (e.key === "Escape") { e.stopPropagation(); onClose(); }
    };
    window.addEventListener("keydown", onKey, true);
    return () => window.removeEventListener("keydown", onKey, true);
  }, [focus, onChange, onClose]);

  return (
    <div className="overlay" onClick={onClose}>
      <div className="palette theme-modal" onClick={(e) => e.stopPropagation()}>
        <div className="palette-head"><Palette size={14} /> Elegir tema</div>
        <div className="theme-options">
          {THEMES.map((t, i) => (
            <button
              key={t.name}
              className={`theme-opt ${i === focus ? "selected" : ""} ${t.name === value ? "current" : ""}`}
              onMouseEnter={() => setFocus(i)}
              onClick={() => { onChange(t.name); onClose(); }}
            >
              <span className="swatch" data-theme-swatch={t.name} />
              <span className="theme-opt-label">{t.label}</span>
              {t.name === value && <Check size={13} className="theme-check" />}
            </button>
          ))}
        </div>
        <div className="palette-head" style={{ borderTop: "1px solid var(--border)", borderBottom: "none" }}>
          <span className="muted" style={{ fontSize: 12 }}>↑/↓ navegar · Enter elegir · Esc cerrar</span>
        </div>
      </div>
    </div>
  );
}
