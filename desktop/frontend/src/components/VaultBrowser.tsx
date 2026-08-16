import { useEffect, useMemo, useRef, useState } from "react";
import { GitBranch, FileText, FlaskConical, Loader2, FolderOpen, type LucideIcon } from "lucide-react";
import { api } from "../api";
import type { NoteDTO } from "../types";
import { LinksPicker, type LinkItem } from "./LinksPicker";

export type StageKey = "electron" | "atom" | "molecule";

const STAGES: { key: StageKey; label: string; icon: LucideIcon }[] = [
  { key: "electron", label: "Electrons", icon: GitBranch },
  { key: "atom", label: "Atoms", icon: FileText },
  { key: "molecule", label: "Molecules", icon: FlaskConical },
];

interface Props {
  stage: StageKey;
  onStage: (s: StageKey) => void;
  onOpenNote: (id: number) => void;
  onFocus: () => void;
  onClose: () => void;
  active: boolean;
  notify: (msg: string) => void;
}

// VaultBrowser is the knowledge-world master list (left column): the vault
// stages (electrons → atoms → molecules). The news list is the inbox, so the
// vault only holds the knowledge notes. Enter opens a note in the detail
// (right) column; the browser stays so you keep browsing the flow.
export function VaultBrowser({ stage, onStage, onOpenNote, onFocus, onClose, active, notify }: Props) {
  const [notes, setNotes] = useState<NoteDTO[] | null>(null);
  const [sel, setSel] = useState(0);
  const [linksOpen, setLinksOpen] = useState(false);
  const [tagFilter, setTagFilter] = useState<string | null>(null);
  const autoLoaded = useRef<string | null>(null);

  useEffect(() => {
    api.listNotes("").then(setNotes).catch((e) => { notify(String(e)); setNotes([]); });
  }, [notify]);

  const bySlug = useMemo(() => new Map((notes ?? []).map((n) => [n.slug, n])), [notes]);
  const byId = useMemo(() => new Map((notes ?? []).map((n) => [n.id, n])), [notes]);

  // the transversal taxonomy: distinct tags across the whole vault, with counts.
  const allTags = useMemo(() => {
    const m = new Map<string, number>();
    for (const n of notes ?? []) {
      for (const t of n.tags ?? []) m.set(t, (m.get(t) ?? 0) + 1);
    }
    return [...m.entries()].sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0]));
  }, [notes]);

  const items: NoteDTO[] = useMemo(() => {
    let list = (notes ?? []).filter((n) => n.type === stage);
    if (tagFilter) list = list.filter((n) => (n.tags ?? []).includes(tagFilter));
    return list;
  }, [notes, stage, tagFilter]);
  const selected: NoteDTO | undefined = items[sel];

  const links: LinkItem[] = useMemo(() => {
    if (!selected) return [];
    const out: LinkItem[] = (selected.wikilinks ?? [])
      .map((slug) => bySlug.get(slug))
      .filter((n): n is NoteDTO => !!n)
      .map((n) => ({ id: n.id, title: n.title, type: n.type, dir: "out" }));
    const incoming: LinkItem[] = (notes ?? [])
      .filter((n) => (n.wikilinks ?? []).includes(selected.slug))
      .map((n) => ({ id: n.id, title: n.title, type: n.type, dir: "in" }));
    return [...out, ...incoming];
  }, [selected, notes, bySlug]);

  // follow a link → jump the vault to that note's stage.
  const goToNote = (id: number) => {
    const n = byId.get(id);
    if (!n) return;
    onStage(n.type as StageKey);
    const idx = (notes ?? []).filter((x) => x.type === n.type).findIndex((x) => x.id === id);
    setSel(Math.max(0, idx));
  };

  const counts = useMemo(() => {
    const c: Record<StageKey, number> = { electron: 0, atom: 0, molecule: 0 };
    for (const n of notes ?? []) {
      if (n.type === "electron" || n.type === "atom" || n.type === "molecule") c[n.type]++;
    }
    return c;
  }, [notes]);

  useEffect(() => { setSel(0); }, [stage, tagFilter]);

  // auto-load the first note of the stage so the reader never lands empty.
  useEffect(() => {
    if (!notes || items.length === 0) return;
    if (autoLoaded.current === stage) return;
    autoLoaded.current = stage;
    onOpenNote(items[0].id);
  }, [stage, notes, items, onOpenNote]);

  // keyboard (capture phase): h/l stage · j/k items · g/G extremes · Enter open
  // · L links · f back to news. Esc closes the links overlay only. Inactive
  // when the reader has focus (keys scroll the note instead).
  useEffect(() => {
    if (!active || linksOpen) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "f") { e.preventDefault(); e.stopPropagation(); onClose(); return; }
      if (e.key === "h" || e.key === "ArrowLeft") {
        e.preventDefault(); e.stopPropagation();
        onStage(STAGES[Math.max(0, STAGES.findIndex((x) => x.key === stage) - 1)].key);
        return;
      }
      if (e.key === "l" || e.key === "ArrowRight") {
        e.preventDefault(); e.stopPropagation();
        onStage(STAGES[Math.min(STAGES.length - 1, STAGES.findIndex((x) => x.key === stage) + 1)].key);
        return;
      }
      if (e.key === "j" || e.key === "ArrowDown") { e.preventDefault(); e.stopPropagation(); setSel((s) => Math.min(s + 1, Math.max(0, items.length - 1))); return; }
      if (e.key === "k" || e.key === "ArrowUp") { e.preventDefault(); e.stopPropagation(); setSel((s) => Math.max(s - 1, 0)); return; }
      if (e.key === "g") { e.preventDefault(); e.stopPropagation(); setSel(0); return; }
      if (e.key === "G") { e.preventDefault(); e.stopPropagation(); setSel(Math.max(0, items.length - 1)); return; }
      if (e.key === "Enter") {
        e.preventDefault(); e.stopPropagation();
        if (selected) { onOpenNote(selected.id); onFocus(); }
        return;
      }
      if (e.key === "L") {
        e.preventDefault(); e.stopPropagation();
        if (selected) setLinksOpen(true);
        return;
      }
    };
    window.addEventListener("keydown", onKey, true);
    return () => window.removeEventListener("keydown", onKey, true);
  }, [active, stage, sel, items, selected, linksOpen, onClose, onOpenNote, onFocus, onStage]);

  if (notes === null) {
    return (
      <div className="vault-browser">
        <div className="empty with-icon"><Loader2 size={22} className="spin" /> Loading vault…</div>
      </div>
    );
  }

  return (
    <div className="vault-browser">
      <div className="vb-head"><FolderOpen size={14} /> Vault</div>

      <div className="vault-tabs">
        {STAGES.map((s) => {
          const Icon = s.icon;
          return (
            <button key={s.key} className={`vault-tab ${stage === s.key ? "active" : ""}`} onClick={() => onStage(s.key)}>
              <Icon size={13} /> {s.label} <span className="vault-count">{counts[s.key]}</span>
            </button>
          );
        })}
      </div>

      {allTags.length > 0 && (
        <div className="vb-tags">
          <button className={`tag-chip ${tagFilter === null ? "active" : ""}`} onClick={() => setTagFilter(null)}>All</button>
          {allTags.map(([tag, count]) => (
            <button key={tag} className={`tag-chip ${tagFilter === tag ? "active" : ""}`} onClick={() => setTagFilter(tagFilter === tag ? null : tag)}>
              #{tag} <span className="tag-count">{count}</span>
            </button>
          ))}
        </div>
      )}

      <div className="vault-list">
        {items.length === 0 && <div className="palette-empty">{tagFilter ? `No notes tagged #${tagFilter} in this stage.` : "No notes in this stage."}</div>}
        {items.map((it, i) => {
          const Icon = STAGES.find((s) => s.key === it.type)?.icon ?? FileText;
          return (
            <div
              key={it.id}
              className={`palette-item ${i === sel ? "selected" : ""}`}
              onMouseEnter={() => setSel(i)}
              onClick={() => { onOpenNote(it.id); onFocus(); }}
            >
              <span className="cmd-left">
                <span className="sp-type"><Icon size={13} /></span>
                <span className="cmd-name">{it.title}</span>
              </span>
            </div>
          );
        })}
      </div>

      {linksOpen && selected && (
        <LinksPicker
          links={links}
          onPick={(item) => { setLinksOpen(false); goToNote(item.id); }}
          onClose={() => setLinksOpen(false)}
        />
      )}
    </div>
  );
}
