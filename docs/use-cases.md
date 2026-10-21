# nsdctl use cases

Real questions from integrating IEC 61850 devices, answered with `nsdctl` and the
official IEC models embedded in it. Every output below is real, produced by
`nsdctl` from the official packages (cut where marked with `…`). No download
or setup is needed: without `--source`, nsdctl uses the embedded default
profile (IEC 61850-7-2, 7-3, 7-4, 7-410 and 7-420).

The models contain identifiers, types and structure, not the descriptive text
of the standards; the semantics column comes from OTFabric's explicit mapping
rules (see [semantic.md](semantic.md)).

- [1. What can I read from a power meter?](#1-what-can-i-read-from-a-power-meter)
- [2. What does this tag from an IED or SCADA export mean?](#2-what-does-this-tag-from-an-ied-or-scada-export-mean)
- [3. Decode raw values: units, multipliers, modes](#3-decode-raw-values-units-multipliers-modes)
- [4. Integrate a battery energy storage system](#4-integrate-a-battery-energy-storage-system)
- [5. Monitor a circuit breaker](#5-monitor-a-circuit-breaker)
- [6. Read a protection setting](#6-read-a-protection-setting)
- [7. Find where a type of data lives](#7-find-where-a-type-of-data-lives)
- [8. A device speaks the older storage model (IEC TR 61850-90-9)](#8-a-device-speaks-the-older-storage-model-iec-tr-61850-90-9)
- [9. Build a tag list or a UI from JSON](#9-build-a-tag-list-or-a-ui-from-json)
- [10. Use the same model from Go](#10-use-the-same-model-from-go)

## 1. What can I read from a power meter?

A meter or a protection relay publishes its measurements in the logical node
class `MMXU`. `model measurements` lists what each data object means, per
phase, with the unit you can expect:

```console
$ nsdctl model measurements MMXU
PATH            QUANTITY        PHASE     REFERENCE     DIRECTION  UNIT  RULE
MMXU.TotW       active-power    total     -             -          W     mmxu.totw
MMXU.TotVAr     reactive-power  total     -             -          VAr   mmxu.totvar
MMXU.TotVA      apparent-power  total     -             -          VA    mmxu.totva
MMXU.TotPF      power-factor    total     -             -          1     mmxu.totpf
MMXU.Hz         frequency       -         -             -          Hz    mmxu.hz
MMXU.PPV.phsAB  voltage         AB        line-line     -          V     mmxu.ppv
MMXU.PPV.phsBC  voltage         BC        line-line     -          V     mmxu.ppv
MMXU.PPV.phsCA  voltage         CA        line-line     -          V     mmxu.ppv
MMXU.PNV.phsA   voltage         A         line-neutral  -          V     mmxu.pnv
MMXU.PNV.phsB   voltage         B         line-neutral  -          V     mmxu.pnv
MMXU.PNV.phsC   voltage         C         line-neutral  -          V     mmxu.pnv
MMXU.PNV.neut   voltage         N         line-neutral  -          V     mmxu.pnv
MMXU.PNV.net    voltage         net       line-neutral  -          V     mmxu.pnv
MMXU.PNV.res    voltage         residual  line-neutral  -          V     mmxu.pnv
MMXU.PhV.phsA   voltage         A         line-ground   -          V     mmxu.phv
MMXU.PhV.phsB   voltage         B         line-ground   -          V     mmxu.phv
MMXU.PhV.phsC   voltage         C         line-ground   -          V     mmxu.phv
MMXU.PhV.neut   voltage         N         line-ground   -          V     mmxu.phv
MMXU.PhV.net    voltage         net       line-ground   -          V     mmxu.phv
MMXU.PhV.res    voltage         residual  line-ground   -          V     mmxu.phv
MMXU.A.phsA     current         A         -             -          A     mmxu.a
…
```

`model show MMXU` lists all 64 data objects, including those inherited from
the abstract base classes and those IEC 61850-7-420 adds (`PFExtQud`, `PQSt`).
To see the attributes a client actually reads for one value, expand it to its
leaves and keep the measured values (`--fc MX`):

```console
$ nsdctl model leaves MMXU.TotW --fc MX
PATH                 TYPE       KIND        FC
MMXU.TotW.instMag.i  INT32      BASIC       MX
MMXU.TotW.instMag.f  FLOAT32    BASIC       MX
MMXU.TotW.mag.i      INT32      BASIC       MX
MMXU.TotW.mag.f      FLOAT32    BASIC       MX
MMXU.TotW.range      RangeKind  ENUMERATED  MX
MMXU.TotW.q          Quality    BASIC       MX
MMXU.TotW.t          Timestamp  BASIC       MX
```

So total active power is `MMXU1.TotW.mag.f` (FLOAT32), with quality `q` and
timestamp `t` next to it. Without `--fc`, `leaves` also lists the
configuration, substitution and description attributes — several thousand
lines for MMXU.

## 2. What does this tag from an IED or SCADA export mean?

Given a reference such as `LD0/MMXU1.PhV.phsA.cVal.mag.f[MX]`, `model describe`
says what it measures:

```console
$ nsdctl model describe 'LD0/MMXU1.PhV.phsA.cVal.mag.f[MX]'
MMXU.PhV.phsA.cVal.mag.f
quantity     voltage
phase        A
reference    line-ground
component    magnitude
unit hint    V
mapping      rule mmxu.phv, rule, high confidence (otfabric-default)
```

`model show` resolves the same reference step by step, with the type of every
element and the namespace that defines it — useful to check a reference
before configuring a client, or to explain a value in a UI:

```console
$ nsdctl model show 'LD0/MMXU1.PhV.phsA.cVal.mag.f[MX]'
MMXU.PhV.phsA.cVal.mag.f → FLOAT32 (BASIC) [MX]

STEP              NAME  TYPE           FC  PRESENCE       DECLARED IN    NAMESPACE
LogicalNode       MMXU  MMXU           -   -              MMXU           IEC 61850-7-4:2007B6
DataObject        PhV   WYE            -   O              MMXU           IEC 61850-7-4:2007B6
SubDataObject     phsA  CMV            -   AtLeastOne(1)  WYE            IEC 61850-7-3:2007B6
DataAttribute     cVal  Vector         MX  M              CMV            IEC 61850-7-3:2007B6
SubDataAttribute  mag   AnalogueValue  MX  M              Vector         IEC 61850-7-3:2007B6
SubDataAttribute  f     FLOAT32        MX  AtLeastOne(1)  AnalogueValue  IEC 61850-7-3:2007B6

quantity     voltage
phase        A
reference    line-ground
component    magnitude
unit hint    V
mapping      rule mmxu.phv, rule, high confidence (otfabric-default)
```

A reference that the model does not define fails with exit code 1 and says
which element is wrong:

```console
$ nsdctl model show 'MMXU.PhV.phsD.cVal.mag.f'
nsdctl: iec61850-model: cannot resolve "MMXU.PhV.phsD": CDC WYE has no member phsD [IEC 61850-7-3:2007B6]
```

## 3. Decode raw values: units, multipliers, modes

Devices report units as numbers: `MMXU1.TotW.units.SIUnit = 38` and
`multiplier = 3`. The enumerations of the model decode them:

```console
$ nsdctl model show SIUnitKind | grep -wE 'VALUE|5|29|33|38|63'
VALUE  LITERAL
5      A
29     V
33     Hz
38     W
63     VAr
```

```console
$ nsdctl model show MultiplierKind | grep -wE 'VALUE|-3|0|3|6'
enumeration MultiplierKind (IEC 61850-7-3:2007B6)
VALUE  LITERAL
-6     µ
-3     m
0      -
3      k
6      M
```

38 with multiplier 3 is kW. The same works for status values; `Beh.stVal = 5`
means the function is off:

```console
$ nsdctl model show BehaviourModeKind
enumeration BehaviourModeKind (IEC 61850-7-4:2007B6)
VALUE  LITERAL
1      on
2      blocked
3      test
4      test/blocked
5      off
```

## 4. Integrate a battery energy storage system

IEC 61850-7-420 models a battery in `DBAT` and the storage system as a DER in
`DSTO`. Which battery data objects exist, and which are mandatory?

```console
$ nsdctl model show DBAT | grep -E 'class|inheritance|^DATA| M  |^Soc|^Avl|^Watt|^Amp'
logical node class DBAT (IEC 61850-7-420:2019A4)
inheritance  DBAT → StorageUnitLN → ControlEquipmentInterfaceLN → ControlledLN → ControllingLN → FunctionLN → StatisticsLN → DomainLN
DATA OBJECT   CDC  PRESENCE   DECLARED IN                  NAMESPACE               SEMANTICS
Beh           ENS  M          DomainLN                     IEC 61850-7-4:2007B6    -
SocPro        SPS  O          DBAT                         IEC 61850-7-420:2019A4  -
BatSt         SPS  M          DBAT                         IEC 61850-7-420:2019A4  -
Amp           MV   O          DBAT                         IEC 61850-7-420:2019A4  -
Watt          MV   O          DBAT                         IEC 61850-7-420:2019A4  -
AvlChaAhr     MV   O          DBAT                         IEC 61850-7-420:2019A4  -
AvlDschAhr    MV   O          DBAT                         IEC 61850-7-420:2019A4  -
SocEffAhrPct  MV   O          DBAT                         IEC 61850-7-420:2019A4  -
SocDvPct      MV   O          DBAT                         IEC 61850-7-420:2019A4  -
BatTyp        ENG  M          DBAT                         IEC 61850-7-420:2019A4  -
```

Search by name when you know part of it, e.g. state of charge (`Soc`):

```console
$ nsdctl model search --name Soc --kind do
KIND  PATH               TYPE  FC  PRESENCE  NAMESPACE
do    DBAT.CycSocRngPct  MV    -   O         IEC 61850-7-420:2019A4
do    DBAT.SocDvPct      MV    -   O         IEC 61850-7-420:2019A4
do    DBAT.SocEffAhrPct  MV    -   O         IEC 61850-7-420:2019A4
do    DBAT.SocPro        SPS   -   O         IEC 61850-7-420:2019A4
do    DTCD.SocDate       TCS   -   O         IEC 61850-7-420:2019A4
do    DTCD.SocUseTgtPct  ING   -   M         IEC 61850-7-420:2019A4
```

The power limits to configure for charging and discharging live in `DSTO`,
inherited from the storage base classes:

```console
$ nsdctl model show DSTO | grep -E '^(ChaWMax|DschWMax|WhRtg|EffWh|EffWhPct) '
EffWh         MV   AtLeastOne(1)  StorageOperationalSettingsLN  IEC 61850-7-420:2019A4  -
EffWhPct      MV   AtLeastOne(1)  StorageOperationalSettingsLN  IEC 61850-7-420:2019A4  -
ChaWMax       ASG  M              StorageOperationalSettingsLN  IEC 61850-7-420:2019A4  -
DschWMax      ASG  O              StorageOperationalSettingsLN  IEC 61850-7-420:2019A4  -
WhRtg         ASG  O              StorageNameplateRatingsLN     IEC 61850-7-420:2019A4  -
```

## 5. Monitor a circuit breaker

`XCBR` is a circuit breaker. Its position, operation counter and blocking
commands come from the switching equipment base class:

```console
$ nsdctl model show XCBR | grep -E 'class|inheritance|^DATA|^(Loc|OpCnt|Pos|BlkOpn|BlkCls|CBOpCap) '
logical node class XCBR (IEC 61850-7-4:2007B6)
inheritance  XCBR → SwitchingEquipmentLN → EquipmentInterfaceLN → FunctionLN → StatisticsLN → DomainLN
DATA OBJECT  CDC  PRESENCE   DECLARED IN           NAMESPACE             SEMANTICS
Loc          SPS  M          SwitchingEquipmentLN  IEC 61850-7-4:2007B6  -
OpCnt        INS  M          SwitchingEquipmentLN  IEC 61850-7-4:2007B6  -
Pos          DPC  M          SwitchingEquipmentLN  IEC 61850-7-4:2007B6  -
BlkOpn       SPC  M          SwitchingEquipmentLN  IEC 61850-7-4:2007B6  -
BlkCls       SPC  M          SwitchingEquipmentLN  IEC 61850-7-4:2007B6  -
CBOpCap      ENS  O          XCBR                  IEC 61850-7-4:2007B6  -
```

```console
$ nsdctl model show XCBR.Pos.stVal
XCBR.Pos.stVal → Dbpos (BASIC) [ST]

STEP           NAME   TYPE   FC  PRESENCE  DECLARED IN           NAMESPACE
LogicalNode    XCBR   XCBR   -   -         XCBR                  IEC 61850-7-4:2007B6
DataObject     Pos    DPC    -   M         SwitchingEquipmentLN  IEC 61850-7-4:2007B6
DataAttribute  stVal  Dbpos  ST  M         DPC                   IEC 61850-7-3:2007B6
```

`Dbpos` is the double-point position (intermediate, off, on, bad state).

## 6. Read a protection setting

The pickup value of an overcurrent function is `PTOC.StrVal`. Settings exist
per functional constraint — as a setpoint (SP), per setting group (SG) or
editable (SE) — and nsdctl says so when the reference leaves it open:

```console
$ nsdctl model show 'PTOC.StrVal.setMag.f'
…

The FC of setMag depends on the realization: ASG/SP.setMag, ASG/SG.setMag, ASG/SE.setMag.
Append [FC] to the reference to select one.
```

```console
$ nsdctl model show 'PTOC.StrVal.setMag.f[SG]'
PTOC.StrVal.setMag.f → FLOAT32 (BASIC) [SG]

STEP              NAME    TYPE              FC  PRESENCE            DECLARED IN       NAMESPACE
LogicalNode       PTOC    PTOC              -   -                   PTOC              IEC 61850-7-4:2007B6
DataObject        StrVal  ASG               -   O                   PTOC              IEC 61850-7-4:2007B6
DataAttribute     setMag  AnalogueValueCtl  SG  M                   ASG/SG            IEC 61850-7-3:2007B6
SubDataAttribute  f       FLOAT32           SG  AllOnlyOneGroup(2)  AnalogueValueCtl  IEC 61850-7-3:2007B6
```

## 7. Find where a type of data lives

Which data objects anywhere in the model are binary counters (CDC `BCR`)?

```console
$ nsdctl model search --cdc BCR --kind do
KIND  PATH           TYPE  FC  PRESENCE       NAMESPACE
do    FCNT.CntVal    BCR   -   M              IEC 61850-7-4:2007B6
do    GGIO.CntVal    BCR   -   Omulti         IEC 61850-7-4:2007B6
do    LCCH.RedRxCnt  BCR   -   OF(RedChLiv)   IEC 61850-7-4:2007B6
do    LCCH.RxCnt     BCR   -   O              IEC 61850-7-4:2007B6
do    LCCH.TxCnt     BCR   -   O              IEC 61850-7-4:2007B6
do    MFLW.VlmCnt    BCR   -   AtLeastOne(1)  IEC 61850-7-4:2007B6
do    MHYD.FishCnt   BCR   -   O              IEC 61850-7-4:2007B6
do    XCBR.SumSwARs  BCR   -   O              IEC 61850-7-4:2007B6
```

`model search` filters by name, CDC, attribute type, functional constraint,
logical node group and standard; `model list --kind enum` lists every
enumeration.

## 8. A device speaks the older storage model (IEC TR 61850-90-9)

Older storage devices use IEC TR 61850-90-9, which defines `DBAT`, `DSTO` and
`SBAT` differently from IEC 61850-7-420. nsdctl keeps both: load the extended
profile and choose the namespace.

```console
$ nsdctl -p extended model show DBAT
nsdctl: iec61850-model: ambiguous logical node class "DBAT": defined in (Tr)IEC 61850-90-9:2018A3, IEC 61850-7-420:2019A4; select one with --ns
```

```console
$ nsdctl -p extended model show DBAT --ns 90-9
logical node class DBAT ((Tr)IEC 61850-90-9:2018A3)
group        D Distributed energy resources
inheritance  DBAT → StorageUnitLN → ControlEquipmentInterfaceLN → ControlledLN → ControllingLN → FunctionLN → StatisticsLN → DomainLN
…
```

What changed between the two models?

```console
$ nsdctl catalog overlaps --left IEC_61850-7-420 --right IEC_TR_61850-90-9 --class conflicting
KIND  NAME             LEFT                    RIGHT                      CLASS        DETAILS
ln    DBAT             IEC 61850-7-420:2019A4  (Tr)IEC 61850-90-9:2018A3  conflicting  6 only left, 1 only right
ln    DER_StorageLN    IEC 61850-7-420:2019A4  (Tr)IEC 61850-90-9:2018A3  conflicting  5 only left, 4 only right, 1 differ
ln    DSTO             IEC 61850-7-420:2019A4  (Tr)IEC 61850-90-9:2018A3  conflicting  5 only left, 4 only right, 1 differ
enum  BatteryTypeKind  IEC 61850-7-420:2019A4  (Tr)IEC 61850-90-9:2018A3  conflicting  2 only left, 1 only right

…
```

```console
$ nsdctl -p extended model leaves DBAT.BatSt --ns 90-9
nsdctl: iec61850-model: cannot resolve "DBAT.BatSt": class DBAT has no data object BatSt [(Tr)IEC 61850-90-9:2018A3]
```

`BatSt` exists in 7-420's DBAT but not in 90-9's; a mapping written for one
does not fit the other. See [namespaces.md](namespaces.md#transitional-namespaces-iec-tr-61850-90-9).

## 9. Build a tag list or a UI from JSON

Every command takes `--json`, so the model feeds scripts directly — here a
tag list of the meter's measurements with their units:

```console
$ nsdctl --json model measurements MMXU | jq -r '.[] | [.path, .quantity, .phase // "-", .unitHint.symbol // "-"] | @tsv' | head -8
MMXU.TotW	active-power	total	W
MMXU.TotVAr	reactive-power	total	VAr
MMXU.TotVA	apparent-power	total	VA
MMXU.TotPF	power-factor	total	1
MMXU.Hz	frequency	-	Hz
MMXU.PPV.phsAB	voltage	AB	V
MMXU.PPV.phsBC	voltage	BC	V
MMXU.PPV.phsCA	voltage	CA	V
```

## 10. Use the same model from Go

The library behind nsdctl answers the same questions in code, with the same
embedded catalog and typed constants:

```go
p, _ := official.Default()
res, _ := resolver.New(p).ResolveString("LD0/MMXU1.TotW.mag.f[MX]")
fmt.Println(res.Type(), res.FC()) // FLOAT32 MX

d, _ := semantic.Describe(p, string(names.MMXU_TotW)+".mag.f")
fmt.Println(d.Quantity, d.UnitHint) // active-power W
```

See the [README](../README.md#quick-start) and [cli.md](cli.md) for every
command and flag.
