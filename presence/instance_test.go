// SPDX-License-Identifier: MIT

package presence_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/otfabric/go-iec61850-model/model"
	"github.com/otfabric/go-iec61850-model/presence"
)

const sampleInstance = `{
  "class": "CSWI",
  "present": ["Beh", "Pos"],
  "context": {"ldNs": "IEC 61850-7-4:2007B", "rootLD": false, "facts": {"MOcond(1)": true}},
  "children": {"Pos": {"present": ["stVal", "q", "t", "ctlModel"],
                       "values": {"ctlModel": "direct-with-normal-security"}}}
}`

func TestReadInstance(t *testing.T) {
	inst, err := presence.ReadInstance(strings.NewReader(sampleInstance))
	if err != nil {
		t.Fatal(err)
	}
	if inst.Class != "CSWI" || len(inst.Present) != 2 || inst.Context.RootLD == nil || *inst.Context.RootLD ||
		!inst.Context.Facts["MOcond(1)"] || inst.Children["Pos"].Values["ctlModel"] != "direct-with-normal-security" {
		t.Errorf("instance: %+v", inst)
	}
	rep, err := presence.New(defaultProfile(t)).Check(inst)
	if err != nil {
		t.Fatal(err)
	}
	if r := byPath(rep)["Mir"]; r.Requirement != presence.Required {
		t.Errorf("Mir with fact MOcond(1): %+v", r)
	}
}

func TestReadInstance_Errors(t *testing.T) {
	deep := strings.Repeat(`{"present":[],"children":{"a":`, presence.MaxDepth+1) + `{"present":[]}` + strings.Repeat("}}", presence.MaxDepth+1)
	brackets := strings.Repeat("[", 200)
	many := `{"present":[` + strings.Repeat(`"a",`, presence.MaxInstanceNames) + `"a"]}`
	big := `{"present":["` + strings.Repeat("a", presence.MaxInstanceBytes) + `"]}`
	for name, tc := range map[string]struct {
		in    string
		limit bool
	}{
		"syntax":        {`{"present": [`, false},
		"unknown field": {`{"present": [], "extra": 1}`, false},
		"trailing":      {`{"present": []} {}`, false},
		"null child":    {`{"present": [], "children": {"a": null}}`, false},
		"empty child":   {`{"present": [], "children": {"": {}}}`, false},
		"deep":          {deep, true},
		"brackets":      {brackets, true},
		"many names":    {many, true},
		"big":           {big, true},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := presence.ReadInstance(strings.NewReader(tc.in))
			if !errors.Is(err, model.ErrInvalidInstance) {
				t.Fatalf("err = %v", err)
			}
			if tc.limit != errors.Is(err, model.ErrLimitExceeded) {
				t.Errorf("limit: err = %v", err)
			}
		})
	}
	if _, err := presence.ReadInstance(nil); !errors.Is(err, model.ErrInvalidInstance) {
		t.Errorf("nil reader: %v", err)
	}
	if _, err := presence.ReadInstance(failingReader{}); !errors.Is(err, model.ErrInvalidInstance) {
		t.Errorf("failing reader: %v", err)
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("boom") }

// Strings with brackets and escaped quotes do not count as nesting.
func TestReadInstance_StringsDoNotNest(t *testing.T) {
	name := strings.Repeat(`[{\"`, 100)
	inst, err := presence.ReadInstance(strings.NewReader(`{"present": ["` + name + `"]}`))
	if err != nil || len(inst.Present) != 1 {
		t.Fatalf("%v %v", inst, err)
	}
}

func FuzzReadInstance(f *testing.F) {
	f.Add([]byte(sampleInstance))
	f.Add([]byte(`{"present":[]}`))
	f.Add([]byte(`{"present":["a.b"],"children":{"a":{"present":["b"],"children":{"b":{"present":[]}}}}}`))
	f.Add([]byte(`[[[[`))
	f.Add([]byte(`{"present":["\"[{"]}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		inst, err := presence.ReadInstance(strings.NewReader(string(data)))
		if err != nil {
			if !errors.Is(err, model.ErrInvalidInstance) {
				t.Fatalf("error does not wrap ErrInvalidInstance: %v", err)
			}
			return
		}
		// Any instance that reads evaluates without panicking.
		elems := []presence.Element{
			{Name: "a", Presence: model.PresenceCondition{Name: "AtLeastOne", Args: "1"}},
			{Name: "b", Type: "AnalogueValue", Presence: model.PresenceCondition{Name: "MO", Args: "a"}},
			{Name: "c", Presence: model.PresenceCondition{Name: "MFscaledAV"}},
			{Name: "d", Presence: model.PresenceCondition{Name: "OmultiRange", Args: "1, 9"}},
		}
		_ = presence.Evaluate("T", elems, inst)
	})
}
