package models

import "time"

// Base é a base dos modelos de recon. Diferente de gorm.Model, não inclui
// DeletedAt (soft delete): os dados de recon são históricos e índices únicos
// não devem ignorar registros removidos.
type Base struct {
	ID        uint `gorm:"primaryKey"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Program representa um programa de bug bounty (HackerOne, Bugcrowd, etc.).
// Scans avulsos são associados ao programa "manual".
type Program struct {
	Base

	Platform string `gorm:"size:100;not null;default:manual;uniqueIndex:ux_programs_platform_name,priority:1"`
	Name     string `gorm:"size:100;not null;uniqueIndex:ux_programs_platform_name,priority:2"`
	IsActive bool   `gorm:"not null;default:false"`

	Domains []Domain `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

// Domain é o alvo raiz de um programa.
type Domain struct {
	Base

	ProgramID uint   `gorm:"not null;index;uniqueIndex:ux_domains_program_name,priority:1"`
	Name      string `gorm:"size:255;not null;uniqueIndex:ux_domains_program_name,priority:2"`

	Program    Program     `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Subdomains []Subdomain `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Scans      []Scan      `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

// Scan é uma execução do pipeline sobre um domínio.
type Scan struct {
	Base

	DomainID   uint      `gorm:"not null;index"`
	Status     string    `gorm:"size:20;not null;default:running"`
	StartedAt  time.Time `gorm:"not null"`
	FinishedAt *time.Time
	Error      string `gorm:"type:text"`

	Domain Domain `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

// Subdomain é um hostname descoberto para um domínio. O nome é único por
// domínio, independente de quantos scans o encontraram.
type Subdomain struct {
	Base

	DomainID uint   `gorm:"not null;index;uniqueIndex:ux_subdomains_domain_name,priority:1"`
	Name     string `gorm:"size:255;not null;uniqueIndex:ux_subdomains_domain_name,priority:2"`

	Domain      Domain       `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Resolutions []Resolution `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	URLs        []URL        `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

// ScanSubdomain registra em qual scan um subdomínio foi descoberto e por qual
// fonte (subfinder, puredns, httpx...).
type ScanSubdomain struct {
	ScanID      uint   `gorm:"primaryKey;autoIncrement:false"`
	SubdomainID uint   `gorm:"primaryKey;autoIncrement:false"`
	Source      string `gorm:"size:50;not null"`
	CreatedAt   time.Time

	Scan      Scan      `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Subdomain Subdomain `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

// Host é um endereço IP para o qual um subdomínio resolveu.
type Host struct {
	Base

	IP string `gorm:"column:ip;size:45;not null;uniqueIndex:ux_hosts_ip"`

	Resolutions []Resolution `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Ports       []Port       `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

// Resolution liga um subdomínio a um host em um scan específico.
type Resolution struct {
	Base

	ScanID      uint `gorm:"not null;index;uniqueIndex:ux_resolutions_scan_subdomain_host,priority:1"`
	SubdomainID uint `gorm:"not null;index;uniqueIndex:ux_resolutions_scan_subdomain_host,priority:2"`
	HostID      uint `gorm:"not null;index;uniqueIndex:ux_resolutions_scan_subdomain_host,priority:3"`

	Scan      Scan      `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Subdomain Subdomain `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Host      Host      `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

// Port é uma porta aberta em um host (usada por naabu/gobuster no futuro).
type Port struct {
	Base

	HostID   uint   `gorm:"not null;index;uniqueIndex:ux_ports_host_number_protocol,priority:1"`
	Number   int    `gorm:"not null;uniqueIndex:ux_ports_host_number_protocol,priority:2"`
	Protocol string `gorm:"size:10;not null;default:tcp;uniqueIndex:ux_ports_host_number_protocol,priority:3"`
	Service  string `gorm:"size:50"`
	State    string `gorm:"size:20;not null;default:open"`

	Host Host `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

// URL é uma URL descoberta para um subdomínio. O hash evita índices únicos em
// colunas TEXT longas (limite de tamanho de índice do PostgreSQL).
type URL struct {
	Base

	SubdomainID uint   `gorm:"not null;index;uniqueIndex:ux_urls_subdomain_hash,priority:1"`
	RawURL      string `gorm:"type:text;not null"`
	Hash        string `gorm:"size:64;not null;uniqueIndex:ux_urls_subdomain_hash,priority:2"`

	Scheme string `gorm:"size:10"`
	Host   string `gorm:"size:255;index"`
	Path   string `gorm:"type:text"`
	Query  string `gorm:"type:text"`

	Subdomain Subdomain   `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Probes    []HTTPProbe `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

// HTTPProbe é o resultado de uma validação HTTP (httpx) em um scan.
type HTTPProbe struct {
	Base

	ScanID uint `gorm:"not null;index;uniqueIndex:ux_http_probes_scan_url,priority:1"`
	URLID  uint `gorm:"not null;index;uniqueIndex:ux_http_probes_scan_url,priority:2"`

	StatusCode    int    `gorm:"column:status_code"`
	Title         string `gorm:"type:text"`
	Server        string `gorm:"size:255"`
	ContentType   string `gorm:"size:255"`
	ContentLength int64
	Location      string `gorm:"type:text"`
	FinalURL      string `gorm:"type:text"`

	IPs          []string `gorm:"serializer:json;type:jsonb"`
	CNAMEs       []string `gorm:"column:cnames;serializer:json;type:jsonb"`
	CDN          string   `gorm:"size:100"`
	Technologies []string `gorm:"serializer:json;type:jsonb"`

	ResponseTime string `gorm:"size:50"`
	Failed       bool   `gorm:"not null;default:false"`
	Error        string `gorm:"type:text"`

	Scan Scan `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	URL  URL  `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}
