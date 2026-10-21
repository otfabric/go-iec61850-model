# Ontology helpers

Package `ontology` turns a profile into a small, typed, deterministic concept
graph with JSON export. It has no RDF, OWL or CIM dependency.

```go
g, err := ontology.BuildGraph(profile, ontology.Options{Semantics: semantic.Default})
rels := g.Relations(ontology.LogicalNodeID("MMXU"), ontology.Contains)
volts := g.Incoming(ontology.QuantityID(semantic.QuantityVoltage), ontology.Represents)
err = g.ExportJSON(w)
```

## Concepts

| Kind | ID form | Example |
| --- | --- | --- |
| namespace | `ns:<id>` | `ns:IEC 61850-7-4:2007B5` |
| logical-node-class | `ln:<class>` | `ln:MMXU` |
| data-object | `do:<class>.<do>` | `do:MMXU.Hz` |
| cdc | `cdc:<name>` | `cdc:WYE` |
| sub-data-object | `sdo:<cdc>.<name>` | `sdo:WYE.phsA` |
| data-attribute | `da:<cdc or type>.<name>` | `da:MV.mag`, `da:AnalogueValue.f` |
| constructed-attribute | `ca:<name>` | `ca:Vector` |
| enumeration | `enum:<name>` | `enum:BehaviourModeKind` |
| basic-type | `basic:<name>` | `basic:FLOAT32` |
| measurement | `measurement:<class>.<do>[.<sdo>]` | `measurement:MMXU.PNV.phsA` |
| quantity | `quantity:<q>` | `quantity:voltage` |
| phase | `phase:<p>` | `phase:A` |
| reference | `reference:<r>` | `reference:line-neutral` |

Concepts carry a label where one is known (NSDoc titles, quantity labels) and
properties such as presence, FC, logical node group, and for measurements the
rule, source, confidence, basis and unit hint.

## Relations

| Predicate | From → To |
| --- | --- |
| `contains` | class → data object; CDC → sub data object / data attribute; constructed type → member |
| `uses` | data object / sub data object → CDC |
| `hasType` | attribute → basic / constructed / enumerated type; data object → underlying type |
| `inheritsFrom` | class → base class; enumeration → parent enumeration |
| `definedIn` | definition → namespace |
| `dependsOn` | namespace → namespace |
| `represents` | measurement → quantity |
| `hasPhase`, `hasReference` | measurement → phase / reference |
| `measuredBy` | measurement → data object or sub data object |

Inherited data objects are linked where they are declared (`do:DomainLN.Beh`),
and a measurement of an inherited data object is measured by that concept.
Relations to definitions missing from the profile are left out.

## Determinism

`Concepts`, `Relations`, `Incoming`, `FindConcepts`, `Walk` (breadth-first)
and `ExportJSON` are ordered; identical input gives byte-identical output.

## Adapter seam

Measurement, quantity, phase and reference concepts are protocol-neutral and
are meant to come from a future shared OTFabric ontology module. When it
exists, this package will link IEC 61850 concepts to it instead of defining
them; the IDs of those kinds are chosen to remain stable.

## Licensing

A graph built from official packages contains IEC identifiers and, when NSDoc
was loaded, IEC texts as labels. Exports of it are local-only, like any other
IEC-derived output; see [licensing.md](licensing.md).
