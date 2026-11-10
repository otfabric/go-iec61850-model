// SPDX-License-Identifier: MIT

//go:build !iecdocs

package official

// embeddedDocs is empty without the iecdocs build tag: the repository
// ships no documentation texts. Build with -tags iecdocs after "make
// docs-extract", or set IEC61850_MODEL_DOCS, to get them.
var embeddedDocs []byte
