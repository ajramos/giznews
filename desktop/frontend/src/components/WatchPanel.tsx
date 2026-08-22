import { useEffect, useState } from "react";
import { Bell, BellOff, FileText } from "lucide-react";
import { api } from "../api";
import type { RuleDTO, WatchHitDTO } from "../types";

interface Props {
  onOpen: (articleID: number) => void;
  onClose: () => void;
  notify: (msg: string) => void;
}

// WatchPanel: what you asked to be told about, and what it has caught. A watch
// is an ordinary rule with a notify action, so it is written, tested and
// versioned like every other rule — this only shows what they found.
export function WatchPanel({ onOpen, onClose, notify }: Props) {
  const [watches, setWatches] = useState<RuleDTO[]>([]);
  const [hits, setHits] = useState<WatchHitDTO[]>([]);
  const [focus, setFocus] = useState(0);

  useEffect(() => {
    void (async () => {
      try {
        const [rules, caught] = await Promise.all([api.listRules(), api.listWatchHits(false)]);
        setWatches(rules.filter((r) => r.actions.some((a) => a.type === "notify")));
        setHits(caught);
        // Seeing them is being told: nothing announces these again.
        const unseen = caught.filter((h) => !h.seen).map((h) => h.article.id);
        if (unseen.length > 0) await api.markWatchHitsSeen(unseen);
      } catch (e) {
        notify(String(e));
      }
    })();
  }, [notify]);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "j" || e.key === "ArrowDown") { e.preventDefault(); e.stopPropagation(); setFocus((f) => Math.min(f + 1, hits.length - 1)); }
      else if (e.key === "k" || e.key === "ArrowUp") { e.preventDefault(); e.stopPropagation(); setFocus((f) => Math.max(f - 1, 0)); }
      else if (e.key === "Enter") { e.preventDefault(); e.stopPropagation(); if (hits[focus]) onOpen(hits[focus].article.id); }
      else if (e.key === "Escape") { e.preventDefault(); e.stopPropagation(); onClose(); }
    };
    window.addEventListener("keydown", onKey, true);
    return () => window.removeEventListener("keydown", onKey, true);
  }, [hits, focus, onOpen, onClose]);

  return (
    <div className="overlay" onClick={onClose}>
      <div className="palette source-picker" onClick={(e) => e.stopPropagation()}>
        <div className="palette-head">
          <Bell size={14} /> Watchlist · {watches.length} watch(es), {hits.length} caught
        </div>
        {watches.length === 0 && (
          <div className="palette-empty" data-testid="no-watches">
            No watches yet. A watch is a rule with a <kbd>notify</kbd> action:
            <br />
            <code>giznews rules add --name "gpt-5 ships" --query "gpt-?5" --notify</code>
          </div>
        )}
        {watches.length > 0 && (
          <div className="watch-rules" data-testid="watch-rules">
            {watches.map((w) => (
              <span key={w.id} className="watch-chip" data-on={w.enabled}>
                {w.enabled ? <Bell size={11} /> : <BellOff size={11} />} {w.name}
              </span>
            ))}
          </div>
        )}
        <div className="source-picker-list">
          {hits.length === 0 && <div className="palette-empty">Nothing caught yet.</div>}
          {hits.map((h, i) => (
            <div
              key={h.article.id}
              className={`source-picker-item ${i === focus ? "selected" : ""}`}
              data-testid="watch-hit"
              onMouseEnter={() => setFocus(i)}
              onClick={() => onOpen(h.article.id)}
            >
              <span className="sp-dot" data-on={!h.seen} />
              <span className="sp-type"><FileText size={13} /></span>
              <span className="sp-name">{h.article.title}</span>
              <span className="sp-meta">{h.rule}</span>
              <span className="sp-state">{h.article.sourceName}</span>
            </div>
          ))}
        </div>
        <div className="palette-head" style={{ borderTop: "1px solid var(--border)", borderBottom: "none", flexWrap: "wrap", gap: "4px 14px" }}>
          <span><kbd>j/k</kbd> navigate</span>
          <span><kbd>Enter</kbd> open</span>
          <span><kbd>Esc</kbd> close</span>
        </div>
      </div>
    </div>
  );
}
