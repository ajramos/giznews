import { CircleHelp } from "lucide-react";
import { HELP } from "../keys";

export function HelpOverlay({ onClose }: { onClose: () => void }) {
  return (
    <div className="overlay" onClick={onClose}>
      <div className="palette help" onClick={(e) => e.stopPropagation()}>
        <div className="palette-head"><CircleHelp size={14} /> Keyboard shortcuts</div>
        <div className="help-list">
          {HELP.map((cat) => (
            <div key={cat.title}>
              <div className="help-cat">{cat.title}</div>
              {cat.rows.map((r) => (
                <div key={r.keys} className="help-row">
                  <span className="keys"><kbd>{r.keys}</kbd></span>
                  <span>{r.label}</span>
                </div>
              ))}
            </div>
          ))}
        </div>
        <div className="help-note">
          <strong>Legend:</strong> ★ = importance (0-3, ★★☆/★★★ = relevant or key) · ● = unread · <span className="star-badge">★</span> = starred · 🗄/strikethrough = archived.
          <br /><strong>Archiving is logical:</strong> articles are never physically deleted; everything is recoverable from the archived view or the undo toast.
        </div>
      </div>
    </div>
  );
}
