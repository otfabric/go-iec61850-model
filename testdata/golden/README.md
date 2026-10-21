# Golden generator output

The output of `nsdctl generate go|docs|json` for the synthetic fixtures in
[../nsd](../nsd), with tool version `golden`. `TestGolden` in
`internal/generator` fails when the generator produces different bytes.

After an intentional change of the output, bump `generator.FormatVersion`
and run `make golden`. The `LOCAL-ONLY.md` notices are part of the expected
output; this directory derives from synthetic fixtures only, not from IEC
files, so it may be committed.
