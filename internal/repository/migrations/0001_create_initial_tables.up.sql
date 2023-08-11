CREATE TABLE IF NOT EXISTS api_keys(
    id TEXT PRIMARY KEY,
    owner_user_id TEXT,
    key_hash TEXT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP NULL
);

CREATE TABLE IF NOT EXISTS apps (
    id TEXT PRIMARY KEY,
    owner_user_id TEXT,
    name TEXT,
    created_at TIMESTAMP,
    updated_at TIMESTAMP NULL
);

CREATE TABLE IF NOT EXISTS api_key_access(
    api_key_id TEXT references api_keys(id),
    app_id TEXT references apps(id)
);