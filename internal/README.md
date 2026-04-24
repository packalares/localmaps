# `internal/` — shared Go packages

Packages here are importable by both `server/` and `worker/` subtrees
because they live under the common module root.

Use this when a type or utility is genuinely needed on both sides of
the gateway/worker split (Geofabrik catalog types, region key
normaliser, WS event shapes, shared DB queries). Code that only one
side needs must stay under `server/internal/` or `worker/internal/` to
keep the blast radius narrow — Go's `internal/` rule enforces it.
