package analytics

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/Divyanth2468/video-ad-tracker/internal/logs"
	"github.com/redis/go-redis/v9"
)

var (
	ctx    = context.Background()
	logger = logs.Logger
)

type RedisAnalytics struct {
	Client *redis.Client
}

func NewRedisAnalyticsFromClient(rdb *redis.Client) *RedisAnalytics {
	return &RedisAnalytics{Client: rdb}
}

func NewRedisAnalytics(addr, password string, db int) *RedisAnalytics {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	logger.WithFields(map[string]interface{}{
		"redisAddr": addr,
		"db":        db,
	}).Info("Initialized Redis client")

	return &RedisAnalytics{Client: rdb}
}

// Increment total clicks
func (ra *RedisAnalytics) IncrementTotal(adId string) error {
	key := "ad:clicks:total:" + adId
	err := ra.Client.Incr(ctx, key).Err()
	if err != nil {
		logger.WithField("key", key).WithError(err).Error("Failed to increment total clicks")
	}
	return err
}

// Add unique IP to HyperLogLog
func (ra *RedisAnalytics) AddUnique(adId, ip string) error {
	key := "ads:clicks:unique:" + adId
	err := ra.Client.PFAdd(ctx, key, ip).Err()
	if err != nil {
		logger.WithFields(map[string]interface{}{"key": key, "ip": ip}).WithError(err).Error("Failed to add unique click")
	}
	return err
}

// Increment hourly click count
func (ra *RedisAnalytics) IncrementHourly(adId string, t time.Time) error {
	key := "ad:clicks:hourly:" + adId + ":" + t.Format("20060102")
	hour := t.Format("15")
	err := ra.Client.HIncrBy(ctx, key, hour, 1).Err()
	if err != nil {
		logger.WithFields(map[string]interface{}{"key": key, "hour": hour}).WithError(err).Error("Failed to increment hourly clicks")
	}
	return err
}

// Increment impression count
func (ra *RedisAnalytics) IncrementImpression(adId string) error {
	key := "ad:impressions:total:" + adId
	err := ra.Client.Incr(ctx, key).Err()
	if err != nil {
		logger.WithField("key", key).WithError(err).Error("Failed to increment impressions")
	}
	return err
}

// Get total impressions
func (ra *RedisAnalytics) GetTotalImpressions(adId string) (int, error) {
	key := "ad:impressions:total:" + adId
	impressions, err := ra.Client.Get(ctx, key).Int()
	if err != nil && err != redis.Nil {
		logger.WithField("key", key).WithError(err).Error("Failed to get impressions")
		return 0, err
	}
	return impressions, nil
}

// GetAnalytics returns aggregated metrics
func (ra *RedisAnalytics) GetAnalytics(adId, timeframe string) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	// Total clicks
	totalClicks, err := ra.Client.Get(ctx, "ad:clicks:total:"+adId).Int()
	if err != nil && err != redis.Nil {
		logger.WithError(err).Error("Failed to get total clicks")
		return nil, err
	}
	result["totalClicks"] = totalClicks

	// Unique clicks
	uniqueClicks, err := ra.Client.PFCount(ctx, "ads:clicks:unique:"+adId).Result()
	if err != nil && err != redis.Nil {
		logger.WithError(err).Error("Failed to get unique clicks")
		return nil, err
	}
	result["uniqueClicks"] = uniqueClicks

	// Hourly clicks
	hourlyClicks := make(map[string]int)
	now := time.Now()
	var days int

	switch timeframe {
	case "1h", "24h":
		days = 1
	case "7d":
		days = 7
	default:
		days = 30
	}

	for i := 0; i < days; i++ {
		day := now.AddDate(0, 0, -i).Format("20060102")
		key := fmt.Sprintf("ad:clicks:hourly:%s:%s", adId, day)
		hmap, err := ra.Client.HGetAll(ctx, key).Result()
		if err != nil && err != redis.Nil {
			logger.WithField("key", key).WithError(err).Error("Failed to get hourly clicks")
			return nil, err
		}
		for hour, val := range hmap {
			if count, err := strconv.Atoi(val); err == nil {
				hourlyClicks[hour] += count
			}
		}
	}
	result["hourlyClicks"] = hourlyClicks

	// Get impressions
	impressions, err := ra.GetTotalImpressions(adId)
	if err != nil {
		return nil, err
	}
	result["impressions"] = impressions

	var ctr float64
	// Calculate CTR
	if impressions > 0 {
		ctr = float64(totalClicks) / float64(impressions)
	} else {
		ctr = 0.0
	}

	result["ctr"] = ctr

	logger.WithFields(map[string]interface{}{
		"adId":        adId,
		"timeframe":   timeframe,
		"totalClicks": totalClicks,
		"unique":      uniqueClicks,
		"ctr":         ctr,
	}).Info("Fetched analytics")

	return result, nil
}

func (ra *RedisAnalytics) CloseRedis() error {
	if ra.Client != nil {
		return ra.Client.Close()
	}
	return nil
}
