# Domain Pitfalls

**Domain:** Side-by-side classical vs post-quantum ratchet protocol demo (Go monorepo)
**Researched:** 2026-04-27
**Library under study:** `github.com/KushnerykPavel/go-doubleratchet` v0.0.2

---

## Critical Pitfalls

Mistakes that cause silent crypto failures, session desync, or builds that never run.

---

### Pitfall 1: InitInitiator receives a `[32]byte`, not `[]byte` — type mismatch at compile time or silent truncation

**What goes wrong:**
`InitInitiator` takes `bobRatchetPK [32]byte` (fixed array), but callers from X3DH or PQXDH handshake results tend to hold public keys as `[]byte` slices. A `[]byte` cannot be passed to `[32]byte` directly; naively converting with `copy` into a zero-initialized array silently pads/truncates without error if the source is wrong length.

Similarly, `InitResponder` takes a `crypto.KeyPair` where both `PrivateKey` and `PublicKey` are `[32]byte` arrays. Feeding a raw seed or an encoded key of wrong length into that struct produces a session that encrypts/decrypts but produces garbage the other side cannot open.

**Warning sign:**
Compilation errors of the form "cannot use []byte as type [32]byte" — or worse, sessions that init cleanly but `Decrypt` always returns an AEAD authentication error.

**Prevention:**
Always convert slice → array with an explicit length check:
```go
var pk [32]byte
if len(raw) != 32 {
    return fmt.Errorf("expected 32-byte public key, got %d", len(raw))
}
copy(pk[:], raw)
```
Write a helper that panics in tests if the length is wrong. Put it in an internal `keys` package so every init site uses the same path.

**Phase that should address it:** Phase 1 (X3DH + Double Ratchet session setup). Write the type-safe conversion helper before the first handshake call.

---

### Pitfall 2: Passing the wrong party's key pair to `InitResponder` (role reversal)

**What goes wrong:**
Double Ratchet asymmetry: the responder (Bob) owns the key pair and the initiator (Alice) only gets Bob's public key. Swapping them — passing Alice's key pair to `InitInitiator` or Bob's private key into the `remotePubKey` slot — produces sessions that encrypt with the wrong root key. Both sides encrypt successfully but `Decrypt` always fails with an authentication error.

The X3DH and PQXDH handshake in this library produces a `HandshakeResult` with three named fields (`RootKey`, `ChainKey`, `PQRKey`). Using `ChainKey` as the `sharedSecret` for Double Ratchet init, or using `RootKey` as both, causes identical silent failure.

**Warning sign:**
Both sides report `Encrypt` success, but `Decrypt` returns a non-nil error on the very first message.

**Prevention:**
- Bob calls `InitResponder(result.RootKey, bobDRKeyPair, cfg)` — use `RootKey` only.
- Alice calls `InitInitiator(result.RootKey, bobDRPub, cfg)` — same `RootKey`, Bob's *public* key only.
- Write a session smoke test immediately after init: Alice encrypts one byte, Bob decrypts; fail fast if error.

**Phase that should address it:** Phase 1 (classical pair wiring). Add the smoke-test as a go test, not a manual check.

---

### Pitfall 3: Calling Decrypt out of order without understanding the skipped-message-key limit

**What goes wrong:**
Double Ratchet stores skipped message keys in a bounded map (`MaxSkip` from `Config`, default `DefaultMaxSkip`). In a pub/sub demo over Centrifugo, messages can arrive out of order if the broker reorders websocket frames (rare but possible). If more messages are skipped than `MaxSkip`, `Decrypt` returns an error and the session state is NOT rolled back — subsequent messages all fail permanently because the chain key has advanced past the skipped range.

**Warning sign:**
After starting the demo under load, some messages decrypt fine then suddenly all further decrypts fail on one side. The error message references "max skip exceeded" or a missing message key.

**Prevention:**
- Keep `MaxSkip` at the default (1000) for a demo — it is generous.
- Use a channel per pair (`ch-classical`, `ch-pq`) so messages from one pair never interleave with the other.
- Log the `N` field from the message header on every decrypt. If `N` jumps unexpectedly, log a warning rather than crashing.
- For the demo: send sequentially (Alice waits for Bob's ack before next message) to avoid any skip scenario.

**Phase that should address it:** Phase 1 (DR wiring), revisit in Phase 3 (Centrifugo integration).

---

### Pitfall 4: `scka.Provider` Snapshot must deep-copy ALL mutable key material

**What goes wrong:**
The Triple Ratchet calls `Snapshot()` before each encrypt/decrypt to support atomic rollback if authentication fails. The library's documentation is explicit: "the snapshot must be a deep copy of ALL mutable state including key material." An implementation that stores a pointer or a shallow copy of a `[]byte` field means rollback silently restores a reference to already-advanced state — after any AEAD failure the session corrupts permanently.

The `MockSCKA` in `scka/testing` does this correctly. Custom KEM-based Provider implementations frequently forget.

**Warning sign:**
After injecting a tampered ciphertext into the PQ session, the session cannot recover even with valid messages.

**Prevention:**
- For the demo, use `MockSCKA` (or a thin wrapper around `crypto/mlkem` that follows `MockSCKA`'s snapshot pattern).
- Snapshot all `[]byte` fields by `append([]byte{}, src...)` not by assignment.
- Test: init a provider, call Send, corrupt the message, call Receive with corrupt msg, then call Receive with the valid original — verify the second Receive succeeds.

**Phase that should address it:** Phase 2 (PQXDH + Triple Ratchet wiring).

---

### Pitfall 5: PQXDH KDF input order — placing `SS` before the DH outputs

**What goes wrong:**
The PQXDH spec and this library's implementation concatenate inputs as:
```
F || DH1 || DH2 || DH3 || [DH4] || SS
```
where `F` is 32 bytes of `0xFF`, and `SS` is the ML-KEM shared secret appended last. A common mistake is either:
- Appending `SS` before any DH output (wrong order)
- Skipping `F` entirely (missing constant prefix)
- Using the initiator's KEM ciphertext bytes instead of `SS` (wrong value)

The library's `pqxdh.SendHandshake` / `pqxdh.ReceiveHandshake` handles this internally. The pitfall surfaces if you implement PQXDH from scratch rather than using the library functions, or if you try to re-derive the key manually for debugging.

**Warning sign:**
Both sides compute `HandshakeResult` but the `RootKey` fields do not match — first message from Alice decrypts to garbage at Bob's side.

**Prevention:**
- Always use the library's `pqxdh.SendHandshake` / `pqxdh.ReceiveHandshake` — do not hand-roll.
- If you must verify manually: print all six inputs (F, DH1–4, SS) as hex and compare byte-for-byte on both sides before HKDF.
- The HKDF call uses SHA-256, salt = 32 zero bytes, length = 96, info = a constant label string. Verify all four HKDF parameters.

**Phase that should address it:** Phase 2 (PQXDH wiring). Add a test that asserts `SendHandshake.RootKey == ReceiveHandshake.RootKey` before wiring into the session.

---

### Pitfall 6: ML-KEM key type confusion — passing `DecapsulationKey` bytes as `EncapsulationKey`

**What goes wrong:**
`crypto/mlkem` uses separate named types: `DecapsulationKey768` (secret, 64-byte seed) and `EncapsulationKey768` (public, 1184 bytes). The API to get the public key is:
```go
dk, _ := mlkem.GenerateKey768()
ekBytes := dk.EncapsulationKey().Bytes()  // 1184 bytes — share this
```
The mistake is serialising the decapsulation key (`dk.Bytes()` = 64 bytes) and sending that as the "public key". `NewEncapsulationKey768` will reject the 64-byte input with an error — but only at runtime. If you use the seed form as the bundle's public key the other side cannot encapsulate.

A second confusion: `ek.Encapsulate()` returns `(sharedKey, ciphertext)` in that order; some callers mistakenly treat the first return as the ciphertext to transmit.

**Warning sign:**
`mlkem.NewEncapsulationKey768(bytes)` returns `error: invalid encapsulation key length` — or Encapsulate panics.

**Prevention:**
- Name variables unambiguously: `kemDecapKey`, `kemEncapKey`, `kemCiphertext`, `kemSharedSecret`.
- Transmit only `dk.EncapsulationKey().Bytes()` (1184 bytes) in the prekey bundle. Never transmit `dk.Bytes()`.
- Assert sizes at construction:
  ```go
  const wantEncapKeySize = mlkem.EncapsulationKeySize768  // 1184
  if len(encapKeyBytes) != wantEncapKeySize { ... }
  ```
- Encapsulate returns `(sharedKey []byte, ciphertext []byte)` — assign both to named variables in the same line.

**Phase that should address it:** Phase 2 (PQXDH + ML-KEM wiring). Test vector: generate key, encapsulate, decapsulate, assert shared secrets match.

---

### Pitfall 7: X3DH DH computation assigns wrong key to wrong role

**What goes wrong:**
X3DH requires exactly four specific DH operations; the library encapsulates them internally through `SendHandshake`/`ReceiveHandshake`, but if you build a prekey bundle incorrectly the computations use the wrong material:
- Putting `OneTimePreKey.PublicKey` in the `SignedPreKey` field (OPK/SPK swap)
- Publishing an `IdentityKey.PublicKey` that was encoded as a signing key rather than a DH key
- Reusing the same ephemeral key across multiple handshakes (destroys forward secrecy)

The `PrekeyBundle` requires `IdentityKey`, `SignedPreKey` (with signature), and optionally `OneTimePreKey`. Swapping SPK and OPK produces a `HandshakeResult` that passes all type checks but yields a different `RootKey` on each side.

**Warning sign:**
`ReceiveHandshake` returns `ErrSignatureVerification` (if SPK signature check fails) or both sides succeed but `RootKey` values differ.

**Prevention:**
- Explicitly label every field by name when constructing `PrekeyBundle` (no positional struct literals).
- Generate a fresh ephemeral key per handshake — never cache it.
- After both sides run their handshake function, assert `alice.RootKey == bob.RootKey` in a unit test before proceeding.

**Phase that should address it:** Phase 1 (X3DH bundle construction). The key-role assertion test must pass before any session init code is written.

---

### Pitfall 8: Triple Ratchet forgets that `TripleRatchetMessage` is a different type from `Message`

**What goes wrong:**
The base `Session.Encrypt` returns `*Message`. The `TripleRatchetSession.Encrypt` returns `TripleRatchetMessage` (a struct combining the EC ratchet message, the SPQR message, and combined ciphertext). Callers who try to encode/send a `TripleRatchetMessage` as if it were a `*Message` — e.g. by JSON-marshalling the wrong struct, or by passing it to a base `Session.Decrypt` — will get either a type error or silent decryption failure.

On the wire, `TripleRatchetMessage` carries substantially more bytes than a classical `*Message` because it embeds SPQR state material. This is exactly the overhead the Grafana dashboard should be measuring — but only if the correct type is being serialised.

**Warning sign:**
The `ch-pq` wire size metrics are identical to `ch-classical` — or both channels show the same byte count per message — meaning the PQ overhead was never included in serialisation.

**Prevention:**
- Create a unified `WireMessage` envelope type with a `protocol` tag (`"classical"` or `"pq"`) and store the appropriate message type under a consistent serialisation contract (e.g. protobuf or JSON with explicit field names).
- Metrics measurement must happen *after* serialising the full `TripleRatchetMessage`, not just the ciphertext field.
- Write a test that asserts `len(serialise(TripleRatchetMessage)) > len(serialise(*Message))`.

**Phase that should address it:** Phase 2/3 (PQ wiring + Centrifugo integration + metrics). Address in phase 2; verify in phase 3 when metrics are wired.

---

### Pitfall 9: PQXDH `HandshakeResult.PQRKey` is not the same as `RootKey` — wrong field fed to SPQR init

**What goes wrong:**
`pqxdh.SendHandshake` returns `HandshakeResult` with three 32-byte fields: `RootKey`, `ChainKey`, `PQRKey`. The Triple Ratchet init expects:
- `sharedSecret` = `RootKey` (fed to EC Double Ratchet root)
- `PQRKey` = seed for SPQR `Provider.InitInitiator`

A common mistake is feeding `RootKey` to both, or feeding `ChainKey` to SPQR. This produces two sessions that each function internally but are cryptographically unrelated — the PQ component provides no real post-quantum protection.

**Warning sign:**
The Triple Ratchet session encrypts/decrypts normally. There is no error. The bug is undetectable at runtime. Only a test that cross-checks `PQRKey` is used as SPQR seed will catch it.

**Prevention:**
- Name all three fields explicitly when destructuring `HandshakeResult`.
- Add an integration test: give SPQR the wrong seed, verify that the Receive call fails (because SPQR internal keys diverge). This forces you to demonstrate that `PQRKey` matters.

**Phase that should address it:** Phase 2 (PQXDH + Triple Ratchet integration test).

---

## Moderate Pitfalls

Mistakes that cause operational problems without breaking crypto correctness.

---

### Pitfall 10: Centrifugo `client.insecure: true` absent — silent connection refusal

**What goes wrong:**
Without either `client.insecure: true` or `client.allow_anonymous_connect_without_token: true` in Centrifugo config, every connection attempt without a JWT token is silently dropped at the server. The `centrifuge-go` client will loop trying to reconnect indefinitely with no useful error message in the default log level. The demo appears to hang on startup.

`allow_anonymous_connect_without_token` is the safer demo option — it allows anonymous clients but still enforces channel-level permissions. Using `insecure: true` disables all permission checks, which is fine for a fully local Docker demo.

**Warning sign:**
`docker compose up` starts cleanly, but no messages appear on either channel. Centrifugo logs show repeated "unauthorized" or the client retries the connect command in a tight loop.

**Prevention:**
Minimal working Centrifugo config for a no-auth demo:
```json
{
  "client": {
    "insecure": true
  },
  "channel_namespaces": [
    {
      "name": "ch",
      "channel_options": {
        "publish": true
      }
    }
  ]
}
```
For the blog demo, document clearly that `insecure: true` is intentional and must never be used in production.

**Phase that should address it:** Phase 3 (Centrifugo integration). Wire up the config before writing any client code.

---

### Pitfall 11: Prometheus per-message metrics with message-ID or timestamp labels

**What goes wrong:**
Adding a label like `message_id` or `unix_ns` to any Prometheus counter or histogram creates one new time series per message. In a running demo, this creates an unbounded cardinality explosion: after a few hours, Prometheus OOMs or query times become unacceptable. The symptom is "too many time series" errors in the Prometheus logs and Grafana dashboards freezing.

For this demo, the only labels needed on message metrics are `protocol` (classical vs pq) and `direction` (send vs recv). That gives 4 time series per metric — permanently bounded.

**Warning sign:**
`prometheus_tsdb_head_series` metric climbs linearly with message count rather than staying flat after warmup.

**Prevention:**
- Approved labels: `protocol` (values: `classical`, `pq`), `direction` (values: `send`, `recv`).
- Forbidden labels on per-message metrics: `message_id`, `session_id`, `user_id`, `timestamp`, `nonce`.
- Use a `Histogram` for latency (not a Gauge with a label for each value).
- Review label list before Phase 4 is considered done.

**Phase that should address it:** Phase 4 (Prometheus metrics). Put a cardinality checklist in the PR review template for that phase.

---

### Pitfall 12: Docker Compose `depends_on` without `condition: service_healthy` causes race

**What goes wrong:**
`depends_on: [centrifugo]` only waits for the Centrifugo container to start running, not for its WebSocket listener to be ready. The Go client processes start immediately, attempt `client.Connect()`, and fail because the TCP listener is not yet accepting connections. The `centrifuge-go` SDK retries with backoff — so this often "works" on fast machines but fails reproducibly in CI or on constrained hardware.

**Warning sign:**
Intermittent "connection refused" errors in client logs during the first 2–3 seconds of `docker compose up`. Grafana shows a gap in metrics at t=0.

**Prevention:**
```yaml
centrifugo:
  image: centrifugo/centrifugo:latest
  healthcheck:
    test: ["CMD", "wget", "-qO-", "http://localhost:8000/health"]
    interval: 2s
    timeout: 5s
    retries: 10
    start_period: 5s

alice-classical:
  depends_on:
    centrifugo:
      condition: service_healthy
```
Centrifugo exposes `/health` at port 8000 by default. All client services should depend on `service_healthy`, not just `service_started`.

**Phase that should address it:** Phase 3 (Docker Compose wiring). Add health check before writing any client container config.

---

### Pitfall 13: `go.work` file breaks Docker multi-stage builds

**What goes wrong:**
A `go.work` file at the monorepo root lists all modules. Inside a Docker multi-stage build, when `COPY` only pulls in one module's directory (common optimization), `go build` fails because the workspace references modules not present in the build context. Error: `module ... not found in workspace`.

Conversely, if you copy the entire repo into Docker, the image becomes very large and layer caching breaks on any file change anywhere in the monorepo.

**Warning sign:**
`docker compose build` works locally but fails in CI, or works only if you `COPY . .` (copying the entire repo).

**Prevention:**
In each service's Dockerfile, set `GOFLAGS=-workfile=off` in the build stage to disable workspace resolution:
```dockerfile
FROM golang:1.23 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN GOFLAGS=-workfile=off go mod download
COPY . .
RUN GOFLAGS=-workfile=off go build -o /service ./cmd/alice-classical
```
Alternatively, copy the entire repo and use a single top-level build context with the `go.work` file included. For a small demo repo this is acceptable.

The `go.work` file should be committed to the repo so that `go work sync` keeps module versions aligned.

**Phase that should address it:** Phase 5 (Docker Compose final wiring). Decide the build strategy (full-copy or per-module + workfile off) before writing Dockerfiles.

---

### Pitfall 14: `go.work` `use` directives vs `replace` — don't mix both for the same module

**What goes wrong:**
If a module is listed under `use` in `go.work` AND under `replace` in a `go.mod` file within the workspace, the workspace `use` takes precedence but the `replace` directive in `go.mod` is silently ignored. This creates an inconsistency where `go mod tidy` inside a submodule produces a different dependency graph than a workspace-level build.

For `go-doubleratchet v0.0.2`, the correct approach is to use a `require` directive pinned to the exact version tag — no `replace`, no `use` for the upstream library. Only local workspace modules (the demo's own binaries) should appear under `use`.

**Warning sign:**
`go mod tidy` in a submodule removes or changes a dependency that the workspace-level build needs. Or `go list -m all` shows different versions depending on whether GOWORK is set.

**Prevention:**
- `go.work` `use` directives: only for modules in this repo (e.g. `./cmd/alice-classical`, `./internal/crypto`).
- `go-doubleratchet` must appear as a versioned `require` in each `go.mod` that needs it — never as a workspace `use`.
- Run `go work sync` after adding or updating modules to keep `go.work.sum` current.

**Phase that should address it:** Phase 1 (monorepo setup).

---

## Minor Pitfalls

Mistakes that cause confusion or wasted debugging time but are easy to fix.

---

### Pitfall 15: Associated data (AD) inconsistency between Encrypt and Decrypt

**What goes wrong:**
Both `Session.Encrypt(plaintext, ad)` and `Session.Decrypt(msg, ad)` take associated data. The AEAD construction binds the ciphertext to the AD — if Alice passes `IKA_pub || IKB_pub` and Bob passes `IKB_pub || IKA_pub` (order reversed), decryption fails with an authentication error that looks identical to a key error.

**Prevention:**
Define AD as a constant function `func sessionAD(initiatorPub, responderPub [32]byte) []byte` that always returns them in the same canonical order (initiator first). Both sides call this function. Never inline the concatenation.

**Phase that should address it:** Phase 1. Define the helper before first encrypt call.

---

### Pitfall 16: `Session.Close()` zeroes key material — calling Encrypt after Close panics

**What goes wrong:**
`Close()` zeroes all key material in the session struct. If the session is stored in a shared struct and `Close()` is called from one goroutine (e.g. shutdown handler) while another goroutine is still in `Encrypt`, the encrypt goroutine accesses zeroed key bytes and panics or silently produces broken ciphertext.

**Prevention:**
Use a mutex or close channel to coordinate shutdown. In the demo, since each Alice/Bob pair runs in a single goroutine per direction, shutdown should be sequential: stop the send loop, flush pending messages, then `Close()`.

**Phase that should address it:** Phase 3 (goroutine architecture for Centrifugo clients).

---

### Pitfall 17: Grafana dashboard JSON hardcodes Prometheus `job` label that doesn't match scrape config

**What goes wrong:**
A committed Grafana dashboard JSON with queries like `ratchet_message_bytes_total{job="demo"}` silently returns no data if the Prometheus scrape config names the job differently (e.g. `job="centrifugal_ratchet"`). Grafana shows empty panels rather than an error.

**Prevention:**
Use a Prometheus `relabel_configs` in `prometheus.yml` to explicitly set `job: "demo"` for the scrape target, and make the dashboard JSON use that exact string. Test dashboards by running `docker compose up` and checking all panels populate before committing the JSON.

**Phase that should address it:** Phase 4 (Grafana dashboard wiring).

---

### Pitfall 18: Triple Ratchet SPQR key material transmitted in chunks — demo may show partial-epoch messages

**What goes wrong:**
Per the Signal SPQR design, Alice's post-quantum public key (EK) is transmitted in chunks across multiple messages; Bob's encapsulation ciphertext (CT) is similarly fragmented. In a low-volume demo exchanging only a handful of messages, the first full PQ epoch may never complete, meaning the `ch-pq` channel shows no PQ ratchet advancement and the metrics reflect only the SPQR overhead bytes (the chunks) without any actual post-quantum key rotation.

This is not a bug — it is correct protocol behaviour — but it looks like the PQ session is "not working" to a reader skimming the metrics.

**Prevention:**
In the demo script, send enough messages to complete at least one full SPQR epoch (check the library's `MockSCKA` for what constitutes a complete epoch). Document this in the README so the blog reader understands what the Grafana dashboard is showing.

**Phase that should address it:** Phase 2 (PQ session wiring) and Phase 5 (README documentation).

---

## Phase-Specific Warnings

| Phase Topic | Likely Pitfall | Mitigation |
|---|---|---|
| Monorepo setup / go.work | Mixed `use`+`replace` for same module (Pitfall 14) | Use `require` for external deps, `use` only for workspace members |
| X3DH bundle construction | OPK/SPK role swap (Pitfall 7) | Key-role assertion test before session init |
| Double Ratchet session init | Wrong key type `[]byte` vs `[32]byte` (Pitfall 1) | Type-safe conversion helper in shared `internal/keys` package |
| Double Ratchet session init | Role reversal: wrong key to wrong init func (Pitfall 2) | Smoke test: encrypt one byte, decrypt, assert success |
| PQXDH wiring | ML-KEM key type confusion (Pitfall 6) | Named variables + size assertions for all KEM values |
| PQXDH wiring | Wrong `HandshakeResult` field to SPQR init (Pitfall 9) | Assert `PQRKey != RootKey` and use explicitly |
| Triple Ratchet wiring | Incorrect `scka.Provider` snapshot implementation (Pitfall 4) | Follow `MockSCKA` deep-copy pattern; rollback recovery test |
| Triple Ratchet message type | `TripleRatchetMessage` vs `*Message` confusion (Pitfall 8) | Wire metrics measurement after full serialisation |
| Centrifugo integration | Missing `insecure: true` → silent connection refusal (Pitfall 10) | Add config before writing any client code |
| Docker Compose | `depends_on` without `condition: service_healthy` (Pitfall 12) | Health check on `/health` endpoint; all clients use `service_healthy` |
| Dockerfiles | `go.work` breaks module-scoped Docker builds (Pitfall 13) | `GOFLAGS=-workfile=off` in builder stage |
| Prometheus metrics | High-cardinality labels (message_id, etc.) (Pitfall 11) | Only `protocol` + `direction` labels on per-message metrics |
| Grafana dashboard | Hardcoded `job` label mismatch (Pitfall 17) | Pin job name in scrape config; verify all panels before commit |
| SPQR epoch size | Demo too short to show full PQ epoch rotation (Pitfall 18) | Send enough messages to complete one SPQR epoch; document in README |

---

## Sources

- Signal PQXDH spec: https://signal.org/docs/specifications/pqxdh/
- Signal Double Ratchet spec: https://signal.org/docs/specifications/doubleratchet/
- Quarkslab Triple Ratchet analysis: https://blog.quarkslab.com/triple-threat-signals-ratchet-goes-post-quantum.html
- Cryspen PQXDH analysis (encoding confusion attack): https://cryspen.com/post/pqxdh/
- crypto/mlkem package docs: https://pkg.go.dev/crypto/mlkem
- go-doubleratchet library (KushnerykPavel): https://github.com/KushnerykPavel/go-doubleratchet
- Docker Compose startup ordering: https://docs.docker.com/compose/how-tos/startup-order/
- Prometheus high cardinality: https://grafana.com/blog/how-to-manage-high-cardinality-metrics-in-prometheus-and-kubernetes/
- Centrifugo configuration: https://centrifugal.dev/docs/server/configuration
- Go module workspaces: https://go.dev/doc/tutorial/workspaces
- go.work + Docker (forum thread): https://forum.golangbridge.org/t/building-workspace-modules-as-individual-projects/32141
- Gabriel Urdhr X3DH notes: https://www.gabriel.urdhr.fr/2024/05/09/x3dh/
