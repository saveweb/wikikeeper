package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"wikikeeper-backend/internal/config"
)

func TestLoginSetsAdminCookie(t *testing.T) {
	e := echo.New()
	h := NewAuthHandler(&config.Config{AdminToken: "secret"})
	form := url.Values{"token": {"secret"}, "redirect_to": {"/wikis/1?tab=stats"}}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()

	if err := h.Login(e.NewContext(req, rec)); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/wikis/1?tab=stats" {
		t.Fatalf("unexpected redirect: status=%d location=%q", rec.Code, rec.Header().Get("Location"))
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("got %d cookies, want 1", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != "admintoken" || cookie.Value != "secret" || !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("unexpected cookie: %#v", cookie)
	}
}

func TestLoginRejectsInvalidToken(t *testing.T) {
	e := echo.New()
	h := NewAuthHandler(&config.Config{AdminToken: "secret"})
	form := url.Values{"token": {"wrong"}}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	rec := httptest.NewRecorder()

	if err := h.Login(e.NewContext(req, rec)); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), "Invalid admin token") {
		t.Fatalf("unexpected response: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if len(rec.Result().Cookies()) != 0 {
		t.Fatal("invalid login set a cookie")
	}
}

func TestLoginRejectsExternalRedirect(t *testing.T) {
	e := echo.New()
	h := NewAuthHandler(&config.Config{AdminToken: "secret"})
	form := url.Values{"token": {"secret"}, "redirect_to": {"https://example.com/steal"}}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	rec := httptest.NewRecorder()

	if err := h.Login(e.NewContext(req, rec)); err != nil {
		t.Fatal(err)
	}
	if got := rec.Header().Get("Location"); got != "/" {
		t.Fatalf("redirect location = %q, want /", got)
	}
}

func TestSafeRedirectRejectsBrowserNormalizedHost(t *testing.T) {
	for _, redirect := range []string{`//example.com/steal`, `/\example.com/steal`, `/%5cexample.com/steal`} {
		if got := safeRedirect(redirect); got != "/" {
			t.Errorf("safeRedirect(%q) = %q, want /", redirect, got)
		}
	}
}

func TestLogoutClearsAdminCookie(t *testing.T) {
	e := echo.New()
	h := NewAuthHandler(&config.Config{AdminToken: "secret"})
	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	rec := httptest.NewRecorder()

	if err := h.Logout(e.NewContext(req, rec)); err != nil {
		t.Fatal(err)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "admintoken" || cookies[0].MaxAge != -1 {
		t.Fatalf("unexpected cookies: %#v", cookies)
	}
}
