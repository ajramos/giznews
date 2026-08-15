import { useEffect, useState } from "react";
import { Info, FileText, GitBranch, FlaskConical, Database, FolderOpen, Cpu } from "lucide-react";
import { api } from "../api";
import type { NoteDTO, StatusDTO } from "../types";

interface Props {
  onClose: () => void;
}

export function StatusModal({ onClose }: Props) {
  const [status, setStatus] = useState<StatusDTO | null>(null);
  const [counts, setCounts] = useState<Record<string, number>>({});

  useEffect(() => {
    api.status().then(setStatus).catch(() => {});
    api.listNotes("").then((ns: NoteDTO[]) => {
      const c: Record<string, number> = {};
      for (const n of ns) c[n.type] = (c[n.type] ?? 0) + 1;
      setCounts(c);
    }).catch(() => {});
  }, []);

  const notesTotal = Object.values(counts).reduce((a, b) => a + b, 0);

  return (
    <div className="overlay" onClick={onClose}>
      <div className="palette status-modal" onClick={(e) => e.stopPropagation()}>
        <div className="palette-head"><Info size={14} /> Status</div>
        <div className="status-body">
          <div className="status-row">
            <span className="sr-label">Articles</span>
            <span>{status ? `${status.unreadArticles} unread · ${status.totalArticles} total` : "…"}</span>
          </div>
          <div className="status-row">
            <span className="sr-label"><Info size={13} /> Pending classification</span>
            <span>{status ? status.pendingClassify : "…"}</span>
          </div>
          <div className="status-row">
            <span className="sr-label"><FileText size={13} /> Atoms</span>
            <span>{counts.atom ?? 0}</span>
          </div>
          <div className="status-row">
            <span className="sr-label"><GitBranch size={13} /> Electrons</span>
            <span>{counts.electron ?? 0}</span>
          </div>
          <div className="status-row">
            <span className="sr-label"><FlaskConical size={13} /> Molecules</span>
            <span>{counts.molecule ?? 0}</span>
          </div>
          <div className="status-row">
            <span className="sr-label"><Database size={13} /> Total notes</span>
            <span>{notesTotal}</span>
          </div>
          <div className="status-row">
            <span className="sr-label"><Cpu size={13} /> LLM</span>
            <span>{status ? `${status.llmProvider} (${status.llmReachable ? "connected" : status.llmEnabled ? "offline" : "off"})` : "…"}</span>
          </div>
          <div className="status-row">
            <span className="sr-label"><FolderOpen size={13} /> Vault</span>
            <span className="sr-path">{status?.vaultPath ?? "…"}</span>
          </div>
        </div>
        <div className="palette-head" style={{ borderTop: "1px solid var(--border)", borderBottom: "none", justifyContent: "flex-end" }}>
          <button onClick={onClose}>Close</button>
        </div>
      </div>
    </div>
  );
}
