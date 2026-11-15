// SPDX-License-Identifier: MIT

package nsd

import (
	"fmt"

	"github.com/otfabric/go-iec61850-model/model"
)

// SchemaNamespace2016 is the XML namespace of NSD documents defined by
// IEC 61850-7-7.
const SchemaNamespace2016 = "http://www.iec.ch/61850/2016/NSD"

// Default safety bounds. They are generous for official NSD packages,
// whose largest documents are a few MiB. SECURITY.md lists them with
// the fixed bounds that have no option.
const (
	DefaultMaxDocumentSize     = 32 << 20  // 32 MiB per XML document
	DefaultMaxArchiveSize      = 256 << 20 // 256 MiB uncompressed per archive
	DefaultMaxArchiveEntries   = 1024
	DefaultMaxDepth            = 64
	DefaultMaxTokens           = 10_000_000
	DefaultMaxCompressionRatio = 200
)

// Options controls parsing. The zero value is not used directly; pass
// [Option] values to the parse and load functions, which start from
// the defaults. Every limit must be positive and at least one schema
// namespace must be accepted: a parse or load function called with
// another value fails before it reads anything, with an error that
// names the option ([model.ErrLimitExceeded] for a limit,
// [model.ErrUnsupportedSchema] for the schema namespaces).
type Options struct {
	// Source names the input in diagnostics and provenance.
	Source string
	// MaxDocumentSize bounds the size of one XML document in bytes.
	MaxDocumentSize int64
	// MaxArchiveSize bounds the total uncompressed size of a ZIP
	// archive in bytes, over the archives nested in it, and the size
	// of the archive file itself.
	MaxArchiveSize int64
	// MaxArchiveEntries bounds the number of entries of a ZIP archive,
	// over the archives nested in it.
	MaxArchiveEntries int
	// MaxDepth bounds the XML element nesting depth.
	MaxDepth int
	// MaxTokens bounds the number of XML tokens of one document. Every
	// attribute of an element counts as one token.
	MaxTokens int
	// MaxCompressionRatio bounds the ratio of uncompressed to
	// compressed size of a ZIP entry larger than 1 MiB.
	MaxCompressionRatio int64
	// Strict turns warnings (unknown elements and attributes, entries
	// that are not processed) into errors.
	Strict bool
	// SchemaNamespaces lists the accepted XML namespaces of NSD
	// documents.
	SchemaNamespaces []string
	// Language selects the NSDoc language applied to namespaces by the
	// package loaders. Empty disables applying documentation.
	Language string
	// FallbackDocs is a doc set the package loaders apply after the
	// NSDoc files, to the titles and descriptions those left empty.
	FallbackDocs *DocSet
}

// Option configures parsing.
type Option func(*Options)

func defaultOptions() Options {
	return Options{
		MaxDocumentSize:     DefaultMaxDocumentSize,
		MaxArchiveSize:      DefaultMaxArchiveSize,
		MaxArchiveEntries:   DefaultMaxArchiveEntries,
		MaxDepth:            DefaultMaxDepth,
		MaxTokens:           DefaultMaxTokens,
		MaxCompressionRatio: DefaultMaxCompressionRatio,
		SchemaNamespaces:    []string{SchemaNamespace2016},
		Language:            "en",
	}
}

// buildOptions applies opts to the defaults and validates the result.
func buildOptions(opts []Option) (Options, error) {
	o := defaultOptions()
	for _, fn := range opts {
		if fn != nil {
			fn(&o)
		}
	}
	return o, o.validate()
}

// validate reports the first option that cannot work: a limit that is
// not positive, or no accepted schema namespace. The error names the
// option. It wraps the sentinel a load with that value ended in before
// the options were validated: [model.ErrLimitExceeded] for a limit,
// [model.ErrUnsupportedSchema] for the schema namespaces.
func (o *Options) validate() error {
	limits := []struct {
		option string
		value  int64
	}{
		{"WithMaxDocumentSize", o.MaxDocumentSize},
		{"WithMaxArchiveSize", o.MaxArchiveSize},
		{"WithMaxArchiveEntries", int64(o.MaxArchiveEntries)},
		{"WithMaxDepth", int64(o.MaxDepth)},
		{"WithMaxTokens", int64(o.MaxTokens)},
		{"WithMaxCompressionRatio", o.MaxCompressionRatio},
	}
	for _, l := range limits {
		if l.value <= 0 {
			return &model.SourceError{
				Source: sourceName(o.Source),
				Reason: fmt.Sprintf("option %s: %d is not a positive limit", l.option, l.value),
				Err:    model.ErrLimitExceeded,
			}
		}
	}
	if len(o.SchemaNamespaces) == 0 {
		return &model.SourceError{
			Source: sourceName(o.Source),
			Reason: "option WithSchemaNamespaces: no namespace is accepted",
			Err:    model.ErrUnsupportedSchema,
		}
	}
	return nil
}

// WithSource names the input in diagnostics and provenance.
func WithSource(name string) Option { return func(o *Options) { o.Source = name } }

// WithStrict makes warnings fatal: unknown elements and attributes and
// unprocessed archive entries fail the import.
func WithStrict() Option { return func(o *Options) { o.Strict = true } }

// WithMaxDocumentSize sets the maximum size of one XML document, in
// bytes. Like every limit it must be positive; see [Options].
func WithMaxDocumentSize(n int64) Option { return func(o *Options) { o.MaxDocumentSize = n } }

// WithMaxArchiveSize sets the maximum total uncompressed archive size.
func WithMaxArchiveSize(n int64) Option { return func(o *Options) { o.MaxArchiveSize = n } }

// WithMaxArchiveEntries sets the maximum number of archive entries.
func WithMaxArchiveEntries(n int) Option { return func(o *Options) { o.MaxArchiveEntries = n } }

// WithMaxDepth sets the maximum XML nesting depth.
func WithMaxDepth(n int) Option { return func(o *Options) { o.MaxDepth = n } }

// WithMaxTokens sets the maximum number of XML tokens per document;
// attributes count as tokens.
func WithMaxTokens(n int) Option { return func(o *Options) { o.MaxTokens = n } }

// WithMaxCompressionRatio sets the maximum compression ratio of an
// archive entry larger than 1 MiB.
func WithMaxCompressionRatio(n int64) Option { return func(o *Options) { o.MaxCompressionRatio = n } }

// WithSchemaNamespaces replaces the accepted XML namespaces of NSD
// documents. Use it when an official package declares a namespace this
// version does not know yet, after checking that the structure is
// compatible. At least one URI must be given; see [Options].
func WithSchemaNamespaces(uris ...string) Option {
	return func(o *Options) { o.SchemaNamespaces = append([]string(nil), uris...) }
}

// WithLanguage selects the NSDoc language the package loaders apply
// (default "en"). An empty language disables applying documentation.
func WithLanguage(lang string) Option { return func(o *Options) { o.Language = lang } }

// WithFallbackDocs makes the package loaders fill the titles and
// descriptions that the package's own NSDoc files leave empty from set,
// e.g. official.Docs("en"). The package's NSDoc always wins. The set
// applies when its language is the [WithLanguage] language (or the set
// declares none); an empty language disables it, as it disables NSDoc.
func WithFallbackDocs(set *DocSet) Option { return func(o *Options) { o.FallbackDocs = set } }
