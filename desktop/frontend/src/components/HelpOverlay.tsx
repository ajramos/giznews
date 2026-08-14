import { HELP_ROWS } from "../keys";

export function HelpOverlay({ onClose }: { onClose: () => void }) {
  return (
    <div className="palette-overlay" onClick={onClose}>
      <div className="palette help" onClick={(e) => e.stopPropagation()}>
        <div className="panel-title">Atajos de teclado</div>
        <div className="help-list">
          {HELP_ROWS.map((r) => (
            <div key={r.key} className="help-row">
              <kbd>{r.key}</kbd>
              <span>{r.label}</span>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
