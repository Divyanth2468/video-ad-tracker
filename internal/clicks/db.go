package clicks

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

func InsertClickEvent(ctx context.Context, db *pgxpool.Pool, event ClickEvent) error {
	_, err := db.Exec(ctx,
		`INSERT INTO click_events (id, ad_id, timestamp, ip_address, video_playback_time)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (id) DO NOTHING;`,
		event.ID, event.AdID, event.Timestamp, event.IPAddress, event.VideoPlaybackTime)
	return err
}
