# References

## Forms

| Form | Example | Type |
| --- | --- | --- |
| Instance reference | `LD0/MMXU1.Hz.mag.f[MX]` | `reference.Ref` |
| Class reference | `MMXU.Hz.mag.f[MX]` | `reference.ClassRef` |
| MMS form | domain `LD0`, item `MMXU1$MX$Hz$mag$f` | `reference.FromMMS`, `Ref.ToMMS` |

`reference.Parse` follows the format and rules of `go-iec61850`'s `ParseRef`
exactly: `LD/LN.DO.DA[FC]`, LD-only and LN-only forms, logical device names up
to 64 characters, two-character FCs (unknown values accepted), no separators
(`/ . $ [ ]`) inside components. `ParseStrict` also enforces 129 characters.
The test suite runs go-iec61850's vectors against this parser.

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

Failures are typed: `ErrUnknownLogicalNode`, `ErrUnknownDataObject`,
`ErrUnknownCDC`, `ErrUnknownAttribute` (including an FC mismatch, with
`Expected` and `Actual` set), `ErrUnknownType`, `ErrCycle` and
`ErrAmbiguousReference`.
