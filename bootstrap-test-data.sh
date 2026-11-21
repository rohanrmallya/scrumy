#!/usr/bin/env bash
#
# bootstrap-test-data.sh — seed the Scrumy sqlite database with dummy data
# for end-to-end manual testing of the whole app.
#
# Covers every major surface:
#   users / team admins, teams (+ Jira settings), capacity plans,
#   team members, sprints, sprint leaves, Jira snapshots,
#   Jira refresh history (jira_snapshot_refresh_logs),
#   presentations + presentation_data (intro + retro).
#
# Usage:
#   ./bootstrap-test-data.sh                  # seeds ./scrumy.db (backs it up first)
#   SCRUMY_DB=/path/to.db ./bootstrap-test-data.sh
#   ./bootstrap-test-data.sh --db /path/to.db [--no-backup] [--help]
#
# Notes:
#   - The server creates/migrates the DB on startup, so start it once
#     (or run `make run`) before seeding. If the DB file does not exist,
#     this script creates it from internal/db/schema.sql.
#   - Existing demo data is wiped (teams, capacity, snapshots, presentations,
#     non-root users). The `root` user is always preserved.
#   - sqlite3 CLI must be installed.
#   - Extra test users reuse root's password hash, so they log in with
#     whatever the root password is (default: `root`).
#
set -euo pipefail

DB="${SCRUMY_DB:-scrumy.db}"
BACKUP=1

while [[ $# -gt 0 ]]; do
  case "$1" in
    --db) DB="${2:?--db needs a path}"; shift 2 ;;
    --no-backup) BACKUP=0; shift ;;
    -h|--help)
      sed -n '2,/^$/p' "$0" | sed 's/^# \?//'
      echo "Env: SCRUMY_DB (default: scrumy.db)"
      exit 0 ;;
    *) echo "Unknown arg: $1 (try --help)" >&2; exit 1 ;;
  esac
done

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SCHEMA_FILE="$SCRIPT_DIR/internal/db/schema.sql"

command -v sqlite3 >/dev/null 2>&1 || { echo "ERROR: sqlite3 CLI not found. Install it first." >&2; exit 1; }

# Escape single quotes for safe embedding in SQL string literals.
esc() { printf "%s" "$1" | sed "s/'/''/g"; }

if [[ ! -f "$DB" ]]; then
  echo "DB file '$DB' not found — creating it from schema."
  if [[ -f "$SCHEMA_FILE" ]]; then
    sqlite3 "$DB" < "$SCHEMA_FILE"
  else
    echo "ERROR: schema file not found at $SCHEMA_FILE" >&2; exit 1
  fi
fi

# The DB must have the migrated tables; the server applies these on boot.
if ! sqlite3 "$DB" "SELECT name FROM sqlite_master WHERE type='table' AND name='jira_snapshot_refresh_logs';" | grep -q jira_snapshot_refresh_logs; then
  echo "ERROR: table jira_snapshot_refresh_logs missing."
  echo "Start the server once (it auto-migrates), then re-run this script." >&2
  exit 1
fi

# Root user must exist (seeded by the server on startup).
ROOT_ID="$(sqlite3 "$DB" "SELECT id FROM users WHERE username='root';")"
ROOT_HASH="$(sqlite3 "$DB" "SELECT password_hash FROM users WHERE username='root';")"
if [[ -z "${ROOT_ID:-}" || -z "${ROOT_HASH:-}" ]]; then
  echo "ERROR: root user not found in $DB. Start the server once to seed it, then re-run." >&2
  exit 1
fi
ROOT_HASH_ESC="$(esc "$ROOT_HASH")"

if [[ "$BACKUP" == 1 ]]; then
  BACKUP_FILE="${DB}.bak.$(date +%Y%m%d-%H%M%S)"
  cp "$DB" "$BACKUP_FILE"
  echo "Backed up $DB -> $BACKUP_FILE"
fi

echo "Seeding test data into $DB ..."

# ---------------------------------------------------------------------------
# Static JSON blobs (no single quotes allowed — they would break SQL literals)
# ---------------------------------------------------------------------------

read -r -d '' SNAP_S24_JSON <<'EOF' || true
{"issues":[{"key":"PHX-101","summary":"Auth service backend","status":"Done","status_category_key":"done","story_points":5,"time_spent_hours":12.0,"time_spent_hours_all":14.0,"status_category_changed_date":"2026-06-05T17:30:00.000Z"},{"key":"PHX-102","summary":"Dashboard chart components","status":"Done","status_category_key":"done","story_points":8,"time_spent_hours":15.5,"time_spent_hours_all":17.5,"status_category_changed_date":"2026-06-08T11:00:00.000Z"},{"key":"PHX-103","summary":"Database query optimization","status":"Done","status_category_key":"done","story_points":3,"time_spent_hours":9.0,"time_spent_hours_all":12.0,"status_category_changed_date":"2026-06-10T16:45:00.000Z"},{"key":"PHX-104","summary":"Push notification service","status":"In Progress","status_category_key":"indeterminate","story_points":5,"time_spent_hours":4.0,"time_spent_hours_all":6.0,"status_category_changed_date":"2026-06-12T09:15:00.000Z"},{"key":"PHX-106","summary":"Hotfix login redirect loop","status":"Done","status_category_key":"done","story_points":2,"time_spent_hours":2.5,"time_spent_hours_all":2.5,"status_category_changed_date":"2026-06-13T14:20:00.000Z","is_unplanned":true}],"totals":{"total_story_points":18,"total_hours_logged":43.0,"total_work_logs":9,"avg_hours_per_sp":2.39},"leaderboard":[{"author_name":"Alice Cooper","hours_logged":18.0,"percentage":41.86,"worklogs":[{"issue_key":"PHX-102","issue_summary":"Dashboard chart components","hours_logged":10.0},{"issue_key":"PHX-101","issue_summary":"Auth service backend","hours_logged":8.0}]},{"author_name":"Bob Marley","hours_logged":15.0,"percentage":34.88,"worklogs":[{"issue_key":"PHX-103","issue_summary":"Database query optimization","hours_logged":9.0},{"issue_key":"PHX-104","issue_summary":"Push notification service","hours_logged":4.0},{"issue_key":"PHX-106","issue_summary":"Hotfix login redirect loop","hours_logged":2.0}]},{"author_name":"Charlie Brown","hours_logged":10.0,"percentage":23.26,"worklogs":[{"issue_key":"PHX-102","issue_summary":"Dashboard chart components","hours_logged":5.5},{"issue_key":"PHX-101","issue_summary":"Auth service backend","hours_logged":4.0},{"issue_key":"PHX-106","issue_summary":"Hotfix login redirect loop","hours_logged":0.5}]}],"totals_all":{"total_story_points":18,"total_hours_logged":52.0,"total_work_logs":12,"avg_hours_per_sp":2.89},"leaderboard_all":[{"author_name":"Alice Cooper","hours_logged":21.5,"percentage":41.35,"worklogs":[{"issue_key":"PHX-102","issue_summary":"Dashboard chart components","hours_logged":11.5},{"issue_key":"PHX-101","issue_summary":"Auth service backend","hours_logged":10.0}]},{"author_name":"Bob Marley","hours_logged":18.0,"percentage":34.62,"worklogs":[{"issue_key":"PHX-103","issue_summary":"Database query optimization","hours_logged":12.0},{"issue_key":"PHX-104","issue_summary":"Push notification service","hours_logged":4.5},{"issue_key":"PHX-106","issue_summary":"Hotfix login redirect loop","hours_logged":1.5}]},{"author_name":"Charlie Brown","hours_logged":12.5,"percentage":24.04,"worklogs":[{"issue_key":"PHX-102","issue_summary":"Dashboard chart components","hours_logged":6.0},{"issue_key":"PHX-101","issue_summary":"Auth service backend","hours_logged":4.0},{"issue_key":"PHX-104","issue_summary":"Push notification service","hours_logged":1.5},{"issue_key":"PHX-106","issue_summary":"Hotfix login redirect loop","hours_logged":1.0}]}]}
EOF

read -r -d '' SNAP_S25_JSON <<'EOF' || true
{"issues":[{"key":"PHX-201","summary":"SSO integration with Okta","status":"Done","status_category_key":"done","story_points":5,"time_spent_hours":11.5,"time_spent_hours_all":13.0,"status_category_changed_date":"2026-06-20T15:00:00.000Z"},{"key":"PHX-202","summary":"Audit log export API","status":"Done","status_category_key":"done","story_points":3,"time_spent_hours":7.0,"time_spent_hours_all":8.5,"status_category_changed_date":"2026-06-23T12:30:00.000Z"},{"key":"PHX-203","summary":"Rate limiting middleware","status":"Done","status_category_key":"done","story_points":3,"time_spent_hours":4.0,"time_spent_hours_all":5.0,"status_category_changed_date":"2026-06-26T10:00:00.000Z"},{"key":"PHX-204","summary":"Notification preferences page","status":"Done","status_category_key":"done","story_points":3,"time_spent_hours":6.5,"time_spent_hours_all":7.5,"status_category_changed_date":"2026-06-25T14:00:00.000Z"},{"key":"PHX-205","summary":"Slow report query pagination","status":"In Progress","status_category_key":"indeterminate","story_points":5,"time_spent_hours":3.0,"time_spent_hours_all":4.5,"status_category_changed_date":"2026-06-26T16:30:00.000Z"},{"key":"PHX-207","summary":"Hotfix session expiry loop","status":"Done","status_category_key":"done","story_points":2,"time_spent_hours":1.5,"time_spent_hours_all":1.5,"status_category_changed_date":"2026-06-27T09:45:00.000Z","is_unplanned":true}],"totals":{"total_story_points":16,"total_hours_logged":33.5,"total_work_logs":14,"avg_hours_per_sp":2.09},"leaderboard":[{"author_name":"Alice Cooper","hours_logged":13.0,"percentage":38.81,"worklogs":[{"issue_key":"PHX-201","issue_summary":"SSO integration with Okta","hours_logged":9.0},{"issue_key":"PHX-204","issue_summary":"Notification preferences page","hours_logged":4.0}]},{"author_name":"Bob Marley","hours_logged":10.5,"percentage":31.34,"worklogs":[{"issue_key":"PHX-202","issue_summary":"Audit log export API","hours_logged":5.0},{"issue_key":"PHX-203","issue_summary":"Rate limiting middleware","hours_logged":2.5},{"issue_key":"PHX-205","issue_summary":"Slow report query pagination","hours_logged":2.0},{"issue_key":"PHX-207","issue_summary":"Hotfix session expiry loop","hours_logged":1.0}]},{"author_name":"Charlie Brown","hours_logged":7.0,"percentage":20.9,"worklogs":[{"issue_key":"PHX-201","issue_summary":"SSO integration with Okta","hours_logged":2.5},{"issue_key":"PHX-202","issue_summary":"Audit log export API","hours_logged":2.0},{"issue_key":"PHX-203","issue_summary":"Rate limiting middleware","hours_logged":1.5},{"issue_key":"PHX-204","issue_summary":"Notification preferences page","hours_logged":1.0}]},{"author_name":"Diana Prince","hours_logged":3.0,"percentage":8.96,"worklogs":[{"issue_key":"PHX-204","issue_summary":"Notification preferences page","hours_logged":1.5},{"issue_key":"PHX-205","issue_summary":"Slow report query pagination","hours_logged":1.0},{"issue_key":"PHX-207","issue_summary":"Hotfix session expiry loop","hours_logged":0.5}]}],"totals_all":{"total_story_points":16,"total_hours_logged":40.0,"total_work_logs":18,"avg_hours_per_sp":2.5},"leaderboard_all":[{"author_name":"Alice Cooper","hours_logged":15.5,"percentage":38.75,"worklogs":[{"issue_key":"PHX-201","issue_summary":"SSO integration with Okta","hours_logged":10.5},{"issue_key":"PHX-204","issue_summary":"Notification preferences page","hours_logged":5.0}]},{"author_name":"Bob Marley","hours_logged":13.0,"percentage":32.5,"worklogs":[{"issue_key":"PHX-202","issue_summary":"Audit log export API","hours_logged":6.0},{"issue_key":"PHX-203","issue_summary":"Rate limiting middleware","hours_logged":3.5},{"issue_key":"PHX-205","issue_summary":"Slow report query pagination","hours_logged":2.5},{"issue_key":"PHX-207","issue_summary":"Hotfix session expiry loop","hours_logged":1.0}]},{"author_name":"Charlie Brown","hours_logged":8.0,"percentage":20.0,"worklogs":[{"issue_key":"PHX-201","issue_summary":"SSO integration with Okta","hours_logged":2.5},{"issue_key":"PHX-202","issue_summary":"Audit log export API","hours_logged":2.5},{"issue_key":"PHX-203","issue_summary":"Rate limiting middleware","hours_logged":1.5},{"issue_key":"PHX-204","issue_summary":"Notification preferences page","hours_logged":1.5}]},{"author_name":"Diana Prince","hours_logged":3.5,"percentage":8.75,"worklogs":[{"issue_key":"PHX-204","issue_summary":"Notification preferences page","hours_logged":1.0},{"issue_key":"PHX-205","issue_summary":"Slow report query pagination","hours_logged":2.0},{"issue_key":"PHX-207","issue_summary":"Hotfix session expiry loop","hours_logged":0.5}]}]}
EOF

read -r -d '' DELTAS_S24_R0 <<'EOF' || true
[{"author_name":"Alice Cooper","prev_hours_logged":8.0,"new_hours_logged":11.0,"hours_delta":3.0},{"author_name":"Bob Marley","prev_hours_logged":6.0,"new_hours_logged":8.5,"hours_delta":2.5},{"author_name":"Charlie Brown","prev_hours_logged":4.0,"new_hours_logged":6.5,"hours_delta":2.5}]
EOF

read -r -d '' DELTAS_S24_R0_ALL <<'EOF' || true
[{"author_name":"Alice Cooper","prev_hours_logged":10.0,"new_hours_logged":13.5,"hours_delta":3.5},{"author_name":"Bob Marley","prev_hours_logged":7.5,"new_hours_logged":10.0,"hours_delta":2.5},{"author_name":"Charlie Brown","prev_hours_logged":4.5,"new_hours_logged":7.5,"hours_delta":3.0}]
EOF

read -r -d '' DELTAS_S24_R1 <<'EOF' || true
[{"author_name":"Alice Cooper","prev_hours_logged":11.0,"new_hours_logged":14.0,"hours_delta":3.0},{"author_name":"Bob Marley","prev_hours_logged":8.5,"new_hours_logged":10.5,"hours_delta":2.0},{"author_name":"Charlie Brown","prev_hours_logged":6.5,"new_hours_logged":9.0,"hours_delta":2.5}]
EOF

read -r -d '' DELTAS_S24_R1_ALL <<'EOF' || true
[{"author_name":"Alice Cooper","prev_hours_logged":13.5,"new_hours_logged":16.5,"hours_delta":3.0},{"author_name":"Bob Marley","prev_hours_logged":10.0,"new_hours_logged":13.0,"hours_delta":3.0},{"author_name":"Charlie Brown","prev_hours_logged":7.5,"new_hours_logged":10.5,"hours_delta":3.0}]
EOF

read -r -d '' DELTAS_S24_R2 <<'EOF' || true
[{"author_name":"Alice Cooper","prev_hours_logged":14.0,"new_hours_logged":16.0,"hours_delta":2.0},{"author_name":"Bob Marley","prev_hours_logged":10.5,"new_hours_logged":12.0,"hours_delta":1.5},{"author_name":"Charlie Brown","prev_hours_logged":9.0,"new_hours_logged":10.0,"hours_delta":1.0}]
EOF

read -r -d '' DELTAS_S24_R2_ALL <<'EOF' || true
[{"author_name":"Alice Cooper","prev_hours_logged":16.5,"new_hours_logged":19.0,"hours_delta":2.5},{"author_name":"Bob Marley","prev_hours_logged":13.0,"new_hours_logged":15.0,"hours_delta":2.0},{"author_name":"Charlie Brown","prev_hours_logged":10.5,"new_hours_logged":12.5,"hours_delta":2.0}]
EOF

read -r -d '' DELTAS_S24_R3 <<'EOF' || true
[{"author_name":"Alice Cooper","prev_hours_logged":16.0,"new_hours_logged":18.0,"hours_delta":2.0},{"author_name":"Bob Marley","prev_hours_logged":12.0,"new_hours_logged":15.0,"hours_delta":3.0},{"author_name":"Charlie Brown","prev_hours_logged":10.0,"new_hours_logged":10.0,"hours_delta":0.0}]
EOF

read -r -d '' DELTAS_S24_R3_ALL <<'EOF' || true
[{"author_name":"Alice Cooper","prev_hours_logged":19.0,"new_hours_logged":21.5,"hours_delta":2.5},{"author_name":"Bob Marley","prev_hours_logged":15.0,"new_hours_logged":18.0,"hours_delta":3.0},{"author_name":"Charlie Brown","prev_hours_logged":12.5,"new_hours_logged":12.5,"hours_delta":0.0}]
EOF

read -r -d '' DELTAS_S25_R1 <<'EOF' || true
[{"author_name":"Alice Cooper","prev_hours_logged":6.0,"new_hours_logged":8.5,"hours_delta":2.5},{"author_name":"Bob Marley","prev_hours_logged":5.5,"new_hours_logged":7.5,"hours_delta":2.0},{"author_name":"Charlie Brown","prev_hours_logged":3.5,"new_hours_logged":5.0,"hours_delta":1.5}]
EOF

read -r -d '' DELTAS_S25_R1_ALL <<'EOF' || true
[{"author_name":"Alice Cooper","prev_hours_logged":7.0,"new_hours_logged":9.5,"hours_delta":2.5},{"author_name":"Bob Marley","prev_hours_logged":6.5,"new_hours_logged":9.0,"hours_delta":2.5},{"author_name":"Charlie Brown","prev_hours_logged":4.0,"new_hours_logged":5.5,"hours_delta":1.5},{"author_name":"Diana Prince","prev_hours_logged":0.5,"new_hours_logged":1.0,"hours_delta":0.5}]
EOF

read -r -d '' DELTAS_S25_R2 <<'EOF' || true
[{"author_name":"Alice Cooper","prev_hours_logged":8.5,"new_hours_logged":11.0,"hours_delta":2.5},{"author_name":"Bob Marley","prev_hours_logged":7.5,"new_hours_logged":9.5,"hours_delta":2.0},{"author_name":"Charlie Brown","prev_hours_logged":5.0,"new_hours_logged":6.5,"hours_delta":1.5},{"author_name":"Diana Prince","prev_hours_logged":0.0,"new_hours_logged":1.0,"hours_delta":1.0}]
EOF

read -r -d '' DELTAS_S25_R2_ALL <<'EOF' || true
[{"author_name":"Alice Cooper","prev_hours_logged":9.5,"new_hours_logged":12.5,"hours_delta":3.0},{"author_name":"Bob Marley","prev_hours_logged":9.0,"new_hours_logged":11.5,"hours_delta":2.5},{"author_name":"Charlie Brown","prev_hours_logged":5.5,"new_hours_logged":7.5,"hours_delta":2.0},{"author_name":"Diana Prince","prev_hours_logged":1.0,"new_hours_logged":2.0,"hours_delta":1.0}]
EOF

read -r -d '' DELTAS_S25_R3 <<'EOF' || true
[{"author_name":"Alice Cooper","prev_hours_logged":11.0,"new_hours_logged":13.0,"hours_delta":2.0},{"author_name":"Bob Marley","prev_hours_logged":9.5,"new_hours_logged":10.5,"hours_delta":1.0},{"author_name":"Charlie Brown","prev_hours_logged":6.5,"new_hours_logged":7.0,"hours_delta":0.5},{"author_name":"Diana Prince","prev_hours_logged":1.0,"new_hours_logged":3.0,"hours_delta":2.0}]
EOF

read -r -d '' DELTAS_S25_R3_ALL <<'EOF' || true
[{"author_name":"Alice Cooper","prev_hours_logged":12.5,"new_hours_logged":15.5,"hours_delta":3.0},{"author_name":"Bob Marley","prev_hours_logged":11.5,"new_hours_logged":13.0,"hours_delta":1.5},{"author_name":"Charlie Brown","prev_hours_logged":7.5,"new_hours_logged":8.0,"hours_delta":0.5},{"author_name":"Diana Prince","prev_hours_logged":2.0,"new_hours_logged":3.5,"hours_delta":1.5}]
EOF

read -r -d '' INTRO_CONTENT <<'EOF' || true
{"learnings":[{"title":"Flaky tests slow us down","content":"Retry logic in CI hid real failures in Sprint 24. Quarantine flaky tests instead of auto-retrying.","tags":["process","ci"]},{"title":"Small PRs ship faster","content":"PRs under 300 lines were reviewed twice as fast and had fewer revert issues.","tags":["engineering"]}],"changes":[{"title":"Freeze scope mid-sprint","content":"No new tickets accepted after Wednesday of week one unless marked as production incident.","tags":["planning"]},{"title":"QA joins standup","content":"Diana joins the daily standup so testing bottlenecks surface one day earlier.","tags":["qa"]}],"previous_data":{"total_sp_delivered":18,"total_hours_logged":43.0,"total_work_logs":9,"avg_hours_per_sp":2.39,"planned_sp":20,"executed_sp":18,"spillovers":1,"total_epics_delivered":2},"epics":[{"id":"epic-auth","title":"Auth service backend","summary":"New login and session service with SSO support.","why_needed":"Legacy auth cannot support SSO for enterprise customers.","when_doing":"Sprint 26","audience":"Platform users","total_sp":8},{"id":"epic-mobile","title":"Push notification service","summary":"Cross platform push delivery with user preferences.","why_needed":"Activation drops when users miss time sensitive updates.","when_doing":"Sprint 26","audience":"Mobile users","total_sp":5}],"contributors":[{"name":"Alice Cooper","contribution":"Shipped dashboard charts and led sprint planning"},{"name":"Bob Marley","contribution":"Drove DB optimization and unblocked QA"},{"name":"Charlie Brown","contribution":"Frontend polish and hotfix turnaround"}],"closing_text":"Plan is set. Lets make Sprint 26 the smoothest one yet."}
EOF

read -r -d '' RETRO24_CONTENT <<'EOF' || true
{"previous_data":{"total_sp_delivered":18,"total_hours_logged":43.0,"total_work_logs":9,"avg_hours_per_sp":2.39,"planned_sp":20,"executed_sp":18,"spillovers":1,"total_epics_delivered":2},"feedback":["Velocity recovered after the auth epic landed early","Push notification spillover was a scope estimation miss, not an effort miss","Hotfix rotation worked well and did not derail the sprint goal"],"contributors":[{"name":"Alice Cooper","contribution":"Top logger at 18 hours across auth and dashboard work"},{"name":"Bob Marley","contribution":"15 hours with the big DB performance win"},{"name":"Charlie Brown","contribution":"10 hours plus fastest hotfix turnaround"}],"closing_text":"Great work closing 18 points. Carry the momentum into Sprint 25."}
EOF

read -r -d '' RETRO25_CONTENT <<'EOF' || true
{"previous_data":{"total_sp_delivered":16,"total_hours_logged":33.5,"total_work_logs":14,"avg_hours_per_sp":2.09,"planned_sp":18,"executed_sp":16,"spillovers":1,"total_epics_delivered":2},"feedback":["SSO integration finished ahead of schedule","Unplanned session expiry hotfix was absorbed without missing the sprint goal","Report pagination spillover was a scope estimation miss, now split for Sprint 26"],"contributors":[{"name":"Alice Cooper","contribution":"Led SSO integration end to end at 13 hours"},{"name":"Bob Marley","contribution":"Delivered audit export API with docs at 10.5 hours"},{"name":"Diana Prince","contribution":"Preferences page plus hotfix support"}],"closing_text":"16 points with unplanned work absorbed. Ready for Sprint 26 kickoff."}
EOF

# ---------------------------------------------------------------------------
# Seed (single sqlite3 invocation; FK-safe delete order, then inserts)
# ---------------------------------------------------------------------------

sqlite3 "$DB" <<SQL
PRAGMA foreign_keys=ON;
BEGIN;

-- Wipe previous demo data (keep root user + sessions untouched apart from orphans).
DELETE FROM jira_snapshot_refresh_logs;
DELETE FROM jira_snapshots;
DELETE FROM sprint_leaves;
DELETE FROM sprints;
DELETE FROM team_members;
DELETE FROM team_mates;
DELETE FROM capacity_plans;
DELETE FROM presentation_data;
DELETE FROM presentations;
DELETE FROM team_admins;
DELETE FROM teams;
DELETE FROM users WHERE username != 'root';

-- Extra users (password = same as root password; hash copied from root row).
INSERT INTO users (id, username, password_hash, role, email) VALUES
  ('user-alice-001', 'alice', '$ROOT_HASH_ESC', 'admin', 'alice@example.com'),
  ('user-bob-002',   'bob',   '$ROOT_HASH_ESC', 'user', 'bob@example.com');

-- Plans. Phoenix points at the local mock Jira so Refresh works offline
-- against jira_mock.json; Atlas is left unconfigured to exercise that path.
INSERT INTO teams (id, name, jira_url, jira_user, jira_token, jira_jql, jira_sp_field, jira_insecure) VALUES
  ('plan-phoenix-001', 'Phoenix Team', 'mock', 'test@example.com', 'mock-token', 'project = PHX ORDER BY created DESC', 'customfield_10016', 0),
  ('plan-atlas-002',   'Atlas Platform', '', '', '', '', '', 0);

INSERT INTO team_admins (team_id, user_id) VALUES
  ('plan-phoenix-001', '$ROOT_ID'),
  ('plan-phoenix-001', 'user-alice-001'),
  ('plan-atlas-002',   '$ROOT_ID');

-- Team Mates for Phoenix Team.
INSERT INTO team_mates (id, team_id, user_id, name, email, jira_account_id, role) VALUES
  ('mate-001', 'plan-phoenix-001', 'user-alice-001', 'Alice Cooper', 'alice@example.com', 'jira-alice-101', 'Tech Lead'),
  ('mate-002', 'plan-phoenix-001', 'user-bob-002',   'Bob Marley',   'bob@example.com',   'jira-bob-102',   'Backend Engineer');

-- Capacity plans.
INSERT INTO capacity_plans (id, team_id, name, status, hours_per_sp, productive_hours, loading_factor, created_at, updated_at) VALUES
  ('cap-phoenix-q2-001', 'plan-phoenix-001', 'FY26 Q2 Capacity', 'active', 6, 6, 0.8, datetime('now','-30 days'), datetime('now','-1 day')),
  ('cap-phoenix-q3-002', 'plan-phoenix-001', 'FY26 Q3 Planning', 'draft', 8, 6, 0.75, datetime('now','-2 days'), datetime('now')),
  ('cap-atlas-v1-003',   'plan-atlas-002',   'Atlas Baseline',    'active', 7, 5.5, 0.8, datetime('now','-10 days'), datetime('now','-1 day'));

-- Team members.
INSERT INTO team_members (id, capacity_plan_id, name, role, utilization_pct, sort_order) VALUES
  ('mem-alice-001',  'cap-phoenix-q2-001', 'Alice Cooper',  'Tech Lead',         100, 1),
  ('mem-bob-002',    'cap-phoenix-q2-001', 'Bob Marley',    'Backend Engineer',  100, 2),
  ('mem-charlie-003','cap-phoenix-q2-001', 'Charlie Brown', 'Frontend Engineer',  80, 3),
  ('mem-diana-004',  'cap-phoenix-q2-001', 'Diana Prince',  'QA Engineer',       100, 4),
  ('mem-evan-005',   'cap-phoenix-q2-001', 'Evan Stone',    'DevOps Engineer',    50, 5),
  ('mem-fiona-006',  'cap-phoenix-q2-001', 'Fiona Glen',    'UI Designer',        60, 6),
  ('mem-gus-007',    'cap-phoenix-q3-002', 'Gus Hall',      'Backend Engineer',  100, 1),
  ('mem-ivy-008',    'cap-phoenix-q3-002', 'Ivy Chen',      'Frontend Engineer',  100, 2),
  ('mem-jack-009',   'cap-atlas-v1-003',   'Jack Ryan',     'Fullstack Engineer', 90, 1);

-- Sprints.
INSERT INTO sprints (id, capacity_plan_id, name, start_date, end_date, sort_order) VALUES
  ('spr-24-001', 'cap-phoenix-q2-001', 'Sprint 24', '2026-06-02', '2026-06-13', 1),
  ('spr-25-002', 'cap-phoenix-q2-001', 'Sprint 25', '2026-06-16', '2026-06-27', 2),
  ('spr-26-003', 'cap-phoenix-q2-001', 'Sprint 26', '2026-06-30', '2026-07-11', 3),
  ('spr-q3-004', 'cap-phoenix-q3-002', 'Sprint 27', '2026-07-14', '2026-07-25', 1),
  ('spr-a1-005', 'cap-atlas-v1-003',   'Atlas Sprint 1', '2026-06-16', '2026-06-27', 1);

-- Leaves (sparse matrix: zero-leave pairs are simply absent).
INSERT INTO sprint_leaves (id, sprint_id, member_id, leaves) VALUES
  ('leave-001', 'spr-24-001', 'mem-charlie-003', 2),
  ('leave-002', 'spr-24-001', 'mem-evan-005',    1),
  ('leave-003', 'spr-25-002', 'mem-alice-001',   1),
  ('leave-004', 'spr-25-002', 'mem-diana-004',   3),
  ('leave-005', 'spr-25-002', 'mem-fiona-006',   2),
  ('leave-006', 'spr-26-003', 'mem-bob-002',     2),
  ('leave-007', 'spr-26-003', 'mem-evan-005',    5),
  ('leave-008', 'spr-a1-005', 'mem-jack-009',    1.5);

-- Jira snapshots.
INSERT INTO jira_snapshots (id, team_id, name, start_date, end_date, all_worklogs, data, created_at, updated_at) VALUES
  ('snap-s24-001', 'plan-phoenix-001', 'Sprint 24 Retro', '2026-06-02', '2026-06-13', 0, '$(esc "$SNAP_S24_JSON")', datetime('now','-12 days'), datetime('now','-2 days')),
  ('snap-s25-002', 'plan-phoenix-001', 'Sprint 25 Retro', '2026-06-16', '2026-06-27', 0, '$(esc "$SNAP_S25_JSON")', datetime('now','-5 days'), datetime('now','-1 day'));

-- Refresh history: each row shows prev -> new growth plus per-user deltas.
INSERT INTO jira_snapshot_refresh_logs (id, snapshot_id, prev_story_points, new_story_points, prev_hours_logged, new_hours_logged, prev_worklogs_count, new_worklogs_count, prev_issues_count, new_issues_count, refreshed_at, user_deltas, user_deltas_all) VALUES
  ('ref-s24-000', 'snap-s24-001', 8, 11, 18.0, 26.0, 4, 6, 2, 3, datetime('now','-11 days'), '$(esc "$DELTAS_S24_R0")', '$(esc "$DELTAS_S24_R0_ALL")'),
  ('ref-s24-001', 'snap-s24-001', 11, 14, 26.0, 33.5, 6, 7, 3, 4, datetime('now','-7 days'), '$(esc "$DELTAS_S24_R1")', '$(esc "$DELTAS_S24_R1_ALL")'),
  ('ref-s24-002', 'snap-s24-001', 14, 16, 33.5, 38.0, 7, 8, 4, 4, datetime('now','-4 days'), '$(esc "$DELTAS_S24_R2")', '$(esc "$DELTAS_S24_R2_ALL")'),
  ('ref-s24-003', 'snap-s24-001', 16, 18, 38.0, 43.0, 8, 9, 4, 5, datetime('now','-2 days'), '$(esc "$DELTAS_S24_R3")', '$(esc "$DELTAS_S24_R3_ALL")'),
  ('ref-s25-001', 'snap-s25-002', 8, 11, 15.0, 21.0, 6, 9, 3, 4, datetime('now','-6 days'), '$(esc "$DELTAS_S25_R1")', '$(esc "$DELTAS_S25_R1_ALL")'),
  ('ref-s25-002', 'snap-s25-002', 11, 14, 21.0, 28.0, 9, 12, 4, 5, datetime('now','-3 days'), '$(esc "$DELTAS_S25_R2")', '$(esc "$DELTAS_S25_R2_ALL")'),
  ('ref-s25-003', 'snap-s25-002', 14, 16, 28.0, 33.5, 12, 14, 5, 6, datetime('now','-1 day'), '$(esc "$DELTAS_S25_R3")', '$(esc "$DELTAS_S25_R3_ALL")');

-- Presentations (intro + retro, default + minimalist, draft + published).
INSERT INTO presentations (id, team_id, type, template_id, title, status, sprint_name, created_at, updated_at) VALUES
  ('pres-intro-26-001', 'plan-phoenix-001', 'intro', 'default',    'Sprint 26 Kickoff', 'published', 'Sprint 26', datetime('now','-3 days'), datetime('now','-1 day')),
  ('pres-retro-24-002', 'plan-phoenix-001', 'retro', 'minimalist', 'Sprint 24 Retro',   'published', 'Sprint 24', datetime('now','-6 days'), datetime('now','-2 days')),
  ('pres-retro-25-003', 'plan-phoenix-001', 'retro', 'default',    'Sprint 25 Retro',   'draft',     'Sprint 25', datetime('now','-1 day'),  datetime('now'));

INSERT INTO presentation_data (id, presentation_id, content, updated_at) VALUES
  ('pdata-001', 'pres-intro-26-001', '$(esc "$INTRO_CONTENT")', datetime('now','-1 day')),
  ('pdata-002', 'pres-retro-24-002', '$(esc "$RETRO24_CONTENT")', datetime('now','-2 days')),
  ('pdata-003', 'pres-retro-25-003', '$(esc "$RETRO25_CONTENT")', datetime('now'));

COMMIT;
SQL

echo "Verifying..."
sqlite3 "$DB" "PRAGMA foreign_key_check;"
sqlite3 "$DB" <<'SQL'
SELECT 'users='         || (SELECT COUNT(*) FROM users);
SELECT 'teams='         || (SELECT COUNT(*) FROM teams);
SELECT 'capacity_plans='|| (SELECT COUNT(*) FROM capacity_plans);
SELECT 'team_members='  || (SELECT COUNT(*) FROM team_members);
SELECT 'team_mates='    || (SELECT COUNT(*) FROM team_mates);
SELECT 'sprints='       || (SELECT COUNT(*) FROM sprints);
SELECT 'sprint_leaves=' || (SELECT COUNT(*) FROM sprint_leaves);
SELECT 'jira_snapshots='|| (SELECT COUNT(*) FROM jira_snapshots);
SELECT 'refresh_logs='  || (SELECT COUNT(*) FROM jira_snapshot_refresh_logs);
SELECT 'presentations=' || (SELECT COUNT(*) FROM presentations);
SQL

cat <<'EOF'

Done. Test the full app with:
  - Login: root / root  (super admin)
  - Login: alice / root (team admin)   |  bob / root (viewer)
  - Plans: Phoenix Team (mock Jira, snapshots, refresh history, presentations)
           Atlas Platform (empty Jira config, baseline capacity plan)
  - Capacity: FY26 Q2 Capacity (active, 6 members, 3 sprints, leaves)
  - Snapshots: Sprint 24 Retro (5 issues, 4 refreshes) + Sprint 25 Retro (6 issues incl. unplanned hotfix, 3 refreshes)
  - Presentations: Sprint 26 Kickoff (intro, published), Sprint 24/25 retros
EOF
