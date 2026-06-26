INSERT OR IGNORE INTO settings (key, value, updated_at) VALUES
    ('ui.network_label', 'Lab network(default)', datetime('now')),
    ('discovery.default_target', '192.168.1.0/24', datetime('now'));
