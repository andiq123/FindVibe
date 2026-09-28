package database

import (
	"github.com/andiq123/FindVibeFiber/internal/config"
	"github.com/andiq123/FindVibeFiber/internal/utils"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func InitDb(dbConfig config.DatabaseConfig) *gorm.DB {
	db, err := gorm.Open(postgres.Open(dbConfig.DSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		utils.GetLogger().Error("Failed to connect to PostgreSQL", "error", err)
		panic(err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		utils.GetLogger().Error("Failed to get database connection", "error", err)
		panic(err)
	}

	sqlDB.SetMaxOpenConns(dbConfig.MaxOpenConns)
	sqlDB.SetMaxIdleConns(dbConfig.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(dbConfig.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(dbConfig.ConnMaxIdleTime)

	utils.GetLogger().Info("PostgreSQL connected",
		"maxOpenConns", dbConfig.MaxOpenConns,
		"maxIdleConns", dbConfig.MaxIdleConns)

	migrate(db)
	return db
}

func migrate(db *gorm.DB) {
	// ponytail: inline DDL, no migrate framework; UUIDs come from Go, no uuid-ossp
	for _, sql := range []string{
		`CREATE TABLE IF NOT EXISTS users (
			id VARCHAR(255) PRIMARY KEY,
			name VARCHAR(255) NOT NULL UNIQUE
		)`,
		`CREATE TABLE IF NOT EXISTS favorite_songs (
			id VARCHAR(255) PRIMARY KEY,
			title VARCHAR(500) NOT NULL,
			artist VARCHAR(500) NOT NULL,
			image VARCHAR(1000),
			link VARCHAR(1000),
			"order" INTEGER NOT NULL DEFAULT 0,
			user_uuid VARCHAR(255) NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT fk_favorite_songs_user FOREIGN KEY (user_uuid) REFERENCES users(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_favorite_songs_id_user ON favorite_songs(id, user_uuid)`,
		`CREATE INDEX IF NOT EXISTS idx_favorite_songs_user_order ON favorite_songs(user_uuid, "order")`,
		`CREATE INDEX IF NOT EXISTS idx_favorite_songs_created_at ON favorite_songs(created_at)`,
		`ALTER TABLE favorite_songs ADD COLUMN IF NOT EXISTS lyrics TEXT`,
		`CREATE OR REPLACE FUNCTION update_updated_at_column()
		RETURNS TRIGGER AS $$
		BEGIN
			NEW.updated_at = CURRENT_TIMESTAMP;
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql`,
		`DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM pg_trigger WHERE tgname = 'update_favorite_songs_updated_at'
			) THEN
				CREATE TRIGGER update_favorite_songs_updated_at
					BEFORE UPDATE ON favorite_songs
					FOR EACH ROW
					EXECUTE FUNCTION update_updated_at_column();
			END IF;
		END $$`,
	} {
		if err := db.Exec(sql).Error; err != nil {
			utils.GetLogger().Warn("Migration warning", "error", err)
		}
	}
}
