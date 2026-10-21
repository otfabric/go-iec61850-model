# Error Taxonomy

go-iec61850-model uses Go's standard error model: sentinel values for category
checks (`errors.Is`) and typed error structs for structured detail
(`errors.As`). All sentinels and typed errors live in package
[`model`](model/errors.go), so one check works across `nsd`, `catalog`,
`reference` and `resolver`.

Messages start with `iec61850-model:`.

---

## Sentinel errors

| Sentinel | When returned |
| --- | --- |
| `ErrInvalidNSD` | Malformed XML, a document with structural errors, an invalid archive, a namespace without ID |
| `ErrUnsupportedSchema` | Unknown root element or XML namespace; a non-NS document where a namespace is required |
| `ErrLimitExceeded` | A size, depth, token, entry, nesting, compression-ratio or path-depth bound was exceeded |
| `ErrUnsafeArchive` | Absolute, traversing, backslash or drive-letter entry names; duplicate or non-regular entries; two manifests |
| `ErrMissingDependency` | A namespace dependency is not loaded |
| `ErrNamespaceConflict` | Two namespaces with the same ID; two releases of one standard in a profile; a selected release that does not satisfy a dependency |
| `ErrUnknownNamespace` | A requested namespace is not loaded; an unparsable namespace ID |
| `ErrUnknownLogicalNode` | A logical node class (or base class) is not defined |
| `ErrUnknownDataObject` | A logical node class has no such data object, including inherited ones |
| `ErrUnknownCDC` | A CDC is not defined |
| `ErrUnknownAttribute` | A CDC or constructed type has no such member, or the attribute has a different functional constraint |
| `ErrUnknownType` | A basic, constructed or enumerated type cannot be found or determined |
| `ErrInvalidReference` | A reference is syntactically invalid |
| `ErrAmbiguousReference` | A name is defined by more than one namespace, or a partial namespace ID or dependency matches several releases |
| `ErrCycle` | Cyclic class inheritance or a type that contains itself |
| `ErrSourceUnavailable` | A file, directory or reader cannot be opened or read; a nil source |
| `ErrNoMapping` | No semantic rule applies to an element, or the rule does not fit the model (CDC, sub data object); a class is not a measured value |
| `ErrUnknownUnit` | A unit string, SIUnit value or multiplier value cannot be interpreted |
| `ErrInvalidRule` | A semantic rule or override is incomplete, names an unknown quantity or an unfitting unit, or conflicts with another |
| `ErrUnknownConcept` | An ontology concept ID is not in the graph |

---

## Typed errors

### `*ReferenceError`

A malformed reference. Unwraps to `Wrapped` when set, otherwise to
`ErrInvalidReference`.

```go
type ReferenceError struct {
    Input   string // the reference that failed to parse
    Reason  string
    Wrapped error
}
```

### `*ResolutionError`

A model path that cannot be resolved, or a resolved path without a semantic
mapping (`ErrNoMapping`). Unwraps to `Err`.

```go
type ResolutionError struct {
    Namespace NamespaceID // namespace of the definition searched, when known
    Path      string      // path resolved so far, e.g. "MMXU.Hz.mag"
    Element   string      // name that failed
    Expected  string      // e.g. the requested FC
    Actual    string      // e.g. the FC the attribute has
    Reason    string
    Err       error       // ErrUnknownDataObject, ErrUnknownAttribute, ...
}
```

### `*AmbiguityError`

A lookup that matched several definitions. Unwraps to `ErrAmbiguousReference`.
`Candidates` lists the namespaces; select one with a `catalog.Profile`.

```go
type AmbiguityError struct {
    Kind       string // "logical node class", "CDC", "namespace", ...
    Name       string
    Candidates []NamespaceID
}
```

### `*DependencyError`

A dependency that a profile cannot satisfy. Unwraps to `Err`:
`ErrMissingDependency`, `ErrAmbiguousReference` (several candidate releases) or
`ErrNamespaceConflict` (the profile selects an incompatible release).

```go
type DependencyError struct {
    Namespace  NamespaceID   // declares the dependency
    Required   NamespaceID   // as declared
    Candidates []NamespaceID
    Reason     string
    Err        error
}
```

### `*SourceError`

A failure tied to an input: file, archive entry (`archive.zip!entry.nsd`) or
reader. Unwraps to `Err`, which joins the sentinel with the underlying error
where there is one (an `os` or `encoding/xml` error).

```go
type SourceError struct {
    Source string
    Reason string
    Err    error
}
```

### `*ValidationError`

A document or namespace with error diagnostics. Unwraps to `Err`, by default
`ErrInvalidNSD`. Each `model.Diagnostic` carries severity, a stable code,
namespace, source and XML path.

```go
type ValidationError struct {
    Diagnostics []Diagnostic
    Err         error
}
```

---

## Diagnostics

Not every finding is an error. The importer reports unsupported elements and
attributes as warnings (errors with `nsd.WithStrict()`), and informational
findings such as unprocessed archive entries. `catalog.Profile.Diagnostics()`
reports conflicting definitions (warnings) and unresolved type references
(errors) without failing profile construction.

Diagnostic codes:

| Code | Severity | Meaning |
| --- | --- | --- |
| `unknown-element`, `unknown-attribute` | warning | Not modeled by the importer, not imported |
| `unexpected-attribute` | warning | An attribute not defined for this element kind, ignored |
| `missing-type` | warning | A data attribute with neither type nor type kind |
| `missing-attribute` | error | A required attribute is missing |
| `invalid-boolean`, `invalid-release`, `invalid-literal` | error | An attribute value has the wrong format |
| `duplicate-definition`, `duplicate-literal-value` | error | A name or literal value is used twice |
| `self-inheritance`, `self-dependency` | error | A class, enumeration or namespace refers to itself |
| `empty-literal-name` | info | An enumeration literal without a name |
| `not-modeled` | info | A ServiceNS or ApplicableServiceNS document |
| `not-processed` | info | A file or archive entry that is not an NSD document |
| `no-documentation` | info | No NSDoc in the selected language for a namespace |
| `duplicate-doc` | warning | An NSDoc text ID used twice |
| `conflicting-definition` | warning | A name defined by two namespaces of a profile |
| `unresolved-reference` | error | A type, class, FC or presence condition not defined in the profile |

---

## nsdctl exit codes

| Code | Meaning |
| --- | --- |
| 0 | Success |
| 1 | Usage error, unknown name, unresolvable path, refused overwrite |
| 2 | Source error: `ErrSourceUnavailable`, `ErrInvalidNSD`, `ErrUnsupportedSchema`, `ErrLimitExceeded`, `ErrUnsafeArchive` |
| 3 | Validation: `*ValidationError`, `*DependencyError`, invalid lock or registry |
| 4 | Differences: `diff --exit-code`, `generate --check`, `sources check --packages` (missing package, other release, digest), `schema --exit-code` |
| 5 | License policy refused `generate --publish` |
