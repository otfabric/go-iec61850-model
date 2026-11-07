# nsdctl exit codes

| Code | Meaning |
| --- | --- |
| 0 | Success |
| 1 | Usage error, unknown name, path that does not resolve, refused overwrite |
| 2 | A source cannot be read or parsed (also unsafe archives and exceeded limits), a download failed |
| 3 | Validation failed: structural errors, unsatisfied dependencies, invalid lock or registry, a downloaded package differs from its pinned digest or is not pinned |
| 4 | Differences: `diff --exit-code` found changes, `generate --check` is out of date, a tracked package is missing or carries another release, local packages differ from the lock, `schema --exit-code` found gaps |
| 5 | The license policy refused `generate --publish` |

Errors go to stderr prefixed with `nsdctl:`; results go to stdout.
