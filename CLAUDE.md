# sweb-go-sdk — instructions for Claude Code

A Go client for the SpaceWeb (sweb.ru) hosting API (JSON-RPC 2.0 over HTTPS).
It is the shared foundation for the `sweb` CLI and a future Terraform provider —
so the **public interface is a contract**: keep it stable and well-typed.

## Architecture (boundary discipline)

- `client.go` — `Client`, functional options, and the private `call()` transport
  (JSON-RPC envelope, Bearer auth, `{result|error}` decoding, non-200 mapping).
  HTTP/auth/retry are **internal**; consumers never see them.
- `auth.go`, `vps.go`, `config.go` — typed operations grouped by service
  (`Client.VPS.List/Create/AvailableConfig`, `Client.CreateToken`).
- `errors.go` — `*Error` (JSON-RPC error object / non-200).
- **stdlib only.** CLI/UX concerns (Cobra, Viper, Charm) belong in the *CLI* repo,
  never here — the SDK stays dependency-light and importable.

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
  and, since this is a personal repo, a `Co-Authored-By: Claude` trailer.
- **Branch + PR**; do not self-merge — merging is the owner's call.

## Security / opsec (BLOCKING)

- **No real account data in the repo.** Fixtures under `testdata/` are synthetic
  (TEST-NET IPs `203.0.113.0/24`, fake names/ids). When recording real API
  responses for the Evidence phase, scrub tokens, IPs, server/contact IDs, and
  any PII *before* anything lands in git.
- Never commit credentials/tokens. `gitleaks` runs in pre-commit and CI-adjacent.
- Read-only methods (`index`, `getAvailableConfig`) are safe to exercise live;
  `create` mutates and bills — never call it in tests/recon.
