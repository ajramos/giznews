import { useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { Palette, Check } from "lucide-react";
import { THEMES, type ThemeName } from "../theme";

interface Props {
  value: ThemeName;
  onChange: (t: ThemeName) => void;
}

export function ThemePicker({ value, onChange }: Props) {
  const [open, setOpen] = useState(false);
  const [focus, setFocus] = useState(0);
  const [pos, setPos] = useState<{ top: number; right: number } | null>(null);
  const btnRef = useRef<HTMLButtonElement>(null);
  const popRef = useRef<HTMLDivElement>(null);

  const cur = THEMES.find((t) => t.name === value) ?? THEMES[0];

  const openPicker = () => {
    const r = btnRef.current?.getBoundingClientRect();
    if (r) {
      setPos({ top: r.bottom + 6, right: Math.max(8, window.innerWidth - r.right) });
    }
    setOpen(true);
  };

  // Key handling in CAPTURE phase + stopPropagation so it never reaches the
  // app-wide vim handler while the picker is open.
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "ArrowDown") { e.preventDefault(); e.stopPropagation(); setFocus((f) => Math.min(f + 1, THEMES.length - 1)); }
      else if (e.key === "ArrowUp") { e.preventDefault(); e.stopPropagation(); setFocus((f) => Math.max(f - 1, 0)); }
      else if (e.key === "Enter") { e.preventDefault(); e.stopPropagation(); onChange(THEMES[focus].name); setOpen(false); }
      else if (e.key === "Escape") { e.preventDefault(); e.stopPropagation(); setOpen(false); }
    };
    const onDown = (e: MouseEvent) => {
      const t = e.target as Node;
      if (btnRef.current?.contains(t) || popRef.current?.contains(t)) return;
      setOpen(false);
    };
    window.addEventListener("keydown", onKey, true);
    window.addEventListener("mousedown", onDown);
    return () => {
      window.removeEventListener("keydown", onKey, true);
      window.removeEventListener("mousedown", onDown);
    };
  }, [open, focus, onChange]);

  return (
    <>
      <span className="theme-picker">
        <button ref={btnRef} className="icon-btn" onClick={() => (open ? setOpen(false) : openPicker())} title="Theme (picker)">
          <Palette size={15} />
          <span className="theme-name">{cur.label}</span>
        </button>
      </span>
      {open && pos &&
        createPortal(
          <div
            ref={popRef}
            className="theme-pop"
            style={{ top: pos.top, right: pos.right }}
            onMouseDown={(e) => e.stopPropagation()}
          >
            {THEMES.map((t, i) => (
              <button
                key={t.name}
                className={`theme-opt ${i === focus ? "selected" : ""} ${t.name === value ? "current" : ""}`}
                onMouseEnter={() => setFocus(i)}
                onClick={() => { onChange(t.name); setOpen(false); }}
              >
                <span className="swatch" data-theme-swatch={t.name} />
                <span className="theme-opt-label">{t.label}</span>
                {t.name === value && <Check size={13} className="theme-check" />}
              </button>
            ))}
          </div>,
          document.body
        )}
    </>
  );
}
