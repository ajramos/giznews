import type { NoteDTO } from "./types";
import type { LinkItem } from "./components/LinksPicker";

// extractMarkdownLinks pulls http(s) URLs out of a markdown body: `[text](url)`
// links and bare URLs. Image URLs and duplicates are dropped.
export function extractMarkdownLinks(md: string): { url: string; text: string }[] {
  if (!md) return [];
  const seen = new Set<string>();
  const out: { url: string; text: string }[] = [];
  const push = (url: string, text: string) => {
    url = url.trim();
    if (!url || seen.has(url)) return;
    if (!/^https?:\/\//i.test(url)) return;
    if (/\.(png|jpe?g|gif|webp|svg|avif)(\?.*)?$/i.test(url)) return;
    seen.add(url);
    out.push({ url, text: text.trim() || url });
  };

  let m: RegExpExecArray | null;
  const mdRe = /\[([^\]]*)\]\((https?:\/\/[^)\s]+)\)/g;
  while ((m = mdRe.exec(md)) !== null) push(m[2], m[1]);

  const bareRe = /https?:\/\/[^\s"'<>)\]]+/g;
  while ((m = bareRe.exec(md)) !== null) push(m[0], m[0]);

  return out;
}

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

// buildArticleLinks builds the picker for an article being read: the links
// embedded in its body, its external URL, plus its Atom note's connections.
export function buildArticleLinks(url: string, contentMD: string, note: NoteDTO | null, notes: NoteDTO[]): LinkItem[] {
  const links: LinkItem[] = [];
  const own = new Set<string>();
  if (url) {
    links.push({ id: 0, title: url, type: "external", dir: "out", url });
    own.add(url);
  }
  for (const l of extractMarkdownLinks(contentMD)) {
    if (own.has(l.url)) continue;
    links.push({ id: 0, title: l.text, type: "external", dir: "out", url: l.url });
  }
  if (note) links.push(...buildNoteLinks(note, notes));
  return links;
}

