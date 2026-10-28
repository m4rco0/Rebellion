package web

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/m4rco0/Rebellion/internal/database/models"
)

type apiResponse struct {
	Items      any   `json:"items"`
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

type apiScanItem struct {
	ID         uint       `json:"id"`
	Domain     string     `json:"domain"`
	Status     string     `json:"status"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at"`
	Error      string     `json:"error,omitempty"`
}

type apiDomainItem struct {
	ID             uint   `json:"id"`
	Name           string `json:"name"`
	Program        string `json:"program"`
	SubdomainCount int64  `json:"subdomain_count"`
	ScanCount      int64  `json:"scan_count"`
	URLCount       int64  `json:"url_count"`
}

type apiSubdomainItem struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	Domain    string `json:"domain"`
	CreatedAt string `json:"created_at"`
	URLCount  int64  `json:"url_count"`
}

type apiURLItem struct {
	ID           uint   `json:"id"`
	RawURL       string `json:"raw_url"`
	Subdomain    string `json:"subdomain"`
	Scheme       string `json:"scheme"`
	Host         string `json:"host"`
	Path         string `json:"path"`
	LastStatus   int    `json:"last_status"`
	ProbeCount   int64  `json:"probe_count"`
	DiscoveredAt string `json:"discovered_at"`
}

type apiProbeItem struct {
	ID           uint     `json:"id"`
	ScanID       uint     `json:"scan_id"`
	URL          string   `json:"url"`
	Subdomain    string   `json:"subdomain"`
	StatusCode   int      `json:"status_code"`
	Title        string   `json:"title"`
	Server       string   `json:"server"`
	ContentType  string   `json:"content_type"`
	Technologies []string `json:"technologies"`
	IPs          []string `json:"ips"`
	CNAMEs       []string `json:"cnames"`
	CDN          string   `json:"cdn"`
	Failed       bool     `json:"failed"`
	Error        string   `json:"error,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		// A resposta já começou a ser escrita; só registramos.
		_ = err
	}
}

func writeAPIError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// health verifica a conexão com o PostgreSQL. Útil para monitoramento e para
// saber se o painel está realmente servindo dados.
func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	sqlDB, err := s.db.DB()
	if err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, "banco indisponível")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		writeAPIError(w, http.StatusServiceUnavailable, "banco indisponível")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) apiStats(w http.ResponseWriter, r *http.Request) {
	db := s.db.WithContext(r.Context())

	counts := map[string]int64{}
	count := func(name string, model any, where string, args ...any) error {
		var total int64
		query := db.Model(model)
		if where != "" {
			query = query.Where(where, args...)
		}
		if err := query.Count(&total).Error; err != nil {
			return err
		}
		counts[name] = total
		return nil
	}

	queries := []error{
		count("programs", &models.Program{}, ""),
		count("domains", &models.Domain{}, ""),
		count("scans", &models.Scan{}, ""),
		count("running_scans", &models.Scan{}, "status = ?", "running"),
		count("subdomains", &models.Subdomain{}, ""),
		count("hosts", &models.Host{}, ""),
		count("urls", &models.URL{}, ""),
		count("probes", &models.HTTPProbe{}, ""),
	}
	for _, err := range queries {
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, "erro ao consultar contadores")
			return
		}
	}

	byStatus := map[string]int64{}
	var rows []labelCount
	if err := db.Raw(`SELECT status AS label, COUNT(*) AS total FROM scans GROUP BY status`).
		Scan(&rows).Error; err != nil {
		writeAPIError(w, http.StatusInternalServerError, "erro ao consultar scans")
		return
	}
	for _, row := range rows {
		byStatus[row.Label] = row.Total
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"counts":          counts,
		"scans_by_status": byStatus,
		"generated_at":    time.Now().UTC(),
	})
}

func (s *Server) apiScans(w http.ResponseWriter, r *http.Request) {
	db := s.db.WithContext(r.Context())
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	status := strings.TrimSpace(r.URL.Query().Get("status"))

	base := db.Model(&models.Scan{}).Joins("JOIN domains ON domains.id = scans.domain_id")
	if query != "" {
		base = base.Where("domains.name ILIKE ?", likePattern(query))
	}
	if status != "" {
		base = base.Where("scans.status = ?", status)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		writeAPIError(w, http.StatusInternalServerError, "erro ao consultar scans")
		return
	}

	pg := newPagination(r, total)
	var scans []models.Scan
	if err := base.Preload("Domain").
		Order("scans.started_at DESC").
		Limit(pg.PageSize).
		Offset(pg.Offset()).
		Find(&scans).Error; err != nil {
		writeAPIError(w, http.StatusInternalServerError, "erro ao consultar scans")
		return
	}

	items := make([]apiScanItem, 0, len(scans))
	for _, scan := range scans {
		items = append(items, apiScanItem{
			ID:         scan.ID,
			Domain:     scan.Domain.Name,
			Status:     scan.Status,
			StartedAt:  scan.StartedAt,
			FinishedAt: scan.FinishedAt,
			Error:      scan.Error,
		})
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Items: items, Page: pg.Page, PageSize: pg.PageSize, Total: total, TotalPages: pg.TotalPages,
	})
}

func (s *Server) apiDomains(w http.ResponseWriter, r *http.Request) {
	db := s.db.WithContext(r.Context())
	query := strings.TrimSpace(r.URL.Query().Get("q"))

	base := db.Model(&models.Domain{}).Joins("JOIN programs ON programs.id = domains.program_id")
	if query != "" {
		base = base.Where("domains.name ILIKE ?", likePattern(query))
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		writeAPIError(w, http.StatusInternalServerError, "erro ao consultar domínios")
		return
	}

	pg := newPagination(r, total)
	var rows []domainRow
	if err := base.
		Select(`domains.*, programs.name AS program_name,
			(SELECT COUNT(*) FROM subdomains WHERE subdomains.domain_id = domains.id) AS subdomain_count,
			(SELECT COUNT(*) FROM scans WHERE scans.domain_id = domains.id) AS scan_count,
			(SELECT COUNT(*) FROM urls
				JOIN subdomains ON subdomains.id = urls.subdomain_id
				WHERE subdomains.domain_id = domains.id) AS url_count`).
		Order("domains.name").
		Limit(pg.PageSize).
		Offset(pg.Offset()).
		Scan(&rows).Error; err != nil {
		writeAPIError(w, http.StatusInternalServerError, "erro ao consultar domínios")
		return
	}

	items := make([]apiDomainItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, apiDomainItem{
			ID:             row.ID,
			Name:           row.Name,
			Program:        row.ProgramName,
			SubdomainCount: row.SubdomainCount,
			ScanCount:      row.ScanCount,
			URLCount:       row.URLCount,
		})
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Items: items, Page: pg.Page, PageSize: pg.PageSize, Total: total, TotalPages: pg.TotalPages,
	})
}

func (s *Server) apiSubdomains(w http.ResponseWriter, r *http.Request) {
	db := s.db.WithContext(r.Context())
	query := strings.TrimSpace(r.URL.Query().Get("q"))

	base := db.Model(&models.Subdomain{}).Joins("JOIN domains ON domains.id = subdomains.domain_id")
	if query != "" {
		base = base.Where("subdomains.name ILIKE ?", likePattern(query))
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		writeAPIError(w, http.StatusInternalServerError, "erro ao consultar subdomínios")
		return
	}

	pg := newPagination(r, total)
	var rows []subdomainRow
	if err := base.
		Select(`subdomains.*, domains.name AS domain_name,
			(SELECT COUNT(*) FROM urls WHERE urls.subdomain_id = subdomains.id) AS url_count`).
		Order("subdomains.name").
		Limit(pg.PageSize).
		Offset(pg.Offset()).
		Scan(&rows).Error; err != nil {
		writeAPIError(w, http.StatusInternalServerError, "erro ao consultar subdomínios")
		return
	}

	items := make([]apiSubdomainItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, apiSubdomainItem{
			ID:        row.ID,
			Name:      row.Name,
			Domain:    row.DomainName,
			CreatedAt: row.CreatedAt.UTC().Format(time.RFC3339),
			URLCount:  row.URLCount,
		})
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Items: items, Page: pg.Page, PageSize: pg.PageSize, Total: total, TotalPages: pg.TotalPages,
	})
}

func (s *Server) apiURLs(w http.ResponseWriter, r *http.Request) {
	db := s.db.WithContext(r.Context())
	query := strings.TrimSpace(r.URL.Query().Get("q"))

	base := db.Model(&models.URL{}).Joins("JOIN subdomains ON subdomains.id = urls.subdomain_id")
	if query != "" {
		base = base.Where("urls.raw_url ILIKE ?", likePattern(query))
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		writeAPIError(w, http.StatusInternalServerError, "erro ao consultar URLs")
		return
	}

	pg := newPagination(r, total)
	var rows []urlRow
	if err := base.
		Select(`urls.*, subdomains.name AS subdomain_name,
			(SELECT status_code FROM http_probes WHERE http_probes.url_id = urls.id ORDER BY id DESC LIMIT 1) AS last_status,
			(SELECT COUNT(*) FROM http_probes WHERE http_probes.url_id = urls.id) AS probe_count`).
		Order("urls.id DESC").
		Limit(pg.PageSize).
		Offset(pg.Offset()).
		Scan(&rows).Error; err != nil {
		writeAPIError(w, http.StatusInternalServerError, "erro ao consultar URLs")
		return
	}

	items := make([]apiURLItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, apiURLItem{
			ID:           row.ID,
			RawURL:       row.RawURL,
			Subdomain:    row.SubdomainName,
			Scheme:       row.Scheme,
			Host:         row.Host,
			Path:         row.Path,
			LastStatus:   row.LastStatus,
			ProbeCount:   row.ProbeCount,
			DiscoveredAt: row.CreatedAt.UTC().Format(time.RFC3339),
		})
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Items: items, Page: pg.Page, PageSize: pg.PageSize, Total: total, TotalPages: pg.TotalPages,
	})
}

func (s *Server) apiProbes(w http.ResponseWriter, r *http.Request) {
	db := s.db.WithContext(r.Context())
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	failed := strings.TrimSpace(r.URL.Query().Get("failed"))

	base := db.Model(&models.HTTPProbe{}).
		Joins("JOIN urls ON urls.id = http_probes.url_id").
		Joins("JOIN subdomains ON subdomains.id = urls.subdomain_id")
	if query != "" {
		pattern := likePattern(query)
		base = base.Where("urls.raw_url ILIKE ? OR subdomains.name ILIKE ?", pattern, pattern)
	}
	if code != "" {
		base = base.Where("http_probes.status_code = ?", code)
	}
	switch failed {
	case "yes":
		base = base.Where("http_probes.failed = ?", true)
	case "no":
		base = base.Where("http_probes.failed = ?", false)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		writeAPIError(w, http.StatusInternalServerError, "erro ao consultar probes")
		return
	}

	pg := newPagination(r, total)
	var probes []models.HTTPProbe
	if err := base.
		Preload("URL").
		Preload("URL.Subdomain").
		Order("http_probes.id DESC").
		Limit(pg.PageSize).
		Offset(pg.Offset()).
		Find(&probes).Error; err != nil {
		writeAPIError(w, http.StatusInternalServerError, "erro ao consultar probes")
		return
	}

	items := make([]apiProbeItem, 0, len(probes))
	for _, probe := range probes {
		items = append(items, apiProbeItem{
			ID:           probe.ID,
			ScanID:       probe.ScanID,
			URL:          probe.URL.RawURL,
			Subdomain:    probe.URL.Subdomain.Name,
			StatusCode:   probe.StatusCode,
			Title:        probe.Title,
			Server:       probe.Server,
			ContentType:  probe.ContentType,
			Technologies: probe.Technologies,
			IPs:          probe.IPs,
			CNAMEs:       probe.CNAMEs,
			CDN:          probe.CDN,
			Failed:       probe.Failed,
			Error:        probe.Error,
		})
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Items: items, Page: pg.Page, PageSize: pg.PageSize, Total: total, TotalPages: pg.TotalPages,
	})
}
