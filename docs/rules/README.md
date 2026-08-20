# Prefilter rulesets

A rule is a regex plus what to do with what it catches. It runs **before** the
LLM classifier, so every article a rule resolves is an article the model never
sees — and never writes a summary or extracts entities for.

That asymmetry is the whole design of `noise.json`: it almost only archives.
Killing junk is free; pre-classifying a good article saves one call and costs
you its summary and its concept links in the vault.

## The order is the logic

Rules are matched **first-match-wins**, in creation order — which, after an
import, is the order they appear in the file. `noise.json` is laid out as:

1. **slop** — listicles, prompt packs, sponsored posts. Above the shield on
   purpose: a listicle about Claude is still a listicle.
2. **keep: labs, models and papers** — the shield. Everything below is
   forbidden from touching an article that names a lab, a model or an arXiv
   paper. It applies nothing; the article goes to the model intact.
3. **noise** — crypto, ticker moves, deals, podcasts, hiring, HN meta threads,
   clickbait.

Three rules ship switched **off**, because each one has a plausible reason to
be wrong for your feed: press-release wires (a real launch sometimes only
exists there), weekly roundups (you may subscribe to one on purpose) and
consumer gadgets ("Apple brings AI to the iPhone" is real news).

## Working on a rule

```sh
# what would this regex catch in the articles you already have?
giznews rules test "\bshares\b"

# what would a stored rule catch?
giznews rules test --rule "noise: ticker moves"

# what would the whole set claim, and how much is left for the model?
giznews classify --dry-run

# load the file (matched by name — importing twice changes nothing)
giznews rules import docs/rules/noise.json
giznews rules import docs/rules/noise.json --dry-run   # just tell me

# turn one on once you have checked it
giznews rules enable "noise: press-release wires"

# take the database back out to a file
giznews rules export my-rules.json
```

`rules test` runs the same matcher the classifier runs, against the same
haystack (`title + author + URL`, case-insensitive), so what it shows is what
would happen.

## Writing one

- The matcher never sees the body or the source name. The domain is usually in
  the URL, which is how the wire and video rules work.
- Go's regexp is RE2: no lookahead, no backreferences. Use alternation and
  `\b`.
- Prefer a narrow regex over a clever one. `\bshares\b` archives *"OpenAI
  shares new details"*; `\bshares (jump|surge|slide)\b` does not.
- Archiving is reversible — the article is archived, not deleted — but nobody
  reads an archive, so a wrong rule is a silent one.
