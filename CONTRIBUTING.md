# Contributing

## Commits & PR titles (Conventional Commits)

This repo uses [Conventional Commits](https://www.conventionalcommits.org/) with
[release-please](https://github.com/googleapis/release-please) for versioning and
the changelog. PRs are **squash-merged**, so the **PR title** becomes the commit
on `main` and drives the release — it MUST be a valid Conventional Commit (CI
enforces this):

```
<type>[optional scope]: <description>
```

- `feat:` → minor bump · `fix:` → patch bump
- `feat!:` / `fix!:` or a `BREAKING CHANGE:` footer → major bump
- `docs:` `chore:` `refactor:` `test:` `ci:` `perf:` → no release on their own

Examples: `feat: add VPS.WaitForIdle`, `fix: decode plan_price as float64`,
`feat(vps)!: rename Foo to Bar`.

## Branches

Branch from `main`; name it `<type>/<slug>` (e.g. `feat/wait-for-idle`). Branches
are deleted automatically on merge.

## Releases — automated, do not hand-edit

Do **not** tag manually or edit `CHANGELOG.md`. release-please opens a *release
PR* that maintains the version and changelog from merged commits; merging it tags
and publishes. Refine wording in that release PR if needed.

## Local checks

`mise run ci` (lint + test) must pass. `pre-commit install` mirrors the CI hooks.
Keep the SDK stdlib-only and dependency-light (its public interface is a contract).
