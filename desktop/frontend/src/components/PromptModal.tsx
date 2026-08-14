import { useEffect, useRef, useState } from "react";
import { CornerDownLeft } from "lucide-react";

interface Props {
  title: string;
  placeholder: string;
  initial?: string;
  onSubmit: (value: string) => void;
  onClose: () => void;
}

// PromptModal: a keyboard-driven text prompt (replaces native window.prompt,
// which is unreliable inside WKWebView). Enter submits, Esc cancels.
export function PromptModal({ title, placeholder, initial = "", onSubmit, onClose }: Props) {
  const [value, setValue] = useState(initial);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    inputRef.current?.focus();
  }, []);

  const submit = () => {
    const v = value.trim();
    if (v) onSubmit(v);
    else onClose();
  };

  return (
    <div className="overlay" onClick={onClose}>
      <div className="palette prompt-modal" onClick={(e) => e.stopPropagation()}>
        <div className="palette-head">{title}</div>
        <input
          ref={inputRef}
          value={value}
          onChange={(e) => setValue(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") { e.preventDefault(); submit(); }
            else if (e.key === "Escape") { e.stopPropagation(); onClose(); }
          }}
          placeholder={placeholder}
        />
        <div className="palette-head" style={{ borderTop: "1px solid var(--border)", borderBottom: "none", justifyContent: "flex-end" }}>
          <span className="muted" style={{ fontSize: 12, display: "inline-flex", alignItems: "center", gap: 6 }}>
            <CornerDownLeft size={12} /> Enter para confirmar
          </span>
        </div>
      </div>
    </div>
  );
}
