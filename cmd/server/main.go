package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/Divyanth2468/video-ad-tracker/internal/ads"
	"github.com/Divyanth2468/video-ad-tracker/internal/analytics"
	"github.com/Divyanth2468/video-ad-tracker/internal/clicks"
	"github.com/Divyanth2468/video-ad-tracker/internal/config"
	logging "github.com/Divyanth2468/video-ad-tracker/internal/logs"
	"github.com/Divyanth2468/video-ad-tracker/internal/worker"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP requests",
		},
		[]string{"method", "path", "status"},
	)
	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"path"},
	)
)

func PrometheusMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		duration := time.Since(start)

		status := fmt.Sprintf("%d", c.Writer.Status())
		httpRequestsTotal.WithLabelValues(c.Request.Method, c.FullPath(), status).Inc()
		httpRequestDuration.WithLabelValues(c.FullPath()).Observe(duration.Seconds())
	}
}

func main() {
	logging.InitLogger()
	logger := logging.Logger
	_ = godotenv.Load(".env")

	config.InitDB()
	defer config.DB.Close()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	redisAddr := os.Getenv("REDIS_ADDR")
	redisPassword := os.Getenv("REDIS_PASSWORD")
	redisDBStr := os.Getenv("REDIS_DB")
	workerCountStr := os.Getenv("WORKER_COUNT")

	if redisAddr == "" {
		logger.Fatal("REDIS_ADDR is required")
	}
	redisDB, err := strconv.Atoi(redisDBStr)
	if err != nil {
		logger.Fatal("Invalid REDIS_DB value")
	}
	workerCount, err := strconv.Atoi(workerCountStr)
	if err != nil {
		logger.Warn("Invalid WORKER_COUNT. Defaulting to 4")
		workerCount = 4
	}

	redisClient := analytics.NewRedisAnalytics(redisAddr, redisPassword, redisDB)

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	worker.StartQueueWorker(ctx, redisClient.Client, config.DB, redisClient, &wg, workerCount)

	prometheus.MustRegister(httpRequestsTotal, httpRequestDuration)
	ads.InitAdMetrics()
	if err := prometheus.Register(collectors.NewGoCollector()); err != nil {
		if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
			log.Fatalf("could not register Go collector: %v", err)
		}
	}

	r := gin.Default()
	r.Use(PrometheusMiddleware())
	r.Static("/assets", "./web/assets")
	r.LoadHTMLFiles("web/index.html")

	r.GET("/", func(c *gin.Context) {
		c.HTML(200, "index.html", nil)
	})

	r.GET("/ads", ads.GetAdHandler(config.DB))
	r.POST("/ads/impression", func(c *gin.Context) {
		var payload struct {
			AdID string `json:"ad_id" binding:"required"`
		}
		if err := c.ShouldBindJSON(&payload); err != nil {
			c.JSON(400, gin.H{"error": "Missing or invalid ad_id"})
			return
		}
		if err := redisClient.IncrementImpression(payload.AdID); err != nil {
			c.JSON(500, gin.H{"error": "Failed to record impression"})
			return
		}
		c.Status(204)
	})

	clickHandler := &clicks.ClickHandler{DB: config.DB, Redis: redisClient}
	r.POST("/ads/click", clickHandler.HandlerClick)
	r.GET("/ads/analytics", redisClient.GetAnalyticsHandler)
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	go func() {
		logger.WithField("port", port).Info("Server starting")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.WithError(err).Fatal("Server failed to start")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Graceful shutdown initiated...")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.WithError(err).Fatal("Server forced to shutdown")
	}

	wg.Wait()

	if err := redisClient.Client.Close(); err != nil {
		logger.WithError(err).Error("Error closing Redis client")
	}

	logger.Info("Server shutdown complete")
}
