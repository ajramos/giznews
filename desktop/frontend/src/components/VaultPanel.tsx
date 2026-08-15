import { useEffect, useMemo, useState } from "react";
import { Inbox, GitBranch, FileText, FlaskConical, Loader2, Link2, type LucideIcon } from "lucide-react";
import { api } from "../api";
import type { ArticleDTO, NoteDTO } from "../types";
import { LinksPicker, type LinkItem } from "./LinksPicker";

type StageKey = "inbox" | "electron" | "atom" | "molecule";

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

const FLOW_HINT: Record<string, string> = {
  inbox: "Inbox → process → Atom",
  atom: "Atom → concepts → Electrons",
  electron: "Electron ← defined by Atoms",
  molecule: "Molecule ← synthesizes Atoms",
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
  onOpenNote: (id: number) => void;
  onOpenArticle: (id: number) => void;
  onClose: () => void;
  notify: (msg: string) => void;
}

export function VaultPanel({ onOpenNote, onOpenArticle, onClose, notify }: Props) {
  const [notes, setNotes] = useState<NoteDTO[] | null>(null);
  const [inbox, setInbox] = useState<ArticleDTO[]>([]);
  const [stage, setStage] = useState<StageKey>("inbox");
  const [sel, setSel] = useState(0);
  const [linksOpen, setLinksOpen] = useState(false);

  useEffect(() => {
    api.listNotes("").then(setNotes).catch((e) => { notify(String(e)); setNotes([]); });
    api.listInbox(200).then(setInbox).catch(() => {});
  }, [notify]);

  const bySlug = useMemo(() => new Map((notes ?? []).map((n) => [n.slug, n])), [notes]);
  const byId = useMemo(() => new Map((notes ?? []).map((n) => [n.id, n])), [notes]);

  const stageNotes = useMemo(
    () => (notes ?? []).filter((n) => n.type === stage),
    [notes, stage]
  );

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

  // navigate the vault to a given note (follow a link)
  const goToNote = (id: number) => {
    const n = byId.get(id);
    if (!n) return;
    setStage(n.type as StageKey);
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

  // keyboard (capture phase; the app's vim handler is suppressed while open)
  useEffect(() => {
    if (linksOpen) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape" || e.key === "f") { e.stopPropagation(); onClose(); return; }
      if (e.key === "h" || e.key === "ArrowLeft") {
        e.preventDefault(); e.stopPropagation();
        setStage((s) => { const i = STAGES.findIndex((x) => x.key === s); return STAGES[Math.max(0, i - 1)].key; });
        setSel(0);
        return;
      }
      if (e.key === "l" || e.key === "ArrowRight") {
        e.preventDefault(); e.stopPropagation();
        setStage((s) => { const i = STAGES.findIndex((x) => x.key === s); return STAGES[Math.min(STAGES.length - 1, i + 1)].key; });
        setSel(0);
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
          onClose();
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
  }, [stage, sel, items, selected, linksOpen, onClose, onOpenNote, onOpenArticle]);

  if (notes === null) {
    return (
      <div className="overlay" onClick={onClose}>
        <div className="palette vault-panel"><div className="empty with-icon"><Loader2 size={22} className="spin" /> Loading vault…</div></div>
      </div>
    );
  }

  return (
    <div className="overlay" onClick={onClose}>
      <div className="palette vault-panel" onClick={(e) => e.stopPropagation()}>
        <div className="palette-head">🔀 Vault flow</div>

        <div className="vault-tabs">
          {STAGES.map((s) => {
            const Icon = s.icon;
            return (
              <button
                key={s.key}
                className={`vault-tab ${stage === s.key ? "active" : ""}`}
                onClick={() => { setStage(s.key); setSel(0); }}
              >
                <Icon size={13} /> {s.label} <span className="vault-count">{counts[s.key]}</span>
              </button>
            );
          })}
        </div>

        <div className="vault-body">
          <div className="vault-list">
            {items.length === 0 && <div className="palette-empty">No items in this stage.</div>}
            {items.map((it, i) => {
              const Icon = TYPE_ICON[it.type] ?? FileText;
              return (
                <div
                  key={it.kind + it.id}
                  className={`palette-item ${i === sel ? "selected" : ""}`}
                  onMouseEnter={() => setSel(i)}
                  onClick={() => {
                    if (it.kind === "note") onOpenNote(it.id);
                    else onOpenArticle(it.id);
                    onClose();
                  }}
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

          <div className="vault-detail">
            {selected ? (
              <>
                <div className="vd-head">
                  <span className="note-type">{selected.type}</span>
                  <h2>{selected.title}</h2>
                </div>
                {selected.summary && <p className="vd-summary">{selected.summary}</p>}
                <div className="vd-flow">{FLOW_HINT[selected.type] ?? ""}</div>

                {selected.kind === "note" && (
                  <>
                    <div className="vd-section">
                      <button className="icon-btn" onClick={() => setLinksOpen(true)} title="Open links picker (L)">
                        <Link2 size={13} /> Links ({links.length})
                      </button>
                    </div>
                    {links.length === 0 && <div className="muted" style={{ fontSize: 12 }}>No connections yet.</div>}
                    {links.map((l) => {
                      const Icon = TYPE_ICON[l.type] ?? FileText;
                      return (
                        <div key={l.id + l.dir} className="vd-link" onClick={() => goToNote(l.id)}>
                          <span className="sp-type"><Icon size={12} /></span>
                          <span className="vd-dir">{l.dir === "out" ? "→" : "←"}</span>
                          <span className="vd-link-title">{l.title}</span>
                        </div>
                      );
                    })}
                  </>
                )}
                {selected.kind === "article" && (
                  <div className="muted" style={{ fontSize: 12 }}>
                    Process this article (<code>:process</code>) to turn it into an Atom note.
                  </div>
                )}
              </>
            ) : (
              <div className="palette-empty">Select an item.</div>
            )}
          </div>
        </div>

        <div className="palette-head" style={{ borderTop: "1px solid var(--border)", borderBottom: "none", flexWrap: "wrap", gap: "4px 12px" }}>
          <span><kbd>h/l</kbd> stage</span>
          <span><kbd>j/k</kbd> items</span>
          <span><kbd>Enter</kbd> open</span>
          <span><kbd>L</kbd> links</span>
          <span><kbd>f/Esc</kbd> close</span>
        </div>
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
