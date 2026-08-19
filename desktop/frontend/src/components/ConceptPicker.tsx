import { useEffect, useMemo, useRef, useState } from "react";
import { Atom, CircleDashed, GitMerge } from "lucide-react";
import { api } from "../api";
import type { ConceptDTO } from "../types";

interface Props {
  onOpenNote: (noteId: number) => void;
  onClose: () => void;
  notify: (msg: string) => void;
}

// ConceptPicker: the knowledge graph seen from its concepts, not its notes.
//   j/k navigate · Enter open the note, or give a pending concept one
//   m mark a concept, m again on another to fold the first into it
//   / filter · Esc close
export function ConceptPicker({ onOpenNote, onClose, notify }: Props) {
  const [concepts, setConcepts] = useState<ConceptDTO[]>([]);
  const [focus, setFocus] = useState(0);
  const [filter, setFilter] = useState("");
  const [mergeFrom, setMergeFrom] = useState<ConceptDTO | null>(null);
  const [busy, setBusy] = useState(false);
  const filterRef = useRef<HTMLInputElement>(null);

  const load = () => api.listConcepts().then(setConcepts).catch((e) => notify(String(e)));
  useEffect(() => { void load(); }, [notify]);

  const shown = useMemo(() => {
    const q = filter.trim().toLowerCase();
    if (!q) return concepts;
    return concepts.filter((c) => c.slug.includes(q) || c.name.toLowerCase().includes(q));
  }, [concepts, filter]);

  useEffect(() => {
    setFocus((f) => Math.min(f, Math.max(0, shown.length - 1)));
  }, [shown.length]);

  const promote = async (c: ConceptDTO) => {
    setBusy(true);
    try {
      const note = await api.promoteConcept(c.slug);
      notify(`${c.name} now has a note`);
      await load();
      onOpenNote(note.id);
    } catch (e) {
      notify(String(e));
    } finally {
      setBusy(false);
    }
  };

  const merge = async (from: ConceptDTO, to: ConceptDTO) => {
    setBusy(true);
    try {
      const res = await api.mergeConcepts(from.slug, to.slug);
      notify(`${from.name} folded into ${to.name} · ${res.notesRelinked} note(s) relinked · ${res.mentions} mentions`);
      setMergeFrom(null);
      await load();
    } catch (e) {
      notify(String(e));
    } finally {
      setBusy(false);
    }
  };

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (filterRef.current && document.activeElement === filterRef.current) {
        if (e.key === "Escape" || e.key === "Enter") {
          e.preventDefault(); e.stopPropagation();
          filterRef.current.blur();
        }
        return; // typing into the filter, keys are text
      }
      const n = shown.length;
      const current = shown[focus];
      if (e.key === "j" || e.key === "ArrowDown") { e.preventDefault(); e.stopPropagation(); setFocus((f) => Math.min(f + 1, n - 1)); }
      else if (e.key === "k" || e.key === "ArrowUp") { e.preventDefault(); e.stopPropagation(); setFocus((f) => Math.max(f - 1, 0)); }
      else if (e.key === "/") { e.preventDefault(); e.stopPropagation(); filterRef.current?.focus(); }
      else if (e.key === "Enter") {
        e.preventDefault(); e.stopPropagation();
        if (!current || busy) return;
        if (current.promoted && current.noteId) onOpenNote(current.noteId);
        else void promote(current);
      } else if (e.key === "m") {
        e.preventDefault(); e.stopPropagation();
        if (!current || busy) return;
        if (!mergeFrom) setMergeFrom(current);
        else if (mergeFrom.slug === current.slug) setMergeFrom(null);
        else void merge(mergeFrom, current);
      } else if (e.key === "Escape") {
        e.preventDefault(); e.stopPropagation();
        if (mergeFrom) setMergeFrom(null);
        else onClose();
      }
    };
    window.addEventListener("keydown", onKey, true);
    return () => window.removeEventListener("keydown", onKey, true);
  }, [shown, focus, mergeFrom, busy, onOpenNote, onClose, notify]);

  const pending = concepts.filter((c) => !c.promoted).length;

  return (
    <div className="overlay" onClick={onClose}>
      <div className="palette source-picker" onClick={(e) => e.stopPropagation()}>
        <div className="palette-head">
          <Atom size={14} /> Concepts ({concepts.length}) · {pending} waiting for a note
        </div>
        <input
          ref={filterRef}
          className="picker-filter"
          placeholder="/ to filter"
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
        />
        {mergeFrom && (
          <div className="palette-empty" data-testid="merge-hint">
            Folding <b>{mergeFrom.name}</b> into… pick one and press <kbd>m</kbd> · <kbd>Esc</kbd> cancels
          </div>
        )}
        <div className="source-picker-list">
          {shown.length === 0 && (
            <div className="palette-empty">No concepts yet — run <kbd>:kb build</kbd>.</div>
          )}
          {shown.map((c, i) => (
            <div
              key={c.slug}
              className={`source-picker-item ${i === focus ? "selected" : ""}`}
              data-testid="concept-row"
              onMouseEnter={() => setFocus(i)}
              onClick={() => { if (c.promoted && c.noteId) onOpenNote(c.noteId); else void promote(c); }}
            >
              <span className="sp-dot" data-on={c.promoted} />
              <span className="sp-type">{c.promoted ? <Atom size={13} /> : <CircleDashed size={13} />}</span>
              <span className="sp-name">{c.name}</span>
              <span className="sp-meta">{c.slug}</span>
              <span className="sp-state">
                {mergeFrom?.slug === c.slug ? <GitMerge size={13} /> : `${c.mentions}×`}
              </span>
            </div>
          ))}
        </div>
        <div className="palette-head" style={{ borderTop: "1px solid var(--border)", borderBottom: "none", flexWrap: "wrap", gap: "4px 14px" }}>
          <span><kbd>j/k</kbd> navigate</span>
          <span><kbd>Enter</kbd> open / promote</span>
          <span><kbd>m</kbd> merge</span>
          <span><kbd>/</kbd> filter</span>
          <span><kbd>Esc</kbd> close</span>
        </div>
      </div>
    </div>
  );
}
