package api

// Handlers for the `settings` tag in contracts/openapi.yaml.
//
// The read path (GET) loads every row from the settings table (minus
// schema.version) and returns the nested tree the UI consumes.
//
// The write paths (PATCH and PUT) accept a tree OR a flat dotted-key
// body, validate each changed value against the schema, and commit them
// atomically through config.Store.Transaction.
//
// The schema path (GET /api/settings/schema) derives the schema from
// config.Defaults() + settingsschema.Annotations and is anonymous.

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/jmoiron/sqlx"

	"github.com/packalares/localmaps/server/internal/apierr"
	"github.com/packalares/localmaps/server/internal/auth"
	"github.com/packalares/localmaps/server/internal/config"
	"github.com/packalares/localmaps/server/internal/settingsschema"
)

// newSettingsHandlers returns the closures bound to the given store. If
// store is nil they fall back to 501 (keeps tests that build a Deps{}
// without a Store working).
type settingsHandlers struct {
	store *config.Store
}

func newSettingsHandlers(s *config.Store) *settingsHandlers { return &settingsHandlers{store: s} }

// GET /api/settings (admin)
func (h *settingsHandlers) get(c fiber.Ctx) error {
	if h.store == nil {
		return notImplemented(c)
	}
	flat, err := h.readAll(c)
	if err != nil {
		return apierr.Write(c, apierr.CodeInternal, "failed to read settings", true)
	}
	return c.JSON(settingsschema.Tree(flat))
}

// PATCH /api/settings (admin)
func (h *settingsHandlers) patch(c fiber.Ctx) error {
	return h.applyWrite(c, false)
}

// PUT /api/settings (admin) — replaces the provided keys. Today it
// behaves the same as PATCH (partial replace) because there is no
// "unknown key ⇒ delete" story yet; openapi doesn't ask for one.
func (h *settingsHandlers) put(c fiber.Ctx) error {
	return h.applyWrite(c, true)
}

// GET /api/settings/schema (anonymous)
func (h *settingsHandlers) schema(c fiber.Ctx) error {
	nodes := settingsschema.BuildSchema(config.Defaults())
	// The openapi SettingsSchemaNode type only contains type/description/
	// default/enumValues/minimum/maximum/items/properties. Our Node adds
	// key, uiGroup, itemType, unit, step, readOnly, pattern — additive
	// properties that the UI reads. See report for NEEDED flag.
	return c.JSON(fiber.Map{
		"version": config.SchemaVersion,
		"nodes":   nodes,
	})
}

// applyWrite is the shared PATCH/PUT path. bodyIsFlat=true lets tests
// opt into a flat JSON body; the handler auto-detects either form.
func (h *settingsHandlers) applyWrite(c fiber.Ctx, _ bool) error {
	if h.store == nil {
		return notImplemented(c)
	}
	raw := c.Body()
	if len(raw) == 0 {
		return apierr.Write(c, apierr.CodeBadRequest, "empty body", false)
	}
	patch, err := decodePatch(raw)
	if err != nil {
		return apierr.Write(c, apierr.CodeBadRequest, err.Error(), false)
	}
	if len(patch) == 0 {
		return apierr.Write(c, apierr.CodeBadRequest, "no settings in body", false)
	}
	nodes := settingsschema.BuildSchema(config.Defaults())
	byKey := settingsschema.ByKey(nodes)
	for k, v := range patch {
		n, ok := byKey[k]
		if !ok {
			return apierr.Write(c, apierr.CodeBadRequest,
				fmt.Sprintf("unknown setting %q", k), false)
		}
		if n.ReadOnly {
			return apierr.Write(c, apierr.CodeBadRequest,
				fmt.Sprintf("%s is read-only", k), false)
		}
		if err := settingsschema.ValidateValue(n, v); err != nil {
			return apierr.Write(c, apierr.CodeBadRequest, err.Error(), false)
		}
	}
	user := "system"
	if id := auth.FromCtx(c); id != nil {
		user = id.Username
	}
	if err := h.writeAll(patch, user); err != nil {
		return apierr.Write(c, apierr.CodeInternal,
			"failed to write settings", true)
	}
	flat, err := h.readAll(c)
	if err != nil {
		return apierr.Write(c, apierr.CodeInternal, "failed to re-read settings", true)
	}
	return c.JSON(settingsschema.Tree(flat))
}

// readAll pulls every settings row and decodes each value. schema.version
// is excluded from the UI-visible map.
func (h *settingsHandlers) readAll(_ fiber.Ctx) (map[string]any, error) {
	type row struct {
		Key   string `db:"key"`
		Value string `db:"value"`
	}
	var rows []row
	if err := h.store.DB().Select(&rows,
		`SELECT key, value FROM settings`); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	out := make(map[string]any, len(rows))
	for _, r := range rows {
		if r.Key == "schema.version" {
			continue
		}
		var v any
		if err := json.Unmarshal([]byte(r.Value), &v); err != nil {
			return nil, fmt.Errorf("decode %s: %w", r.Key, err)
		}
		out[r.Key] = v
	}
	return out, nil
}

// writeAll commits patch rows in a single transaction. A failure on any
// row rolls back the lot.
func (h *settingsHandlers) writeAll(patch map[string]any, user string) error {
	return h.store.Transaction(func(tx *sqlx.Tx) error {
		ts := time.Now().UTC().Format(time.RFC3339Nano)
		for k, v := range patch {
			b, err := json.Marshal(v)
			if err != nil {
				return fmt.Errorf("encode %s: %w", k, err)
			}
			if _, err := tx.Exec(
				`INSERT INTO settings (key, value, updated_at, updated_by)
				 VALUES (?, ?, ?, ?)
				 ON CONFLICT(key) DO UPDATE SET
				     value = excluded.value,
				     updated_at = excluded.updated_at,
				     updated_by = excluded.updated_by`,
				k, string(b), ts, user,
			); err != nil {
				return fmt.Errorf("write %s: %w", k, err)
			}
		}
		return nil
	})
}

// decodePatch accepts either a flat dotted-key object or a nested tree
// and returns the flat representation used for validation + writes.
func decodePatch(raw []byte) (map[string]any, error) {
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("invalid JSON body: %w", err)
	}
	// Decide: if any top-level key contains a dot → it's already flat.
	flat := map[string]any{}
	nested := map[string]any{}
	for k, v := range obj {
		if strings.Contains(k, ".") {
			flat[k] = v
		} else {
			nested[k] = v
		}
	}
	if len(flat) == 0 {
		return settingsschema.Flatten(nested), nil
	}
	if len(nested) == 0 {
		return flat, nil
	}
	// Mixed — flatten nested and merge.
	merged := settingsschema.Flatten(nested)
	for k, v := range flat {
		merged[k] = v
	}
	return merged, nil
}

// --- Registration shims wired by router.go ---------------------------
// The router calls the top-level functions below for readability; they
// delegate to a lazily-constructed handler bound to the live store.
var pkgSettings *settingsHandlers

func setSettingsStore(s *config.Store) { pkgSettings = newSettingsHandlers(s) }

func settingsGetHandler(c fiber.Ctx) error {
	if pkgSettings == nil {
		return notImplemented(c)
	}
	return pkgSettings.get(c)
}
func settingsPutHandler(c fiber.Ctx) error {
	if pkgSettings == nil {
		return notImplemented(c)
	}
	return pkgSettings.put(c)
}
func settingsPatchHandler(c fiber.Ctx) error {
	if pkgSettings == nil {
		return notImplemented(c)
	}
	return pkgSettings.patch(c)
}
func settingsSchemaHandler(c fiber.Ctx) error {
	// Schema endpoint is pure — works even when the store is nil.
	h := &settingsHandlers{}
	return h.schema(c)
}
