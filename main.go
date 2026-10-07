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

	var fresh []Mission
	var pushErr error
	st, err := NewFetcher().Search(ctx, in, skip, func(m Mission) bool {
		m.FirstSeenAt, m.IsNew = m.ScrapedAt, true
		if at, ok := mon.FirstSeen(m.ID); ok {
			m.FirstSeenAt, m.IsNew = at, false
		}
		if pushErr = c.PushData(m); pushErr != nil {
			return false
		}
		if m.IsNew {
			fresh = append(fresh, m)
		}
		return true
	})
	if err == nil {
		err = pushErr
	}
	slog.Info("summary", "scanned", st.Scanned, "returned", st.Returned, "filtered", st.Filtered,
		"duplicates", st.Dupes, "alreadySeen", st.Seen)
	if err == nil && in.OnlyNew {
		for _, m := range fresh {
			mon.Add(m.ID, m.FirstSeenAt)
		}
		err = mon.Save(c)
		if in.SlackWebhookURL != "" && len(fresh) > 0 {
			if serr := sendSlack(ctx, in.SlackWebhookURL, fresh); serr != nil {
				slog.Warn("slack alert failed", "err", serr) // not fatal: missions are already in the dataset
			} else {
				slog.Info("slack alert sent", "missions", len(fresh))
			}
		}
	}
	if serr := c.SetValue("", "OUTPUT", st); serr != nil && err == nil {
		err = serr
	}
	return err
}
