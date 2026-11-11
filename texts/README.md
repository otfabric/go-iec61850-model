# Public description texts

OT Fabric's own titles and descriptions for the IEC 61850 information model,
MIT licensed. `make texts` (`internal/tools/textgen`) turns these tables and
the committed catalog into `official/texts_gen.json.gz`, which the library
embeds; `make texts-check`, part of `make check`, fails when that file is stale
or when a catalog element has no text.

| Table | Keys | Content |
| --- | --- | --- |
| `en/ln.tsv` | `LN:` | logical node class titles |
| `en/abbreviations.tsv` | `ABBR:`, and composition of `DO:` | expansions of the terms names are built from |
| `en/do.tsv` | `DO:` | data object texts that replace the composed text, by `name` or `class.name` |
| `en/cdc.tsv` | `CDC:` | common data class titles |
| `en/da.tsv` | `DA:`, `SDO:`, `SP:` | attribute texts, by member name, for every CDC |
| `en/ca.tsv`, `en/sda.tsv` | `CA:`, `SDA:` | constructed attribute types and members |
| `en/fc.tsv`, `en/bt.tsv`, `en/pc.tsv` | `FC:`, `BT:`, `PC:` | functional constraints, basic types, presence conditions |

Enumeration and literal texts (`ENUM:`, `LIT:`) are derived from the names.

Format: `name<TAB>text`, optionally `<TAB>?` to flag a text for review; lines
starting with `#` are comments. A data object description is the expansions
of its terms in order, with the split into the fewest terms (`OpTmClsAlm` →
"Operate time close alarm"); digits stand for themselves.

## Rules

- Write the texts yourself, from the names and their use in the model. Do not
  copy or paraphrase text from a standard, an NSDoc or a vendor manual.
- Plain English, sentence case, no trailing period. Term expansions are
  lowercase fragments (`W` → "active power") because they are joined.
- A new namespace or release that adds elements makes `make texts-check` fail
  until every new element has a text.
