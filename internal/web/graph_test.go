package web

import (
	"errors"
	"testing"
)

func TestGraphDomainID(t *testing.T) {
	domains := []graphDomainOption{
		{ID: 1, Name: "example.com", Subdomains: 12},
		{ID: 2, Name: "outro.dev", Subdomains: 3},
	}

	cases := []struct {
		raw  string
		want uint
		err  error
	}{
		{raw: "", want: 0},
		{raw: "1", want: 1},
		{raw: " 2 ", want: 2},
		{raw: "3", err: errGraphDomainNotFound},
		{raw: "0", err: errGraphDomainInvalid},
		{raw: "-1", err: errGraphDomainInvalid},
		{raw: "abc", err: errGraphDomainInvalid},
	}

	for _, tc := range cases {
		got, err := graphDomainID(tc.raw, domains)
		if tc.err != nil {
			if !errors.Is(err, tc.err) {
				t.Errorf("graphDomainID(%q): erro %v; esperado %v", tc.raw, err, tc.err)
			}
			continue
		}
		if err != nil {
			t.Errorf("graphDomainID(%q): %v", tc.raw, err)
			continue
		}
		if got != tc.want {
			t.Errorf("graphDomainID(%q) = %d; esperado %d", tc.raw, got, tc.want)
		}
	}
}

func TestGraphAPIURL(t *testing.T) {
	if got := graphAPIURL(0, 40); got != "/api/graph?limit=40" {
		t.Errorf("visão geral = %q", got)
	}
	if got := graphAPIURL(2, 25); got != "/api/graph?domain=2&limit=25" {
		t.Errorf("domínio = %q", got)
	}
}

func TestAppendGraphDomain(t *testing.T) {
	data := graphResponse{Scope: "example.com", Limit: 10}
	domain := graphDomainOption{ID: 7, Name: "example.com", Subdomains: 42}
	rows := []graphSubdomainRow{
		{ID: 10, Name: "api.example.com", URLCount: 5},
		{ID: 11, Name: "www.example.com", URLCount: 2},
	}

	appendGraphDomain(&data, domain, rows)

	if len(data.Nodes) != 3 {
		t.Fatalf("esperado 3 nós, obteve %d", len(data.Nodes))
	}
	if len(data.Links) != 2 {
		t.Fatalf("esperado 2 vínculos, obteve %d", len(data.Links))
	}

	root := data.Nodes[0]
	if root.ID != "d:7" || root.Type != "domain" || root.Count != 42 {
		t.Errorf("nó do domínio incorreto: %+v", root)
	}
	if root.URL != "/domains/7" {
		t.Errorf("URL do domínio = %q", root.URL)
	}

	child := data.Nodes[1]
	if child.ID != "s:10" || child.Type != "subdomain" || child.Count != 5 {
		t.Errorf("nó do subdomínio incorreto: %+v", child)
	}
	if child.DomainID != 7 || child.Domain != "example.com" {
		t.Errorf("domínio do subdomínio incorreto: %+v", child)
	}
	if child.URL != "/subdomains/10" {
		t.Errorf("URL do subdomínio = %q", child.URL)
	}

	if data.Links[0].Source != "d:7" || data.Links[0].Target != "s:10" {
		t.Errorf("vínculo incorreto: %+v", data.Links[0])
	}
	if data.Scope != "example.com" {
		t.Errorf("scope alterado: %q", data.Scope)
	}
}
