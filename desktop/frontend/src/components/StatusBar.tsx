import { useContext, createContext } from "react";
import { CONTEXT_KEYS } from "../keys";

export type UIContext = "list" | "reader" | "search" | "graph" | "digest" | "vault";

const Ctx = createContext<UIContext>("list");
export const useUIContext = () => useContext(Ctx);

// Bottom status bar: mode/filter/count on the left, contextual keys, and
// LLM + auto-refresh state on the right.
export function StatusBar({
  context,
  modeLabel,
  filter,
  count,
  rangeStatus,
  bulk,
  bulkCount,
  autoRefresh,
  llmOn,
  llmReachable,
  llmProvider,
  onToggleAuto,
}: {
  context: UIContext;
  modeLabel: string;
  filter?: string;
  count?: number;
  rangeStatus?: string;
  bulk: boolean;
  bulkCount: number;
  autoRefresh: boolean;
  llmOn: boolean;
  llmReachable: boolean;
  llmProvider: string;
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
        {rangeStatus && <span className="range-badge">{rangeStatus}</span>}
        {count !== undefined && <span className="count-badge">{count}</span>}
      </div>
      <div className="sb-keys">
        {bulk
          ? [
              { key: "space", label: "toggle" },
              { key: "j/k", label: "navigate" },
              { key: "a", label: "archive" },
              { key: "t", label: "read" },
              { key: "m", label: "star" },
              { key: "c", label: "classify" },
              { key: "p", label: "note" },
              { key: "y", label: "summarize" },
              { key: "Esc/v", label: "exit" },
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
        <button className="pill auto" title="Auto-refresh every 15 min" onClick={onToggleAuto}>
          {autoRefresh ? "auto 15m ✓" : "auto off"}
        </button>
        {llmOn && llmReachable ? (
          <span className="pill llm on" title={`LLM connected (${llmProvider})`}>● {llmProvider}</span>
        ) : llmOn ? (
          <span className="pill llm off" title="LLM configured but not responding">○ {llmProvider}</span>
        ) : (
          <span className="pill" title="LLM disabled in config.json (llm.enabled)">LLM off</span>
        )}
      </div>
    </footer>
  );
}
