# nsdctl

`nsdctl` is the companion CLI of this module. It works on NSD packages you
obtained yourself and never downloads anything.

```bash
go install github.com/otfabric/go-iec61850-model/cmd/nsdctl@latest
# or: make build && ./bin/nsdctl
```

## Global flags

| Flag | Meaning |
| --- | --- |
| `-s, --source PATH` | NSD source: ZIP archive, directory or document; repeatable |
| `-p, --profile ID` | Namespace to select, e.g. `IEC 61850-7-4:2007B5` or just `IEC 61850-7-4`; repeatable. Default: the only release of each standard |
| `--json` | Machine-readable JSON instead of text |
| `--strict` | Treat importer warnings (unsupported elements and attributes) as errors |
| `--lang CODE` | NSDoc language to apply (default `en`; empty disables) |

## Commands

| Command | Purpose |
| --- | --- |
| `inspect [SOURCE...]` | Namespaces, counts, dependencies, digests, manifest, documentation, warnings |
| `validate [SOURCE...]` | Parse, then check each namespace's profile (or `--profile`): dependencies, conflicts, unresolved references |
| `dependencies [SOURCE...]` (`deps`) | Each declared dependency and whether the loaded sources satisfy it: ok, missing, ambiguous |
| `diff OLD NEW [--exit-code]` | Added, removed and changed definitions between two sources |
| `model show NAME\|PATH` | A class (with inherited data objects and semantics), CDC, type, enumeration or basic type; or a resolved path with every step |
| `model list [--kind ln\|abstract\|cdc\|ca\|enum\|basic]` | Definitions of one kind |
| `model search [--name --exact --cdc --type --fc --group --standard --abstract --kind]` | Search across the profile |
| `model describe REF` | Semantic descriptor: quantity, phase, reference, component, unit hint, rule |
| `model measurements LNCLASS` | Mapped measurements of a class, per phase |
| `model leaves LNCLASS[.DO...] [--fc FC]` | All leaf attributes below a class or path |
| `generate go\|docs\|json -o DIR` | Deterministic output; see [generation.md](generation.md) |
| `sources check [--index --lock --packages DIR --tiers T]` | Validate the registry and lock; with `--packages`, check that every tracked package is present and carries its release, and compare digests with the lock |
| `sources lock --packages DIR [--write]` | Produce lock entries (digest, content, dependencies, status) for the tracked packages found in DIR |
| `schema [SOURCE...] [--exit-code]` | XSD constructs (e.g. of IEC 61850-7-7) that the importer does not model |
| `version` | Version, tag, commit, build date |

Examples:

```bash
nsdctl inspect ./nsd/IEC_61850-7-4.zip
nsdctl -s ./nsd validate
nsdctl -s ./nsd -p "IEC 61850-7-4:2007B5" model show MMXU
nsdctl -s ./nsd -p "IEC 61850-7-4:2007B5" model show 'LD0/MMXU1.PNV.phsA.cVal.mag.f[MX]'
nsdctl -s ./nsd -p "IEC 61850-7-4:2007B5" model search --cdc WYE --kind do
nsdctl diff --exit-code ./nsd-2019/ ./nsd-2025/
nsdctl -s ./nsd -p "IEC 61850-7-4:2007B5" generate docs -o ./generated-docs
```

Quote references with brackets in shells that glob (`zsh`).

## Exit codes

| Code | Meaning |
| --- | --- |
| 0 | Success |
| 1 | Usage error, unknown name, path that does not resolve, refused overwrite |
| 2 | A source cannot be read or parsed (also unsafe archives and exceeded limits) |
| 3 | Validation failed: structural errors, unsatisfied dependencies, invalid lock or registry |
| 4 | Differences: `diff --exit-code` found changes, `generate --check` is out of date, a tracked package is missing or carries another release, local packages differ from the lock, `schema --exit-code` found gaps |
| 5 | The license policy refused `generate --publish` |

Errors go to stderr prefixed with `nsdctl:`; results go to stdout.
