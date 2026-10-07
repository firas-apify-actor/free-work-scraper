# Sources — Free-Work (free-work.com)

## 1. Compliance check (checked 2026-10-08)

### robots.txt (https://www.free-work.com/robots.txt, last modified 2026-07-20)
- `User-agent: *` disallows only `/login`, `/logout`, `/fw-deals`. Job and mission search pages are **not** disallowed.
- No `Crawl-delay`. Specific bots (Yandex, Baidu, HTTrack, Wget, MJ12bot, ...) are fully disallowed; `OAI-SearchBot` is allowed.
- We must not send a Wget/HTTrack-like User-Agent. Use an honest, descriptive one.

### Terms of use (https://www.free-work.com/fr/terms, section 7 "Propriété intellectuelle")
Relevant clauses (French, quoted):
- "Il est également interdit à l'Utilisateur de faire du Site ou d'un quelconque de ses éléments une exploitation professionnelle ou commerciale, directement ou indirectement, seul ou avec des tiers, à titre onéreux ou gratuit."
- "L'Utilisateur reconnaît qu'il lui est également interdit d'extraire une part substantielle (en quantité ou en qualité) de la base de données propriété de Free-Work/AGSI SAS et de la fixer par tous moyens sur un autre support, d'en faire une exploitation commerciale ou autre, la diffuser, la partager, la reproduire."
- "Toute reproduction, diffusion, représentation, combinaison ou modification [...] d'un élément quelconque du Site ou des Services est interdite."
- The legal mentions (`/fr/terms/legal-mentions`) reserve all reproduction rights.

### Assessment
- **Technically allowed** by robots.txt for public pages.
- **Contractually risky**: the terms forbid commercial exploitation of the Site and extraction of a *substantial part* of its database (sui generis database right, EU Directive 96/9/EC). A paid Actor that returns missions at scale is commercial exploitation of that data. This is not a clear green light.
- Not a legal opinion. Owner decision required before building the scraper (M2).

### Hard rules (from CLAUDE.md)
No login, no captcha bypass, no recruiter personal data (names, emails, phones), public pages only.

### Mitigations if the owner decides to proceed
- Keep volume low and polite (~1 req/s, honest User-Agent, `maxItems` caps, no full-catalogue dumps).
- Return only factual fields (title, company, rate, remote, location, skills, dates, URL); link back to the original; do not republish full descriptions by default (`includeDescriptions` defaults to false).
- Consider asking Free-Work/AGSI for written permission or an official feed/API.
- Be ready to take the Actor down if asked.

## 2. Endpoints (discovered 2026-10-08)

Use an honest `User-Agent` (`free-work-scraper/<version> (+https://github.com/firas-apify-actor/free-work-scraper)`), `Accept: application/json`, no cookies, no login. Plain `curl` works; no captcha or bot wall was met on datacenter-free requests (not yet tested through Apify datacenter proxy).

### Primary: JSON API (what the website's own front-end calls)
`GET https://www.free-work.com/api/job_postings`

Returns a bare JSON array (no total count, no pagination envelope). 8 test calls in total were enough to map it.

| Param | Notes |
|---|---|
| `searchKeywords` | Free text. (`query`/`q` are ignored.) With keywords the order is relevance; **without keywords the order is newest first**. |
| `contracts` | Scalar: `contractor`, `permanent`, `fixed-term` (other values e.g. internship not observed). Array syntax (`contracts[]=`) → HTTP 400. |
| `remoteMode` | `full`, `partial`, `none` (**confirmed server-side filter**, 2026-10-08; also re-checked client-side). |
| `page` | 1-based. |
| `itemsPerPage` | Accepted (tried 2–30); the scraper uses 30. Pages can overlap slightly (dedupe by `id`). |
| `premium=false` | Used by the site's home widget; not needed. |

Unknown params are **silently ignored** (no error), so every filter must be verified by checking the result, not the status code. Tried and ignored: `locations`, `location`, `sort`, `order`, `orderBy`, `publishedSince`, `publishedAt`. `order[...]` → 400.

### Fallback: server-rendered search page
`GET https://www.free-work.com/fr/tech-it/jobs?query=…&contracts=contractor&remote=full&locations=fr~ile-de-france~paris~paris&page=2`

A Nuxt page whose `<script id="__NUXT_DATA__">` holds a devalue-encoded payload; `data["jobs-search-/fr/tech-it/jobs"]` = `{jobs[16], totalItems, filters, page}`. It **does** honour `locations` and gives `totalItems`, but it carries fewer fields (single `dailySalary`, no min/max) and is HTML-sized (~700 KB/page). Keep only as fallback for location filtering / totals.

### Response fields we use (API)
`id`, `title`, `slug`, `job.slug`, `company.name`, `contracts[]`, `minDailySalary`/`maxDailySalary` (numbers, EUR, **already structured: no TJM string parsing needed**), `minAnnualSalary`/`maxAnnualSalary`, `currency`, `remoteMode`, `location.{label,locality,adminLevel1,adminLevel2,key,latitude,longitude}`, `skills[].name`, `duration`+`durationPeriod` (`month`/`year`), `startsAt`, `experienceLevel` (`junior?`/`intermediate`/`senior`/`expert`/null), `publishedAt`, `description` (HTML), `renewable`.

Mission URL: `https://www.free-work.com/fr/tech-it/job-mission/{job.slug}/{slug}`.

**Never output:** `applicationContact`, `applicationUrl`, `applicationForm`, `candidateProfile`, `companyDescription`, `company.*` beyond the name. `applicationContact` was null in all 30 sampled items, but free-text descriptions can contain emails/phone numbers (1 of 30 matched): strip them when `includeDescriptions` is on.

Sample (trimmed, texts cut to 200 chars): `testdata/job_postings_sample.json`.

## 3. Input → site parameter mapping

| Input | Site | Notes |
|---|---|---|
| `keywords` | `searchKeywords` | |
| `contractTypes` `freelance` / `cdi` / `cdd` | `contracts=contractor` / `permanent` / `fixed-term` | One API call per contract type, dedupe by `id`. `internship` not observed: reject with a clear error until verified. |
| `remoteMode` `any` / `full` / `partial` / `none` | no param / `full` / `partial` / `none` | Also re-checked client-side on the `remoteMode` field. |
| `locations` (city/region names) | match `location.key` prefix client-side (`fr~{region}~{dept}~{city}`, slugified, empty segment = wildcard) | API has no location filter; scan more pages. Page endpoint supports `locations=<key>` if volume gets too high. |
| `publishedWithinDays` | client-side on `publishedAt` | Without keywords results are newest-first, so stop at the first older item. With keywords order is relevance: scan up to a page cap. |
| `minDailyRate` | client-side on `maxDailySalary` (fallback `minDailySalary`) | |
| `maxItems` | stop condition | |
| `includeDescriptions` (default **false**, mitigation from §1) | strip HTML, redact emails/phones from `description` | Always present in the list response: no detail call needed → half the requests. |

## 4. Open questions for M2
1. Does the `/api/job_postings` filter param for location exist (e.g. `locations` in a different casing)? Otherwise client-side matching.
2. ~~remoteMode API filter~~ confirmed (`remoteMode=full|partial|none`).
3. Max safe `itemsPerPage`; does the datacenter proxy get blocked?
4. Is there an internship contract value?
