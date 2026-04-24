export { encodeState, encodeRoute, buildShareUrl } from "./encode";
export { decodeURL, decodeRoute } from "./decode";
export { useRestoreOnMount, applyDecoded } from "./restore";
export type {
  ShareableState,
  DecodedState,
  EncodedState,
  ShareRoute,
  ShareRouteOptions,
  ShareWaypoint,
} from "./types";
export { URL_BUDGET_CHARS } from "./types";
