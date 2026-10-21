# Synthetic NSD fixtures

These files are **independently authored test fixtures**, written by hand for
this repository. They are not IEC 61850 NSD files, not excerpts of them, and
not complete: each contains only the handful of definitions the tests need.

- Structure follows the NSD format of IEC 61850-7-7 so the importer is
  exercised on realistic input.
- Namespace identifiers, versions and releases are test values; they do not
  claim to match any published IEC namespace release.
- Class, object and attribute names (MMXU, Hz, MV, mag, ...) are the short
  identifiers needed to test resolution chains. All titles and descriptions
  are written for these tests and are not IEC text.

Never add official IEC files, or content copied from them, to this directory.
See [docs/licensing.md](../../docs/licensing.md).

| File | Content |
| --- | --- |
| `synthetic-7-2.nsd` | Basic types, functional constraints, trigger options, presence conditions |
| `synthetic-7-3.nsd` | Enumerations, constructed attributes and CDCs (MV, CMV, WYE, DEL, SPS, ENS, INS, LPL, BCR) |
| `synthetic-7-4.nsd` | Abstract LN classes and LLN0, LPHD, MMXU, MMTR, GGIO (release 1) |
| `synthetic-7-4-r2.nsd` | The same namespace, release 2, with changes for diff tests |
| `synthetic-7-4.en.nsdoc` | Test documentation texts for the 7-4 fixtures |
| `synthetic-ext.nsd` | An extension namespace with a D-group class that inherits from 7-4 |
