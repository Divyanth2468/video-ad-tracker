package clicks

import "time"

type ClickEvent struct {
	ID                string    `json:"id"`
	AdID              string    `json:"adId"`
	Timestamp         time.Time `json:"timestamp"`
	IPAddress         string    `json:"ipAddress"`
	VideoPlaybackTime float64   `json:"videoPlaybackTime"`
}
