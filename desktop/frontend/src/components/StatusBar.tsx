import { useContext, createContext } from "react";
import { CONTEXT_KEYS } from "../keys";

export type UIContext = "list" | "reader" | "search" | "graph" | "digest";

const Ctx = createContext<UIContext>("list");
export const useUIContext = () => useContext(Ctx);

export function StatusBar({
  context,
  modeLabel,
  filter,
  count,
  llmOn,
}: {
  context: UIContext;
  modeLabel: string;
  filter?: string;
  count?: number;
  llmOn: boolean;
}) {
  const keys = CONTEXT_KEYS[context] ?? CONTEXT_KEYS.list;
  return (
    <footer className="statusbar">
      <div className="sb-context">
        <span className="mode">{modeLabel}</span>
        {filter && <span className="muted">· {filter}</span>}
        {count !== undefined && <span className="count-badge">{count}</span>}
      </div>
      <div className="sb-keys">
        {keys.map((k) => (
          <span key={k.key + k.label} className="sb-key">
            <kbd>{k.key}</kbd> {k.label}
          </span>
        ))}
      </div>
      <div className="sb-right">
        <span className={`pill llm ${llmOn ? "on" : "off"}`}>⦿ {llmOn ? "ollama" : "sin LLM"}</span>
      </div>
    </footer>
  );
}
