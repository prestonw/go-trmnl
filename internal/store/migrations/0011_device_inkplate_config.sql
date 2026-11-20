-- Add inkplate_config JSON column for device rendering preferences.
ALTER TABLE devices ADD COLUMN inkplate_config TEXT DEFAULT '{}';
