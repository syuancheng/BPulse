package migrations

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const createSchemaMigrations = `CREATE TABLE IF NOT EXISTS schema_migrations (
  version VARCHAR(64) NOT NULL PRIMARY KEY,
  checksum CHAR(64) NOT NULL,
  state VARCHAR(16) NOT NULL,
  applied_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
) ENGINE=InnoDB`

const migrationLockName = "bp_companion_schema_migrations"

type file struct {
	version  string
	path     string
	checksum string
}

func Up(ctx context.Context, db *sql.DB, directory string, statementTimeout time.Duration) error {
	return withLock(ctx, db, func(conn *sql.Conn) error {
		if _, err := conn.ExecContext(ctx, createSchemaMigrations); err != nil {
			return fmt.Errorf("create schema migrations table: %w", err)
		}
		if err := rejectDirtyMigrations(ctx, conn); err != nil {
			return err
		}
		files, err := migrationFiles(directory, ".up.sql")
		if err != nil {
			return err
		}
		if err := rejectMissingMigrationFiles(ctx, conn, files); err != nil {
			return err
		}
		for _, migration := range files {
			var checksum, state string
			err := conn.QueryRowContext(ctx, "SELECT checksum, state FROM schema_migrations WHERE version = ?", migration.version).Scan(&checksum, &state)
			switch {
			case err == nil:
				if checksum != migration.checksum {
					return fmt.Errorf("migration %s checksum changed", migration.version)
				}
				if state != "applied" {
					return fmt.Errorf("migration %s is dirty with state %s; inspect schema before recovery", migration.version, state)
				}
				continue
			case err != sql.ErrNoRows:
				return fmt.Errorf("query migration %s: %w", migration.version, err)
			}
			if err := apply(ctx, conn, migration, statementTimeout); err != nil {
				return err
			}
		}
		return nil
	})
}

func DownOne(ctx context.Context, db *sql.DB, directory string, statementTimeout time.Duration) error {
	return withLock(ctx, db, func(conn *sql.Conn) error {
		if _, err := conn.ExecContext(ctx, createSchemaMigrations); err != nil {
			return fmt.Errorf("create schema migrations table: %w", err)
		}
		if err := rejectDirtyMigrations(ctx, conn); err != nil {
			return err
		}

		var version string
		err := conn.QueryRowContext(ctx, "SELECT version FROM schema_migrations ORDER BY version DESC LIMIT 1").Scan(&version)
		if err == sql.ErrNoRows {
			return nil
		}
		if err != nil {
			return fmt.Errorf("find latest migration: %w", err)
		}
		path := filepath.Join(directory, version+".down.sql")
		contents, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read down migration %s: %w", version, err)
		}
		if err := validateSingleStatement(contents); err != nil {
			return fmt.Errorf("validate down migration %s: %w", version, err)
		}
		if _, err := conn.ExecContext(ctx, "UPDATE schema_migrations SET state = 'reverting' WHERE version = ? AND state = 'applied'", version); err != nil {
			return fmt.Errorf("mark migration %s reverting: %w", version, err)
		}
		statementCtx, cancelStatement := context.WithTimeout(ctx, statementTimeout)
		_, err = conn.ExecContext(statementCtx, string(contents))
		cancelStatement()
		if err != nil {
			return fmt.Errorf("execute down migration %s (left in reverting state): %w", version, err)
		}
		if _, err := conn.ExecContext(ctx, "DELETE FROM schema_migrations WHERE version = ? AND state = 'reverting'", version); err != nil {
			return fmt.Errorf("remove migration %s (left in reverting state): %w", version, err)
		}
		return nil
	})
}

func rejectDirtyMigrations(ctx context.Context, conn *sql.Conn) error {
	var version, state string
	err := conn.QueryRowContext(ctx, "SELECT version, state FROM schema_migrations WHERE state <> 'applied' ORDER BY version LIMIT 1").Scan(&version, &state)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return fmt.Errorf("check dirty migrations: %w", err)
	}
	return fmt.Errorf("migration %s is dirty with state %s; inspect schema before recovery", version, state)
}

func rejectMissingMigrationFiles(ctx context.Context, conn *sql.Conn, files []file) error {
	available := make(map[string]struct{}, len(files))
	for _, migration := range files {
		available[migration.version] = struct{}{}
	}
	rows, err := conn.QueryContext(ctx, "SELECT version FROM schema_migrations WHERE state = 'applied' ORDER BY version")
	if err != nil {
		return fmt.Errorf("list applied migrations: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return fmt.Errorf("scan applied migration: %w", err)
		}
		if _, ok := available[version]; !ok {
			return fmt.Errorf("applied migration %s is missing from migration directory", version)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate applied migrations: %w", err)
	}
	return nil
}

func apply(ctx context.Context, conn *sql.Conn, migration file, statementTimeout time.Duration) error {
	contents, err := os.ReadFile(migration.path)
	if err != nil {
		return fmt.Errorf("read migration %s: %w", migration.version, err)
	}
	if err := validateSingleStatement(contents); err != nil {
		return fmt.Errorf("validate migration %s: %w", migration.version, err)
	}
	if _, err := conn.ExecContext(ctx, "INSERT INTO schema_migrations (version, checksum, state) VALUES (?, ?, 'applying')", migration.version, migration.checksum); err != nil {
		return fmt.Errorf("mark migration %s applying: %w", migration.version, err)
	}
	statementCtx, cancelStatement := context.WithTimeout(ctx, statementTimeout)
	_, err = conn.ExecContext(statementCtx, string(contents))
	cancelStatement()
	if err != nil {
		return fmt.Errorf("execute migration %s (left in applying state): %w", migration.version, err)
	}
	if _, err := conn.ExecContext(ctx, "UPDATE schema_migrations SET state = 'applied', applied_at = CURRENT_TIMESTAMP(6) WHERE version = ? AND state = 'applying'", migration.version); err != nil {
		return fmt.Errorf("mark migration %s applied (left in applying state): %w", migration.version, err)
	}
	return nil
}

func withLock(ctx context.Context, db *sql.DB, operation func(*sql.Conn) error) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("reserve migration connection: %w", err)
	}
	defer conn.Close()
	var acquired int
	if err := conn.QueryRowContext(ctx, "SELECT GET_LOCK(?, 10)", migrationLockName).Scan(&acquired); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	if acquired != 1 {
		return fmt.Errorf("acquire migration lock: timed out")
	}
	defer func() {
		releaseCtx, cancelRelease := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelRelease()
		_, _ = conn.ExecContext(releaseCtx, "SELECT RELEASE_LOCK(?)", migrationLockName)
	}()
	return operation(conn)
}

func validateSingleStatement(contents []byte) error {
	count, err := countSQLStatements(string(contents))
	if err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("SQL statement is empty")
	}
	if count != 1 {
		return fmt.Errorf("found %d SQL statements; use one numbered file per statement", count)
	}
	return nil
}

func countSQLStatements(sqlText string) (int, error) {
	const (
		stateNormal = iota
		stateSingleQuote
		stateDoubleQuote
		stateBacktick
		stateLineComment
		stateBlockComment
	)
	state := stateNormal
	statements := 0
	hasToken := false

	for i := 0; i < len(sqlText); i++ {
		char := sqlText[i]
		switch state {
		case stateNormal:
			switch {
			case char == '-' && i+2 < len(sqlText) && sqlText[i+1] == '-' && isSQLWhitespace(sqlText[i+2]):
				state = stateLineComment
				i++
			case char == '#':
				state = stateLineComment
			case char == '/' && i+1 < len(sqlText) && sqlText[i+1] == '*':
				state = stateBlockComment
				i++
			case char == '\'':
				state = stateSingleQuote
				hasToken = true
			case char == '"':
				state = stateDoubleQuote
				hasToken = true
			case char == '`':
				state = stateBacktick
				hasToken = true
			case char == ';':
				if hasToken {
					statements++
					hasToken = false
				}
			case isSQLWhitespace(char):
			default:
				hasToken = true
			}
		case stateSingleQuote, stateDoubleQuote, stateBacktick:
			quote := byte('\'')
			if state == stateDoubleQuote {
				quote = '"'
			} else if state == stateBacktick {
				quote = '`'
			}
			if char == '\\' && state != stateBacktick && i+1 < len(sqlText) {
				i++
			} else if char == quote {
				if i+1 < len(sqlText) && sqlText[i+1] == quote {
					i++
				} else {
					state = stateNormal
				}
			}
		case stateLineComment:
			if char == '\n' || char == '\r' {
				state = stateNormal
			}
		case stateBlockComment:
			if char == '*' && i+1 < len(sqlText) && sqlText[i+1] == '/' {
				state = stateNormal
				i++
			}
		}
	}

	if state == stateSingleQuote || state == stateDoubleQuote || state == stateBacktick || state == stateBlockComment {
		return 0, fmt.Errorf("unterminated SQL quote or block comment")
	}
	if hasToken {
		statements++
	}
	return statements, nil
}

func isSQLWhitespace(char byte) bool {
	return char == ' ' || char == '\t' || char == '\n' || char == '\r' || char == '\f'
}

func migrationFiles(directory, suffix string) ([]file, error) {
	paths, err := filepath.Glob(filepath.Join(directory, "*"+suffix))
	if err != nil {
		return nil, fmt.Errorf("list migrations: %w", err)
	}
	result := make([]file, 0, len(paths))
	for _, path := range paths {
		base := filepath.Base(path)
		version := strings.TrimSuffix(base, suffix)
		if version == "" {
			return nil, fmt.Errorf("invalid migration filename %q", base)
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", version, err)
		}
		hash := sha256.Sum256(contents)
		result = append(result, file{version: version, path: path, checksum: hex.EncodeToString(hash[:])})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].version < result[j].version })
	return result, nil
}
