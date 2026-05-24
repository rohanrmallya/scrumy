# Scrumy 🚀

Scrumy is a developer capacity planner, retrospective analytics dashboard, and sprint presentation builder. By bridging the gap between developer schedules (leaves, capacity) and actual Jira outcomes (velocity, logged hours), Scrumy helps engineering leaders plan sprints accurately and generate beautiful retrospective slide decks with one click.

---

## 🎯 The Need & Purpose

Sprint planning and retrospectives are often plagued by manual effort and guesswork:

1. **Capacity Guesswork**: Leave schedules, individual utilization levels, and overhead loading factors are rarely combined dynamically to compute true target story points.
2. **Scattered Retro Data**: Calculating actual worklogged hours, average hours per story point, and closed issues requires tedious manual spreadsheet calculations.
3. **Manual Presentations**: Creating sprint kickoff (intro) and retro slide decks involves constant copy-pasting of charts and leaderboard statistics.

**Scrumy solves this** by hosting planning tools, Jira date-bounded snapshots, and presentation slides under a unified, shareable plan workspace.

---

## ✨ Core Features

### 1. Plan Workspaces & Administration

- Create dedicated spaces for separate teams.
- Admin-level access control: only designated administrators can edit capacity plans, adjust Jira credentials, or delete snapshots.
- Root account (`root` / `root`) seeded automatically for initial configuration.

### 2. Capacity Planning & Target Modeling

- Add team members with custom **Utilization %** (utilization-adjusted leave days are dynamically factored in).
- Add sprints with specific date ranges.
- Set leaves per developer per sprint.
- Auto-calculate capacity metrics using:
  - **Gross Person Days** = sum(utilization) × weekdays
  - **Leave Adjusted Overhead** = sum(leaves × utilization)
  - **Net Person Days** = Gross − Leaves
  - **Loaded Person Days** = Net × Loading Factor (e.g. 0.8 loading for meeting/overhead buffers)
  - **Target Story Points** = (Loaded Person Days × Productive Hours) ÷ Hours per Story Point
  - **Thin & Stretch Targets** to guide sprint commitments.

### 3. Jira Integration & Date-Bounded Snapshots

- Configure Jira Cloud API credentials (URL, User Email, Token) and a JQL filter.
- Generate **date-bounded persistent snapshots** from Jira.
- Automatically detect custom Story Points fields or fall back to standard naming conventions.
- Pull closed issues and parse worklogs, filtering entries to sum hours **strictly matching the snapshot date range** (with a checkbox option to include all worklogs).
- Display snapshot diagnostics: Story Points Burned, Hours Logged, Worklog Counts, and Hours/SP.

### 4. Interactive Logged Hours Leaderboard

- Standalone snapshot viewer page supporting link-sharing and direct browser bookmarking.
- Inline accordion-collapsible leaderboard rows showing which specific issues developers spent time on, their summaries, and exact hours logged (hyperlinked directly to Jira Browse).
- Inline search filtering to quickly find developers by name.
- Full table listing of all issues included in the snapshot.

### 5. Markdown Slideshow Presentations

- Create **Sprint Intro** (kickoffs) and **Sprint Retro** presentations.
- Clean fullscreen slideshow player with keyboard navigation, progress bars, and custom layouts.
- Pick between template visual designs: **Default (Dark/Modern)** or **Minimalist (Light/Simple)**.
- **Auto-Import Snapshots**: Select any saved Jira snapshot to automatically populate delivered SP, logged hours, Average SP/Hr, spillovers, and developer contributors.

---

## 🛠️ Technology Stack

- **Backend**: Go (Go 1.21+), `go-chi` (lightweight router), SQLite (using `go-sqlite3` in WAL mode for single-writer efficiency), and static file embedding.
- **Frontend**: Svelte 5 (reactive states and derived computations), SvelteKit (configured with static adapter for SPA routing), TypeScript, and Vite.
- **Design & Styling**: Custom Vanilla CSS with responsive variables, glassmorphic blur overlays, hover state transitions, and dark/light modes.

---

## 🚀 Setup & Installation

### Prerequisites

- [Go](https://go.dev/doc/install) (1.21 or higher)
- [Node.js](https://nodejs.org/en/download/) (Node 18+ & npm)
- CGO compiler tools (GCC) required by `go-sqlite3`

### 1. Install Dependencies

Clone the repository and install the frontend dependencies:

```bash
cd web
npm install
cd ..
```

### 2. Run in Development Mode

To run the Svelte development server (Vite) and Go backend concurrently, run the following Makefile target:

```bash
make dev
```

- **Frontend Dev Server**: http://localhost:5173
- **Backend Dev Server**: http://localhost:8080
- Login using the default root credentials:
  - **Username**: `root`
  - **Password**: `root`

### 3. Compile and Run in Production

To compile frontend static files, embed them in Go, and compile a single CGO binary:

```bash
make build
```

Start the compiled binary:

```bash
make run
```

The server will start running on port `8080`: http://localhost:8080. You can configure the port and SQLite database location using environment variables:

```bash
PORT=9000 SCRUMY_DB=mydata.db ./scrumy
```

---

## 🗄️ Database Schema Design

Scrumy runs on a SQLite database (`scrumy.db`) with auto-applied migrations defined in [db.go](file:///Users/rohanmallya/Development/prodalign/scrumy/internal/db/db.go).

- `users`: Stores user roles (`admin` | `user`), credentials, and bcrypt password hashes.
- `sessions`: Handles active user authentication cookies.
- `plans`: Stores the workspaces, plan admins, and Jira credentials.
- `plan_admins`: Junction table mapping plans to admin users.
- `capacity_plans`: Stores active, draft, or archived capacity profiles, loading factors, and productive hour variables.
- `team_members`: Lists team members and their utilization levels.
- `sprints`: Stores sprint scheduling dates.
- `sprint_leaves`: Records planned leave days per developer per sprint.
- `presentations`: StoresIntro/Retro slide configurations, template styles, epics, learnings, and imported metrics.
- `jira_snapshots`: Stores date-bounded metrics, issue details, and worklog breakdowns as serialized JSON data.
