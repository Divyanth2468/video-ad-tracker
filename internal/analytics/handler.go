package analytics

import (
	"net/http"

	"github.com/Divyanth2468/video-ad-tracker/internal/logs"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func (ra *RedisAnalytics) GetAnalyticsHandler(c *gin.Context) {
	logger := logs.Logger.WithField("path", "/ads/analytics")

	adId := c.Query("adId")
	timeframe := c.Query("timeframe")

	if adId == "" || timeframe == "" {
		logger.Warn("Missing required query parameters")
		c.JSON(http.StatusBadRequest, gin.H{"error": "adId and timeframe are required"})
		return
	}

	data, err := ra.GetAnalytics(adId, timeframe)
	if err != nil {
		logger.WithError(err).Error("Failed to fetch analytics")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch analytics"})
		return
	}

	logger.WithFields(logrus.Fields{
		"adId":      adId,
		"timeframe": timeframe,
	}).Info("Fetched analytics successfully")

	c.JSON(http.StatusOK, data)
}
