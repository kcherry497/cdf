# CDF format specification (v1)

## On disk

A `.cdf` file is **pretty-printed JSON** (2-space indent, UTF-8, LF line
endings). No wrapper, no HTML, no magic bytes — just JSON. The file is
identified as CDF by the presence of `meta.cdf_version` at the top level.

Extension `.cdf` is convention; renaming to `.json` does not change
behavior.

## Schema

```jsonc
{
  "meta": {
    "cdf_version": 1,                    // required, integer, currently 1
    "title": "string, ≤120 chars",       // required
    "subtitle": "string, ≤240 chars",    // optional
    "updated": "YYYY-MM-DD"              // required, ISO date
  },
  "sections": [                          // required, array (may be empty)
    {
      "id": "string",                    // required, unique within file,
                                         //   ASCII [a-z0-9-_], used as anchor
      "name": "string, ≤80 chars",       // required, section heading
      "note": "string",                  // optional, one paragraph
      "rows": [                          // required, array (may be empty)
        {
          "date": "string",              // required, freeform date/period
          "action": "string, ≤200 chars",// required, short label
          "detail": "string"             // optional, longer text (wraps)
        }
      ]
    }
  ]
}
```

Extra fields at any level are **preserved on round-trip** but ignored by
the renderer.

## Row classification (visual marker in reader/editor)

The renderer determines each row's marker color from its `action` and
`date`; the source JSON never carries color state.

| Rule (first match wins)                     | Marker | Semantic |
|---------------------------------------------|--------|----------|
| `action` starts with `MILESTONE:` (case-sensitive) | teal   | key event |
| `action` starts with `EXPOSURE:` (case-sensitive)  | red    | security incident |
| `date` is exactly `—` or `-`                        | amber  | deferred / no date |
| otherwise                                           | dim    | ordinary entry |

Tags are LITERAL — they render as part of the action label AND control
the marker. Do not strip.

## Rendering (reader/editor contract)

- **Background**: terminal default (dark terminals get dark; light terminals
  get light, but the palette is tuned for dark).
- **Colors** (24-bit ANSI, degrade to 256-color or 16-color as needed):
  - ink: default fg
  - muted: `#7a8090`
  - accent: `#4fd1c5`
  - warn: `#d4a94a`
  - danger: `#e07070`
- **Tables**: full box-drawing borders on every cell (`┌─┬─┐│├─┼─┤└─┴─┘`).
  Fallback ASCII `+-|` when unicode isn't available.
- **Font**: whatever the terminal is set to (user's choice).
- **Layout**: single column, full terminal width, wrapping.

## Round-trip guarantee

Reading a CDF, mutating fields programmatically, and re-serializing MUST
preserve unknown fields byte-equivalently (up to JSON canonicalization).
This is what makes CDF diff-friendly.

## Export

The reader can emit an HTML export for browser-based one-off sharing:
`cdf export file.cdf > file.html`. The HTML is self-contained, dark,
bordered, and matches the terminal rendering visually — but it is a
one-way export, not the primary format. Recipients edit by getting the
`.cdf` and running `cdf` on it.

## Version

CDF v1. Increment `meta.cdf_version` when the schema or renderer contract
breaks compatibility.
