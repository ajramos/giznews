import type { NoteDTO } from "./types";
import type { LinkItem } from "./components/LinksPicker";

// buildNoteLinks computes a note's connections: outgoing [[wikilinks]] and
// incoming notes that link back to it. Shared by the vault flow and the note
// reader's links picker.
export function buildNoteLinks(note: NoteDTO, notes: NoteDTO[]): LinkItem[] {
  const bySlug = new Map(notes.map((n) => [n.slug, n]));
  const out: LinkItem[] = (note.wikilinks ?? [])
    .map((slug) => bySlug.get(slug))
    .filter((n): n is NoteDTO => !!n)
    .map((n) => ({ id: n.id, title: n.title, type: n.type, dir: "out" }));
  const incoming: LinkItem[] = notes
    .filter((n) => (n.wikilinks ?? []).includes(note.slug))
    .map((n) => ({ id: n.id, title: n.title, type: n.type, dir: "in" }));
  return [...out, ...incoming];
}
