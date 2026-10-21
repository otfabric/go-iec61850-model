// SPDX-License-Identifier: MIT

package nsd

import "encoding/xml"

// The raw types mirror the NSD XML structure of IEC 61850-7-7. Every
// type collects attributes and child elements it does not know in
// Attrs and Any, so the normalizer can report them instead of dropping
// them silently.

type rawAny struct {
	XMLName xml.Name
	Attrs   []xml.Attr `xml:",any,attr"`
}

type rawDocAttrs struct {
	TitleID string `xml:"titleID,attr"`
	DescID  string `xml:"descID,attr"`
}

type rawNS struct {
	XMLName          xml.Name
	ID               string `xml:"id,attr"`
	Version          string `xml:"version,attr"`
	Revision         string `xml:"revision,attr"`
	Release          string `xml:"release,attr"`
	PublicationStage string `xml:"publicationStage,attr"`
	UMLVersion       string `xml:"umlVersion,attr"`
	UMLDate          string `xml:"umlDate,attr"`
	TissuesApplied   string `xml:"tissuesApplied,attr"`
	NamespaceType    string `xml:"namespaceType,attr"`
	AppVersion       string `xml:"appVersion,attr"`
	rawNSDVersion
	rawDocAttrs
	Attrs []xml.Attr `xml:",any,attr"`

	Changes               []rawChanges           `xml:"Changes"`
	Copyright             *rawCopyright          `xml:"Copyright"`
	DependsOn             []rawDependsOn         `xml:"DependsOn"`
	BasicTypes            *rawBasicTypes         `xml:"BasicTypes"`
	FunctionalConstraints *rawFCs                `xml:"FunctionalConstraints"`
	TriggerOptions        *rawTriggerOptions     `xml:"TriggerOptions"`
	PresenceConditions    *rawPresenceConditions `xml:"PresenceConditions"`
	Abbreviations         *rawAbbreviations      `xml:"Abbreviations"`
	Enumerations          *rawEnumerations       `xml:"Enumerations"`
	ConstructedAttributes *rawConstructedAttrs   `xml:"ConstructedAttributes"`
	CDCs                  *rawCDCs               `xml:"CDCs"`
	AbstractLNClasses     *rawAbstractLNClasses  `xml:"AbstractLNClasses"`
	LNClasses             *rawLNClasses          `xml:"LNClasses"`
	Any                   []rawAny               `xml:",any"`
}

type rawCopyright struct {
	Attrs   []xml.Attr  `xml:",any,attr"`
	Notice  string      `xml:"Notice"`
	License *rawLicense `xml:"License"`
	Any     []rawAny    `xml:",any"`
}

type rawLicense struct {
	URI   string     `xml:"uri,attr"`
	Kind  string     `xml:"kind,attr"`
	Attrs []xml.Attr `xml:",any,attr"`
}

// rawNSDVersion is the version of the NSD format a document declares.
type rawNSDVersion struct {
	NSDVersion  string `xml:"nsdVersion,attr"`
	NSDRevision string `xml:"nsdRevision,attr"`
	NSDRelease  string `xml:"nsdRelease,attr"`
}

type rawChanges struct {
	Version   string     `xml:"version,attr"`
	Revision  string     `xml:"revision,attr"`
	Release   string     `xml:"release,attr"`
	Tissues   string     `xml:"tissues,attr"`
	ChangesID string     `xml:"changesID,attr"`
	Date      string     `xml:"date,attr"`
	Attrs     []xml.Attr `xml:",any,attr"`
	Any       []rawAny   `xml:",any"`
}

type rawDependsOn struct {
	DependencyType   string     `xml:"dependencyType,attr"`
	ID               string     `xml:"id,attr"`
	Version          string     `xml:"version,attr"`
	Revision         string     `xml:"revision,attr"`
	Release          string     `xml:"release,attr"`
	PublicationStage string     `xml:"publicationStage,attr"`
	Attrs            []xml.Attr `xml:",any,attr"`
	Any              []rawAny   `xml:",any"`
}

type rawContainer struct {
	Attrs []xml.Attr `xml:",any,attr"`
}

type rawBasicTypes struct {
	rawContainer
	Items []rawNamed `xml:"BasicType"`
	Any   []rawAny   `xml:",any"`
}

type rawNamed struct {
	Name       string `xml:"name,attr"`
	Deprecated string `xml:"deprecated,attr"`
	rawDocAttrs
	Attrs []xml.Attr `xml:",any,attr"`
	Any   []rawAny   `xml:",any"`
}

type rawFCs struct {
	rawContainer
	Items []rawAbbreviated `xml:"FunctionalConstraint"`
	Any   []rawAny         `xml:",any"`
}

type rawTriggerOptions struct {
	rawContainer
	Items []rawAbbreviated `xml:"TriggerOption"`
	Any   []rawAny         `xml:",any"`
}

type rawAbbreviated struct {
	Abbreviation string `xml:"abbreviation,attr"`
	rawDocAttrs
	Attrs              []xml.Attr             `xml:",any,attr"`
	ApplicableServices *rawApplicableServices `xml:"ApplicableServices"`
	Any                []rawAny               `xml:",any"`
}

type rawApplicableServices struct {
	Attrs           []xml.Attr     `xml:",any,attr"`
	Services        []rawNamed     `xml:"Service"`
	DataSetMemberOf []rawDataSetCB `xml:"DataSetMemberOf"`
	Any             []rawAny       `xml:",any"`
}

type rawDataSetCB struct {
	CB    string     `xml:"cb,attr"`
	Attrs []xml.Attr `xml:",any,attr"`
	Any   []rawAny   `xml:",any"`
}

type rawPresenceConditions struct {
	rawContainer
	Items []rawPresenceCondition `xml:"PresenceCondition"`
	Any   []rawAny               `xml:",any"`
}

type rawPresenceCondition struct {
	Name     string `xml:"name,attr"`
	Argument string `xml:"argument,attr"`
	rawDocAttrs
	Attrs []xml.Attr `xml:",any,attr"`
	Any   []rawAny   `xml:",any"`
}

type rawAbbreviations struct {
	rawContainer
	Items []rawNamed `xml:"Abbreviation"`
	Any   []rawAny   `xml:",any"`
}

type rawEnumerations struct {
	rawContainer
	Items []rawEnumeration `xml:"Enumeration"`
	Any   []rawAny         `xml:",any"`
}

type rawEnumeration struct {
	Name          string `xml:"name,attr"`
	InheritedFrom string `xml:"inheritedFrom,attr"`
	Deprecated    string `xml:"deprecated,attr"`
	rawDocAttrs
	Attrs    []xml.Attr   `xml:",any,attr"`
	Literals []rawLiteral `xml:"Literal"`
	Any      []rawAny     `xml:",any"`
}

type rawLiteral struct {
	Name       string `xml:"name,attr"`
	LiteralVal string `xml:"literalVal,attr"`
	Deprecated string `xml:"deprecated,attr"`
	rawDocAttrs
	Attrs []xml.Attr `xml:",any,attr"`
	Any   []rawAny   `xml:",any"`
}

type rawConstructedAttrs struct {
	rawContainer
	Items []rawConstructedAttr `xml:"ConstructedAttribute"`
	Any   []rawAny             `xml:",any"`
}

type rawConstructedAttr struct {
	Name       string `xml:"name,attr"`
	Deprecated string `xml:"deprecated,attr"`
	rawDocAttrs
	Attrs    []xml.Attr     `xml:",any,attr"`
	Children []rawCDCMember `xml:",any"`
}

type rawCDCs struct {
	rawContainer
	Items []rawCDC `xml:"CDC"`
	Any   []rawAny `xml:",any"`
}

type rawCDC struct {
	Name                  string `xml:"name,attr"`
	Variant               string `xml:"variant,attr"`
	EnumParameterized     string `xml:"enumParameterized,attr"`
	TypeKindParameterized string `xml:"typeKindParameterized,attr"`
	Statistics            string `xml:"statistics,attr"`
	Deprecated            string `xml:"deprecated,attr"`
	Informative           string `xml:"informative,attr"`
	rawDocAttrs
	Attrs []xml.Attr `xml:",any,attr"`
	// Children keeps sub data objects, data attributes and service
	// parameters in document order.
	Children []rawCDCMember `xml:",any"`
}

// rawCDCMember is any child of a CDC or constructed attribute. The
// element name decides how it is interpreted; the attribute set is the
// union of those of SubDataObject, DataAttribute, SubDataAttribute and
// ServiceParameter.
type rawCDCMember struct {
	XMLName            xml.Name
	Name               string `xml:"name,attr"`
	Type               string `xml:"type,attr"`
	TypeKind           string `xml:"typeKind,attr"`
	FC                 string `xml:"fc,attr"`
	UnderlyingType     string `xml:"underlyingType,attr"`
	UnderlyingTypeKind string `xml:"underlyingTypeKind,attr"`
	rawMemberAttrs
	Attrs []xml.Attr `xml:",any,attr"`
	Any   []rawAny   `xml:",any"`
}

type rawMemberAttrs struct {
	PresCond          string `xml:"presCond,attr"`
	PresCondArgs      string `xml:"presCondArgs,attr"`
	PresCondArgsID    string `xml:"presCondArgsID,attr"`
	DSPresCond        string `xml:"dsPresCond,attr"`
	DSPresCondArgs    string `xml:"dsPresCondArgs,attr"`
	DSPresCondArgsID  string `xml:"dsPresCondArgsID,attr"`
	IsArray           string `xml:"isArray,attr"`
	SizeAttribute     string `xml:"sizeAttribute,attr"`
	MinIndexAttribute string `xml:"minIndexAttribute,attr"`
	MaxIndexAttribute string `xml:"maxIndexAttribute,attr"`
	Transient         string `xml:"transient,attr"`
	Dchg              string `xml:"dchg,attr"`
	Qchg              string `xml:"qchg,attr"`
	Dupd              string `xml:"dupd,attr"`
	DefaultValue      string `xml:"defaultValue,attr"`
	MinValue          string `xml:"minValue,attr"`
	MaxValue          string `xml:"maxValue,attr"`
	MinIndex          string `xml:"minIndex,attr"`
	Deprecated        string `xml:"deprecated,attr"`
	Informative       string `xml:"informative,attr"`
	rawDocAttrs
}

type rawAbstractLNClasses struct {
	rawContainer
	Items []rawLNClass `xml:"AbstractLNClass"`
	Any   []rawAny     `xml:",any"`
}

type rawLNClasses struct {
	rawContainer
	Items []rawLNClass `xml:"LNClass"`
	// Abstract holds abstract classes listed among the concrete ones,
	// as official namespaces do.
	Abstract []rawLNClass `xml:"AbstractLNClass"`
	Any      []rawAny     `xml:",any"`
}

type rawLNClass struct {
	Name        string `xml:"name,attr"`
	Base        string `xml:"base,attr"`
	CanHaveLOG  string `xml:"canHaveLOG,attr"`
	IsExtension string `xml:"isExtension,attr"`
	Deprecated  string `xml:"deprecated,attr"`
	Informative string `xml:"informative,attr"`
	rawDocAttrs
	Attrs       []xml.Attr     `xml:",any,attr"`
	DataObjects []rawCDCMember `xml:"DataObject"`
	Any         []rawAny       `xml:",any"`
}

type rawNSDoc struct {
	XMLName  xml.Name
	ID       string          `xml:"id,attr"`
	Version  string          `xml:"version,attr"`
	Revision string          `xml:"revision,attr"`
	Release  string          `xml:"release,attr"`
	Lang     string          `xml:"lang,attr"`
	Attrs    []xml.Attr      `xml:",any,attr"`
	Docs     []rawNSDocEntry `xml:"Doc"`
	Any      []rawAny        `xml:",any"`
}

type rawNSDocEntry struct {
	ID   string
	Text string
}

// UnmarshalXML collects the character data of a Doc element, including
// that of nested markup, as plain text.
func (e *rawNSDocEntry) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, a := range start.Attr {
		if a.Name.Local == "id" {
			e.ID = a.Value
		}
	}
	var buf []byte
	depth := 1
	for depth > 0 {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
		case xml.EndElement:
			depth--
		case xml.CharData:
			buf = append(buf, t...)
		}
	}
	e.Text = string(buf)
	return nil
}
