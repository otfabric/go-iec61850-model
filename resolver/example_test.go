// SPDX-License-Identifier: MIT

package resolver_test

import (
	"errors"
	"fmt"
	"log"

	"github.com/otfabric/go-iec61850-model/catalog"
	"github.com/otfabric/go-iec61850-model/model"
	"github.com/otfabric/go-iec61850-model/nsd"
	"github.com/otfabric/go-iec61850-model/reference"
	"github.com/otfabric/go-iec61850-model/resolver"
)

// The examples load the synthetic fixtures of testdata/nsd. With
// official packages, load them the same way with nsd.LoadZIP.
func loadProfile() *catalog.Profile {
	pkg, err := nsd.LoadDir("../testdata/nsd")
	if err != nil {
		log.Fatal(err)
	}
	c, err := catalog.New(pkg)
	if err != nil {
		log.Fatal(err)
	}
	// Two releases of IEC 61850-7-4 are loaded: select one explicitly.
	id, err := model.ParseNamespaceID("IEC 61850-7-4:2007B1")
	if err != nil {
		log.Fatal(err)
	}
	p, err := c.Profile(id)
	if err != nil {
		log.Fatal(err)
	}
	return p
}

func Example() {
	r := resolver.New(loadProfile())

	ref, err := reference.Parse("LD0/MMXU1.Hz.mag.f[MX]")
	if err != nil {
		log.Fatal(err)
	}
	res, err := r.Resolve(ref)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(res.DataObjectPath())
	fmt.Println(res.AttributePath())
	fmt.Println(res.Type())
	// Output:
	// Hz
	// mag.f
	// FLOAT32
}

func ExampleResolver_ResolveString() {
	r := resolver.New(loadProfile())
	res, err := r.ResolveString("MMXU.PNV.phsA.cVal.mag.f")
	if err != nil {
		log.Fatal(err)
	}
	for _, s := range res.Steps {
		fmt.Printf("%-16s %-6s %-14s %s\n", s.Kind, s.Name, s.Type, s.Source)
	}
	// Output:
	// LogicalNode      MMXU   MMXU           IEC 61850-7-4:2007B1
	// DataObject       PNV    WYE            IEC 61850-7-4:2007B1
	// SubDataObject    phsA   CMV            IEC 61850-7-3:2007B1
	// DataAttribute    cVal   Vector         IEC 61850-7-3:2007B1
	// SubDataAttribute mag    AnalogueValue  IEC 61850-7-3:2007B1
	// SubDataAttribute f      FLOAT32        IEC 61850-7-3:2007B1
}

func ExampleResolver_Leaves() {
	r := resolver.New(loadProfile())
	leaves, err := r.Leaves("MMXU", []string{"Hz"}, model.FCMX)
	if err != nil {
		log.Fatal(err)
	}
	for _, l := range leaves {
		fmt.Printf("%s [%s] %s\n", l.Path(), l.FC(), l.Type())
	}
	// Output:
	// MMXU.Hz.instMag.i [MX] INT32
	// MMXU.Hz.instMag.f [MX] FLOAT32
	// MMXU.Hz.mag.i [MX] INT32
	// MMXU.Hz.mag.f [MX] FLOAT32
	// MMXU.Hz.q [MX] Quality
	// MMXU.Hz.t [MX] Timestamp
}

func ExampleResolver_ResolvePath_errors() {
	r := resolver.New(loadProfile())
	_, err := r.ResolveString("MMXU.Hz.mag.f[ST]")
	fmt.Println(errors.Is(err, model.ErrUnknownAttribute))
	var re *model.ResolutionError
	if errors.As(err, &re) {
		fmt.Println(re.Path, re.Expected, re.Actual)
	}
	// Output:
	// true
	// MMXU.Hz.mag ST MX
}
