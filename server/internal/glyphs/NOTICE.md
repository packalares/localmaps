# Font Glyph Attribution

The pre-built MapLibre glyph PBFs under `fonts/` are derived from the
[openmaptiles/fonts](https://github.com/openmaptiles/fonts) project
(the `gh-pages` branch, which ships the fontnik-compiled output of the
master branch TTF sources).

Three font stacks are embedded:

- `fonts/Noto Sans Regular/` — sourced from `Klokantech Noto Sans Regular`
- `fonts/Noto Sans Bold/`    — sourced from `Klokantech Noto Sans Bold`
- `fonts/Noto Sans Italic/`  — sourced from `Klokantech Noto Sans Italic`

The upstream directory names carry a "Klokantech" prefix because the
Noto Sans TTFs were patched by Klokan Technologies to cover additional
scripts. We rename the directories to the bare `Noto Sans <weight>`
form so style sheets reference the stack by the canonical Noto name.
No bytes inside the PBFs are modified.

## License

The underlying Noto Sans fonts are released by Google under the
[SIL Open Font License 1.1](https://scripts.sil.org/OFL) and the
[Apache License 2.0](https://www.apache.org/licenses/LICENSE-2.0).
The openmaptiles/fonts project packaging is Apache 2.0. Redistribution
of the generated PBFs is permitted under both licenses; consumers of
this binary inherit those terms for the embedded glyph bytes.

See the upstream LICENSE at
<https://github.com/openmaptiles/fonts/blob/master/LICENSE.md> for the
authoritative text.
