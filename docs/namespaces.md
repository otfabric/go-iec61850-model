# Namespaces, releases and profiles

## Namespace IDs

An NSD document declares `id`, `version`, `revision` and `release`, e.g.
`IEC 61850-7-4`, `2007`, `B`, `5`. `model.NamespaceID` holds them; its string
form is `IEC 61850-7-4:2007B5` and `model.ParseNamespaceID` reads it back. A
`DependsOn` element usually names version and revision only; a release that
matches both satisfies it (`NamespaceID.Satisfies`). A dependency that names a
release is satisfied by that release or a later one of the same version and
revision (releases only add corrections); when several loaded releases
qualify, select one with `--profile` / `Catalog.Profile`.

Namespace releases are tracked separately from the Go module's semantic
version. Publication year of a standard and release date of its NSD package
are not the same thing; see [nsd-sources.md](nsd-sources.md).

## Catalogs hold many releases

`catalog.New` accepts any number of namespaces, including several releases of
the same standard. Two namespaces with the same full ID are rejected. The
catalog orders namespaces by ID only to make output stable; that order implies
no preference.

Lookups on a catalog (`LogicalNode`, `CDC`, ...) return a definition only when
exactly one loaded namespace defines the name. Otherwise they fail with a
`*model.AmbiguityError` that lists the candidates.

## Profiles select

`Catalog.Profile(ids...)` builds a consistent selection:

1. Each ID may be partial (`IEC 61850-7-4`, or with version and revision only).
   It selects the only loaded release that matches, and fails if several do.
2. At most one release per standard; selecting two fails with
   `ErrNamespaceConflict`.
3. Dependencies are added when exactly one loaded release satisfies them. None
   gives `ErrMissingDependency`; several give `ErrAmbiguousReference`; a
   selected release that does not satisfy a dependency gives
   `ErrNamespaceConflict`. All three are `*model.DependencyError`.
4. `Profile.Diagnostics()` reports names defined by two namespaces of the
   profile (lookups of them stay ambiguous) and references to types, classes,
   functional constraints or presence conditions the profile does not define.

`Catalog.DefaultProfile()` succeeds only when the catalog holds exactly one
release of each standard. The project's default policy — the latest
compatible, officially released, available and validated set — will be driven
by the lock file once releases are validated; it is never "highest year per
part".

## Named profiles

`sources/index.json` declares named profiles by IEC part; the
[`official`](../official) package embeds them with the catalog, and `nsdctl
--profile` accepts their names:

| Profile | Parts | Use |
| --- | --- | --- |
| `default` | 7-2, 7-3, 7-4, 7-410, 7-420 | Current core, domain, DER and storage models |
| `extended` | `default` plus 90-3, 90-4, 90-6, 90-9, 90-11, 90-14, 90-21 | Historical and specialised models from the technical reports |

```go
p, err := official.Default()            // *catalog.Profile
ln, err := p.LogicalNode("DBAT")        // IEC 61850-7-420

x, err := official.Extended()
_, err = x.LogicalNode("DBAT")          // ErrAmbiguousSymbol: 7-420 and 90-9 define it
ln, err = x.LogicalNodeIn(model.NamespaceID{Standard: "(Tr)IEC 61850-90-9"}, "DBAT")
```

## Namespace identity

Namespace identity is part of a definition's identity: two classes with the
same name in two namespaces are two definitions, not one. They are never
merged, deduplicated or overwritten, and there is no implicit precedence
between namespaces.

- **Unqualified lookups** (`LogicalNode`, `CDC`, `DataObjects`, ...) fail with
  a `*model.AmbiguityError` (`ErrAmbiguousSymbol`, an alias of
  `ErrAmbiguousReference`) when two namespaces of the set define the name.
- **Qualified lookups** take a namespace, full or partial: `LogicalNodeIn`,
  `InheritanceIn`, `DataObjectsIn`, `DataObjectIn`, `CDCIn`, `CDCVariantsIn`,
  `ConstructedAttributeIn`, `EnumerationIn`. `ResolveNamespace` resolves a
  partial ID; `resolver.Resolver.In(ns)` resolves paths in one namespace.
- **Scoped resolution.** A definition's references resolve in its own
  namespace first, then in its dependencies, level by level (`Scope(ns)`):
  DBAT of 90-9 inherits from the StorageUnitLN of 90-9, and the data objects
  that both inherit from DomainLN come from 7-4. Two namespaces at the same
  level that define the name are an ambiguity, never a choice.
- **Overlap reports.** `Overlaps(left, right)` compares every name two
  namespaces both define: classes by their resolved structure (inheritance
  chain, every data object with its CDC and presence), CDCs, constructed
  attributes and enumerations. Each is `identical`, `compatible-extension` or
  `conflicting`; `nsdctl catalog overlaps` prints the report.

## Transitional namespaces: IEC TR 61850-90-9

IEC TR 61850-90-9 defines transitional electrical energy storage models,
several of which IEC 61850-7-420 later took over. Both define `DBAT`, `DSTO`
and `SBAT`, and the definitions differ, so 90-9 is left out of the default
profile and kept in `extended` (REQUIREMENTS.md, section 5.2.1). Use 7-420 for
new DER applications; load 90-9 to interpret historical devices and to
compare the models:

```text
$ nsdctl catalog overlaps --left IEC_61850-7-420 --right IEC_TR_61850-90-9
KIND  NAME                          LEFT                    RIGHT                      CLASS                 DETAILS
ln    DBAT                          IEC 61850-7-420:2019A4  (Tr)IEC 61850-90-9:2018A3  conflicting           6 only left, 1 only right
ln    DER_StorageLN                 IEC 61850-7-420:2019A4  (Tr)IEC 61850-90-9:2018A3  conflicting           5 only left, 4 only right, 1 differ
ln    DSTO                          IEC 61850-7-420:2019A4  (Tr)IEC 61850-90-9:2018A3  conflicting           5 only left, 4 only right, 1 differ
ln    SBAT                          IEC 61850-7-420:2019A4  (Tr)IEC 61850-90-9:2018A3  compatible-extension  1 only left
ln    StorageNameplateRatingsLN     IEC 61850-7-420:2019A4  (Tr)IEC 61850-90-9:2018A3  compatible-extension  5 only left
ln    StorageOperationalSettingsLN  IEC 61850-7-420:2019A4  (Tr)IEC 61850-90-9:2018A3  compatible-extension  5 only left
ln    StorageUnitLN                 IEC 61850-7-420:2019A4  (Tr)IEC 61850-90-9:2018A3  identical             -
enum  BatteryTypeKind               IEC 61850-7-420:2019A4  (Tr)IEC 61850-90-9:2018A3  conflicting           2 only left, 1 only right
enum  ChargeSourceKind              IEC 61850-7-420:2019A4  (Tr)IEC 61850-90-9:2018A3  identical             -
```

The default profile has one remaining overlap: FSCH, defined by 7-4 and, as a
deprecated class, by 7-410.

## Comparing releases

`catalog.Diff(old, new)` compares two sources (two namespaces, packages or
profiles) by definition name and reports added, removed and changed classes,
data objects, CDCs, attributes, constructed types, enumerations, literals,
basic types, FCs, presence conditions and dependencies, ordered by path.
