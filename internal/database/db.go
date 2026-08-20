package database

import (
	"log"

	"github.com/m4rco0/Rebellion/internal/database/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func InitDB(dsn string) (*gorm.DB, error) {
	var err error
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})

	if err != nil {
		log.Fatalln("[!] Erro ao conectar com o banco de dados:", err)
		return nil, err
	}

	err = db.AutoMigrate(
		&models.Program{},
		&models.Domain{},
		&models.Endpoint{},
		&models.Host{},
		&models.Port{},
		&models.Scan{},
		&models.Subdomain{},
		&models.URL{},
	)

	if err != nil {
		log.Fatalln("[!] Erro ao migrar o banco de dados:", err)
		return nil, err
	}
	return db, err
}
