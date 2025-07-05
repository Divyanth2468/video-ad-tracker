package clicks

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/Divyanth2468/video-ad-tracker/internal/analytics"
	"github.com/Divyanth2468/video-ad-tracker/internal/logs"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ClickHandler struct {
	DB    *pgxpool.Pool
	Redis *analytics.RedisAnalytics
}

var logger = logs.Logger

type RetryableClick struct {
	Event ClickEvent `json:"event"`
	Retry int        `json:"retry"`
}

func (h *ClickHandler) HandlerClick(c *gin.Context) {
	var event ClickEvent

	if err := c.ShouldBindJSON(&event); err != nil {
		logger.WithError(err).Warn("Invalid click payload")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	if event.AdID == "" || event.IPAddress == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing adId or ipAddress"})
		return
	}

	event.ID = uuid.New().String()
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	logger.WithFields(map[string]interface{}{
		"adId":      event.AdID,
		"ip":        event.IPAddress,
		"timestamp": event.Timestamp.Format(time.RFC3339),
		"id":        event.ID,
	}).Info("Received click event")

	wrapper := RetryableClick{
		Event: event,
		Retry: 0,
	}

	data, err := json.Marshal(wrapper)
	if err != nil {
		logger.WithError(err).WithField("adId", event.AdID).Error("Failed to serialize click event")
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to serialize click"})
		return
	}

	if err := h.Redis.Client.LPush(c.Request.Context(), "click_queue", data).Err(); err != nil {
		logger.WithError(err).WithField("adId", event.AdID).Error("Failed to push click event to Redis queue")
		FallbackToDisk(wrapper)
		c.JSON(http.StatusAccepted, gin.H{"message": "Queued via fallback"})
		return
	}

	logger.WithField("adId", event.AdID).Info("Click event pushed to Redis")

	c.JSON(http.StatusAccepted, gin.H{"message": "Click event queued"})
}

func FallbackToDisk(event RetryableClick) {
	f, err := os.OpenFile("fallback_clicks.jsonl", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		logger.WithError(err).Error("Failed to open fallback file")
		return
	}
	defer f.Close()
	logger.Printf("fall back file opened")
	data, err := json.Marshal(event)
	if err != nil {
		logger.WithError(err).Error("Failed to marshal click event for fallback")
		return
	}

	if _, err := f.Write(append(data, '\n')); err != nil {
		logger.WithError(err).Error("Failed to write click to fallback file")
	}
}
