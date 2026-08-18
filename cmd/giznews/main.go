// giznews is an AI-powered news reader and knowledge-graph builder.
//
// The CLI exposes the core pipeline (fetch → classify → digest → kb → search).
// The desktop app is a thin presentation layer over the same services.
package main

import (
	"flag"
	"fmt"
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
  classify  Classify unread articles (rules ⚡ + LLM)
  digest    Generate the daily digest (grouped by theme)
  kb        Knowledge-graph operations (build, list, sync)
  search    Semantic + keyword search over notes and articles
  sources   Manage news sources (list, add, enable, disable)
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
	configPath := fs.String("config", "", "path to config JSON")
	_ = fs.Parse(flagArgs(args))
	return config.LoadConfig(*configPath)
}

// flagArgs keeps only the flags out of a subcommand's arguments. Go's flag
// package stops at the first positional argument, so without this
// `kb merge a b --config=x` would silently fall back to the default config.
func flagArgs(args []string) []string {
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "-") {
			continue
		}
		out = append(out, a)
		// "--config path" carries its value in the next argument.
		if !strings.Contains(a, "=") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
			i++
			out = append(out, args[i])
		}
	}
	return out
}
