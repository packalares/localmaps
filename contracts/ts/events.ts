/**
 * WebSocket event shapes for `/api/ws` (see
 * `docs/03-contracts.md` §"WebSocket event shapes" and the
 * `/api/ws` description in `openapi.yaml`).
 *
 * The ONLY client→server payloads are subscribe / unsubscribe with a
 * channel name. Channels are free-form strings of the form
 * `region.<name>` or `job.<id>`; the gateway is the authority on
 * channel naming conventions — we do not constrain them here beyond
 * "non-empty string".
 *
 * Server→client payloads are a discriminated union on `type`:
 *   - `region.progress` | `region.ready` | `region.failed` → `data: Region`
 *   - `job.started` | `job.progress` | `job.complete` | `job.failed` → `data: Job`
 *
 * Both the const-object `WsEventType` and the narrow event interfaces
 * are exported so Go-generated clients and the UI can share the
 * literal set without hand-coding strings.
 */

import type { components } from './api.js';

type Region = components['schemas']['Region'];
type Job = components['schemas']['Job'];

// --------------------------------------------------------------------
// Event type literals
// --------------------------------------------------------------------

export const WsEventType = {
  RegionProgress: 'region.progress',
  RegionReady: 'region.ready',
  RegionFailed: 'region.failed',
  JobStarted: 'job.started',
  JobProgress: 'job.progress',
  JobComplete: 'job.complete',
  JobFailed: 'job.failed',
} as const;

export type WsEventType = typeof WsEventType[keyof typeof WsEventType];

export const WsClientMessageType = {
  Subscribe: 'subscribe',
  Unsubscribe: 'unsubscribe',
} as const;

export type WsClientMessageType =
  typeof WsClientMessageType[keyof typeof WsClientMessageType];

// --------------------------------------------------------------------
// Server → client events (discriminated union on `type`)
// --------------------------------------------------------------------

export interface WsRegionProgressEvent {
  type: typeof WsEventType.RegionProgress;
  data: Region;
}

export interface WsRegionReadyEvent {
  type: typeof WsEventType.RegionReady;
  data: Region;
}

export interface WsRegionFailedEvent {
  type: typeof WsEventType.RegionFailed;
  data: Region;
}

export interface WsJobStartedEvent {
  type: typeof WsEventType.JobStarted;
  data: Job;
}

export interface WsJobProgressEvent {
  type: typeof WsEventType.JobProgress;
  data: Job;
}

export interface WsJobCompleteEvent {
  type: typeof WsEventType.JobComplete;
  data: Job;
}

export interface WsJobFailedEvent {
  type: typeof WsEventType.JobFailed;
  data: Job;
}

export type WsServerEvent =
  | WsRegionProgressEvent
  | WsRegionReadyEvent
  | WsRegionFailedEvent
  | WsJobStartedEvent
  | WsJobProgressEvent
  | WsJobCompleteEvent
  | WsJobFailedEvent;

// --------------------------------------------------------------------
// Client → server messages (discriminated union on `type`)
// --------------------------------------------------------------------

export interface WsSubscribeMessage {
  type: typeof WsClientMessageType.Subscribe;
  channel: string;
}

export interface WsUnsubscribeMessage {
  type: typeof WsClientMessageType.Unsubscribe;
  channel: string;
}

export type WsClientMessage = WsSubscribeMessage | WsUnsubscribeMessage;

// --------------------------------------------------------------------
// Narrowing helpers — tiny type-guards so consumers can write
// `if (isJobEvent(ev)) { ev.data.state ... }` without re-checking
// string literals.
// --------------------------------------------------------------------

const JOB_EVENT_TYPES: ReadonlySet<WsEventType> = new Set<WsEventType>([
  WsEventType.JobStarted,
  WsEventType.JobProgress,
  WsEventType.JobComplete,
  WsEventType.JobFailed,
]);

const REGION_EVENT_TYPES: ReadonlySet<WsEventType> = new Set<WsEventType>([
  WsEventType.RegionProgress,
  WsEventType.RegionReady,
  WsEventType.RegionFailed,
]);

export function isJobEvent(
  ev: WsServerEvent,
): ev is
  | WsJobStartedEvent
  | WsJobProgressEvent
  | WsJobCompleteEvent
  | WsJobFailedEvent {
  return JOB_EVENT_TYPES.has(ev.type);
}

export function isRegionEvent(
  ev: WsServerEvent,
): ev is WsRegionProgressEvent | WsRegionReadyEvent | WsRegionFailedEvent {
  return REGION_EVENT_TYPES.has(ev.type);
}
