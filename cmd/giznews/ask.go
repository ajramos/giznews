package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/ajramos/giznews/internal/llm"
	"github.com/ajramos/giznews/internal/search"
)

// runAsk answers a question from the vault, with citations to the notes it read.
func runAsk(args []string, logger *log.Logger) {
	cfg, d, ctx := loadAndOpenDB(args, logger)
	defer d.Close()

	question := strings.TrimSpace(firstNonFlag(args))
	if question == "" {
		logger.Fatal(`usage: giznews ask "<question>"`)
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
		logger.Fatalf("ask: %v", err)
	}

	answer, err := svc.Ask(ctx, question, search.AskOptions{
		Model:    cfg.LLM.Model,
		Language: cfg.LLM.Language,
	})
	if err != nil {
		logger.Fatalf("ask: %v", err)
	}

	if answer.Grounded {
		fmt.Printf("\n%s\n\n", answer.Text)
	} else if len(answer.Sources) == 0 {
		fmt.Println("\nNothing in your vault touches that.")
		return
	} else {
		// Never fill the gap with the model's own knowledge: say plainly that
		// this is a ranking, not an answer.
		fmt.Println("\nNo answer written — this is what the vault has on it:")
	}

	fmt.Println("Sources:")
	for _, src := range answer.Sources {
		switch {
		case src.Slug != "":
			fmt.Printf("  [[%s]] — %s\n", src.Slug, src.Title)
		default:
			fmt.Printf("  %s — %s\n", src.Title, src.Source)
		}
	}
	for _, slug := range answer.Dropped {
		fmt.Printf("  ! dropped an invented citation to %q\n", slug)
	}
}
