package database

import (
	"fmt"
	"time"

	Logger "github.com/ewinjuman/go-lib/v2/logger"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func NewConnection(config Option, logger *Logger.Logger) (*gorm.DB, error) {
	var conn *gorm.DB
	var err error

	switch config.DbType {
	case "mysql":
		conn, err = createMysqlConnection(config)
	case "postgres", "":
		conn, err = createPostgresConnection(config)
	default:
		return nil, fmt.Errorf("unsupported database type: %s", config.DbType)
	}
	if err != nil {
		return nil, err
	}

	logLevel := gormlogger.Silent
	if config.LogMode {
		logLevel = gormlogger.Info
	}
	conn.Logger = Logger.NewGormLogger(logger, gormlogger.Config{
		SlowThreshold:             time.Second,
		LogLevel:                  logLevel,
		IgnoreRecordNotFoundError: false,
	})

	return conn, nil
}
