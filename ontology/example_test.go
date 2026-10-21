// SPDX-License-Identifier: MIT

package ontology_test

import (
	"fmt"
	"log"

	"github.com/otfabric/go-iec61850-model/catalog"
	"github.com/otfabric/go-iec61850-model/model"
	"github.com/otfabric/go-iec61850-model/nsd"
	"github.com/otfabric/go-iec61850-model/ontology"
	"github.com/otfabric/go-iec61850-model/semantic"
)

func loadGraph() *ontology.Graph {
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
	g, err := ontology.BuildGraph(p, ontology.Options{Semantics: semantic.Default})
	if err != nil {
		log.Fatal(err)
	}
	return g
}

func ExampleGraph_Incoming() {
	g := loadGraph()
	// Which measurements represent apparent power?
	for _, r := range g.Incoming(ontology.QuantityID(semantic.QuantityApparentPower), ontology.Represents) {
		m, _ := g.Concept(r.From)
		fmt.Println(m.Name, "-", m.Label)
	}
	// Output:
	// MMXU.TotVA - Apparent power, phase total
}

func ExampleGraph_Walk() {
	g := loadGraph()
	_ = g.Walk(ontology.DataObjectID("MMXU", "PNV"), func(c ontology.Concept, depth int) bool {
		fmt.Printf("%*s%s\n", depth*2, "", c.ID)
		return depth < 2
	}, ontology.Uses, ontology.Contains)
	// Output:
	// do:MMXU.PNV
	//   cdc:WYE
	//     da:WYE.d
	//     sdo:WYE.net
	//     sdo:WYE.neut
	//     sdo:WYE.phsA
	//     sdo:WYE.phsB
	//     sdo:WYE.phsC
}
