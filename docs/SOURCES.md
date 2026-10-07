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
