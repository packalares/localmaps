/**
 * `ErrorCode` — the authoritative enum of gateway-wide error codes.
 *
 * Mirrors `components.schemas.ErrorCode` in `openapi.yaml`. The
 * const-object-plus-type pattern gives consumers a single name that
 * works as both a runtime value (for emitting errors) and a type
 * (for narrowing in switch/case).
 *
 * If a new code is added to the OpenAPI spec, add it here too; the
 * compile-time assertion at the bottom will fail until the list is
 * in sync with the generated `components['schemas']['ErrorCode']`
 * union.
 */

import type { components } from './api.js';
import type { Equals, Expect } from './type-utils.js';

export const ErrorCode = {
  BadRequest: 'BAD_REQUEST',
  Unauthorized: 'UNAUTHORIZED',
  Forbidden: 'FORBIDDEN',
  NotFound: 'NOT_FOUND',
  Conflict: 'CONFLICT',
  RateLimited: 'RATE_LIMITED',
  Internal: 'INTERNAL',
  UpstreamUnavailable: 'UPSTREAM_UNAVAILABLE',
  RegionNotReady: 'REGION_NOT_READY',
  RegionNotInstalled: 'REGION_NOT_INSTALLED',
  RegionOutOfCoverage: 'REGION_OUT_OF_COVERAGE',
  InvalidRegionName: 'INVALID_REGION_NAME',
  JobNotFound: 'JOB_NOT_FOUND',
  SchemaMismatch: 'SCHEMA_MISMATCH',
  EgressDenied: 'EGRESS_DENIED',
} as const;

export type ErrorCode = typeof ErrorCode[keyof typeof ErrorCode];

// Compile-time assertion: the const-object values must cover exactly
// the same string-literal union that openapi-typescript generated.
type _AssertErrorCodeParity = Expect<
  Equals<ErrorCode, components['schemas']['ErrorCode']>
>;
// Reference the alias so `noUnusedLocals` doesn't complain.
export type _ErrorCodeParityCheck = _AssertErrorCodeParity;
