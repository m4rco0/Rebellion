package web

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/m4rco0/Rebellion/internal/database/models"
)

// sampleData monta um conjunto pequeno, mas representativo, de dados para
// executar todos os templates sem precisar de banco.
func sampleData() (models.Scan, models.Subdomain, models.URL, models.HTTPProbe) {
	now := time.Date(2026, 10, 28, 15, 30, 0, 0, time.UTC)

	program := models.Program{Platform: "manual", Name: "manual"}
	program.ID = 1

	domain := models.Domain{ProgramID: 1, Name: "example.com", Program: program}
	domain.ID = 1
	domain.CreatedAt = now
	domain.UpdatedAt = now

	finished := now.Add(3*time.Minute + 20*time.Second)
	scan := models.Scan{
		DomainID:   1,
		Status:     "completed",
		StartedAt:  now,
		FinishedAt: &finished,
		Domain:     domain,
	}
	scan.ID = 1
	scan.CreatedAt = now
	scan.UpdatedAt = now

	subdomain := models.Subdomain{DomainID: 1, Name: "api.example.com", Domain: domain}
	subdomain.ID = 1
	subdomain.CreatedAt = now

	u := models.URL{
		SubdomainID: 1,
		RawURL:      "https://api.example.com/v1/users?page=1",
		Hash:        "0123456789abcdef",
		Scheme:      "https",
		Host:        "api.example.com",
		Path:        "/v1/users",
		Query:       "page=1",
		Subdomain:   subdomain,
	}
	u.ID = 1
	u.CreatedAt = now

	probe := models.HTTPProbe{
		ScanID:        1,
		URLID:         1,
		StatusCode:    200,
		Title:         "API de usuários",
		Server:        "nginx/1.25",
		ContentType:   "application/json",
		ContentLength: 512,
		IPs:           []string{"1.2.3.4"},
		CNAMEs:        []string{"cdn.example.net"},
		CDN:           "cloudflare",
		Technologies:  []string{"nginx", "go"},
		ResponseTime:  "42ms",
		URL:           u,
		Scan:          scan,
	}
	probe.ID = 1
	probe.CreatedAt = now

	return scan, subdomain, u, probe
}

func testPagination() pagination {
	return pagination{
		Path:       "/urls",
		Filter:     "&q=api",
		Page:       2,
		PageSize:   25,
		Total:      60,
		TotalPages: 3,
		PrevPage:   1,
		NextPage:   3,
		HasPrev:    true,
		HasNext:    true,
	}
}

func TestTemplatesRender(t *testing.T) {
	templates, err := loadTemplates()
	if err != nil {
		t.Fatalf("carregar templates: %v", err)
	}
	s := &Server{templates: templates}

	scan, subdomain, u, probe := sampleData()
	pg := testPagination()
	page := pageData{Title: "Teste", Active: "dashboard"}

	domainRowSample := domainRow{
		Domain:         scan.Domain,
		ProgramName:    "manual",
		SubdomainCount: 12,
		ScanCount:      3,
		URLCount:       40,
	}
	subdomainRowSample := subdomainRow{
		Subdomain:  subdomain,
		DomainName: "example.com",
		URLCount:   8,
	}
	urlRowSample := urlRow{
		URL:           u,
		SubdomainName: "api.example.com",
		LastStatus:    200,
		ProbeCount:    2,
	}
	resolutionSample := resolutionRow{
		Resolution:    models.Resolution{ScanID: 1, SubdomainID: 1, HostID: 1},
		SubdomainName: "api.example.com",
		IP:            "1.2.3.4",
	}

	cases := []struct {
		name string
		page string
		data any
	}{
		{
			name: "dashboard",
			page: "dashboard.html",
			data: dashboardPage{
				pageData: page,
				Counts: dashboardCounts{
					Programs: 1, Domains: 1, Scans: 3, Running: 1,
					Subdomains: 12, Hosts: 4, URLs: 40, Probes: 25,
				},
				RecentScans:     []models.Scan{scan},
				ScanStatus:      []labelCount{{Label: "completed", Total: 2}, {Label: "failed", Total: 1}},
				TopDomains:      []labelCount{{Label: "example.com", Total: 12}},
				TopCodes:        []codeCount{{Code: 200, Total: 20}, {Code: 404, Total: 5}},
				TopTechnologies: []labelCount{{Label: "nginx", Total: 9}},
			},
		},
		{
			name: "scans",
			page: "scans.html",
			data: scansPage{
				pageData:     pageData{Title: "Scans", Active: "scans"},
				Pagination:   pg,
				Scans:        []models.Scan{scan},
				Query:        "example",
				StatusFilter: "completed",
			},
		},
		{
			name: "scan_detail",
			page: "scan_detail.html",
			data: scanDetailPage{
				pageData:    pageData{Title: "Scan", Active: "scans"},
				Scan:        scan,
				Subdomains:  []scanSubdomainRow{{Subdomain: subdomain, Source: "subfinder"}},
				Resolutions: []resolutionRow{resolutionSample},
				Probes:      []models.HTTPProbe{probe},
				Counts:      struct{ Subdomains, Resolutions, Probes int64 }{1, 1, 1},
			},
		},
		{
			name: "domains",
			page: "domains.html",
			data: domainsPage{
				pageData:   pageData{Title: "Domínios", Active: "domains"},
				Pagination: pg,
				Domains:    []domainRow{domainRowSample},
				Query:      "example",
			},
		},
		{
			name: "domain_detail",
			page: "domain_detail.html",
			data: domainDetailPage{
				pageData:   pageData{Title: "example.com", Active: "domains"},
				Domain:     scan.Domain,
				Counts:     struct{ Subdomains, Scans, URLs, Probes int64 }{12, 3, 40, 25},
				Scans:      []models.Scan{scan},
				Subdomains: []subdomainRow{subdomainRowSample},
				Pagination: pg,
			},
		},
		{
			name: "subdomains",
			page: "subdomains.html",
			data: subdomainsPage{
				pageData:   pageData{Title: "Subdomínios", Active: "subdomains"},
				Pagination: pg,
				Subdomains: []subdomainRow{subdomainRowSample},
				Query:      "api",
			},
		},
		{
			name: "subdomain_detail",
			page: "subdomain_detail.html",
			data: subdomainDetailPage{
				pageData:    pageData{Title: "api.example.com", Active: "subdomains"},
				Subdomain:   subdomain,
				Counts:      struct{ URLs, Resolutions, Probes int64 }{8, 2, 5},
				Resolutions: []resolutionRow{resolutionSample},
				URLs:        []urlRow{urlRowSample},
				Probes:      []models.HTTPProbe{probe},
				Pagination:  pg,
			},
		},
		{
			name: "graph",
			page: "graph.html",
			data: graphPage{
				pageData: pageData{Title: "Grafo", Active: "graph"},
				Domains: []graphDomainOption{
					{ID: 1, Name: "example.com", Subdomains: 12},
					{ID: 2, Name: "outro.dev", Subdomains: 3},
				},
				DomainID:    1,
				Limit:       40,
				GraphURL:    "/api/graph?domain=1&limit=40",
				LimitValues: []int{10, 25, 50},
			},
		},
		{
			name: "urls",
			page: "urls.html",
			data: urlsPage{
				pageData:   pageData{Title: "URLs", Active: "urls"},
				Pagination: pg,
				URLs:       []urlRow{urlRowSample},
				Query:      "api",
			},
		},
		{
			name: "probes",
			page: "probes.html",
			data: probesPage{
				pageData:   pageData{Title: "HTTP", Active: "probes"},
				Pagination: pg,
				Probes:     []models.HTTPProbe{probe},
				Query:      "api",
				Code:       "200",
				Failed:     "no",
			},
		},
		{
			name: "error",
			page: "error.html",
			data: errorPage{
				pageData: pageData{Title: "Erro"},
				Status:   http.StatusNotFound,
				Message:  "Página não encontrada",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			s.render(rec, http.StatusOK, tc.page, tc.data)

			if rec.Code != http.StatusOK {
				t.Fatalf("status %d, esperado 200; corpo: %s", rec.Code, rec.Body.String())
			}
			if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
				t.Errorf("Content-Type %q", ct)
			}
			body := rec.Body.String()
			if !strings.Contains(body, "Rebellion") {
				t.Errorf("página sem o layout esperado: %.120q", body)
			}
			if strings.Contains(body, "erro ao renderizar") {
				t.Errorf("template falhou: %s", body)
			}
		})
	}
}

func TestTemplatesEmptyStates(t *testing.T) {
	templates, err := loadTemplates()
	if err != nil {
		t.Fatalf("carregar templates: %v", err)
	}
	s := &Server{templates: templates}

	cases := []struct {
		page string
		data any
	}{
		{"dashboard.html", dashboardPage{pageData: pageData{Title: "Visão geral"}}},
		{"scans.html", scansPage{pageData: pageData{Title: "Scans"}}},
		{"domains.html", domainsPage{pageData: pageData{Title: "Domínios"}}},
		{"subdomains.html", subdomainsPage{pageData: pageData{Title: "Subdomínios"}}},
		{"graph.html", graphPage{pageData: pageData{Title: "Grafo"}}},
		{"urls.html", urlsPage{pageData: pageData{Title: "URLs"}}},
		{"probes.html", probesPage{pageData: pageData{Title: "HTTP"}}},
	}

	for _, tc := range cases {
		t.Run(tc.page, func(t *testing.T) {
			rec := httptest.NewRecorder()
			s.render(rec, http.StatusOK, tc.page, tc.data)
			if rec.Code != http.StatusOK {
				t.Fatalf("status %d; corpo: %s", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), "Nada por aqui") {
				t.Errorf("estado vazio não renderizado")
			}
		})
	}
}

func TestBasicAuth(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	handler := BasicAuth("admin", "segredo", next)

	request := func(user, password string, withAuth bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if withAuth {
			req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(user+":"+password)))
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}

	if rec := request("", "", false); rec.Code != http.StatusUnauthorized {
		t.Errorf("sem credenciais: status %d, esperado 401", rec.Code)
	}
	if rec := request("admin", "errado", true); rec.Code != http.StatusUnauthorized {
		t.Errorf("senha errada: status %d, esperado 401", rec.Code)
	}
	if rec := request("admin", "segredo", true); rec.Code != http.StatusNoContent {
		t.Errorf("credenciais corretas: status %d, esperado 204", rec.Code)
	}

	passthrough := BasicAuth("", "", next)
	rec := httptest.NewRecorder()
	passthrough.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusNoContent {
		t.Errorf("sem configuração deveria passar direto, obteve %d", rec.Code)
	}
}

func TestBasicAuthAllowHealth(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	handler := BasicAuthAllowHealth("admin", "segredo", next)

	health := httptest.NewRecorder()
	handler.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if health.Code != http.StatusNoContent {
		t.Errorf("/healthz deveria ignorar a autenticação, obteve %d", health.Code)
	}

	dashboard := httptest.NewRecorder()
	handler.ServeHTTP(dashboard, httptest.NewRequest(http.MethodGet, "/", nil))
	if dashboard.Code != http.StatusUnauthorized {
		t.Errorf("/ deveria exigir autenticação, obteve %d", dashboard.Code)
	}
}

func TestSecurityHeaders(t *testing.T) {
	rec := httptest.NewRecorder()
	handler := securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	expected := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "no-referrer",
	}
	for header, want := range expected {
		if got := rec.Header().Get(header); got != want {
			t.Errorf("%s = %q; esperado %q", header, got, want)
		}
	}
	if rec.Header().Get("Content-Security-Policy") == "" {
		t.Error("Content-Security-Policy ausente")
	}
}

func TestNewPagination(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/urls?page=2&page_size=10&q=api&code=200", nil)
	pg := newPagination(req, 35)

	if pg.Page != 2 || pg.PageSize != 10 {
		t.Errorf("paginação incorreta: %+v", pg)
	}
	if pg.TotalPages != 4 || !pg.HasPrev || !pg.HasNext {
		t.Errorf("limites incorretos: %+v", pg)
	}
	if pg.Filter != "&code=200&q=api" {
		t.Errorf("filtros não preservados: %q", pg.Filter)
	}

	overflow := newPagination(httptest.NewRequest(http.MethodGet, "/urls?page=99", nil), 5)
	if overflow.Page != 1 || overflow.TotalPages != 1 {
		t.Errorf("página fora do intervalo deveria ser ajustada: %+v", overflow)
	}

	capped := newPagination(httptest.NewRequest(http.MethodGet, "/urls?page_size=9999", nil), 5)
	if capped.PageSize != maxPageSize {
		t.Errorf("page_size deveria ser limitado a %d, obteve %d", maxPageSize, capped.PageSize)
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("abcdef", 4); got != "abc…" {
		t.Errorf("truncate = %q", got)
	}
	if got := truncate("abc", 10); got != "abc" {
		t.Errorf("truncate curto = %q", got)
	}
}

func TestPercent(t *testing.T) {
	cases := []struct {
		part, total int64
		want        int
	}{
		{0, 0, 0},
		{0, 10, 0},
		{5, 10, 50},
		{1, 1000, 1},
		{20, 10, 100},
	}
	for _, tc := range cases {
		if got := percent(tc.part, tc.total); got != tc.want {
			t.Errorf("percent(%d, %d) = %d; esperado %d", tc.part, tc.total, got, tc.want)
		}
	}
}
