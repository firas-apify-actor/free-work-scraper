package main

import (
	"log/slog"
	"os"
)

func main() {
	slog.Info("free-work-scraper started")
	c := NewClient()
	var in map[string]any
	if err := c.Input(&in); err != nil {
		slog.Error("read input", "err", err)
		os.Exit(1)
	}
	slog.Info("input", "input", in)
	slog.Info("free-work-scraper finished")
}
