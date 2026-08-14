package main

import (
	"context"
	"fmt"
	"log"

	"github.com/ajramos/giznews/internal/kb"
	"github.com/ajramos/giznews/internal/llm"
)

// runKB manages the knowledge graph: build, list, synth.
func runKB(args []string, logger *log.Logger) {
	if len(args) == 0 {
		logger.Fatal("usage: giznews kb <build|list|synth> [category]")
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
		Model:               cfg.LLM.Model,
		UseLLM:              prov != nil,
	}, prov, logger)
	if err != nil {
		logger.Fatalf("kb: %v", err)
	}

	switch args[0] {
	case "build":
		res, err := svc.Build(ctx)
		if err != nil {
			logger.Fatalf("kb build: %v", err)
		}
		fmt.Printf("kb build: %d atoms, %d electrons created, %d updated (skipped %d)\n",
			res.AtomsCreated, res.ElectronsCreated, res.ElectronsUpdated, res.ArticlesSkipped)

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

	default:
		logger.Fatalf("unknown kb subcommand %q (expected build|list|synth)", args[0])
	}
	_ = context.Background
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-3]) + "..."
}
