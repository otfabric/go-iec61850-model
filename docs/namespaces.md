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

## Comparing releases

`catalog.Diff(old, new)` compares two sources (two namespaces, packages or
profiles) by definition name and reports added, removed and changed classes,
data objects, CDCs, attributes, constructed types, enumerations, literals,
basic types, FCs, presence conditions and dependencies, ordered by path.
