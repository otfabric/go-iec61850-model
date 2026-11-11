// SPDX-License-Identifier: MIT

package model

import "testing"

// IEC 61850-7-2:2010 Table 20 defines twelve data attribute FCs plus
// the service-only wildcard XX; IEC 61850-8-1:2011 Figure 4 NOTE 1
// adds the SCSM FCs CO, RP, LG, BR, GO, GS, MS and US. MDL-06.
func TestSpec_7_2_Table20_8_1_Figure4_FunctionalConstraints(t *testing.T) {
	acsi := []FunctionalConstraint{FCST, FCMX, FCSP, FCSV, FCCF, FCDC, FCSG, FCSE, FCSR, FCOR, FCBL, FCEX}
	scsm := []FunctionalConstraint{FCCO, FCRP, FCLG, FCBR, FCGO, FCGS, FCMS, FCUS}
	for _, fc := range acsi {
		if !fc.IsACSI() || fc.IsSCSM() || !fc.IsStandard() {
			t.Errorf("%s: ACSI %v SCSM %v standard %v", fc, fc.IsACSI(), fc.IsSCSM(), fc.IsStandard())
		}
	}
	for _, fc := range scsm {
		if fc.IsACSI() || !fc.IsSCSM() || !fc.IsStandard() {
			t.Errorf("%s: ACSI %v SCSM %v standard %v", fc, fc.IsACSI(), fc.IsSCSM(), fc.IsStandard())
		}
	}
	for _, fc := range []FunctionalConstraint{FCXX, "mx", "ZZ", ""} {
		if fc.IsStandard() || fc.IsACSI() || fc.IsSCSM() {
			t.Errorf("%q must not be standard", fc)
		}
	}
	if len(acsiFCs) != 12 || len(scsmFCs) != 8 {
		t.Errorf("sets: %d ACSI, %d SCSM", len(acsiFCs), len(scsmFCs))
	}
}
