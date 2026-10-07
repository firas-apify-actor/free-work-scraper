package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	userAgent = "free-work-scraper/0.1 (+https://github.com/firas-apify-actor/free-work-scraper)"
	pageSize  = 30
	maxPages  = 40 // ponytail: relevance-ordered search can't stop on date; cap = 1200 raw items per contract type
)

// Fetcher is a polite HTTP client: ~1 req/s, retries on 429/5xx, optional Apify proxy.
type Fetcher struct {
	base        string
	client      *http.Client
	minInterval time.Duration
	retryBase   time.Duration
	last        time.Time
}

func NewFetcher() *Fetcher {
	tr := &http.Transport{}
	if pw := os.Getenv("APIFY_PROXY_PASSWORD"); pw != "" {
		u, _ := url.Parse("http://auto:" + pw + "@proxy.apify.com:8000")
		tr.Proxy = http.ProxyURL(u)
	}
	return &Fetcher{
		base:        "https://www.free-work.com",
		client:      &http.Client{Transport: tr, Timeout: 30 * time.Second},
		minInterval: time.Second,
		retryBase:   2 * time.Second,
	}
}

func (f *Fetcher) getJSON(ctx context.Context, path string, v any) error {
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		if wait := f.minInterval - time.Since(f.last); wait > 0 {
			select {
			case <-time.After(wait):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		f.last = time.Now()
		req, err := http.NewRequestWithContext(ctx, "GET", f.base+path, nil)
		if err != nil {
			return err
		}
		req.Header.Set("User-Agent", userAgent)
		req.Header.Set("Accept", "application/json")
		resp, err := f.client.Do(req)
		delay := f.retryBase << attempt
		if err == nil {
			body, rerr := io.ReadAll(resp.Body)
			resp.Body.Close()
			switch {
			case rerr != nil:
				err = rerr
			case resp.StatusCode == 429 || resp.StatusCode >= 500:
				err = fmt.Errorf("HTTP %d", resp.StatusCode)
				if s, e := strconv.Atoi(resp.Header.Get("Retry-After")); e == nil {
					delay = time.Duration(s) * time.Second
				}
			case resp.StatusCode >= 400:
				return fmt.Errorf("GET %s: HTTP %d: %.200s", path, resp.StatusCode, body)
			default:
				if err := json.Unmarshal(body, v); err != nil {
					return fmt.Errorf("GET %s: unexpected response shape (site changed?): %w", path, err)
				}
				return nil
			}
		}
		lastErr = err
		slog.Warn("request failed, retrying", "path", path, "err", err, "in", delay)
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return fmt.Errorf("GET %s: giving up: %w", path, lastErr)
}

type rawJob struct {
	ID              int                   `json:"id"`
	Title           string                `json:"title"`
	Slug            string                `json:"slug"`
	Description     string                `json:"description"`
	PublishedAt     string                `json:"publishedAt"`
	Contracts       []string              `json:"contracts"`
	MinDailySalary  *float64              `json:"minDailySalary"`
	MaxDailySalary  *float64              `json:"maxDailySalary"`
	MinAnnualSalary *float64              `json:"minAnnualSalary"`
	MaxAnnualSalary *float64              `json:"maxAnnualSalary"`
	Duration        *int                  `json:"duration"` // months
	StartsAt        string                `json:"startsAt"`
	RemoteMode      string                `json:"remoteMode"`
	ExperienceLevel string                `json:"experienceLevel"`
	Job             struct{ Slug string } `json:"job"`
	Company         struct{ Name string } `json:"company"`
	Location        struct {
		Label       string `json:"label"`
		AdminLevel1 string `json:"adminLevel1"`
		Key         string `json:"key"`
	} `json:"location"`
	Skills []struct{ Name string } `json:"skills"`
}

// Mission is the flat output item (see CLAUDE.md output contract).
// ponytail: field normalization and rate parsing rules are hardened in M3.
type Mission struct {
	ID                int      `json:"id"`
	URL               string   `json:"url"`
	Title             string   `json:"title"`
	Company           string   `json:"company"`
	ContractType      string   `json:"contractType"`
	DailyRateMin      *float64 `json:"dailyRateMin"`
	DailyRateMax      *float64 `json:"dailyRateMax"`
	DailyRateCurrency string   `json:"dailyRateCurrency"`
	SalaryMin         *float64 `json:"salaryMin"`
	SalaryMax         *float64 `json:"salaryMax"`
	RemoteMode        string   `json:"remoteMode"`
	Location          string   `json:"location"`
	Region            string   `json:"region"`
	Skills            []string `json:"skills"`
	DurationMonths    *int     `json:"durationMonths"`
	StartDate         string   `json:"startDate"`
	ExperienceLevel   string   `json:"experienceLevel"`
	PublishedAt       string   `json:"publishedAt"`
	Description       string   `json:"description,omitempty"`
	ScrapedAt         string   `json:"scrapedAt"`
}

var contractParam = map[string]string{"freelance": "contractor", "cdi": "permanent", "cdd": "fixed-term"}

func (r rawJob) mission(withDesc bool) Mission {
	ct := ""
	for _, c := range r.Contracts {
		if c == "contractor" {
			ct = "freelance"
			break
		}
	}
	if ct == "" && len(r.Contracts) > 0 {
		ct = r.Contracts[0]
		for k, v := range contractParam {
			if v == ct {
				ct = k
			}
		}
	}
	rm := r.RemoteMode
	if rm == "" {
		rm = "unknown"
	}
	m := Mission{
		ID: r.ID, Title: r.Title, Company: r.Company.Name, ContractType: ct,
		URL:          fmt.Sprintf("https://www.free-work.com/fr/tech-it/job-mission/%s/%s", r.Job.Slug, r.Slug),
		DailyRateMin: r.MinDailySalary, DailyRateMax: r.MaxDailySalary, DailyRateCurrency: "EUR",
		SalaryMin: r.MinAnnualSalary, SalaryMax: r.MaxAnnualSalary, RemoteMode: rm,
		Location: r.Location.Label, Region: r.Location.AdminLevel1, DurationMonths: r.Duration,
		ExperienceLevel: r.ExperienceLevel, PublishedAt: r.PublishedAt,
		ScrapedAt: time.Now().UTC().Format(time.RFC3339), Skills: []string{},
	}
	for _, s := range r.Skills {
		m.Skills = append(m.Skills, s.Name)
	}
	if len(r.StartsAt) >= 10 {
		m.StartDate = r.StartsAt[:10]
	}
	if withDesc {
		m.Description = plainText(r.Description)
	}
	return m
}

var (
	tagRe   = regexp.MustCompile(`<[^>]*>`)
	blockRe = regexp.MustCompile(`(?i)</(p|li|ul|ol|h[1-6]|div)>|<br\s*/?>`)
	mailRe  = regexp.MustCompile(`[\w.+-]+@[\w-]+(\.[\w-]+)+`)
	phoneRe = regexp.MustCompile(`(?:\+33|0033|0)\s?[1-9](?:[\s.-]?\d{2}){4}`)
	spaceRe = regexp.MustCompile(`[ \t]+|\n{3,}`)
)

// plainText strips HTML and redacts emails/phone numbers (no recruiter personal data).
func plainText(h string) string {
	s := tagRe.ReplaceAllString(blockRe.ReplaceAllString(h, "\n"), "")
	s = html.UnescapeString(s)
	s = mailRe.ReplaceAllString(s, "[email removed]")
	s = phoneRe.ReplaceAllString(s, "[phone removed]")
	s = spaceRe.ReplaceAllStringFunc(s, func(m string) string {
		if m[0] == '\n' {
			return "\n\n"
		}
		return " "
	})
	return strings.TrimSpace(s)
}

var slugRepl = strings.NewReplacer("é", "e", "è", "e", "ê", "e", "ë", "e", "à", "a", "â", "a", "î", "i", "ï", "i",
	"ô", "o", "ö", "o", "ù", "u", "û", "u", "ü", "u", "ç", "c", "œ", "oe", "'", "", "’", "")
var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

func slug(s string) string {
	return strings.Trim(nonAlnum.ReplaceAllString(slugRepl.Replace(strings.ToLower(s)), "-"), "-")
}

// matchLocation: any name equals a segment of the location key (fr~region~dept~city) or the label's slug contains it.
func matchLocation(r rawJob, names []string) bool {
	if len(names) == 0 {
		return true
	}
	segs := strings.Split(r.Location.Key, "~")
	label := slug(r.Location.Label)
	for _, n := range names {
		n = slug(n)
		for _, s := range segs {
			if s == n {
				return true
			}
		}
		if n != "" && strings.Contains(label, n) {
			return true
		}
	}
	return false
}

type Stats struct {
	Scanned  int `json:"scanned"`  // raw postings fetched
	Filtered int `json:"filtered"` // dropped by input filters
	Dupes    int `json:"duplicates"`
	Returned int `json:"returned"`
}

// validate fails loudly when the API shape changed instead of silently yielding junk.
func validate(r rawJob) error {
	if r.ID == 0 || r.Title == "" || r.PublishedAt == "" || r.Slug == "" {
		return fmt.Errorf("posting missing required fields (id=%d title=%q slug=%q publishedAt=%q): site structure changed?", r.ID, r.Title, r.Slug, r.PublishedAt)
	}
	return nil
}

// Search pages through the API, applies input filters, and calls emit per new mission
// (emit returns false to stop). Dedupes by id.
func (f *Fetcher) Search(ctx context.Context, in Input, emit func(Mission) bool) (Stats, error) {
	var st Stats
	seen := map[int]bool{}
	cutoff := time.Now().AddDate(0, 0, -in.PublishedWithinDays)
	for _, ct := range in.ContractTypes {
		for page := 1; page <= maxPages; page++ {
			q := url.Values{"itemsPerPage": {strconv.Itoa(pageSize)}, "page": {strconv.Itoa(page)},
				"contracts": {contractParam[ct]}}
			if in.Keywords != "" {
				q.Set("searchKeywords", in.Keywords)
			}
			if in.RemoteMode != "any" {
				q.Set("remoteMode", in.RemoteMode)
			}
			var jobs []rawJob
			if err := f.getJSON(ctx, "/api/job_postings?"+q.Encode(), &jobs); err != nil {
				return st, err
			}
			if page == 1 && len(jobs) == 0 && in.Keywords == "" {
				return st, fmt.Errorf("API returned 0 postings for an unfiltered search: site structure changed?")
			}
			for _, r := range jobs {
				if err := validate(r); err != nil {
					return st, err
				}
				st.Scanned++
				if seen[r.ID] {
					st.Dupes++
					continue
				}
				seen[r.ID] = true
				if !in.keep(r, cutoff) {
					st.Filtered++
					continue
				}
				st.Returned++
				if !emit(r.mission(in.IncludeDescriptions)) || st.Returned >= in.MaxItems {
					return st, nil
				}
			}
			if len(jobs) < pageSize {
				break
			}
		}
	}
	return st, nil
}

func (in Input) keep(r rawJob, cutoff time.Time) bool {
	if t, err := time.Parse(time.RFC3339, r.PublishedAt); err == nil && t.Before(cutoff) {
		return false
	}
	if in.RemoteMode != "any" && r.RemoteMode != in.RemoteMode {
		return false
	}
	if in.MinDailyRate > 0 {
		hi := r.MaxDailySalary
		if hi == nil {
			hi = r.MinDailySalary
		}
		if hi == nil || *hi < float64(in.MinDailyRate) {
			return false
		}
	}
	return matchLocation(r, in.Locations)
}
