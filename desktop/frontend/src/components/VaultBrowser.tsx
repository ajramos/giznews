import { useEffect, useMemo, useState } from "react";
import { Inbox, GitBranch, FileText, FlaskConical, Loader2, type LucideIcon } from "lucide-react";
import { api } from "../api";
import type { ArticleDTO, NoteDTO } from "../types";
import { LinksPicker, type LinkItem } from "./LinksPicker";

export type StageKey = "inbox" | "electron" | "atom" | "molecule";

const STAGES: { key: StageKey; label: string; icon: LucideIcon }[] = [
  { key: "inbox", label: "00 Inbox", icon: Inbox },
  { key: "electron", label: "01 Electrons", icon: GitBranch },
  { key: "atom", label: "02 Atoms", icon: FileText },
  { key: "molecule", label: "03 Molecules", icon: FlaskConical },
];

const TYPE_ICON: Record<string, LucideIcon> = {
  atom: FileText,
  electron: GitBranch,
  molecule: FlaskConical,
  inbox: Inbox,
};

interface Item {
  kind: "article" | "note";
  id: number;
  title: string;
  type: string;
  summary?: string;
  importance?: number;
}

interface Props {
  stage: StageKey;
  onStage: (s: StageKey) => void;
  onOpenNote: (id: number) => void;
  onOpenArticle: (id: number) => void;
  onClose: () => void;
  notify: (msg: string) => void;
}

// VaultBrowser is the knowledge-world master list (left column): the vault
// stages (inbox → electrons → atoms → molecules). Enter opens an item in the
// detail (right) column; the browser stays so you keep browsing the flow.
export function VaultBrowser({ stage, onStage, onOpenNote, onOpenArticle, onClose, notify }: Props) {
  const [notes, setNotes] = useState<NoteDTO[] | null>(null);
  const [inbox, setInbox] = useState<ArticleDTO[]>([]);
  const [sel, setSel] = useState(0);
  const [linksOpen, setLinksOpen] = useState(false);

  useEffect(() => {
    api.listNotes("").then(setNotes).catch((e) => { notify(String(e)); setNotes([]); });
    api.listInbox(200).then(setInbox).catch(() => {});
  }, [notify]);

  const bySlug = useMemo(() => new Map((notes ?? []).map((n) => [n.slug, n])), [notes]);
  const byId = useMemo(() => new Map((notes ?? []).map((n) => [n.id, n])), [notes]);

  const stageNotes = useMemo(() => (notes ?? []).filter((n) => n.type === stage), [notes, stage]);

  const items: Item[] = useMemo(() => {
    if (stage === "inbox") {
      return inbox.map((a) => ({ kind: "article", id: a.id, title: a.title, type: "inbox", summary: a.summary, importance: a.importance }));
    }
    return stageNotes.map((n) => ({ kind: "note", id: n.id, title: n.title, type: n.type }));
  }, [stage, inbox, stageNotes]);

  const selected: Item | undefined = items[sel];

  const links: LinkItem[] = useMemo(() => {
    if (!selected || selected.kind !== "note") return [];
    const note = byId.get(selected.id);
    if (!note) return [];
    const out: LinkItem[] = (note.wikilinks ?? [])
      .map((slug) => bySlug.get(slug))
      .filter((n): n is NoteDTO => !!n)
      .map((n) => ({ id: n.id, title: n.title, type: n.type, dir: "out" }));
    const incoming: LinkItem[] = (notes ?? [])
      .filter((n) => (n.wikilinks ?? []).includes(note.slug))
      .map((n) => ({ id: n.id, title: n.title, type: n.type, dir: "in" }));
    return [...out, ...incoming];
  }, [selected, notes, bySlug, byId]);

  // follow a link → jump the vault to that note's stage.
  const goToNote = (id: number) => {
    const n = byId.get(id);
    if (!n) return;
    onStage(n.type as StageKey);
    const idx = (notes ?? []).filter((x) => x.type === n.type).findIndex((x) => x.id === id);
    setSel(Math.max(0, idx));
  };

  const counts = useMemo(() => {
    const c: Record<StageKey, number> = { inbox: inbox.length, electron: 0, atom: 0, molecule: 0 };
    for (const n of notes ?? []) {
      if (n.type === "electron" || n.type === "atom" || n.type === "molecule") c[n.type]++;
    }
    return c;
  }, [notes, inbox]);

  useEffect(() => { setSel(0); }, [stage]);

  // keyboard (capture phase): h/l stage · j/k items · g/G extremes · Enter open
  // · L links (note) · f back to news. Esc closes the links overlay only.
  useEffect(() => {
    if (linksOpen) return;
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
        if (selected) {
          if (selected.kind === "note") onOpenNote(selected.id);
          else onOpenArticle(selected.id);
        }
        return;
      }
      if (e.key === "L") {
        e.preventDefault(); e.stopPropagation();
        if (selected && selected.kind === "note") setLinksOpen(true);
        return;
      }
    };
    window.addEventListener("keydown", onKey, true);
    return () => window.removeEventListener("keydown", onKey, true);
  }, [stage, sel, items, selected, linksOpen, onClose, onOpenNote, onOpenArticle, onStage]);

  if (notes === null) {
    return (
      <div className="vault-browser">
        <div className="empty with-icon"><Loader2 size={22} className="spin" /> Loading vault…</div>
      </div>
    );
  }

  return (
    <div className="vault-browser">
      <div className="vb-head">🔀 Vault</div>

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

      <div className="vault-list">
        {items.length === 0 && <div className="palette-empty">No items in this stage.</div>}
        {items.map((it, i) => {
          const Icon = TYPE_ICON[it.type] ?? FileText;
          return (
            <div
              key={it.kind + it.id}
              className={`palette-item ${i === sel ? "selected" : ""}`}
              onMouseEnter={() => setSel(i)}
              onClick={() => { if (it.kind === "note") onOpenNote(it.id); else onOpenArticle(it.id); }}
            >
              <span className="cmd-left">
                <span className="sp-type"><Icon size={13} /></span>
                <span className="cmd-name">{it.title}</span>
              </span>
              {it.importance !== undefined && <span className="cmd-hint">{"★".repeat(it.importance)}</span>}
            </div>
          );
        })}
      </div>

      <div className="vb-foot">
        <span><kbd>h/l</kbd> stage</span>
        <span><kbd>j/k</kbd> items</span>
        <span><kbd>Enter</kbd> open</span>
        <span><kbd>L</kbd> links</span>
        <span><kbd>f</kbd> news</span>
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
