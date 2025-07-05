package clicks

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Divyanth2468/video-ad-tracker/internal/analytics"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func setupRouter(handler *ClickHandler) *gin.Engine {
	r := gin.Default()
	r.POST("/ads/click", handler.HandlerClick)
	return r
}

func TestHandlerClick_ValidInput(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Mock Redis client that always succeeds
	mockRedis := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	handler := &ClickHandler{
		Redis: &analytics.RedisAnalytics{Client: mockRedis},
	}

	router := setupRouter(handler)

	click := ClickEvent{
		AdID:              "11111111-1111-1111-1111-111111111111",
		Timestamp:         time.Now(),
		IPAddress:         "127.0.0.1",
		VideoPlaybackTime: 10.5,
	}

	body, _ := json.Marshal(click)
	req, _ := http.NewRequest(http.MethodPost, "/ads/click", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
}

func TestHandlerClick_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := &ClickHandler{
		Redis: &analytics.RedisAnalytics{Client: redis.NewClient(&redis.Options{
			Addr: "localhost:6379",
		})},
	}

	router := setupRouter(handler)

	req, _ := http.NewRequest(http.MethodPost, "/ads/click", bytes.NewBuffer([]byte(`invalid-json`)))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
