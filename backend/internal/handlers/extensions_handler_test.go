package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

func TestExtensionsHistoryRejectsReverseTimeRange(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/wikis/test/extensions/history?from=2026-08-01T00:00:00Z&to=2026-07-01T00:00:00Z",
		nil,
	)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(uuid.NewString())

	handler := NewExtensionsHandler(nil)
	require.NoError(t, handler.GetExtensionsHistory(c))
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.JSONEq(t, `{"detail":"from time must not be after to time"}`, rec.Body.String())
}
