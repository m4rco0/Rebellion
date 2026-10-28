package store

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	neturl "net/url"
	"strings"
	"time"

	"github.com/m4rco0/Rebellion/internal/database/models"
	"github.com/m4rco0/Rebellion/internal/storage"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"

	manualPlatform = "manual"
	manualProgram  = "manual"

	// insertBatchSize limita quantas linhas vão em cada INSERT. O protocolo
	// estendido do PostgreSQL aceita no máximo 65535 parâmetros por comando, e
	// um scan real gera dezenas de milhares de subdomínios e URLs. A tabela mais
	// larga (http_probes, 18 colunas) fica com ~36 mil parâmetros por lote, bem
	// abaixo do limite.
	insertBatchSize = 2000

	// queryBatchSize limita o tamanho das listas usadas em cláusulas IN (?),
	// que também contam como parâmetros do comando.
	queryBatchSize = 5000
)

// Store concentra a escrita no banco. O restante da aplicação não deve chamar
// GORM diretamente: assim as regras de upsert, escopo e transação ficam em um
// único lugar.
type Store struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Store {
	return &Store{db: db}
}

// StartScan cria (ou reutiliza) o programa manual e o domínio, e abre um novo
// scan com status "running".
func (s *Store) StartScan(domainName string) (*models.Scan, error) {
	name := NormalizeDomain(domainName)
	if name == "" {
		return nil, errors.New("domínio vazio")
	}

	var scan models.Scan
	err := s.db.Transaction(func(tx *gorm.DB) error {
		program := models.Program{Platform: manualPlatform, Name: manualProgram}
		if err := tx.Where("platform = ? AND name = ?", program.Platform, program.Name).
			FirstOrCreate(&program).Error; err != nil {
			return fmt.Errorf("programa: %w", err)
		}

		domain := models.Domain{ProgramID: program.ID, Name: name}
		if err := tx.Where("program_id = ? AND name = ?", program.ID, name).
			FirstOrCreate(&domain).Error; err != nil {
			return fmt.Errorf("domínio: %w", err)
		}

		scan = models.Scan{
			DomainID:  domain.ID,
			Status:    StatusRunning,
			StartedAt: time.Now().UTC(),
		}
		if err := tx.Create(&scan).Error; err != nil {
			return fmt.Errorf("scan: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return &scan, nil
}

// FinishScan encerra um scan registrando status, horário e erro (se houver).
func (s *Store) FinishScan(scanID uint, status string, scanErr error) error {
	if status == "" {
		status = StatusCompleted
	}

	updates := map[string]any{
		"status":      status,
		"finished_at": time.Now().UTC(),
		"error":       "",
	}
	if scanErr != nil {
		updates["error"] = scanErr.Error()
	}

	res := s.db.Model(&models.Scan{}).Where("id = ?", scanID).Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("scan %d não encontrado", scanID)
	}

	return nil
}

// SubdomainSaveResult descreve o que SaveSubdomains fez. Processed conta os
// nomes únicos dentro do escopo do domínio; Created conta apenas os que ainda
// não existiam no domínio.
type SubdomainSaveResult struct {
	Processed int
	Created   int
}

// SaveSubdomains faz upsert dos subdomínios do domínio do scan e registra a
// descoberta no scan atual. Entradas fora do escopo do domínio são ignoradas.
//
// O banco garante unicidade por (domain_id, name): repetir um nome já conhecido
// não cria outra linha, apenas reutiliza o registro e vincula o scan atual.
func (s *Store) SaveSubdomains(scanID uint, names []string, source string) (SubdomainSaveResult, error) {
	info, err := s.scanInfo(scanID)
	if err != nil {
		return SubdomainSaveResult{}, err
	}

	cleaned := make([]string, 0, len(names))
	for _, name := range names {
		name = normalizeHost(name)
		if name == "" || !inScope(name, info.DomainName) {
			continue
		}
		cleaned = append(cleaned, name)
	}
	if len(cleaned) == 0 {
		return SubdomainSaveResult{}, nil
	}

	var result SubdomainSaveResult
	err = s.db.Transaction(func(tx *gorm.DB) error {
		ids, created, err := upsertSubdomains(tx, scanID, info.DomainID, cleaned, source)
		if err != nil {
			return err
		}
		result.Processed = len(ids)
		result.Created = created
		return nil
	})
	if err != nil {
		return SubdomainSaveResult{}, err
	}

	return result, nil
}

// SaveURLs faz upsert das URLs descobertas (gau/waybackurls) e dos
// subdomínios que ainda não existiam.
func (s *Store) SaveURLs(scanID uint, rawURLs []string) error {
	info, err := s.scanInfo(scanID)
	if err != nil {
		return err
	}

	parsed := parseURLs(rawURLs, info.DomainName)
	if len(parsed) == 0 {
		return nil
	}

	hosts := make([]string, 0, len(parsed))
	for _, item := range parsed {
		hosts = append(hosts, item.Host)
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		subdomainIDs, _, err := upsertSubdomains(tx, scanID, info.DomainID, hosts, "url")
		if err != nil {
			return err
		}

		rows := make([]models.URL, 0, len(parsed))
		for _, item := range parsed {
			subdomainID, ok := subdomainIDs[item.Host]
			if !ok {
				continue
			}
			rows = append(rows, models.URL{
				SubdomainID: subdomainID,
				RawURL:      item.RawURL,
				Hash:        item.Hash,
				Scheme:      item.Scheme,
				Host:        item.Host,
				Path:        item.Path,
				Query:       item.Query,
			})
		}
		if len(rows) == 0 {
			return nil
		}

		return tx.Clauses(clause.OnConflict{DoNothing: true}).
			CreateInBatches(&rows, insertBatchSize).Error
	})
}

// SaveResolutions registra os IPs para os quais um subdomínio resolveu.
func (s *Store) SaveResolutions(scanID uint, subdomainName string, ips []string) error {
	info, err := s.scanInfo(scanID)
	if err != nil {
		return err
	}

	name := normalizeHost(subdomainName)
	if name == "" {
		return errors.New("subdomínio vazio")
	}
	if !inScope(name, info.DomainName) {
		return nil
	}

	ips = normalizeIPs(ips)
	if len(ips) == 0 {
		return nil
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		subdomainIDs, _, err := upsertSubdomains(tx, scanID, info.DomainID, []string{name}, "dns")
		if err != nil {
			return err
		}
		subdomainID, ok := subdomainIDs[name]
		if !ok {
			return fmt.Errorf("subdomínio %q não encontrado após upsert", name)
		}

		hostIDs, err := upsertHosts(tx, ips)
		if err != nil {
			return err
		}

		rows := make([]models.Resolution, 0, len(ips))
		for _, ip := range ips {
			hostID, ok := hostIDs[ip]
			if !ok {
				continue
			}
			rows = append(rows, models.Resolution{
				ScanID:      scanID,
				SubdomainID: subdomainID,
				HostID:      hostID,
			})
		}
		if len(rows) == 0 {
			return nil
		}

		return tx.Clauses(clause.OnConflict{DoNothing: true}).
			CreateInBatches(&rows, insertBatchSize).Error
	})
}

// SaveHTTPProbes persiste a saída do httpx: subdomínios, URLs, IPs,
// resoluções e a observação HTTP do scan. É idempotente: repetir o mesmo scan
// atualiza a observação em vez de duplicá-la.
func (s *Store) SaveHTTPProbes(scanID uint, probes []storage.Probe) error {
	info, err := s.scanInfo(scanID)
	if err != nil {
		return err
	}

	items := make([]probeItem, 0, len(probes))
	seen := make(map[string]int, len(probes))
	for _, probe := range probes {
		rawURL := probe.Input
		if rawURL == "" {
			rawURL = probe.URL
		}

		parsed, ok := parseURL(rawURL, info.DomainName)
		if !ok {
			// Redirecionamentos podem levar para outro host; nesse caso a URL
			// final fica registrada em FinalURL, mas não criamos entidades fora
			// do domínio do scan.
			parsed, ok = parseURL(probe.URL, info.DomainName)
			if !ok {
				continue
			}
		}

		// Duas linhas para a mesma URL no mesmo insert fariam o ON CONFLICT
		// DO UPDATE afetar a mesma linha duas vezes (erro no PostgreSQL).
		key := parsed.Host + "|" + parsed.Hash
		if idx, exists := seen[key]; exists {
			items[idx] = probeItem{parsed: parsed, probe: probe}
			continue
		}
		seen[key] = len(items)
		items = append(items, probeItem{parsed: parsed, probe: probe})
	}
	if len(items) == 0 {
		return nil
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		hosts := make([]string, 0, len(items))
		for _, item := range items {
			hosts = append(hosts, item.parsed.Host)
		}

		subdomainIDs, _, err := upsertSubdomains(tx, scanID, info.DomainID, hosts, "httpx")
		if err != nil {
			return err
		}

		urls := make([]models.URL, 0, len(items))
		hashes := make([]string, 0, len(items))
		for _, item := range items {
			subdomainID, ok := subdomainIDs[item.parsed.Host]
			if !ok {
				continue
			}
			urls = append(urls, models.URL{
				SubdomainID: subdomainID,
				RawURL:      item.parsed.RawURL,
				Hash:        item.parsed.Hash,
				Scheme:      item.parsed.Scheme,
				Host:        item.parsed.Host,
				Path:        item.parsed.Path,
				Query:       item.parsed.Query,
			})
			hashes = append(hashes, item.parsed.Hash)
		}
		if len(urls) == 0 {
			return nil
		}

		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).
			CreateInBatches(&urls, insertBatchSize).Error; err != nil {
			return err
		}

		subdomainIDList := make([]uint, 0, len(subdomainIDs))
		for _, id := range subdomainIDs {
			subdomainIDList = append(subdomainIDList, id)
		}
		storedURLs, err := findStoredURLs(tx, subdomainIDList, uniqueStrings(hashes))
		if err != nil {
			return err
		}
		urlIDs := make(map[urlKey]uint, len(storedURLs))
		for _, u := range storedURLs {
			urlIDs[urlKey{subdomainID: u.SubdomainID, hash: u.Hash}] = u.ID
		}

		ips := make([]string, 0, len(items))
		for _, item := range items {
			ips = append(ips, item.probe.IPs...)
		}
		ips = normalizeIPs(ips)

		hostIDs, err := upsertHosts(tx, ips)
		if err != nil {
			return err
		}

		resolutions := make([]models.Resolution, 0, len(items)*2)
		resolutionKeys := make(map[resolutionKey]struct{}, len(items)*2)
		for _, item := range items {
			subdomainID, ok := subdomainIDs[item.parsed.Host]
			if !ok {
				continue
			}
			for _, ip := range normalizeIPs(item.probe.IPs) {
				hostID, ok := hostIDs[ip]
				if !ok {
					continue
				}
				key := resolutionKey{subdomainID: subdomainID, hostID: hostID}
				if _, exists := resolutionKeys[key]; exists {
					continue
				}
				resolutionKeys[key] = struct{}{}
				resolutions = append(resolutions, models.Resolution{
					ScanID:      scanID,
					SubdomainID: subdomainID,
					HostID:      hostID,
				})
			}
		}
		if len(resolutions) > 0 {
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).
				CreateInBatches(&resolutions, insertBatchSize).Error; err != nil {
				return err
			}
		}

		rows := make([]models.HTTPProbe, 0, len(items))
		for _, item := range items {
			subdomainID, ok := subdomainIDs[item.parsed.Host]
			if !ok {
				continue
			}
			urlID, ok := urlIDs[urlKey{subdomainID: subdomainID, hash: item.parsed.Hash}]
			if !ok {
				continue
			}

			probe := item.probe
			rows = append(rows, models.HTTPProbe{
				ScanID:        scanID,
				URLID:         urlID,
				StatusCode:    probe.StatusCode,
				Title:         probe.Title,
				Server:        probe.Webserver,
				ContentType:   probe.ContentType,
				ContentLength: probe.ContentLength,
				Location:      probe.Location,
				FinalURL:      probe.URL,
				IPs:           probe.IPs,
				CNAMEs:        probe.CNAMEs,
				CDN:           probe.CDN,
				Technologies:  probe.Technologies,
				ResponseTime:  probe.ResponseTime,
				Failed:        probe.Failed,
				Error:         probe.Error,
			})
		}
		if len(rows) == 0 {
			return nil
		}

		return tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "scan_id"}, {Name: "url_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"status_code", "title", "server", "content_type", "content_length",
				"location", "final_url", "ips", "cnames", "cdn", "technologies",
				"response_time", "failed", "error", "updated_at",
			}),
		}).CreateInBatches(&rows, insertBatchSize).Error
	})
}

type scanInfo struct {
	DomainID   uint
	DomainName string
}

func (s *Store) scanInfo(scanID uint) (scanInfo, error) {
	var scan models.Scan
	if err := s.db.Preload("Domain").First(&scan, scanID).Error; err != nil {
		return scanInfo{}, fmt.Errorf("scan %d: %w", scanID, err)
	}

	return scanInfo{
		DomainID:   scan.DomainID,
		DomainName: scan.Domain.Name,
	}, nil
}

// upsertSubdomains grava apenas os nomes que ainda não existem, reutiliza os
// já cadastrados e vincula todos ao scan atual. Retorna o mapa nome->id e a
// quantidade de subdomínios realmente novos.
func upsertSubdomains(tx *gorm.DB, scanID, domainID uint, names []string, source string) (map[string]uint, int, error) {
	nameList := uniqueStrings(names)
	if len(nameList) == 0 {
		return map[string]uint{}, 0, nil
	}
	if source == "" {
		source = "unknown"
	}

	known, err := findSubdomainsByName(tx, domainID, nameList)
	if err != nil {
		return nil, 0, fmt.Errorf("consultar subdomínios: %w", err)
	}

	existing := make(map[string]struct{}, len(known))
	stored := known
	for _, subdomain := range known {
		existing[subdomain.Name] = struct{}{}
	}

	fresh := make([]models.Subdomain, 0, len(nameList)-len(existing))
	freshNames := make([]string, 0, cap(fresh))
	for _, name := range nameList {
		if _, ok := existing[name]; ok {
			continue
		}
		fresh = append(fresh, models.Subdomain{DomainID: domainID, Name: name})
		freshNames = append(freshNames, name)
	}
	if len(fresh) > 0 {
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).
			CreateInBatches(&fresh, insertBatchSize).Error; err != nil {
			return nil, 0, fmt.Errorf("salvar subdomínios: %w", err)
		}

		inserted, err := findSubdomainsByName(tx, domainID, freshNames)
		if err != nil {
			return nil, 0, fmt.Errorf("consultar subdomínios: %w", err)
		}
		stored = append(stored, inserted...)
	}

	ids := make(map[string]uint, len(stored))
	links := make([]models.ScanSubdomain, 0, len(stored))
	for _, subdomain := range stored {
		ids[subdomain.Name] = subdomain.ID
		links = append(links, models.ScanSubdomain{
			ScanID:      scanID,
			SubdomainID: subdomain.ID,
			Source:      source,
		})
	}
	if len(links) > 0 {
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).
			CreateInBatches(&links, insertBatchSize).Error; err != nil {
			return nil, 0, fmt.Errorf("vincular subdomínios ao scan: %w", err)
		}
	}

	return ids, len(fresh), nil
}

// findSubdomainsByName busca os subdomínios já gravados para o domínio,
// dividindo a cláusula IN em lotes para não estourar o limite de parâmetros.
func findSubdomainsByName(tx *gorm.DB, domainID uint, names []string) ([]models.Subdomain, error) {
	var found []models.Subdomain
	for _, nameChunk := range chunkSlice(names, queryBatchSize) {
		var page []models.Subdomain
		if err := tx.Where("domain_id = ? AND name IN ?", domainID, nameChunk).
			Find(&page).Error; err != nil {
			return nil, err
		}
		found = append(found, page...)
	}
	return found, nil
}

func upsertHosts(tx *gorm.DB, ips []string) (map[string]uint, error) {
	ipList := uniqueStrings(ips)
	if len(ipList) == 0 {
		return map[string]uint{}, nil
	}

	rows := make([]models.Host, 0, len(ipList))
	for _, ip := range ipList {
		rows = append(rows, models.Host{IP: ip})
	}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).
		CreateInBatches(&rows, insertBatchSize).Error; err != nil {
		return nil, fmt.Errorf("salvar hosts: %w", err)
	}

	var stored []models.Host
	for _, ipChunk := range chunkSlice(ipList, queryBatchSize) {
		var page []models.Host
		if err := tx.Where("ip IN ?", ipChunk).Find(&page).Error; err != nil {
			return nil, fmt.Errorf("consultar hosts: %w", err)
		}
		stored = append(stored, page...)
	}

	ids := make(map[string]uint, len(stored))
	for _, host := range stored {
		ids[host.IP] = host.ID
	}

	return ids, nil
}

// findStoredURLs busca as URLs já gravadas para os subdomínios e hashes
// informados. Como um scan pode ter dezenas de milhares de URLs, a consulta é
// dividida em lotes para não estourar o limite de parâmetros do PostgreSQL.
func findStoredURLs(tx *gorm.DB, subdomainIDs []uint, hashes []string) ([]models.URL, error) {
	var stored []models.URL
	for _, idChunk := range chunkSlice(subdomainIDs, queryBatchSize) {
		for _, hashChunk := range chunkSlice(hashes, queryBatchSize) {
			var page []models.URL
			if err := tx.Where("subdomain_id IN ? AND hash IN ?", idChunk, hashChunk).
				Find(&page).Error; err != nil {
				return nil, fmt.Errorf("consultar URLs: %w", err)
			}
			stored = append(stored, page...)
		}
	}
	return stored, nil
}

// chunkSlice divide values em fatias de, no máximo, size elementos. É usado
// para manter INSERTs e cláusulas IN dentro do limite de parâmetros do
// protocolo estendido do PostgreSQL.
func chunkSlice[T any](values []T, size int) [][]T {
	if size < 1 {
		size = 1
	}
	chunks := make([][]T, 0, (len(values)+size-1)/size)
	for start := 0; start < len(values); start += size {
		end := start + size
		if end > len(values) {
			end = len(values)
		}
		chunks = append(chunks, values[start:end])
	}
	return chunks
}

type parsedURL struct {
	RawURL string
	Hash   string
	Scheme string
	Host   string
	Path   string
	Query  string
}

type probeItem struct {
	parsed parsedURL
	probe  storage.Probe
}

type urlKey struct {
	subdomainID uint
	hash        string
}

type resolutionKey struct {
	subdomainID uint
	hostID      uint
}

func parseURLs(rawURLs []string, domain string) []parsedURL {
	seen := make(map[string]struct{}, len(rawURLs))
	out := make([]parsedURL, 0, len(rawURLs))

	for _, rawURL := range rawURLs {
		parsed, ok := parseURL(rawURL, domain)
		if !ok {
			continue
		}
		key := fmt.Sprintf("%s|%s", parsed.Host, parsed.Hash)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, parsed)
	}

	return out
}

func parseURL(rawURL, domain string) (parsedURL, bool) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return parsedURL{}, false
	}

	u, err := neturl.Parse(rawURL)
	if err != nil {
		return parsedURL{}, false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return parsedURL{}, false
	}

	host := normalizeHost(u.Hostname())
	if host == "" || !inScope(host, domain) {
		return parsedURL{}, false
	}

	sum := sha256.Sum256([]byte(rawURL))
	return parsedURL{
		RawURL: rawURL,
		Hash:   hex.EncodeToString(sum[:]),
		Scheme: u.Scheme,
		Host:   host,
		Path:   u.EscapedPath(),
		Query:  u.RawQuery,
	}, true
}

// NormalizeDomain transforma uma entrada do usuário no nome canônico do
// domínio (sem esquema, caminho, barra final, espaços ou maiúsculas).
func NormalizeDomain(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	for _, prefix := range []string{"http://", "https://", "ftp://", "ssh://"} {
		name = strings.TrimPrefix(name, prefix)
	}
	if idx := strings.Index(name, "/"); idx >= 0 {
		name = name[:idx]
	}
	name = strings.TrimSuffix(name, ".")
	return name
}

func normalizeHost(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	name = strings.TrimSuffix(name, ".")
	return name
}

func inScope(host, domain string) bool {
	host = normalizeHost(host)
	domain = normalizeHost(domain)
	if host == "" || domain == "" {
		return false
	}
	return host == domain || strings.HasSuffix(host, "."+domain)
}

func normalizeIPs(ips []string) []string {
	seen := make(map[string]struct{}, len(ips))
	out := make([]string, 0, len(ips))
	for _, ip := range ips {
		parsed := net.ParseIP(strings.TrimSpace(ip))
		if parsed == nil {
			continue
		}
		canonical := parsed.String()
		if _, exists := seen[canonical]; exists {
			continue
		}
		seen[canonical] = struct{}{}
		out = append(out, canonical)
	}
	return out
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
