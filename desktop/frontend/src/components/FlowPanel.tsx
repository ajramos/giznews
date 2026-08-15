import { useEffect, useState } from "react";
import { RefreshCw, Rss, Download, Tag, Network, FolderOpen, Search, Loader2, GitBranch } from "lucide-react";
import { api } from "../api";
import type { FlowStatus } from "../types";

interface Props {
  onClose: () => void;
}

interface Node {
  key: string;
  label: string;
  icon: React.ReactNode;
  lines: string[];
  active?: boolean;
}

// FlowPanel shows the end-to-end pipeline with live counts per stage.
export function FlowPanel({ onClose }: Props) {
  const [flow, setFlow] = useState<FlowStatus | null>(null);

  const load = () => api.flow().then(setFlow).catch(() => {});
  useEffect(() => {
    load();
    const iv = window.setInterval(load, 4000);
    return () => window.clearInterval(iv);
  }, []);

  const nodes: Node[] = flow
    ? [
        { key: "sources", label: "Sources", icon: <Rss size={15} />, lines: [`${flow.sourcesEnabled}/${flow.sourcesTotal} enabled`] },
        { key: "fetch", label: "Fetch", icon: <Download size={15} />, lines: [`${flow.articlesTotal} articles`] },
        { key: "classify", label: "Classify", icon: <Tag size={15} />, lines: [`${flow.classified} classified`, `${flow.pendingClassify} pending`], active: flow.pendingClassify > 0 },
        { key: "kb", label: "KB build", icon: <Network size={15} />, lines: [`${flow.atoms} atoms · ${flow.electrons} electrons`, `${flow.molecules} molecules`] },
        { key: "vault", label: "Vault", icon: <FolderOpen size={15} />, lines: [flow.vaultPath.replace(/^.*\//, "~/…/")] },
        { key: "search", label: "Search", icon: <Search size={15} />, lines: [`${flow.notesEmbedded} notes · ${flow.articlesEmbedded} articles`] },
      ]
    : [];

  return (
    <div className="overlay" onClick={onClose}>
      <div className="palette flow-panel" onClick={(e) => e.stopPropagation()}>
        <div className="palette-head">
          <GitBranch size={14} /> Pipeline flow
          <span className="muted" style={{ marginLeft: "auto", fontSize: 12 }}>
            {flow && flow.runningJobs > 0 ? `${flow.runningJobs} job(s) running` : "idle"}
          </span>
          <button className="icon-btn" onClick={load} title="Refresh"><RefreshCw size={13} /></button>
        </div>
        <div className="flow-body">
          {!flow ? (
            <div className="empty with-icon"><Loader2 size={22} className="spin" /> Loading…</div>
          ) : (
            <div className="flow-nodes">
              {nodes.map((n, i) => (
                <div key={n.key} className="flow-node-wrap">
                  <div className={`flow-node ${n.active ? "active" : ""}`}>
                    <span className="flow-node-icon">{n.icon}</span>
                    <span className="flow-node-label">{n.label}</span>
                    <span className="flow-node-lines">
                      {n.lines.map((l, j) => <span key={j} className="flow-line">{l}</span>)}
                    </span>
                  </div>
                  {i < nodes.length - 1 && <span className="flow-arrow">→</span>}
                </div>
              ))}
            </div>
          )}
        </div>
        <div className="palette-head" style={{ borderTop: "1px solid var(--border)", borderBottom: "none", justifyContent: "flex-end" }}>
          <span className="muted" style={{ fontSize: 12 }}>Sources → Fetch → Classify → KB → Vault · Search · <kbd>Esc</kbd> close</span>
        </div>
      </div>
    </div>
  );
}
