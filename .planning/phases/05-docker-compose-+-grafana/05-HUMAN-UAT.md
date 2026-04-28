---
status: partial
phase: 05-docker-compose-+-grafana
source: [05-VERIFICATION.md]
started: 2026-04-28T00:00:00Z
updated: 2026-04-28T00:00:00Z
---

## Current Test

[awaiting human testing]

## Tests

### 1. End-to-end stack startup
expected: From a clean clone with no pre-built images, run `docker compose up`. All seven services (centrifugo, bob-classical, alice-classical, bob-pq, alice-pq, prometheus, grafana) reach healthy state within ~60 seconds. Grafana dashboard at http://localhost:3000 shows all five panels (2 Stat wire-bytes, 1 wire-size Time Series, 2 latency) populated with live non-zero data.
result: [pending]

## Summary

total: 1
passed: 0
issues: 0
pending: 1
skipped: 0
blocked: 0

## Gaps
