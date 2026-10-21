// SPDX-License-Identifier: MIT

// Package ontology exposes a namespace profile as a small, typed
// concept graph: logical node classes contain data objects, data
// objects use CDCs, CDCs contain attributes, attributes have types,
// definitions belong to namespaces, and — with semantic mapping
// enabled — measurements represent quantities, belong to phases and
// are measured by data objects.
//
//	g, err := ontology.BuildGraph(profile, ontology.Options{Semantics: semantic.Default})
//	for _, r := range g.Relations(ontology.LogicalNodeID("MMXU"), ontology.Contains) { ... }
//	err = g.ExportJSON(os.Stdout)
//
// The graph is deterministic: listings, traversals and the JSON export
// are ordered, so identical input gives byte-identical output. It uses
// no RDF, OWL or CIM libraries.
//
// # Adapter seam
//
// Measurement, quantity, phase and reference concepts are
// protocol-neutral. They are expected to move to a shared OTFabric
// ontology module used by Modbus, SunSpec, DNP3 and OPC UA mappings as
// well; this package will then link IEC 61850 concepts to that module
// instead of defining them. Concept IDs of the semantic kinds
// ("quantity:voltage", "phase:A") are chosen to stay stable across
// that move.
package ontology
