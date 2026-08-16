// frontmatter.ts — separate helpers for the YAML frontmatter that the kb
// builders prepend to note content (for Obsidian). The reader strips it and
// renders the metadata separately instead of showing the raw YAML block.

const FRONTMATTER_RE = /^\s*---\r?\n[\s\S]*?\r?\n---\r?\n?/;

// stripFrontmatter removes a leading `---\n…\n---` block from note content.
// Returns the body untouched when there is no frontmatter.
export function stripFrontmatter(content: string): string {
  return content.replace(FRONTMATTER_RE, "").replace(/^\r?\n/, "");
}
