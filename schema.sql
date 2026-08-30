-- Scrumy Database Schema

CREATE TABLE IF NOT EXISTS plans (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS capacity_plans (
    id                  TEXT PRIMARY KEY,
    plan_id             TEXT NOT NULL REFERENCES plans(id) ON DELETE CASCADE,
    name                TEXT NOT NULL,
    status              TEXT NOT NULL DEFAULT 'draft', -- draft | active | archived
    hours_per_sp        REAL NOT NULL DEFAULT 8,
    productive_hours    REAL NOT NULL DEFAULT 6,
    loading_factor      REAL NOT NULL DEFAULT 0.75,
    created_at          DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at          DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS team_members (
    id                TEXT PRIMARY KEY,
    capacity_plan_id  TEXT NOT NULL REFERENCES capacity_plans(id) ON DELETE CASCADE,
    name              TEXT NOT NULL,
    role              TEXT NOT NULL DEFAULT '',
    utilization_pct   REAL NOT NULL DEFAULT 100,
    sort_order        INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS sprints (
    id                TEXT PRIMARY KEY,
    capacity_plan_id  TEXT NOT NULL REFERENCES capacity_plans(id) ON DELETE CASCADE,
    name              TEXT NOT NULL,
    start_date        TEXT NOT NULL, -- YYYY-MM-DD
    end_date          TEXT NOT NULL, -- YYYY-MM-DD
    sort_order        INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS sprint_leaves (
    id         TEXT PRIMARY KEY,
    sprint_id  TEXT NOT NULL REFERENCES sprints(id) ON DELETE CASCADE,
    member_id  TEXT NOT NULL REFERENCES team_members(id) ON DELETE CASCADE,
    leaves     REAL NOT NULL DEFAULT 0,
    UNIQUE(sprint_id, member_id)
);

CREATE TABLE IF NOT EXISTS presentations (
    id          TEXT PRIMARY KEY,
    plan_id     TEXT NOT NULL REFERENCES plans(id) ON DELETE CASCADE,
    type        TEXT NOT NULL, -- intro | retro
    template_id TEXT NOT NULL DEFAULT 'default', -- default | minimalist
    title       TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'draft', -- draft | published
    sprint_name TEXT NOT NULL DEFAULT '',
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Stores the full JSON blob of presentation content
CREATE TABLE IF NOT EXISTS presentation_data (
    id              TEXT PRIMARY KEY,
    presentation_id TEXT NOT NULL REFERENCES presentations(id) ON DELETE CASCADE UNIQUE,
    content         TEXT NOT NULL DEFAULT '{}', -- JSON
    updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS users (
    id            TEXT PRIMARY KEY,
    username      TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL DEFAULT 'user', -- admin | user
    created_at    DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS plan_admins (
    plan_id TEXT NOT NULL REFERENCES plans(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY (plan_id, user_id)
);

CREATE TABLE IF NOT EXISTS sessions (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at DATETIME NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_capacity_plans_plan_id ON capacity_plans(plan_id);
CREATE INDEX IF NOT EXISTS idx_team_members_capacity_plan_id ON team_members(capacity_plan_id);
CREATE INDEX IF NOT EXISTS idx_sprints_capacity_plan_id ON sprints(capacity_plan_id);
CREATE INDEX IF NOT EXISTS idx_sprint_leaves_sprint_id ON sprint_leaves(sprint_id);
CREATE INDEX IF NOT EXISTS idx_presentations_plan_id ON presentations(plan_id);

CREATE TABLE IF NOT EXISTS jira_snapshot_refresh_logs (
    id                  TEXT PRIMARY KEY,
    snapshot_id         TEXT NOT NULL REFERENCES jira_snapshots(id) ON DELETE CASCADE,
    refreshed_at        DATETIME DEFAULT CURRENT_TIMESTAMP,
    prev_story_points   REAL NOT NULL,
    new_story_points    REAL NOT NULL,
    prev_hours_logged   REAL NOT NULL,
    new_hours_logged    REAL NOT NULL,
    prev_worklogs_count INTEGER NOT NULL,
    new_worklogs_count  INTEGER NOT NULL,
    prev_issues_count   INTEGER NOT NULL,
    new_issues_count    INTEGER NOT NULL,
    user_deltas         TEXT NOT NULL DEFAULT '[]',
    user_deltas_all     TEXT NOT NULL DEFAULT '[]'
);

CREATE INDEX IF NOT EXISTS idx_jira_snapshot_refresh_logs_snapshot_id ON jira_snapshot_refresh_logs(snapshot_id);

