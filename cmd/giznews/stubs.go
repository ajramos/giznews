package main

import (
	"fmt"
	"log"
)

// The remaining commands (search, serve) are wired into the config + database
// plumbing now; their real logic lands in the phase that owns them.

func runSearch(args []string, logger *log.Logger) {
	_, d, _ := loadAndOpenDB(args, logger)
	defer d.Close()
	logger.Println("search: not implemented yet (phase 5 — semantic search)")
	fmt.Println("search: nothing to do")
}

func runServe(args []string, logger *log.Logger) {
	_, d, _ := loadAndOpenDB(args, logger)
	defer d.Close()
	logger.Fatal("serve: not implemented yet — run `giznews fetch` manually for now")
}
