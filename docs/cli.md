# nsdctl

`nsdctl` is the companion CLI of this module. It works on NSD packages you
obtained yourself and never downloads anything.

Pre-built binaries for Linux, macOS and Windows are attached to every
release; see [Install nsdctl](../README.md#install-nsdctl). With Go:

```bash
go install github.com/otfabric/go-iec61850-model/cmd/nsdctl@latest
# or, in a clone: make build && ./bin/nsdctl
```

## Global flags

| Flag | Meaning |
| --- | --- |
| `-s, --source PATH` | NSD source: ZIP archive, directory or document; repeatable |
| `-p, --profile ID` | Namespace to select, e.g. `IEC 61850-7-4:2007B5` or just `IEC 61850-7-4`; repeatable. Default: the only release of each standard |
| `--json` | Machine-readable JSON instead of text |
| `--strict` | Treat importer warnings (unsupported elements and attributes) as errors |
| `--lang CODE` | NSDoc language to apply (default `en`; empty disables) |

## Commands

| Command | Purpose |
| --- | --- |
| `inspect [SOURCE...]` | Namespaces, counts, dependencies, digests, manifest, documentation, warnings |
| `validate [SOURCE...]` | Parse, then check each namespace's profile (or `--profile`): dependencies, conflicts, unresolved references |
| `dependencies [SOURCE...]` (`deps`) | Each declared dependency and whether the loaded sources satisfy it: ok, missing, ambiguous |
| `diff OLD NEW [--exit-code]` | Added, removed and changed definitions between two sources |
| `model show NAME\|PATH` | A class (with inherited data objects and semantics), CDC, type, enumeration or basic type; or a resolved path with every step |
| `model list [--kind ln\|abstract\|cdc\|ca\|enum\|basic]` | Definitions of one kind |
| `model search [--name --exact --cdc --type --fc --group --standard --abstract --kind]` | Search across the profile |
| `model describe REF` | Semantic descriptor: quantity, phase, reference, component, unit hint, rule |
| `model measurements LNCLASS` | Mapped measurements of a class, per phase |
| `model leaves LNCLASS[.DO...] [--fc FC]` | All leaf attributes below a class or path |
| `generate go\|docs\|json -o DIR` | Deterministic output; see [generation.md](generation.md) |
| `sources check [--index --lock --packages DIR --tiers T]` | Validate the registry and lock; with `--packages`, check that every tracked package is present and carries its release, and compare digests with the lock |
| `sources fetch [--dest DIR --tiers T --allow-unpinned --refresh]` | Download the tracked packages from their official locations, verified against the lock, into `.nsd-cache/` or a directory outside the repository |
| `sources lock --packages DIR [--write]` | Produce lock entries (digest, content, dependencies, status) for the tracked packages found in DIR |
| `schema [SOURCE...] [--exit-code]` | XSD constructs (e.g. of IEC 61850-7-7) that the importer does not model |
| `version` | Version, tag, commit, build date, Go version, platform (`--json` too) |

Examples:

```bash
nsdctl inspect ./nsd/IEC_61850-7-4.zip
nsdctl -s ./nsd validate
nsdctl -s ./nsd -p "IEC 61850-7-4:2007B5" model show MMXU
nsdctl -s ./nsd -p "IEC 61850-7-4:2007B5" model show 'LD0/MMXU1.PNV.phsA.cVal.mag.f[MX]'
nsdctl -s ./nsd -p "IEC 61850-7-4:2007B5" model search --cdc WYE --kind do
nsdctl diff --exit-code ./nsd-2019/ ./nsd-2025/
nsdctl -s ./nsd -p "IEC 61850-7-4:2007B5" generate docs -o ./generated-docs
```

Quote references with brackets in shells that glob (`zsh`).

## Walkthrough with the official packages

Every example below is real output of `nsdctl` run on the 14 official packages
tracked by this repository (`make nsd-fetch`), cut where marked with `…`. The
packages contain the model, not documentation texts, so the output shows
identifiers, types and structure only.

The examples use `-s .nsd-cache` (the directory `make nsd-fetch` fills). Any
directory, ZIP archive or single NSD file works as a source.

### Version

```console
$ nsdctl version
nsdctl version: v0.1.0
tag:         v0.1.0
commit:      3f2a9c1e
build date:  2026-10-21T16:00:00Z
go version:  go1.27.3
platform:    darwin/arm64
```

Release binaries carry version, tag, commit and build date. A binary built with
`go install` reports the module version and commit the Go toolchain records.

### Download and check the packages

`sources fetch` downloads every tracked package from its official location and
verifies it against the digest pinned in `sources/nsd.lock.json`; files that
already match are not downloaded again:

```console
$ nsdctl sources fetch
PACKAGE                                  STATUS  LOCK    SHA256        ERROR
IEC_61850-7-2.NSD.2007B6.Light.zip       cached  pinned  d2ecb929472f  -
IEC_61850-7-3.NSD.2007B6.Light.zip       cached  pinned  5305aa90706d  -
IEC_61850-7-4.NSD.2007B6.Light.zip       cached  pinned  429c9b5cad3e  -
IEC_61850-7-410.NSD.2024A4.Light.zip     cached  pinned  dd6231f14072  -
IEC_61850-7-420.NSD.2019A4.Light.zip     cached  pinned  0b6774ea69a0  -
IEC_TR_61850-90-9.NSD.2018A3.Light.zip   cached  pinned  68a86c06177e  -
IEC_TR_61850-90-14.NSD.2020A3.Light.zip  cached  pinned  d6f06ace3eca  -
IEC_TR_61850-90-6.NSD.2018B.Light.zip    cached  pinned  083347f18438  -
IEC_TR_61850-90-3.NSD.2015B.Light.zip    cached  pinned  813b32e8c5a2  -
IEC_TR_61850-90-4.NSD.2018A.Light.zip    cached  pinned  99319b01bbe8  -
IEC_TR_61850-90-11.NSD.2019A2.light.zip  cached  pinned  63401cd1e065  -
IEC_TR_61850-90-21.NSD.2022A2.Light.zip  cached  pinned  d6df9731479a  -
IEC_61850-8-1.NSD.2003A4.Light.zip       cached  pinned  b5b54ab11731  -
IEC_61850-7-7.2017.NSD.2017A.full.zip    cached  pinned  d6c5f7fdac0f  -
```

`sources check --packages` confirms that every tracked package is present and
carries the namespace release its name announces, and compares digests with the
lock:

```console
$ nsdctl sources check --packages .nsd-cache
  info: index: registries[1]: IEC 61850 Code Components has not been verified yet [unverified-registry]
  info: index: registries[2]: IEC webstore has not been verified yet [unverified-registry]
TIER         PART   PACKAGE                                  STATUS  NAMESPACES
required     7-2    IEC_61850-7-2.NSD.2007B6.Light.zip       ok      IEC 61850-7-2:2007B6
required     7-3    IEC_61850-7-3.NSD.2007B6.Light.zip       ok      IEC 61850-7-3:2007B6
required     7-4    IEC_61850-7-4.NSD.2007B6.Light.zip       ok      IEC 61850-7-4:2007B6
required     7-410  IEC_61850-7-410.NSD.2024A4.Light.zip     ok      IEC 61850-7-410:2024A4
required     7-420  IEC_61850-7-420.NSD.2019A4.Light.zip     ok      IEC 61850-7-420:2019A4
recommended  90-9   IEC_TR_61850-90-9.NSD.2018A3.Light.zip   ok      (Tr)IEC 61850-90-9:2018A3
recommended  90-14  IEC_TR_61850-90-14.NSD.2020A3.Light.zip  ok      (Tr)IEC 61850-90-14:2020A3
optional     90-6   IEC_TR_61850-90-6.NSD.2018B.Light.zip    ok      (Tr)IEC61850-90-6:2018B
optional     90-3   IEC_TR_61850-90-3.NSD.2015B.Light.zip    ok      (Tr)IEC61850-90-3:2015B
optional     90-4   IEC_TR_61850-90-4.NSD.2018A.Light.zip    ok      (Tr)IEC 61850-90-4:2018A
optional     90-11  IEC_TR_61850-90-11.NSD.2019A2.light.zip  ok      (Tr)IEC 61850-90-11:2019A2
optional     90-21  IEC_TR_61850-90-21.NSD.2022A2.Light.zip  ok      (Tr)IEC 61850-90-21:2022A2
optional     8-1    IEC_61850-8-1.NSD.2003A4.Light.zip       ok      IEC 61850-8-1:2003A4
optional     7-7    IEC_61850-7-7.2017.NSD.2017A.full.zip    ok      3 schema files
14 locked namespaces
NAMESPACE                   STATUS         REDISTRIBUTION  SHA256
(Tr)IEC 61850-90-11:2019A2  nsd-validated  local-only      63401cd1e065
…
```

### Inspect, validate, dependencies

```console
$ nsdctl inspect .nsd-cache/IEC_61850-7-420.NSD.2019A4.Light.zip
.nsd-cache/IEC_61850-7-420.NSD.2019A4.Light.zip
  sha256    0b6774ea69a00295d22ddf4409c819fd943dbfb7d24661e0a0db0a35b70451fe
  manifest  IECManifest (9 elements)
  NAMESPACE               LN  ABSTRACT  CDC  TYPES  ENUMS  BASIC  DEPENDS ON
  IEC 61850-7-420:2019A4  50  32        0    0      48     0      IEC 61850-7-4:2007B
```

```console
$ nsdctl dependencies .nsd-cache
NAMESPACE                   REQUIRES               STATUS  SATISFIED BY
(Tr)IEC 61850-90-11:2019A2  IEC 61850-7-4:2007B    ok      IEC 61850-7-4:2007B6
(Tr)IEC 61850-90-14:2020A3  IEC 61850-7-4:2007B    ok      IEC 61850-7-4:2007B6
(Tr)IEC 61850-90-21:2022A2  IEC 61850-7-4:2007B    ok      IEC 61850-7-4:2007B6
(Tr)IEC 61850-90-4:2018A    IEC 61850-7-4:2007B    ok      IEC 61850-7-4:2007B6
(Tr)IEC 61850-90-9:2018A3   IEC 61850-7-420:2019A  ok      IEC 61850-7-420:2019A4
(Tr)IEC61850-90-3:2015B     IEC 61850-7-4:2007B    ok      IEC 61850-7-4:2007B6
(Tr)IEC61850-90-6:2018B     IEC 61850-7-4:2007B    ok      IEC 61850-7-4:2007B6
IEC 61850-7-3:2007B6        IEC 61850-7-2:2007B    ok      IEC 61850-7-2:2007B6
IEC 61850-7-4:2007B6        IEC 61850-7-3:2007B    ok      IEC 61850-7-3:2007B6
IEC 61850-7-410:2024A4      IEC 61850-7-4:2007B    ok      IEC 61850-7-4:2007B6
IEC 61850-7-420:2019A4      IEC 61850-7-4:2007B    ok      IEC 61850-7-4:2007B6
```

`validate` checks every namespace together with its dependencies. The official
packages validate without errors; the warnings are upstream gaps (references to
definitions no package provides) and names that two namespaces define (see
[nsd-sources.md](nsd-sources.md)):

```console
$ nsdctl validate .nsd-cache
…
  warning: DCCT.CnttAnc: underlying type "AncillaryServiceKind" is not defined [unresolved-reference]
  warning: DCCT.HeatRtePrc: CDC "SCR" is not defined [unresolved-reference]
  warning: DCCT.CRtePrc: CDC "SCR" is not defined [unresolved-reference]
  warning: FSCH: logical node class "FSCH" is defined by IEC 61850-7-4:2007B6, IEC 61850-7-410:2024A4; lookups of it are ambiguous [conflicting-definition]
  warning: RST.setMag: presence condition "MFnsg" is not defined [unresolved-reference]
  warning: RST._setMag: presence condition "MFsg" is not defined [unresolved-reference]
  warning: RST.__setMag: presence condition "MFsg" is not defined [unresolved-reference]
12 namespaces, 0 errors, 26 warnings
```

### What does a logical node hold?

`model show` lists every data object of a class, including inherited ones and
those that extension namespaces add (here 90-6 and 7-420 extend MMXU), with
the namespace that declares each and the rule-based semantics:

```console
$ nsdctl -s .nsd-cache model show MMXU
logical node class MMXU (IEC 61850-7-4:2007B6)
group        M Metering and measurement
inheritance  MMXU → FunctionLN → StatisticsLN → DomainLN

DATA OBJECT  CDC  PRESENCE   DECLARED IN   NAMESPACE                SEMANTICS
NamPlt       LPL  MONamPlt   DomainLN      IEC 61850-7-4:2007B6     -
Beh          ENS  M          DomainLN      IEC 61850-7-4:2007B6     -
Health       ENS  O          DomainLN      IEC 61850-7-4:2007B6     -
Mir          SPS  MOcond(1)  DomainLN      IEC 61850-7-4:2007B6     -
Mod          ENC  O          DomainLN      IEC 61850-7-4:2007B6     -
InRef        ORG  Omulti     DomainLN      IEC 61850-7-4:2007B6     -
ClcExp       SPS  O          StatisticsLN  IEC 61850-7-4:2007B6     -
…
```

…and at the end of the same listing, the data objects added by extensions:

```console
HzRte        MV   O          MMXU          IEC 61850-7-4:2007B6     -
ClcTotVA     ENG  O          MMXU          IEC 61850-7-4:2007B6     -
PFSign       ENG  O          MMXU          IEC 61850-7-4:2007B6     -
PwrFlwDir    ACD  O          MMXU          (Tr)IEC61850-90-6:2018B  -
PFExtQud     SPS  O          MMXU          IEC 61850-7-420:2019A4   -
PQSt         ENS  O          MMXU          IEC 61850-7-420:2019A4   -
```

### Resolve a device reference

```console
$ nsdctl -s .nsd-cache model show 'LD0/MMXU1.PhV.phsA.cVal.mag.f[MX]'
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

Setting CDCs exist in variants (SP, SG, SE). Without an FC the resolver says
so; with one it selects the variant:

```console
$ nsdctl -s .nsd-cache model show 'PTOC.StrVal.setMag.f'
PTOC.StrVal.setMag.f → FLOAT32 (BASIC)

STEP              NAME    TYPE              FC  PRESENCE            DECLARED IN       NAMESPACE
LogicalNode       PTOC    PTOC              -   -                   PTOC              IEC 61850-7-4:2007B6
DataObject        StrVal  ASG               -   O                   PTOC              IEC 61850-7-4:2007B6
DataAttribute     setMag  AnalogueValueCtl  -   M                   ASG/SP            IEC 61850-7-3:2007B6
SubDataAttribute  f       FLOAT32           -   AllOnlyOneGroup(2)  AnalogueValueCtl  IEC 61850-7-3:2007B6

The FC of setMag depends on the realization: ASG/SP.setMag, ASG/SG.setMag, ASG/SE.setMag.
Append [FC] to the reference to select one.
```

```console
$ nsdctl -s .nsd-cache model show 'PTOC.StrVal.setMag.f[SG]'
PTOC.StrVal.setMag.f → FLOAT32 (BASIC) [SG]

STEP              NAME    TYPE              FC  PRESENCE            DECLARED IN       NAMESPACE
LogicalNode       PTOC    PTOC              -   -                   PTOC              IEC 61850-7-4:2007B6
DataObject        StrVal  ASG               -   O                   PTOC              IEC 61850-7-4:2007B6
DataAttribute     setMag  AnalogueValueCtl  SG  M                   ASG/SG            IEC 61850-7-3:2007B6
SubDataAttribute  f       FLOAT32           SG  AllOnlyOneGroup(2)  AnalogueValueCtl  IEC 61850-7-3:2007B6
```

### CDCs, variants and enumerations

```console
$ nsdctl -s .nsd-cache model show SPG
CDC SPG/SP (IEC 61850-7-3:2007B6)

MEMBER   KIND            TYPE          FC  PRESENCE
setVal   data attribute  BOOLEAN       SP  M
d        data attribute  VisString255  DC  O
dU       data attribute  Unicode255    DC  O
cdcName  data attribute  VisString255  EX  O
dataNs   data attribute  VisString255  EX  MOdataNs

CDC SPG/SG (IEC 61850-7-3:2007B6)

MEMBER   KIND            TYPE          FC  PRESENCE
setVal   data attribute  BOOLEAN       SG  M
d        data attribute  VisString255  DC  O
dU       data attribute  Unicode255    DC  O
cdcName  data attribute  VisString255  EX  O
dataNs   data attribute  VisString255  EX  MOdataNs
…
```

```console
$ nsdctl -s .nsd-cache model show BehaviourModeKind
enumeration BehaviourModeKind (IEC 61850-7-4:2007B6)
VALUE  LITERAL
1      on
2      blocked
3      test
4      test/blocked
5      off
```

### Semantics

```console
$ nsdctl -s .nsd-cache model describe 'LD0/MMXU1.PPV.phsAB.cVal.mag.f[MX]'
MMXU.PPV.phsAB.cVal.mag.f
quantity     voltage
phase        AB
reference    line-line
component    magnitude
unit hint    V
mapping      rule mmxu.ppv, rule, high confidence (otfabric-default)
```

```console
$ nsdctl -s .nsd-cache model measurements MMXU
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
…
```

```console
$ nsdctl -s .nsd-cache model measurements MMTR
PATH          QUANTITY         PHASE  REFERENCE  DIRECTION  UNIT  RULE
MMTR.TotVAh   apparent-energy  total  -          -          VAh   mmtr.totvah
MMTR.TotWh    active-energy    total  -          -          Wh    mmtr.totwh
MMTR.TotVArh  reactive-energy  total  -          -          VArh  mmtr.totvarh
MMTR.SupWh    active-energy    total  -          supply     Wh    mmtr.supwh
MMTR.SupVArh  reactive-energy  total  -          supply     VArh  mmtr.supvarh
MMTR.DmdWh    active-energy    total  -          demand     Wh    mmtr.dmdwh
MMTR.DmdVArh  reactive-energy  total  -          demand     VArh  mmtr.dmdvarh
```

### Search, list, leaves

```console
$ nsdctl -s .nsd-cache model search --cdc WYE --kind do --standard "IEC 61850-7-4"
KIND  PATH            TYPE  FC  PRESENCE            NAMESPACE
do    MFLK.PhPiLoFil  WYE   -   O                   IEC 61850-7-4:2007B6
do    MFLK.PhPiMax    WYE   -   O                   IEC 61850-7-4:2007B6
do    MFLK.PhPiRoot   WYE   -   O                   IEC 61850-7-4:2007B6
do    MFLK.PhPlt      WYE   -   O                   IEC 61850-7-4:2007B6
do    MFLK.PhPst      WYE   -   O                   IEC 61850-7-4:2007B6
do    MHAI.HATm       WYE   -   O                   IEC 61850-7-4:2007B6
do    MHAI.HCfA       WYE   -   O                   IEC 61850-7-4:2007B6
do    MHAI.HCfPhV     WYE   -   O                   IEC 61850-7-4:2007B6
do    MHAI.HKFact     WYE   -   O                   IEC 61850-7-4:2007B6
…
```

```console
$ nsdctl -s .nsd-cache model list --kind ln
NAME     NAMESPACE                   INFO
AATS     (Tr)IEC61850-90-6:2018B     Automatic control (base AutomaticControlLN)
ACTM     IEC 61850-7-410:2024A4      Automatic control (base ControllingLN)
AEBR     IEC 61850-7-410:2024A4      Automatic control (base StateChangeOpFunLN)
AEMC     IEC 61850-7-410:2024A4      Automatic control (base BulkGenerationMgmtFunLN)
AENC     IEC 61850-7-410:2024A4      Automatic control (base EnergyOpFunLN)
AENN     IEC 61850-7-410:2024A4      Automatic control (base EnergyOpFunLN)
AENX     IEC 61850-7-410:2024A4      Automatic control (base EnergyOpFunLN)
…
```

```console
$ nsdctl -s .nsd-cache model leaves MMXU.TotW --fc MX
PATH                 TYPE       KIND        FC
MMXU.TotW.instMag.i  INT32      BASIC       MX
MMXU.TotW.instMag.f  FLOAT32    BASIC       MX
MMXU.TotW.mag.i      INT32      BASIC       MX
MMXU.TotW.mag.f      FLOAT32    BASIC       MX
MMXU.TotW.range      RangeKind  ENUMERATED  MX
MMXU.TotW.q          Quality    BASIC       MX
MMXU.TotW.t          Timestamp  BASIC       MX
```

### Ambiguity and profiles

IEC TR 61850-90-9 and IEC 61850-7-420 both define `DBAT`. With both loaded,
`nsdctl` refuses to guess:

```console
$ nsdctl -s .nsd-cache model show DBAT
nsdctl: iec61850-model: ambiguous logical node class "DBAT": defined in (Tr)IEC 61850-90-9:2018A3, IEC 61850-7-420:2019A4
```

Exit code 1.

Select the releases with `--profile` (dependencies are added automatically):

```console
$ nsdctl -s .nsd-cache -p "IEC 61850-7-420" model show DBAT
logical node class DBAT (IEC 61850-7-420:2019A4)
group        D Distributed energy resources
inheritance  DBAT → StorageUnitLN → ControlEquipmentInterfaceLN → ControlledLN → ControllingLN → FunctionLN → StatisticsLN → DomainLN

DATA OBJECT   CDC  PRESENCE   DECLARED IN                  NAMESPACE               SEMANTICS
NamPlt        LPL  MONamPlt   DomainLN                     IEC 61850-7-4:2007B6    -
…
```

### Compare releases

`diff` compares two sources definition by definition, here the storage models
of the technical report 90-9 with the later 7-420 edition that absorbed them:

```console
$ nsdctl diff .nsd-cache/IEC_TR_61850-90-9.NSD.2018A3.Light.zip .nsd-cache/IEC_61850-7-420.NSD.2019A4.Light.zip
added    DataObject DBAT.BatOn added
added    DataObject DBAT.BatSt added
added    DataObject DBAT.BatTest added
removed  DataObject DBAT.BatTestRsl removed
added    DataObject DBAT.BatVNom added
added    DataObject DBAT.TestAddRsl added
added    DataObject DBAT.TestRsl added
143 changes
```

### Schema coverage

`schema` compares the NSD XML schema of IEC 61850-7-7 with what the importer
models. Everything outside the service namespaces is modeled:

```console
$ nsdctl schema .nsd-cache/IEC_61850-7-7.2017.NSD.2017A.full.zip
…
.nsd-cache/IEC_61850-7-7.2017.NSD.2017A.full.zip!NSD.xsd
  target namespace  http://www.iec.ch/61850/2016/NSD
  declares          42 elements, 51 attributes
  not modeled       elements: ApplicableServiceNS, AppliesTo, ServiceCDC, ServiceCDCs, ServiceConstructedAttribute, ServiceConstructedAttributes, ServiceDataAttribute, ServiceNS, ServiceNsUsage, ServiceTypeRealization, ServiceTypeRealizations
                    attributes: cdc
```

### Generate

```console
$ nsdctl -s .nsd-cache generate go -o .generated/go
wrote LOCAL-ONLY.md
wrote NOTICE.md
wrote names_gen.go
output is local-only: do not commit or publish it (see LOCAL-ONLY.md)
```

The generated file starts with its provenance, lists names that are ambiguous
in the profile, and declares typed constants:

```go
// Code generated by nsdctl generate go; DO NOT EDIT.
//
// Generator: nsdctl v0.1.0 (output format 3)
// Distribution: local-only; derived from IEC NSD files, do not commit or publish (see docs/licensing.md of github.com/otfabric/go-iec61850-model)
// Namespaces:
//   (Tr)IEC 61850-90-11:2019A2 sha256:b835d6bccacbb4c72abeeeb45babf33c3bc1b4bf919dd7e3b2afb9708cac4650
//   (Tr)IEC 61850-90-14:2020A3 sha256:f9bcb0a9faa3dde20757358047941760c4d28ce658f6e0798e90a4b1c18cc5d4
//   (Tr)IEC 61850-90-21:2022A2 sha256:cc959cf716195ae95e430981bf2f5075658ff37fedf1fea3359fe9a152aca3df
// …
// Logical node classes.
const (
	LN_AATS    LNClass = "AATS"
	LN_ACTM    LNClass = "ACTM"
// …
	MMXU_Hz         DataObjectRef = "MMXU.Hz"         // MV, O
	MMXU_PPV        DataObjectRef = "MMXU.PPV"        // DEL, O
```

`--check` compares instead of writing (exit code 4 when out of date):

```console
$ nsdctl -s .nsd-cache generate go -o .generated/go --check
.generated/go is up to date (3 files)
```

### JSON

Every command takes `--json`:

```console
$ nsdctl -s .nsd-cache --json model describe MMXU.Hz
{
  "path": "MMXU.Hz",
  "lnClass": "MMXU",
  "dataObject": "Hz",
  "cdc": "MV",
  "presence": {
    "name": "O"
  },
  "namespace": {
    "standard": "IEC 61850-7-4",
    "version": "2007",
    "revision": "B",
    "release": 6
  },
  "quantity": "frequency",
  "unitHint": {
    "symbol": "Hz"
  },
  "confidence": "high",
  "basis": "rule",
  "rule": "mmxu.hz",
  "source": "otfabric-default"
}
```


## Exit codes

| Code | Meaning |
| --- | --- |
| 0 | Success |
| 1 | Usage error, unknown name, path that does not resolve, refused overwrite |
| 2 | A source cannot be read or parsed (also unsafe archives and exceeded limits), a download failed |
| 3 | Validation failed: structural errors, unsatisfied dependencies, invalid lock or registry, a downloaded package differs from its pinned digest or is not pinned |
| 4 | Differences: `diff --exit-code` found changes, `generate --check` is out of date, a tracked package is missing or carries another release, local packages differ from the lock, `schema --exit-code` found gaps |
| 5 | The license policy refused `generate --publish` |

Errors go to stderr prefixed with `nsdctl:`; results go to stdout.
