import { useCallback, useEffect, useMemo, useState } from "react";
import { Network, Loader2, RefreshCw } from "lucide-react";
import { api } from "../api";
import type { NoteDTO } from "../types";

interface Props {
  noteId: number | null;
  onOpenNote: (id: number) => void;
  onBuild: () => void;
  notify: (msg: string) => void;
}

interface GNode {
  id: number;
  title: string;
  type: string;
  x: number;
  y: number;
  isCenter: boolean;
}

const W = 620;
const H = 400;
const MARGIN = 46;

const TYPE_COLOR: Record<string, string> = {
  atom: "#4f8cff",
  electron: "#a78bfa",
  molecule: "#e3b341",
};

// deterministic PRNG so the layout is stable across renders
function mulberry32(seed: number) {
  let a = seed;
  return () => {
    a |= 0; a = (a + 0x6D2B79F5) | 0;
    let t = Math.imul(a ^ (a >>> 15), 1 | a);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

function layout(nodes: { id: number; title: string; type: string; isCenter: boolean }[]): GNode[] {
  const rnd = mulberry32(42);
  const pts = nodes.map((n) => ({
    ...n,
    x: n.isCenter ? W / 2 : MARGIN + rnd() * (W - 2 * MARGIN),
    y: n.isCenter ? H / 2 : MARGIN + rnd() * (H - 2 * MARGIN),
    vx: 0, vy: 0,
  }));
  // simple force: repulsion + pull to center
  const C = 0.018, R = 9000, SPRING = 0.02;
  for (let iter = 0; iter < 140; iter++) {
    for (let i = 0; i < pts.length; i++) {
      let fx = 0, fy = 0;
      const a = pts[i];
      for (let j = 0; j < pts.length; j++) {
        if (i === j) continue;
        const b = pts[j];
        let dx = a.x - b.x, dy = a.y - b.y;
        let d2 = dx * dx + dy * dy || 1;
        let inv = R / d2;
        fx += dx * inv; fy += dy * inv;
      }
      fx += (W / 2 - a.x) * SPRING;
      fy += (H / 2 - a.y) * SPRING;
      a.vx = (a.vx + fx * C) * 0.85;
      a.vy = (a.vy + fy * C) * 0.85;
      a.x = Math.max(MARGIN, Math.min(W - MARGIN, a.x + a.vx));
      a.y = Math.max(MARGIN, Math.min(H - MARGIN, a.y + a.vy));
    }
  }
  return pts.map((p) => ({ id: p.id, title: p.title, type: p.type, x: p.x, y: p.y, isCenter: p.isCenter }));
}

export function GraphPanel({ noteId, onOpenNote, onBuild, notify }: Props) {
  const [center, setCenter] = useState<NoteDTO | null>(null);
  const [neighbors, setNeighbors] = useState<NoteDTO[]>([]);
  const [loading, setLoading] = useState(false);
  const [hover, setHover] = useState<number | null>(null);

  const load = useCallback(async (id: number) => {
    setLoading(true);
    try {
      const c = await api.getNote(id);
      const nb = await api.graphNeighbors(id);
      setCenter(c);
      setNeighbors(nb);
    } catch (e) {
      notify(String(e));
      setCenter(null);
      setNeighbors([]);
    } finally {
      setLoading(false);
    }
  }, [notify]);

  useEffect(() => {
    if (noteId) void load(noteId);
  }, [noteId, load]);

  const nodes = useMemo<GNode[]>(() => {
    if (!center) return [];
    const list = [{ id: center.id, title: center.title, type: center.type, isCenter: true }];
    for (const n of neighbors) {
      if (n.id !== center.id) list.push({ id: n.id, title: n.title, type: n.type, isCenter: false });
    }
    return layout(list);
  }, [center, neighbors]);

  const legend = useMemo(() => {
    const counts: Record<string, number> = {};
    for (const n of nodes) counts[n.type] = (counts[n.type] ?? 0) + 1;
    return counts;
  }, [nodes]);

  if (loading) return <div className="panel"><div className="empty with-icon"><Loader2 size={24} className="spin" /> Cargando grafo…</div></div>;

  if (!center) {
    return (
      <div className="panel graph-panel">
        <div className="panel-title"><Network size={13} /> Grafo de conocimiento</div>
        <div className="empty with-icon">
          <Network size={32} />
          <span>Este artículo aún no tiene nota.</span>
          <span className="muted">Genera el knowledge graph con <code>:kb build</code></span>
          <button onClick={onBuild}>Generar grafo</button>
        </div>
      </div>
    );
  }

  return (
    <div className="panel graph-panel">
      <div className="panel-title">
        <Network size={13} /> Grafo de conocimiento
        <button className="icon-btn" onClick={() => noteId && void load(noteId)} title="Recargar"><RefreshCw size={13} /></button>
      </div>

      <div className="graph-current">
        <span className="note-type">{center.type}</span>
        <h2>{center.title}</h2>
      </div>

      <svg className="graph-canvas" viewBox={`0 0 ${W} ${H}`} width="100%" height="100%">
        <defs>
          <marker id="arrow" markerWidth="8" markerHeight="8" refX="7" refY="3" orient="auto">
            <path d="M0,0 L0,6 L8,3 z" fill="var(--border)" />
          </marker>
        </defs>
        {nodes.filter((n) => !n.isCenter).map((n) => (
          <line
            key={`e-${n.id}`}
            x1={W / 2} y1={H / 2} x2={n.x} y2={n.y}
            stroke="var(--border)" strokeWidth="1.2" markerEnd="url(#arrow)"
          />
        ))}
        {nodes.map((n) => (
          <g
            key={n.id}
            className="graph-node"
            transform={`translate(${n.x},${n.y})`}
            onMouseEnter={() => setHover(n.id)}
            onMouseLeave={() => setHover(null)}
            onClick={() => !n.isCenter && void load(n.id)}
            onDoubleClick={() => onOpenNote(n.id)}
          >
            <circle
              r={n.isCenter ? 16 : 10}
              fill={TYPE_COLOR[n.type] ?? "var(--fg-dim)"}
              opacity={n.isCenter ? 1 : 0.85}
              stroke={n.isCenter ? "var(--fg)" : "none"}
              strokeWidth={n.isCenter ? 2 : 0}
            />
            {(hover === n.id || n.isCenter || nodes.length <= 9) && (
              <text y={n.isCenter ? 30 : 22} textAnchor="middle" className="graph-label">
                {n.title.length > 24 ? n.title.slice(0, 23) + "…" : n.title}
              </text>
            )}
          </g>
        ))}
      </svg>

      <div className="graph-legend">
        {Object.entries(legend).map(([type, count]) => (
          <span key={type} className="lg">
            <span style={{ width: 10, height: 10, borderRadius: "50%", background: TYPE_COLOR[type] ?? "var(--fg-dim)", display: "inline-block" }} />
            {type} ({count})
          </span>
        ))}
        <span className="muted">· clic = expandir · doble clic = abrir nota</span>
      </div>
    </div>
  );
}
