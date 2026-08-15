import { useEffect, useMemo, useRef, useState } from "react";
import { Network, Loader2, RefreshCw, FolderOpen } from "lucide-react";
import * as d3force from "d3-force";
import * as d3zoom from "d3-zoom";
import * as d3drag from "d3-drag";
import * as d3select from "d3-selection";
import "d3-transition"; // adds .transition() to selections
import { api } from "../api";
import type { NoteDTO } from "../types";

interface Props {
  focusNoteId: number | null;
  refresh: number;
  onOpenNote: (id: number) => void;
  onBuild: () => void;
  notify: (msg: string) => void;
}

interface GNode {
  id: number;
  title: string;
  type: string;
}
interface GLink {
  source: number | GNode;
  target: number | GNode;
}

const TYPE_COLOR: Record<string, string> = {
  atom: "var(--accent)",
  electron: "var(--cat-research)",
  molecule: "var(--cat-funding)",
  inbox: "var(--fg-dim)",
};

// Build the full knowledge-graph: every note is a node; every [[wikilink]]
// between two notes is an edge.
function buildGraph(notes: NoteDTO[]): { nodes: GNode[]; links: GLink[] } {
  const nodes: GNode[] = notes.map((n) => ({ id: n.id, title: n.title, type: n.type }));
  const bySlug = new Map(notes.map((n) => [n.slug, n]));
  const seen = new Set<string>();
  const links: GLink[] = [];
  for (const n of notes) {
    for (const slug of n.wikilinks ?? []) {
      const t = bySlug.get(slug);
      if (!t || t.id === n.id) continue;
      const key = n.id < t.id ? `${n.id}-${t.id}` : `${t.id}-${n.id}`;
      if (seen.has(key)) continue;
      seen.add(key);
      links.push({ source: n.id, target: t.id });
    }
  }
  return { nodes, links };
}

// render the graph imperatively with d3; returns a cleanup function.
function renderGraph(
  svg: SVGSVGElement,
  nodes: GNode[],
  links: GLink[],
  focusId: number | null,
  onOpenNote: (id: number) => void
): () => void {
  const W = 800;
  const H = 560;

  d3select.select(svg).selectAll("*").remove();
  const root = d3select.select(svg).append("g");

  // defs: arrow marker + halo filter
  const defs = d3select.select(svg).append("defs");
  defs
    .append("marker")
    .attr("id", "graph-arrow")
    .attr("viewBox", "0 -5 10 10")
    .attr("refX", 16)
    .attr("refY", 0)
    .attr("markerWidth", 5)
    .attr("markerHeight", 5)
    .attr("orient", "auto")
    .append("path")
    .attr("d", "M0,-5L10,0L0,5")
    .attr("fill", "var(--border)");

  const simulation = d3force
    .forceSimulation(nodes as d3force.SimulationNodeDatum[])
    .force("link", d3force.forceLink(links as any).id((d: any) => d.id).distance(130))
    .force("charge", d3force.forceManyBody().strength(-420))
    .force("center", d3force.forceCenter(W / 2, H / 2))
    .force("collide", d3force.forceCollide(34));

  // edges: curved arcs with arrow
  const link = root
    .append("g")
    .attr("class", "links")
    .selectAll("path")
    .data(links)
    .join("path")
    .attr("fill", "none")
    .attr("stroke", "var(--border)")
    .attr("stroke-width", 1.2)
    .attr("marker-end", "url(#graph-arrow)");

  // nodes
  const node = root
    .append("g")
    .attr("class", "nodes")
    .selectAll("g")
    .data(nodes)
    .join("g")
    .attr("class", "graph-node")
    .on("click", (_event: MouseEvent, d: any) => onOpenNote(d.id))
    .on("mouseenter", (event: MouseEvent, d: any) => {
      d3select.select(event.currentTarget as Element).attr("data-hover", "1");
      link.attr("opacity", (l: any) =>
        l.source.id === d.id || l.target.id === d.id ? 1 : 0.12
      );
    })
    .on("mouseleave", () => {
      node.attr("data-hover", null);
      link.attr("opacity", 1);
    });

  // transparent hit-area so clicks register anywhere near the node
  node
    .append("circle")
    .attr("r", 26)
    .attr("fill", "transparent");

  node
    .append("circle")
    .attr("r", (d: any) => (d.type === "molecule" ? 13 : d.type === "electron" ? 12 : 11))
    .attr("fill", (d: any) => TYPE_COLOR[d.type] ?? "var(--fg-dim)")
    .attr("stroke", "var(--bg)")
    .attr("stroke-width", 2);

  node
    .append("text")
    .attr("dx", 15)
    .attr("dy", 3)
    .attr("class", "graph-label")
    .text((d: any) => (d.title.length > 24 ? d.title.slice(0, 23) + "…" : d.title));

  // drag
  node.call(
    (d3drag.drag() as any)
      .on("start", (event: any, d: any) => {
        if (!event.active) simulation.alphaTarget(0.25).restart();
        d.fx = d.x;
        d.fy = d.y;
      })
      .on("drag", (event: any, d: any) => {
        d.fx = event.x;
        d.fy = event.y;
      })
      .on("end", (event: any, d: any) => {
        if (!event.active) simulation.alphaTarget(0);
        d.fx = null;
        d.fy = null;
      })
  );

  // zoom + pan
  const zoom = (d3zoom.zoom() as any)
    .scaleExtent([0.4, 4])
    .on("zoom", (event: any) => root.attr("transform", event.transform));
  d3select.select(svg).call(zoom);

  simulation.on("tick", () => {
    link.attr("d", (d: any) => {
      const dx = d.target.x - d.source.x;
      const dy = d.target.y - d.source.y;
      const r = Math.hypot(dx, dy) / 1.6;
      return `M${d.source.x},${d.source.y} A${r} ${r} 0 0 1 ${d.target.x},${d.target.y}`;
    });
    node.attr("transform", (d: any) => `translate(${d.x},${d.y})`);
  });

  // highlight the focused note and zoom to it once the layout settles
  if (focusId != null) {
    node.filter((d: any) => d.id === focusId).attr("data-focus", "1");
  }

  // fit-to-content zoom (applied directly so nodes are stable for clicks)
  const fitTimer = window.setTimeout(() => {
    const b = (root.node() as SVGGElement).getBBox();
    if (b.width === 0 || b.height === 0) return;
    const scale = Math.min(1.4, Math.max(0.35, Math.min(W / b.width, H / b.height) * 0.9));
    const tx = (W - b.width * scale) / 2 - b.x * scale;
    const ty = (H - b.height * scale) / 2 - b.y * scale;
    d3select.select(svg).call(zoom.transform, (d3zoom as any).zoomIdentity.translate(tx, ty).scale(scale));
  }, 400);

  return () => {
    window.clearTimeout(fitTimer);
    simulation.stop();
  };
}

export function GraphPanel({ focusNoteId, refresh, onOpenNote, onBuild, notify }: Props) {
  const svgRef = useRef<SVGSVGElement>(null);
  const [notes, setNotes] = useState<NoteDTO[] | null>(null);
  const focusRef = useRef<number | null>(null);
  focusRef.current = focusNoteId;

  const load = () => {
    api.listNotes("")
      .then((ns) => setNotes(ns))
      .catch((e) => { notify(String(e)); setNotes([]); });
  };

  useEffect(load, [refresh]); // eslint-disable-line react-hooks/exhaustive-deps

  const { nodes, links } = useMemo(() => (notes ? buildGraph(notes) : { nodes: [], links: [] }), [notes]);

  useEffect(() => {
    if (!notes || notes.length === 0 || !svgRef.current) return;
    const cleanup = renderGraph(svgRef.current, nodes, links, focusNoteId, onOpenNote);
    return cleanup;
  }, [notes, nodes, links, focusNoteId, onOpenNote]);

  const legend = useMemo(() => {
    const c: Record<string, number> = {};
    for (const n of nodes) c[n.type] = (c[n.type] ?? 0) + 1;
    return c;
  }, [nodes]);

  if (notes === null) {
    return <div className="panel"><div className="empty with-icon"><Loader2 size={24} className="spin" /> Loading graph…</div></div>;
  }

  if (notes.length === 0) {
    return (
      <div className="panel graph-panel">
        <div className="panel-title"><Network size={13} /> Knowledge graph</div>
        <div className="empty with-icon">
          <Network size={32} />
          <span>No notes in the knowledge graph yet.</span>
          <button onClick={onBuild}>Generate note for this article</button>
          <span className="muted">For the full graph run <code>:process</code> (or <code>:kb build</code>)</span>
        </div>
      </div>
    );
  }

  return (
    <div className="panel graph-panel">
      <div className="panel-title">
        <Network size={13} /> Knowledge graph
        <span className="muted" style={{ fontSize: 12 }}>{nodes.length} notes · {links.length} connections</span>
        <span style={{ flex: 1 }} />
        <button className="icon-btn" onClick={load} title="Reload"><RefreshCw size={13} /></button>
        <button className="icon-btn" onClick={() => void api.openVault()} title="Open vault in Obsidian"><FolderOpen size={13} /> Vault</button>
      </div>

      <svg ref={svgRef} className="graph-canvas" viewBox="0 0 800 560" width="100%" height="100%" />

      <div className="graph-legend">
        {Object.entries(legend).map(([type, count]) => (
          <span key={type} className="lg">
            <span style={{ width: 11, height: 11, borderRadius: "50%", background: TYPE_COLOR[type] ?? "var(--fg-dim)", display: "inline-block" }} />
            {type} ({count})
          </span>
        ))}
        <span className="muted">· click = open note · drag = move · scroll = zoom</span>
      </div>
    </div>
  );
}
