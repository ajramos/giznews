package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/ajramos/giznews/internal/config"
	"github.com/ajramos/giznews/internal/db"
)

// runSources manages the source registry: list, add, enable, disable.
func runSources(args []string, logger *log.Logger) {
	if len(args) == 0 {
		usage()
		os.Exit(2)
	}

	cfg, err := loadConfig(args)
	if err != nil {
		logger.Fatalf("config: %v", err)
	}
	d, err := db.Open(cfg.ResolveDBPath())
	if err != nil {
		logger.Fatalf("open database: %v", err)
	}
	defer d.Close()

	ctx := context.Background()
	repo := db.NewSourceRepo(d)

	switch args[0] {
	case "list":
		sources, err := repo.List(ctx)
		if err != nil {
			logger.Fatalf("list sources: %v", err)
		}
		if len(sources) == 0 {
			fmt.Println("no sources configured")
			return
		}
		for _, s := range sources {
			state := "enabled"
			if !s.Enabled {
				state = "disabled"
			}
			health := "ok"
			switch {
			case s.ConsecutiveFailures > 0:
				health = fmt.Sprintf("%d failure(s)", s.ConsecutiveFailures)
			case s.EmptyCycles > 0:
				health = fmt.Sprintf("%d empty cycle(s)", s.EmptyCycles)
			}
			if s.LastError != "" {
				health = "!! " + truncate(s.LastError, 40)
			}
			lastOK := s.LastOK
			if lastOK == "" {
				lastOK = "never"
			}
			fmt.Printf("%3d  %-4s  %-8s  %-24s  %-30s  %s\n", s.ID, s.Type, state, truncate(s.Name, 24), health, truncate(lastOK, 22))
		}

	case "add":
		addArgs := args[1:]
		fs := flag.NewFlagSet("add", flag.ContinueOnError)
		name := fs.String("name", "", "display name")
		srcType := fs.String("type", "rss", "rss | hackernews | arxiv | gmail")
		group := fs.String("group", "general", "group name")
		url := fs.String("url", "", "feed URL")
		// --config is a global and may sit anywhere; this flag set does not
		// declare it, and without taking it out first the parse fails loudly
		// over a flag that was perfectly valid.
		_ = fs.Parse(stripConfig(addArgs))
		if *name == "" || *url == "" {
			logger.Fatal("usage: giznews sources add --name NAME --url URL [--type rss] [--group GROUP]")
		}
		s, err := repo.Create(ctx, db.NewSource{
			Name: *name, Type: db.SourceType(*srcType), URL: *url, Group: *group, Enabled: true,
		})
		if err != nil {
			logger.Fatalf("add source: %v", err)
		}
		fmt.Printf("added source #%d: %s (%s)\n", s.ID, s.Name, s.URL)

	case "enable", "disable":
		if len(args) < 2 {
			logger.Fatal("usage: giznews sources <enable|disable> <id>")
		}
		var id int64
		if _, err := fmt.Sscanf(args[1], "%d", &id); err != nil {
			logger.Fatalf("invalid source id %q", args[1])
		}
		if err := repo.SetEnabled(ctx, id, args[0] == "enable"); err != nil {
			logger.Fatalf("%s source: %v", args[0], err)
		}
		fmt.Printf("source #%d %sd\n", id, args[0])

	default:
		logger.Fatalf("unknown sources subcommand %q (expected list|add|enable|disable)", args[0])
	}
}

// loadAndOpenDB is a helper shared by stub commands that only need plumbing.
func loadAndOpenDB(args []string, logger *log.Logger) (*config.Config, *db.DB, context.Context) {
	cfg, err := loadConfig(args)
	if err != nil {
		logger.Fatalf("config: %v", err)
	}
	d, err := db.Open(cfg.ResolveDBPath())
	if err != nil {
		logger.Fatalf("open database: %v", err)
	}
	return cfg, d, context.Background()
}
