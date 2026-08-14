package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/ajramos/giznews/internal/llm"
	"github.com/ajramos/giznews/internal/search"
)

// runSearch runs hybrid semantic + keyword search over notes and articles.
func runSearch(args []string, logger *log.Logger) {
	cfg, d, ctx := loadAndOpenDB(args, logger)
	defer d.Close()

	query := ""
	for _, a := range args {
		if strings.HasPrefix(a, "--config=") || strings.HasPrefix(a, "--config") {
			continue
		}
		query = a
		break
	}
	query = strings.TrimSpace(strings.Trim(query, `"`))
	if query == "" {
		logger.Fatal("usage: giznews search \"<query>\"")
	}

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

	svc, err := search.NewService(d, prov, search.Options{Model: cfg.LLM.EmbeddingModel}, logger)
	if err != nil {
		logger.Fatalf("search: %v", err)
	}

	// Ensure embeddings exist for the queried corpus.
	if prov != nil {
		if _, err := svc.Index(ctx); err != nil {
			logger.Printf("search index: %v", err)
		}
	}

	results, err := svc.Search(ctx, query, 15)
	if err != nil {
		logger.Fatalf("search: %v", err)
	}
	if len(results) == 0 {
		fmt.Println("no results")
		return
	}
	for i, r := range results {
		icon := "📄"
		if r.Kind == "note" {
			icon = "🧠"
		}
		fmt.Printf("%2d. %s [%s] %s\n", i+1, icon, r.Kind, r.Title)
		if r.Source != "" {
			fmt.Printf("     %s\n", r.Source)
		}
		fmt.Printf("     %s\n", r.Snippet)
	}
}
