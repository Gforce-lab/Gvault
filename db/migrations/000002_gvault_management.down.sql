DROP TABLE IF EXISTS provider_health;
DROP TABLE IF EXISTS snapshot_tags;
ALTER TABLE snapshots
    DROP COLUMN IF EXISTS retention_policy,
    DROP COLUMN IF EXISTS retention_hold,
    DROP COLUMN IF EXISTS version_tag;
