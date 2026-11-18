ALTER TABLE snapshots
    ALTER COLUMN artifact_backend SET DEFAULT 'fake';

DELETE FROM provider_health WHERE provider = 'nvidia-snapshot';
