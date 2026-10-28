const BASE = '/api';

async function req<T>(method: string, path: string, body?: unknown): Promise<T> {
	const res = await fetch(`${BASE}${path}`, {
		method,
		headers: body ? { 'Content-Type': 'application/json' } : {},
		body: body ? JSON.stringify(body) : undefined,
	});
	if (!res.ok) {
		const err = await res.json().catch(() => ({ error: res.statusText }));
		throw new Error(err.error || res.statusText);
	}
	return res.json();
}

const get = <T>(path: string) => req<T>('GET', path);
const post = <T>(path: string, body?: unknown) => req<T>('POST', path, body);
const put = <T>(path: string, body?: unknown) => req<T>('PUT', path, body);
const del = <T>(path: string, body?: unknown) => req<T>('DELETE', path, body);

// ─── Types ────────────────────────────────────────────────────────────────────
export interface User {
	id: string;
	username: string;
	role: string;
	created_at: string;
}

export interface ManagedUser {
	id: string;
	username: string;
	role: string;
	created_at: string;
	is_super_admin?: boolean;
}

export interface UserSearchResult {
	id: string;
	username: string;
}

export interface AuthRequest {
	username: string;
	password?: string;
}

export interface Team {
	id: string;
	name: string;
	created_at: string;
	updated_at: string;
	capacity_plan_count: number;
	presentation_count: number;
	admins?: string[];
	is_admin?: boolean;
	jira_url?: string;
	jira_user?: string;
	jira_jql?: string;
	jira_sp_field?: string;
	jira_token_set?: boolean;
	jira_insecure?: boolean;
}

// Plan is a deprecated alias for Team, kept for backward compatibility.
export type Plan = Team;

export interface CapacityPlan {
	id: string;
	team_id: string;
	name: string;
	status: string;
	hours_per_sp: number;
	productive_hours: number;
	loading_factor: number;
	created_at: string;
	updated_at: string;
	members: TeamMember[];
	sprints: Sprint[];
}

export interface TeamMember {
	id: string;
	capacity_plan_id: string;
	name: string;
	role: string;
	utilization_pct: number;
	sort_order: number;
}

export interface Sprint {
	id: string;
	capacity_plan_id: string;
	name: string;
	start_date: string;
	end_date: string;
	sort_order: number;
	leaves: SprintLeave[];
}

export interface SprintLeave {
	id: string;
	sprint_id: string;
	member_id: string;
	leaves: number;
}

export interface SprintSummary {
	sprint_id: string;
	sprint_name: string;
	gross_person_days: number;
	leaves: number;
	net_person_days: number;
	loaded_person_days: number;
	target_sp: number;
	thin_target: number;
	stretch_target: number;
}

export interface CapacitySummary {
	sprints: SprintSummary[];
	totals: SprintSummary;
}

export interface Presentation {
	id: string;
	team_id: string;
	type: 'intro' | 'retro';
	template_id: string;
	title: string;
	status: 'draft' | 'published';
	sprint_name: string;
	created_at: string;
	updated_at: string;
	content?: IntroContent | RetroContent;
}

export interface PreviousData {
	total_sp_delivered: number;
	total_hours_logged: number;
	total_work_logs: number;
	avg_hours_per_sp: number;
	planned_sp: number;
	executed_sp: number;
	spillovers: number;
	total_epics_delivered: number;
}

export interface Epic {
	id: string;
	title: string;
	summary: string;
	why_needed: string;
	when_doing: string;
	audience: string;
	total_sp: number;
}

export interface Learning {
	title: string;
	content: string;
	tags: string[];
}

export interface Change {
	title: string;
	content: string;
	tags: string[];
}

export interface Contributor {
	name: string;
	contribution: string;
}

export interface IntroContent {
	learnings: Learning[];
	changes: Change[];
	previous_data: PreviousData;
	epics: Epic[];
	contributors?: Contributor[];
	closing_text?: string;
}

export interface RetroContent {
	previous_data: PreviousData;
	feedback: string[];
	contributors?: Contributor[];
	closing_text?: string;
}

export interface JiraIssue {
	key: string;
	summary: string;
	status: string;
	status_category_key?: string;
	story_points: number;
	time_spent_hours: number;
	time_spent_hours_all?: number;
	status_category_changed_date: string;
	is_unplanned?: boolean;
}

export interface JiraTotals {
	total_story_points: number;
	total_hours_logged: number;
	total_work_logs: number;
	avg_hours_per_sp: number;
}

export interface JiraWorklogItem {
	issue_key: string;
	issue_summary: string;
	hours_logged: number;
}

export interface JiraLeaderboardEntry {
	author_name: string;
	hours_logged: number;
	percentage: number;
	worklogs?: JiraWorklogItem[];
}

export interface JiraSnapshotData {
	issues: JiraIssue[];
	totals: JiraTotals;
	leaderboard: JiraLeaderboardEntry[];
	totals_all?: JiraTotals;
	leaderboard_all?: JiraLeaderboardEntry[];
}

export interface JiraUserRefreshDelta {
	author_name: string;
	prev_hours_logged: number;
	new_hours_logged: number;
	hours_delta: number;
}

export interface JiraSnapshotRefresh {
	id: string;
	snapshot_id: string;
	refreshed_at: string;
	prev_story_points: number;
	new_story_points: number;
	prev_hours_logged: number;
	new_hours_logged: number;
	prev_worklogs_count: number;
	new_worklogs_count: number;
	prev_issues_count: number;
	new_issues_count: number;
	user_deltas?: JiraUserRefreshDelta[];
	user_deltas_all?: JiraUserRefreshDelta[];
}

export interface JiraSnapshot {
	id: string;
	team_id: string;
	name: string;
	start_date: string;
	end_date: string;
	all_worklogs: boolean;
	data: JiraSnapshotData;
	created_at: string;
	updated_at: string;
	refresh_history?: JiraSnapshotRefresh[];
}

export interface JiraSettings {
	jira_url: string;
	jira_user: string;
	jira_token?: string;
	jira_jql: string;
	jira_sp_field: string;
	jira_insecure?: boolean;
}

export interface SnapshotRefreshResult {
	team_id: string;
	team_name: string;
	snapshot_id: string;
	snapshot_name: string;
	status: 'refreshed' | 'skipped' | 'failed';
	error?: string;
}

export interface BulkRefreshResponse {
	total: number;
	refreshed: number;
	skipped: number;
	failed: number;
	results: SnapshotRefreshResult[];
}

export interface BulkAddAdminResponse {
	username: string;
	user_id: string;
	total_teams: number;
	added: number;
	message?: string;
}

// ─── API calls ────────────────────────────────────────────────────────────────
export const api = {
	teams: {
		list: () => get<Team[]>('/teams'),
		create: (name: string) => post<Team>('/teams', { name }),
		get: (id: string) => get<Team>(`/teams/${id}`),
		update: (id: string, name: string) => put<Team>(`/teams/${id}`, { name }),
		delete: (id: string) => del<{ deleted: boolean }>(`/teams/${id}`),
		addAdmin: (id: string, username: string, options?: { all_teams?: boolean }) =>
			post<{ added: boolean; total_teams?: number; added_count?: number }>(`/teams/${id}/admins`, { username, all_teams: options?.all_teams }),
		removeAdmin: (id: string, username: string) => del<{ removed: boolean }>(`/teams/${id}/admins`, { username }),
	},
	admin: {
		refreshAllSnapshots: () => post<BulkRefreshResponse>('/admin/snapshots/refresh-all'),
		addAdminToAllTeams: (username: string) => post<BulkAddAdminResponse>('/admin/teams/add-admin', { username }),
	},
	capacity: {
		list: (teamID: string) => get<CapacityPlan[]>(`/teams/${teamID}/capacity`),
		create: (teamID: string, body: Partial<CapacityPlan>) => post<CapacityPlan>(`/teams/${teamID}/capacity`, body),
		get: (teamID: string, cpID: string) => get<CapacityPlan>(`/teams/${teamID}/capacity/${cpID}`),
		update: (teamID: string, cpID: string, body: Partial<CapacityPlan>) => put<CapacityPlan>(`/teams/${teamID}/capacity/${cpID}`, body),
		delete: (teamID: string, cpID: string) => del<{ deleted: boolean }>(`/teams/${teamID}/capacity/${cpID}`),
		summary: (teamID: string, cpID: string) => get<CapacitySummary>(`/teams/${teamID}/capacity/${cpID}/summary`),
		addMember: (teamID: string, cpID: string, body: Partial<TeamMember>) => post<CapacityPlan>(`/teams/${teamID}/capacity/${cpID}/members`, body),
		updateMember: (teamID: string, cpID: string, mID: string, body: Partial<TeamMember>) => put<CapacityPlan>(`/teams/${teamID}/capacity/${cpID}/members/${mID}`, body),
		deleteMember: (teamID: string, cpID: string, mID: string) => del<CapacityPlan>(`/teams/${teamID}/capacity/${cpID}/members/${mID}`),
		addSprint: (teamID: string, cpID: string, body: Partial<Sprint>) => post<CapacityPlan>(`/teams/${teamID}/capacity/${cpID}/sprints`, body),
		updateSprint: (teamID: string, cpID: string, sID: string, body: Partial<Sprint>) => put<CapacityPlan>(`/teams/${teamID}/capacity/${cpID}/sprints/${sID}`, body),
		deleteSprint: (teamID: string, cpID: string, sID: string) => del<CapacityPlan>(`/teams/${teamID}/capacity/${cpID}/sprints/${sID}`),
		upsertLeave: (teamID: string, cpID: string, sID: string, member_id: string, leaves: number) =>
			post<CapacityPlan>(`/teams/${teamID}/capacity/${cpID}/sprints/${sID}/leaves`, { member_id, leaves }),
	},
	presentations: {
		list: (teamID: string) => get<Presentation[]>(`/teams/${teamID}/presentations`),
		create: (teamID: string, body: { type: string; template_id: string; title: string; sprint_name: string }) => post<Presentation>(`/teams/${teamID}/presentations`, body),
		get: (teamID: string, presID: string) => get<Presentation>(`/teams/${teamID}/presentations/${presID}`),
		update: (teamID: string, presID: string, body: Partial<Presentation> & { content?: unknown }) => put<Presentation>(`/teams/${teamID}/presentations/${presID}`, body),
		delete: (teamID: string, presID: string) => del<{ deleted: boolean }>(`/teams/${teamID}/presentations/${presID}`),
		publish: (teamID: string, presID: string) => post<Presentation>(`/teams/${teamID}/presentations/${presID}/publish`),
		unpublish: (teamID: string, presID: string) => post<Presentation>(`/teams/${teamID}/presentations/${presID}/unpublish`),
		addFeedback: (teamID: string, presID: string, item: string) => post<Presentation>(`/teams/${teamID}/presentations/${presID}/feedback`, { item }),
	},
	jira: {
		saveSettings: (teamID: string, settings: JiraSettings) => put<{ updated: boolean }>(`/teams/${teamID}/jira/settings`, settings),
		testConnection: (teamID: string, settings: Partial<JiraSettings>) => post<{ ok: boolean }>(`/teams/${teamID}/jira/test-connection`, settings),
		createSnapshot: (teamID: string, body: { name: string; start_date: string; end_date: string; all_worklogs: boolean }) => post<JiraSnapshot>(`/teams/${teamID}/jira/snapshots`, body),
		listSnapshots: (teamID: string) => get<JiraSnapshot[]>(`/teams/${teamID}/jira/snapshots`),
		getSnapshot: (teamID: string, id: string) => get<JiraSnapshot>(`/teams/${teamID}/jira/snapshots/${id}`),
		updateSnapshot: (teamID: string, id: string, body: { name: string; start_date: string; end_date: string }) => put<JiraSnapshot>(`/teams/${teamID}/jira/snapshots/${id}`, body),
		refreshSnapshot: (teamID: string, id: string) => post<JiraSnapshot>(`/teams/${teamID}/jira/snapshots/${id}/refresh`),
		deleteSnapshot: (teamID: string, id: string) => del<{ deleted: boolean }>(`/teams/${teamID}/jira/snapshots/${id}`),
		toggleIssuePlanned: (teamID: string, snapshotID: string, issueKey: string, is_unplanned?: boolean) =>
			post<JiraSnapshot>(`/teams/${teamID}/jira/snapshots/${snapshotID}/issues/${issueKey}/toggle-planned`, { is_unplanned }),
	},
	auth: {
		login: (body: AuthRequest) => post<User>('/auth/login', body),
		register: (body: AuthRequest) => post<{ id: string }>('/auth/register', body),
		logout: () => post<{ message: string }>('/auth/logout'),
		me: () => get<User>('/auth/me'),
		changePassword: (body: { current_password: string; new_password: string }) =>
			post<{ message: string }>('/auth/change-password', body),
	},
	users: {
		list: () => get<ManagedUser[]>('/users'),
		search: (q: string) => get<UserSearchResult[]>(`/users/search?q=${encodeURIComponent(q)}`),
		create: (body: { username: string; password: string; role?: string }) =>
			post<ManagedUser>('/users', body),
		updateRole: (id: string, role: string) => put<ManagedUser>(`/users/${id}`, { role }),
		resetPassword: (id: string, new_password: string) =>
			put<{ message: string }>(`/users/${id}/password`, { new_password }),
		delete: (id: string) => del<{ deleted: boolean }>(`/users/${id}`),
	},
};
