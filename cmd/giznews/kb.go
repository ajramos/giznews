package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/ajramos/giznews/internal/db"
	"github.com/ajramos/giznews/internal/kb"
	"github.com/ajramos/giznews/internal/llm"
)

// runKB manages the knowledge graph: build, list, themes, synth.
func runKB(args []string, logger *log.Logger) {
	if len(args) == 0 {
		logger.Fatal("usage: giznews kb <build [--dry-run]|list|themes|synth|index|sync|concepts|merge|alias> [args]")
	}

	cfg, d, ctx := loadAndOpenDB(args[1:], logger)
	defer d.Close()

	var prov llm.Provider
	if cfg.LLM.Enabled {
		var err error
		prov, err = llm.NewProvider(llm.Options{
			Provider: cfg.LLM.Provider,
			Model:    cfg.LLM.Model,
			Endpoint: cfg.LLM.Endpoint,
			Region:   cfg.LLM.Region,
			APIKey:   cfg.LLM.APIKey,
			Timeout:  cfg.LLMTimeout(),
		})
		if err != nil {
			logger.Fatalf("llm: %v", err)
		}
	}

	svc, err := kb.NewService(d, cfg.ResolveVaultPath(), kb.Options{
		ImportanceThreshold: cfg.Classify.ImportanceThreshold,
		MinOccurrences:      cfg.KB.MinOccurrences,
		AgeDays:             cfg.KB.AgeDays,
		ThemeDays:           cfg.KB.ThemeDays,
		Limit:               cfg.KB.Limit,
		Model:               cfg.LLM.Model,
		UseLLM:              prov != nil,
	}, prov, logger)
	if err != nil {
		logger.Fatalf("kb: %v", err)
	}

	switch args[0] {
	case "build":
		if hasFlag(args, "dry-run") {
			plan, err := svc.Preview(ctx)
			if err != nil {
				logger.Fatalf("kb build --dry-run: %v", err)
			}
			printPlan(plan)
			return
		}
		res, err := svc.Build(ctx)
		if err != nil {
			logger.Fatalf("kb build: %v", err)
		}
		fmt.Printf("kb build: %d atoms, %d concepts tracked, %d electrons created, %d updated, %d articles skipped\n",
			res.AtomsCreated, res.ConceptsTracked, res.ElectronsCreated, res.ElectronsUpdated, res.ArticlesSkipped)
		if res.AtomsRefreshed > 0 {
			fmt.Printf("  %d note(s) refreshed from their article\n", res.AtomsRefreshed)
		}
		if res.MoleculesCreated > 0 || res.MoleculesUpdated > 0 {
			fmt.Printf("  %d theme(s) written, %d refreshed\n", res.MoleculesCreated, res.MoleculesUpdated)
		}
		if res.NotesImported > 0 {
			fmt.Printf("  %d note(s) of your own read into the graph\n", res.NotesImported)
		}
		if res.EditedNotesKept > 0 {
			fmt.Printf("  %d note(s) you had edited: your text was kept\n", res.EditedNotesKept)
		}

	case "list":
		noteType := "atom"
		if len(args) > 1 {
			noteType = args[1]
		}
		notes, err := svc.ListNotes(ctx, noteType, 200)
		if err != nil {
			logger.Fatalf("kb list: %v", err)
		}
		for _, n := range notes {
			fmt.Printf("%4d  %-9s  %-40s  %d links\n", n.ID, n.Type, truncate(n.Title, 40), len(n.Wikilinks))
		}
		if len(notes) == 0 {
			fmt.Printf("no %s notes yet — run `giznews kb build`\n", noteType)
		}

	case "themes":
		res, err := svc.BuildThemes(ctx)
		if err != nil {
			logger.Fatalf("kb themes: %v", err)
		}
		if res.Found == 0 {
			fmt.Println("no themes yet — notes have to name the same concepts together first")
			break
		}
		fmt.Printf("kb themes: %d found, %d molecule(s) created, %d updated\n", res.Found, res.Created, res.Updated)

	case "synth":
		if len(args) < 2 {
			logger.Fatal("usage: giznews kb synth <category>")
		}
		res, err := svc.Synthesize(ctx, args[1])
		if err != nil {
			logger.Fatalf("kb synth: %v", err)
		}
		if res.MoleculesCreated == 0 {
			fmt.Printf("no atoms found for category %q (build first?)\n", args[1])
		} else {
			fmt.Printf("molecule created for %q in vault\n", args[1])
		}

	case "sync":
		res, err := svc.SyncVault(ctx)
		if err != nil {
			logger.Fatalf("kb sync: %v", err)
		}
		fmt.Printf("vault sync: %d note(s) imported, %d updated, %d concept mention(s) recorded\n",
			res.Imported, res.Updated, res.Mentions)

	case "index":
		res, err := svc.BuildIndex(ctx)
		if err != nil {
			logger.Fatalf("kb index: %v", err)
		}
		fmt.Printf("vault index refreshed: %d concepts listed, %d unresolved, %d daily note(s)\n",
			res.TopConcepts, res.Dangling, res.DailyNotes)

	case "concepts":
		concepts, err := db.NewConceptRepo(d).Top(ctx, 50)
		if err != nil {
			logger.Fatalf("kb concepts: %v", err)
		}
		for _, c := range concepts {
			state := "electron"
			if c.NoteID == 0 {
				state = "pending"
			}
			fmt.Printf("%4d  %-8s  %-30s  %s\n", c.Mentions, state, truncate(c.Slug, 30), c.Name)
		}
		if len(concepts) == 0 {
			fmt.Println("no concepts yet — run `giznews kb build`")
		}

	case "merge":
		if len(args) < 3 {
			logger.Fatal("usage: giznews kb merge <from-slug> <into-slug>")
		}
		res, err := svc.MergeConcepts(ctx, args[1], args[2])
		if err != nil {
			logger.Fatalf("kb merge: %v", err)
		}
		fmt.Printf("merged %q into %q: %d note(s) relinked, %d mention(s) total\n",
			args[1], args[2], res.NotesRelinked, res.Mentions)
		if res.Redirected {
			fmt.Printf("the old note now redirects to [[%s]]\n", args[2])
		}

	case "alias":
		if len(args) < 3 {
			logger.Fatal("usage: giznews kb alias <alias-slug> <canonical-slug>")
		}
		if err := db.NewConceptRepo(d).Alias(ctx, args[1], args[2]); err != nil {
			logger.Fatalf("kb alias: %v", err)
		}
		fmt.Printf("%q now resolves to %q\n", args[1], args[2])

	default:
		logger.Fatalf("unknown kb subcommand %q (expected build|list|themes|synth|index|sync|concepts|merge|alias)", args[0])
	}
	_ = context.Background
}

// hasFlag reports whether a boolean flag was given, in any position.
func hasFlag(args []string, name string) bool {
	for _, a := range args {
		if a == "--"+name || a == "-"+name {
			return true
		}
	}
	return false
}

// printPlan renders a dry run: what the build would write, before it writes it.
func printPlan(p *kb.BuildPreview) {
	fmt.Printf("dry run — nothing was written\n")
	fmt.Printf("parameters: importance >= %d · last %d days · max %d atoms · %d mention(s) to promote a concept\n\n",
		p.Importance, p.AgeDays, p.Limit, p.MinOccurrences)

	fmt.Printf("%d article(s) would become notes\n", len(p.Atoms))
	for _, a := range p.Atoms {
		fmt.Printf("  ★%d %-9s %-44s → %s\n", a.Importance, a.Category, truncate(a.Title, 44), a.Slug)
		if len(a.Concepts) > 0 {
			fmt.Printf("            concepts: %s\n", strings.Join(a.Concepts, ", "))
		}
	}

	if len(p.Promoting) > 0 {
		fmt.Printf("\n%d concept(s) would get a note\n", len(p.Promoting))
		for _, c := range p.Promoting {
			fmt.Printf("  %-30s %d → %d mention(s)\n", truncate(c.Slug, 30), c.Mentions, c.After)
		}
	}
	if len(p.Pending) > 0 {
		fmt.Printf("\n%d concept(s) would still be waiting\n", len(p.Pending))
		for _, c := range p.Pending {
			fmt.Printf("  %-30s %d → %d mention(s)\n", truncate(c.Slug, 30), c.Mentions, c.After)
		}
	}

	if len(p.Themes) > 0 {
		fmt.Printf("\n%d theme(s) would be gathered into molecules\n", len(p.Themes))
		for _, t := range p.Themes {
			state := "refreshed"
			if t.New {
				state = "new"
			}
			fmt.Printf("  %-30s %2d note(s)  %-9s %s\n", truncate(t.Slug, 30), t.Notes, state, truncate(t.Title, 40))
		}
	}

	fmt.Printf("\n%d note(s) would be refreshed from their article\n", p.StaleAtoms)
	fmt.Printf("%d note(s) of your own would be read in, %d re-read after your edits\n", p.VaultNew, p.VaultEdits)
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-3]) + "..."
}
