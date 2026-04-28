---
status: partial
phase: 03-centrifugo-integration
source: [03-VERIFICATION.md]
started: 2026-04-28
updated: 2026-04-28
---

## Current Test

[awaiting human testing]

## Tests

### 1. Centrifugo health endpoint responds
expected: Start Centrifugo with `centrifugo -c centrifugo/config.json`, then `curl http://localhost:8000/health` returns HTTP 200 with `{"status":"ok"}` or similar
result: [pending]

### 2. Full end-to-end exchange (classical pair)
expected: With Centrifugo running, start `bob-classical` then `alice-classical`. Both connect, handshake completes, alice sends 5 messages, bob echoes 5 back, both exit 0.
result: [pending]

### 3. Full end-to-end exchange (PQ pair)
expected: With Centrifugo running, start `bob-pq` then `alice-pq`. Both connect, PQXDH handshake completes, alice sends 5 messages, bob echoes 5 back, both exit 0.
result: [pending]

## Summary

total: 3
passed: 0
issues: 0
pending: 3
skipped: 0
blocked: 0

## Gaps
