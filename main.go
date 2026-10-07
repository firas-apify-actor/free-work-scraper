package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
)

type Input struct {
	Keywords            string   `json:"keywords"`
	ContractTypes       []string `json:"contractTypes"`
	RemoteMode          string   `json:"remoteMode"`
	Locations           []string `json:"locations"`
	MinDailyRate        int      `json:"minDailyRate"`
	PublishedWithinDays int      `json:"publishedWithinDays"`
	MaxItems            int      `json:"maxItems"`
	IncludeDescriptions bool     `json:"includeDescriptions"`
	OnlyNew             bool     `json:"onlyNew"`
	StateKey            string   `json:"stateKey"`
	SlackWebhookURL     string   `json:"slackWebhookUrl"`
	ComputeBenchmark    bool     `json:"computeRateBenchmark"`
	UseApifyProxy       bool     `json:"useApifyProxy"`
}

func defaultInput() Input {
	return Input{Keywords: "devops", ContractTypes: []string{"freelance"}, RemoteMode: "any",
		PublishedWithinDays: 7, MaxItems: 100, StateKey: "default"}
}

func (in Input) validate() error {
	if in.MaxItems < 1 || in.PublishedWithinDays < 1 {
		return fmt.Errorf("maxItems and publishedWithinDays must be >= 1")
	}
	for _, c := range in.ContractTypes {
		if _, ok := contractParam[c]; !ok {
			return fmt.Errorf("unsupported contract type %q (use freelance, cdi or cdd)", c)
		}
	}
	switch in.RemoteMode {
	case "any", "full", "partial", "none":
		return nil
	}
	return fmt.Errorf("unsupported remoteMode %q (use any, full, partial or none)", in.RemoteMode)
}

func main() {
	if err := run(context.Background()); err != nil {
		slog.Error("run failed", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	c := NewClient()
	in := defaultInput()
	if err := c.Input(&in); err != nil {
		return fmt.Errorf("read input: %w", err)
	}
	if err := in.validate(); err != nil {
		return err
	}
	slog.Info("started", "keywords", in.Keywords, "maxItems", in.MaxItems, "onlyNew", in.OnlyNew)

	// Seen-state is read on every run (so isNew/firstSeenAt are meaningful) but only
	// advanced in monitor mode, after a successful run.
	mon, err := LoadMonitor(c, in.StateKey)
	if err != nil {
		return err
	}
	var skip func(int) bool
	if in.OnlyNew {
		skip = func(id int) bool { _, ok := mon.FirstSeen(id); return ok }
	}

	var fresh, all []Mission
	var pushErr error
	limitHit := false
	pushed := 0
	st, err := NewFetcher(in.UseApifyProxy).Search(ctx, in, skip, func(m Mission) bool {
		m.FirstSeenAt, m.IsNew = m.ScrapedAt, true
		if at, ok := mon.FirstSeen(m.ID); ok {
			m.FirstSeenAt, m.IsNew = at, false
		}
		// Charge first so we never hand out an item the user's spending limit doesn't cover.
		n, limit, cerr := c.Charge("mission-returned", 1)
		if cerr != nil {
			pushErr = cerr
			return false
		}
		if n == 0 {
			limitHit = true
			return false
		}
		if pushErr = c.PushData(m); pushErr != nil {
			return false
		}
		pushed++
		all = append(all, m)
		if m.IsNew {
			fresh = append(fresh, m)
		}
		if limit {
			limitHit = true
			return false
		}
		return true
	})
	st.Returned = pushed
	if err == nil {
		err = pushErr
	}
	if limitHit {
		slog.Warn("stopped: the run's maximum charge limit was reached")
	}

	if err == nil && in.ComputeBenchmark && !limitHit {
		err = runBenchmark(c, all)
	}
	if err == nil && in.OnlyNew {
		for _, m := range fresh {
			mon.Add(m.ID, m.FirstSeenAt)
		}
		err = mon.Save(c)
		if in.SlackWebhookURL != "" && len(fresh) > 0 && !limitHit {
			if serr := sendSlack(ctx, in.SlackWebhookURL, fresh); serr != nil {
				slog.Warn("slack alert failed", "err", serr) // not fatal: missions are already in the dataset
			} else if _, _, cerr := c.Charge("alert-sent", 1); cerr != nil {
				slog.Warn("alert charge failed", "err", cerr)
			} else {
				slog.Info("slack alert sent", "missions", len(fresh))
			}
		}
	}

	out := struct {
		Stats
		Charged map[string]int `json:"charged"`
	}{st, c.ChargedCounts()}
	slog.Info("summary", "scanned", st.Scanned, "returned", st.Returned, "filtered", st.Filtered,
		"duplicates", st.Dupes, "alreadySeen", st.Seen, "charged", out.Charged)
	if serr := c.SetValue("", "OUTPUT", out); serr != nil && err == nil {
		err = serr
	}
	return err
}

// runBenchmark charges one rate-benchmark per group (>=5 rated missions) and stores the
// charged ones in the BENCHMARK record of the default key-value store.
func runBenchmark(c *Client, ms []Mission) error {
	var done []Benchmark
	for _, b := range computeBenchmarks(ms) {
		n, _, err := c.Charge("rate-benchmark", 1)
		if err != nil {
			return err
		}
		if n == 0 {
			slog.Warn("benchmark skipped: charge limit reached")
			break
		}
		done = append(done, b)
	}
	slog.Info("rate benchmarks", "groups", len(done))
	return c.SetValue("", "BENCHMARK", done)
}
