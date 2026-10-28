package models_test

import (
	"sync"
	"testing"

	"github.com/m4rco0/Rebellion/internal/database/models"
	"gorm.io/gorm/schema"
)

// Garante que os modelos e as tags do GORM (índices, FKs, serializer) são
// válidos sem precisar de um banco.
func TestModelsParse(t *testing.T) {
	cache := &sync.Map{}
	namer := schema.NamingStrategy{}

	expected := []struct {
		model any
		table string
	}{
		{&models.Program{}, "programs"},
		{&models.Domain{}, "domains"},
		{&models.Scan{}, "scans"},
		{&models.Subdomain{}, "subdomains"},
		{&models.ScanSubdomain{}, "scan_subdomains"},
		{&models.Host{}, "hosts"},
		{&models.Port{}, "ports"},
		{&models.Resolution{}, "resolutions"},
		{&models.URL{}, "urls"},
		{&models.HTTPProbe{}, "http_probes"},
	}

	for _, item := range expected {
		parsed, err := schema.Parse(item.model, cache, namer)
		if err != nil {
			t.Errorf("parse de %T falhou: %v", item.model, err)
			continue
		}
		if parsed.Table != item.table {
			t.Errorf("%T: tabela esperada %q, obteve %q", item.model, item.table, parsed.Table)
		}
	}
}
