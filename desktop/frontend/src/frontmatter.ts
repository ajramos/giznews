// frontmatter.ts — separate helpers for the YAML frontmatter that the kb
// builders prepend to note content (for Obsidian). The reader strips it and
// renders the metadata separately instead of showing the raw YAML block.

const FRONTMATTER_RE = /^\s*---\r?\n[\s\S]*?\r?\n---\r?\n?/;

// stripFrontmatter removes a leading `---\n…\n---` block from note content.
// Returns the body untouched when there is no frontmatter.
export function stripFrontmatter(content: string): string {
  return content.replace(FRONTMATTER_RE, "").replace(/^\r?\n/, "");
}

const LEADING_H1_RE = /^#\s+[^\n]*\r?\n?/;

// noteBody returns the content ready to render in the reader: the frontmatter
// block and the leading H1 (the note title, already shown in the header) are
// both stripped, leaving only the sections.
export function noteBody(content: string): string {
  return stripFrontmatter(content).replace(LEADING_H1_RE, "").replace(/^\r?\n/, "");
}
