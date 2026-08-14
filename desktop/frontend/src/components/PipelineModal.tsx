import { Check, Loader2, X, Download, Circle } from "lucide-react";
import type { ReactNode } from "react";

export interface PipelineStep {
  key: string;
  label: string;
  icon: ReactNode;
  status: "pending" | "running" | "done" | "error";
  summary?: string;
}

interface Props {
  steps: PipelineStep[];
  onClose: () => void;
}

function StatusIcon({ status }: { status: PipelineStep["status"] }) {
  switch (status) {
    case "done": return <Check size={15} className="pl-status done" />;
    case "running": return <Loader2 size={15} className="spin pl-status running" />;
    case "error": return <X size={15} className="pl-status error" />;
    default: return <Circle size={9} className="pl-status pending" />;
  }
}

// PipelineModal shows the progress + result of the article-processing pipeline
// (fetch → classify → knowledge graph) with per-step counts.
export function PipelineModal({ steps, onClose }: Props) {
  const finished = steps.every((s) => s.status === "done" || s.status === "error");
  return (
    <div className="overlay" onClick={onClose}>
      <div className="palette pipeline-modal" onClick={(e) => e.stopPropagation()}>
        <div className="palette-head"><Download size={14} /> Procesar artículos</div>
        <div className="pipeline-steps">
          {steps.map((s) => (
            <div key={s.key} className={`pipeline-step ${s.status}`}>
              <span className="pl-icon">{s.icon}</span>
              <span className="pl-label">{s.label}</span>
              <span className="pl-summary">{s.summary ?? ""}</span>
              <StatusIcon status={s.status} />
            </div>
          ))}
        </div>
        <div className="palette-head" style={{ borderTop: "1px solid var(--border)", borderBottom: "none", justifyContent: "flex-end" }}>
          {finished ? (
            <button onClick={onClose}>Cerrar</button>
          ) : (
            <span className="muted" style={{ fontSize: 12 }}>Procesando…</span>
          )}
        </div>
      </div>
    </div>
  );
}
