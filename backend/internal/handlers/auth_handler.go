package handlers

import (
	"bytes"
	"crypto/subtle"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"wikikeeper-backend/internal/config"
)

// AuthHandler handles authentication HTTP requests
type AuthHandler struct {
	config *config.Config
}

var loginTemplate = template.Must(template.New("login").Parse(`<!doctype html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Admin login - WikiKeeper</title>
    <script src="https://cdn.tailwindcss.com"></script>
</head>
<body class="min-h-screen bg-gray-50 flex items-center justify-center px-4">
    <main class="w-full max-w-sm">
        <a href="/" class="block mb-6 text-center text-xl font-bold text-blue-600">WikiKeeper</a>
        <form method="post" action="/login" class="bg-white border border-gray-200 rounded-lg p-6 shadow-sm">
            <h1 class="text-lg font-semibold text-gray-900 mb-5">Admin login</h1>
            {{if .Error}}<p role="alert" class="mb-4 text-sm text-red-700">{{.Error}}</p>{{end}}
            <input type="hidden" name="redirect_to" value="{{.RedirectTo}}">
            <label for="token" class="block text-sm font-medium text-gray-700 mb-2">Admin token</label>
            <input id="token" name="token" type="password" required autofocus autocomplete="current-password"
                class="w-full px-3 py-2 border border-gray-300 rounded focus:outline-none focus:ring-2 focus:ring-blue-500">
            <button type="submit" class="mt-5 w-full px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded hover:bg-blue-700">Login</button>
        </form>
    </main>
</body>
</html>`))

// NewAuthHandler creates a new auth handler
func NewAuthHandler(cfg *config.Config) *AuthHandler {
	return &AuthHandler{config: cfg}
}

func (h *AuthHandler) LoginPage(c echo.Context) error {
	if h.config.AdminToken == "" {
		return c.String(http.StatusServiceUnavailable, "Admin authentication is not configured")
	}
	return h.renderLogin(c, http.StatusOK, "", safeRedirect(c.QueryParam("redirect_to")))
}

func (h *AuthHandler) Login(c echo.Context) error {
	if h.config.AdminToken == "" {
		return c.String(http.StatusServiceUnavailable, "Admin authentication is not configured")
	}

	redirectTo := safeRedirect(c.FormValue("redirect_to"))
	token := c.FormValue("token")
	if subtle.ConstantTimeCompare([]byte(token), []byte(h.config.AdminToken)) != 1 {
		return h.renderLogin(c, http.StatusUnauthorized, "Invalid admin token", redirectTo)
	}

	c.SetCookie(&http.Cookie{
		Name:     "admintoken",
		Value:    token,
		Path:     "/",
		MaxAge:   int((30 * 24 * time.Hour) / time.Second),
		HttpOnly: true,
		Secure:   requestIsHTTPS(c.Request()),
		SameSite: http.SameSiteLaxMode,
	})
	return c.Redirect(http.StatusSeeOther, redirectTo)
}

func (h *AuthHandler) Logout(c echo.Context) error {
	c.SetCookie(&http.Cookie{
		Name:     "admintoken",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   requestIsHTTPS(c.Request()),
		SameSite: http.SameSiteLaxMode,
	})
	return c.Redirect(http.StatusSeeOther, "/")
}

// Check handles GET /api/auth/check
// This endpoint checks if the user has a valid admin token cookie
func (h *AuthHandler) Check(c echo.Context) error {
	// If no admin token configured, not authenticated
	if h.config.AdminToken == "" {
		return c.JSON(http.StatusOK, map[string]bool{"authenticated": false})
	}

	// Dev mode bypass: if admin token is "test", always return authenticated
	// This avoids cross-site cookie issues in local development
	if h.config.AdminToken == "test" {
		return c.JSON(http.StatusOK, map[string]bool{"authenticated": true})
	}

	cookie, err := c.Cookie("admintoken")
	if err != nil {
		return c.JSON(http.StatusOK, map[string]bool{"authenticated": false})
	}

	isAuthenticated := cookie.Value == h.config.AdminToken
	return c.JSON(http.StatusOK, map[string]bool{"authenticated": isAuthenticated})
}

func (h *AuthHandler) renderLogin(c echo.Context, status int, errorMessage, redirectTo string) error {
	data := struct {
		Error      string
		RedirectTo string
	}{errorMessage, redirectTo}
	var body bytes.Buffer
	if err := loginTemplate.Execute(&body, data); err != nil {
		return err
	}
	return c.HTMLBlob(status, body.Bytes())
}

func safeRedirect(value string) string {
	if value == "" {
		return "/"
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return "/"
	}
	decodedPath, unescapeErr := url.PathUnescape(parsed.Path)
	if unescapeErr != nil || !strings.HasPrefix(parsed.Path, "/") || strings.HasPrefix(parsed.Path, "//") || strings.Contains(decodedPath, `\`) || parsed.IsAbs() || parsed.Host != "" {
		return "/"
	}
	return parsed.RequestURI()
}

func requestIsHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	proto := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0])
	return strings.EqualFold(proto, "https")
}
