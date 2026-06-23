package migrations

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrationFilesAreOrderedAndChecksummed(t *testing.T) {
	directory := t.TempDir()
	for name, body := range map[string]string{
		"000002_second.up.sql": "SELECT 2",
		"000001_first.up.sql":  "SELECT 1",
	} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(body), 0o600); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
	}

	got, err := migrationFiles(directory, ".up.sql")
	if err != nil {
		t.Fatalf("migrationFiles() error = %v", err)
	}
	if len(got) != 2 || got[0].version != "000001_first" || got[1].version != "000002_second" {
		t.Fatalf("migrationFiles() versions = %#v", got)
	}
	if len(got[0].checksum) != 64 {
		t.Fatalf("checksum length = %d, want 64", len(got[0].checksum))
	}
}

func TestValidateSingleStatement(t *testing.T) {
	tests := []struct {
		name    string
		sql     string
		wantErr bool
	}{
		{name: "one statement", sql: "CREATE TABLE example (id BIGINT);"},
		{name: "semicolon in string", sql: "INSERT INTO example (note) VALUES ('one;two');"},
		{name: "semicolon in comments", sql: "/* migration; comment */ CREATE TABLE example (id BIGINT); -- trailing; comment"},
		{name: "empty", sql: "  ", wantErr: true},
		{name: "multiple statements", sql: "SELECT 1; SELECT 2;", wantErr: true},
		{name: "unterminated string", sql: "SELECT 'value", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateSingleStatement([]byte(test.sql))
			if (err != nil) != test.wantErr {
				t.Fatalf("validateSingleStatement() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}
