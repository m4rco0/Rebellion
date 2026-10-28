package store_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/m4rco0/Rebellion/internal/database"
	"github.com/m4rco0/Rebellion/internal/database/models"
	"github.com/m4rco0/Rebellion/internal/storage"
	"github.com/m4rco0/Rebellion/internal/store"
	"gorm.io/gorm"
)

// Os testes de integração exigem um PostgreSQL descartável. Exemplo:
//
//	TEST_DATABASE_URL="host=localhost user=root password=123 dbname=rebellion_test port=5432 sslmode=disable" go test ./internal/store
//
// Sem a variável, os testes são ignorados.
func newTestStore(t *testing.T) (*store.Store, *gorm.DB) {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("defina TEST_DATABASE_URL para rodar os testes de integração")
	}

	db, err := database.InitDB(dsn)
	if err != nil {
		t.Fatalf("inicializar banco de teste: %v", err)
	}

	if err := db.Exec(
		"TRUNCATE TABLE http_probes, urls, resolutions, ports, hosts, scan_subdomains, subdomains, scans, domains, programs RESTART IDENTITY CASCADE",
	).Error; err != nil {
		t.Fatalf("limpar banco de teste: %v", err)
	}

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return store.New(db), db
}

func TestStartScanIsIdempotentForDomain(t *testing.T) {
	st, db := newTestStore(t)

	first, err := st.StartScan("https://Example.com/caminho")
	if err != nil {
		t.Fatalf("StartScan: %v", err)
	}
	second, err := st.StartScan("example.com")
	if err != nil {
		t.Fatalf("StartScan: %v", err)
	}

	if first.DomainID != second.DomainID {
		t.Errorf("mesmo domínio deveria reutilizar o registro: %d != %d", first.DomainID, second.DomainID)
	}
	if first.ID == second.ID {
		t.Error("cada execução deveria criar um scan novo")
	}

	var domain models.Domain
	if err := db.First(&domain, first.DomainID).Error; err != nil {
		t.Fatalf("buscar domínio: %v", err)
	}
	if domain.Name != "example.com" {
		t.Errorf("domínio deveria ser normalizado, obteve %q", domain.Name)
	}

	var domains, scans, programs int64
	db.Model(&models.Domain{}).Count(&domains)
	db.Model(&models.Scan{}).Count(&scans)
	db.Model(&models.Program{}).Count(&programs)
	if domains != 1 || scans != 2 || programs != 1 {
		t.Errorf("contagens inesperadas: domains=%d scans=%d programs=%d", domains, scans, programs)
	}
}

func TestSaveSubdomainsIsIdempotentAndScoped(t *testing.T) {
	st, db := newTestStore(t)

	scan, err := st.StartScan("example.com")
	if err != nil {
		t.Fatalf("StartScan: %v", err)
	}

	names := []string{
		"www.example.com",
		"api.example.com",
		"WWW.EXAMPLE.COM",
		"fora-do-escopo.com",
		"example.com",
	}
	created, err := st.SaveSubdomains(scan.ID, names, "subfinder")
	if err != nil {
		t.Fatalf("SaveSubdomains: %v", err)
	}
	if created.Processed != 3 || created.Created != 3 {
		t.Errorf("primeira gravação deveria criar 3: %+v", created)
	}
	result, err := st.SaveSubdomains(scan.ID, names, "puredns")
	if err != nil {
		t.Fatalf("SaveSubdomains (repetido): %v", err)
	}
	if result.Processed != 3 || result.Created != 0 {
		t.Errorf("repetição não deveria criar novos: %+v", result)
	}

	var subdomains, links int64
	db.Model(&models.Subdomain{}).Count(&subdomains)
	db.Model(&models.ScanSubdomain{}).Count(&links)

	if subdomains != 3 {
		t.Errorf("esperado 3 subdomínios (www, api, raiz), obteve %d", subdomains)
	}
	if links != 3 {
		t.Errorf("vínculo com o scan deveria ser idempotente, obteve %d", links)
	}
}

// TestSaveSubdomainsDoesNotDuplicateAcrossScans garante que dois scans do
// mesmo domínio compartilham os registros de subdomínio: a tabela subdomains
// não ganha cópias e cada scan mantém o seu próprio vínculo.
func TestSaveSubdomainsDoesNotDuplicateAcrossScans(t *testing.T) {
	st, db := newTestStore(t)

	first, err := st.StartScan("example.com")
	if err != nil {
		t.Fatalf("StartScan (primeiro): %v", err)
	}
	second, err := st.StartScan("example.com")
	if err != nil {
		t.Fatalf("StartScan (segundo): %v", err)
	}

	names := []string{"www.example.com", "api.example.com"}

	created, err := st.SaveSubdomains(first.ID, names, "subfinder")
	if err != nil {
		t.Fatalf("SaveSubdomains (primeiro scan): %v", err)
	}
	if created.Processed != 2 || created.Created != 2 {
		t.Errorf("primeiro scan deveria criar 2: %+v", created)
	}

	reused, err := st.SaveSubdomains(second.ID, names, "subfinder")
	if err != nil {
		t.Fatalf("SaveSubdomains (segundo scan): %v", err)
	}
	if reused.Processed != 2 || reused.Created != 0 {
		t.Errorf("segundo scan deveria reutilizar os 2 registros: %+v", reused)
	}

	var subdomains, links, duplicateGroups int64
	db.Model(&models.Subdomain{}).Count(&subdomains)
	db.Model(&models.ScanSubdomain{}).Count(&links)
	if err := db.Raw(`
		SELECT count(*) FROM (
			SELECT domain_id, name FROM subdomains
			GROUP BY domain_id, name HAVING count(*) > 1
		) AS duplicados`).Scan(&duplicateGroups).Error; err != nil {
		t.Fatalf("consultar duplicatas: %v", err)
	}

	if subdomains != 2 {
		t.Errorf("subdomains deveria ter 2 linhas, obteve %d", subdomains)
	}
	if links != 4 {
		t.Errorf("cada scan deveria ter 2 vínculos (total 4), obteve %d", links)
	}
	if duplicateGroups != 0 {
		t.Errorf("encontrados %d grupos duplicados em subdomains", duplicateGroups)
	}
}

// TestSaveSubdomainsAboveParameterLimit reproduz o volume de um scan real:
// antes do batching, um único INSERT com esta quantidade de subdomínios
// estourava o limite de 65535 parâmetros do protocolo estendido do PostgreSQL.
func TestSaveSubdomainsAboveParameterLimit(t *testing.T) {
	st, db := newTestStore(t)

	scan, err := st.StartScan("example.com")
	if err != nil {
		t.Fatalf("StartScan: %v", err)
	}

	// 40 mil linhas já geram ~160 mil parâmetros em um único INSERT, bem acima
	// do limite de 65535 do protocolo estendido.
	const total = 40000
	names := make([]string, 0, total)
	for i := 0; i < total; i++ {
		names = append(names, fmt.Sprintf("h%05d.example.com", i))
	}

	created, err := st.SaveSubdomains(scan.ID, names, "subfinder")
	if err != nil {
		t.Fatalf("SaveSubdomains com %d nomes: %v", total, err)
	}
	if created.Processed != total || created.Created != total {
		t.Errorf("primeira gravação inesperada: %+v", created)
	}

	var subdomains, links int64
	db.Model(&models.Subdomain{}).Count(&subdomains)
	db.Model(&models.ScanSubdomain{}).Count(&links)
	if subdomains != total {
		t.Errorf("esperado %d subdomínios, obteve %d", total, subdomains)
	}
	if links != total {
		t.Errorf("esperado %d vínculos com o scan, obteve %d", total, links)
	}

	// Repetir a operação não pode duplicar nada.
	repeated, err := st.SaveSubdomains(scan.ID, names, "puredns")
	if err != nil {
		t.Fatalf("SaveSubdomains repetido: %v", err)
	}
	if repeated.Created != 0 {
		t.Errorf("repetição não deveria criar novos: %+v", repeated)
	}
	db.Model(&models.Subdomain{}).Count(&subdomains)
	db.Model(&models.ScanSubdomain{}).Count(&links)
	if subdomains != total || links != total {
		t.Errorf("operação deveria ser idempotente: subdomains=%d links=%d", subdomains, links)
	}
}

func TestSaveURLsNormalizesAndScopes(t *testing.T) {
	st, db := newTestStore(t)

	scan, err := st.StartScan("example.com")
	if err != nil {
		t.Fatalf("StartScan: %v", err)
	}

	urls := []string{
		"https://www.example.com/a?b=1",
		"https://www.example.com/a?b=1",
		"https://api.example.com/b",
		"https://fora-do-escopo.com/x",
		"ftp://www.example.com/arquivo",
		"não é uma url",
	}
	if err := st.SaveURLs(scan.ID, urls); err != nil {
		t.Fatalf("SaveURLs: %v", err)
	}
	if err := st.SaveURLs(scan.ID, urls); err != nil {
		t.Fatalf("SaveURLs (repetido): %v", err)
	}

	var count int64
	db.Model(&models.URL{}).Count(&count)
	if count != 2 {
		t.Errorf("esperado 2 URLs, obteve %d", count)
	}

	var url models.URL
	if err := db.Where("raw_url = ?", "https://www.example.com/a?b=1").First(&url).Error; err != nil {
		t.Fatalf("buscar URL: %v", err)
	}
	if url.Scheme != "https" || url.Host != "www.example.com" || url.Path != "/a" || url.Query != "b=1" {
		t.Errorf("URL parseada incorretamente: %+v", url)
	}
}

func TestSaveHTTPProbesUpsertsAndLinks(t *testing.T) {
	st, db := newTestStore(t)

	scan, err := st.StartScan("example.com")
	if err != nil {
		t.Fatalf("StartScan: %v", err)
	}

	probe := storage.Probe{
		Input:         "https://www.example.com/a?b=1",
		URL:           "https://www.example.com/a?b=1",
		StatusCode:    200,
		Title:         "Home",
		Webserver:     "nginx",
		ContentType:   "text/html",
		ContentLength: 123,
		Technologies:  []string{"nginx", "php"},
		IPs:           []string{"1.2.3.4"},
		CNAMEs:        []string{"cdn.example.net"},
		CDN:           "cloudflare",
		ResponseTime:  "10ms",
	}

	if err := st.SaveHTTPProbes(scan.ID, []storage.Probe{probe}); err != nil {
		t.Fatalf("SaveHTTPProbes: %v", err)
	}

	probe.StatusCode = 204
	probe.Title = "Sem conteúdo"
	if err := st.SaveHTTPProbes(scan.ID, []storage.Probe{probe}); err != nil {
		t.Fatalf("SaveHTTPProbes (repetido): %v", err)
	}

	var probes, hosts, resolutions int64
	db.Model(&models.HTTPProbe{}).Count(&probes)
	db.Model(&models.Host{}).Count(&hosts)
	db.Model(&models.Resolution{}).Count(&resolutions)

	if probes != 1 {
		t.Errorf("probe deveria ser atualizado, não duplicado; obteve %d", probes)
	}
	if hosts != 1 {
		t.Errorf("esperado 1 host, obteve %d", hosts)
	}
	if resolutions != 1 {
		t.Errorf("esperado 1 resolução, obteve %d", resolutions)
	}

	var stored models.HTTPProbe
	if err := db.First(&stored).Error; err != nil {
		t.Fatalf("buscar probe: %v", err)
	}
	if stored.StatusCode != 204 || stored.Title != "Sem conteúdo" {
		t.Errorf("probe não foi atualizado: %+v", stored)
	}
	if len(stored.Technologies) != 2 || stored.Technologies[1] != "php" {
		t.Errorf("tecnologias não persistidas: %v", stored.Technologies)
	}
	if len(stored.IPs) != 1 || stored.IPs[0] != "1.2.3.4" {
		t.Errorf("IPs não persistidos: %v", stored.IPs)
	}
}

func TestSaveResolutions(t *testing.T) {
	st, db := newTestStore(t)

	scan, err := st.StartScan("example.com")
	if err != nil {
		t.Fatalf("StartScan: %v", err)
	}

	if err := st.SaveResolutions(scan.ID, "api.example.com", []string{"1.2.3.4", "inválido", "::1"}); err != nil {
		t.Fatalf("SaveResolutions: %v", err)
	}
	if err := st.SaveResolutions(scan.ID, "api.example.com", []string{"1.2.3.4"}); err != nil {
		t.Fatalf("SaveResolutions (repetido): %v", err)
	}

	var hosts, resolutions int64
	db.Model(&models.Host{}).Count(&hosts)
	db.Model(&models.Resolution{}).Count(&resolutions)

	if hosts != 2 {
		t.Errorf("esperado 2 hosts válidos, obteve %d", hosts)
	}
	if resolutions != 2 {
		t.Errorf("esperado 2 resoluções, obteve %d", resolutions)
	}
}

func TestFinishScan(t *testing.T) {
	st, db := newTestStore(t)

	scan, err := st.StartScan("example.com")
	if err != nil {
		t.Fatalf("StartScan: %v", err)
	}

	if err := st.FinishScan(scan.ID, store.StatusFailed, os.ErrDeadlineExceeded); err != nil {
		t.Fatalf("FinishScan: %v", err)
	}

	var stored models.Scan
	if err := db.First(&stored, scan.ID).Error; err != nil {
		t.Fatalf("buscar scan: %v", err)
	}
	if stored.Status != store.StatusFailed {
		t.Errorf("status esperado %q, obteve %q", store.StatusFailed, stored.Status)
	}
	if stored.FinishedAt == nil {
		t.Error("FinishedAt deveria ser preenchido")
	}
	if stored.Error == "" {
		t.Error("Error deveria conter a mensagem da falha")
	}
}
