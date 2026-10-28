package web

import (
	"bytes"
	"crypto/subtle"
	"embed"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	defaultPageSize = 25
	maxPageSize     = 100
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static
var staticFS embed.FS

// Server expõe uma interface web somente-leitura sobre o banco do Rebellion.
type Server struct {
	db        *gorm.DB
	templates map[string]*template.Template
	mux       *http.ServeMux
}

func New(db *gorm.DB) (*Server, error) {
	templates, err := loadTemplates()
	if err != nil {
		return nil, err
	}

	s := &Server{
		db:        db,
		templates: templates,
		mux:       http.NewServeMux(),
	}
	s.routes()

	return s, nil
}

// Handler devolve o http.Handler com cabeçalhos de segurança, logging e
// recuperação de panics.
func (s *Server) Handler() http.Handler {
	return securityHeaders(logRequests(recoverPanics(s.mux)))
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; "+
				"script-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) routes() {
	s.mux.Handle("GET /static/", http.FileServer(http.FS(staticFS)))

	s.mux.HandleFunc("GET /{$}", s.dashboard)
	s.mux.HandleFunc("GET /scans", s.scans)
	s.mux.HandleFunc("GET /scans/{id}", s.scanDetail)
	s.mux.HandleFunc("GET /domains", s.domains)
	s.mux.HandleFunc("GET /domains/{id}", s.domainDetail)
	s.mux.HandleFunc("GET /subdomains", s.subdomains)
	s.mux.HandleFunc("GET /subdomains/{id}", s.subdomainDetail)
	s.mux.HandleFunc("GET /graph", s.graph)
	s.mux.HandleFunc("GET /urls", s.urls)
	s.mux.HandleFunc("GET /probes", s.probes)

	s.mux.HandleFunc("GET /api/stats", s.apiStats)
	s.mux.HandleFunc("GET /healthz", s.health)
	s.mux.HandleFunc("GET /api/scans", s.apiScans)
	s.mux.HandleFunc("GET /api/domains", s.apiDomains)
	s.mux.HandleFunc("GET /api/subdomains", s.apiSubdomains)
	s.mux.HandleFunc("GET /api/graph", s.apiGraph)
	s.mux.HandleFunc("GET /api/urls", s.apiURLs)
	s.mux.HandleFunc("GET /api/probes", s.apiProbes)

	s.mux.HandleFunc("/", s.notFound)
}

type pageData struct {
	Title  string
	Active string
}

type errorPage struct {
	pageData
	Status  int
	Message string
}

func (s *Server) render(w http.ResponseWriter, status int, page string, data any) {
	t, ok := s.templates[page]
	if !ok {
		log.Printf("template %q não encontrado", page)
		http.Error(w, "erro interno", http.StatusInternalServerError)
		return
	}

	// Renderiza em memória antes de enviar: se um campo faltar ou um template
	// falhar, o cliente recebe uma resposta de erro em vez de uma página
	// truncada.
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "layout", data); err != nil {
		log.Printf("renderizar %s: %v", page, err)
		http.Error(w, "erro ao renderizar a página", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if _, err := buf.WriteTo(w); err != nil {
		log.Printf("enviar %s: %v", page, err)
	}
}

func (s *Server) renderError(w http.ResponseWriter, r *http.Request, status int, message string) {
	s.render(w, status, "error.html", errorPage{
		pageData: pageData{Title: "Erro", Active: ""},
		Status:   status,
		Message:  message,
	})
}

func (s *Server) notFound(w http.ResponseWriter, r *http.Request) {
	s.renderError(w, r, http.StatusNotFound, "Página não encontrada")
}

func (s *Server) internalError(w http.ResponseWriter, r *http.Request, err error) {
	log.Printf("%s %s: %v", r.Method, r.URL.Path, err)
	s.renderError(w, r, http.StatusInternalServerError, "Erro ao consultar o banco de dados")
}

func loadTemplates() (map[string]*template.Template, error) {
	pages := []string{
		"dashboard.html",
		"scans.html",
		"scan_detail.html",
		"domains.html",
		"domain_detail.html",
		"subdomains.html",
		"subdomain_detail.html",
		"graph.html",
		"urls.html",
		"probes.html",
		"error.html",
	}

	out := make(map[string]*template.Template, len(pages))
	for _, page := range pages {
		t, err := template.New("layout").Funcs(templateFuncs).ParseFS(
			templatesFS,
			"templates/layout.html",
			"templates/partials.html",
			"templates/"+page,
		)
		if err != nil {
			return nil, fmt.Errorf("carregar template %s: %w", page, err)
		}
		out[page] = t
	}

	return out, nil
}

var templateFuncs = template.FuncMap{
	"fmtTime":     fmtTime,
	"fmtTimePtr":  fmtTimePtr,
	"duration":    formatDuration,
	"statusClass": statusClass,
	"httpClass":   httpClass,
	"add":         func(a, b int) int { return a + b },
	"sub":         func(a, b int) int { return a - b },
	"join":        strings.Join,
	"pct":         percent,
	"truncate":    truncate,
}

// BasicAuth protege o painel com HTTP Basic Auth quando usuário e senha são
// configurados. Sem credenciais (user vazio), não faz nada.
func BasicAuth(user, password string, next http.Handler) http.Handler {
	if user == "" {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser, gotPassword, ok := r.BasicAuth()
		if !ok ||
			subtle.ConstantTimeCompare([]byte(gotUser), []byte(user)) != 1 ||
			subtle.ConstantTimeCompare([]byte(gotPassword), []byte(password)) != 1 {
			w.Header().Set("WWW-Authenticate", `Basic realm="Rebellion", charset="UTF-8"`)
			http.Error(w, "autenticação necessária", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// BasicAuthAllowHealth aplica BasicAuth, mas mantém /healthz aberto para que
// orquestradores (Docker, Kubernetes) consigam verificar a saúde do processo
// sem conhecer as credenciais. O endpoint só devolve o estado do banco.
func BasicAuthAllowHealth(user, password string, next http.Handler) http.Handler {
	protected := BasicAuth(user, password, next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}
		protected.ServeHTTP(w, r)
	})
}

func percent(part, total int64) int {
	if total <= 0 || part <= 0 {
		return 0
	}
	value := int(float64(part) / float64(total) * 100)
	if value < 1 {
		return 1
	}
	if value > 100 {
		return 100
	}
	return value
}

func truncate(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	if limit <= 1 {
		return string(runes[:limit])
	}
	return string(runes[:limit-1]) + "…"
}

func fmtTime(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.Local().Format("02/01/2006 15:04")
}

func fmtTimePtr(t *time.Time) string {
	if t == nil {
		return "—"
	}
	return fmtTime(*t)
}

func formatDuration(start time.Time, end *time.Time) string {
	if start.IsZero() {
		return "—"
	}

	finish := time.Now()
	if end != nil {
		finish = *end
	}
	d := finish.Sub(start)
	if d < 0 {
		d = 0
	}

	switch {
	case d < time.Minute:
		return fmt.Sprintf("%.0fs", d.Seconds())
	case d < time.Hour:
		return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
	default:
		return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
	}
}

func statusClass(status string) string {
	switch status {
	case "completed":
		return "ok"
	case "failed":
		return "err"
	case "running":
		return "warn"
	default:
		return "muted"
	}
}

func httpClass(code int) string {
	switch {
	case code >= 200 && code < 300:
		return "ok"
	case code >= 300 && code < 400:
		return "info"
	case code >= 400 && code < 500:
		return "warn"
	case code >= 500:
		return "err"
	default:
		return "muted"
	}
}

func queryInt(r *http.Request, key string, def, min, max int) int {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return def
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func parseID(r *http.Request) (uint, bool) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil || id == 0 {
		return 0, false
	}
	return uint(id), true
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s (%s)", r.Method, r.URL.RequestURI(), time.Since(start).Round(time.Millisecond))
	})
}

func recoverPanics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Printf("panic em %s %s: %v", r.Method, r.URL.Path, recovered)
				http.Error(w, "erro interno", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
