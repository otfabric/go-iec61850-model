# AGENTS.md — go-iec61850-model

## What this is

Pure Go, transport-independent library for the IEC 61850 information model: a typed, edition-aware catalog generated from the official NSD namespaces (IEC 61850-7-7), plus the `nsdctl` tool. No MMS, GOOSE, Sampled Values or SCL runtime. Profile: **information-model**.

## Organisation standards

This repository follows **otfabric/engineering policy 1.1**, profile **information-model** (`.otfabric/policy.yaml`, mode `enforce`). Organisation skills: `otfabric` Claude Code plugin. Where this file and the organisation standards disagree, this file wins here — say so in your report.

## Where the truth lives

| Question | Source |
|---|---|
| IEC content and what may be committed | docs/licensing.md, CONTRIBUTING.md "Source Policy" |
| Tracked packages and digests | `sources/nsd.lock.json`, docs/nsd-sources.md |
| Generated output | `names`, `official`, `docs/reference` (docs/generation.md) |
| Model semantics | docs/model.md, docs/namespaces.md, docs/semantic.md |
| Errors | ERRORS.md |
| Release history | RELEASE.md (`## vX.Y.Z` … `---`) |

## Hard rules

1. Never commit official IEC material (NSD or NSDoc files, XSD schemas, packages, PDFs, copied text). Public CI runs on synthetic fixtures only.
2. Never lower `COVERAGE_MIN` (90) or `COVER_PKG_MIN`, add lint suppressions, or skip/delete tests to make CI pass.
3. Never edit `names`, `official` or `docs/reference` by hand: they come from `make nsd-publish`.
4. Packages in this library use the standard library only; a new runtime dependency needs an ADR. This library stays on v0 or v1.

## Commands

| When you change… | Run |
|---|---|
| anything | `make check` |
| an NSD or reference parser | `make fuzz` |
| generator output on purpose | `make golden` |
| tracked packages | `make nsd-fetch nsd-check official` |
| committed generated output | `make nsd-publish nsd-publish-check` (needs NSD_CACHE) |

## Definition of Done

`make check` green; tests for new behaviour including error paths; docs updated in the same change; RELEASE.md section for user-visible changes.

## Git and releases

Branches and PRs. Do not push to `main`, tag or run the Release workflow unless the maintainer asks in the session.
