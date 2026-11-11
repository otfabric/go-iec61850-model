// SPDX-License-Identifier: MIT

package semantic_test

// Spec tests: the default rules against the official namespaces
// embedded in package official (IEC 61850-7-4:2007B6, 7-420:2019A4 and
// the technical reports of the extended profile). Each expectation
// follows the data object name, its CDC and the data object description
// of the standard; data objects whose unit or meaning the standard
// leaves open must be excluded with a reason, never guessed.

import (
	"errors"
	"strings"
	"testing"

	"github.com/otfabric/go-iec61850-model/catalog"
	"github.com/otfabric/go-iec61850-model/model"
	"github.com/otfabric/go-iec61850-model/official"
	"github.com/otfabric/go-iec61850-model/semantic"
)

func officialProfile(t *testing.T, name string) *catalog.Profile {
	t.Helper()
	p, err := official.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// want is the expected meaning of a reference; empty fields must be
// empty in the descriptor, unit "-" means no unit hint.
type want struct {
	ref       string
	quantity  semantic.Quantity
	phase     semantic.Phase
	reference semantic.Reference
	of        semantic.Quantity
	unit      string
	rule      string
}

func expect(t *testing.T, l semantic.Lookup, tests []want) {
	t.Helper()
	for _, tc := range tests {
		d, err := semantic.Describe(l, tc.ref)
		if err != nil {
			t.Errorf("%s: %v", tc.ref, err)
			continue
		}
		unit := d.UnitHint.String()
		if unit == "" {
			unit = "-"
		}
		if d.Quantity != tc.quantity || d.Phase != tc.phase || d.Reference != tc.reference || d.Of != tc.of || unit != tc.unit || d.Rule != tc.rule {
			t.Errorf("%s = %s/%s/%s/of %s/%s/%s, want %s/%s/%s/of %s/%s/%s", tc.ref,
				d.Quantity, d.Phase, d.Reference, d.Of, unit, d.Rule,
				tc.quantity, tc.phase, tc.reference, tc.of, tc.unit, tc.rule)
		}
		if d.Basis != semantic.BasisRule || d.Source != semantic.DefaultSource {
			t.Errorf("%s: basis %s, source %s", tc.ref, d.Basis, d.Source)
		}
	}
}

// excluded asserts that the references are deliberately unmapped, with
// a reason that contains the given text.
func excluded(t *testing.T, l semantic.Lookup, reason string, refs ...string) {
	t.Helper()
	for _, ref := range refs {
		_, err := semantic.Describe(l, ref)
		if !errors.Is(err, model.ErrNoMapping) || !strings.Contains(err.Error(), "is not mapped: ") || !strings.Contains(err.Error(), reason) {
			t.Errorf("%s: error = %v, want an exclusion mentioning %q", ref, err, reason)
		}
	}
}

// noGaps asserts that every measured value of the classes is mapped or
// excluded.
func noGaps(t *testing.T, l semantic.Lookup, classes ...string) {
	t.Helper()
	seen := map[string]bool{}
	for _, ln := range l.ListLogicalNodes() {
		for _, cls := range classes {
			if ln.Name != cls {
				continue
			}
			c, err := semantic.Default.CoverageOf(l, ln.Source, ln.Name)
			if err != nil {
				t.Fatal(err)
			}
			seen[cls] = true
			for _, d := range c.DataObjects {
				if d.Status == semantic.StatusUnmapped {
					t.Errorf("%s (%s): %s (%s) is unmapped: %s", cls, c.Namespace, d.Name, d.CDC, d.Reason)
				}
			}
		}
	}
	for _, cls := range classes {
		if !seen[cls] {
			t.Errorf("class %s is not in the profile", cls)
		}
	}
}

func TestSpec_7_4_MMXU_MMXN_ThreePhaseAndNonPhaseMeasurement(t *testing.T) {
	p := officialProfile(t, "default")
	expect(t, p, []want{
		{"MMXU.PNV.phsA.cVal.mag.f", semantic.QuantityVoltage, semantic.PhaseA, semantic.ReferenceLineNeutral, "", "V", "mmxu.pnv"},
		{"MMXU.PPV.phsCA", semantic.QuantityVoltage, semantic.PhaseCA, semantic.ReferenceLineLine, "", "V", "mmxu.ppv"},
		{"MMXU.TotW.mag.f", semantic.QuantityActivePower, semantic.PhaseTotal, "", "", "W", "mmxu.totw"},
		{"MMXU.HzRte.mag.f", semantic.QuantityFrequencyRate, "", "", "", "Hz/s", "mmxu.hzrte"},
		{"MMXN.Imp.cVal.ang.f", semantic.QuantityImpedance, "", "", "", "deg", "mmxn.imp"},
		{"MMXN.VolAmpr", semantic.QuantityReactivePower, "", "", "", "VAr", "mmxn.volampr"},
	})
	excluded(t, p, "statistic over the phases", "MMXU.AvAPhs", "MMXU.MaxPhVPhs", "MMXU.MinZPhs")
	noGaps(t, p, "MMXU", "MMXN")
}

func TestSpec_7_4_MMTR_MMTN_Metering(t *testing.T) {
	p := officialProfile(t, "default")
	expect(t, p, []want{
		{"MMTN.TotWh.actVal", semantic.QuantityActiveEnergy, semantic.PhaseTotal, "", "", "Wh", "mmtn.totwh"},
		{"MMTN.DmdVArh", semantic.QuantityReactiveEnergy, semantic.PhaseTotal, "", "", "VArh", "mmtn.dmdvarh"},
	})
	d, err := semantic.Describe(p, "MMTN.SupWh")
	if err != nil || d.Direction != semantic.DirectionSupply {
		t.Errorf("MMTN.SupWh: %+v %v", d, err)
	}
	// IEC TR 61850-90-6 adds the energies as measured values (MV) to
	// MMTR and MMTN, per quadrant too.
	e := officialProfile(t, "extended")
	expect(t, e, []want{
		{"MMTR.Q3VArhMeas.mag.f", semantic.QuantityReactiveEnergy, semantic.PhaseTotal, "", "", "VArh", "mmtr.q3varhmeas"},
		{"MMTN.TotVAhMeas", semantic.QuantityApparentEnergy, semantic.PhaseTotal, "", "", "VAh", "mmtn.totvahmeas"},
	})
	if d, err := semantic.Describe(e, "MMTR.DmdWhMeas"); err != nil || d.Direction != semantic.DirectionDemand {
		t.Errorf("MMTR.DmdWhMeas: %+v %v", d, err)
	}
	noGaps(t, e, "MMTR", "MMTN")
}

func TestSpec_7_4_MSQI_SequenceAndImbalance(t *testing.T) {
	p := officialProfile(t, "default")
	expect(t, p, []want{
		{"MSQI.SeqA.c1.cVal.mag.f", semantic.QuantityCurrent, "", "", "", "A", "msqi.seqa"},
		{"MSQI.SeqV.c3", semantic.QuantityVoltage, "", "", "", "V", "msqi.seqv"},
		{"MSQI.ImbA.phsB", semantic.QuantityImbalance, semantic.PhaseB, "", semantic.QuantityCurrent, "-", "msqi.imba"},
		{"MSQI.ImbPPV.phsAB", semantic.QuantityImbalance, semantic.PhaseAB, semantic.ReferenceLineLine, semantic.QuantityVoltage, "-", "msqi.imbppv"},
	})
	// SEQ components c1..c3 carry no sequence: that is the device's seqT.
	d, err := semantic.Describe(p, "MSQI.SeqA.c2")
	if err != nil || d.SubDataObject != "c2" || d.Sequence != "" || len(d.Notes) != 1 || !strings.Contains(d.Notes[0], "seqT") {
		t.Errorf("SEQ component: %+v %v", d, err)
	}
	for ref, seq := range map[string]semantic.Sequence{"MSQI.ImbNgA": semantic.SequenceNegative, "MSQI.ImbZroV": semantic.SequenceZero} {
		if d, err := semantic.Describe(p, ref); err != nil || d.Sequence != seq {
			t.Errorf("%s: %+v %v", ref, d, err)
		}
	}
	excluded(t, p, "does not state", "MSQI.DQ0Seq")
	excluded(t, p, "statistic", "MSQI.MaxImbA", "MSQI.MaxImbPPV", "MSQI.MaxImbV")
	ds, err := semantic.FindMeasurements(p, "MSQI")
	if err != nil {
		t.Fatal(err)
	}
	var seqA int
	for _, d := range ds {
		if d.DataObject == "SeqA" {
			seqA++
		}
	}
	if seqA != 3 {
		t.Errorf("SeqA expands into c1, c2, c3: %d", seqA)
	}
	noGaps(t, p, "MSQI")
}

func TestSpec_7_3_7_4_MHAI_MHAN_Harmonics(t *testing.T) {
	p := officialProfile(t, "default")
	expect(t, p, []want{
		{"MHAI.HA.phsAHar(3).cVal.mag.f", semantic.QuantityCurrent, semantic.PhaseA, "", "", "A", "mhai.ha"},
		{"MHAI.HPPV.phsBCHar(5)", semantic.QuantityVoltage, semantic.PhaseBC, semantic.ReferenceLineLine, "", "V", "mhai.hppv"},
		{"MHAI.HW.netHar(2)", semantic.QuantityActivePower, semantic.PhaseNet, "", "", "W", "mhai.hw"},
		{"MHAI.ThdA.phsC.cVal.mag.f", semantic.QuantityHarmonicDistortion, semantic.PhaseC, "", semantic.QuantityCurrent, "%", "mhai.thda"},
		{"MHAI.ThdPPV.phsAB", semantic.QuantityHarmonicDistortion, semantic.PhaseAB, semantic.ReferenceLineLine, semantic.QuantityVoltage, "%", "mhai.thdppv"},
		{"MHAI.HKFact.phsA", semantic.QuantityKFactor, semantic.PhaseA, "", semantic.QuantityCurrent, "1", "mhai.hkfact"},
		{"MHAI.Hz", semantic.QuantityFrequency, "", "", "", "Hz", "harmonics.hz"},
		{"MHAN.HaAmp.har(7).cVal.ang.f", semantic.QuantityCurrent, "", "", "", "deg", "mhan.haamp"},
		{"MHAN.TddAmp.mag.f", semantic.QuantityDemandDistortion, "", "", semantic.QuantityCurrent, "%", "mhan.tddamp"},
		{"MHAN.HaCfVol", semantic.QuantityCrestFactor, "", "", semantic.QuantityVoltage, "1", "mhan.hacfvol"},
	})
	// IEC 61850-7-3 HMV.har: element 0 is the DC component, element n
	// the n-th harmonic (numCyc 1).
	for ref, n := range map[string]int{"MHAI.HA.phsAHar(0)": 0, "MHAN.HaVol.har(11).cVal.mag.f": 11} {
		d, err := semantic.Describe(p, ref)
		if err != nil || d.Harmonic == nil || *d.Harmonic != n || !strings.Contains(strings.Join(d.Notes, " "), "numCyc") {
			t.Errorf("%s: %+v %v", ref, d, err)
		}
	}
	if d, err := semantic.Describe(p, "MHAI.HA.phsAHar"); err != nil || d.Harmonic != nil || d.SubDataObject != "phsAHar" {
		t.Errorf("harmonic array as a whole: %+v %v", d, err)
	}
	if d, err := semantic.Describe(p, "MHAN.HaAmp"); err != nil || d.Harmonic != nil || len(d.Notes) != 1 {
		t.Errorf("harmonic data object as a whole: %+v %v", d, err)
	}
	excluded(t, p, "no such quantity", "MHAI.HTiFact", "MHAI.HATm", "MHAN.HaTiFact")
	excluded(t, p, "does not state", "MHAN.HaTdFact")
	noGaps(t, p, "MHAI", "MHAN")
}

func TestSpec_7_4_MMDC_DCMeasurement(t *testing.T) {
	p := officialProfile(t, "default")
	expect(t, p, []want{
		{"MMDC.Vol.mag.f", semantic.QuantityVoltage, "", semantic.ReferencePolePole, "", "V", "mmdc.vol"},
		{"MMDC.VolNgGnd", semantic.QuantityVoltage, "", semantic.ReferenceNegativeGround, "", "V", "mmdc.volnggnd"},
		{"MMDC.RisPsGnd", semantic.QuantityResistance, "", semantic.ReferencePositiveGround, "", "ohm", "mmdc.rispsgnd"},
		{"MMDC.Watt", semantic.QuantityActivePower, "", "", "", "W", "mmdc.watt"},
	})
	noGaps(t, p, "MMDC")
}

func TestSpec_7_4_MMET_MFLW_MENV_MHYD_ProcessMeasurement(t *testing.T) {
	p := officialProfile(t, "default")
	expect(t, p, []want{
		{"MMET.EnvTmp.mag.f", semantic.QuantityTemperature, "", "", "", "degC", "mmet.envtmp"},
		{"MMET.HorWdSpd", semantic.QuantitySpeed, "", "", "", "m/s", "mmet.horwdspd"},
		{"MMET.HorWdDir", semantic.QuantityDirection, "", "", "", "deg", "mmet.horwddir"},
		{"MMET.DctInsol", semantic.QuantityIrradiance, "", "", "", "W/m²", "mmet.dctinsol"},
		{"MMET.EnvHum", semantic.QuantityHumidity, "", "", "", "%", "mmet.envhum"},
		{"MMET.EnvPres", semantic.QuantityPressure, "", "", "", "Pa", "mmet.envpres"},
		{"MMET.PoaInsol", semantic.QuantityIrradiance, "", "", "", "W/m²", "mmet.poainsol"},
		{"MFLW.FlwRte", semantic.QuantityFlowRate, "", "", "", "m³/s", "mflw.flwrte"},
		{"MFLW.VlmCnt.actVal", semantic.QuantityVolume, "", "", "", "m³", "mflw.vlmcnt"},
		{"MFLW.FanSpd", semantic.QuantityRotationalSpeed, "", "", "", "-", "mflw.fanspd"},
		{"MENV.Snd", semantic.QuantitySoundLevel, "", "", "", "dB", "menv.snd"},
		{"MHYD.HydPH", semantic.QuantityPH, "", "", "", "1", "mhyd.hydph"},
		{"MHYD.Slnt", semantic.QuantitySalinity, "", "", "", "kg/m³", "mhyd.slnt"},
		{"MHYD.FishCnt", semantic.QuantityCount, "", "", "", "-", "mhyd.fishcnt"},
	})
	excluded(t, p, "the model states neither the unit nor the basis", "MENV.CO2Em", "MENV.Dust", "MENV.O3Air")
	excluded(t, p, "the model states neither", "MMET.RnFll", "MMET.SnwCvr")
	excluded(t, p, "oktas", "MMET.CloudCvr")
	excluded(t, p, "FlwRte", "MFLW.Flw")
	noGaps(t, p, "MMET", "MFLW", "MENV", "MHYD")
	noGaps(t, officialProfile(t, "extended"), "MMET")
}

func TestSpec_7_4_SIML_SPRS_STMP_SVBR_SARC_Supervision(t *testing.T) {
	p := officialProfile(t, "default")
	expect(t, p, []want{
		{"SIML.Tmp.mag.f", semantic.QuantityTemperature, "", "", "", "degC", "linesupervision.tmp"},
		{"SIML.H2ppm", semantic.QuantityConcentration, "", "", "", "ppm", "siml.h2ppm"},
		{"SIML.H2OPap", semantic.QuantityMoisture, "", "", "", "%", "siml.h2opap"},
		{"SIML.Lev", semantic.QuantityLevel, "", "", "", "m", "siml.lev"},
		{"SPRS.Pres", semantic.QuantityPressure, "", "", "", "Pa", "sprs.pres"},
		{"SPRS.PresRte", semantic.QuantityPressureRate, "", "", "", "Pa/s", "sprs.presrte"},
		{"STMP.TmpRte", semantic.QuantityTemperatureRate, "", "", "", "K/s", "stmp.tmprte"},
		{"SVBR.Vbr", semantic.QuantityVibration, "", "", "", "-", "svbr.vbr"},
		{"SVBR.AxDsp", semantic.QuantityDisplacement, "", "", "", "m", "svbr.axdsp"},
		{"SARC.FACntRs.stVal", semantic.QuantityCount, "", "", "", "-", "sarc.facntrs"},
		{"STMP.OpCntRs.stVal", semantic.QuantityOperationCount, "", "", "", "-", "supervision.opcntrs"},
	})
	if d, _ := semantic.Describe(p, "SIML.CH4ppm"); d == nil || len(d.Notes) != 1 || !strings.Contains(d.Notes[0], "methane") {
		t.Errorf("gas notes: %+v", d)
	}
	excluded(t, p, "does not state", "SIML.CmbuGas")
	// Alarms and trips are boolean indications, not quantities.
	if _, err := semantic.Describe(p, "STMP.Alm"); !errors.Is(err, model.ErrNoMapping) {
		t.Errorf("STMP.Alm: %v", err)
	}
	noGaps(t, p, "SIML", "SPRS", "STMP", "SVBR", "SARC")
	e := officialProfile(t, "extended")
	excluded(t, e, "IEC TR 61850-90-3", "SIML.Mst", "SIML.FullGasRte")
	noGaps(t, e, "SIML")
}

func TestSpec_7_4_ZBAT_ZAXN_BatteryAndAuxiliaryNetwork(t *testing.T) {
	p := officialProfile(t, "default")
	expect(t, p, []want{
		{"ZBAT.Vol.mag.f", semantic.QuantityVoltage, "", "", "", "V", "battery.vol"},
		{"ZBAT.Amp", semantic.QuantityCurrent, "", "", "", "A", "battery.amp"},
		{"ZAXN.Vol", semantic.QuantityVoltage, "", "", "", "V", "zaxn.vol"},
		{"ZAXN.Amp", semantic.QuantityCurrent, "", "", "", "A", "zaxn.amp"},
	})
	excluded(t, p, "SIUnitKind has no unit", "ZBAT.VolChgRte")
	noGaps(t, p, "ZBAT", "ZAXN")
	// The battery charger is in IEC TR 61850-90-3 only.
	e := officialProfile(t, "extended")
	expect(t, e, []want{
		{"ZBTC.ChaA", semantic.QuantityCurrent, "", "", "", "A", "batterycharger.chaa"},
		{"ZBTC.ChaV", semantic.QuantityVoltage, "", "", "", "V", "batterycharger.chav"},
		{"ZBTC.ChaTms.stVal", semantic.QuantityDuration, "", "", "", "s", "batterycharger.chatms"},
	})
	noGaps(t, e, "ZBTC")
}

func TestSpec_7_4_XCBR_XSWI_YLTC_ATCC_PositionsAndCounters(t *testing.T) {
	p := officialProfile(t, "default")
	expect(t, p, []want{
		{"XCBR.Pos.stVal", semantic.QuantitySwitchPosition, "", "", "", "-", "switching.pos"},
		{"XSWI.Pos", semantic.QuantitySwitchPosition, "", "", "", "-", "switching.pos"},
		{"XCBR.OpCnt.stVal", semantic.QuantityOperationCount, "", "", "", "-", "switching.opcnt"},
		{"YLTC.TapPos.valWTr.posVal", semantic.QuantityTapPosition, "", "", "", "-", "yltc.tappos"},
		{"YLTC.OpCnt", semantic.QuantityOperationCount, "", "", "", "-", "yltc.opcnt"},
		{"ATCC.TapPos", semantic.QuantityTapPosition, "", "", "", "-", "atcc.tappos"},
		{"ATCC.HiTapPos.stVal", semantic.QuantityTapPosition, "", "", "", "-", "atcc.hitappos"},
		{"ATCC.CtlV.mag.f", semantic.QuantityVoltage, "", "", "", "V", "atcc.ctlv"},
		{"ATCC.PhAng", semantic.QuantityPhaseAngle, "", "", "", "deg", "atcc.phang"},
	})
	for ref, comp := range map[string]semantic.Component{
		"XCBR.Pos.stVal": semantic.ComponentState, "YLTC.TapPos.valWTr.posVal": semantic.ComponentState,
		"YLTC.TapPos.valWTr": semantic.ComponentState, "YLTC.TapPos.valWTr.transInd": semantic.ComponentOther,
		"XCBR.Pos.ctlModel": semantic.ComponentOther, "XCBR.Pos.q": semantic.ComponentQuality,
	} {
		if d, err := semantic.Describe(p, ref); err != nil || d.Component != comp || !d.UnitHint.IsZero() {
			t.Errorf("%s: %+v %v", ref, d, err)
		}
	}
	excluded(t, p, "A²t", "XCBR.SumSwARs")
	noGaps(t, p, "XCBR", "XSWI", "YLTC", "ATCC")
}

func TestSpec_7_4_KPMP_KFAN_PumpsAndFans(t *testing.T) {
	p := officialProfile(t, "default")
	expect(t, p, []want{
		{"KPMP.Spd.mag.f", semantic.QuantityRotationalSpeed, "", "", "", "-", "kpmp.spd"},
		{"KPMP.OpCtl.stVal", semantic.QuantityOperatingState, "", "", "", "-", "kpmp.opctl"},
		{"KFAN.SpdSv.instMag.f", semantic.QuantityRotationalSpeed, "", "", "", "-", "kfan.spdsv"},
		{"KFAN.OpCtl", semantic.QuantityOperatingState, "", "", "", "-", "kfan.opctl"},
	})
	// A set point is a control, not a measured value.
	if _, err := semantic.Describe(p, "KPMP.SpdSpt"); !errors.Is(err, model.ErrNoMapping) {
		t.Errorf("KPMP.SpdSpt: %v", err)
	}
	noGaps(t, p, "KPMP", "KFAN")
}

func TestSpec_7_420_DER(t *testing.T) {
	p := officialProfile(t, "default")
	expect(t, p, []want{
		{"DGEN.DEROpSt.stVal", semantic.QuantityOperatingState, "", "", "", "-", "derstate.deropst"},
		{"DGEN.GnOpSt", semantic.QuantityOperatingState, "", "", "", "-", "dgen.gnopst"},
		{"DGEN.GnEnTot.mag.f", semantic.QuantityActiveEnergy, "", "", "", "Wh", "dergenerator.gnentot"},
		{"DGEN.OpTms.stVal", semantic.QuantityDuration, "", "", "", "s", "dergenerator.optms"},
		{"DGEN.TotStrCnt", semantic.QuantityCount, "", "", "", "-", "dergenerator.totstrcnt"},
		{"DGEN.AvlUpW", semantic.QuantityActivePower, "", "", "", "W", "deractualpower.avlupw"},
		{"DSTO.SocWh", semantic.QuantityActiveEnergy, "", "", "", "Wh", "derstorage.socwh"},
		{"DSTO.UseSocPct", semantic.QuantityStateOfCharge, "", "", "", "%", "derstorage.usesocpct"},
		{"DSTO.ChaCycCnt", semantic.QuantityCount, "", "", "", "-", "derstorage.chacyccnt"},
		{"DINV.VArAvl", semantic.QuantityReactivePower, "", "", "", "VAr", "dinv.varavl"},
		{"SBAT.CelTmpMax", semantic.QuantityTemperature, "", "", "", "degC", "sbat.celtmpmax"},
		{"SINV.HeatSinkTmp", semantic.QuantityTemperature, "", "", "", "degC", "sinv.heatsinktmp"},
	})
	for _, tc := range []want{
		{"DBAT.EffAhr", semantic.QuantityCharge, "", "", "", "Ah", "dbat.effahr"},
		{"DBAT.SocEffAhrPct", semantic.QuantityStateOfCharge, "", "", "", "%", "dbat.soceffahrpct"},
		{"DBAT.Watt", semantic.QuantityActivePower, "", "", "", "W", "dbat.watt"},
	} {
		// DBAT is defined by 7-420 and IEC TR 61850-90-9 alike.
		ds, err := semantic.FindMeasurementsIn(p, model.NamespaceID{Standard: "IEC 61850-7-420", Version: "2019", Revision: "A", Release: 4}, "DBAT")
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, d := range ds {
			if d.Path == tc.ref {
				found = d.Quantity == tc.quantity && d.UnitHint.String() == tc.unit && d.Rule == tc.rule
			}
		}
		if !found {
			t.Errorf("%s not described as %s %s by %s", tc.ref, tc.quantity, tc.unit, tc.rule)
		}
	}
	excluded(t, p, "operational setting", "DGEN.VArTot", "DSTO.EffWh", "DLOD.IvarAvl")
	excluded(t, p, "the model does not state", "DGEN.GnH")
	excluded(t, p, "rating", "DGEN.CnstWMax")
	// DPVC has status and control data objects only.
	if ds, err := semantic.FindMeasurements(p, "DPVC"); err != nil || len(ds) != 0 {
		t.Errorf("DPVC: %v %v", ds, err)
	}
	noGaps(t, p, "DGEN", "DLOD", "DSTO", "DBAT", "DINV", "DPVC", "SBAT", "SINV")
	noGaps(t, officialProfile(t, "extended"), "SBAT", "DBAT", "DSTO")
}

// TestSpec_DefaultRulesMatchTheModel checks every default rule and
// exclusion against the extended profile: the class (or a class that
// inherits it) has the data object, and for a rule with the rule's CDC.
// A rule for a name the standard does not define would never apply.
func TestSpec_DefaultRulesMatchTheModel(t *testing.T) {
	p := officialProfile(t, "extended")
	type key struct{ cls, do string }
	cdcs := map[key]map[string]bool{}
	for _, ln := range p.ListLogicalNodes() {
		chain, err := p.InheritanceIn(ln.Source, ln.Name)
		if err != nil {
			t.Fatal(err)
		}
		dos, err := p.DataObjectsIn(ln.Source, ln.Name)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range chain {
			for _, do := range dos {
				k := key{c.Name, do.Name}
				if cdcs[k] == nil {
					cdcs[k] = map[string]bool{}
				}
				cdcs[k][do.Type] = true
			}
		}
	}
	for _, r := range semantic.DefaultRules() {
		if r.LNClass == semantic.AnyClass {
			continue
		}
		if !cdcs[key{r.LNClass, r.DataObject}][r.CDC] {
			t.Errorf("rule %s: no %s.%s of CDC %s in the extended profile", r.ID, r.LNClass, r.DataObject, r.CDC)
		}
	}
	for _, e := range semantic.DefaultExclusions() {
		if cdcs[key{e.LNClass, e.DataObject}] == nil {
			t.Errorf("exclusion %s.%s: not in the extended profile", e.LNClass, e.DataObject)
		}
	}
	// Every class of every embedded profile can be described.
	for _, prof := range official.Profiles() {
		pp := officialProfile(t, prof.Name)
		if _, err := semantic.Coverage(pp); err != nil {
			t.Errorf("Coverage(%s): %v", prof.Name, err)
		}
	}
}
