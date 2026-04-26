// Package sprites serves the MapLibre sprite atlases embedded into the
// binary. Atlases are pre-built from the Maki icon set (Apache 2.0, see
// atlases/LICENSE-maki.txt) at build time by scripts/gen-sprites.js and
// live under atlases/.
//
// MapLibre fetches four files for a `sprite: "/api/sprites/default"`
// style URL: default.{json,png} and default@2x.{json,png}. The density
// argument to Lookup is "" for @1x and "2x" for @2x.
package sprites

import "embed"

//go:embed atlases/default.json atlases/default.png atlases/default@2x.json atlases/default@2x.png
var atlasFS embed.FS

// Sprite bundles the two files MapLibre needs to render an atlas: the
// JSON position index and the PNG image.
type Sprite struct {
	JSON []byte
	PNG  []byte
}

// Lookup returns the embedded sprite atlas for the given name + density.
// density is either "" (1x) or "2x". Only name == "default" is shipped
// today; unknown names return ok == false.
func Lookup(name, density string) (Sprite, bool) {
	if name != "default" {
		return Sprite{}, false
	}
	suffix := ""
	switch density {
	case "", "1x":
		suffix = ""
	case "2x":
		suffix = "@2x"
	default:
		return Sprite{}, false
	}
	jsonBytes, err := atlasFS.ReadFile("atlases/" + name + suffix + ".json")
	if err != nil {
		return Sprite{}, false
	}
	pngBytes, err := atlasFS.ReadFile("atlases/" + name + suffix + ".png")
	if err != nil {
		return Sprite{}, false
	}
	return Sprite{JSON: jsonBytes, PNG: pngBytes}, true
}
