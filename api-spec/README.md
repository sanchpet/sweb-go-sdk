# API specification snapshot

A machine-readable snapshot of the SpaceWeb (sweb.ru) JSON-RPC API, as
[OpenRPC](https://spec.open-rpc.org/) documents — one per API object.

SpaceWeb publishes no official machine-readable spec. Its documentation site
[apidoc.sweb.ru](https://apidoc.sweb.ru) is a single-page app that inlines one
OpenRPC document per object into its JavaScript bundle. The generator here
resolves the bundle chunks through the site's `asset-manifest.json`, scans them
for the inlined documents, and writes each as pretty-printed, key-sorted JSON so
the snapshot diffs cleanly when upstream changes.

> **Provenance.** These files are *derived by parsing the public documentation
> site* — they are not an official published artifact. SpaceWeb may change the
> API, or the shape of its docs bundle, at any time and without notice. Treat
> this snapshot as a best-effort mirror, authoritative only insofar as it
> matches the live docs.

## Layout

- `openrpc/<object>.json` — one OpenRPC document per API object, named by the
  object's server URL path (`/vps/ip` → `vps-ip.json`). Where one path serves
  two documents, the smaller is suffixed (`vps-ip-2.json`).
- `gen/` — the generator (`go run ./api-spec/gen`), stdlib only.

## Regenerate

```sh
go run ./api-spec/gen
```

The output is deterministic; a run with no upstream change produces no diff.

## Drift detection

`.github/workflows/api-spec-drift.yml` regenerates the snapshot on a schedule.
When upstream has changed it opens a pull request with the new snapshot and a
tracking issue, so a change to the SpaceWeb API surfaces as a reviewable diff
here and the SDK can react.
