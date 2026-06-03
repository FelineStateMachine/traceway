ALTER TABLE endpoints ADD COLUMN distributed_trace_id TEXT DEFAULT NULL;
ALTER TABLE tasks ADD COLUMN distributed_trace_id TEXT DEFAULT NULL;
ALTER TABLE exception_stack_traces ADD COLUMN distributed_trace_id TEXT DEFAULT NULL;
