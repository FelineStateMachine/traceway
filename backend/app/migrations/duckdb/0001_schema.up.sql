CREATE TABLE IF NOT EXISTS projects (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    token TEXT NOT NULL,
    framework TEXT NOT NULL DEFAULT 'custom',
    organization_id BIGINT,
    source_map_token TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_projects_token ON projects(token);
CREATE INDEX IF NOT EXISTS idx_projects_organization_id ON projects(organization_id);

CREATE SEQUENCE IF NOT EXISTS seq_users_id START 1;
CREATE TABLE IF NOT EXISTS users (
    id BIGINT PRIMARY KEY DEFAULT nextval('seq_users_id'),
    email TEXT NOT NULL,
    name TEXT NOT NULL,
    password TEXT NOT NULL,
    password_reset_token TEXT,
    password_reset_expires_at TIMESTAMP,
    password_reset_requested_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users(email);

CREATE SEQUENCE IF NOT EXISTS seq_organizations_id START 1;
CREATE TABLE IF NOT EXISTS organizations (
    id BIGINT PRIMARY KEY DEFAULT nextval('seq_organizations_id'),
    name TEXT NOT NULL,
    timezone TEXT NOT NULL DEFAULT 'UTC',
    created_at TIMESTAMP DEFAULT now()
);

CREATE SEQUENCE IF NOT EXISTS seq_organization_users_id START 1;
CREATE TABLE IF NOT EXISTS organization_users (
    id BIGINT PRIMARY KEY DEFAULT nextval('seq_organization_users_id'),
    user_id BIGINT NOT NULL,
    organization_id BIGINT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('owner','admin','user','readonly')),
    created_at TIMESTAMP DEFAULT now(),
    UNIQUE(user_id, organization_id)
);

CREATE SEQUENCE IF NOT EXISTS seq_invitations_id START 1;
CREATE TABLE IF NOT EXISTS invitations (
    id BIGINT PRIMARY KEY DEFAULT nextval('seq_invitations_id'),
    organization_id BIGINT NOT NULL,
    email TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('admin','user','readonly')),
    token TEXT NOT NULL UNIQUE,
    invited_by BIGINT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','accepted','expired')),
    expires_at TIMESTAMP NOT NULL,
    accepted_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_invitations_email_org_pending ON invitations(email, organization_id);

CREATE SEQUENCE IF NOT EXISTS seq_source_maps_id START 1;
CREATE TABLE IF NOT EXISTS source_maps (
    id BIGINT PRIMARY KEY DEFAULT nextval('seq_source_maps_id'),
    project_id TEXT NOT NULL,
    version TEXT NOT NULL,
    file_name TEXT NOT NULL,
    storage_key TEXT NOT NULL,
    file_size BIGINT NOT NULL,
    uploaded_at TIMESTAMP NOT NULL DEFAULT now(),
    UNIQUE(project_id, version, file_name)
);

CREATE INDEX IF NOT EXISTS idx_source_maps_project_version ON source_maps(project_id, version);

CREATE SEQUENCE IF NOT EXISTS seq_metric_registry_id START 1;
CREATE TABLE IF NOT EXISTS metric_registry (
    id BIGINT PRIMARY KEY DEFAULT nextval('seq_metric_registry_id'),
    project_id TEXT NOT NULL,
    name TEXT NOT NULL,
    metric_type TEXT NOT NULL DEFAULT 'gauge',
    unit TEXT DEFAULT '',
    description TEXT DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    UNIQUE(project_id, name)
);

CREATE SEQUENCE IF NOT EXISTS seq_widget_groups_id START 1;
CREATE TABLE IF NOT EXISTS widget_groups (
    id BIGINT PRIMARY KEY DEFAULT nextval('seq_widget_groups_id'),
    project_id TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT DEFAULT '',
    is_default BOOLEAN NOT NULL DEFAULT false,
    created_by BIGINT,
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    updated_at TIMESTAMP NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_widget_groups_project_id ON widget_groups(project_id);

CREATE SEQUENCE IF NOT EXISTS seq_widget_group_widgets_id START 1;
CREATE TABLE IF NOT EXISTS widget_group_widgets (
    id BIGINT PRIMARY KEY DEFAULT nextval('seq_widget_group_widgets_id'),
    widget_group_id BIGINT NOT NULL,
    title TEXT NOT NULL,
    widget_type TEXT NOT NULL,
    config TEXT NOT NULL DEFAULT '{}',
    position BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    updated_at TIMESTAMP NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_widget_group_widgets_widget_group_id ON widget_group_widgets(widget_group_id);
