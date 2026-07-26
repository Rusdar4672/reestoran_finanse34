package repository

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/yourusername/restaurant-finance/internal/config"
	"github.com/yourusername/restaurant-finance/internal/core"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Store struct {
	DB *gorm.DB
}

func ConnectDB(cfg *config.Config) (*Store, error) {
	switch cfg.DBDriver {
	case "sqlite":
		return connectSQLite(cfg)
	case "postgres", "":
		return connectPostgres(cfg)
	default:
		return nil, fmt.Errorf("unsupported database driver %q", cfg.DBDriver)
	}
}

func connectPostgres(cfg *config.Config) (*Store, error) {
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%d sslmode=%s TimeZone=%s",
		cfg.DBHost, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBPort, cfg.DBSSLMode, cfg.Timezone,
	)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("connect PostgreSQL: %w", err)
	}
	store := &Store{DB: db}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("access connection pool: %w", err)
	}
	sqlDB.SetMaxOpenConns(cfg.DBMaxOpen)
	sqlDB.SetMaxIdleConns(cfg.DBMaxIdle)
	sqlDB.SetConnMaxLifetime(cfg.DBMaxLifetime)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping PostgreSQL: %w", err)
	}
	if err := store.Migrate(); err != nil {
		return nil, err
	}
	return store, nil
}

func connectSQLite(cfg *config.Config) (*Store, error) {
	if cfg.SQLitePath == "" {
		return nil, fmt.Errorf("sqlite path is required")
	}
	absolutePath, err := filepath.Abs(cfg.SQLitePath)
	if err != nil {
		return nil, fmt.Errorf("resolve sqlite path: %w", err)
	}
	dsn := absolutePath +
		"?_pragma=foreign_keys(1)" +
		"&_pragma=journal_mode(WAL)" +
		"&_pragma=synchronous(NORMAL)" +
		"&_pragma=busy_timeout(5000)"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{PrepareStmt: true})
	if err != nil {
		return nil, fmt.Errorf("connect sqlite: %w", err)
	}
	store := &Store{DB: db}
	if err := store.configureSQLite(cfg); err != nil {
		return nil, err
	}
	if err := store.Migrate(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *Store) configureSQLite(cfg *config.Config) error {
	sqlDB, err := s.DB.DB()
	if err != nil {
		return err
	}
	sqlDB.SetMaxOpenConns(cfg.DBMaxOpen)
	sqlDB.SetMaxIdleConns(cfg.DBMaxIdle)
	return nil
}

func (s *Store) Migrate() error {
	if err := s.DB.AutoMigrate(
		&core.Restaurant{},
		&core.FinancialCategory{},
		&core.FinancialEntry{},
		&core.PlanValue{},
		&core.CalculationRule{},
		&core.Employee{},
		&core.Shift{},
		&core.POSConnection{},
	); err != nil {
		return fmt.Errorf("auto migrate: %w", err)
	}
	if err := s.DB.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS ux_financial_entries_external
		ON financial_entries (restaurant_id, source, external_id)
		WHERE external_id <> ''
	`).Error; err != nil {
		return fmt.Errorf("create external entry index: %w", err)
	}
	return nil
}
