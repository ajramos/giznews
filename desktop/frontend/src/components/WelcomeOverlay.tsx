import { Keyboard } from "lucide-react";

export function WelcomeOverlay({ onDone }: { onDone: () => void }) {
  return (
    <div className="overlay">
      <div className="palette welcome" onClick={(e) => e.stopPropagation()}>
        <div className="palette-head">
          <Keyboard size={15} /> Bienvenido a GizNews
        </div>
        <div className="welcome-body">
          <p>
            Un lector de noticias de IA que se maneja con <strong>vim</strong> y construye tu
            <strong> knowledge graph</strong> sobre la marcha.
          </p>
          <div className="welcome-tip">
            <kbd>j</kbd>/<kbd>k</kbd> navegar · <kbd>Enter</kbd> leer · <kbd>y</kbd> resumen IA
          </div>
          <div className="welcome-tip">
            <kbd>a</kbd> archivar (con deshacer) · <kbd>m</kbd> destacar · <kbd>t</kbd> leído
          </div>
          <div className="welcome-tip">
            <kbd>s</kbd> búsqueda · <kbd>g</kbd> grafo · <kbd>d</kbd> digest · <kbd>:</kbd> comandos · <kbd>?</kbd> ayuda
          </div>
          <div className="welcome-tip">
            <kbd>v</kbd> selección múltiple · <kbd>5j</kbd> saltar 5 · <kbd>3a</kbd> archivar 3
          </div>
          <p className="muted" style={{ fontSize: 12 }}>
            Nada se borra nunca: archivar es lógico y todo es recuperable.
          </p>
        </div>
        <div className="palette-head" style={{ borderTop: "1px solid var(--border)", borderBottom: "none", justifyContent: "flex-end" }}>
          <button onClick={onDone}>Empezar</button>
        </div>
      </div>
    </div>
  );
}
