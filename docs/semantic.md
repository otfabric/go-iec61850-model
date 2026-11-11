# Semantic mapping

Package `semantic` describes model elements in engineering terms. It is
OTFabric interpretation on top of the normative model and is always marked as
such; it never changes what the model defines and never claims a device
implements an element.

```go
d, err := semantic.Describe(profile, "LD0/MMXU1.PNV.phsA.cVal.mag.f[MX]")
// d.Quantity  = voltage        d.Phase     = A
// d.Reference = line-neutral   d.Component = magnitude
// d.UnitHint  = V              d.Rule      = mmxu.pnv (high, rule, otfabric-default)
```

## Vocabulary

| Concept | Values |
| --- | --- |
| `Quantity`, electrical | frequency, frequency-rate, voltage, current, active-power, reactive-power, apparent-power, power-factor, active-energy, reactive-energy, apparent-energy, impedance, resistance, phase-angle, charge, state-of-charge |
| `Quantity`, power quality | total-harmonic-distortion, total-demand-distortion, k-factor, crest-factor, imbalance (with `Of`: the current or voltage they refer to) |
| `Quantity`, process | temperature, temperature-rate, pressure, pressure-rate, flow-rate, volume, level, displacement, speed, rotational-speed, direction, humidity, moisture, irradiance, density, salinity, conductivity, ph, concentration, sound-level, vibration, valve-position, duration |
| `Quantity`, states and counts | switch-position, tap-position, operating-state, operation-count, count (no unit hint) |
| `Phase` | A, B, C, N, AB, BC, CA, net, residual, total |
| `Reference` | line-neutral, line-ground, line-line; for DC circuits pole-pole, positive-ground, negative-ground |
| `Direction` (energy) | supply, demand |
| `Sequence` | positive, negative, zero |
| `Component` | magnitude, instantaneous-magnitude, angle, complex, counter, state, quality, timestamp, unit-config, other |

Rotational speed, vibration and imbalance have no default unit: the standard
leaves it to the device (rpm or 1/s; displacement, velocity or acceleration;
A or %), so their descriptors carry no unit hint. The vocabulary is
protocol-neutral so other OTFabric libraries (Modbus, SunSpec, DNP3, OPC UA)
can share it; it is expected to move to a shared ontology module later.

## Rules

Every descriptor comes from one explicit `Rule`:

| Field | Meaning |
| --- | --- |
| `ID` | Stable identifier, reported in every descriptor |
| `LNClass`, `DataObject` | What the rule maps; `LNClass` may be `*`, and a class rule also applies to subclasses |
| `CDC` | The CDC the data object must have; otherwise the rule does not apply |
| `Quantity`, `Phase`, `Reference`, `Direction`, `Of`, `Sequence` | The meaning |
| `Unit` | Unit hint, default the quantity's unit |
| `Confidence`, `Source`, `Note` | Audit information |

Conservatism:

- No rule, no description: `ErrNoMapping`. Nothing is inferred from parts of
  names at run time; the default rules were written from the data object
  names, CDCs and descriptions of the standards.
- A rule whose CDC differs from the model's is not applied.
- For WYE the phase comes from the sub data object (phsA → A, neut → N, net,
  res); for DEL (phsAB, phsBC, phsCA) the reference is always line-line. A
  sub data object outside those lists is not mapped.
- Wildcard rules (`*`) get medium confidence; the only default one maps `Hz`
  of CDC MV.

The default rules (`semantic.DefaultRules()`) cover the measurement and
status classes below. A rule on an abstract class applies to every class that
inherits it (SupervisionLN to all S classes, SwitchingEquipmentLN to XCBR and
XSWI, ...).

| Standard | Classes |
| --- | --- |
| IEC 61850-7-4, measurement | MMXU (also HzRte), MMXN (also Imp), MMTR, MMTN, MSQI, MHAI, MHAN, MMDC, MMET, MFLW, MENV, MHYD |
| IEC 61850-7-4, supervision | SIML, SARC, STMP, SPRS, SVBR, ZBAT, ZAXN, the operation counter of every S class |
| IEC 61850-7-4, process | XCBR and XSWI (Pos, OpCnt), YLTC and ATCC (TapPos, OpCnt, ATCC measurements), KPMP and KFAN (Spd, SpdSv, OpCtl) |
| IEC 61850-7-420 | DGEN, DLOD, DSTO and the other DER units (DEROpSt, energies, available powers, counts, times), DBAT, DINV, SBAT, SINV |
| Technical reports | ZBTC and SBAT of IEC TR 61850-90-3; the energies as measured values (`*Meas`, per quadrant) of IEC TR 61850-90-6 in MMTR and MMTN |

Status values are mapped where they are a quantity of their own: switch
positions (DPC `stVal`), tap positions (ISC `valWTr.posVal`), operating
states (ENS, KPMP/KFAN `OpCtl`) and counters (INS, INC `stVal`). Alarms,
trips, set points, settings and descriptions are not quantities and are
reported as not measured.

ZINV, ZRCT, MPRS, MCCF and MSTA, which IEC 61850-7-420:2009 (Edition 1)
defines, have no NSD and are in no embedded namespace; 7-420:2019 models
inverters and rectifiers with DINV and DRTF. 7-410 classes have no default
rules beyond those they inherit.

## Exclusions

Where the standard does not state the unit or the meaning of a measured value,
the data object is excluded with a reason instead of guessed
(`semantic.DefaultExclusions()`). `Describe` fails with `ErrNoMapping` and the
reason, and the coverage report counts it as excluded. Examples:

| Data objects | Reason |
| --- | --- |
| MMXU `Av*Phs`, `Max*Phs`, `Min*Phs`; MSQI `MaxImb*` | Statistics over the phases, which the vocabulary does not model |
| MENV emissions, dust, ozone | Concentration or emission rate: neither the unit nor the basis is stated |
| MMET cloud, rain, snow and ice cover | The unit (oktas, depth, rate, water equivalent) is not stated |
| MSQI `DQ0Seq`, DGEN `GnH` | The quantity (current or voltage) is not stated |
| ZBAT and DBAT `VolChgRte` | A rate in V/s, for which SIUnitKind has no unit |
| XCBR `SumSwARs` | Whether A or A²t is device configuration |
| 7-420 operational settings (`VArTot`, `EffWh`, ...) | Settings published as MV, not measurements |

`Mapper.WithExclusions` adds exclusions; an override for the same class and
data object takes precedence over an exclusion, and an exclusion on a class
takes precedence over a default rule on its base class.

## Harmonics and sequences

HMV, HWYE and HDEL hold arrays of complex values. The phase comes from the
sub data object (`phsAHar` → A, `phsABHar` → AB, line-line for HDEL), and the
array index from the path: `MHAI.HA.phsAHar(3).cVal.mag.f` has `Harmonic` 3.
IEC 61850-7-3 puts the DC component at index 0 and harmonic *n* at index *n*
when `numCyc` is 1; with a larger `numCyc` the array interleaves subharmonics
(divide by 2^(numCyc−1)), and `hvRef` says whether a magnitude is absolute or a
ratio. Both are device configuration, so every harmonic descriptor carries a
note saying so. `FindMeasurements` lists one descriptor per phase array.

SEQ data objects (MSQI `SeqA`, `SeqV`) expand into `c1`, `c2` and `c3`. Which
symmetrical component each holds (positive, negative, zero, or direct,
quadrature, zero) is the device's `seqT` attribute; the descriptor leaves
`Sequence` empty and says so in a note. `Sequence` is set only where the data
object names the component (`ImbNgA`: negative, `ImbZroV`: zero).

## Coverage

`semantic.Coverage(profile)` (and `Mapper.CoverageOf` for one class) reports
per concrete class how every data object is treated:

| Status | Meaning |
| --- | --- |
| mapped | A rule describes it |
| excluded | Deliberately unmapped, with the reason |
| unmapped | A measured value (MV, CMV, WYE, DEL, SEQ, HMV, HWYE, HDEL, HST, SAV, BCR) without a rule, or with a rule whose CDC differs: a gap |
| not-measured | Status, control, setting or description without a rule |

For the classes of the table above every measured value is mapped or excluded;
`TestSpec_*` and the per-class golden files in
[semantic/testdata/golden](../semantic/testdata/golden) pin this. The
generated `coverage.md` and `coverage.json` (`nsdctl generate docs|json`) list
the counts per class.

## Overrides

```go
m, err := semantic.Default.WithOverrides(semantic.Rule{
	ID: "vendor.zinv.w", LNClass: "ZINV", DataObject: "W", CDC: "MV",
	Quantity: semantic.QuantityActivePower, Phase: semantic.PhaseTotal,
	Source: "vendor:example", Confidence: semantic.ConfidenceMedium,
})
d, err := m.Describe(profile, "ZINV.W")   // d.Basis == semantic.BasisOverride
```

Overrides must name their source, take precedence over default rules, and are
reported with `Basis = override`. Conflicting or incomplete rules fail with
`ErrInvalidRule`.

## Components

The attribute path below the data object decides the component: `mag` →
magnitude, `instMag` → instantaneous magnitude, `cVal` → complex, `cVal.mag` →
magnitude, `cVal.ang` → angle (unit hint deg), `actVal` → counter (note:
multiply by `pulsQty`), `stVal` and `valWTr.posVal` → state, `q`, `t`,
`units` → quality, timestamp, unit-config, anything else → other. Only value
components carry a unit hint, and states and counts carry none. An integer
leaf (`.i`) gets a note that it must be scaled with the `sVC` configuration.

## Unit hints and device units

`UnitHint` is what the rule expects: the base unit of the quantity (V, degC,
m³/s, ...), never a multiplier, because the namespaces fix none. A unit the
standard text gives (MHYD `Slnt` in g/l) is stated as the equal base unit
(kg/m³). The device's real unit and multiplier are in its configured `units`
attribute:

```go
u, err := semantic.DeviceUnit(profile, siUnit, multiplier) // e.g. 29, 3 → kV
ok := d.Quantity.Accepts(u)
value := raw * u.Factor()
```

`DeviceUnit` reads the SIUnitKind and MultiplierKind enumerations of the
loaded model; no unit codes are hard-coded. `NormalizeUnit` parses unit
strings ("kV", "MVAr", "kvar", "kWh", "%") into the same `Unit` type and
rejects what it does not know.

## Finding measurements

- `FindMeasurements(profile, "MMXU")` — every mapped data object of a class,
  WYE, DEL, HWYE and HDEL expanded per phase and SEQ per component, in model
  order.
- `Match(profile, semantic.Criteria{Quantity: ..., Phase: ..., Of: ..., MinConfidence: ...})`
  — across every class of the profile, e.g. every temperature, or every THD
  of a current.
