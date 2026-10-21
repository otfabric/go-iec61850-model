// SPDX-License-Identifier: MIT

package catalog_test

import (
	"errors"
	"fmt"
	"log"

	"github.com/otfabric/go-iec61850-model/catalog"
	"github.com/otfabric/go-iec61850-model/model"
	"github.com/otfabric/go-iec61850-model/nsd"
)

func loadCatalog() *catalog.Catalog {
	pkg, err := nsd.LoadDir("../testdata/nsd")
	if err != nil {
		log.Fatal(err)
	}
	c, err := catalog.New(pkg)
	if err != nil {
		log.Fatal(err)
	}
	return c
}

func Example() {
	c := loadCatalog()

	// Two releases of IEC 61850-7-4 are loaded, so the catalog refuses to guess.
	_, err := c.LogicalNode("MMXU")
	fmt.Println(errors.Is(err, model.ErrAmbiguousReference))

	p, err := c.Profile(model.NamespaceID{Standard: "IEC 61850-7-4", Version: "2007", Revision: "B", Release: 1})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(p)

	mmxu, err := p.LogicalNode("MMXU")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(mmxu.Name, mmxu.Group(), mmxu.Group().Label())
	// Output:
	// true
	// IEC 61850-7-2:2007B1, IEC 61850-7-3:2007B1, IEC 61850-7-4:2007B1
	// MMXU M Metering and measurement
}

func ExampleProfile_DataObjects() {
	p, err := loadCatalog().Profile(model.NamespaceID{Standard: "IEC 61850-7-4", Release: 1})
	if err != nil {
		log.Fatal(err)
	}
	dos, err := p.DataObjects("MMXU")
	if err != nil {
		log.Fatal(err)
	}
	for _, do := range dos[:6] {
		fmt.Printf("%-6s %-4s %-2s %s\n", do.Name, do.Type, do.Presence, do.DeclaredIn.Name)
	}
	// Output:
	// Beh    ENS  M  DomainLN
	// Health ENS  O  DomainLN
	// NamPlt LPL  O  DomainLN
	// ClcExp SPS  O  StatisticsLN
	// TotW   MV   O  MMXU
	// TotVAr MV   O  MMXU
}

func ExampleDiff() {
	c := loadCatalog()
	r1, _ := c.Namespace(model.NamespaceID{Standard: "IEC 61850-7-4", Version: "2007", Revision: "B", Release: 1})
	r2, _ := c.Namespace(model.NamespaceID{Standard: "IEC 61850-7-4", Version: "2007", Revision: "B", Release: 2})
	for _, ch := range catalog.Diff(r1, r2) {
		fmt.Println(ch.Description)
	}
	// Output:
	// DataObject GGIO.IntIn: type changed from "INS" to "MV"
	// DataObject MMXU.TotPF added
	// DataObject MMXU.TotVA: presence changed from "O" to "M"
	// DataObject MMXU.W removed
}

func ExampleProfile_Search() {
	p, err := loadCatalog().Profile(model.NamespaceID{Standard: "IEC 61850-7-4", Release: 1})
	if err != nil {
		log.Fatal(err)
	}
	for _, r := range p.Search(catalog.Query{Kinds: []catalog.Kind{catalog.KindDataObject}, CDC: "WYE"}) {
		fmt.Println(r.Path, r.Type)
	}
	// Output:
	// MMXU.A WYE
	// MMXU.PNV WYE
	// MMXU.PhV WYE
	// MMXU.W WYE
}
