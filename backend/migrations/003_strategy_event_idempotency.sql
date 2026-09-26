ALTER TABLE strategy_events ADD COLUMN event_key TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS strategy_events_event_key_unique
  ON strategy_events(event_key);
