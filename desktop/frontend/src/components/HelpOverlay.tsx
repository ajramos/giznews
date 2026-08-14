import { CircleHelp } from "lucide-react";
import { HELP } from "../keys";

export function HelpOverlay({ onClose }: { onClose: () => void }) {
  return (
    <div className="overlay" onClick={onClose}>
      <div className="palette help" onClick={(e) => e.stopPropagation()}>
        <div className="palette-head"><CircleHelp size={14} /> Atajos de teclado</div>
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
          <strong>Archivar es lógico:</strong> los artículos nunca se borran físicamente; todo es recuperable desde la vista de archivados o con el toast de deshacer.
        </div>
      </div>
    </div>
  );
}
