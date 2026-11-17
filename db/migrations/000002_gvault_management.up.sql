-- gvault Milestone 5 management fields (additive; does not alter M1 columns)

ALTER TABLE snapshots
    ADD COLUMN IF NOT EXISTS retention_policy TEXT NOT NULL DEFAULT 'Retain',
    ADD COLUMN IF NOT EXISTS retention_hold BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS version_tag TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS snapshot_tags (
    snapshot_id TEXT NOT NULL REFERENCES snapshots (id) ON DELETE CASCADE,
    tag_key     TEXT NOT NULL,
    tag_value   TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (snapshot_id, tag_key)
);

CREATE INDEX IF NOT EXISTS snapshot_tags_key_idx ON snapshot_tags (tag_key, tag_value);

CREATE TABLE IF NOT EXISTS provider_health (
    provider        TEXT PRIMARY KEY,
    healthy         BOOLEAN NOT NULL DEFAULT FALSE,
    version         TEXT NOT NULL DEFAULT '',
    last_checked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    details_jsonb   JSONB NOT NULL DEFAULT '{}'::jsonb
);

INSERT INTO provider_health (provider, healthy, version, details_jsonb)
VALUES ('fake', TRUE, 'fake-0.1', '{"simulated":true}'::jsonb)
ON CONFLICT (provider) DO NOTHING;
