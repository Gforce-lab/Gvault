-- GPU Snapshot Vault — initial catalog schema (Milestone 1)

CREATE TABLE IF NOT EXISTS snapshots (
    id                      TEXT PRIMARY KEY,
    name                    TEXT NOT NULL,
    cluster_id              TEXT NOT NULL,
    namespace               TEXT NOT NULL,
    provider                TEXT NOT NULL,
    status                  TEXT NOT NULL,
    source_pod_name         TEXT NOT NULL,
    source_pod_uid          TEXT NOT NULL DEFAULT '',
    source_container_name   TEXT NOT NULL,
    provider_snapshot_name  TEXT NOT NULL DEFAULT '',
    provider_snapshot_uid   TEXT NOT NULL DEFAULT '',
    provider_content_name   TEXT NOT NULL DEFAULT '',
    provider_version        TEXT NOT NULL DEFAULT '',
    model_name              TEXT NOT NULL DEFAULT '',
    model_revision          TEXT NOT NULL DEFAULT '',
    framework               TEXT NOT NULL DEFAULT '',
    framework_version       TEXT NOT NULL DEFAULT '',
    image_digest            TEXT NOT NULL DEFAULT '',
    gpu_product             TEXT NOT NULL DEFAULT '',
    gpu_count               INTEGER NOT NULL DEFAULT 1,
    driver_version          TEXT NOT NULL DEFAULT '',
    architecture            TEXT NOT NULL DEFAULT 'amd64',
    kernel_version          TEXT NOT NULL DEFAULT '',
    artifact_backend        TEXT NOT NULL DEFAULT 'fake',
    artifact_ref_jsonb      JSONB NOT NULL DEFAULT '{}'::jsonb,
    size_bytes              BIGINT NULL,
    labels_jsonb            JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ready_at                TIMESTAMPTZ NULL,
    deleted_at              TIMESTAMPTZ NULL,
    resource_version        BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT snapshots_status_check CHECK (
        status IN ('PENDING', 'CAPTURING', 'READY', 'FAILED', 'DELETING', 'DELETED')
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS snapshots_cluster_ns_name_uidx
    ON snapshots (cluster_id, namespace, name)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS snapshots_status_gpu_idx
    ON snapshots (status, gpu_product);

CREATE INDEX IF NOT EXISTS snapshots_provider_binding_idx
    ON snapshots (provider, provider_snapshot_name, provider_snapshot_uid);

CREATE TABLE IF NOT EXISTS operations (
    id                TEXT PRIMARY KEY,
    type              TEXT NOT NULL,
    snapshot_id       TEXT NOT NULL REFERENCES snapshots (id),
    cluster_id        TEXT NOT NULL,
    namespace         TEXT NOT NULL,
    status            TEXT NOT NULL,
    requested_by      TEXT NOT NULL DEFAULT '',
    request_id        TEXT NOT NULL DEFAULT '',
    idempotency_key   TEXT NOT NULL DEFAULT '',
    target_pod_name   TEXT NULL,
    target_node_name  TEXT NULL,
    error_code        TEXT NULL,
    error_message     TEXT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at       TIMESTAMPTZ NULL,
    CONSTRAINT operations_type_check CHECK (
        type IN ('CAPTURE', 'RESTORE', 'DELETE')
    ),
    CONSTRAINT operations_status_check CHECK (
        status IN ('PENDING', 'RUNNING', 'SUCCEEDED', 'FAILED', 'CANCELLED')
    )
);

CREATE UNIQUE INDEX IF NOT EXISTS operations_idempotency_uidx
    ON operations (cluster_id, namespace, type, idempotency_key)
    WHERE idempotency_key <> '';

CREATE INDEX IF NOT EXISTS operations_snapshot_idx
    ON operations (snapshot_id, created_at DESC);

CREATE TABLE IF NOT EXISTS audit_events (
    id              TEXT PRIMARY KEY,
    actor           TEXT NOT NULL,
    action          TEXT NOT NULL,
    resource_type   TEXT NOT NULL,
    resource_id     TEXT NOT NULL,
    decision        TEXT NOT NULL,
    request_id      TEXT NOT NULL DEFAULT '',
    timestamp       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    attributes_jsonb JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS audit_events_resource_idx
    ON audit_events (resource_type, resource_id, timestamp DESC);
