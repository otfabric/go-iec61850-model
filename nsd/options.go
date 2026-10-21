// SPDX-License-Identifier: MIT

package nsd

// SchemaNamespace2016 is the XML namespace of NSD documents defined by
// IEC 61850-7-7.
const SchemaNamespace2016 = "http://www.iec.ch/61850/2016/NSD"

// Default safety bounds. They are generous for official NSD packages,
// whose largest documents are a few MiB, and small enough to stop
// malicious input early.
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
// the defaults.
type Options struct {
	// Source names the input in diagnostics and provenance.
	Source string
	// MaxDocumentSize bounds the size of one XML document in bytes.
	MaxDocumentSize int64
	// MaxArchiveSize bounds the total uncompressed size of a ZIP
	// archive in bytes.
	MaxArchiveSize int64
	// MaxArchiveEntries bounds the number of entries of a ZIP archive.
	MaxArchiveEntries int
	// MaxDepth bounds the XML element nesting depth.
	MaxDepth int
	// MaxTokens bounds the number of XML tokens of one document.
	MaxTokens int
	// MaxCompressionRatio bounds the ratio of uncompressed to
	// compressed size of a ZIP entry.
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

func buildOptions(opts []Option) Options {
	o := defaultOptions()
	for _, fn := range opts {
		if fn != nil {
			fn(&o)
		}
	}
	return o
}

// WithSource names the input in diagnostics and provenance.
func WithSource(name string) Option { return func(o *Options) { o.Source = name } }

// WithStrict makes warnings fatal: unknown elements and attributes and
// unprocessed archive entries fail the import.
func WithStrict() Option { return func(o *Options) { o.Strict = true } }

// WithMaxDocumentSize sets the maximum size of one XML document.
func WithMaxDocumentSize(n int64) Option { return func(o *Options) { o.MaxDocumentSize = n } }

// WithMaxArchiveSize sets the maximum total uncompressed archive size.
func WithMaxArchiveSize(n int64) Option { return func(o *Options) { o.MaxArchiveSize = n } }

// WithMaxArchiveEntries sets the maximum number of archive entries.
func WithMaxArchiveEntries(n int) Option { return func(o *Options) { o.MaxArchiveEntries = n } }

// WithMaxDepth sets the maximum XML nesting depth.
func WithMaxDepth(n int) Option { return func(o *Options) { o.MaxDepth = n } }

// WithMaxTokens sets the maximum number of XML tokens per document.
func WithMaxTokens(n int) Option { return func(o *Options) { o.MaxTokens = n } }

// WithMaxCompressionRatio sets the maximum compression ratio of an
// archive entry.
func WithMaxCompressionRatio(n int64) Option { return func(o *Options) { o.MaxCompressionRatio = n } }

// WithSchemaNamespaces replaces the accepted XML namespaces of NSD
// documents. Use it when an official package declares a namespace this
// version does not know yet, after checking that the structure is
// compatible.
func WithSchemaNamespaces(uris ...string) Option {
	return func(o *Options) { o.SchemaNamespaces = append([]string(nil), uris...) }
}

// WithLanguage selects the NSDoc language the package loaders apply
// (default "en"). An empty language disables applying documentation.
func WithLanguage(lang string) Option { return func(o *Options) { o.Language = lang } }
