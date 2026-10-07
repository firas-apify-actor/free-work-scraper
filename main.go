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
}

func defaultInput() Input {
	return Input{Keywords: "devops", ContractTypes: []string{"freelance"}, RemoteMode: "any",
		PublishedWithinDays: 7, MaxItems: 100}
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
	slog.Info("started", "keywords", in.Keywords, "maxItems", in.MaxItems)

	var pushErr error
	st, err := NewFetcher().Search(ctx, in, func(m Mission) bool {
		pushErr = c.PushData(m)
		return pushErr == nil
	})
	if err == nil {
		err = pushErr
	}
	slog.Info("summary", "scanned", st.Scanned, "returned", st.Returned, "filtered", st.Filtered, "duplicates", st.Dupes)
	if serr := c.SetValue("", "OUTPUT", st); serr != nil && err == nil {
		err = serr
	}
	return err
}
