package worker

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"

	"github.com/Divyanth2468/video-ad-tracker/internal/analytics"
	"github.com/Divyanth2468/video-ad-tracker/internal/clicks"
	"github.com/Divyanth2468/video-ad-tracker/internal/logs"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

var logger = logs.Logger

func StartQueueWorker(
	ctx context.Context,
	rdb *redis.Client,
	db *pgxpool.Pool,
	analytics *analytics.RedisAnalytics,
	wg *sync.WaitGroup,
	workerCount int,
) {
	// Periodic analytics sync goroutine
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				logger.Info("Analytics sync stopped due to context cancellation")
				return
			case <-ticker.C:
				err := SyncRedisAnalyticsToPostgres(analytics, db)
				if err != nil {
					logger.WithError(err).Error("Periodic sync to Postgres failed")
				}
			}
		}
	}()

	// Periodic flush of fallback disk to Redis
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				logger.Info("Fallback flush stopped due to context cancellation")
				return
			case <-ticker.C:
				flushFallbackToRedis(ctx, rdb)
			}
		}
	}()

	// Start queue workers
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			log.Printf("[Worker %d] Started", workerID)

			for {
				select {
				case <-ctx.Done():
					log.Printf("[Worker %d] Shutdown signal received. Exiting...", workerID)
					return
				default:
					if err := ensureRedisConnected(rdb); err != nil {
						log.Printf("[Worker %d] Redis not reachable: %v", workerID, err)
						time.Sleep(3 * time.Second)
						continue
					}

					redisCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
					data, err := rdb.RPopLPush(redisCtx, "click_queue", "click_processing").Result()
					cancel()

					if err != nil {
						if err != redis.Nil {
							log.Printf("[Worker %d] RPOPLPUSH error: %v", workerID, err)
						}
						time.Sleep(1 * time.Second)
						continue
					}

					var wrapper clicks.RetryableClick
					if err := json.Unmarshal([]byte(data), &wrapper); err != nil {
						log.Printf("[Worker %d] JSON unmarshal failed: %v. Discarding.", workerID, err)
						_ = rdb.LRem(ctx, "click_processing", 0, data)
						continue
					}

					err = clicks.InsertClickEvent(ctx, db, wrapper.Event)
					if err != nil {
						log.Printf("[Worker %d] DB insert failed for AdID %s: %v", workerID, wrapper.Event.AdID, err)
						wrapper.Retry++
						if wrapper.Retry >= 3 {
							dlqData, _ := json.Marshal(wrapper)
							_ = rdb.LPush(ctx, "click_dead", dlqData)
						} else {
							retryData, _ := json.Marshal(wrapper)
							_ = rdb.LPush(ctx, "click_queue", retryData)
						}
						_ = rdb.LRem(ctx, "click_processing", 0, data)
						time.Sleep(2 * time.Second)
						continue
					}

					if err := analytics.IncrementTotal(wrapper.Event.AdID); err != nil {
						log.Printf("[Worker %d] IncrementTotal failed: %v", workerID, err)
					}
					if err := analytics.AddUnique(wrapper.Event.AdID, wrapper.Event.IPAddress); err != nil {
						log.Printf("[Worker %d] AddUnique failed: %v", workerID, err)
					}
					if err := analytics.IncrementHourly(wrapper.Event.AdID, wrapper.Event.Timestamp); err != nil {
						log.Printf("[Worker %d] IncrementHourly failed: %v", workerID, err)
					}

					_ = rdb.LRem(ctx, "click_processing", 0, data)
				}
			}
		}(i)
	}
}

func ensureRedisConnected(rdb *redis.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return rdb.Ping(ctx).Err()
}

func flushFallbackToRedis(ctx context.Context, rdb *redis.Client) {
	file, err := os.Open("fallback_clicks.jsonl")
	if err != nil {
		if !os.IsNotExist(err) {
			logger.WithError(err).Error("Failed to open fallback file")
		}
		return
	}
	defer file.Close()

	scanner := json.NewDecoder(file)
	var unprocessed []clicks.RetryableClick

	for scanner.More() {
		var wrapper clicks.RetryableClick
		if err := scanner.Decode(&wrapper); err != nil {
			logger.WithError(err).Error("Failed to decode fallback event")
			continue // malformed line, skip
		}

		data, _ := json.Marshal(wrapper)
		if err := rdb.LPush(ctx, "click_queue", data).Err(); err != nil {
			logger.WithError(err).Error("Failed to requeue fallback event")
			unprocessed = append(unprocessed, wrapper) // keep for retry
		}
	}

	file.Close()

	// If some events couldn't be re-queued, rewrite them
	if len(unprocessed) > 0 {
		f, err := os.Create("fallback_clicks.jsonl")
		if err != nil {
			logger.WithError(err).Error("Failed to rewrite fallback file")
			return
		}
		defer f.Close()

		enc := json.NewEncoder(f)
		for _, ev := range unprocessed {
			if err := enc.Encode(ev); err != nil {
				logger.WithError(err).Error("Failed to encode event while rewriting fallback file")
			}
		}
	} else {
		// All events successfully pushed, delete the file
		if err := os.Remove("fallback_clicks.jsonl"); err != nil {
			logger.WithError(err).Error("Failed to delete fallback file")
		}
	}
}

func SyncRedisAnalyticsToPostgres(redis *analytics.RedisAnalytics, db *pgxpool.Pool) error {
	adIDs := []string{
		"11111111-1111-1111-1111-111111111111",
		"22222222-2222-2222-2222-222222222222",
		"33333333-3333-3333-3333-333333333333",
	}

	for _, adID := range adIDs {
		data, err := redis.GetAnalytics(adID, "1h")
		if err != nil {
			logger.WithField("adID", adID).WithError(err).Error("Failed to fetch analytics for sync")
			continue
		}

		_, err = db.Exec(context.Background(), `
			INSERT INTO ad_analytics (ad_id, total_clicks, unique_clicks, impressions, ctr, updated_at)
			VALUES ($1, $2, $3, $4, $5, NOW())
			ON CONFLICT (ad_id) DO UPDATE SET
				total_clicks = EXCLUDED.total_clicks,
				unique_clicks = EXCLUDED.unique_clicks,
				impressions = EXCLUDED.impressions,
				ctr = EXCLUDED.ctr,
				updated_at = NOW()
		`, adID, data["totalClicks"], data["uniqueClicks"], data["impressions"], data["ctr"])

		if err != nil {
			logger.WithField("adID", adID).WithError(err).Error("Failed to sync analytics to DB")
			continue
		}
		logger.WithField("adID", adID).Info("Synced analytics to DB")
	}

	return nil
}
