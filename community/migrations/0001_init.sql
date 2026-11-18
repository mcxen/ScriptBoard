-- Users. Passwords are stored as PBKDF2-SHA256 hashes with a random salt.
CREATE TABLE IF NOT EXISTS users (
  id TEXT PRIMARY KEY,
  username TEXT NOT NULL UNIQUE,
  pass_hash TEXT NOT NULL,
  pass_salt TEXT NOT NULL,
  created_at INTEGER NOT NULL
);

-- Login sessions. Only a SHA-256 hash of each token is stored.
CREATE TABLE IF NOT EXISTS sessions (
  token_hash TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id),
  expires_at INTEGER NOT NULL
);

-- Uploaded scripts. Only status = 'approved' is visible to the public.
CREATE TABLE IF NOT EXISTS scripts (
  id TEXT PRIMARY KEY,
  author_id TEXT NOT NULL REFERENCES users(id),
  author_name TEXT NOT NULL,
  name TEXT NOT NULL,
  icon TEXT,
  description TEXT,
  source TEXT NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('pending', 'approved', 'rejected')),
  downloads INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL,
  reviewed_at INTEGER,
  review_note TEXT
);

CREATE INDEX IF NOT EXISTS scripts_by_status ON scripts (status, created_at);
CREATE INDEX IF NOT EXISTS scripts_by_author ON scripts (author_id, created_at);
