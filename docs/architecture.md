# Architecture

```text
             Official IEC NSD packages (loaded by the caller)
                               |
                         nsd  (importer)
                               |
                        model (typed IR)
                               |
                  catalog (index, profiles, search, diff)
                               |
               +---------------+---------------+
               |                               |
           resolver  <-------- reference
               |
           semantic (quantities, phases, units, rules)
               |
           ontology (concept graph, JSON export)
               |
        other OTFabric libraries (go-iec61850 via adapter, UIs, tools)
```

## Packages and dependencies

| Package | Imports | Responsibility |
| --- | --- | --- |
| `model` | standard library | Types, provenance, diagnostics, errors |
| `nsd` | `model` | Parse NSD/NSDoc XML and ZIP packages safely, normalize into `model` |
| `catalog` | `model` | Index namespaces, build profiles, qualified and scoped lookups, overlap reports, search, diff |
| `reference` | `model` | Instance and class reference syntax |
| `resolver` | `model`, `catalog`, `reference` | Resolve paths to types |
| `semantic` | `model`, `resolver` | Describe elements in engineering terms from explicit rules |
| `ontology` | `model`, `catalog`, `semantic` | Concept graph, traversal, JSON export |
| `official` | `model`, `catalog` | Generated: the official namespaces embedded, with the named profiles |
| `names` | standard library | Generated: typed constants of the default profile |

`catalog` does not import `nsd`: it accepts any `catalog.Source` (something
with `Namespaces() []*model.Namespace`), which `*nsd.Package`,
`*model.Namespace`, `*catalog.Catalog` and `*catalog.Profile` implement. The
resolver depends on the small `resolver.Lookup` interface, which both
`*catalog.Profile` and `*catalog.Catalog` satisfy.

## Boundaries

- The module **must not import** `go-iec61850`, `go-mms` or any transport stack.
  `go-iec61850` may import this module through a thin adapter. The reference
  format and validation rules match `go-iec61850`'s `Ref`, so values convert
  field by field (`LD`, `LN`, `Path`, `FC`).
- SCL (device configuration) stays in `go-iec61850/scl`. This module owns the
  normative class and type catalog only.
- No network access, no disk writes, no global mutable state. Catalogs and
  profiles are immutable after construction and safe for concurrent reads.

## Data flow

1. `nsd` reads bytes under size limits, pre-scans the token stream (depth,
   tokens, DTDs), decodes into raw structures that collect unknown attributes
   and elements, normalizes them into `model` types with diagnostics, and runs
   structural validation.
2. Package loaders apply NSDoc texts of the selected language to matching
   namespaces and record provenance.
3. `catalog.New` indexes every namespace by name. Lookups on a catalog fail on
   names defined more than once.
4. `Catalog.Profile` selects one release per standard, closes over
   dependencies, and checks conflicts and unresolved references.
5. `resolver` walks LN class → (inherited) data object → CDC → sub data
   objects → data attribute → constructed type members → leaf type, recording
   for every step where it is declared and in which namespace.

6. `semantic` describes a resolved path with the rule that matches its class
   (or a base class) and data object, provided the CDC fits; WYE/DEL sub data
   objects give the phase, the attribute path the component.
7. `ontology` turns a profile, optionally with the semantic descriptors, into
   a concept graph.

## Tools

| Package | Responsibility |
| --- | --- |
| `cmd/nsdctl` | CLI over all packages (cobra); nested module |
| `internal/generator` | Deterministic Go, Markdown and JSON output; write/check; license gate |
| `internal/sources` | Source registry and version lock; digest verification |
| `internal/fixtures` | Test helper loading the synthetic fixtures |

Only `cmd/nsdctl`, a nested module, has a dependency outside the standard library.
