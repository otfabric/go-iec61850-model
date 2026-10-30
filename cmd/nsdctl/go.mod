module github.com/otfabric/go-iec61850-model/cmd/nsdctl

go 1.24.0

require (
	github.com/otfabric/go-iec61850-model v0.1.1
	github.com/spf13/cobra v1.10.2
)

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
)

// nsdctl is built from the library in this repository, including its internal
// packages. v0.1.1 still contains this package, so the requirement alone is an
// ambiguous import.
replace github.com/otfabric/go-iec61850-model => ../..
