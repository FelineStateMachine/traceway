CREATE TABLE IF NOT EXISTS sessions (
    id TEXT NOT NULL PRIMARY KEY,
    project_id TEXT NOT NULL,
    started_at TEXT NOT NULL,
    ended_at TEXT,
    duration BIGINT NOT NULL DEFAULT 0,
    client_ip TEXT NOT NULL DEFAULT '',
    attributes TEXT NOT NULL DEFAULT '{}',
    app_version TEXT NOT NULL DEFAULT '',
    server_name TEXT NOT NULL DEFAULT '',
    distributed_trace_id TEXT DEFAULT NULL
);
CREATE INDEX IF NOT EXISTS idx_sessions_project_started ON sessions(project_id, started_at);

ALTER TABLE session_recordings ADD COLUMN session_id TEXT DEFAULT NULL;
ALTER TABLE session_recordings ADD COLUMN segment_index BIGINT DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_session_recordings_session ON session_recordings(session_id, segment_index);

ALTER TABLE exception_stack_traces ADD COLUMN session_id TEXT DEFAULT NULL;
CREATE INDEX IF NOT EXISTS idx_exceptions_session ON exception_stack_traces(project_id, session_id);
