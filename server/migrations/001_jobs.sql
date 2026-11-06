-- One job per YouTube video, so a second request for a lecture finds the first.
-- A job is held by one worker until locked_until. A worker that dies stops
-- renewing it, and the job goes back to free when the time passes. active_at is
-- the last sign of life, so a job nobody is working on can be ended.
CREATE TABLE jobs (
    id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    video_id     text NOT NULL UNIQUE,
    locked_by    text,
    locked_until timestamptz,
    active_at    timestamptz NOT NULL DEFAULT now(),
    created_at   timestamptz NOT NULL DEFAULT now()
);

-- One row per step of a job. prompt_version says what made the result, so a new
-- prompt runs only that step again.
CREATE TABLE job_steps (
    job_id         bigint NOT NULL REFERENCES jobs (id) ON DELETE CASCADE,
    name           text NOT NULL CHECK (name IN ('captions', 'notes', 'flashcards', 'quiz')),
    prompt_version text NOT NULL,
    status         text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'done', 'failed')),
    attempts       integer NOT NULL DEFAULT 0, -- also proves a running step still belongs to its worker
    result         jsonb,
    error          text NOT NULL DEFAULT '', -- a plain sentence for students, never a raw error
    input_tokens   integer NOT NULL DEFAULT 0, -- tokens and cost add up over all attempts
    output_tokens  integer NOT NULL DEFAULT 0,
    cost_usd       numeric(12, 6) NOT NULL DEFAULT 0,
    started_at     timestamptz,
    finished_at    timestamptz,
    PRIMARY KEY (job_id, name),
    CONSTRAINT done_has_result CHECK (status <> 'done' OR result IS NOT NULL)
);

-- Finding work only looks at unfinished steps, which are few.
CREATE INDEX job_steps_unfinished ON job_steps (job_id) WHERE status IN ('pending', 'running');

-- Supabase also serves the public schema over its own web API. With row level
-- security on and no policies, that API sees nothing, and our own connection,
-- which owns the tables, is not affected.
ALTER TABLE jobs ENABLE ROW LEVEL SECURITY;
ALTER TABLE job_steps ENABLE ROW LEVEL SECURITY;
