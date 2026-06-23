package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/go-sql-driver/mysql"
	"github.com/syuancheng/BPulse/server/internal/bprecord"
	"github.com/syuancheng/BPulse/server/internal/careplan"
	"github.com/syuancheng/BPulse/server/internal/config"
	"github.com/syuancheng/BPulse/server/internal/httpapi"
	"github.com/syuancheng/BPulse/server/internal/task"
	"github.com/syuancheng/BPulse/server/internal/user"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load configuration: %v", err)
	}
	db, err := sql.Open("mysql", cfg.MySQLDSN)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(cfg.DBMaxOpenConns)
	db.SetMaxIdleConns(cfg.DBMaxIdleConns)
	db.SetConnMaxLifetime(cfg.DBConnLifetime)
	pingCtx, cancelPing := context.WithTimeout(context.Background(), cfg.DBPingTimeout)
	if err := db.PingContext(pingCtx); err != nil {
		cancelPing()
		log.Fatalf("connect database: %v", err)
	}
	cancelPing()

	userService := user.NewService(user.NewMySQLRepository(db, cfg.DBQueryTimeout))
	keyring, err := bprecord.NewKeyringFromBase64(cfg.DataKeyVersion, cfg.DataKeyBase64)
	if err != nil {
		log.Fatalf("load data encryption key: %v", err)
	}
	carePlanRepository := careplan.NewMySQLRepository(db, cfg.DBQueryTimeout)
	carePlanService := careplan.NewService(carePlanRepository)
	taskService := task.NewService(carePlanRepository, task.NewMySQLRepository(db, cfg.DBQueryTimeout), task.RealClock{})
	bpRecordService := bprecord.NewService(bprecord.NewMySQLRepository(db, cfg.DBQueryTimeout), keyring, bprecord.RealClock{})

	server := &http.Server{
		Addr:              cfg.Address,
		Handler:           httpapi.NewRouter(httpapi.Dependencies{Environment: cfg.Environment, Users: userService, CarePlans: carePlanService, Tasks: taskService, BPRecords: bpRecordService}),
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-stop
		ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("shutdown HTTP server: %v", err)
		}
	}()

	log.Printf("starting HTTP server environment=%s address=%s", cfg.Environment, cfg.Address)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("serve HTTP: %v", err)
	}
}
