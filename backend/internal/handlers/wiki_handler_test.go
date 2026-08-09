package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWikiHandlerListRejectsInvalidOrder(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet,
		"/api/wikis?order_by=updated_at%20DESC%3B%20DROP%20TABLE%20wikis", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	handler := NewWikiHandler(nil, nil, nil)

	require.NoError(t, handler.List(c))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"detail":"Invalid order_by value"}`, rec.Body.String())
}

func TestWikiHandlerListRejectsInvalidIsActive(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/wikis?is_active=disabled", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	handler := NewWikiHandler(nil, nil, nil)

	require.NoError(t, handler.List(c))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"detail":"Invalid is_active value"}`, rec.Body.String())
}

func TestWikiHandlerListRejectsOfflineStatus(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/wikis?status=offline", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	handler := NewWikiHandler(nil, nil, nil)

	require.NoError(t, handler.List(c))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"detail":"Invalid status value"}`, rec.Body.String())
}
