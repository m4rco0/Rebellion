package database

import (
	"fmt"

	"github.com/m4rco0/Rebellion/internal/database/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

// InitDB conecta ao PostgreSQL e aplica o schema atual via AutoMigrate.
//
// AutoMigrate é adequado enquanto o schema ainda está em desenvolvimento; ele
// não remove/renomeia colunas nem executa migrações de dados. Quando o schema
// estabilizar, troque por migrações versionadas (goose/atlas).
func InitDB(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("conectar ao postgres: %w", err)
	}

	if err := db.AutoMigrate(
		&models.Program{},
		&models.Domain{},
		&models.Scan{},
		&models.Subdomain{},
		&models.ScanSubdomain{},
		&models.Host{},
		&models.Port{},
		&models.Resolution{},
		&models.URL{},
		&models.HTTPProbe{},
	); err != nil {
		return nil, fmt.Errorf("migrar schema: %w", err)
	}

	DB = db
	return db, nil
}
