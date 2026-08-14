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
  bulk,
  bulkCount,
  autoRefresh,
  llmOn,
  onToggleAuto,
}: {
  context: UIContext;
  modeLabel: string;
  filter?: string;
  count?: number;
  bulk: boolean;
  bulkCount: number;
  autoRefresh: boolean;
  llmOn: boolean;
  onToggleAuto: () => void;
}) {
  const keys = CONTEXT_KEYS[context] ?? CONTEXT_KEYS.list;
  return (
    <footer className="statusbar">
      <div className="sb-context">
        {bulk ? (
          <span className="mode bulk">BULK ({bulkCount})</span>
        ) : (
          <span className="mode">{modeLabel}</span>
        )}
        {filter && <span className="muted">· {filter}</span>}
        {count !== undefined && <span className="count-badge">{count}</span>}
      </div>
      <div className="sb-keys">
        {bulk
          ? [
              { key: "j/k", label: "extender" },
              { key: "a", label: "archivar selección" },
              { key: "t", label: "leído selección" },
              { key: "Esc/v", label: "salir" },
            ].map((k) => (
              <span key={k.key + k.label} className="sb-key"><kbd>{k.key}</kbd> {k.label}</span>
            ))
          : keys.map((k) => (
              <span key={k.key + k.label} className="sb-key">
                <kbd>{k.key}</kbd> {k.label}
              </span>
            ))}
      </div>
      <div className="sb-right">
        <button className="pill auto" title="Auto-refresh cada 15 min" onClick={onToggleAuto}>
          {autoRefresh ? "auto 15m ✓" : "auto off"}
        </button>
        <span className={`pill llm ${llmOn ? "on" : "off"}`}>⦿ {llmOn ? "ollama" : "sin LLM"}</span>
      </div>
    </footer>
  );
}
