package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"

	"shipr-backend/internal/caddy"
	"shipr-backend/internal/config"
	"shipr-backend/internal/crypto"
	"shipr-backend/internal/database"
	"shipr-backend/internal/docker"
	"shipr-backend/internal/tasks"
	"shipr-backend/internal/worker"
)

func main() {
	cfg := config.Load()
	log.Printf("[INFO] Starting Shipr Build & Runtime Worker Engine...")

	// 1. PostgreSQL connection
	ctx := context.Background()
	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("[FATAL] Could not connect to PostgreSQL: %v", err)
	}
	defer pool.Close()
	db := database.New(pool)

	// 2. Redis connection
	redisOpt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		redisOpt = &redis.Options{Addr: "localhost:6379"}
	}
	rdb := redis.NewClient(redisOpt)
	defer rdb.Close()

	// 3. Crypto & Clients
	encryptor, err := crypto.NewEncryptor(cfg.MasterKey)
	if err != nil {
		log.Fatalf("[FATAL] Failed to initialize AES encryptor: %v", err)
	}

	dockerCli, err := docker.NewClient(cfg.DockerNetwork)
	if err != nil {
		log.Printf("[WARN] Docker client warning: %v", err)
	}

	caddyCli := caddy.NewClient(cfg.CaddyAdminURL)

	// 4. Initialize Worker Engine
	buildWorker := worker.NewWorker(cfg, db, dockerCli, caddyCli, encryptor, rdb)

	// 5. Asynq Server setup
	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: redisOpt.Addr, Password: redisOpt.Password, DB: redisOpt.DB},
		asynq.Config{
			Concurrency: 5, // Process up to 5 concurrent builds
			Queues: map[string]int{
				"default": 1,
			},
		},
	)

	mux := asynq.NewServeMux()
	mux.HandleFunc(tasks.TypeDeployApplication, buildWorker.ProcessDeployTask)

	log.Printf("[INFO] Worker is listening for tasks on queue 'default'...")

	go func() {
		if err := srv.Run(mux); err != nil {
			log.Fatalf("[FATAL] Asynq worker server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Printf("[INFO] Shutting down Asynq Worker Engine...")
	srv.Shutdown()
	log.Printf("[INFO] Worker stopped.")
}
