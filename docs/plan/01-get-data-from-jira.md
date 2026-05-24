# Plan: Jira Integration & Retro Presentation Helper

This document outlines the design and implementation plan for integrating Jira Cloud with Scrumy to automatically retrieve completed issues, calculate metrics, and display a worklog leaderboard using persistent date-bounded snapshots.

---

## 1. Objective

Integrate Scrumy with Atlassian Jira Cloud APIs to:
1. Configure Jira URL, credentials (email + API token), and base JQL at the Plan workspace level.
2. Support creating multiple persistent **Jira Snapshots** per Plan. Each snapshot is cut between a `start_date` and an `end_date`.
3. Filter Jira issues using the base JQL plus `statusCategoryChangedDate` bounded by the snapshot's dates.
4. Retrieve worklogs for the matching issues, filtering worklog entries to only sum time logged **strictly within the snapshot's start/end date range**.
5. Display a leaderboard of developers, total story points burned, and list of closed issues.
6. Support importing a snapshot's aggregated metrics directly into retro presentations.

---

## 2. Database Schema Changes

We will extend the `plans` table to store Jira connection credentials, and create a new table `jira_snapshots` to persist historical sprint analyses.

```sql
-- SQLite migrations executed in internal/db/db.go

-- 1. Add Jira settings columns to plans
ALTER TABLE plans ADD COLUMN jira_url TEXT NOT NULL DEFAULT '';
ALTER TABLE plans ADD COLUMN jira_user TEXT NOT NULL DEFAULT '';
ALTER TABLE plans ADD COLUMN jira_token TEXT NOT NULL DEFAULT '';
ALTER TABLE plans ADD COLUMN jira_jql TEXT NOT NULL DEFAULT '';
ALTER TABLE plans ADD COLUMN jira_sp_field TEXT NOT NULL DEFAULT '';

-- 2. Create jira_snapshots table
CREATE TABLE IF NOT EXISTS jira_snapshots (
    id          TEXT PRIMARY KEY,
    plan_id     TEXT NOT NULL REFERENCES plans(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    start_date  TEXT NOT NULL, -- YYYY-MM-DD
    end_date    TEXT NOT NULL, -- YYYY-MM-DD
    data        TEXT NOT NULL DEFAULT '{}', -- JSON blob with issues, totals, leaderboard
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_jira_snapshots_plan_id ON jira_snapshots(plan_id);
```

---

## 3. Backend API Endpoints

We will add the following endpoints to Scurmy's Go backend:

### 3.1 `PUT /api/plans/{planID}/jira/settings`
Saves Jira settings for the workspace (masks the token on response).

### 3.2 `POST /api/plans/{planID}/jira/test-connection`
Validates Jira credentials by fetching `/rest/api/3/myself`.

### 3.3 `POST /api/plans/{planID}/jira/snapshots`
Creates a new snapshot, pulls data from Jira for the dates, runs analysis, and saves it in the database.
- **Request Body**:
  ```json
  {
    "name": "Sprint 24 Retro",
    "start_date": "2026-05-01",
    "end_date": "2026-05-15"
  }
  ```

### 3.4 `GET /api/plans/{planID}/jira/snapshots`
Lists all snapshots under the plan.

### 3.5 `GET /api/plans/{planID}/jira/snapshots/{snapshotID}`
Retrieves a specific snapshot.

### 3.6 `POST /api/plans/{planID}/jira/snapshots/{snapshotID}/refresh`
Re-fetches Jira data, updates the analysis calculations, and persists them back to the database.

### 3.7 `DELETE /api/plans/{planID}/jira/snapshots/{snapshotID}`
Deletes a snapshot.

---

## 4. Jira API & Analysis Logic

We will write a `jira` package under `internal/jira` that implements the API client and data aggregation:

1. **JQL Construction**:
   To search for issues, we construct the query:
   `({base_jql}) AND statusCategoryChangedDate >= "{start_date}" AND statusCategoryChangedDate <= "{end_date} 23:59"`
2. **Issue Fields to Request**:
   `GET /rest/api/3/search?jql=...&fields=summary,status,assignee,worklog,timespent,customfield_XXXX`
3. **Story Points Detection**:
   If the story points field ID is not configured, we inspect standard IDs or query `/rest/api/3/field` to find the custom field named "Story Points" or "Story point estimate".
4. **Worklog Aggregation**:
   - For each issue, check the `worklog.worklogs` array.
   - Filter each worklog entry: Only include if the worklog's `started` (or `created`) date is **between the start_date and end_date (inclusive)** of the snapshot.
   - Sum up `timeSpentSeconds` of the matched worklog entries grouped by `author.displayName`.
   - Build a leaderboard of authors sorted by hours logged descending.

---

## 5. UI/UX Design (Svelte 5)

We will introduce a "Jira Integration & Snapshots" card/section on the Plan details page:

1. **Settings / Config**:
   - Section to configure Jira URL, email, API Token, JQL, and custom fields.
   - "Test Connection" button.
2. **Snapshots Panel**:
   - List of previously generated snapshots.
   - "New Snapshot" button: opens a modal prompting for name, start date, and end date.
   - When a snapshot is clicked:
     - Expand to show details.
     - Metrics Card: Story Points Burned, Hours Logged in range, Leaderboard.
     - Issues table.
     - "Refresh" and "Delete" buttons.
     - "Create Presentation from Snapshot" button: redirects to presentation creation form with pre-filled metrics.

---

## 6. Pre-work and Tasks for the User

1. **Generate API Token**: Generate an Atlassian API token at [Atlassian Account Security](https://id.atlassian.com/manage-profile/security/api-tokens).
2. **Identify Story Points Custom Field ID**:
   Run:
   ```bash
   curl -u <email>:<api_token> -X GET "https://<your-domain>.atlassian.net/rest/api/3/field" | jq '.[] | select(.name | contains("Story Point"))'
   ```
3. **Verify JQL**: Confirm your base JQL (e.g. `project = MYPROJ AND statusCategory = Done`) retrieves the correct items in the Jira UI.
