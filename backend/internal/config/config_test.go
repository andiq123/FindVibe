package config

import (
	"os"
	"strings"
	"testing"
)

func TestDatabaseDSNPrefersDATABASE_URL(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://u:p@h:5432/db?sslmode=require")
	t.Setenv("DB_HOST", "should-not-use")
	dsn := databaseDSN()
	if dsn != "postgres://u:p@h:5432/db?sslmode=require" {
		t.Fatalf("expected DATABASE_URL, got %q", dsn)
	}
}

func TestDatabaseDSNFromParts(t *testing.T) {
	os.Unsetenv("DATABASE_URL")
	t.Setenv("DB_HOST", "postgresql-find-vibe.alwaysdata.net")
	t.Setenv("DB_PORT", "5432")
	t.Setenv("DB_USER", "find-vibe")
	t.Setenv("DB_PASSWORD", "secret")
	t.Setenv("DB_NAME", "find-vibe_db")
	t.Setenv("DB_SSLMODE", "require")

	dsn := databaseDSN()
	for _, want := range []string{
		"host=postgresql-find-vibe.alwaysdata.net",
		"user=find-vibe",
		"dbname=find-vibe_db",
		"sslmode=require",
	} {
		if !strings.Contains(dsn, want) {
			t.Fatalf("dsn %q missing %q", dsn, want)
		}
	}
}
