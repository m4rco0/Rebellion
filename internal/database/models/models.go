package models

import "gorm.io/gorm"

type Program struct {
	gorm.Model
	Platform string   `gorm:"size:100"`
	IsActive bool     `gorm:"default:false"`
	Name     string   `gorm:"size:100"`
	Domains  []Domain `gorm:"foreignKey:ProgramID"`
}

type Domain struct {
	gorm.Model
	ProgramID  uint        `gorm:"index"`
	Name       string      `gorm:"size:100"`
	Subdomains []Subdomain `gorm:"foreignKey:DomainID"`
}

type Subdomain struct {
	gorm.Model
	DomainID uint   `gorm:"index"`
	Name     string `gorm:"size:100"`
	Hosts    []Host `gorm:"many2many:subdomain_hosts;"`
}

type Host struct {
	gorm.Model
	EnderecoIP string      `gorm:"size:100;uniqueIndex"`
	Subdomains []Subdomain `gorm:"many2many:subdomain_hosts;"`
	Portas     []Ports     `gorm:"foreignKey:HostID"`
}

type Ports struct {
	gorm.Model
	HostID   uint `gorm:"index"`
	OpenPort int
}

type Endpoint struct {
	gorm.Model
	SubdomainID uint `gorm:"index"`
	PortID      uint `gorm:"index"`
}

type URL struct {
	gorm.Model
	SubdomainID uint   `gorm:"index"`
	PortID      uint   `gorm:"index"`
	PathURL     string `gorm:"size:255;uniqueIndex;not null"`
	StatusCode  int
	Tecnologia  string `gorm:"size:100"`
}
