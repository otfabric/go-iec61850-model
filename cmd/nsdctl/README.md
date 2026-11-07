# nsdctl

`nsdctl` is the companion CLI of [go-iec61850-model](../../README.md): it
inspects and validates IEC 61850 NSD namespace packages, explores the model,
compares releases and generates Go declarations, documentation and JSON
exports. It never downloads IEC packages.

This directory is its own Go module
(`github.com/otfabric/go-iec61850-model/cmd/nsdctl`). Installation is
described in the [repository README](../../README.md#install-nsdctl).

| Document | Contents |
|---|---|
| [docs/cli.md](docs/cli.md) | Global flags, commands and a walkthrough with real output |
| [docs/exitcodes.md](docs/exitcodes.md) | Process exit codes |
