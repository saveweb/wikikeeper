package pages

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"wikikeeper-backend/internal/config"
	"wikikeeper-backend/internal/services"
)

type M map[string]any

type Pages struct {
	db               *gorm.DB
	cfg              *config.Config
	collectorService *services.CollectorService
	archiveService   *services.ArchiveService
	templateDir      string
	baseTemplates    *template.Template
}

func NewPages(db *gorm.DB, cfg *config.Config, collectorService *services.CollectorService, archiveService *services.ArchiveService) *Pages {
	templateDir := "web/templates"
	if _, err := os.Stat(templateDir); os.IsNotExist(err) {
		templateDir = filepath.Join("..", "web", "templates")
	}

	p := &Pages{
		db:               db,
		cfg:              cfg,
		collectorService: collectorService,
		archiveService:   archiveService,
		templateDir:      templateDir,
	}

	if cfg.LogLevel != "DEBUG" {
		p.baseTemplates = p.parseBaseTemplates()
	}

	return p
}

func (p *Pages) parseBaseTemplates() *template.Template {
	t := template.New("").Funcs(funcMap())
	baseFile := filepath.Join(p.templateDir, "base.html")
	t = template.Must(t.ParseFiles(baseFile))

	partialsDir := filepath.Join(p.templateDir, "partials")
	if files, err := filepath.Glob(filepath.Join(partialsDir, "*.html")); err == nil && len(files) > 0 {
		t = template.Must(t.ParseFiles(files...))
	}
	return t
}

func (p *Pages) getTemplates() *template.Template {
	if p.cfg.LogLevel == "DEBUG" {
		return p.parseBaseTemplates()
	}
	return p.baseTemplates
}

func (p *Pages) render(c echo.Context, pageFile string, data M) error {
	t, err := p.getTemplates().Clone()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "template clone: "+err.Error())
	}
	_, err = t.ParseFiles(filepath.Join(p.templateDir, pageFile))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "template parse "+pageFile+": "+err.Error())
	}

	var buf bytes.Buffer
	tmplName := "base"
	if p.isHTMX(c) {
		tmplName = "content"
	}
	if err := t.ExecuteTemplate(&buf, tmplName, data); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "template execute "+pageFile+": "+err.Error())
	}
	return c.HTMLBlob(http.StatusOK, buf.Bytes())
}

func (p *Pages) renderPartial(c echo.Context, templateName string, data M) error {
	t := p.getTemplates()
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, templateName, data); err != nil {
		return err
	}
	return c.HTMLBlob(http.StatusOK, buf.Bytes())
}

func (p *Pages) isHTMX(c echo.Context) bool {
	return c.Request().Header.Get("HX-Request") == "true"
}

func (p *Pages) isAdmin(c echo.Context) bool {
	if p.cfg.AdminToken == "" {
		return false
	}
	if p.cfg.AdminToken == "test" {
		return true
	}
	cookie, err := c.Cookie("admintoken")
	if err != nil {
		return false
	}
	return cookie.Value == p.cfg.AdminToken
}

func (p *Pages) baseData(c echo.Context, title string) M {
	return M{
		"Title":   title,
		"IsAdmin": p.isAdmin(c),
	}
}

func toJS(v any) template.JS {
	b, _ := json.Marshal(v)
	return template.JS(b)
}

func funcMap() template.FuncMap {
	return template.FuncMap{
		"formatDate":   formatDate,
		"formatNumber": formatNumber,
		"formatBytes":  formatBytes,
		"relTime":      relTime,
		"truncate":     truncate,
		"sub":          func(a, b int) int { return a - b },
		"add":          func(a, b int) int { return a + b },
		"mul":          func(a, b int) int { return a * b },
		"div":          func(a, b int) int { return a / b },
		"seq":          seq,
		"toa":          toa,
		"statusColor":  statusColor,
		"json":         toJS,
		"tolower":      strings.ToLower,
		"queryStr":     queryStr,
	}
}

func formatDate(v any) string {
	switch val := v.(type) {
	case time.Time:
		return val.Format("2006-01-02 15:04")
	case *time.Time:
		if val == nil {
			return ""
		}
		return val.Format("2006-01-02 15:04")
	case string:
		return val
	default:
		return fmt.Sprintf("%v", v)
	}
}

func formatNumber(v any) string {
	switch val := v.(type) {
	case int64:
		return strconv.FormatInt(val, 10)
	case int:
		return strconv.Itoa(val)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func formatBytes(v any) string {
	var b int64
	switch val := v.(type) {
	case int64:
		b = val
	case *int64:
		if val == nil {
			return ""
		}
		b = *val
	case int:
		b = int64(val)
	case string:
		n, _ := strconv.ParseInt(val, 10, 64)
		b = n
	default:
		return fmt.Sprintf("%v", v)
	}
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

func relTime(v any) string {
	var t time.Time
	switch val := v.(type) {
	case time.Time:
		t = val
	case *time.Time:
		if val == nil {
			return ""
		}
		t = *val
	case string:
		if val == "" {
			return ""
		}
		parsed, err := time.Parse(time.RFC3339, val)
		if err != nil {
			return val
		}
		t = parsed
	default:
		return fmt.Sprintf("%v", v)
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%dmo ago", int(d.Hours()/(24*30)))
	default:
		return fmt.Sprintf("%dy ago", int(d.Hours()/(24*365)))
	}
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}

func seq(n int) []int {
	s := make([]int, n)
	for i := range s {
		s[i] = i + 1
	}
	return s
}

func toa(v any) string {
	switch val := v.(type) {
	case *time.Time:
		if val == nil {
			return ""
		}
		return val.Format(time.RFC3339)
	case *string:
		if val == nil {
			return ""
		}
		return *val
	case *int:
		if val == nil {
			return "0"
		}
		return strconv.Itoa(*val)
	case *int64:
		if val == nil {
			return "0"
		}
		return strconv.FormatInt(*val, 10)
	case string:
		return val
	case int:
		return strconv.Itoa(val)
	case int64:
		return strconv.FormatInt(val, 10)
	case bool:
		return strconv.FormatBool(val)
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case fmt.Stringer:
		return val.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

func statusColor(v any) string {
	s := fmt.Sprintf("%v", v)
	switch s {
	case "ok":
		return "green"
	case "error":
		return "red"
	case "pending":
		return "yellow"
	case "offline":
		return "gray"
	default:
		return "gray"
	}
}

func queryStr(params map[string]string) string {
	parts := make([]string, 0, len(params))
	for k, v := range params {
		if v != "" {
			parts = append(parts, fmt.Sprintf("%s=%s", k, v))
		}
	}
	return strings.Join(parts, "&")
}

func totalPages(total int64, pageSize int) int {
	return int(math.Ceil(float64(total) / float64(pageSize)))
}

func walkTemplates(dir string) []string {
	var files []string
	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(path, ".html") {
			files = append(files, path)
		}
		return nil
	})
	return files
}
