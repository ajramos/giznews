import { useContext, createContext } from "react";

export type UIContext = "list" | "reader" | "search" | "graph" | "digest";

const Ctx = createContext<UIContext>("list");
export const useUIContext = () => useContext(Ctx);

// Minimal status bar: mode/filter on the left, LLM + auto-refresh on the right.
// Shortcuts live in the ? help overlay instead of a noisy key row.
export function StatusBar({
  modeLabel,
  filter,
  count,
  bulk,
  bulkCount,
  autoRefresh,
  llmOn,
  llmReachable,
  llmProvider,
  onToggleAuto,
}: {
  modeLabel: string;
  filter?: string;
  count?: number;
  bulk: boolean;
  bulkCount: number;
  autoRefresh: boolean;
  llmOn: boolean;
  llmReachable: boolean;
  llmProvider: string;
  onToggleAuto: () => void;
}) {
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
      <div className="sb-right">
        <button className="pill auto" title="Auto-refresh cada 15 min" onClick={onToggleAuto}>
          {autoRefresh ? "auto 15m ✓" : "auto off"}
        </button>
        {llmOn && llmReachable ? (
          <span className="pill llm on" title={`LLM conectado (${llmProvider})`}>● {llmProvider}</span>
        ) : llmOn ? (
          <span className="pill llm off" title="LLM configurado pero no responde">○ {llmProvider}</span>
        ) : (
          <span className="pill" title="LLM desactivado en config.json (llm.enabled)">LLM off</span>
        )}
      </div>
    </footer>
  );
}
