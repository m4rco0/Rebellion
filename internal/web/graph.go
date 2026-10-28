package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const (
	// graphDefaultLimit é quantos subdomínios cada domínio mostra por padrão.
	graphDefaultLimit = 40
	graphMaxLimit     = 200
	// graphMaxDomains limita quantos domínios aparecem na visão geral.
	graphMaxDomains = 8
	// graphMaxSubdomainNodes evita grafos grandes demais na visão geral.
	graphMaxSubdomainNodes = 240
)

var (
	errGraphDomainNotFound = errors.New("domínio não encontrado")
	errGraphDomainInvalid  = errors.New("parâmetro domain inválido")
)

// graphDomainOption é um domínio disponível no seletor da página de grafo.
type graphDomainOption struct {
	ID         uint
	Name       string
	Subdomains int64
}

type graphNode struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Type        string `json:"type"`
	Domain      string `json:"domain,omitempty"`
	DomainID    uint   `json:"domain_id,omitempty"`
	SubdomainID uint   `json:"subdomain_id,omitempty"`
	Count       int64  `json:"count"`
	URL         string `json:"url,omitempty"`
}

type graphLink struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

type graphResponse struct {
	Nodes            []graphNode `json:"nodes"`
	Links            []graphLink `json:"links"`
	Scope            string      `json:"scope"`
	DomainID         uint        `json:"domain_id,omitempty"`
	Limit            int         `json:"limit"`
	Total            int64       `json:"total"`
	Shown            int         `json:"shown"`
	Truncated        bool        `json:"truncated"`
	DomainsTruncated bool        `json:"domains_truncated"`
}

type graphSubdomainRow struct {
	ID       uint
	Name     string
	URLCount int64
}

type graphPage struct {
	pageData
	Domains []graphDomainOption
	// DomainID é 0 quando a visão geral está selecionada.
	DomainID    uint
	Limit       int
	GraphURL    string
	LimitValues []int
}

// -----------------------------------------------------------------------------
// Página
// -----------------------------------------------------------------------------

func (s *Server) graph(w http.ResponseWriter, r *http.Request) {
	limit := queryInt(r, "limit", graphDefaultLimit, 1, graphMaxLimit)

	domains, err := s.graphDomains(r.Context())
	if err != nil {
		s.internalError(w, r, err)
		return
	}

	domainID, err := graphDomainID(r.URL.Query().Get("domain"), domains)
	if errors.Is(err, errGraphDomainInvalid) {
		s.renderError(w, r, http.StatusBadRequest, "Parâmetro de domínio inválido")
		return
	}
	if err != nil {
		s.renderError(w, r, http.StatusNotFound, "Domínio não encontrado")
		return
	}

	s.render(w, http.StatusOK, "graph.html", graphPage{
		pageData:    pageData{Title: "Grafo", Active: "graph"},
		Domains:     domains,
		DomainID:    domainID,
		Limit:       limit,
		GraphURL:    graphAPIURL(domainID, limit),
		LimitValues: []int{10, 25, 50, 100, 200},
	})
}

// -----------------------------------------------------------------------------
// Dados
// -----------------------------------------------------------------------------

func (s *Server) apiGraph(w http.ResponseWriter, r *http.Request) {
	limit := queryInt(r, "limit", graphDefaultLimit, 1, graphMaxLimit)

	domains, err := s.graphDomains(r.Context())
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "erro ao consultar o grafo")
		return
	}

	domainID, err := graphDomainID(r.URL.Query().Get("domain"), domains)
	if errors.Is(err, errGraphDomainInvalid) {
		writeAPIError(w, http.StatusBadRequest, "parâmetro domain inválido")
		return
	}
	if errors.Is(err, errGraphDomainNotFound) {
		writeAPIError(w, http.StatusNotFound, "domínio não encontrado")
		return
	}
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, "parâmetro domain inválido")
		return
	}

	data, err := s.buildGraph(r.Context(), domains, domainID, limit)
	if err != nil {
		if errors.Is(err, errGraphDomainNotFound) {
			writeAPIError(w, http.StatusNotFound, "domínio não encontrado")
			return
		}
		writeAPIError(w, http.StatusInternalServerError, "erro ao consultar o grafo")
		return
	}

	writeJSON(w, http.StatusOK, data)
}

func (s *Server) graphDomains(ctx context.Context) ([]graphDomainOption, error) {
	var domains []graphDomainOption
	err := s.db.WithContext(ctx).Raw(`
		SELECT domains.id, domains.name,
			(SELECT count(*) FROM subdomains WHERE subdomains.domain_id = domains.id) AS subdomains
		FROM domains
		ORDER BY subdomains DESC, domains.name`).Scan(&domains).Error
	if err != nil {
		return nil, fmt.Errorf("consultar domínios do grafo: %w", err)
	}
	return domains, nil
}

// graphDomainID valida o parâmetro domain. Vazio significa visão geral.
func graphDomainID(raw string, domains []graphDomainOption) (uint, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}

	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		return 0, errGraphDomainInvalid
	}
	for _, domain := range domains {
		if domain.ID == uint(id) {
			return domain.ID, nil
		}
	}
	return 0, errGraphDomainNotFound
}

func graphAPIURL(domainID uint, limit int) string {
	values := url.Values{}
	if domainID != 0 {
		values.Set("domain", strconv.FormatUint(uint64(domainID), 10))
	}
	values.Set("limit", strconv.Itoa(limit))
	return "/api/graph?" + values.Encode()
}

func (s *Server) buildGraph(ctx context.Context, domains []graphDomainOption, domainID uint, limit int) (graphResponse, error) {
	data := graphResponse{Limit: limit}

	if domainID != 0 {
		var selected graphDomainOption
		for _, domain := range domains {
			if domain.ID == domainID {
				selected = domain
				break
			}
		}
		if selected.ID == 0 {
			return graphResponse{}, errGraphDomainNotFound
		}
		data.Scope = selected.Name
		data.DomainID = selected.ID
		data.Total = selected.Subdomains

		rows, truncated, err := s.graphSubdomains(ctx, selected.ID, limit)
		if err != nil {
			return graphResponse{}, err
		}
		data.Truncated = truncated
		appendGraphDomain(&data, selected, rows)

		data.Shown = len(rows)
		return data, nil
	}

	data.Scope = "visão geral"

	shownDomains := domains
	if len(shownDomains) > graphMaxDomains {
		shownDomains = shownDomains[:graphMaxDomains]
		data.DomainsTruncated = true
	}

	remaining := graphMaxSubdomainNodes
	for _, domain := range shownDomains {
		data.Total += domain.Subdomains

		budget := limit
		if budget > remaining {
			budget = remaining
		}
		if budget <= 0 {
			data.Truncated = true
			continue
		}

		rows, truncated, err := s.graphSubdomains(ctx, domain.ID, budget)
		if err != nil {
			return graphResponse{}, err
		}
		if truncated || domain.Subdomains > int64(len(rows)) {
			data.Truncated = true
		}
		remaining -= len(rows)
		appendGraphDomain(&data, domain, rows)
		data.Shown += len(rows)
	}

	return data, nil
}

// graphSubdomains devolve até limit subdomínios do domínio (limit+1 é
// consultado apenas para saber se há mais registros).
func (s *Server) graphSubdomains(ctx context.Context, domainID uint, limit int) ([]graphSubdomainRow, bool, error) {
	var rows []graphSubdomainRow
	err := s.db.WithContext(ctx).Raw(`
		SELECT subdomains.id, subdomains.name,
			(SELECT count(*) FROM urls WHERE urls.subdomain_id = subdomains.id) AS url_count
		FROM subdomains
		WHERE subdomains.domain_id = ?
		ORDER BY subdomains.name
		LIMIT ?`, domainID, limit+1).Scan(&rows).Error
	if err != nil {
		return nil, false, fmt.Errorf("consultar subdomínios do grafo: %w", err)
	}

	truncated := len(rows) > limit
	if truncated {
		rows = rows[:limit]
	}
	return rows, truncated, nil
}

func appendGraphDomain(data *graphResponse, domain graphDomainOption, subdomains []graphSubdomainRow) {
	domainNodeID := fmt.Sprintf("d:%d", domain.ID)
	data.Nodes = append(data.Nodes, graphNode{
		ID:       domainNodeID,
		Label:    domain.Name,
		Type:     "domain",
		Domain:   domain.Name,
		DomainID: domain.ID,
		Count:    domain.Subdomains,
		URL:      fmt.Sprintf("/domains/%d", domain.ID),
	})

	for _, subdomain := range subdomains {
		subdomainNodeID := fmt.Sprintf("s:%d", subdomain.ID)
		data.Nodes = append(data.Nodes, graphNode{
			ID:          subdomainNodeID,
			Label:       subdomain.Name,
			Type:        "subdomain",
			Domain:      domain.Name,
			DomainID:    domain.ID,
			SubdomainID: subdomain.ID,
			Count:       subdomain.URLCount,
			URL:         fmt.Sprintf("/subdomains/%d", subdomain.ID),
		})
		data.Links = append(data.Links, graphLink{Source: domainNodeID, Target: subdomainNodeID})
	}
}
