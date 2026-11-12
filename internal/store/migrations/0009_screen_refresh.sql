-- Per-screen refresh cadence in seconds. NULL means "use the plugin's
-- default refresh interval" (the pre-existing behavior). A non-NULL value
-- makes the screen "live": the server re-renders it at that cadence and the
-- device holds on it (no playlist advance) while polling at the cadence.
ALTER TABLE screens ADD COLUMN refresh_seconds INTEGER;
