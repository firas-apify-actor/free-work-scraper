# Free-Work scraper — IT freelance missions & day rates (TJM)

**Free-Work scraper** extracts IT **freelance missions** (and CDI/CDD jobs) from Free-Work, the French IT job board, as clean structured data: **day rate (TJM) as numbers**, stack, remote mode, location, duration and start date. Run it once for a market snapshot, or schedule it in **monitor mode** to get only the missions you haven't seen yet, with an optional Slack alert.

Built for freelancers who want job alerts, IT consulting firms (ESN) watching the market, and recruiters benchmarking rates.

## Price

Pay per event, no start fee:

| Event | Price |
|---|---|
| Mission returned | **$2.50 per 1,000 missions** ($0.0025 each) |
| Day-rate benchmark (optional) | $0.05 per benchmark |
| Slack alert sent (optional) | $0.01 per alert |

You only pay for missions that land in your dataset. Missions removed by your filters, duplicates, and missions already seen in monitor mode are free. Set a maximum cost per run and the Actor stops there.

## What you get

One flat item per mission:

| Field | Description |
|---|---|
| `id`, `url` | Mission id and link to the original listing on Free-Work |
| `title`, `company` | Job title and company name |
| `contractType` | `freelance`, `cdi` or `cdd` |
| `dailyRateMin`, `dailyRateMax`, `dailyRateCurrency` | Day rate (TJM) in EUR, `null` when not published |
| `salaryMin`, `salaryMax` | Annual salary in EUR, when published |
| `remoteMode` | `full`, `partial`, `none` or `unknown` |
| `location`, `region` | City and region |
| `skills` | List of technologies |
| `durationMonths`, `startDate` | Mission length and start date (ISO) |
| `experienceLevel` | `junior`, `intermediate`, `senior`, `expert` or `unknown` |
| `publishedAt` | Publication time (ISO 8601) |
| `description` | Plain-text description, only if `includeDescriptions` is on |
| `isNew`, `firstSeenAt` | Monitor-mode flags |
| `scrapedAt` | When the item was collected |

Example:

```json
{
  "id": 123456,
  "url": "https://www.free-work.com/fr/tech-it/job-mission/ingenieur-devops-cloud/example-mission",
  "title": "Senior Platform Engineer / DevOps",
  "company": "Example Conseil",
  "contractType": "freelance",
  "dailyRateMin": 550,
  "dailyRateMax": 650,
  "dailyRateCurrency": "EUR",
  "salaryMin": null,
  "salaryMax": null,
  "remoteMode": "partial",
  "location": "Paris, Île-de-France",
  "region": "Île-de-France",
  "skills": ["Kubernetes", "Terraform", "GitHub Actions"],
  "durationMonths": 12,
  "startDate": "2026-11-02",
  "experienceLevel": "senior",
  "publishedAt": "2026-10-06T17:32:25+02:00",
  "isNew": true,
  "firstSeenAt": "2026-10-08T07:00:03Z",
  "scrapedAt": "2026-10-08T07:00:03Z"
}
```

## Use cases

- **Freelancer job alerts:** schedule a daily run with your keywords, `remoteMode`, minimum TJM and `onlyNew`, and get new missions in a Slack channel.
- **ESN market watch:** track which skills and regions are in demand and at what day rate.
- **TJM benchmarks:** turn on `computeRateBenchmark` for median, 25th and 75th percentile day rates per role and region.

## Input examples

Full-remote DevOps missions paying at least 600 EUR/day, published in the last 7 days:

```json
{
  "keywords": "devops",
  "contractTypes": ["freelance"],
  "remoteMode": "full",
  "minDailyRate": 600,
  "publishedWithinDays": 7,
  "maxItems": 100
}
```

Daily monitor for Kubernetes missions in Île-de-France with a Slack alert:

```json
{
  "keywords": "kubernetes",
  "locations": ["Île-de-France"],
  "onlyNew": true,
  "stateKey": "k8s-idf",
  "slackWebhookUrl": "https://hooks.slack.com/services/..."
}
```

## Monitor mode, scheduling and Slack

1. Set `onlyNew` to `true`.
2. Save the configuration as a **task** and give it its own `stateKey`, so each task remembers the missions it already returned.
3. Add a **schedule** (for example every morning).
4. Optionally add a Slack incoming webhook: you get one compact message (up to 20 lines) per run that finds new missions.

The Actor remembers up to 50,000 mission ids per `stateKey`.

## FAQ

**Is the data public?** Yes. The Actor only reads publicly visible listings, without logging in. It never collects recruiter names, emails or phone numbers, and it removes emails and phone numbers from descriptions.

**How is it billed?** Pay per event, see the price table. A run can never go over the maximum cost you set.

**How many missions can I get?** Up to `maxItems` per run. Requests are throttled to about one per second to stay polite, so large runs take a little time.

**Why is a day rate empty?** Many listings don't publish one. The Actor never guesses: no published rate means `null`.

**What if the site changes?** The Actor fails with a clear error instead of returning an empty dataset silently.

This Actor is an independent tool and is not affiliated with Free-Work. Respect the source site's terms of use when you use the data.

## En français

**Free-Work scraper** récupère les missions freelance IT (et offres CDI/CDD) de Free-Work sous forme de données structurées : **TJM** chiffré, compétences, télétravail, lieu, durée et date de démarrage. Utilisez-le pour une veille ponctuelle ou en **mode monitoring** (uniquement les nouvelles missions) avec alerte Slack. Facturation à l'usage : 2,50 $ pour 1 000 missions, sans frais de démarrage ; seules les missions réellement livrées sont facturées. Données publiques uniquement, aucune donnée personnelle de recruteur.
