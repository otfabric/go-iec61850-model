// SPDX-License-Identifier: MIT

package semantic_test

import (
	"fmt"
	"log"

	"github.com/otfabric/go-iec61850-model/catalog"
	"github.com/otfabric/go-iec61850-model/model"
	"github.com/otfabric/go-iec61850-model/nsd"
	"github.com/otfabric/go-iec61850-model/semantic"
)

// loadProfile loads the synthetic fixtures; load official packages the
// same way with nsd.LoadZIP.
func loadProfile() *catalog.Profile {
	pkg, err := nsd.LoadDir("../testdata/nsd")
	if err != nil {
		log.Fatal(err)
	}
	c, err := catalog.New(pkg)
	if err != nil {
		log.Fatal(err)
	}
	p, err := c.Profile(model.NamespaceID{Standard: "IEC 61850-7-4", Release: 1})
	if err != nil {
		log.Fatal(err)
	}
	return p
}

func ExampleDescribe() {
	d, err := semantic.Describe(loadProfile(), "MMXU.Hz")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(d.Quantity)
	fmt.Println(d.UnitHint)
	fmt.Println(d.Rule, d.Confidence)
	// Output:
	// frequency
	// Hz
	// mmxu.hz high
}

func ExampleDescribe_attribute() {
	d, err := semantic.Describe(loadProfile(), "LD0/MMXU1.PNV.phsA.cVal.ang.f[MX]")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(d.Quantity, d.Phase, d.Reference, d.Component, d.UnitHint)
	// Output:
	// voltage A line-neutral angle deg
}

func ExampleFindMeasurements() {
	ds, err := semantic.FindMeasurements(loadProfile(), "MMXU")
	if err != nil {
		log.Fatal(err)
	}
	for _, d := range ds[:8] {
		fmt.Printf("%-14s %-14s %-5s %-12s %s\n", d.DataObject+"."+d.SubDataObject, d.Quantity, d.Phase, d.Reference, d.UnitHint)
	}
	// Output:
	// TotW.          active-power   total              W
	// TotVAr.        reactive-power total              VAr
	// TotVA.         apparent-power total              VA
	// Hz.            frequency                         Hz
	// PPV.phsAB      voltage        AB    line-line    V
	// PPV.phsBC      voltage        BC    line-line    V
	// PPV.phsCA      voltage        CA    line-line    V
	// PNV.phsA       voltage        A     line-neutral V
}

func ExampleMatch() {
	ds, err := semantic.Match(loadProfile(), semantic.Criteria{Quantity: semantic.QuantityCurrent, Phase: semantic.PhaseB})
	if err != nil {
		log.Fatal(err)
	}
	for _, d := range ds {
		fmt.Println(d.Path)
	}
	// Output:
	// MMXU.A.phsB
}

func ExampleDeviceUnit() {
	// A device reports units.SIUnit = 29 and units.multiplier = 3.
	u, err := semantic.DeviceUnit(loadProfile(), 29, 3)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(u, u.Factor(), semantic.QuantityVoltage.Accepts(u))
	// Output:
	// kV 1000 true
}

func ExampleMapper_WithOverrides() {
	m, err := semantic.Default.WithOverrides(semantic.Rule{
		ID: "site.dtst.outw", LNClass: "DTST", DataObject: "OutW", CDC: "MV",
		Quantity: semantic.QuantityActivePower, Phase: semantic.PhaseTotal,
		Source: "site:plant-a", Confidence: semantic.ConfidenceMedium,
	})
	if err != nil {
		log.Fatal(err)
	}
	p, err := loadCatalogWithExtension()
	if err != nil {
		log.Fatal(err)
	}
	d, err := m.Describe(p, "DTST.OutW")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(d.Quantity, d.Basis, d.Source)
	// Output:
	// active-power override site:plant-a
}

func loadCatalogWithExtension() (*catalog.Profile, error) {
	pkg, err := nsd.LoadDir("../testdata/nsd")
	if err != nil {
		return nil, err
	}
	c, err := catalog.New(pkg)
	if err != nil {
		return nil, err
	}
	return c.Profile(
		model.NamespaceID{Standard: "OTF TEST-EXT"},
		model.NamespaceID{Standard: "IEC 61850-7-4", Release: 1},
	)
}
