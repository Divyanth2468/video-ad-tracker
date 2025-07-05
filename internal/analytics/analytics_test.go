package analytics

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestRedisAnalytics(t *testing.T) (*RedisAnalytics, *miniredis.Miniredis) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to start miniredis: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
	})

	return &RedisAnalytics{Client: rdb}, s
}

func TestIncrementTotal(t *testing.T) {
	ra, s := newTestRedisAnalytics(t)
	defer s.Close()

	err := ra.IncrementTotal("test-ad")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	val, err := s.Get("ad:clicks:total:test-ad")
	if err != nil {
		t.Errorf("Failed to get key from Redis: %v", err)
	}
	if val != "1" {
		t.Errorf("Expected click count to be 1, got %s", val)
	}

}

func TestAddUnique(t *testing.T) {
	ra, s := newTestRedisAnalytics(t)
	defer s.Close()

	err := ra.AddUnique("test-ad", "192.168.1.1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

}

func TestIncrementHourly(t *testing.T) {
	ra, s := newTestRedisAnalytics(t)
	defer s.Close()

	now := time.Date(2025, 7, 2, 18, 0, 0, 0, time.UTC)
	err := ra.IncrementHourly("test-ad", now)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	key := "ad:clicks:hourly:test-ad:20250702"
	if val := s.HGet(key, "18"); val != "1" {
		t.Errorf("Expected HGET %s 18 to be 1, got %s", key, val)
	}
}

func TestIncrementImpression(t *testing.T) {
	ra, s := newTestRedisAnalytics(t)
	defer s.Close()

	err := ra.IncrementImpression("test-ad")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	val, err := s.Get("ad:impressions:total:test-ad")
	if err != nil {
		t.Errorf("Failed to get key from Redis: %v", err)
	}
	if val != "1" {
		t.Errorf("Expected click count to be 1, got %s", val)
	}

}

func TestGetTotalImpressions(t *testing.T) {
	ra, s := newTestRedisAnalytics(t)
	defer s.Close()

	s.Set("ad:impressions:total:test-ad", "5")

	val, err := ra.GetTotalImpressions("test-ad")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if val != 5 {
		t.Errorf("Expected 5, got %d", val)
	}
}

func TestGetAnalytics(t *testing.T) {
	ra, s := newTestRedisAnalytics(t)
	defer s.Close()

	// Seed Redis with mock data
	s.Set("ad:clicks:total:test-ad", "10")
	s.PfAdd("ads:clicks:unique:test-ad", "ip1", "ip2", "ip3")
	s.Set("ad:impressions:total:test-ad", "20")
	s.HSet("ad:clicks:hourly:test-ad:20250702", "18", "5")

	// Run test
	result, err := ra.GetAnalytics("test-ad", "1h")
	if err != nil {
		t.Fatalf("Error getting analytics: %v", err)
	}

	if result["totalClicks"].(int) != 10 {
		t.Errorf("Expected totalClicks=10, got %v", result["totalClicks"])
	}
	if result["impressions"].(int) != 20 {
		t.Errorf("Expected impressions=20, got %v", result["impressions"])
	}
	if result["ctr"].(float64) != 0.5 {
		t.Errorf("Expected CTR=0.5, got %v", result["ctr"])
	}
}
