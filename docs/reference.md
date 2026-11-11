# References

## Forms

| Form | Example | Type |
| --- | --- | --- |
| Instance reference | `LD0/MMXU1.Hz.mag.f[MX]` | `reference.Ref` |
| Class reference | `MMXU.Hz.mag.f[MX]` | `reference.ClassRef` |
| MMS form | domain `LD0`, item `MMXU1$MX$Hz$mag$f` | `reference.FromMMS`, `Ref.ToMMS` |

`reference.Parse` follows the format and rules of `go-iec61850`'s `ParseRef`:
`LD/LN.DO.DA[FC]`, LD-only and LN-only forms, logical device names up to 64
characters (checked by `Validate`, so `FromMMS` and literal `Ref` values are
covered too), FCs of two upper-case letters (unknown values accepted, `[mx]`
rejected), no separators (`/ . $ [ ]`) inside components. The test suite runs
go-iec61850's vectors against this parser.

| Rule | Source | Checked by |
| --- | --- | --- |
| ObjectReference ≤ 129, LD ≤ 64, path below the LD ≤ 64; the `[FC]` suffix is not counted | IEC 61850-7-2:2010 Table 4, §22.2 NOTE 1 | `ParseStrict` |
| Array element `name(n)`, e.g. `phsAHar(3)` | 7-2:2010 §6.1.2.3, §12.3.3.2.3 | `Parse`, `Validate`, `SplitIndex`; the resolver accepts an index only on an array element |
| Character set A-Z a-z 0-9 `_`; LD name and LN prefix start alpha; prefix+instance ≤ 12; DataObjectName ≤ 12, upper-case start; LN.DO.DA ≤ 61 | 7-2:2010 §22.2 | `Ref.ValidateNames` (opt-in; `Parse` stays lenient for devices) |
| MMS domain and item ID ≤ 64 (maxIdentifier) | IEC 61850-8-1:2011 §8.1.3.2.2, §7.3.1 | `ToMMS`, `FromMMS` (`ErrLimitExceeded`) |

`ToMMS` rejects a reference with an array index: 8-1 addresses array elements
with MMS alternate access, not with the item ID. The bounds of an index
(`sizeAttribute`, `maxPts`) are device configuration and are not checked.

### Functional constraints

`model.FunctionalConstraint.IsACSI` is the set of IEC 61850-7-2:2010 Table 20
(ST MX SP SV CF DC SG SE SR OR BL EX), which the 7-2 NSD defines too.
`IsSCSM` is the set IEC 61850-8-1:2011 Figure 4 adds for the MMS mapping
(CO RP LG BR GO GS MS US). `IsStandard` is either. `FCXX` is the service-only
wildcard of Table 20; 8-1 does not map it, and it is not standard.

`reference.Normalize` accepts any of the three forms and returns the canonical
string. It changes notation only: nothing is trimmed, case-folded or inferred.

## Logical node instance names

`reference.ParseLNName("Q0MMXU1")` returns prefix `Q0`, class `MMXU`, instance
`1`: the instance is the trailing digits, the class the four upper-case letters
before them, the rest the prefix. `LLN0` is a class on its own.
`Ref.Class()` converts an instance reference into a class reference this way.

## Three levels of validity

1. **Syntax** — `Parse`, `ParseClass`, `Validate`. No model needed.
2. **Model** — `resolver.New(profile).Resolve(ref)` checks that the class,
   data objects and attributes exist in the selected namespaces, with the
   requested FC, and returns the resolved type with provenance.
3. **Device** — whether a device implements an optional data object is not in
   the catalog. Use SCL or discovery.

## Resolution

```go
r := resolver.New(profile)

res, err := r.ResolveString("LD0/MMXU1.PNV.phsA.cVal.mag.f[MX]")
res.DataObjectPath() // "PNV.phsA"
res.AttributePath()  // "cVal.mag.f"
res.Type()           // "FLOAT32"
res.FC()             // "MX"
for _, s := range res.Steps {
	// s.Kind, s.Name, s.Type, s.Presence, s.DeclaredIn, s.Source
}

leaves, err := r.Leaves("MMXU", []string{"Hz"}, model.FCMX) // every MX leaf below MMXU.Hz
```

Input without `/` is a class reference. An instance name there
(`MMXU1.Hz`) fails with `ErrUnknownLogicalNode` and a hint that an instance
reference needs its logical device (`LD0/MMXU1.Hz`).

### Control services (FC CO)

IEC 61850-8-1:2011 §20.3 maps the control service parameters within the
controllable data object, with FC CO: `SBO` (VISIBLE STRING129, Table 83) and
the structures `SBOw`, `Oper` and `Cancel` (Tables 85, 90, 88). The resolver
builds them for every CDC whose NSD declares service parameters (SPC, DPC, INC,
ENC, BSC, ISC, APC, BAC):

| Member | Type | Source |
| --- | --- | --- |
| `ctlVal` | the CDC's service parameter; the data object's type for ENC | 7-3 "parameters for control services", NSD `ServiceParameter` |
| `operTm` | `Timestamp` (optional: TimeActivatedOperate only) | 8-1 Table 80, Table 90 NOTE 2 |
| `origin` | `Originator` | 8-1 Table 80 |
| `ctlNum` | `INT8U` | 8-1 Table 80 |
| `T` | `Timestamp` | 8-1 Table 80 |
| `Test` | `BOOLEAN` | 8-1 Table 80 |
| `Check` | `Check` (not in `Cancel`) | 8-1 Table 80, Table 88 |

```go
r.ResolveString("LD0/XCBR1.Pos.Oper.ctlVal[CO]")      // BOOLEAN
r.ResolveString("LD0/LLN0.Mod.Oper.ctlVal[CO]")       // BehaviourModeKind (parameterized)
r.ResolveString("LD0/XCBR1.Pos.Oper.origin.orCat")    // OriginatorCategoryKind
r.Leaves("XCBR", []string{"Pos"}, model.FCCO)         // SBO, SBOw.*, Oper.*, Cancel.*
```

The steps have the kinds `ControlService` and `ServiceParameter`, and FC CO.
`Pos[CO]` resolves; `Pos.Oper.ctlVal[ST]` fails; `Pos.ctlVal` fails with a hint
to `Oper.ctlVal[CO]`. Which of the services a device has depends on its
`ctlModel` (8-1 Table 81); the model lists all four, `Oper` as mandatory and the
others as optional. `Leaves` without an FC includes them after the data
attributes.

Failures are typed: `ErrUnknownLogicalNode`, `ErrUnknownDataObject`,
`ErrUnknownCDC`, `ErrUnknownAttribute` (including an FC mismatch, with
`Expected` and `Actual` set), `ErrUnknownType`, `ErrCycle` and
`ErrAmbiguousReference`.
