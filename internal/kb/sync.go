package kb

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// The vault is a directory the user also writes in, so a generated note marks
// the part giznews owns. Everything between the markers is rewritten on every
// build; everything outside them belongs to whoever typed it.
const (
	markerBegin = "<!-- giznews:begin -->"
	markerEnd   = "<!-- giznews:end -->"
)

// SyncOutcome says what happened to a note file.
type SyncOutcome string

const (
	// SyncCreated: the file did not exist.
	SyncCreated SyncOutcome = "created"
	// SyncReplaced: the file was exactly as giznews left it, and was replaced.
	SyncReplaced SyncOutcome = "replaced"
	// SyncMerged: the user had edited the file; their text was kept and only
	// the generated region was refreshed.
	SyncMerged SyncOutcome = "merged"
	// SyncKept: the user had edited a file with no generated region to refresh
	// — there is no safe way to update it, so it was left alone.
	SyncKept SyncOutcome = "kept"
)

// SyncInput describes one note write.
type SyncInput struct {
	NoteType string
	Slug     string
	Content  string   // freshly rendered: frontmatter block + body
	LastHash string   // hash of the file giznews last wrote, empty if unknown
	LastTags []string // tags giznews last wrote, to tell the user's apart
}

// SyncResult reports where the note went and how.
type SyncResult struct {
	Path    string
	Hash    string
	Outcome SyncOutcome
}

// Sync writes a generated note without destroying what the user wrote. An
// untouched file is replaced whole; an edited one keeps its own text and gets
// only its generated region and the frontmatter giznews owns refreshed.
func (v *Vault) Sync(in SyncInput) (SyncResult, error) {
	path := v.NotePath(in.NoteType, in.Slug)
	fresh := wrapGenerated(in.Content)

	existing, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return SyncResult{}, fmt.Errorf("kb: read note: %w", err)
		}
		hash, err := v.writeFile(path, fresh)
		return SyncResult{Path: path, Hash: hash, Outcome: SyncCreated}, err
	}

	if in.LastHash != "" && hashOf(existing) == in.LastHash {
		hash, err := v.writeFile(path, fresh)
		return SyncResult{Path: path, Hash: hash, Outcome: SyncReplaced}, err
	}

	// From here the file no longer matches what giznews wrote: someone edited
	// it, or it predates the markers.
	if !strings.Contains(string(existing), markerBegin) {
		if in.LastHash == "" {
			// Nothing was ever recorded for this file, so there is no edit to
			// lose: adopt it with the generated content.
			hash, err := v.writeFile(path, fresh)
			return SyncResult{Path: path, Hash: hash, Outcome: SyncReplaced}, err
		}
		return SyncResult{Path: path, Hash: in.LastHash, Outcome: SyncKept}, nil
	}

	merged, err := mergeNote(string(existing), in.Content, in.LastTags)
	if err != nil {
		return SyncResult{Path: path, Hash: in.LastHash, Outcome: SyncKept}, nil
	}
	hash, err := v.writeFile(path, merged)
	return SyncResult{Path: path, Hash: hash, Outcome: SyncMerged}, err
}

// writeFile writes the note atomically and returns its hash.
func (v *Vault) writeFile(path, content string) (string, error) {
	if err := os.MkdirAll(dirOf(path), 0o755); err != nil {
		return "", fmt.Errorf("kb: create note dir: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("kb: write note: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return "", fmt.Errorf("kb: rename note: %w", err)
	}
	return hashOf([]byte(content)), nil
}

func hashOf(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// wrapGenerated puts the body of a rendered note between the markers, leaving
// the frontmatter block outside them.
func wrapGenerated(content string) string {
	fm, body := splitFrontmatter(content)
	return fm + markerBegin + "\n" + strings.TrimLeft(body, "\n") + markerEnd + "\n"
}

// splitFrontmatter separates the leading YAML block (delimiters included) from
// the rest. A note without one yields an empty block.
func splitFrontmatter(content string) (frontmatter, body string) {
	if !strings.HasPrefix(content, "---\n") {
		return "", content
	}
	rest := content[len("---\n"):]
	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		return "", content
	}
	cut := len("---\n") + end + len("\n---\n")
	return content[:cut], content[cut:]
}

// mergeNote rebuilds an edited file: the user's text outside the generated
// region is kept verbatim, the region is replaced, and the frontmatter keeps
// any property the user added.
func mergeNote(existing, fresh string, lastTags []string) (string, error) {
	begin := strings.Index(existing, markerBegin)
	end := strings.Index(existing, markerEnd)
	if begin < 0 || end < begin {
		return "", fmt.Errorf("kb: no generated region")
	}

	existingFM, _ := splitFrontmatter(existing)
	freshFM, freshBody := splitFrontmatter(fresh)
	mergedFM, err := mergeFrontmatter(existingFM, freshFM, lastTags)
	if err != nil {
		return "", err
	}

	// Text the user put before the region (after the frontmatter) and after it.
	head := existing[len(existingFM):begin]
	tail := existing[end+len(markerEnd):]

	return mergedFM + head + markerBegin + "\n" +
		strings.TrimLeft(freshBody, "\n") + markerEnd + tail, nil
}

// generatedKeys are the frontmatter properties giznews maintains. Anything else
// in the block was added by the user.
var generatedKeys = map[string]bool{
	"type": true, "created": true, "status": true, "source": true,
	"url": true, "rating": true, "category": true, "tags": true,
}

// mergeFrontmatter keeps the freshly generated properties, copies the user's
// own back verbatim — parsing and re-emitting them would quietly rewrite their
// values, turning "reviewed: 2026-08-19" into a timestamp — and unions the tag
// lists so a tag typed in Obsidian is never dropped.
func mergeFrontmatter(existingFM, freshFM string, lastTags []string) (string, error) {
	if existingFM == "" {
		return freshFM, nil
	}
	blocks, order := frontmatterBlocks(existingFM)

	out := strings.TrimSuffix(freshFM, "---\n")
	if extra := userTags(blocks["tags"], lastTags, freshFM); len(extra) > 0 {
		if !strings.Contains(out, "\ntags:\n") && !strings.HasPrefix(out, "tags:\n") {
			out += "tags:\n"
		}
		for _, t := range extra {
			out += "  - " + yamlValue(t) + "\n"
		}
	}
	for _, key := range order {
		if generatedKeys[key] {
			continue
		}
		out += strings.Join(blocks[key], "")
	}
	return out + "---\n", nil
}

var frontmatterKeyRe = regexp.MustCompile(`^([A-Za-z0-9_.-]+):`)

// frontmatterBlocks splits a YAML block into its top-level keys, keeping each
// key's lines exactly as they were written, plus the order they appeared in.
func frontmatterBlocks(block string) (map[string][]string, []string) {
	out := map[string][]string{}
	var order []string
	current := ""
	for _, line := range strings.SplitAfter(strings.Trim(block, "-\n")+"\n", "\n") {
		if line == "" {
			continue
		}
		if m := frontmatterKeyRe.FindStringSubmatch(line); m != nil {
			current = m[1]
			if _, seen := out[current]; !seen {
				order = append(order, current)
			}
		}
		if current == "" {
			continue
		}
		out[current] = append(out[current], line)
	}
	return out, order
}

// userTags returns the tags in the file that giznews did not put there and that
// the fresh block does not already carry.
func userTags(tagLines []string, lastTags []string, freshFM string) []string {
	generated := map[string]bool{}
	for _, t := range lastTags {
		generated[t] = true
	}
	var out []string
	for _, line := range tagLines {
		item, ok := strings.CutPrefix(strings.TrimRight(line, "\n"), "  - ")
		if !ok {
			continue
		}
		tag := unquoteYAML(strings.TrimSpace(item))
		if tag == "" || generated[tag] {
			continue
		}
		if strings.Contains(freshFM, "  - "+yamlValue(tag)+"\n") {
			continue
		}
		out = append(out, tag)
	}
	return out
}

// unquoteYAML undoes the quoting yamlValue applies.
func unquoteYAML(s string) string {
	if len(s) < 2 || !strings.HasPrefix(s, `"`) || !strings.HasSuffix(s, `"`) {
		return s
	}
	s = s[1 : len(s)-1]
	s = strings.ReplaceAll(s, `\"`, `"`)
	return strings.ReplaceAll(s, `\\`, `\`)
}
