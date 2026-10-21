# The model

Package `model` holds the normalized form of NSD definitions. All slices keep
document order.

| IEC 61850 concept | Type | Notes |
| --- | --- | --- |
| Namespace | `Namespace`, `NamespaceID` | ID = standard, version, revision, release; dependencies; provenance |
| Logical node class | `LogicalNodeClass` | `Abstract` for abstract classes; `Base` names the parent class; `Group()` from the first letter |
| Data object | `DataObject` | `Type` is the CDC; `UnderlyingType` parameterizes CDCs such as ENS; presence; array attributes |
| Common data class | `CommonDataClass` | `SubDataObjects`, `DataAttributes`, `ServiceParameters`; `Position` orders members across kinds |
| Data attribute | `DataAttribute` | `Type`, `TypeKind`, `FC`, presence, trigger options (dchg, qchg, dupd) |
| Constructed attribute | `ConstructedAttribute` | Members are `DataAttribute` values without FC; they take the FC of the enclosing attribute |
| Enumeration | `Enumeration`, `Literal` | `InheritedFrom` for extended enumerations |
| Basic type | `BasicType` | BOOLEAN, INT32, FLOAT32, Timestamp, Quality, ... as the namespace defines them |
| Functional constraint | `FunctionalConstraint`, `FunctionalConstraintDef` | String-backed, same values as go-iec61850 |
| Presence condition | `PresenceCondition`, `PresenceConditionDef` | Name plus verbatim arguments, e.g. `AtLeastOne(1)` |
| Documentation | `Doc` | `TitleID`/`DescID`; texts filled from NSDoc when loaded |

## Inheritance

A class lists only the data objects it declares. `catalog.Profile.DataObjects`
returns the flattened list: root class first, then each subclass, with
redeclared data objects replacing inherited ones in place. Every entry says
which class declares it.

## Type kinds and parameterized CDCs

`TypeKind` is `BASIC`, `ENUMERATED`, `CONSTRUCTED`, `PACKED_LIST` or
`undefined`. Some CDCs leave the type of an attribute open (ENS `stVal`); the
data object supplies it through `UnderlyingType`. The resolver applies it and
marks the step `Parameterized`. When an attribute declares no kind, the
resolver infers it from the profile and fails if the name is defined as more
than one kind.

## Multi-instance data objects

Data objects with a multi-instance presence condition (`Omulti`, `Mmulti`, ...)
appear on devices with an instance number: `Ind` becomes `Ind1`, `Ind2`. The
resolver accepts the numbered name when the class has no data object with that
exact name and records the number in `Step.Instance`.

## Provenance and diagnostics

Every top-level definition has a `Source` namespace ID. `Namespace.Provenance`
records the input name and SHA-256 digests. `Diagnostic` values locate findings
by source and XML path, e.g. `/NS/LNClasses/LNClass[MMXU]/DataObject[Hz]`.

## Official-package specifics

- **CDC variants.** `CommonDataClass.Variant` (`Key()` gives `SPG/SG`). Lookups
  by name return the CDC (first variant); `CDCVariants` returns all. The resolver
  selects the variant by FC; without an FC, an attribute whose FC differs between
  variants resolves with an empty FC and lists the alternatives in
  `Step.Variants`.
- **Per-FC copies.** `DataAttribute.BaseName()` strips leading underscores
  (`__setMag` → `setMag`); the resolver accepts the device name and selects the
  copy by FC; leaf expansion reports device names.
- **Extension classes.** `LogicalNodeClass.IsExtension`; `Extensions(name)` lists
  them, `DataObjects`/`DataObject` include their data objects, and `LogicalNode`
  returns the base definition.
- **Namespace metadata.** `Copyright` (notice, license), `Changes`, `NSDVersion`,
  `NamespaceType`, `AppVersion`, `DependencyTypes`; `FunctionalConstraintDef`
  carries `Services` and `DataSetMemberOf`; `DataAttribute` carries `MinValue`,
  `MaxValue`, `MinIndex`.
- **Service namespaces.** ServiceNS documents (IEC 61850-8-1) are recognized;
  `nsd.Package.ServiceNamespaces()` lists their IDs. Their content is not modeled
  yet.
