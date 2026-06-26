CREATE TABLE IF NOT EXISTS schema_migrations (
    version TEXT PRIMARY KEY,
    applied_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at DATETIME NOT NULL
);

INSERT OR IGNORE INTO settings (key, value, updated_at) VALUES
    ('app.name', 'LabPeek', datetime('now')),
    ('app.version', '0.1.0', datetime('now'));

CREATE TABLE IF NOT EXISTS assets (
    id TEXT PRIMARY KEY,
    display_name TEXT NOT NULL,
    discovered_name TEXT,
    canonical_name TEXT,
    asset_type TEXT NOT NULL DEFAULT 'unknown',
    status TEXT NOT NULL DEFAULT 'active',
    description TEXT,
    manufacturer TEXT,
    model TEXT,
    serial_number TEXT,
    os_name TEXT,
    os_version TEXT,
    firmware_version TEXT,
    primary_ip TEXT,
    primary_mac TEXT,
    mac_vendor TEXT,
    location TEXT,
    rack TEXT,
    role TEXT,
    environment TEXT,
    criticality TEXT,
    owner TEXT,
    notes TEXT,
    manual_data_json TEXT NOT NULL DEFAULT '{}',
    discovered_data_json TEXT NOT NULL DEFAULT '{}',
    confidence_score INTEGER NOT NULL DEFAULT 0,
    first_seen_at DATETIME,
    last_seen_at DATETIME,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_assets_status ON assets(status);
CREATE INDEX IF NOT EXISTS idx_assets_type ON assets(asset_type);
CREATE INDEX IF NOT EXISTS idx_assets_primary_ip ON assets(primary_ip);
CREATE INDEX IF NOT EXISTS idx_assets_primary_mac ON assets(primary_mac);
CREATE INDEX IF NOT EXISTS idx_assets_last_seen ON assets(last_seen_at);
CREATE INDEX IF NOT EXISTS idx_assets_display_name ON assets(display_name);

CREATE TABLE IF NOT EXISTS asset_identities (
    id TEXT PRIMARY KEY,
    asset_id TEXT NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    identity_type TEXT NOT NULL,
    identity_value TEXT NOT NULL,
    confidence INTEGER NOT NULL DEFAULT 0,
    first_seen_at DATETIME NOT NULL,
    last_seen_at DATETIME NOT NULL,
    source TEXT,
    UNIQUE(identity_type, identity_value)
);

CREATE INDEX IF NOT EXISTS idx_identities_asset ON asset_identities(asset_id);
CREATE INDEX IF NOT EXISTS idx_identities_lookup ON asset_identities(identity_type, identity_value);

CREATE TABLE IF NOT EXISTS services (
    id TEXT PRIMARY KEY,
    asset_id TEXT REFERENCES assets(id) ON DELETE SET NULL,
    display_name TEXT NOT NULL,
    discovered_name TEXT,
    ip_address TEXT NOT NULL,
    port INTEGER NOT NULL,
    protocol TEXT NOT NULL,
    transport TEXT NOT NULL DEFAULT 'tcp',
    service_name TEXT,
    product TEXT,
    version TEXT,
    url TEXT,
    http_title TEXT,
    tls_subject TEXT,
    tls_issuer TEXT,
    tls_not_before DATETIME,
    tls_not_after DATETIME,
    externally_exposed INTEGER NOT NULL DEFAULT 0,
    authentication_required INTEGER NOT NULL DEFAULT 0,
    criticality TEXT,
    tags_json TEXT NOT NULL DEFAULT '[]',
    notes TEXT,
    status TEXT NOT NULL DEFAULT 'active',
    confidence_score INTEGER NOT NULL DEFAULT 0,
    first_seen_at DATETIME,
    last_seen_at DATETIME,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    UNIQUE(ip_address, port, protocol, transport)
);

CREATE INDEX IF NOT EXISTS idx_services_asset ON services(asset_id);
CREATE INDEX IF NOT EXISTS idx_services_ip_port ON services(ip_address, port, protocol);
CREATE INDEX IF NOT EXISTS idx_services_status ON services(status);

CREATE TABLE IF NOT EXISTS interfaces (
    id TEXT PRIMARY KEY,
    asset_id TEXT NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    name TEXT,
    mac_address TEXT,
    ip_address TEXT,
    description TEXT,
    speed TEXT,
    discovered_data_json TEXT NOT NULL DEFAULT '{}',
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_interfaces_asset ON interfaces(asset_id);
CREATE INDEX IF NOT EXISTS idx_interfaces_mac ON interfaces(mac_address);

CREATE TABLE IF NOT EXISTS networks (
    id TEXT PRIMARY KEY,
    cidr TEXT NOT NULL UNIQUE,
    display_name TEXT,
    description TEXT,
    vlan_id INTEGER,
    location TEXT,
    notes TEXT,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS discovery_runs (
    id TEXT PRIMARY KEY,
    profile TEXT NOT NULL,
    target TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'queued',
    started_at DATETIME,
    completed_at DATETIME,
    current_phase TEXT,
    progress_percent INTEGER NOT NULL DEFAULT 0,
    hosts_found INTEGER NOT NULL DEFAULT 0,
    services_found INTEGER NOT NULL DEFAULT 0,
    observations_count INTEGER NOT NULL DEFAULT 0,
    error TEXT,
    logs TEXT NOT NULL DEFAULT '',
    raw_output_path TEXT,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_runs_status ON discovery_runs(status);
CREATE INDEX IF NOT EXISTS idx_runs_created ON discovery_runs(created_at DESC);

CREATE TABLE IF NOT EXISTS discovery_observations (
    id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL REFERENCES discovery_runs(id) ON DELETE CASCADE,
    source TEXT NOT NULL,
    observed_at DATETIME NOT NULL,
    ip_address TEXT,
    mac_address TEXT,
    hostname TEXT,
    service_port INTEGER,
    service_protocol TEXT,
    raw_json TEXT NOT NULL DEFAULT '{}',
    matched_asset_id TEXT REFERENCES assets(id) ON DELETE SET NULL,
    confidence INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_obs_run ON discovery_observations(run_id);
CREATE INDEX IF NOT EXISTS idx_obs_ip ON discovery_observations(ip_address);
CREATE INDEX IF NOT EXISTS idx_obs_mac ON discovery_observations(mac_address);

CREATE TABLE IF NOT EXISTS suggestions (
    id TEXT PRIMARY KEY,
    suggestion_type TEXT NOT NULL,
    asset_id TEXT REFERENCES assets(id) ON DELETE CASCADE,
    service_id TEXT REFERENCES services(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    description TEXT,
    proposed_change_json TEXT NOT NULL DEFAULT '{}',
    status TEXT NOT NULL DEFAULT 'open',
    created_at DATETIME NOT NULL,
    resolved_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_suggestions_status ON suggestions(status);

CREATE TABLE IF NOT EXISTS relationships (
    id TEXT PRIMARY KEY,
    source_asset_id TEXT NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    target_asset_id TEXT NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    relationship_type TEXT NOT NULL,
    description TEXT,
    confidence INTEGER NOT NULL DEFAULT 0,
    discovered INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS changes (
    id TEXT PRIMARY KEY,
    entity_type TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    change_type TEXT NOT NULL,
    old_value_json TEXT,
    new_value_json TEXT,
    source TEXT NOT NULL,
    created_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_changes_entity ON changes(entity_type, entity_id);
CREATE INDEX IF NOT EXISTS idx_changes_created ON changes(created_at DESC);

CREATE TABLE IF NOT EXISTS tags (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    colour TEXT,
    created_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS asset_tags (
    asset_id TEXT NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
    tag_id TEXT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY(asset_id, tag_id)
);
