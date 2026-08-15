import { Keyboard } from "lucide-react";

export function WelcomeOverlay({ onDone }: { onDone: () => void }) {
  return (
    <div className="overlay">
      <div className="palette welcome" onClick={(e) => e.stopPropagation()}>
        <div className="palette-head">
          <Keyboard size={15} /> Welcome to GizNews
        </div>
        <div className="welcome-body">
          <p>
            An AI news reader driven by <strong>vim</strong> keys that builds your
            <strong> knowledge graph</strong> as you go.
          </p>
          <div className="welcome-tip">
            <kbd>j</kbd>/<kbd>k</kbd> navigate · <kbd>Enter</kbd> read · <kbd>y</kbd> AI summary
          </div>
          <div className="welcome-tip">
            <kbd>a</kbd> archive (with undo) · <kbd>m</kbd> star · <kbd>t</kbd> read
          </div>
          <div className="welcome-tip">
            <kbd>s</kbd> search · <kbd>g</kbd> graph · <kbd>d</kbd> digest · <kbd>:</kbd> commands · <kbd>?</kbd> help
          </div>
          <div className="welcome-tip">
            <kbd>v</kbd> bulk selection · <kbd>5j</kbd> jump 5 · <kbd>3a</kbd> archive 3
          </div>
          <p className="muted" style={{ fontSize: 12 }}>
            Nothing is ever deleted: archiving is logical and everything is recoverable.
          </p>
        </div>
        <div className="palette-head" style={{ borderTop: "1px solid var(--border)", borderBottom: "none", justifyContent: "flex-end" }}>
          <button onClick={onDone}>Get started</button>
        </div>
      </div>
    </div>
  );
}
