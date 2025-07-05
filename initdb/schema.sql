CREATE TABLE IF NOT EXISTS ads (
  id UUID PRIMARY KEY,
  video_url TEXT NOT NULL,
  target_url TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS click_events (
  id UUID PRIMARY KEY,
  ad_id UUID REFERENCES ads(id),
  timestamp TIMESTAMPTZ NOT NULL,
  ip_address TEXT,
  video_playback_time FLOAT
);

CREATE TABLE IF NOT EXISTS ad_analytics (
    ad_id UUID PRIMARY KEY,
    total_clicks INTEGER DEFAULT 0,
    unique_clicks INTEGER DEFAULT 0,
    impressions INTEGER DEFAULT 0,
    ctr FLOAT DEFAULT 0.0,
    updated_at TIMESTAMP DEFAULT NOW()
);
