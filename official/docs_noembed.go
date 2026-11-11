// SPDX-License-Identifier: MIT

//go:build !iecdocs

package official

// embeddedDocs is empty without the iecdocs build tag: only the public
// texts (texts_gen.json.gz) are embedded. Build with -tags iecdocs after
// "make docs-extract", or set IEC61850_MODEL_DOCS, to add a local doc set.
var embeddedDocs []byte
