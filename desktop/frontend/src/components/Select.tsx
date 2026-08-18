import { useEffect, useRef, useState } from "react";
import { ChevronDown, Check } from "lucide-react";

export interface SelectOption {
  value: string;
  label: string;
}

interface Props {
  value: string;
  options: SelectOption[];
  onChange: (v: string) => void;
  title?: string;
  className?: string;
}

// Select: a custom dropdown (native <select> ignores styling in WKWebView).
// Keyboard: ↑/↓ navigate · Enter pick · Esc close.
export function Select({ value, options, onChange, title, className }: Props) {
  const [open, setOpen] = useState(false);
  const [focus, setFocus] = useState(0);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (open) setFocus(Math.max(0, options.findIndex((o) => o.value === value)));
  }, [open, value, options]);

  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      if (!ref.current?.contains(e.target as Node)) setOpen(false);
    };
    window.addEventListener("mousedown", onDown);
    return () => window.removeEventListener("mousedown", onDown);
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "ArrowDown") { e.preventDefault(); e.stopPropagation(); setFocus((f) => Math.min(f + 1, options.length - 1)); }
      else if (e.key === "ArrowUp") { e.preventDefault(); e.stopPropagation(); setFocus((f) => Math.max(f - 1, 0)); }
      else if (e.key === "Enter") { e.preventDefault(); e.stopPropagation(); const o = options[focus]; if (o) { onChange(o.value); setOpen(false); } }
      else if (e.key === "Escape") { e.preventDefault(); e.stopPropagation(); setOpen(false); }
    };
    window.addEventListener("keydown", onKey, true);
    return () => window.removeEventListener("keydown", onKey, true);
  }, [open, focus, options, onChange]);

  const current = options.find((o) => o.value === value);

  return (
    <div className={`select ${className ?? ""}`} ref={ref}>
      <button type="button" className="select-btn" onClick={() => setOpen((o) => !o)} title={title}>
        <span className="select-value">{current?.label ?? value}</span>
        <ChevronDown size={14} className="select-caret" />
      </button>
      {open && (
        <div className="select-pop">
          {options.map((o, i) => (
            <button
              type="button"
              key={o.value}
              className={`select-opt ${i === focus ? "selected" : ""} ${o.value === value ? "current" : ""}`}
              onMouseEnter={() => setFocus(i)}
              onClick={() => { onChange(o.value); setOpen(false); }}
            >
              <span className="select-opt-label">{o.label}</span>
              {o.value === value && <Check size={13} />}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
