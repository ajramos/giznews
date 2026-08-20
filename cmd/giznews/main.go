// giznews is an AI-powered news reader and knowledge-graph builder.
//
// The CLI exposes the core pipeline (fetch → classify → digest → kb → search).
// The desktop app is a thin presentation layer over the same services.
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/ajramos/giznews/internal/config"
)

const version = "0.1.0-dev"

func main() {
	flag.Usage = usage
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	logger := log.New(os.Stderr, "giznews: ", log.LstdFlags|log.Lmsgprefix)

	switch cmd {
	case "version", "--version", "-v":
		fmt.Printf("giznews %s\n", version)
	case "init":
		runInit(logger)
	case "fetch":
		runFetch(args, logger)
	case "classify":
		runClassify(args, logger)
	case "digest":
		runDigest(args, logger)
	case "kb":
		runKB(args, logger)
	case "search":
		runSearch(args, logger)
	case "serve":
		runServe(args, logger)
	case "sources":
		runSources(args, logger)
	case "rules":
		runRules(args, logger)
	case "gmail-auth":
		runGmailAuth(args, logger)
	case "help", "--help", "-h":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", cmd)
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `giznews — AI-powered news reader and knowledge-graph builder

Usage:
  giznews <command> [options]

Commands:
  init      Create default config, database and knowledge vault skeleton
  fetch     Fetch new articles from all enabled sources
  classify  Classify unread articles (rules ⚡ + LLM; --dry-run to plan it)
  digest    Generate the daily digest (grouped by theme)
  kb        Knowledge-graph operations (build, list, sync)
  search    Semantic + keyword search over notes and articles
  sources   Manage news sources (list, add, enable, disable)
  rules     Manage the ⚡ prefilter (list, test, import, export, add, rm)
  gmail-auth  Run the Gmail OAuth flow (newsletters)
  serve     Run the daemon (background fetch on schedule)
  version   Show version
  help      Show this help

Global options:
  --config PATH   Path to config JSON (default: ~/.config/giznews/config.json)
`)
}

func loadConfig(args []string) (*config.Config, error) {
	fs := flag.NewFlagSet("giznews", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	configPath := fs.String("config", "", "path to config JSON")
	_ = fs.Parse(configArgs(args))
	return config.LoadConfig(*configPath)
}

// configArgs picks the config flag out of a subcommand's arguments, wherever it
// sits. Go's flag package stops at the first positional argument and rejects
// flags it does not know, so neither `kb merge a b --config=x` nor
// `kb build --dry-run --config=x` would otherwise find the config it was given.
func configArgs(args []string) []string {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "-config" || a == "--config" {
			if i+1 < len(args) {
				return []string{"-config", args[i+1]}
			}
			return nil
		}
		if value, ok := strings.CutPrefix(a, "--config="); ok {
			return []string{"-config", value}
		}
		if value, ok := strings.CutPrefix(a, "-config="); ok {
			return []string{"-config", value}
		}
	}
	return nil
}
