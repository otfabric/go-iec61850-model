// SPDX-License-Identifier: MIT

package nsd

import (
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/otfabric/go-iec61850-model/model"
)

// Fixed bounds of the importer. They are not options: SECURITY.md lists
// them with the configurable limits.
const (
	// maxDiagnostics bounds the findings one document may carry. A
	// document with more is refused with model.ErrLimitExceeded.
	maxDiagnostics = 10_000
	// maxPackageDiagnostics bounds the findings of one package (an
	// archive, a directory or a single file) over all its documents.
	maxPackageDiagnostics = 100_000
	// maxQuoted bounds, in bytes, how much of a name, an attribute value
	// or a namespace URI a diagnostic path or message quotes. The
	// longest name of the embedded catalog has 119 bytes.
	maxQuoted = 256
)

// clip returns s cut to maxQuoted bytes on a rune boundary, with "..."
// appended when it was cut, so that the size of a path or message does
// not depend on the length of the names it quotes.
func clip(s string) string {
	if len(s) <= maxQuoted {
		return s
	}
	n := maxQuoted
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n] + "..."
}

// hasControl reports whether s contains a control character or a
// Unicode line or paragraph separator.
func hasControl(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) || r == '\u2028' || r == '\u2029' {
			return true
		}
	}
	return false
}

// normalizer converts the raw XML structures into the model types and
// collects diagnostics on the way.
type normalizer struct {
	source string
	strict bool
	id     model.NamespaceID
	diags  model.Diagnostics
	// dropped counts the findings beyond maxDiagnostics; they are not
	// kept and their messages are not built.
	dropped int
}

// add records a finding. The message is formatted only when the
// finding is kept.
func (n *normalizer) add(sev model.Severity, code, path, format string, args ...any) {
	if len(n.diags) >= maxDiagnostics {
		n.dropped++
		return
	}
	msg := format
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	}
	n.diags = append(n.diags, model.Diagnostic{Severity: sev, Code: code, Namespace: n.id, Path: path, Message: msg})
}

// name reports a name-bearing attribute (name, abbreviation, id,
// version, revision) that contains a control character. Names end up
// in generated source and documents, one per line.
func (n *normalizer) name(path, attr, v string) {
	if hasControl(v) {
		n.add(model.SeverityError, "invalid-name", path, "attribute %q contains a control character", attr)
	}
}

// unknownAttrs reports attributes the importer does not model. XML
// namespace declarations and xsi/xml attributes are ignored.
func (n *normalizer) unknownAttrs(path string, attrs []xml.Attr) {
	for _, a := range attrs {
		if isMetaAttr(a.Name) {
			continue
		}
		n.add(model.SeverityWarning, "unknown-attribute", path, "attribute %q is not supported and was not imported", qname(a.Name))
	}
}

func (n *normalizer) unknownElems(path string, elems []rawAny) {
	for _, e := range elems {
		n.add(model.SeverityWarning, "unknown-element", path, "element %q is not supported and was not imported", clip(e.XMLName.Local))
	}
}

func isMetaAttr(name xml.Name) bool {
	switch {
	case name.Space == "" && name.Local == "xmlns":
		return true
	case name.Space == "xmlns":
		return true
	case name.Space == "http://www.w3.org/2001/XMLSchema-instance":
		return true
	case name.Space == "http://www.w3.org/XML/1998/namespace", name.Space == "xml":
		return true
	}
	return false
}

func qname(name xml.Name) string {
	if name.Space == "" {
		return clip(name.Local)
	}
	return clip(name.Space) + ":" + clip(name.Local)
}

func (n *normalizer) boolean(path, attr, v string) bool {
	switch strings.TrimSpace(v) {
	case "", "false", "0":
		return false
	case "true", "1":
		return true
	default:
		n.add(model.SeverityError, "invalid-boolean", path, "attribute %q has invalid boolean value %q", attr, clip(v))
		return false
	}
}

func (n *normalizer) release(path, v string) int {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	r, err := strconv.Atoi(v)
	if err != nil || r < 0 {
		n.add(model.SeverityError, "invalid-release", path, "invalid release %q", clip(v))
		return 0
	}
	return r
}

func (n *normalizer) require(path, attr, v string) {
	if strings.TrimSpace(v) == "" {
		n.add(model.SeverityError, "missing-attribute", path, "required attribute %q is missing", attr)
	}
}

func doc(d rawDocAttrs) model.Doc { return model.Doc{TitleID: d.TitleID, DescID: d.DescID} }

// elemPath returns the path of a child element, e.g.
// "/NS/CDCs/CDC[MV]". The element name and the name are quoted up to
// maxQuoted bytes each.
func elemPath(parent, elem, name string) string {
	if name == "" {
		return parent + "/" + clip(elem)
	}
	return parent + "/" + clip(elem) + "[" + clip(name) + "]"
}

func (n *normalizer) namespace(raw *rawNS) *model.Namespace {
	const root = "/NS"
	n.id = model.NamespaceID{
		Standard: strings.TrimSpace(raw.ID),
		Version:  strings.TrimSpace(raw.Version),
		Revision: strings.TrimSpace(raw.Revision),
	}
	n.id.Release = n.release(root, raw.Release)
	n.require(root, "id", raw.ID)
	n.require(root, "version", raw.Version)
	n.identity(root, n.id)
	n.unknownAttrs(root, raw.Attrs)
	n.unknownElems(root, raw.Any)

	ns := &model.Namespace{
		ID:               n.id,
		PublicationStage: raw.PublicationStage,
		UMLVersion:       raw.UMLVersion,
		UMLDate:          raw.UMLDate,
		TissuesApplied:   raw.TissuesApplied,
		NamespaceType:    raw.NamespaceType,
		AppVersion:       raw.AppVersion,
		NSDVersion:       nsdVersion(raw.rawNSDVersion),
		Doc:              doc(raw.rawDocAttrs),
	}
	if c := raw.Copyright; c != nil {
		p := root + "/Copyright"
		n.container(p, c.Attrs, c.Any)
		ns.Copyright.Notice = normalizeSpace(c.Notice)
		if l := c.License; l != nil {
			n.unknownAttrs(p+"/License", l.Attrs)
			ns.Copyright.LicenseURI, ns.Copyright.LicenseKind = l.URI, l.Kind
		}
	}
	for _, c := range raw.Changes {
		p := root + "/Changes"
		n.unknownAttrs(p, c.Attrs)
		n.unknownElems(p, c.Any)
		ns.Changes = append(ns.Changes, model.NamespaceChange{
			Version: c.Version, Revision: c.Revision, Release: c.Release, Tissues: c.Tissues, ChangesID: c.ChangesID, Date: c.Date,
		})
	}

	for _, d := range raw.DependsOn {
		p := elemPath(root, "DependsOn", d.ID)
		n.require(p, "id", d.ID)
		n.unknownAttrs(p, d.Attrs)
		n.unknownElems(p, d.Any)
		if d.DependencyType != "" {
			if ns.DependencyTypes == nil {
				ns.DependencyTypes = map[string]string{}
			}
			ns.DependencyTypes[strings.TrimSpace(d.ID)] = d.DependencyType
		}
		dep := model.NamespaceID{
			Standard: strings.TrimSpace(d.ID),
			Version:  strings.TrimSpace(d.Version),
			Revision: strings.TrimSpace(d.Revision),
			Release:  n.release(p, d.Release),
		}
		n.identity(p, dep)
		ns.Dependencies = append(ns.Dependencies, dep)
	}

	if c := raw.BasicTypes; c != nil {
		p := root + "/BasicTypes"
		n.container(p, c.Attrs, c.Any)
		for _, it := range c.Items {
			ip := elemPath(p, "BasicType", it.Name)
			n.named(ip, &it)
			ns.BasicTypes = append(ns.BasicTypes, model.BasicType{Name: it.Name, Doc: doc(it.rawDocAttrs), Source: n.id})
		}
	}
	if c := raw.FunctionalConstraints; c != nil {
		p := root + "/FunctionalConstraints"
		n.container(p, c.Attrs, c.Any)
		for _, it := range c.Items {
			ip := elemPath(p, "FunctionalConstraint", it.Abbreviation)
			n.abbreviated(ip, &it)
			fc := model.FunctionalConstraintDef{Abbreviation: model.FunctionalConstraint(it.Abbreviation), Doc: doc(it.rawDocAttrs), Source: n.id}
			if as := it.ApplicableServices; as != nil {
				ap := ip + "/ApplicableServices"
				n.container(ap, as.Attrs, as.Any)
				for _, sv := range as.Services {
					n.named(elemPath(ap, "Service", sv.Name), &sv)
					fc.Services = append(fc.Services, sv.Name)
				}
				for _, cb := range as.DataSetMemberOf {
					cp := elemPath(ap, "DataSetMemberOf", cb.CB)
					n.require(cp, "cb", cb.CB)
					n.unknownAttrs(cp, cb.Attrs)
					n.unknownElems(cp, cb.Any)
					fc.DataSetMemberOf = append(fc.DataSetMemberOf, cb.CB)
				}
			}
			ns.FunctionalConstraints = append(ns.FunctionalConstraints, fc)
		}
	}
	if c := raw.TriggerOptions; c != nil {
		p := root + "/TriggerOptions"
		n.container(p, c.Attrs, c.Any)
		for _, it := range c.Items {
			ip := elemPath(p, "TriggerOption", it.Abbreviation)
			n.abbreviated(ip, &it)
			if it.ApplicableServices != nil {
				n.add(model.SeverityWarning, "unknown-element", ip, "element \"ApplicableServices\" is not supported here and was not imported")
			}
			ns.TriggerOptions = append(ns.TriggerOptions, model.TriggerOption{Abbreviation: it.Abbreviation, Doc: doc(it.rawDocAttrs), Source: n.id})
		}
	}
	if c := raw.PresenceConditions; c != nil {
		p := root + "/PresenceConditions"
		n.container(p, c.Attrs, c.Any)
		for _, it := range c.Items {
			ip := elemPath(p, "PresenceCondition", it.Name)
			n.require(ip, "name", it.Name)
			n.name(ip, "name", it.Name)
			n.unknownAttrs(ip, it.Attrs)
			n.unknownElems(ip, it.Any)
			ns.PresenceConditions = append(ns.PresenceConditions, model.PresenceConditionDef{
				Name: it.Name, Argument: it.Argument, Doc: doc(it.rawDocAttrs), Source: n.id,
			})
		}
	}
	if c := raw.Abbreviations; c != nil {
		p := root + "/Abbreviations"
		n.container(p, c.Attrs, c.Any)
		for _, it := range c.Items {
			ip := elemPath(p, "Abbreviation", it.Name)
			n.named(ip, &it)
			ns.Abbreviations = append(ns.Abbreviations, model.Abbreviation{Name: it.Name, Doc: doc(it.rawDocAttrs), Source: n.id})
		}
	}
	if c := raw.Enumerations; c != nil {
		p := root + "/Enumerations"
		n.container(p, c.Attrs, c.Any)
		for i := range c.Items {
			ns.Enumerations = append(ns.Enumerations, n.enumeration(p, &c.Items[i]))
		}
	}
	if c := raw.ConstructedAttributes; c != nil {
		p := root + "/ConstructedAttributes"
		n.container(p, c.Attrs, c.Any)
		for i := range c.Items {
			ns.ConstructedAttributes = append(ns.ConstructedAttributes, n.constructed(p, &c.Items[i]))
		}
	}
	if c := raw.CDCs; c != nil {
		p := root + "/CDCs"
		n.container(p, c.Attrs, c.Any)
		for i := range c.Items {
			ns.CDCs = append(ns.CDCs, n.cdc(p, &c.Items[i]))
		}
	}
	if c := raw.AbstractLNClasses; c != nil {
		p := root + "/AbstractLNClasses"
		n.container(p, c.Attrs, c.Any)
		for i := range c.Items {
			ns.AbstractLNClasses = append(ns.AbstractLNClasses, n.lnClass(p, "AbstractLNClass", &c.Items[i], true))
		}
	}
	if c := raw.LNClasses; c != nil {
		p := root + "/LNClasses"
		n.container(p, c.Attrs, c.Any)
		for i := range c.Abstract {
			ns.AbstractLNClasses = append(ns.AbstractLNClasses, n.lnClass(p, "AbstractLNClass", &c.Abstract[i], true))
		}
		for i := range c.Items {
			ns.LNClasses = append(ns.LNClasses, n.lnClass(p, "LNClass", &c.Items[i], false))
		}
	}
	return ns
}

func (n *normalizer) container(path string, attrs []xml.Attr, elems []rawAny) {
	n.unknownAttrs(path, attrs)
	n.unknownElems(path, elems)
}

// identity reports the control characters in the standard, version and
// revision of a namespace ID.
func (n *normalizer) identity(path string, id model.NamespaceID) {
	n.name(path, "id", id.Standard)
	n.name(path, "version", id.Version)
	n.name(path, "revision", id.Revision)
}

func (n *normalizer) named(path string, it *rawNamed) {
	n.require(path, "name", it.Name)
	n.name(path, "name", it.Name)
	n.boolean(path, "deprecated", it.Deprecated)
	n.unknownAttrs(path, it.Attrs)
	n.unknownElems(path, it.Any)
}

func (n *normalizer) abbreviated(path string, it *rawAbbreviated) {
	n.require(path, "abbreviation", it.Abbreviation)
	n.name(path, "abbreviation", it.Abbreviation)
	n.unknownAttrs(path, it.Attrs)
	n.unknownElems(path, it.Any)
}

func (n *normalizer) enumeration(parent string, raw *rawEnumeration) model.Enumeration {
	p := elemPath(parent, "Enumeration", raw.Name)
	n.require(p, "name", raw.Name)
	n.name(p, "name", raw.Name)
	n.unknownAttrs(p, raw.Attrs)
	n.unknownElems(p, raw.Any)
	e := model.Enumeration{
		Name:          raw.Name,
		InheritedFrom: raw.InheritedFrom,
		Deprecated:    n.boolean(p, "deprecated", raw.Deprecated),
		Doc:           doc(raw.rawDocAttrs),
		Source:        n.id,
	}
	for _, l := range raw.Literals {
		lp := elemPath(p, "Literal", l.Name)
		if l.Name == "" {
			lp = elemPath(p, "Literal", "@literalVal="+l.LiteralVal)
			n.add(model.SeverityInfo, "empty-literal-name", lp, "literal has an empty name")
		}
		n.name(lp, "name", l.Name)
		n.unknownAttrs(lp, l.Attrs)
		n.unknownElems(lp, l.Any)
		// An enumeration value is a 32-bit integer: the type the
		// generator declares and the width of an MMS enumerated value.
		// It is parsed with that size, so the result is the same on
		// every platform.
		v, err := strconv.ParseInt(strings.TrimSpace(l.LiteralVal), 10, 32)
		if err != nil {
			v = 0
			n.add(model.SeverityError, "invalid-literal", lp, "literalVal %q is not a 32-bit integer", clip(l.LiteralVal))
		}
		e.Literals = append(e.Literals, model.Literal{
			Name: l.Name, Value: int(v), Deprecated: n.boolean(lp, "deprecated", l.Deprecated), Doc: doc(l.rawDocAttrs),
		})
	}
	return e
}

func (n *normalizer) constructed(parent string, raw *rawConstructedAttr) model.ConstructedAttribute {
	p := elemPath(parent, "ConstructedAttribute", raw.Name)
	n.require(p, "name", raw.Name)
	n.name(p, "name", raw.Name)
	n.unknownAttrs(p, raw.Attrs)
	ca := model.ConstructedAttribute{
		Name:       raw.Name,
		Deprecated: n.boolean(p, "deprecated", raw.Deprecated),
		Doc:        doc(raw.rawDocAttrs),
		Source:     n.id,
	}
	for i := range raw.Children {
		c := &raw.Children[i]
		if c.XMLName.Local != "SubDataAttribute" {
			n.add(model.SeverityWarning, "unknown-element", p, "element %q is not supported and was not imported", clip(c.XMLName.Local))
			continue
		}
		cp := elemPath(p, "SubDataAttribute", c.Name)
		ca.SubDataAttributes = append(ca.SubDataAttributes, n.attribute(cp, c, i, false))
	}
	return ca
}

func (n *normalizer) cdc(parent string, raw *rawCDC) model.CommonDataClass {
	key := raw.Name
	if raw.Variant != "" {
		key += "/" + raw.Variant
	}
	p := elemPath(parent, "CDC", key)
	n.require(p, "name", raw.Name)
	n.name(p, "name", raw.Name)
	n.name(p, "variant", raw.Variant)
	n.unknownAttrs(p, raw.Attrs)
	c := model.CommonDataClass{
		Name:                  raw.Name,
		Variant:               raw.Variant,
		EnumParameterized:     n.boolean(p, "enumParameterized", raw.EnumParameterized),
		TypeKindParameterized: n.boolean(p, "typeKindParameterized", raw.TypeKindParameterized),
		Statistics:            n.boolean(p, "statistics", raw.Statistics),
		Deprecated:            n.boolean(p, "deprecated", raw.Deprecated),
		Informative:           n.boolean(p, "informative", raw.Informative),
		Doc:                   doc(raw.rawDocAttrs),
		Source:                n.id,
	}
	for i := range raw.Children {
		m := &raw.Children[i]
		mp := elemPath(p, m.XMLName.Local, m.Name)
		switch m.XMLName.Local {
		case "SubDataObject":
			c.SubDataObjects = append(c.SubDataObjects, n.dataObject(mp, m, i))
		case "DataAttribute":
			c.DataAttributes = append(c.DataAttributes, n.attribute(mp, m, i, true))
		case "ServiceParameter":
			n.require(mp, "name", m.Name)
			n.name(mp, "name", m.Name)
			n.unknownAttrs(mp, m.Attrs)
			n.unknownElems(mp, m.Any)
			c.ServiceParameters = append(c.ServiceParameters, model.ServiceParameter{
				Name:     m.Name,
				Type:     m.Type,
				TypeKind: model.TypeKind(m.TypeKind),
				Presence: model.PresenceCondition{Name: m.PresCond, Args: m.PresCondArgs, ArgsID: m.PresCondArgsID},
				Doc:      doc(m.rawDocAttrs),
				Position: i,
			})
		default:
			n.add(model.SeverityWarning, "unknown-element", p, "element %q is not supported and was not imported", clip(m.XMLName.Local))
		}
	}
	return c
}

func (n *normalizer) lnClass(parent, elem string, raw *rawLNClass, abstract bool) model.LogicalNodeClass {
	p := elemPath(parent, elem, raw.Name)
	n.require(p, "name", raw.Name)
	n.name(p, "name", raw.Name)
	n.unknownAttrs(p, raw.Attrs)
	n.unknownElems(p, raw.Any)
	ln := model.LogicalNodeClass{
		Name:        raw.Name,
		Base:        raw.Base,
		Abstract:    abstract,
		CanHaveLOG:  n.boolean(p, "canHaveLOG", raw.CanHaveLOG),
		IsExtension: n.boolean(p, "isExtension", raw.IsExtension),
		Deprecated:  n.boolean(p, "deprecated", raw.Deprecated),
		Informative: n.boolean(p, "informative", raw.Informative),
		Doc:         doc(raw.rawDocAttrs),
		Source:      n.id,
	}
	for i := range raw.DataObjects {
		d := &raw.DataObjects[i]
		ln.DataObjects = append(ln.DataObjects, n.dataObject(elemPath(p, "DataObject", d.Name), d, i))
	}
	return ln
}

func (n *normalizer) dataObject(p string, m *rawCDCMember, pos int) model.DataObject {
	n.require(p, "name", m.Name)
	n.name(p, "name", m.Name)
	n.require(p, "type", m.Type)
	if m.FC != "" {
		n.add(model.SeverityWarning, "unexpected-attribute", p, "attribute \"fc\" is not defined for data objects and was ignored")
	}
	if m.TypeKind != "" {
		n.add(model.SeverityWarning, "unexpected-attribute", p, "attribute \"typeKind\" is not defined for data objects and was ignored")
	}
	n.unexpectedTriggers(p, m)
	n.unknownAttrs(p, m.Attrs)
	n.unknownElems(p, m.Any)
	return model.DataObject{
		Name:               m.Name,
		Type:               m.Type,
		UnderlyingType:     m.UnderlyingType,
		UnderlyingTypeKind: model.TypeKind(m.UnderlyingTypeKind),
		Presence:           model.PresenceCondition{Name: m.PresCond, Args: m.PresCondArgs, ArgsID: m.PresCondArgsID},
		DSPresence:         model.PresenceCondition{Name: m.DSPresCond, Args: m.DSPresCondArgs, ArgsID: m.DSPresCondArgsID},
		IsArray:            n.boolean(p, "isArray", m.IsArray),
		SizeAttribute:      m.SizeAttribute,
		MinIndexAttribute:  m.MinIndexAttribute,
		MaxIndexAttribute:  m.MaxIndexAttribute,
		Transient:          n.boolean(p, "transient", m.Transient),
		Deprecated:         n.boolean(p, "deprecated", m.Deprecated),
		Informative:        n.boolean(p, "informative", m.Informative),
		Doc:                doc(m.rawDocAttrs),
		Position:           pos,
	}
}

func (n *normalizer) unexpectedTriggers(p string, m *rawCDCMember) {
	for _, a := range []struct{ name, v string }{{"dchg", m.Dchg}, {"qchg", m.Qchg}, {"dupd", m.Dupd}, {"defaultValue", m.DefaultValue}, {"minValue", m.MinValue}, {"maxValue", m.MaxValue}} {
		if a.v != "" {
			n.add(model.SeverityWarning, "unexpected-attribute", p, "attribute %q is not defined for data objects and was ignored", a.name)
		}
	}
}

func (n *normalizer) attribute(p string, m *rawCDCMember, pos int, withFC bool) model.DataAttribute {
	n.require(p, "name", m.Name)
	n.name(p, "name", m.Name)
	if withFC {
		n.require(p, "fc", m.FC)
	} else if m.FC != "" {
		n.add(model.SeverityWarning, "unexpected-attribute", p, "attribute \"fc\" is not defined for sub data attributes and was ignored")
	}
	if m.Type == "" && m.TypeKind == "" {
		n.add(model.SeverityWarning, "missing-type", p, "attribute has neither type nor typeKind")
	}
	if m.UnderlyingType != "" || m.UnderlyingTypeKind != "" {
		n.add(model.SeverityWarning, "unexpected-attribute", p, "underlying types are not defined for data attributes and were ignored")
	}
	if m.Transient != "" || m.DSPresCond != "" {
		n.add(model.SeverityWarning, "unexpected-attribute", p, "data object attributes (transient, dsPresCond) are not defined for data attributes and were ignored")
	}
	n.unknownAttrs(p, m.Attrs)
	n.unknownElems(p, m.Any)
	da := model.DataAttribute{
		Name:              m.Name,
		Type:              m.Type,
		TypeKind:          model.TypeKind(m.TypeKind),
		Presence:          model.PresenceCondition{Name: m.PresCond, Args: m.PresCondArgs, ArgsID: m.PresCondArgsID},
		IsArray:           n.boolean(p, "isArray", m.IsArray),
		SizeAttribute:     m.SizeAttribute,
		MinIndexAttribute: m.MinIndexAttribute,
		MaxIndexAttribute: m.MaxIndexAttribute,
		Dchg:              n.boolean(p, "dchg", m.Dchg),
		Qchg:              n.boolean(p, "qchg", m.Qchg),
		Dupd:              n.boolean(p, "dupd", m.Dupd),
		DefaultValue:      m.DefaultValue,
		MinValue:          m.MinValue,
		MaxValue:          m.MaxValue,
		MinIndex:          m.MinIndex,
		Deprecated:        n.boolean(p, "deprecated", m.Deprecated),
		Informative:       n.boolean(p, "informative", m.Informative),
		Doc:               doc(m.rawDocAttrs),
		Position:          pos,
	}
	if withFC {
		da.FC = model.FunctionalConstraint(m.FC)
	}
	return da
}

func (n *normalizer) idFromAttrs(attrs []xml.Attr, path string) model.NamespaceID {
	var id model.NamespaceID
	for _, a := range attrs {
		switch a.Name.Local {
		case "id":
			id.Standard = strings.TrimSpace(a.Value)
		case "version":
			id.Version = strings.TrimSpace(a.Value)
		case "revision":
			id.Revision = strings.TrimSpace(a.Value)
		case "release":
			id.Release = n.release(path, a.Value)
		}
	}
	n.identity(path, id)
	return id
}

func (n *normalizer) nsdoc(raw *rawNSDoc) *NSDoc {
	const root = "/NSDoc"
	n.id = model.NamespaceID{
		Standard: strings.TrimSpace(raw.ID),
		Version:  strings.TrimSpace(raw.Version),
		Revision: strings.TrimSpace(raw.Revision),
	}
	n.id.Release = n.release(root, raw.Release)
	n.require(root, "id", raw.ID)
	n.identity(root, n.id)
	n.unknownAttrs(root, raw.Attrs)
	n.unknownElems(root, raw.Any)
	d := &NSDoc{ID: n.id, Lang: raw.Lang, texts: make(map[string]string, len(raw.Docs))}
	for _, e := range raw.Docs {
		if e.ID == "" {
			n.add(model.SeverityWarning, "missing-attribute", root+"/Doc", "Doc element without id was ignored")
			continue
		}
		if _, dup := d.texts[e.ID]; dup {
			n.add(model.SeverityWarning, "duplicate-doc", elemPath(root, "Doc", e.ID), "duplicate Doc id; the first text is kept")
			continue
		}
		d.texts[e.ID] = normalizeSpace(e.Text)
	}
	return d
}

func normalizeSpace(s string) string { return strings.Join(strings.Fields(s), " ") }

// nsdVersion joins the NSD format version attributes, e.g. "2017B6".
func nsdVersion(v rawNSDVersion) string {
	return strings.TrimSpace(v.NSDVersion) + strings.TrimSpace(v.NSDRevision) + strings.TrimSpace(v.NSDRelease)
}
