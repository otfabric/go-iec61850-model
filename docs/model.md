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

Data objects with a multi-instance presence condition (`Mmulti`, `Omulti`,
`MmultiRange`, `OmultiRange`, `MmultiF`) appear on devices with an instance
number: `Ind` becomes `Ind1`, `Ind2`. The resolver accepts the numbered name
when the class has no data object with that exact name and records the number
in `Step.Instance`. For `MmultiRange`/`OmultiRange` the argument `"min, max"`
bounds the number: `TmAChr` is `OmultiRange(33, 48)`, so `TmAChr33` resolves
and `TmAChr32` fails with `ErrUnknownDataObject`. The bare name (`TmAChr`)
resolves too, because the Data-Instance-ID of IEC 61850-7-2:2010 §22.2 is
optional; the rule that all instances of a class use the same number of digits
needs the whole logical node and is not checked.

## Presence conditions

`PresenceCondition` keeps the NSD name and argument verbatim; `IsMandatory` and
`IsOptional` recognize only `M` and `O`. The library has no evaluator for the
conditional forms. The Edition 2.1 NSD (2007B) uses the condition names of the
7-2 NSD, which replace the names of IEC 61850-7-3:2010 Table 1. The main
correspondences:

| IEC 61850-7-3:2010 Table 1 | NSD (7-2 PresenceConditions) |
| --- | --- |
| `PICS_SUBST` | `MFsubst` |
| `GC_1` | `AtLeastOne` |
| `GC_2_n` | `AllOrNonePerGroup` |
| `GC_2_XOR_n` | `AllOnlyOneGroup` |
| `GC_1_EXCL` | `AtMostOne` |
| `AC_LN0_M` | `MOln0` |
| `AC_LN0_EX` | `MFln0` |
| `AC_DLD_M` | `MOlnNs` |
| `AC_DLN_M` | `MOdataNs` |
| `AC_SCAV` | `MFscaledAV` |
| `AC_RMS_M` | `MOrms` |
| `AC_CO_SBO` | `MOsbo` |
| `AC_ST` | `MAllOrNonePerGroup` |

The table is a reading aid, not a normative mapping; the NSD is authoritative.

## Edition 2 (2010) and Edition 2.1 (NSD 2007B6)

The embedded 2007B6 namespaces are Edition 2.1. Compared with the tables of
IEC 61850-7-4:2010 and 7-3:2010 (Edition 2), a cross-check of 20 LN classes and
19 CDCs found no CDC-type or FC difference on shared elements; the differences
are edition deltas:

- **Inherited data objects.** Abstract base classes (DomainLN, StatisticsLN,
  ...) add NamPlt, Beh, Health, Mir, Mod, InRef, the Clc* family, InSyn, Blk and
  BlkRef, which the 2010 tables list only through "Common LN". New data objects
  include XCBR/XSWI `DscDlTmms` and `POWOpnTmms`, PTOC/PDIS/PTRC `FltA`,
  `FltPhV` and `FltValTyp`, RREC `Rec1PhCnt`/`Rec3PhCnt`, TCTR `HoldTmms`,
  `Clip`, `AccMeas`, `AccPro`, `ScndTmms`, `DynRng*`, and MMXU `HzRte`.
- **Renames.** ZBAT `LoBatVal`→`LoBatVol`, `HiBatVal`→`HiBatVol`; ATCC
  `ParTraMod`→`ParTrfMod`; YPTR `MaxPwr`→`MaxVA`, `PwrRtg`→`VARtg`,
  `OvlTEmg`→`OvlTmEmg`, `*Spt`→`*Set`; GGIO `CntRs1`→`CntVal`. XSWI `MaxOpCap`
  is removed. Multi-instance names lose the trailing "1" of Edition 2
  (`RxSrc1`→`RxSrc`, `TmAChr33`→`TmAChr` with `OmultiRange(33, 48)`).
- **Presence.** ZBAT.Vol M→O; ATCC.CtlV and ParOp M→O; TCTR.ARtg and HzRtg
  O→M; CSWI and GGIO LocKey/LocSta O→OF; C/C1 group conditions became
  `AtLeastOne` (MSQI, PTRC, SIMG, ATCC).
- **CDCs.** `cdcNs` is removed (`cdcName` remains, now O); `dbRef`, `zeroDbRef`
  and `dbAngRef` are new in MV, CMV, APC and BAC; INC.stVal gains `dupd`;
  DPC/DPS stVal is typed `Dbpos`; ASG.setMag is `AnalogueValueCtl`. Type
  names: CtlModels→CtlModelKind, SboClasses→SboClassKind, `VISIBLE
  STRING255`→VisString255, TimeStamp→Timestamp.
- **Transient flag.** LPHD PwrUp/PwrDn, PTRC.Op, RREC.OpCls and ZBAT.BatTest
  are transient in the 2010 tables but not in the NSD. Not yet verified
  against the Edition 2.1 text.

Use the `edition2` profile (2007A3) for devices that declare Edition 2.

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
