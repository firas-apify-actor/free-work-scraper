# Changelog

## 1.0 — 2026-10-08
- First release: search Free-Work missions by keywords, contract type, remote mode, location, publication date and minimum day rate.
- Structured day rates (TJM), skills, duration, start date, experience level.
- Monitor mode (`onlyNew`, `stateKey`) with Slack alert.
- Optional day-rate benchmark (`computeRateBenchmark`).
- Pay-per-event billing: `mission-returned`, `rate-benchmark`, `alert-sent`.
- `useApifyProxy` option (off by default: requests from Apify's own IPs work; the datacenter proxy was refused for this site).

## 0.2
- Full input schema and dataset schema, rate parser.

## 0.1
- Bootstrap.
