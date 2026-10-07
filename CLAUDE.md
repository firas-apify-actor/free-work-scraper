# CLAUDE.md — Free-Work Missions Scraper (Apify Actor)

Owner: frayess_mosbehi (Firas Mosbehi)
Status: new Actor, not yet published.

## Product in one sentence

Scrape IT freelance missions and jobs from Free-Work (free-work.com), the main French IT
job board, with structured **day rates (TJM)**, stack, remote mode and location — and a
monitor mode that returns only new missions, for freelancers, IT consulting firms (ESN) and
recruiters.

## Why it can win (market check, Oct 2026)

- Existing Free-Work Actors on Apify Store are small (single-digit monthly users).
- One multi-source competitor charges ~$8 per 1,000 missions.
- We win with: lower price (~$2–3 / 1,000), the cleanest day-rate parsing, and alerts.
- The owner is a French DevOps freelancer: he is the target user. Optimize for what a
  freelancer filters on: TJM, remote, stack, location, duration, start date.

## Before writing scraping code

1. Read `https://www.free-work.com/robots.txt` and the site's terms of use. Only scrape what
   is allowed and publicly visible without login. If anything is unclear, stop and ask the owner.
2. Open the search page in a browser and inspect network requests. **Prefer the site's own
   JSON endpoints** over HTML parsing (faster, cheaper, more stable). Document the endpoints
   you use in `docs/SOURCES.md`.
3. Never log in, never bypass captchas, never collect recruiter personal data (names, emails, phones).

## Stack

- Go 1.23+, stdlib first (`net/http`, `encoding/json`, `log/slog`, `time`). If there is no
  JSON endpoint and HTML must be parsed, `github.com/PuerkitoBio/goquery` is the only allowed
  dependency. No headless browser unless there is no other way (10x more expensive per result).
- There is no official Apify Go SDK. All platform calls live in `apify.go` (small, stdlib only):
  read `INPUT`, push dataset items, get/set records in a named key-value store, charge
  pay-per-event events (`POST /v2/actor-runs/{ACTOR_RUN_ID}/charge`) and track spend against
  `ACTOR_MAX_TOTAL_CHARGE_USD` to know when the charge limit is reached.
  - On the platform it uses `APIFY_TOKEN`, `ACTOR_RUN_ID`, `ACTOR_DEFAULT_DATASET_ID`,
    `ACTOR_DEFAULT_KEY_VALUE_STORE_ID`, `APIFY_PROXY_PASSWORD`.
  - Locally (no `ACTOR_RUN_ID`) it reads/writes `./storage/` in the same layout as the JS SDK
    (`storage/key_value_stores/default/INPUT.json`, `storage/datasets/default/*.json`). With
    `ACTOR_TEST_PAY_PER_EVENT=true` it writes charges to `storage/datasets/charging-log/`.
- Apify proxy (datacenter first) as `http.Transport.Proxy`:
  `http://auto:<APIFY_PROXY_PASSWORD>@proxy.apify.com:8000`. Only use residential proxies if
  datacenter is blocked, and account for its cost in pricing.
- Layout (flat, single `main` package): `main.go`, `apify.go`, `freework.go` (fetch + map),
  `parse.go` + `parse_test.go`, `monitor.go`, `Dockerfile` (multi-stage `golang` →
  `gcr.io/distroless/static`), `go.mod`, `.actor/actor.json`, `.actor/input_schema.json`,
  `.actor/dataset_schema.json`, `README.md`, `CHANGELOG.md`.

## Input (`.actor/input_schema.json`)

| Field | Type | Default | Notes |
|---|---|---|---|
| `keywords` | string | "devops" | Search text |
| `contractTypes` | array | ["freelance"] | freelance, cdi, cdd, internship… (map to site values) |
| `remoteMode` | string | "any" | any, full, partial, none |
| `locations` | array | [] | City or region names |
| `minDailyRate` | integer | – | Filter after parsing, in EUR |
| `publishedWithinDays` | integer | 7 | |
| `maxItems` | integer | 100 | Hard cap |
| `includeDescriptions` | boolean | true | Full description text |
| `onlyNew` | boolean | false | Monitor mode |
| `stateKey` | string | "default" | Separate monitor states per saved task |
| `computeRateBenchmark` | boolean | false | Paid add-on |
| `slackWebhookUrl` | string (secret) | – | Optional alert; mark `isSecret: true` |

Every field: `title`, `description` (in English, one line), and an example in `prefill`.

## Output contract (one item per mission)

Flat, stable, documented in `.actor/dataset_schema.json`:
`id`, `url`, `title`, `company` (company name only), `contractType`, `dailyRateMin`,
`dailyRateMax`, `dailyRateCurrency` ("EUR"), `salaryMin`, `salaryMax`, `remoteMode`,
`location`, `region`, `skills[]`, `durationMonths`, `startDate`, `experienceLevel`,
`publishedAt` (ISO), `description` (optional), `isNew`, `firstSeenAt`, `scrapedAt`.

Parsing rules:
- TJM strings like "450-550 €/j", "500€ HT/jour", "TJM: 600" → numbers. Unparseable → null,
  never guess. Add a table-driven test in `parse_test.go` with at least 15 real-world strings.
- Normalize remote values to: `full`, `partial`, `none`, `unknown`.
- Deduplicate by mission `id` within a run.

## Monetization (pay-per-event, set prices in Console)

| Event name | When | Target price |
|---|---|---|
| `mission-returned` (primary) | One mission pushed to the dataset | $0.0025 |
| `rate-benchmark` | One role/region TJM benchmark (median, p25, p75, sample size) | $0.05 |
| `alert-sent` | One Slack/webhook alert containing new missions | $0.01 |

- No start fee (or the platform minimum). Leaders advertise "no start fee".
- Push the item, then `charge("mission-returned", 1)`; stop as soon as `apify.go` reports the
  charge limit is reached.
- Never charge for: filtered-out items, duplicates, already-seen items in `onlyNew` mode,
  benchmarks with sample size < 5.
- Test with `ACTOR_TEST_PAY_PER_EVENT=true go run .`, check `storage/datasets/charging-log/`.

## Monitor mode

- Named key-value store `free-work-monitor-state`, key = `stateKey`.
- Store seen mission IDs (cap 50,000, drop oldest). Return only unseen missions.
- If `slackWebhookUrl` is set and there are new missions: send ONE message with a compact
  list (title, TJM, remote, location, link), max 20 lines, then charge `alert-sent` once.

## Reliability

- Polite rate: max ~1 request/second per run unless the site clearly allows more.
- Retries with backoff on 429/5xx, honour `Retry-After`.
- If the page structure changes (0 results where results are expected), fail loudly with a clear
  error message instead of returning an empty dataset silently.
- End-of-run log: missions found, returned, filtered, already seen, events charged.

## Store page (README.md)

- Title (Console): **Free-Work Scraper — IT Freelance Missions & Day Rates (TJM)**
- First line contains "Free-Work scraper" and "freelance missions".
- Sections: value prop → price per 1,000 in dollars → output fields table + JSON example →
  use cases (freelancers' job alerts, ESN market watch, TJM benchmarks) → input examples →
  monitor mode + scheduling + Slack → FAQ (public listings only, no personal data, how billing works).
- Add a French one-paragraph summary at the end (French buyers search in French too).
- No invented statistics, no competitor names.

## Workflow

- Repo: `github.com/firas-apify-actor/free-work-scraper`. Work is tracked as GitHub milestones
  (M0 Bootstrap → M7 Growth & pricing) and issues. One branch + PR per issue, `Closes #N`.
- Develop with `go run .` and `storage/key_value_stores/default/INPUT.json` (`maxItems: 20`).
- `go vet ./... && go test ./...` before every commit (CI runs the same plus `docker build`).
- Deploy with `apify push` (Docker build on the platform). Before every push: tests pass, one
  run per main mode, output matches the contract, charging log looks right.
- Bump version in `.actor/actor.json` for input/output changes; update `CHANGELOG.md`.
- Publish first as free or with a low price to collect early runs, then switch on pricing
  once success rate is consistently > 95%.