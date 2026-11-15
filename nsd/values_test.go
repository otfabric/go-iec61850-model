// SPDX-License-Identifier: MIT

package nsd_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/otfabric/go-iec61850-model/model"
	"github.com/otfabric/go-iec61850-model/nsd"
)

// An enumeration value is a 32-bit integer: the type the generator
// declares and the width of an MMS enumerated value. The same document
// is accepted or refused on every platform.
func TestLiteralValueWidth(t *testing.T) {
	for _, tc := range []struct {
		val  string
		want int
		ok   bool
	}{
		{"0", 0, true},
		{"1", 1, true},
		{"-1", -1, true},
		{" 7 ", 7, true},
		{"+7", 7, true},
		{"2147483646", 2147483646, true},
		{"2147483647", 2147483647, true}, // math.MaxInt32
		{"2147483648", 0, false},
		{"-2147483647", -2147483647, true},
		{"-2147483648", -2147483648, true}, // math.MinInt32
		{"-2147483649", 0, false},
		{"4294967295", 0, false}, // math.MaxUint32
		{"4294967296", 0, false},
		{"9223372036854775807", 0, false}, // math.MaxInt64
		{"9223372036854775808", 0, false},
		{"-9223372036854775808", 0, false}, // math.MinInt64
		{"-9223372036854775809", 0, false},
		{"", 0, false},
		{"1.0", 0, false},
		{"0x10", 0, false},
		{"1e3", 0, false},
	} {
		doc, err := parse(t, nsOpen+`<Enumerations><Enumeration name="E"><Literal name="a" literalVal="`+tc.val+`"/></Enumeration></Enumerations></NS>`)
		if !tc.ok {
			var ve *model.ValidationError
			if !errors.As(err, &ve) || len(ve.Diagnostics) != 1 || ve.Diagnostics[0].Code != "invalid-literal" ||
				ve.Diagnostics[0].Message != fmt.Sprintf("literalVal %q is not a 32-bit integer", tc.val) {
				t.Errorf("literalVal %q: error = %v, want invalid-literal", tc.val, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("literalVal %q: %v", tc.val, err)
			continue
		}
		if got := doc.Namespace.Enumerations[0].Literals[0].Value; got != tc.want {
			t.Errorf("literalVal %q = %d, want %d", tc.val, got, tc.want)
		}
	}
}

// A name-bearing attribute with a control character is an error: names
// are written one per line into generated source and documents.
func TestNamesWithControlCharacters(t *testing.T) {
	// Each document has %s in one name-bearing attribute.
	docs := map[string]string{
		"NS id":                `<NS xmlns="http://www.iec.ch/61850/2016/NSD" id="T%sU" version="2026"/>`,
		"NS version":           `<NS xmlns="http://www.iec.ch/61850/2016/NSD" id="T" version="20%s26"/>`,
		"NS revision":          `<NS xmlns="http://www.iec.ch/61850/2016/NSD" id="T" version="2026" revision="A%sB"/>`,
		"DependsOn id":         nsOpen + `<DependsOn id="D%sE" version="1"/></NS>`,
		"DependsOn version":    nsOpen + `<DependsOn id="D" version="1%s2"/></NS>`,
		"DependsOn revision":   nsOpen + `<DependsOn id="D" version="1" revision="A%sB"/></NS>`,
		"BasicType":            nsOpen + `<BasicTypes><BasicType name="B%sC"/></BasicTypes></NS>`,
		"FunctionalConstraint": nsOpen + `<FunctionalConstraints><FunctionalConstraint abbreviation="S%sT"/></FunctionalConstraints></NS>`,
		"Service":              nsOpen + `<FunctionalConstraints><FunctionalConstraint abbreviation="ST"><ApplicableServices><Service name="G%sV"/></ApplicableServices></FunctionalConstraint></FunctionalConstraints></NS>`,
		"TriggerOption":        nsOpen + `<TriggerOptions><TriggerOption abbreviation="d%sg"/></TriggerOptions></NS>`,
		"PresenceCondition":    nsOpen + `<PresenceConditions><PresenceCondition name="M%sO"/></PresenceConditions></NS>`,
		"Abbreviation":         nsOpen + `<Abbreviations><Abbreviation name="A%sB"/></Abbreviations></NS>`,
		"Enumeration":          nsOpen + `<Enumerations><Enumeration name="E%sK"/></Enumerations></NS>`,
		"Literal":              nsOpen + `<Enumerations><Enumeration name="E"><Literal name="l%sm" literalVal="1"/></Enumeration></Enumerations></NS>`,
		"ConstructedAttribute": nsOpen + `<ConstructedAttributes><ConstructedAttribute name="C%sA"/></ConstructedAttributes></NS>`,
		"SubDataAttribute":     nsOpen + `<ConstructedAttributes><ConstructedAttribute name="C"><SubDataAttribute name="s%st" type="B" typeKind="BASIC"/></ConstructedAttribute></ConstructedAttributes></NS>`,
		"CDC":                  nsOpen + `<CDCs><CDC name="X%sY"/></CDCs></NS>`,
		"CDC variant":          nsOpen + `<CDCs><CDC name="X" variant="S%sP"/></CDCs></NS>`,
		"SubDataObject":        nsOpen + `<CDCs><CDC name="X"><SubDataObject name="p%sq" type="X"/></CDC></CDCs></NS>`,
		"DataAttribute":        nsOpen + `<CDCs><CDC name="X"><DataAttribute name="a%sb" fc="ST" type="B" typeKind="BASIC"/></CDC></CDCs></NS>`,
		"ServiceParameter":     nsOpen + `<CDCs><CDC name="X"><ServiceParameter name="c%sd" type="B" typeKind="BASIC"/></CDC></CDCs></NS>`,
		"LNClass":              nsOpen + `<LNClasses><LNClass name="L%sN"/></LNClasses></NS>`,
		"AbstractLNClass":      nsOpen + `<LNClasses><AbstractLNClass name="L%sN"/></LNClasses></NS>`,
		"DataObject":           nsOpen + `<LNClasses><LNClass name="L"><DataObject name="D%sO" type="X"/></LNClass></LNClasses></NS>`,
		"NSDoc id":             `<NSDoc xmlns="http://www.iec.ch/61850/2016/NSD" id="T%sU" version="1" lang="en"/>`,
		"NSDoc version":        `<NSDoc xmlns="http://www.iec.ch/61850/2016/NSD" id="T" version="1%s2" lang="en"/>`,
		"ServiceNS id":         `<ServiceNS xmlns="http://www.iec.ch/61850/2016/NSD" id="S%sT" version="2003"/>`,
		"ServiceNS revision":   `<ServiceNS xmlns="http://www.iec.ch/61850/2016/NSD" id="S" version="2003" revision="A%sB"/>`,
	}
	controls := map[string]string{
		"a line feed":                  "\n",
		"a line feed reference":        "&#10;",
		"a carriage return reference":  "&#13;",
		"a tab":                        "\t",
		"a tab reference":              "&#9;",
		"a next line character":        "\u0085",
		"a delete character reference": "&#127;",
		"a line separator":             "\u2028",
		"a paragraph separator":        "&#x2029;",
	}
	for where, doc := range docs {
		if _, err := parse(t, fmt.Sprintf(doc, "")); err != nil {
			t.Errorf("%s without a control character: %v", where, err)
		}
		// Other characters are names as before: a space, a non-breaking
		// space, letters outside ASCII.
		if _, err := parse(t, fmt.Sprintf(doc, " \u00a0\u00e9\u4e16")); err != nil {
			t.Errorf("%s with other characters: %v", where, err)
		}
		for what, c := range controls {
			_, err := parse(t, fmt.Sprintf(doc, c))
			var ve *model.ValidationError
			if !errors.As(err, &ve) || !errors.Is(err, model.ErrInvalidNSD) {
				t.Errorf("%s with %s: error = %v, want a validation error", where, what, err)
				continue
			}
			found := false
			for _, d := range ve.Diagnostics {
				found = found || d.Code == "invalid-name" && d.Severity == model.SeverityError && strings.Contains(d.Message, "contains a control character")
			}
			if !found {
				t.Errorf("%s with %s: diagnostics %v, want invalid-name", where, what, ve.Diagnostics)
			}
		}
	}
	// White space around an ID is removed before it is checked, as before.
	doc, err := parse(t, "<NS xmlns=\"http://www.iec.ch/61850/2016/NSD\" id=\"\n T \t\" version=\" 2026\n\" revision=\"\tA \"/>")
	if err != nil || doc.ID != (model.NamespaceID{Standard: "T", Version: "2026", Revision: "A"}) {
		t.Errorf("ID with white space around it: %v, error %v", doc, err)
	}
}

// The zero and nil values of the doc set types.
func TestDocSetZeroAndNilValues(t *testing.T) {
	var zero nsd.DocSet
	if zero.Len() != 0 || zero.Lang() != "" || zero.Source() != "" || len(zero.Keys()) != 0 {
		t.Errorf("zero set %+v", &zero)
	}
	if _, ok := zero.Text("LN:MMXU"); ok {
		t.Error("zero set has a text")
	}
	if err := zero.Set("LN:MMXU#title", " Measurement  class "); err != nil {
		t.Fatalf("Set on the zero value: %v", err)
	}
	if got, ok := zero.Text("LN:MMXU"); !ok || got.Title != "Measurement class" || zero.Len() != 1 {
		t.Errorf("text %+v after Set on the zero value", got)
	}
	if err := zero.Set("nokind", "x"); !errors.Is(err, model.ErrInvalidNSD) {
		t.Errorf("malformed key: error = %v", err)
	}
	data, err := nsd.EncodeDocSets(&zero, &nsd.DocSet{})
	if err != nil || !strings.Contains(string(data), `"texts": {}`) {
		t.Fatalf("encode zero sets: %s, %v", data, err)
	}
	if sets, err := nsd.ReadDocSets(data); err != nil || len(sets) != 2 || sets[0].Len() != 1 || sets[1].Len() != 0 {
		t.Errorf("round trip of zero sets: %v", err)
	}

	var none *nsd.DocSet
	if none.Len() != 0 || none.Lang() != "" || none.Source() != "" || len(none.Keys()) != 0 {
		t.Error("nil set is not empty")
	}
	if _, ok := none.Text("LN:MMXU"); ok {
		t.Error("nil set has a text")
	}
	if err := none.Set("LN:MMXU", "x"); !errors.Is(err, model.ErrInvalidNSD) {
		t.Errorf("Set on a nil set: error = %v, want ErrInvalidNSD", err)
	}

	// SelectDocSet returns nil for a language without a set; what a
	// caller does with it next fails with an error or reports nothing.
	sets := []*nsd.DocSet{nsd.NewDocSet("en"), nil}
	if s := nsd.SelectDocSet(sets, "en"); s == nil || s.Lang() != "en" {
		t.Error("SelectDocSet(en)")
	}
	missing := nsd.SelectDocSet(sets, "fr")
	if missing != nil || missing.Len() != 0 {
		t.Errorf("SelectDocSet(fr) = %v", missing)
	}
	if _, err := nsd.EncodeDocSets(missing); !errors.Is(err, model.ErrInvalidNSD) || !strings.Contains(err.Error(), "doc set 0 is nil") {
		t.Errorf("EncodeDocSets(nil): error = %v, want ErrInvalidNSD", err)
	}
	if _, err := nsd.EncodeDocSets(nsd.NewDocSet("en"), nil); !errors.Is(err, model.ErrInvalidNSD) || !strings.Contains(err.Error(), "doc set 1 is nil") {
		t.Errorf("EncodeDocSets(en, nil): error = %v, want ErrInvalidNSD", err)
	}
	if data, err := nsd.EncodeDocSets(); err != nil || !strings.Contains(string(data), `"sets": null`) {
		t.Errorf("EncodeDocSets(): %s, %v", data, err)
	}
	if n := nsd.ApplyDocSet(&model.Namespace{}, missing); n != 0 {
		t.Errorf("ApplyDocSet(nil set) = %d", n)
	}

	var noDoc *nsd.NSDoc
	if s, err := nsd.DocSetFromNSDoc(noDoc); s != nil || !errors.Is(err, model.ErrInvalidNSD) {
		t.Errorf("DocSetFromNSDoc(nil) = %v, %v, want ErrInvalidNSD", s, err)
	}
	if _, ok := noDoc.Text("id"); ok || noDoc.Len() != 0 || len(noDoc.IDs()) != 0 || noDoc.Matches(model.NamespaceID{}) {
		t.Error("nil NSDoc is not empty")
	}
	var zeroDoc nsd.NSDoc
	if _, ok := zeroDoc.Text("id"); ok || zeroDoc.Len() != 0 || len(zeroDoc.IDs()) != 0 || !zeroDoc.Matches(model.NamespaceID{}) {
		t.Error("zero NSDoc is not empty")
	}
	if s, err := nsd.DocSetFromNSDoc(&zeroDoc); err != nil || s.Len() != 0 {
		t.Errorf("DocSetFromNSDoc(zero) = %v, %v", s, err)
	}
	if n := nsd.ApplyDoc(&model.Namespace{}, noDoc); n != 0 {
		t.Errorf("ApplyDoc(nil) = %d", n)
	}
}
