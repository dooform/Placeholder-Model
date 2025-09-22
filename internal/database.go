package internal

import (
	"fmt"

	"DF-PLCH/internal/config"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var DB *gorm.DB

func InitDB(cfg *config.Config) error {
	dsn := cfg.Database.DSN()

	var err error
	DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// Auto-migrate the schema
	if err := autoMigrate(); err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	fmt.Println("Database connected and migrated successfully")
	return nil
}

func autoMigrate() error {
	// Drop all existing tables to ensure clean state
	fmt.Println("Dropping all existing tables...")
	DB.Exec("SET FOREIGN_KEY_CHECKS = 0")
	DB.Exec("DROP TABLE IF EXISTS activity_logs")
	DB.Exec("DROP TABLE IF EXISTS documents")
	DB.Exec("DROP TABLE IF EXISTS templates")
	DB.Exec("SET FOREIGN_KEY_CHECKS = 1")

	// Create tables with raw SQL to avoid GORM foreign key issues
	fmt.Println("Creating templates table with raw SQL...")
	result := DB.Exec(`
		CREATE TABLE templates (
			id varchar(191) PRIMARY KEY,
			filename longtext NOT NULL,
			original_name longtext,
			gcs_path longtext NOT NULL,
			file_size bigint,
			mime_type longtext,
			placeholders json,
			created_at datetime(3) NULL,
			updated_at datetime(3) NULL,
			deleted_at datetime(3) NULL,
			INDEX idx_templates_deleted_at (deleted_at)
		)
	`)
	if result.Error != nil {
		return fmt.Errorf("failed to create templates table: %w", result.Error)
	}

	fmt.Println("Creating documents table with raw SQL...")
	result = DB.Exec(`
		CREATE TABLE documents (
			id varchar(191) PRIMARY KEY,
			template_id varchar(191) NOT NULL,
			filename longtext NOT NULL,
			gcs_path longtext NOT NULL,
			file_size bigint,
			mime_type longtext,
			data json,
			status varchar(191) DEFAULT 'completed',
			created_at datetime(3) NULL,
			updated_at datetime(3) NULL,
			deleted_at datetime(3) NULL,
			INDEX idx_documents_template_id (template_id),
			INDEX idx_documents_deleted_at (deleted_at)
		)
	`)
	if result.Error != nil {
		return fmt.Errorf("failed to create documents table: %w", result.Error)
	}

	fmt.Println("Creating activity_logs table with raw SQL...")
	result = DB.Exec(`
		CREATE TABLE activity_logs (
			id varchar(191) PRIMARY KEY,
			method varchar(10) NOT NULL,
			path varchar(255) NOT NULL,
			user_agent text,
			ip_address varchar(45),
			request_body text,
			query_params text,
			status_code int NOT NULL,
			response_time bigint NOT NULL,
			created_at datetime(3) NULL,
			updated_at datetime(3) NULL,
			deleted_at datetime(3) NULL,
			INDEX idx_activity_logs_deleted_at (deleted_at),
			INDEX idx_activity_logs_method (method),
			INDEX idx_activity_logs_path (path),
			INDEX idx_activity_logs_created_at (created_at)
		)
	`)
	if result.Error != nil {
		return fmt.Errorf("failed to create activity_logs table: %w", result.Error)
	}

	fmt.Println("Tables created successfully with raw SQL")
	return nil
}

func CloseDB() error {
	if DB != nil {
		sqlDB, err := DB.DB()
		if err != nil {
			return err
		}
		return sqlDB.Close()
	}
	return nil
}