# Presence evaluation

Package [`presence`](../presence) evaluates the presence conditions of the
model against an **instance**: the data objects of one logical node, or the
attributes and sub data objects of one data object, that a device or an SCL
type actually has. Resolution never does this; the evaluation is opt-in.

For each element of the class the result says what the condition requires in
this instance and whether the instance meets it:

| Field | Values |
| --- | --- |
| `Requirement` | `required`, `optional`, `forbidden`, `undetermined` |
| `Verdict` | `satisfied`, `violated`, `undetermined` |
| `Reason` | why, in words: the fact or element that decided it |
| `Missing` | for an undetermined requirement: the fact it needs, e.g. `ctlModel`, `fact MOcond(1)` |

The evaluator never assumes a default. When a fact is missing the requirement
is undetermined; the verdict is still decided when both outcomes agree: an
element under `MO…` that is present is satisfied whether or not the condition
holds, and an absent `OF…` element too.

## API

```go
p, _ := official.Default()
rep, err := presence.New(p).LogicalNode("CSWI", &presence.Instance{
	Present: []string{"Beh", "Pos"},
	Children: map[string]*presence.Instance{
		"Pos": {Present: []string{"stVal", "q", "t", "ctlModel"},
			Values: map[string]string{"ctlModel": "sbo-with-enhanced-security"}},
	},
})
for _, v := range rep.Violations() { fmt.Println(v) }
// Pos.stSeld: MOsbo: required, violated (ctlModel sbo-with-enhanced-security, …)
```

| Entry point | Evaluates |
| --- | --- |
| `presence.New(l).LogicalNode(class, inst)` | the data objects of a class, inherited ones included, and the described children against their CDCs |
| `Evaluator.CDC(name, fc, inst)` | the attributes and sub data objects of a CDC (variant by key, FC or `inst.Class`) |
| `Evaluator.ConstructedAttribute(name, inst)` | the members of a constructed type |
| `Evaluator.Check(inst)` | `inst.Class` as a class, CDC or constructed type |
| `Evaluator.Validate(class, inst)` | as `LogicalNode`; a `*model.PresenceError` (`ErrPresenceViolation`) when a condition is violated |
| `resolver.Resolver.Presence` / `ValidatePresence` | the same through a resolver, honouring `Resolver.In` |
| `presence.Evaluate(class, elems, inst)` | a hand-made element list, without a model |
| `presence.ValidateNamespace(ns)` | the conditions a namespace declares (see below) |
| `presence.ReadInstance(r)` | the JSON form of an instance |
| `presence.Conditions()`, `Describe(name)`, `Known(name)` | how each condition is treated |

`Report.All`, `Violations`, `Undetermined`, `OK` and `Err` walk the child
reports too.

## Instances

```json
{
  "class": "XCBR",
  "present": ["Beh", "Loc", "OpCnt", "Pos", "BlkOpn", "BlkCls", "CBOpCap", "Ind1", "Ind2"],
  "context": {"ldNs": "IEC 61850-7-4:2007B", "rootLD": false, "facts": {"MOcond(1)": true}},
  "children": {
    "Pos": {"present": ["stVal", "q", "t", "ctlModel"],
            "values": {"ctlModel": "sbo-with-normal-security"}}
  }
}
```

- `present` uses instance names: numbered instances of multi-instance data
  objects (`Ind1`), device names of per-FC attribute copies (`setMag` for
  `_setMag`). A dotted entry (`mag.i`) records a member of a present
  attribute. A name that matches no element is listed in `Report.Unknown`.
- `values` holds the attribute values conditions read: `ctlModel` (name or
  number) and `hvRef`. An attribute with a value, and a child with an entry in
  `children`, counts as present.
- `children` describe present children further; they inherit the context.
- `context`: `lnClass` (set by the logical node evaluation), `rootLD`, `ldNs`,
  `lnNs` (defaults to the namespace of the class), `dataNs` (set per data
  object from its declaring class), `ctlModel`, and `facts`.
- `facts` state whether a condition holds, keyed by the condition as printed
  (`MOcond(1)`, `MFsubst`) or by element and condition (`Mir:MOcond(1)`),
  which wins. A fact overrides what the evaluator would derive.

`ReadInstance` rejects unknown fields, trailing data, null children, more than
32 levels and more than 4 MiB or 100,000 names (`ErrInvalidInstance`, plus
`ErrLimitExceeded` for the limits).

## Conditions

The names are those of the 7-2 NSD; [model.md](model.md#presence-conditions)
maps them to IEC 61850-7-3:2010 Table 1. Every condition name of the embedded
catalog has a rule (a test enforces it).

| Family | Conditions | Decided by |
| --- | --- | --- |
| Unconditional | `M`, `O`, `F`, `na` | nothing; `na` (data set presence) is optional |
| Multi-instance | `Mmulti`, `Omulti`, `MmultiRange(min, max)`, `OmultiRange(min, max)` | required/optional; numbered instances must lie in the range and use one number of digits per data object (7-2 §22.2) |
| Sibling | `MF`, `MO`, `OF`, `OM`, `FM`, `MmultiF` | whether the element the argument names is present: mandatory/forbidden, mandatory/optional, optional/forbidden, optional/mandatory, forbidden/mandatory |
| Group | `AtLeastOne(n)` | one or more members of group n |
| | `AtMostOne` | no more than one of the elements with this condition |
| | `AllOrNonePerGroup(n)` | every member of group n once one is present |
| | `AllOnlyOneGroup(n)` | exactly one group, complete; members of the other groups forbidden |
| | `AllAtLeastOneGroup(n)` | every group all or none, and at least one group complete (BCR: actual value with its time stamp, frozen value with its settings) |
| | `MAllOrNonePerGroup(n)` | all or none; the group is required when the data object has status information: ctlModel `status-only`, or the fact |
| Context | `MOln0`, `MFln0` | the logical node class is LLN0 |
| | `MOrootLD` | `rootLD` |
| | `MOlnNs` | `lnNs` differs from `ldNs` |
| | `MOdataNs` | `dataNs` differs from `lnNs` (else `ldNs`) |
| | `MOcdcNs` | the CDC's namespace differs from `dataNs` (else `lnNs`, `ldNs`) |
| | `MOsbo`, `MOenhanced` | ctlModel is an SBO model / an enhanced-security model |
| | `MOrms` | `hvRef` is `rms` |
| | `MFsubst` | another substitution attribute is present, or the fact |
| | `MFscaledAV`, `MFscaledMagV`, `MFscaledAngV` | an AnalogueValue attribute (Vector `mag`, `ang`) has its integer member `i`; members only `f` → forbidden |
| Fact | `MOcond(id)`, `MFcond(id)`, `OFcond(id)` | the condition the class text identifies by `id` (its text is `PresenceCondition.ArgsText` when an NSDoc or doc set is loaded) |
| | `MOoperTm`, `MONamPlt`, `MORange`, `OMSynPh`, `MFsg`, `MFnsg` | time-activated operation, a name plate that differs from the device's, range supervision, synchrophasors, setting-group membership |

Namespaces compare by standard, version and revision: `ldNs` "IEC
61850-7-4:2007B" matches every 2007B release.

## Validating a namespace

`presence.ValidateNamespace` reports, as warnings: a condition without a rule
(`presence-unknown-condition`), a missing argument (`presence-missing-argument`),
a malformed group number or range (`presence-invalid-argument`), an argument on
a condition that takes none (`presence-unexpected-argument`) and a sibling the
class, CDC or type does not declare (`presence-unknown-sibling`; siblings
inherited from another namespace are not checked). In the embedded catalog
it reports one finding: IEC 61850-7-410 AEMC.VolFunRef is `Mmulti(VolSpt)`,
an `Mmulti` with a sibling argument, evaluated as `Mmulti`.

## nsdctl

```console
$ nsdctl model check CSWI --present Beh,Pos --value Pos.ctlModel=sbo-with-enhanced-security
$ nsdctl model check --instance xcbr.json
$ nsdctl model check --scl station.scd --ln CTRL/Q0XCBR1 --root-ld false
$ nsdctl model check --scl device.icd --ln-type T_XCBR --fact MFsubst=false
$ nsdctl validate --presence .nsd-cache
```

`model check` lists the violated and undetermined elements (`--all`: every
element) and exits with 1 when a condition is violated. With `--scl` it reads
the data type templates (DO, SDO, DA, BDA, the `Val` of ctlModel and hvRef)
and, for `--ln LD/LN`, the logical node's lnType, its DOI ctlModel/hvRef values
and the ldNs and lnNs of the name plates; see
[cli.md](../cmd/nsdctl/docs/cli.md).
