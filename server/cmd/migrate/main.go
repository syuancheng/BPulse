package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/syuancheng/BPulse/server/internal/migrations"
)

func main() {
	direction := flag.String("direction", "up", "migration direction: up or down-one")
	directory := flag.String("dir", "migrations", "migration directory")
	statementTimeout := flag.Duration("statement-timeout", durationFromEnvironment("MIGRATION_STATEMENT_TIMEOUT", 5*time.Minute), "timeout for each migration SQL statement")
	flag.Parse()
	if *statementTimeout <= 0 {
		log.Fatal("statement-timeout must be positive")
	}

	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		log.Fatal("MYSQL_DSN is required")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()
	db.SetConnMaxLifetime(3 * time.Minute)
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	pingCtx, cancelPing := context.WithTimeout(ctx, 10*time.Second)
	if err := db.PingContext(pingCtx); err != nil {
		cancelPing()
		log.Fatalf("connect database: %v", err)
	}
	cancelPing()

	switch *direction {
	case "up":
		err = migrations.Up(ctx, db, *directory, *statementTimeout)
	case "down-one":
		err = migrations.DownOne(ctx, db, *directory, *statementTimeout)
	default:
		err = fmt.Errorf("unsupported migration direction %q", *direction)
	}
	if err != nil {
		log.Fatalf("migrate %s: %v", *direction, err)
	}
	log.Printf("migration direction=%s completed", *direction)
}

func durationFromEnvironment(name string, fallback time.Duration) time.Duration {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		log.Fatalf("parse %s: %v", name, err)
	}
	return parsed
}
