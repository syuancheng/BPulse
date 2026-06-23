package config

import (
	"testing"

	"github.com/go-sql-driver/mysql"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("APP_ENV", "local")
	t.Setenv("PORT", "")
	t.Setenv("MYSQL_DSN", "synthetic:password@tcp(localhost:3306)/synthetic")
	t.Setenv("DATA_ENCRYPTION_KEY_B64", "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=")

	got, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Environment != "local" || got.Address != ":8080" {
		t.Fatalf("Load() = %#v", got)
	}
	if got.MySQLDSN == "" {
		t.Fatal("Load() returned empty normalized DSN")
	}
}

func TestLoadRejectsInvalidPort(t *testing.T) {
	t.Setenv("APP_ENV", "local")
	t.Setenv("MYSQL_DSN", "synthetic:password@tcp(localhost:3306)/synthetic")
	t.Setenv("DATA_ENCRYPTION_KEY_B64", "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=")
	t.Setenv("PORT", "70000")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want invalid port error")
	}
}

func TestLoadRejectsInvalidEnvironment(t *testing.T) {
	t.Setenv("MYSQL_DSN", "synthetic:password@tcp(localhost:3306)/synthetic")
	t.Setenv("DATA_ENCRYPTION_KEY_B64", "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=")
	t.Setenv("APP_ENV", "staging")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want invalid environment error")
	}
}

func TestLoadRequiresMySQLDSN(t *testing.T) {
	t.Setenv("APP_ENV", "local")
	t.Setenv("MYSQL_DSN", "")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want missing MYSQL_DSN error")
	}
}

func TestLoadRequiresExplicitEnvironment(t *testing.T) {
	t.Setenv("APP_ENV", "")
	t.Setenv("MYSQL_DSN", "synthetic:password@tcp(localhost:3306)/synthetic")
	t.Setenv("DATA_ENCRYPTION_KEY_B64", "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want missing APP_ENV error")
	}
}

func TestLoadRequiresDataEncryptionKey(t *testing.T) {
	t.Setenv("APP_ENV", "local")
	t.Setenv("MYSQL_DSN", "synthetic:password@tcp(localhost:3306)/synthetic")
	t.Setenv("DATA_ENCRYPTION_KEY_B64", "")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want missing DATA_ENCRYPTION_KEY_B64 error")
	}
}

func TestNormalizeMySQLDSNForcesUTC(t *testing.T) {
	got, err := normalizeMySQLDSN("synthetic:password@tcp(localhost:3306)/synthetic?parseTime=false&loc=Local&time_zone=%27%2B08%3A00%27")
	if err != nil {
		t.Fatalf("normalizeMySQLDSN() error = %v", err)
	}
	parsed, err := mysql.ParseDSN(got)
	if err != nil {
		t.Fatalf("parse normalized DSN: %v", err)
	}
	if !parsed.ParseTime || parsed.Loc.String() != "UTC" || parsed.Params["time_zone"] != "'+00:00'" {
		t.Fatalf("normalized DSN config = %#v", parsed)
	}
}
