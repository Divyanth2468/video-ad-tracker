package ads

import (
	"net/http"
	"time"

	"github.com/Divyanth2468/video-ad-tracker/internal/logs"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

func GetAdHandler(db *pgxpool.Pool) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		logger := logs.Logger.WithField("path", "/ads")

		rows, err := db.Query(c, "SELECT id, video_url, target_url FROM ads")
		if err != nil {
			logger.WithError(err).Error("Failed to query ads")
			adsRequestCounter.WithLabelValues("500").Inc()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch ads"})
			return
		}
		defer rows.Close()

		var ads []Ad
		for rows.Next() {
			var ad Ad
			if err := rows.Scan(&ad.ID, &ad.VideoURL, &ad.TargetURL); err != nil {
				logger.WithError(err).Warn("Failed to scan ad row")
				continue
			}
			ads = append(ads, ad)
		}

		duration := time.Since(start).Seconds()
		adsQueryDuration.Observe(duration)

		logger.WithField("count", len(ads)).Info("Fetched ads successfully")
		adsRequestCounter.WithLabelValues("200").Inc()
		c.JSON(http.StatusOK, ads)
	}
}
