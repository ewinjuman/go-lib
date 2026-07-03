package database

import (
	"cmp"
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func createPostgresConnection(config Option) (*gorm.DB, error) {
	sslMode := func(b bool) string {
		if b {
			return "enable"
		}
		return "disable"
	}
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=%s TimeZone=%s",
		config.Host,
		config.Username,
		config.Password,
		config.Schema,
		config.Port,
		sslMode(config.SslMode),
		cmp.Or(config.Timezone, "Asia/Jakarta"))

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return db, fmt.Errorf("failed to connect postgres: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return db, fmt.Errorf("failed to get postgres connection: %w", err)
	}
	if err := sqlDB.Ping(); err != nil {
		return db, fmt.Errorf("failed to ping postgres: %w", err)
	}

	sqlDB.SetMaxIdleConns(config.MaxIdleConn)
	sqlDB.SetMaxOpenConns(config.MaxOpenConn)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)

	return db, nil
}
