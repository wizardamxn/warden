-- Accounts, the library of each student's lectures, and one job per video and
-- template instead of one per video.

-- email is stored trimmed and in lower case, so it's compared that way.
-- template is what the student's next notes are made with.
CREATE TABLE users (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    email         text NOT NULL UNIQUE,
    password_hash text NOT NULL, -- argon2id, never the password
    template      text NOT NULL CHECK (template IN ('neet', 'general')),
    created_at    timestamptz NOT NULL DEFAULT now()
);

-- Only the SHA-256 of the cookie's token is kept, so a copy of this table
-- can't be used to log in.
CREATE TABLE sessions (
    token_hash text PRIMARY KEY,
    user_id    bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- A job is now one video made with one template. Jobs made so far used the
-- NEET prompts.
ALTER TABLE jobs ADD COLUMN template text NOT NULL DEFAULT 'neet' CHECK (template IN ('neet', 'general'));
ALTER TABLE jobs ALTER COLUMN template DROP DEFAULT;
ALTER TABLE jobs DROP CONSTRAINT jobs_video_id_key;
ALTER TABLE jobs ADD CONSTRAINT jobs_video_template_key UNIQUE (video_id, template);

-- The lectures a student added. added_at counts toward the daily limit, and
-- Revise reads it. Adding a lecture again keeps the first added_at.
CREATE TABLE library (
    user_id  bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    video_id text NOT NULL,
    template text NOT NULL,
    added_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, video_id, template),
    FOREIGN KEY (video_id, template) REFERENCES jobs (video_id, template)
);

-- As in 001: Supabase's own web API sees nothing, our connection is unaffected.
ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE library ENABLE ROW LEVEL SECURITY;
