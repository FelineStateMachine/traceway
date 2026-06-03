CREATE SEQUENCE IF NOT EXISTS seq_notification_channels_id START 1;
CREATE TABLE IF NOT EXISTS notification_channels (
    id BIGINT PRIMARY KEY DEFAULT nextval('seq_notification_channels_id'),
    project_id TEXT NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    channel_type TEXT NOT NULL DEFAULT '',
    config TEXT NOT NULL DEFAULT '{}',
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_by BIGINT,
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    updated_at TIMESTAMP NOT NULL DEFAULT now()
);

CREATE SEQUENCE IF NOT EXISTS seq_notification_rules_id START 1;
CREATE TABLE IF NOT EXISTS notification_rules (
    id BIGINT PRIMARY KEY DEFAULT nextval('seq_notification_rules_id'),
    project_id TEXT NOT NULL,
    channel_id BIGINT NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    rule_type TEXT NOT NULL DEFAULT '',
    config TEXT NOT NULL DEFAULT '{}',
    enabled BOOLEAN NOT NULL DEFAULT true,
    cooldown_minutes BIGINT NOT NULL DEFAULT 15,
    severity TEXT NOT NULL DEFAULT '',
    snoozed_until TIMESTAMP,
    created_by BIGINT,
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    updated_at TIMESTAMP NOT NULL DEFAULT now()
);

CREATE SEQUENCE IF NOT EXISTS seq_notification_history_id START 1;
CREATE TABLE IF NOT EXISTS notification_history (
    id BIGINT PRIMARY KEY DEFAULT nextval('seq_notification_history_id'),
    project_id TEXT NOT NULL,
    rule_id BIGINT,
    channel_id BIGINT,
    rule_type TEXT NOT NULL DEFAULT '',
    rule_name TEXT NOT NULL DEFAULT '',
    channel_name TEXT NOT NULL DEFAULT '',
    severity TEXT NOT NULL DEFAULT '',
    subject TEXT NOT NULL DEFAULT '',
    body TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'sent',
    error_message TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT now()
);
