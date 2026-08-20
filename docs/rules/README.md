# Prefilter rulesets

A rule is a regex plus what to do with what it catches. It runs **before** the
LLM classifier, so every article a rule resolves is an article the model never
sees — and never writes a summary or extracts entities for.

That asymmetry is the whole design of `noise.json`: it almost only archives.
Killing junk is free; pre-classifying a good article saves one call and costs
you its summary and its concept links in the vault.

Two files ship here:

- **`noise.json`** — what you never want to read. Archives it.
- **`high-value.json`** — what is worth three stars whatever else happens.
  Boosts it.

## Boost: importance without losing the summary

A plain `importance` action resolves the article and skips the model, which is
exactly backwards for the good stuff: those are the articles most worth having
a summary and entities for, and at ★3 they are the ones that end up in the
knowledge base.

So `high-value.json` uses `boost`, an importance **floor** applied *after* the
model has classified the article: "whatever the classifier decides, this is at
least a 3". It never lowers anything, it keeps the summary and the entities,
and a boosted article is **never archived** — an explicit "this matters"
outranks a pattern that says "this usually does not". That last part also means
boosts do not care where they sit in the file: they annotate, they do not claim.

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

## What earns three stars

One rule per kind of thing that changes what you would do next:

| Rule | Fires on |
| --- | --- |
| frontier model release | a named frontier model **and** a shipping verb, in the same line |
| open weights | open-sourcing **and** a model/weights/checkpoint — the most actionable thing in the feed |
| safety and evaluation landmarks | system cards, preparedness frameworks, the eval vocabulary labs use when publishing something real |
| regulation with teeth | an act, an order, a court, a fine — not policy chatter |
| billion-scale money | a funding or M&A verb **and** a billion-scale number |
| capability milestone | claims that a ceiling moved (SOTA, beats experts, gold medal) |
| compute, chips and supply | export controls, named accelerators, or a billion-scale data centre |
| leadership shake-up | a role **and** a departure — a company name alone is not enough |
| security incident | breaches, leaked weights, CVEs. "Jailbreak" is deliberately absent: there is a paper about one every week |

Most are written as `A.*B|B.*A` because Go's `.` does not cross newlines, so
both halves have to land in the same line — the title — and headlines put them
in either order. Requiring two things is what keeps the false-positive rate
down: `$10 billion` alone is usually quarterly revenue.

Deliberately *not* boosted: a new paper, a seed round, a chip that is not
named, a Show HN. Those are exactly the calls the model is better at than a
regex, and every extra ★3 makes the real ones worth less.

## Working on a rule

```sh
# what would this regex catch in the articles you already have?
giznews rules test "\bshares\b"

# what would a stored rule catch?
giznews rules test --rule "noise: ticker moves"

# what would the whole set claim, and how much is left for the model?
giznews classify --dry-run

# load the files (matched by name — importing twice changes nothing)
giznews rules import docs/rules/noise.json
giznews rules import docs/rules/high-value.json
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
