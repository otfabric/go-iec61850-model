// SPDX-License-Identifier: MIT

//go:build iecdocs

package official

import _ "embed"

// embeddedDocs holds the local doc set generated with "make docs-extract"
// (official/docs_gen.json.gz, git-ignored). It is embedded only in builds
// with the iecdocs tag and applied on top of the public texts; it is never
// committed or released.
//
//go:embed docs_gen.json.gz
var embeddedDocs []byte
