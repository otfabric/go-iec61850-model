# nsdctl exit codes

nsdctl uses the organisation exit-code table. The constants are in
[main.go](../main.go).

| Code | Meaning |
| --- | --- |
| 0 | Success |
| 1 | Findings: the command completed and the result reports problems. `validate` found errors, `dependencies` found unsatisfied dependencies, `diff --exit-code` found changes, `generate --check` is out of date, `sources check --packages` found a tracked package missing or carrying another release or local packages that differ from the lock, `schema --exit-code` found gaps |
| 2 | Usage: invalid flags or arguments, unknown command, unknown or ambiguous name, path that does not resolve, refused overwrite without `--force`, `sources fetch --dest` inside the Git work tree |
| 3 | Invalid input: a source, registry or lock cannot be read or is invalid (also unsafe archives, exceeded limits, unsupported schemas, structural or dependency errors while loading, `--strict` warnings), a tracked package cannot be loaded, a downloaded package differs from its pinned digest, is not pinned or is too large |
| 4 | Transport: a download of `sources fetch` failed |
| 9 | Permission: the license policy refused `generate --publish` |
| 10 | Internal: writing the output (stdout, generated files, the lock, the download cache) failed, or an unexpected error |
| 130 | Interrupted: SIGINT or SIGTERM before a result |

Errors go to stderr prefixed with `nsdctl:`; results go to stdout.

## Changes from v0.2.2

The release after v0.2.2 moved nsdctl to the organisation table. Scripts
that test exit codes need this mapping:

| Before (v0.2.2 and earlier) | Now | Case |
| --- | --- | --- |
| 1 | 2 | Usage error, unknown name, path that does not resolve, refused overwrite |
| 1 | 10 | Other failures (writing the output, unexpected errors) |
| 2 | 3 | A source cannot be read or parsed (also unsafe archives and exceeded limits) |
| 2 | 4 | A download of `sources fetch` failed |
| 3 | 3 | Invalid lock or registry, structural or dependency errors while loading, a tracked package that cannot be loaded, digest mismatch, unpinned package |
| 3 | 1 | `validate` found errors, `dependencies` found unsatisfied dependencies |
| 4 | 1 | Differences: `diff --exit-code`, `generate --check`, `sources check --packages`, `schema --exit-code` |
| 5 | 9 | The license policy refused `generate --publish` |
| – | 130 | Interrupted (new) |
