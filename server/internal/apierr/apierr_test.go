package apierr_test

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"

	"github.com/packalares/localmaps/server/internal/apierr"
)

func TestWriteMatchesOpenAPIShape(t *testing.T) {
	app := fiber.New()
	app.Get("/boom", func(c fiber.Ctx) error {
		c.Locals("traceId", "a7f3e9d1b2c4")
		return apierr.Write(c, apierr.CodeRegionNotReady,
			"Romania is still downloading (45%)", true)
	})

	req := httptest.NewRequest("GET", "/boom", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusServiceUnavailable, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &got))

	// Top-level keys per openapi.yaml ErrorResponse schema.
	require.Contains(t, got, "error")
	require.Contains(t, got, "traceId")
	require.Equal(t, "a7f3e9d1b2c4", got["traceId"])

	errObj, ok := got["error"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "REGION_NOT_READY", errObj["code"])
	require.Equal(t, "Romania is still downloading (45%)", errObj["message"])
	require.Equal(t, true, errObj["retryable"])
}

func TestHTTPStatusMapping(t *testing.T) {
	cases := []struct {
		code   apierr.ErrorCode
		status int
	}{
		{apierr.CodeBadRequest, fiber.StatusBadRequest},
		{apierr.CodeUnauthorized, fiber.StatusUnauthorized},
		{apierr.CodeForbidden, fiber.StatusForbidden},
		{apierr.CodeNotFound, fiber.StatusNotFound},
		{apierr.CodeConflict, fiber.StatusConflict},
		{apierr.CodeRateLimited, fiber.StatusTooManyRequests},
		{apierr.CodeInternal, fiber.StatusInternalServerError},
		{apierr.CodeUpstreamUnavailable, fiber.StatusBadGateway},
		{apierr.CodeRegionNotReady, fiber.StatusServiceUnavailable},
		{apierr.CodeJobNotFound, fiber.StatusNotFound},
		{apierr.CodeEgressDenied, fiber.StatusForbidden},
	}
	for _, tc := range cases {
		app := fiber.New()
		app.Get("/", func(c fiber.Ctx) error {
			return apierr.Write(c, tc.code, "x", false)
		})
		resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
		require.NoError(t, err)
		require.Equalf(t, tc.status, resp.StatusCode, "code %s", tc.code)
	}
}
