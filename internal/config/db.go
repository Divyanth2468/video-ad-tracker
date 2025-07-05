package config

import (
	"context"
	"os"
	"time"

	"github.com/Divyanth2468/video-ad-tracker/internal/logs"
	"github.com/jackc/pgx/v5/pgxpool"
)

var DB *pgxpool.Pool
var logger = logs.Logger

func InitDB() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		logger.Fatal("DATABASE_URL not set")
	}

	var pool *pgxpool.Pool
	var err error

	maxRetries := 10
	for i := 0; i < maxRetries; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		pool, err = pgxpool.New(ctx, dbURL)
		if err != nil {
			logger.WithError(err).Errorf("Attempt %d: Failed to create pool", i+1)
			time.Sleep(time.Second * time.Duration(i+1))
			continue
		}

		err = pool.Ping(ctx)
		if err == nil {
			DB = pool
			logger.Info("Connected to database")
			return
		}

		logger.WithError(err).Errorf("Attempt %d: Database ping failed", i+1)
		time.Sleep(time.Second * time.Duration(i+1))
	}

	logger.WithError(err).Fatal("Exceeded max retries: Unable to connect to database")
}
