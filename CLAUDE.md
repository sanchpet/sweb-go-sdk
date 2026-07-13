# sweb-go-sdk — instructions for Claude Code

A Go client for the SpaceWeb (sweb.ru) hosting API (JSON-RPC 2.0 over HTTPS).
It is the shared foundation for the `sweb` CLI and a future Terraform provider —
so the **public interface is a contract**: keep it stable and well-typed.

## Architecture (per-service packages, ADR-0019)

- `internal/transport` — `transport.Client`, functional `Option`s, and `Call`
  (JSON-RPC envelope, Bearer auth with transparent token refresh, `{result|error}`
  decoding, non-200 mapping, the `getToken` exchange). HTTP/auth/retry are
  **internal/** — compiler-enforced unimportable by outside consumers.
- One package per service — `vps`, `ip`, `backup`, `remotebackup`, `dns`,
  `domains`, `balancer`, `dbaas`, `ssl`, `monitoring` (+ `monitoring/checks`,
  `monitoring/contacts`). Each carries its own local vocabulary (`vps.Service`,
  `balancer.Config`, `dns.Record`) with no cross-service name collision; a
  `Service{ t *transport.Client }` whose methods call `s.t.Call(ctx, <ep>, …)`.
  `New(t)` constructs one over the shared transport.
- `sweb.go` — the root **facade**: `New()` wires the service clients over one
  transport and exposes them as fields (`Client.VPS`, `Client.IP`, …), preserving
  every call site; it re-exports the options (`sweb.WithToken` = `transport.WithToken`)
  and delegates `Client.CreateToken`.
- `flex` — `flex.Int` / `flex.Float` (see conventions), a public leaf.
- `apierr` — `*apierr.Error` (JSON-RPC error object / non-200), a public leaf.
- Dependency direction is acyclic: `sweb → {services, transport}`;
  `services → {transport, flex, apierr}`; `transport → {flex, apierr}`.
- **stdlib only.** CLI/UX concerns (Cobra, Viper, Charm) belong in the *CLI* repo,
  never here — the SDK stays dependency-light and importable.

## API conventions (learned against the live API — keep to them)

- **Numbers arrive polymorphic.** SpaceWeb quotes numeric fields inconsistently
  (bare `1`, quoted `"1024"`, or `null`) and even returns money as `int`-or-`float`.
  Decode every numeric API field through **`flex.Int` / `flex.Float`**, never a bare
  `int`/`float64` — a strict type panics on real payloads. Same for shape drift:
  a field documented as an array can arrive as a bare object when populated
  (`local_ip`), so tolerant `UnmarshalJSON` over strict typing.
- **Mutating actions answer `1`/`0`.** Action methods (`rename`, `changePlan`,
  `powerOn`/`powerOff`/`reboot`, `addLocal`, …) return `1` on success and `0` on
  failure — decode into `flex.Int` and treat non-`1` as an error. Group siblings
  behind a private helper (`vps.powerAction`, `ip.localAction`). **Sentinels are
  not uniform even within one endpoint:** on `/domains/dns`, `editMx` answers
  integer `1` but `editSrv`/`editNS`/`editTxt`/`editMain` answer boolean `true`
  (hence the split `DNS.editOne`/`DNS.editBool` helpers). Confirm the sentinel per
  method, don't assume the endpoint's first method sets the rule.
- **Async lifecycle.** create/resize/power settle over a *sequence* of async
  actions with `is_running` staying `1`; "settled" means `current_action` is idle,
  not `is_running == 1`. Poll via `WaitForIdle` (reads `index.current_action`).
- **Evidence-first typing.** Types are reconciled against *recorded real
  responses*; a method whose result shape hasn't been observed live is left
  `json.RawMessage` (e.g. `create`) rather than guessed. Document doc-vs-reality
  gaps inline (see the `changePlan` / `getFirstOrderInfo` notes).
- **Tests are offline.** Exercise handlers against the `serve(t, handler)` mock
  server; never hit the live API in tests — `create` mutates and bills.

## Build & test (mise-first)

```sh
mise install      # Go + golangci-lint + pre-commit, pinned in mise.toml
mise run test     # go test ./...
mise run lint     # golangci-lint run
mise run ci       # lint + test (what CI runs)
pre-commit install && pre-commit run -a
```

## Conventions

- **English** for all repo artifacts (code, comments, docs, commits, PRs).
- Commits: small and focused; end every commit with `Signed-off-by:` (`--signoff`)
  and, since this is a personal repo, an `Assisted-By: Claude <noreply@anthropic.com>`
  trailer (the owner authors, Claude assists — not `Co-Authored-By`).
- **Branch + PR**; do not self-merge — merging is the owner's call.
- **Conventional Commits + release-please (BLOCKING):** commit / PR-title format is
  `<type>[scope]: <desc>` (`feat`→minor, `fix`→patch, `!` or `BREAKING CHANGE`→major).
  PRs are squash-merged, so the **PR title is the release commit** — CI enforces its
  format (`pr-title` workflow). Versioning and `CHANGELOG.md` are automated by
  **release-please** — never `git tag` or edit the changelog by hand; merge the
  release PR it opens. See `CONTRIBUTING.md`.

## Security / opsec (BLOCKING)

- **No real account data in the repo.** Fixtures under `testdata/` are synthetic
  (TEST-NET IPs `203.0.113.0/24`, fake names/ids). When recording real API
  responses for the Evidence phase, scrub tokens, IPs, server/contact IDs, and
  any PII *before* anything lands in git.
- Never commit credentials/tokens. `gitleaks` runs in pre-commit and CI-adjacent.
- Read-only methods (`index`, `getAvailableConfig`) are safe to exercise live;
  `create` mutates and bills — never call it in tests/recon.
