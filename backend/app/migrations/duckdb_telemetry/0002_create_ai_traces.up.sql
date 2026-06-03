CREATE TABLE IF NOT EXISTS ai_traces (
    id TEXT NOT NULL,
    project_id TEXT NOT NULL,
    recorded_at TEXT NOT NULL,
    duration BIGINT NOT NULL DEFAULT 0,
    status_code BIGINT NOT NULL DEFAULT 0,
    model TEXT NOT NULL DEFAULT '',
    response_model TEXT NOT NULL DEFAULT '',
    provider TEXT NOT NULL DEFAULT '',
    operation TEXT NOT NULL DEFAULT '',
    input_tokens BIGINT NOT NULL DEFAULT 0,
    output_tokens BIGINT NOT NULL DEFAULT 0,
    total_tokens BIGINT NOT NULL DEFAULT 0,
    cached_tokens BIGINT NOT NULL DEFAULT 0,
    reasoning_tokens BIGINT NOT NULL DEFAULT 0,
    input_cost DOUBLE NOT NULL DEFAULT 0,
    output_cost DOUBLE NOT NULL DEFAULT 0,
    total_cost DOUBLE NOT NULL DEFAULT 0,
    trace_name TEXT NOT NULL DEFAULT '',
    user_id TEXT NOT NULL DEFAULT '',
    finish_reason TEXT NOT NULL DEFAULT '',
    server_name TEXT NOT NULL DEFAULT '',
    app_version TEXT NOT NULL DEFAULT '',
    storage_key TEXT NOT NULL DEFAULT '',
    attributes TEXT NOT NULL DEFAULT '{}'
);
CREATE INDEX IF NOT EXISTS idx_ai_traces_project_recorded ON ai_traces(project_id, recorded_at);
CREATE INDEX IF NOT EXISTS idx_ai_traces_project_trace_name ON ai_traces(project_id, trace_name);
