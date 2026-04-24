// Package apierr provides the uniform error envelope defined in
// contracts/openapi.yaml (schemas ErrorResponse / ErrorCode).
package apierr

import (
	"github.com/gofiber/fiber/v3"
)

// ErrorCode is the enum of error codes declared in openapi.yaml under
// components/schemas/ErrorCode. Do not add values that aren't in the spec.
type ErrorCode string

// Exported error codes. MUST stay in sync with contracts/openapi.yaml.
const (
	CodeBadRequest           ErrorCode = "BAD_REQUEST"
	CodeUnauthorized         ErrorCode = "UNAUTHORIZED"
	CodeForbidden            ErrorCode = "FORBIDDEN"
	CodeNotFound             ErrorCode = "NOT_FOUND"
	CodeConflict             ErrorCode = "CONFLICT"
	CodeRateLimited          ErrorCode = "RATE_LIMITED"
	CodeInternal             ErrorCode = "INTERNAL"
	CodeUpstreamUnavailable  ErrorCode = "UPSTREAM_UNAVAILABLE"
	CodeRegionNotReady       ErrorCode = "REGION_NOT_READY"
	CodeRegionNotInstalled   ErrorCode = "REGION_NOT_INSTALLED"
	CodeRegionOutOfCoverage  ErrorCode = "REGION_OUT_OF_COVERAGE"
	CodeInvalidRegionName    ErrorCode = "INVALID_REGION_NAME"
	CodeJobNotFound          ErrorCode = "JOB_NOT_FOUND"
	CodeSchemaMismatch       ErrorCode = "SCHEMA_MISMATCH"
	CodeEgressDenied         ErrorCode = "EGRESS_DENIED"
)

// Body is the inner `error` object of the ErrorResponse envelope.
type Body struct {
	Code      ErrorCode              `json:"code"`
	Message   string                 `json:"message"`
	Retryable bool                   `json:"retryable"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// ErrorResponse mirrors components/schemas/ErrorResponse in openapi.yaml.
type ErrorResponse struct {
	Error   Body   `json:"error"`
	TraceID string `json:"traceId"`
}

// traceIDFromCtx returns the trace id set by the telemetry middleware, or "".
func traceIDFromCtx(c fiber.Ctx) string {
	if v := c.Locals("traceId"); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// httpStatusForCode maps an ErrorCode to its default HTTP status.
func httpStatusForCode(code ErrorCode) int {
	switch code {
	case CodeBadRequest, CodeInvalidRegionName, CodeSchemaMismatch:
		return fiber.StatusBadRequest
	case CodeUnauthorized:
		return fiber.StatusUnauthorized
	case CodeForbidden, CodeEgressDenied:
		return fiber.StatusForbidden
	case CodeNotFound, CodeRegionNotInstalled, CodeJobNotFound:
		return fiber.StatusNotFound
	case CodeConflict:
		return fiber.StatusConflict
	case CodeRateLimited:
		return fiber.StatusTooManyRequests
	case CodeRegionNotReady:
		return fiber.StatusServiceUnavailable
	case CodeUpstreamUnavailable, CodeRegionOutOfCoverage:
		return fiber.StatusBadGateway
	default:
		return fiber.StatusInternalServerError
	}
}

// Write renders an ErrorResponse JSON body and sends it on the Fiber context.
// The chosen HTTP status comes from httpStatusForCode.
func Write(c fiber.Ctx, code ErrorCode, message string, retryable bool) error {
	return WriteWithStatus(c, httpStatusForCode(code), code, message, retryable)
}

// WriteWithStatus lets the caller override the HTTP status (rarely needed).
func WriteWithStatus(c fiber.Ctx, status int, code ErrorCode, message string, retryable bool) error {
	resp := ErrorResponse{
		Error: Body{
			Code:      code,
			Message:   message,
			Retryable: retryable,
		},
		TraceID: traceIDFromCtx(c),
	}
	return c.Status(status).JSON(resp)
}
