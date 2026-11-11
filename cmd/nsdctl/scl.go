// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"github.com/otfabric/go-iec61850-model/model"
	"github.com/otfabric/go-iec61850-model/presence"
)

// maxSCLBytes bounds the SCL file model check reads.
const maxSCLBytes = 64 << 20

// The subset of SCL (IEC 61850-6) that model check reads: the data
// type templates and the logical nodes of the logical devices, with the
// instance values of ctlModel, hvRef, ldNs and lnNs.
type sclDoc struct {
	IEDs      []sclIED     `xml:"IED"`
	Templates sclTemplates `xml:"DataTypeTemplates"`
}

type sclIED struct {
	Name         string `xml:"name,attr"`
	AccessPoints []struct {
		Server struct {
			LDevices []sclLDevice `xml:"LDevice"`
		} `xml:"Server"`
	} `xml:"AccessPoint"`
}

type sclLDevice struct {
	Inst string  `xml:"inst,attr"`
	LN0  *sclLN  `xml:"LN0"`
	LNs  []sclLN `xml:"LN"`
}

type sclLN struct {
	Prefix  string   `xml:"prefix,attr"`
	LNClass string   `xml:"lnClass,attr"`
	Inst    string   `xml:"inst,attr"`
	LNType  string   `xml:"lnType,attr"`
	DOIs    []sclDOI `xml:"DOI"`
}

func (ln *sclLN) name() string { return ln.Prefix + ln.LNClass + ln.Inst }

type sclDOI struct {
	Name string   `xml:"name,attr"`
	DAIs []sclDAI `xml:"DAI"`
	SDIs []sclDOI `xml:"SDI"`
}

type sclDAI struct {
	Name string   `xml:"name,attr"`
	Vals []string `xml:"Val"`
}

type sclTemplates struct {
	LNodeTypes []struct {
		ID      string `xml:"id,attr"`
		LNClass string `xml:"lnClass,attr"`
		DOs     []struct {
			Name string `xml:"name,attr"`
			Type string `xml:"type,attr"`
		} `xml:"DO"`
	} `xml:"LNodeType"`
	DOTypes []struct {
		ID   string `xml:"id,attr"`
		CDC  string `xml:"cdc,attr"`
		SDOs []struct {
			Name string `xml:"name,attr"`
			Type string `xml:"type,attr"`
		} `xml:"SDO"`
		DAs []sclDA `xml:"DA"`
	} `xml:"DOType"`
	DATypes []struct {
		ID   string  `xml:"id,attr"`
		BDAs []sclDA `xml:"BDA"`
	} `xml:"DAType"`
}

type sclDA struct {
	Name  string   `xml:"name,attr"`
	BType string   `xml:"bType,attr"`
	Type  string   `xml:"type,attr"`
	Vals  []string `xml:"Val"`
}

// valueAttrs are the attribute values the presence conditions read.
var valueAttrs = map[string]bool{"ctlModel": true, "hvRef": true}

// readSCL parses an SCL document.
func readSCL(r io.Reader) (*sclDoc, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxSCLBytes+1))
	if err != nil {
		return nil, fmt.Errorf("%w: SCL: %v", model.ErrInvalidInstance, err)
	}
	if len(data) > maxSCLBytes {
		return nil, fmt.Errorf("%w: %w: SCL larger than %d bytes", model.ErrInvalidInstance, model.ErrLimitExceeded, maxSCLBytes)
	}
	var doc sclDoc
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Strict = true
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("%w: SCL: %v", model.ErrInvalidInstance, err)
	}
	return &doc, nil
}

// sclIndex builds presence instances from the data type templates.
type sclIndex struct {
	doc     *sclDoc
	lnTypes map[string]int
	doTypes map[string]int
	daTypes map[string]int
}

func newSCLIndex(doc *sclDoc) *sclIndex {
	ix := &sclIndex{doc: doc, lnTypes: map[string]int{}, doTypes: map[string]int{}, daTypes: map[string]int{}}
	for i, t := range doc.Templates.LNodeTypes {
		ix.lnTypes[t.ID] = i
	}
	for i, t := range doc.Templates.DOTypes {
		ix.doTypes[t.ID] = i
	}
	for i, t := range doc.Templates.DATypes {
		ix.daTypes[t.ID] = i
	}
	return ix
}

// lnType returns the instance of a logical node type.
func (ix *sclIndex) lnType(id string) (*presence.Instance, error) {
	i, ok := ix.lnTypes[id]
	if !ok {
		return nil, fmt.Errorf("%w: SCL has no LNodeType %q", model.ErrInvalidInstance, id)
	}
	t := ix.doc.Templates.LNodeTypes[i]
	inst := &presence.Instance{Class: t.LNClass, Present: []string{}, Children: map[string]*presence.Instance{}}
	for _, do := range t.DOs {
		inst.Present = append(inst.Present, do.Name)
		child, err := ix.doType(do.Type, 1)
		if err != nil {
			return nil, err
		}
		inst.Children[do.Name] = child
	}
	return inst, nil
}

func (ix *sclIndex) doType(id string, depth int) (*presence.Instance, error) {
	if depth > presence.MaxDepth {
		return nil, fmt.Errorf("%w: %w: SCL types nested deeper than %d", model.ErrInvalidInstance, model.ErrLimitExceeded, presence.MaxDepth)
	}
	i, ok := ix.doTypes[id]
	if !ok {
		return nil, fmt.Errorf("%w: SCL has no DOType %q", model.ErrInvalidInstance, id)
	}
	t := ix.doc.Templates.DOTypes[i]
	inst := &presence.Instance{Class: t.CDC, Present: []string{}}
	for _, s := range t.SDOs {
		child, err := ix.doType(s.Type, depth+1)
		if err != nil {
			return nil, err
		}
		inst.Present = append(inst.Present, s.Name)
		addChild(inst, s.Name, child)
	}
	if err := ix.attributes(inst, t.DAs, depth); err != nil {
		return nil, err
	}
	return inst, nil
}

func (ix *sclIndex) attributes(inst *presence.Instance, das []sclDA, depth int) error {
	for _, da := range das {
		inst.Present = append(inst.Present, da.Name)
		if valueAttrs[da.Name] && len(da.Vals) > 0 {
			setValue(inst, da.Name, da.Vals[0])
		}
		if da.BType != "Struct" {
			continue
		}
		child, err := ix.daType(da.Type, depth+1)
		if err != nil {
			return err
		}
		addChild(inst, da.Name, child)
	}
	return nil
}

func (ix *sclIndex) daType(id string, depth int) (*presence.Instance, error) {
	if depth > presence.MaxDepth {
		return nil, fmt.Errorf("%w: %w: SCL types nested deeper than %d", model.ErrInvalidInstance, model.ErrLimitExceeded, presence.MaxDepth)
	}
	i, ok := ix.daTypes[id]
	if !ok {
		return nil, fmt.Errorf("%w: SCL has no DAType %q", model.ErrInvalidInstance, id)
	}
	inst := &presence.Instance{Present: []string{}}
	return inst, ix.attributes(inst, ix.doc.Templates.DATypes[i].BDAs, depth)
}

func addChild(inst *presence.Instance, name string, child *presence.Instance) {
	if inst.Children == nil {
		inst.Children = map[string]*presence.Instance{}
	}
	child.Class = ""
	inst.Children[name] = child
}

func setValue(inst *presence.Instance, name, v string) {
	if inst.Values == nil {
		inst.Values = map[string]string{}
	}
	inst.Values[name] = strings.TrimSpace(v)
}

// ln returns the instance of a logical node "LD/LN" of the SCL (LN is
// "LLN0" or prefix, class and instance, e.g. "Q0CSWI1"), with the
// ctlModel and hvRef values of its DOIs and the ldNs and lnNs of the
// name plates.
func (ix *sclIndex) ln(ref string) (*presence.Instance, error) {
	ldInst, lnName, ok := strings.Cut(ref, "/")
	if !ok || ldInst == "" || lnName == "" {
		return nil, fmt.Errorf("%w: --ln %q is not LD/LN", model.ErrInvalidInstance, ref)
	}
	var matches []*presence.Instance
	for _, ied := range ix.doc.IEDs {
		for _, ap := range ied.AccessPoints {
			for _, ld := range ap.Server.LDevices {
				if ld.Inst != ldInst && ied.Name+ld.Inst != ldInst {
					continue
				}
				ln := findLN(&ld, lnName)
				if ln == nil {
					continue
				}
				inst, err := ix.lnType(ln.LNType)
				if err != nil {
					return nil, err
				}
				applyDOIs(inst, ln.DOIs)
				if ld.LN0 != nil {
					if v := doiValue(ld.LN0.DOIs, "NamPlt", "ldNs"); v != "" {
						inst.Context.LDNamespace = v
					}
				}
				if v := doiValue(ln.DOIs, "NamPlt", "lnNs"); v != "" {
					inst.Context.LNNamespace = v
				}
				matches = append(matches, inst)
			}
		}
	}
	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("%w: SCL has no logical node %q", model.ErrInvalidInstance, ref)
	case 1:
		return matches[0], nil
	}
	return nil, fmt.Errorf("%w: %d IEDs have a logical node %q; prefix the LD with the IED name", model.ErrInvalidInstance, len(matches), ref)
}

func findLN(ld *sclLDevice, name string) *sclLN {
	if name == "LLN0" && ld.LN0 != nil {
		return ld.LN0
	}
	for i := range ld.LNs {
		if ld.LNs[i].name() == name {
			return &ld.LNs[i]
		}
	}
	return nil
}

// applyDOIs sets the instance values of the data objects.
func applyDOIs(inst *presence.Instance, dois []sclDOI) {
	for _, doi := range dois {
		child := inst.Children[doi.Name]
		if child == nil {
			continue
		}
		for _, dai := range doi.DAIs {
			if valueAttrs[dai.Name] && len(dai.Vals) > 0 {
				setValue(child, dai.Name, dai.Vals[0])
			}
		}
	}
}

func doiValue(dois []sclDOI, do, da string) string {
	for _, doi := range dois {
		if doi.Name != do {
			continue
		}
		for _, dai := range doi.DAIs {
			if dai.Name == da && len(dai.Vals) > 0 {
				return strings.TrimSpace(dai.Vals[0])
			}
		}
	}
	return ""
}
