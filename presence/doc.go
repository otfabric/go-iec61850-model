// SPDX-License-Identifier: MIT

// Package presence evaluates the presence conditions of IEC 61850
// information models (IEC 61850-7-3 Table 1 and the PresenceConditions
// of the 7-2 NSD, as IEC 61850-7-7 declares them) against an instance:
// the data objects of a logical node, or the attributes and sub data
// objects of a data object, that a device or an SCL type actually has.
//
// For every element of the class, a [Result] says what the condition
// requires in this instance ([Required], [Optional], [Forbidden]) and
// whether the instance meets it ([Satisfied], [Violated]):
//
//	ev := presence.New(profile)
//	rep, err := ev.LogicalNode("CSWI", &presence.Instance{
//		Present: []string{"Beh", "Pos"},
//		Children: map[string]*presence.Instance{
//			"Pos": {Present: []string{"stVal", "q", "t", "ctlModel"},
//				Values: map[string]string{"ctlModel": "sbo-with-enhanced-security"}},
//		},
//	})
//	// rep.Violations() lists Pos.sboTimeout, Pos.stSeld, Pos.operTimeout.
//
// Conditions fall into five families:
//
//   - Unconditional: M, O, F, na, and the multi-instance Mmulti, Omulti,
//     MmultiRange and OmultiRange, which also check instance numbers.
//   - Sibling: MF, MO, OF, OM, FM and MmultiF name another element of
//     the same class; the condition holds when that element is present.
//   - Group: AtLeastOne, AtMostOne, AllOrNonePerGroup, AllOnlyOneGroup,
//     AllAtLeastOneGroup and MAllOrNonePerGroup relate the elements that
//     share the condition (and group number).
//   - Context: MOln0, MFln0, MOrootLD, MOlnNs, MOdataNs, MOcdcNs, MOsbo,
//     MOenhanced, MOrms, MFsubst and the MFscaled* family depend on where
//     the instance sits (LLN0, the root logical device, the namespaces
//     declared by ldNs, lnNs and dataNs), on attribute values (ctlModel,
//     hvRef) or on other elements of the instance.
//   - Fact: MOcond, MFcond, OFcond and the conditions whose subject is
//     not visible in the model (MOoperTm, MONamPlt, MORange, OMSynPh,
//     MFsg, MFnsg) need a fact from the caller ([Context.Facts]).
//
// When the instance lacks a fact a condition depends on, the requirement
// is [Undetermined] and [Result.Missing] names the fact. The evaluator
// never assumes a default; the verdict is still [Satisfied] or
// [Violated] when every possible value of the fact gives the same
// outcome (an MO element that is present satisfies its condition either
// way). [Conditions] lists every condition with its family and the fact
// it may need.
//
// The evaluator reads the model through [Lookup], which
// *catalog.Profile implements; [resolver.Resolver.Presence] is the same
// evaluation through a resolver. [ValidateNamespace] checks the presence
// conditions a namespace declares (known names, well-formed arguments,
// existing siblings), and [ReadInstance] reads the JSON form of an
// instance that nsdctl model check uses.
package presence
