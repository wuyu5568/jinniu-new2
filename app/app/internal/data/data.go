package data

import (
	"context"
	"log"

	"github.com/jinniu/app/app/app/internal/conf"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Data holds shared infrastructure (GORM DB). Schema from scripts/schema.sql only.
type Data struct {
	db *gorm.DB
}

// DB exposes the GORM handle for repositories.
func (d *Data) DB() *gorm.DB {
	return d.db
}

// Ping checks MySQL connectivity (for readiness /health).
func (d *Data) Ping(ctx context.Context) error {
	sqlDB, err := d.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

// NewData opens MySQL via GORM without AutoMigrate.
func NewData(c *conf.Data) (*Data, func(), error) {
	cfg := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	}
	db, err := gorm.Open(mysql.Open(c.Database.Source), cfg)
	if err != nil {
		return nil, nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, nil, err
	}
	d := &Data{db: db}
	cleanup := func() {
		if err := sqlDB.Close(); err != nil {
			log.Printf("close db: %v", err)
		}
	}
	return d, cleanup, nil
}
