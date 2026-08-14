import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Network, Loader2, RefreshCw, FolderOpen } from "lucide-react";
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
  r: number;
  isCenter: boolean;
}

const W = 660;
const H = 440;
const MARGIN = 60;

const TYPE_COLOR: Record<string, string> = {
  atom: "var(--accent)",
  electron: "var(--cat-research)",
  molecule: "var(--cat-funding)",
  inbox: "var(--fg-dim)",
};

// deterministic PRNG so the layout is stable across renders
function mulberry32(seed: number) {
  let a = seed;
  return () => {
    a |= 0; a = (a + 0x6d2b79f5) | 0;
    let t = Math.imul(a ^ (a >>> 15), 1 | a);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

interface SimNode extends GNode {
  vx: number;
  vy: number;
}

// Fruchterman–Reingold force layout: Coulomb repulsion + edge springs +
// collision avoidance + gravity, with cooling. Produces a clean, stable,
// non-overlapping layout for the small star-shaped knowledge graph.
function layout(input: { id: number; title: string; type: string; isCenter: boolean }[]): GNode[] {
  const rnd = mulberry32(42);
  const pts: SimNode[] = input.map((n) => ({
    ...n,
    x: 0, y: 0, vx: 0, vy: 0,
    r: n.isCenter ? 24 : 15,
  }));

  // initial placement: centre node at centre, others on a circle around it
  pts.forEach((p, i) => {
    if (p.isCenter) {
      p.x = W / 2; p.y = H / 2;
    } else {
      const ang = (i / Math.max(1, pts.length)) * Math.PI * 2 - Math.PI / 2;
      p.x = W / 2 + Math.cos(ang) * 150;
      p.y = H / 2 + Math.sin(ang) * 150;
    }
  });

  const K = 160; // ideal edge length
  const ITER = 320;

  for (let iter = 0; iter < ITER; iter++) {
    const t = Math.max(0.02, 1 - iter / ITER); // cooling factor

    // repulsion + collision between every pair
    for (let i = 0; i < pts.length; i++) {
      for (let j = i + 1; j < pts.length; j++) {
        const a = pts[i], b = pts[j];
        let dx = a.x - b.x, dy = a.y - b.y;
        let d2 = dx * dx + dy * dy;
        if (d2 < 0.01) { dx = rnd() - 0.5; dy = rnd() - 0.5; d2 = dx * dx + dy * dy; }
        const d = Math.sqrt(d2);
        const rep = (K * K) / d * 0.9 * t;
        const fx = (dx / d) * rep, fy = (dy / d) * rep;
        a.vx += fx; a.vy += fy;
        b.vx -= fx; b.vy -= fy;

        const minD = a.r + b.r + 14;
        if (d < minD) {
          const push = ((minD - d) / 2) * 0.9;
          a.x += (dx / d) * push; a.y += (dy / d) * push;
          b.x -= (dx / d) * push; b.y -= (dy / d) * push;
        }
      }
    }

    // springs: every node pulls toward the centre (star-shaped edges)
    for (const p of pts) {
      if (p.isCenter) continue;
      const dx = W / 2 - p.x, dy = H / 2 - p.y;
      const d = Math.sqrt(dx * dx + dy * dy) || 1;
      const spr = (d - K) * 0.06 * t;
      p.vx += (dx / d) * spr;
      p.vy += (dy / d) * spr;
    }

    // gravity keeps the component centred; centre node is pinned
    for (const p of pts) {
      if (p.isCenter) {
        p.vx *= 0.05; p.vy *= 0.05;
      } else {
        p.vx += (W / 2 - p.x) * 0.012;
        p.vy += (H / 2 - p.y) * 0.012;
      }
    }

    // integrate with damping
    for (const p of pts) {
      p.vx *= 0.82;
      p.vy *= 0.82;
      p.x = Math.max(MARGIN, Math.min(W - MARGIN, p.x + p.vx));
      p.y = Math.max(MARGIN, Math.min(H - MARGIN, p.y + p.vy));
    }
  }

  return pts.map(({ vx, vy, ...rest }) => rest);
}

export function GraphPanel({ noteId, onOpenNote, onBuild, notify }: Props) {
  const [center, setCenter] = useState<NoteDTO | null>(null);
  const [neighbors, setNeighbors] = useState<NoteDTO[]>([]);
  const [loading, setLoading] = useState(false);
  const [expanding, setExpanding] = useState(false);
  const [hover, setHover] = useState<number | null>(null);
  const centerRef = useRef<NoteDTO | null>(null);

  const load = useCallback(async (id: number, expand = false) => {
    if (expand && centerRef.current) setExpanding(true);
    else setLoading(true);
    try {
      const c = await api.getNote(id);
      const nb = await api.graphNeighbors(id);
      centerRef.current = c;
      setCenter(c);
      setNeighbors(nb);
    } catch (e) {
      notify(String(e));
      centerRef.current = null;
      setCenter(null);
      setNeighbors([]);
    } finally {
      setLoading(false);
      setExpanding(false);
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
          <span>Este artículo aún no tiene nota en el knowledge graph.</span>
          <button onClick={onBuild}>Generar nota de este artículo</button>
          <span className="muted">El grafo completo se construye con <code>:kb build</code></span>
        </div>
      </div>
    );
  }

  return (
    <div className="panel graph-panel">
      <div className="panel-title">
        <Network size={13} /> Grafo de conocimiento
        <button className="icon-btn" onClick={() => noteId && void load(noteId)} title="Recargar"><RefreshCw size={13} /></button>
        <button className="icon-btn" onClick={() => void api.openVault()} title="Abrir vault en Obsidian"><FolderOpen size={13} /> Vault</button>
      </div>

      <div className="graph-current">
        <span className="note-type">{center.type}</span>
        <h2>{center.title}</h2>
        <span className="muted" style={{ fontSize: 12 }}>{neighbors.length} conexión(es) — clic en un nodo para centrarlo, doble clic para abrir</span>
      </div>

      <svg className="graph-canvas" viewBox={`0 0 ${W} ${H}`} width="100%" height="100%">
        {expanding && (
          <g className="graph-expanding">
            <rect x="0" y="0" width={W} height={H} fill="var(--bg-panel)" opacity="0.5" />
            <text x={W / 2} y={H / 2} textAnchor="middle" className="graph-label">expandiendo…</text>
          </g>
        )}
        <defs>
          <marker id="arrow" markerWidth="8" markerHeight="8" refX="7" refY="3" orient="auto">
            <path d="M0,0 L0,6 L8,3 z" fill="var(--border)" />
          </marker>
        </defs>
        {nodes.filter((n) => !n.isCenter).map((n) => (
          <line
            key={`e-${n.id}`}
            x1={W / 2} y1={H / 2} x2={n.x} y2={n.y}
            stroke="var(--border)" strokeWidth="1.4"
          />
        ))}
        {nodes.map((n) => {
          const color = TYPE_COLOR[n.type] ?? "var(--fg-dim)";
          const r = hover === n.id ? n.r + 2 : n.r;
          return (
            <g
              key={n.id}
              className="graph-node"
              transform={`translate(${n.x},${n.y})`}
              onMouseEnter={() => setHover(n.id)}
              onMouseLeave={() => setHover(null)}
              onClick={() => !n.isCenter && void load(n.id, true)}
              onDoubleClick={() => onOpenNote(n.id)}
            >
              <title>{`[${n.type}] ${n.title}`}</title>
              <circle
                r={r}
                fill={color}
                stroke={n.isCenter ? "var(--fg)" : "var(--bg)"}
                strokeWidth={n.isCenter ? 2.5 : 2}
              />
              {n.isCenter && <circle r={n.r - 7} fill="var(--bg)" opacity={0.35} />}
              <text y={r + 14} textAnchor="middle" className="graph-label" fontWeight={n.isCenter ? 600 : 400}>
                {n.title.length > 26 ? n.title.slice(0, 25) + "…" : n.title}
              </text>
              <text y={r + 26} textAnchor="middle" className="graph-label graph-sublabel">
                {n.type}
              </text>
            </g>
          );
        })}
      </svg>

      <div className="graph-legend">
        {Object.entries(legend).map(([type, count]) => (
          <span key={type} className="lg">
            <span style={{ width: 11, height: 11, borderRadius: "50%", background: TYPE_COLOR[type] ?? "var(--fg-dim)", display: "inline-block" }} />
            {type} ({count})
          </span>
        ))}
      </div>
    </div>
  );
}
