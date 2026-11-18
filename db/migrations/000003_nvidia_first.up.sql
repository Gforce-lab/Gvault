-- NVIDIA-first defaults: catalog artifact backend + provider health projection

ALTER TABLE snapshots
    ALTER COLUMN artifact_backend SET DEFAULT 'nvidia-snapshot';

UPDATE provider_health
SET healthy = FALSE,
    version = 'v0.3.0',
    details_jsonb = '{"pin":"v0.3.0","note":"requires live kube + NVIDIA Snapshot operator"}'::jsonb,
    last_checked_at = NOW()
WHERE provider = 'fake';

INSERT INTO provider_health (provider, healthy, version, details_jsonb)
VALUES (
    'nvidia-snapshot',
    FALSE,
    'v0.3.0',
    '{"pin":"v0.3.0","phase":"1","amd":"out-of-scope"}'::jsonb
)
ON CONFLICT (provider) DO UPDATE
SET version = EXCLUDED.version,
    details_jsonb = EXCLUDED.details_jsonb,
    last_checked_at = NOW();
