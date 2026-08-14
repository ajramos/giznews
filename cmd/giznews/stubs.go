package main

import (
	"log"
)

// serve is intentionally left for the final polish phase; fetch + classify +
// digest + kb can already be chained manually or via cron.
func runServe(args []string, logger *log.Logger) {
	_, d, _ := loadAndOpenDB(args, logger)
	defer d.Close()
	logger.Fatal("serve: not implemented yet — chain `giznews fetch` in a cron/launchd job for now")
}
