package web

import (
	"net/http"
	"strings"

	"github.com/m4rco0/Rebellion/internal/database/models"
	"gorm.io/gorm"
)

// -----------------------------------------------------------------------------
// Tipos de página
// -----------------------------------------------------------------------------

type dashboardCounts struct {
	Programs   int64
	Domains    int64
	Scans      int64
	Running    int64
	Subdomains int64
	Hosts      int64
	URLs       int64
	Probes     int64
}

type labelCount struct {
	Label string
	Total int64
}

type codeCount struct {
	Code  int
	Total int64
}

type dashboardPage struct {
	pageData
	Counts          dashboardCounts
	RecentScans     []models.Scan
	ScanStatus      []labelCount
	TopDomains      []labelCount
	TopCodes        []codeCount
	TopTechnologies []labelCount
}

type scansPage struct {
	pageData
	Pagination   pagination
	Scans        []models.Scan
	Query        string
	StatusFilter string
}

type scanSubdomainRow struct {
	models.Subdomain
	Source string
}

type resolutionRow struct {
	models.Resolution
	SubdomainName string
	IP            string
}

type scanDetailPage struct {
	pageData
	Scan         models.Scan
	Subdomains   []scanSubdomainRow
	Resolutions  []resolutionRow
	Probes       []models.HTTPProbe
	Counts       struct{ Subdomains, Resolutions, Probes int64 }
	ProbesCutoff bool
}

type domainRow struct {
	models.Domain
	ProgramName    string
	SubdomainCount int64
	ScanCount      int64
	URLCount       int64
}

type domainsPage struct {
	pageData
	Pagination pagination
	Domains    []domainRow
	Query      string
}

type domainDetailPage struct {
	pageData
	Domain     models.Domain
	Counts     struct{ Subdomains, Scans, URLs, Probes int64 }
	Scans      []models.Scan
	Subdomains []subdomainRow
	Pagination pagination
}

type subdomainRow struct {
	models.Subdomain
	DomainName string
	URLCount   int64
}

type subdomainsPage struct {
	pageData
	Pagination pagination
	Subdomains []subdomainRow
	Query      string
}

type subdomainDetailPage struct {
	pageData
	Subdomain   models.Subdomain
	Counts      struct{ URLs, Resolutions, Probes int64 }
	Resolutions []resolutionRow
	URLs        []urlRow
	Probes      []models.HTTPProbe
	Pagination  pagination
}

type urlRow struct {
	models.URL
	SubdomainName string
	LastStatus    int
	ProbeCount    int64
}

type urlsPage struct {
	pageData
	Pagination pagination
	URLs       []urlRow
	Query      string
}

type probesPage struct {
	pageData
	Pagination pagination
	Probes     []models.HTTPProbe
	Query      string
	Code       string
	Failed     string
}

// -----------------------------------------------------------------------------
// Dashboard
// -----------------------------------------------------------------------------

func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	db := s.db.WithContext(r.Context())
	data := dashboardPage{
		pageData: pageData{Title: "Visão geral", Active: "dashboard"},
	}

	counts := &data.Counts
	if err := db.Model(&models.Program{}).Count(&counts.Programs).Error; err != nil {
		s.internalError(w, r, err)
		return
	}
	db.Model(&models.Domain{}).Count(&counts.Domains)
	db.Model(&models.Scan{}).Count(&counts.Scans)
	db.Model(&models.Scan{}).Where("status = ?", "running").Count(&counts.Running)
	db.Model(&models.Subdomain{}).Count(&counts.Subdomains)
	db.Model(&models.Host{}).Count(&counts.Hosts)
	db.Model(&models.URL{}).Count(&counts.URLs)
	db.Model(&models.HTTPProbe{}).Count(&counts.Probes)

	if err := db.Preload("Domain").
		Order("started_at DESC").
		Limit(8).
		Find(&data.RecentScans).Error; err != nil {
		s.internalError(w, r, err)
		return
	}

	db.Raw(`SELECT status AS label, COUNT(*) AS total
		FROM scans GROUP BY status ORDER BY total DESC`).Scan(&data.ScanStatus)

	db.Raw(`SELECT domains.name AS label, COUNT(subdomains.id) AS total
		FROM domains
		LEFT JOIN subdomains ON subdomains.domain_id = domains.id
		GROUP BY domains.name
		ORDER BY total DESC, domains.name
		LIMIT 8`).Scan(&data.TopDomains)

	db.Raw(`SELECT status_code AS code, COUNT(*) AS total
		FROM http_probes
		WHERE status_code > 0
		GROUP BY status_code
		ORDER BY total DESC
		LIMIT 8`).Scan(&data.TopCodes)

	// Tecnologias ficam em jsonb; a falha aqui não deve derrubar o dashboard
	// (por exemplo, em um banco antigo sem a coluna).
	_ = db.Raw(`SELECT tech AS label, COUNT(*) AS total
		FROM http_probes
		CROSS JOIN LATERAL jsonb_array_elements_text(http_probes.technologies) AS tech
		WHERE tech <> ''
		GROUP BY tech
		ORDER BY total DESC
		LIMIT 10`).Scan(&data.TopTechnologies).Error

	s.render(w, http.StatusOK, "dashboard.html", data)
}

// -----------------------------------------------------------------------------
// Scans
// -----------------------------------------------------------------------------

func (s *Server) scans(w http.ResponseWriter, r *http.Request) {
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
		s.internalError(w, r, err)
		return
	}

	pg := newPagination(r, total)
	var scans []models.Scan
	if err := base.Preload("Domain").
		Order("scans.started_at DESC").
		Limit(pg.PageSize).
		Offset(pg.Offset()).
		Find(&scans).Error; err != nil {
		s.internalError(w, r, err)
		return
	}

	s.render(w, http.StatusOK, "scans.html", scansPage{
		pageData:     pageData{Title: "Scans", Active: "scans"},
		Pagination:   pg,
		Scans:        scans,
		Query:        query,
		StatusFilter: status,
	})
}

func (s *Server) scanDetail(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		s.renderError(w, r, http.StatusBadRequest, "ID de scan inválido")
		return
	}

	db := s.db.WithContext(r.Context())
	var scan models.Scan
	if err := db.Preload("Domain.Program").First(&scan, id).Error; err != nil {
		s.notFoundOrError(w, r, err, "Scan não encontrado")
		return
	}

	data := scanDetailPage{
		pageData: pageData{Title: "Scan #" + r.PathValue("id"), Active: "scans"},
		Scan:     scan,
	}

	if err := db.Table("subdomains").
		Select("subdomains.*, scan_subdomains.source").
		Joins("JOIN scan_subdomains ON scan_subdomains.subdomain_id = subdomains.id").
		Where("scan_subdomains.scan_id = ?", scan.ID).
		Order("subdomains.name").
		Limit(500).
		Find(&data.Subdomains).Error; err != nil {
		s.internalError(w, r, err)
		return
	}
	data.Counts.Subdomains = int64(len(data.Subdomains))

	if err := db.Table("resolutions").
		Select("resolutions.*, subdomains.name AS subdomain_name, hosts.ip AS ip").
		Joins("JOIN subdomains ON subdomains.id = resolutions.subdomain_id").
		Joins("JOIN hosts ON hosts.id = resolutions.host_id").
		Where("resolutions.scan_id = ?", scan.ID).
		Order("subdomains.name").
		Limit(500).
		Find(&data.Resolutions).Error; err != nil {
		s.internalError(w, r, err)
		return
	}
	data.Counts.Resolutions = int64(len(data.Resolutions))

	const probeLimit = 200
	if err := db.Preload("URL").Preload("URL.Subdomain").
		Where("scan_id = ?", scan.ID).
		Order("id").
		Limit(probeLimit + 1).
		Find(&data.Probes).Error; err != nil {
		s.internalError(w, r, err)
		return
	}
	if len(data.Probes) > probeLimit {
		data.Probes = data.Probes[:probeLimit]
		data.ProbesCutoff = true
	}
	data.Counts.Probes = int64(len(data.Probes))

	s.render(w, http.StatusOK, "scan_detail.html", data)
}

// -----------------------------------------------------------------------------
// Domínios
// -----------------------------------------------------------------------------

func (s *Server) domains(w http.ResponseWriter, r *http.Request) {
	db := s.db.WithContext(r.Context())
	query := strings.TrimSpace(r.URL.Query().Get("q"))

	base := db.Model(&models.Domain{}).Joins("JOIN programs ON programs.id = domains.program_id")
	if query != "" {
		base = base.Where("domains.name ILIKE ?", likePattern(query))
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		s.internalError(w, r, err)
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
		s.internalError(w, r, err)
		return
	}

	s.render(w, http.StatusOK, "domains.html", domainsPage{
		pageData:   pageData{Title: "Domínios", Active: "domains"},
		Pagination: pg,
		Domains:    rows,
		Query:      query,
	})
}

func (s *Server) domainDetail(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		s.renderError(w, r, http.StatusBadRequest, "ID de domínio inválido")
		return
	}

	db := s.db.WithContext(r.Context())
	var domain models.Domain
	if err := db.Preload("Program").First(&domain, id).Error; err != nil {
		s.notFoundOrError(w, r, err, "Domínio não encontrado")
		return
	}

	data := domainDetailPage{
		pageData: pageData{Title: domain.Name, Active: "domains"},
		Domain:   domain,
	}

	if err := db.Model(&models.Subdomain{}).Where("domain_id = ?", domain.ID).Count(&data.Counts.Subdomains).Error; err != nil {
		s.internalError(w, r, err)
		return
	}
	db.Model(&models.Scan{}).Where("domain_id = ?", domain.ID).Count(&data.Counts.Scans)
	db.Model(&models.URL{}).
		Joins("JOIN subdomains ON subdomains.id = urls.subdomain_id").
		Where("subdomains.domain_id = ?", domain.ID).
		Count(&data.Counts.URLs)
	db.Model(&models.HTTPProbe{}).
		Joins("JOIN urls ON urls.id = http_probes.url_id").
		Joins("JOIN subdomains ON subdomains.id = urls.subdomain_id").
		Where("subdomains.domain_id = ?", domain.ID).
		Count(&data.Counts.Probes)

	if err := db.Where("domain_id = ?", domain.ID).
		Order("started_at DESC").
		Limit(10).
		Find(&data.Scans).Error; err != nil {
		s.internalError(w, r, err)
		return
	}

	pg := newPagination(r, data.Counts.Subdomains)
	if err := db.Table("subdomains").
		Select(`subdomains.*, domains.name AS domain_name,
			(SELECT COUNT(*) FROM urls WHERE urls.subdomain_id = subdomains.id) AS url_count`).
		Joins("JOIN domains ON domains.id = subdomains.domain_id").
		Where("subdomains.domain_id = ?", domain.ID).
		Order("subdomains.name").
		Limit(pg.PageSize).
		Offset(pg.Offset()).
		Scan(&data.Subdomains).Error; err != nil {
		s.internalError(w, r, err)
		return
	}
	data.Pagination = pg

	s.render(w, http.StatusOK, "domain_detail.html", data)
}

// -----------------------------------------------------------------------------
// Subdomínios
// -----------------------------------------------------------------------------

func (s *Server) subdomains(w http.ResponseWriter, r *http.Request) {
	db := s.db.WithContext(r.Context())
	query := strings.TrimSpace(r.URL.Query().Get("q"))

	base := db.Model(&models.Subdomain{}).Joins("JOIN domains ON domains.id = subdomains.domain_id")
	if query != "" {
		base = base.Where("subdomains.name ILIKE ?", likePattern(query))
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		s.internalError(w, r, err)
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
		s.internalError(w, r, err)
		return
	}

	s.render(w, http.StatusOK, "subdomains.html", subdomainsPage{
		pageData:   pageData{Title: "Subdomínios", Active: "subdomains"},
		Pagination: pg,
		Subdomains: rows,
		Query:      query,
	})
}

func (s *Server) subdomainDetail(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r)
	if !ok {
		s.renderError(w, r, http.StatusBadRequest, "ID de subdomínio inválido")
		return
	}

	db := s.db.WithContext(r.Context())
	var subdomain models.Subdomain
	if err := db.Preload("Domain").First(&subdomain, id).Error; err != nil {
		s.notFoundOrError(w, r, err, "Subdomínio não encontrado")
		return
	}

	data := subdomainDetailPage{
		pageData:  pageData{Title: subdomain.Name, Active: "subdomains"},
		Subdomain: subdomain,
	}

	db.Model(&models.URL{}).Where("subdomain_id = ?", subdomain.ID).Count(&data.Counts.URLs)
	db.Model(&models.Resolution{}).Where("subdomain_id = ?", subdomain.ID).Count(&data.Counts.Resolutions)
	db.Model(&models.HTTPProbe{}).
		Joins("JOIN urls ON urls.id = http_probes.url_id").
		Where("urls.subdomain_id = ?", subdomain.ID).
		Count(&data.Counts.Probes)

	if err := db.Table("resolutions").
		Select("resolutions.*, hosts.ip AS ip").
		Joins("JOIN hosts ON hosts.id = resolutions.host_id").
		Where("resolutions.subdomain_id = ?", subdomain.ID).
		Order("resolutions.scan_id DESC, hosts.ip").
		Limit(100).
		Find(&data.Resolutions).Error; err != nil {
		s.internalError(w, r, err)
		return
	}

	pg := newPagination(r, data.Counts.URLs)
	if err := db.Table("urls").
		Select(`urls.*,
			(SELECT status_code FROM http_probes WHERE http_probes.url_id = urls.id ORDER BY id DESC LIMIT 1) AS last_status,
			(SELECT COUNT(*) FROM http_probes WHERE http_probes.url_id = urls.id) AS probe_count`).
		Where("urls.subdomain_id = ?", subdomain.ID).
		Order("urls.id").
		Limit(pg.PageSize).
		Offset(pg.Offset()).
		Scan(&data.URLs).Error; err != nil {
		s.internalError(w, r, err)
		return
	}
	data.Pagination = pg

	if err := db.Preload("URL").
		Joins("JOIN urls ON urls.id = http_probes.url_id").
		Where("urls.subdomain_id = ?", subdomain.ID).
		Order("http_probes.id DESC").
		Limit(50).
		Find(&data.Probes).Error; err != nil {
		s.internalError(w, r, err)
		return
	}

	s.render(w, http.StatusOK, "subdomain_detail.html", data)
}

// -----------------------------------------------------------------------------
// URLs
// -----------------------------------------------------------------------------

func (s *Server) urls(w http.ResponseWriter, r *http.Request) {
	db := s.db.WithContext(r.Context())
	query := strings.TrimSpace(r.URL.Query().Get("q"))

	base := db.Model(&models.URL{}).Joins("JOIN subdomains ON subdomains.id = urls.subdomain_id")
	if query != "" {
		base = base.Where("urls.raw_url ILIKE ?", likePattern(query))
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		s.internalError(w, r, err)
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
		s.internalError(w, r, err)
		return
	}

	s.render(w, http.StatusOK, "urls.html", urlsPage{
		pageData:   pageData{Title: "URLs", Active: "urls"},
		Pagination: pg,
		URLs:       rows,
		Query:      query,
	})
}

// -----------------------------------------------------------------------------
// Probes HTTP
// -----------------------------------------------------------------------------

func (s *Server) probes(w http.ResponseWriter, r *http.Request) {
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
		s.internalError(w, r, err)
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
		s.internalError(w, r, err)
		return
	}

	s.render(w, http.StatusOK, "probes.html", probesPage{
		pageData:   pageData{Title: "HTTP", Active: "probes"},
		Pagination: pg,
		Probes:     probes,
		Query:      query,
		Code:       code,
		Failed:     failed,
	})
}

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------

func (s *Server) notFoundOrError(w http.ResponseWriter, r *http.Request, err error, message string) {
	if err == gorm.ErrRecordNotFound {
		s.renderError(w, r, http.StatusNotFound, message)
		return
	}
	s.internalError(w, r, err)
}

// likePattern escapa curingas do LIKE para que a busca seja literal.
func likePattern(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + replacer.Replace(value) + "%"
}

type pagination struct {
	Path       string
	Filter     string
	Page       int
	PageSize   int
	Total      int64
	TotalPages int
	PrevPage   int
	NextPage   int
	HasPrev    bool
	HasNext    bool
}

func (p pagination) Offset() int {
	return (p.Page - 1) * p.PageSize
}

func newPagination(r *http.Request, total int64) pagination {
	page := queryInt(r, "page", 1, 1, 1<<30)
	size := queryInt(r, "page_size", defaultPageSize, 1, maxPageSize)

	pages := int((total + int64(size) - 1) / int64(size))
	if pages < 1 {
		pages = 1
	}
	if page > pages {
		page = pages
	}

	values := r.URL.Query()
	values.Del("page")
	values.Del("page_size")
	filter := ""
	if encoded := values.Encode(); encoded != "" {
		filter = "&" + encoded
	}

	return pagination{
		Path:       r.URL.Path,
		Filter:     filter,
		Page:       page,
		PageSize:   size,
		Total:      total,
		TotalPages: pages,
		PrevPage:   page - 1,
		NextPage:   page + 1,
		HasPrev:    page > 1,
		HasNext:    page < pages,
	}
}
