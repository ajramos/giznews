import { useEffect, useRef, useState } from "react";
import { Ban, CheckCircle2, Loader2, ListChecks, XCircle, type LucideIcon } from "lucide-react";
import { api } from "../api";
import type { JobDTO } from "../types";

interface Props {
  onClose: () => void;
  notify: (msg: string) => void;
}

const STATUS_META: Record<JobDTO["status"], { icon: LucideIcon; spin?: boolean }> = {
  running: { icon: Loader2, spin: true },
  done: { icon: CheckCircle2 },
  error: { icon: XCircle },
  canceled: { icon: Ban },
};

function age(iso: string): string {
  const s = Math.max(0, Math.floor((Date.now() - new Date(iso).getTime()) / 1000));
  if (s < 60) return `${s}s ago`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m ago`;
  return `${Math.floor(m / 60)}h ago`;
}

function took(j: JobDTO): string {
  if (!j.finishedAt) return "";
  const ms = new Date(j.finishedAt).getTime() - new Date(j.createdAt).getTime();
  if (ms < 0) return "";
  const s = Math.round(ms / 1000);
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  return `${m}m ${s % 60}s`;
}

function detail(j: JobDTO): string {
  const parts = [`${j.name} — ${j.status}`];
  if (j.phase && j.total > 0) parts.push(`${j.phase} ${j.done}/${j.total}`);
  const t = took(j);
  if (t) parts.push(`took ${t}`);
  const msg = j.message || j.errMsg;
  if (msg) parts.push(msg);
  return parts.join(" · ");
}

// JobsPanel: browse background jobs (fetch/classify/kb/index/bulk/…). It polls
// the backend registry so progress is visible in both Wails and mock modes.
//   j/k navigate · Enter details · d remove · c clear finished · x cancel · Esc close
export function JobsPanel({ onClose, notify }: Props) {
  const [jobs, setJobs] = useState<JobDTO[]>([]);
  const [focus, setFocus] = useState(0);
  const jobsRef = useRef<JobDTO[]>(jobs);
  jobsRef.current = jobs;

  useEffect(() => {
    let alive = true;
    const load = () => api.listJobs().then((j) => { if (alive) setJobs(j); }).catch(() => {});
    load();
    const t = window.setInterval(load, 700);
    return () => { alive = false; window.clearInterval(t); };
  }, []);

  useEffect(() => {
    setFocus((f) => Math.min(f, Math.max(0, jobs.length - 1)));
  }, [jobs.length]);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const list = jobsRef.current;
      const n = list.length;
      if (e.key === "j" || e.key === "ArrowDown") { e.preventDefault(); e.stopPropagation(); setFocus((f) => Math.min(f + 1, n - 1)); }
      else if (e.key === "k" || e.key === "ArrowUp") { e.preventDefault(); e.stopPropagation(); setFocus((f) => Math.max(f - 1, 0)); }
      else if (e.key === "Enter") { e.preventDefault(); e.stopPropagation(); const j = list[focus]; if (j) notify(detail(j)); }
      else if (e.key === "d") { e.preventDefault(); e.stopPropagation(); const j = list[focus]; if (j) void api.removeJob(j.id).then(() => setJobs((p) => p.filter((x) => x.id !== j.id))).catch((err) => notify(String(err))); }
      else if (e.key === "c") { e.preventDefault(); e.stopPropagation(); void api.clearFinishedJobs().then(() => setJobs((p) => p.filter((x) => x.status === "running"))).catch((err) => notify(String(err))); }
      else if (e.key === "x") { e.preventDefault(); e.stopPropagation(); const j = list[focus]; if (j && j.status === "running") { void api.cancelJob(j.id).catch((err) => notify(String(err))); notify("Cancel requested"); } }
      else if (e.key === "Escape") { e.preventDefault(); e.stopPropagation(); onClose(); }
    };
    window.addEventListener("keydown", onKey, true);
    return () => window.removeEventListener("keydown", onKey, true);
  }, [focus, onClose, notify]);

  return (
    <div className="overlay" onClick={onClose}>
      <div className="palette jobs-picker" onClick={(e) => e.stopPropagation()}>
        <div className="palette-head">
          <ListChecks size={14} /> Background jobs ({jobs.length})
          <span className="muted" style={{ marginLeft: "auto", fontSize: 12 }}>
            {jobs.filter((j) => j.status === "running").length} running
          </span>
        </div>
        <div className="jobs-list">
          {jobs.length === 0 && (
            <div className="palette-empty">No background jobs yet. Run a long command (e.g. <kbd>:classify</kbd>) to see one here.</div>
          )}
          {jobs.map((j, i) => {
            const meta = STATUS_META[j.status];
            const Icon = meta.icon;
            const pct = j.total > 0 ? Math.round((j.done / j.total) * 100) : 0;
            const sub = j.errMsg
              ? j.errMsg
              : [j.phase && j.total > 0 ? `${j.phase} ${j.done}/${j.total}` : (j.phase || j.message || ""), age(j.createdAt)].filter(Boolean).join(" · ");
            return (
              <button
                key={j.id}
                className={`job-item ${i === focus ? "selected" : ""}`}
                onMouseEnter={() => setFocus(i)}
                onClick={() => notify(detail(j))}
              >
                <span className="job-row">
                  <span className="sp-type"><Icon size={13} className={meta.spin ? "spin" : ""} /></span>
                  <span className="job-name">{j.name}</span>
                  <span className={`job-status ${j.status}`}>{j.status}</span>
                </span>
                <span className="job-sub">{sub}</span>
                {j.status === "running" && j.total > 0 && (
                  <span className="job-bar"><span className="job-bar-fill" style={{ width: `${pct}%` }} /></span>
                )}
              </button>
            );
          })}
        </div>
        <div className="palette-head" style={{ borderTop: "1px solid var(--border)", borderBottom: "none", flexWrap: "wrap", gap: "4px 14px" }}>
          <span><kbd>j/k</kbd> navigate</span>
          <span><kbd>Enter</kbd> details</span>
          <span><kbd>d</kbd> remove</span>
          <span><kbd>c</kbd> clear finished</span>
          <span><kbd>x</kbd> cancel</span>
          <span><kbd>Esc</kbd> close</span>
        </div>
      </div>
    </div>
  );
}
