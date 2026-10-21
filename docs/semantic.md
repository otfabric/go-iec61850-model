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
| `Quantity` | frequency, voltage, current, active-power, reactive-power, apparent-power, power-factor, active-energy, reactive-energy, apparent-energy, impedance |
| `Phase` | A, B, C, N, AB, BC, CA, net, residual, total |
| `Reference` (voltage) | line-neutral, line-ground, line-line |
| `Direction` (energy) | supply, demand |
| `Component` | magnitude, instantaneous-magnitude, angle, complex, counter, quality, timestamp, unit-config, other |

The vocabulary is protocol-neutral so other OTFabric libraries (Modbus,
SunSpec, DNP3, OPC UA) can share it; it is expected to move to a shared
ontology module later.

## Rules

Every descriptor comes from one explicit `Rule`:

| Field | Meaning |
| --- | --- |
| `ID` | Stable identifier, reported in every descriptor |
| `LNClass`, `DataObject` | What the rule maps; `LNClass` may be `*`, and a class rule also applies to subclasses |
| `CDC` | The CDC the data object must have; otherwise the rule does not apply |
| `Quantity`, `Phase`, `Reference`, `Direction` | The meaning |
| `Unit` | Unit hint, default the quantity's unit |
| `Confidence`, `Source`, `Note` | Audit information |

Conservatism:

- No rule, no description: `ErrNoMapping`. Nothing is inferred from parts of
  names.
- A rule whose CDC differs from the model's is not applied.
- For WYE the phase comes from the sub data object (phsA → A, neut → N, net,
  res); for DEL (phsAB, phsBC, phsCA) the reference is always line-line. A
  sub data object outside those lists is not mapped.
- Wildcard rules (`*`) get medium confidence; the only default one maps `Hz`
  of CDC MV.

The default rules (`semantic.DefaultRules()`) cover MMXU, MMXN and MMTR:

| Class | Data objects |
| --- | --- |
| MMXU | Hz, TotW, TotVAr, TotVA, TotPF (total); PPV (line-line), PNV (line-neutral), PhV (line-ground), A, W, VAr, VA, PF, Z (per phase) |
| MMXN | Amp, Vol, Watt, VolAmpr, VolAmp, PwrFact, Hz |
| MMTR | TotWh, TotVArh, TotVAh (total); SupWh, SupVArh (supply); DmdWh, DmdVArh (demand) |

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
multiply by `pulsQty`), `q`, `t`, `units` → quality, timestamp, unit-config,
anything else → other. Only value components carry a unit hint. An integer
leaf (`.i`) gets a note that it must be scaled with the `sVC` configuration.

## Unit hints and device units

`UnitHint` is what the rule expects. The device's real unit is in its
configured `units` attribute:

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
  WYE and DEL expanded per phase, in model order.
- `Match(profile, semantic.Criteria{Quantity: ..., Phase: ..., MinConfidence: ...})`
  — across every class of the profile.
