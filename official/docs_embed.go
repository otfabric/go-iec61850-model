// SPDX-License-Identifier: MIT

//go:build iecdocs

package official

import _ "embed"

// embeddedDocs holds the doc sets generated locally with "make
// docs-extract" (official/docs_gen.json.gz, git-ignored). It is
// embedded only in builds with the iecdocs tag; the repository ships
// no documentation texts.
//
//go:embed docs_gen.json.gz
var embeddedDocs []byte
