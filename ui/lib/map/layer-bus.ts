import type {
  AddLayerObject,
  Map as MapLibreMap,
  SourceSpecification,
} from "maplibre-gl";

/**
 * Distributive `Omit` — preserves the union shape of `AddLayerObject`
 * so callers retain access to `paint` / `layout` / `filter` fields on
 * each individual layer variant (line, circle, fill, …).
 */
type DistributiveOmit<T, K extends PropertyKey> = T extends unknown
  ? Omit<T, K>
  : never;

/** Layer spec accepted by `registerLayer`; id + source are injected. */
export type RegisterLayerInput = DistributiveOmit<
  AddLayerObject,
  "id" | "source"
> & {
  source?: string;
};

/**
 * Shared helper for sibling feature modules (route panel, POI pane, etc.)
 * that need to mutate the single MapLibre instance held by `MapView` via
 * the Zustand store. Using this bus keeps ownership explicit and makes
 * registration idempotent — re-mounting a component does not leave stale
 * sources or layers behind.
 */

/**
 * A minimal MapLibre-like surface used at runtime. We depend on the real
 * types for callers, but tests can pass a hand-rolled stub that fulfils
 * this interface.
 */
export interface LayerBusMap {
  getSource: (id: string) => unknown;
  getLayer: (id: string) => unknown;
  addSource: (id: string, source: SourceSpecification) => void;
  addLayer: (layer: AddLayerObject, beforeId?: string) => void;
  removeLayer: (id: string) => void;
  removeSource: (id: string) => void;
}

/**
 * Register or replace a paired (source, layer) on the map. The `id` is used
 * as both the source id and layer id — callers that need to separate them
 * should supply a `layer.source` field inside `layer` and drop `source`.
 *
 * When the id already exists, both the layer and source are removed and
 * then re-added so the call is idempotent.
 */
export function registerLayer(
  map: LayerBusMap | MapLibreMap,
  id: string,
  source: SourceSpecification,
  layer: RegisterLayerInput,
  beforeId?: string,
): void {
  const m = map as LayerBusMap;
  // Layer first (it references the source), then source — order matters
  // for removal. For addition we do source first.
  if (m.getLayer(id)) m.removeLayer(id);
  if (m.getSource(id)) m.removeSource(id);

  m.addSource(id, source);
  const resolvedLayer = {
    ...layer,
    id,
    source: layer.source ?? id,
  } as AddLayerObject;
  m.addLayer(resolvedLayer, beforeId);
}

/**
 * Remove a previously-registered (source, layer) pair by id. Missing ids
 * are a silent no-op so callers can safely invoke this inside React
 * effect cleanup without first checking.
 */
export function unregisterLayer(
  map: LayerBusMap | MapLibreMap,
  id: string,
): void {
  const m = map as LayerBusMap;
  if (m.getLayer(id)) m.removeLayer(id);
  if (m.getSource(id)) m.removeSource(id);
}
