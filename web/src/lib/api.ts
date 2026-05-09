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
const del = <T>(path: string) => req<T>('DELETE', path);

// ─── Types ────────────────────────────────────────────────────────────────────
export interface Plan {
	id: string;
	name: string;
	created_at: string;
	updated_at: string;
	capacity_plan_count: number;
	presentation_count: number;
}

export interface CapacityPlan {
	id: string;
	plan_id: string;
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
	plan_id: string;
	type: 'intro' | 'retro';
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
}

export interface RetroContent {
	previous_data: PreviousData;
	feedback: string[];
	contributors?: Contributor[];
}

// ─── API calls ────────────────────────────────────────────────────────────────
export const api = {
	plans: {
		list: () => get<Plan[]>('/plans'),
		create: (name: string) => post<Plan>('/plans', { name }),
		get: (id: string) => get<Plan>(`/plans/${id}`),
		update: (id: string, name: string) => put<Plan>(`/plans/${id}`, { name }),
		delete: (id: string) => del<{ deleted: boolean }>(`/plans/${id}`),
	},
	capacity: {
		list: (planID: string) => get<CapacityPlan[]>(`/plans/${planID}/capacity`),
		create: (planID: string, body: Partial<CapacityPlan>) => post<CapacityPlan>(`/plans/${planID}/capacity`, body),
		get: (planID: string, cpID: string) => get<CapacityPlan>(`/plans/${planID}/capacity/${cpID}`),
		update: (planID: string, cpID: string, body: Partial<CapacityPlan>) => put<CapacityPlan>(`/plans/${planID}/capacity/${cpID}`, body),
		delete: (planID: string, cpID: string) => del<{ deleted: boolean }>(`/plans/${planID}/capacity/${cpID}`),
		summary: (planID: string, cpID: string) => get<CapacitySummary>(`/plans/${planID}/capacity/${cpID}/summary`),
		addMember: (planID: string, cpID: string, body: Partial<TeamMember>) => post<CapacityPlan>(`/plans/${planID}/capacity/${cpID}/members`, body),
		updateMember: (planID: string, cpID: string, mID: string, body: Partial<TeamMember>) => put<CapacityPlan>(`/plans/${planID}/capacity/${cpID}/members/${mID}`, body),
		deleteMember: (planID: string, cpID: string, mID: string) => del<CapacityPlan>(`/plans/${planID}/capacity/${cpID}/members/${mID}`),
		addSprint: (planID: string, cpID: string, body: Partial<Sprint>) => post<CapacityPlan>(`/plans/${planID}/capacity/${cpID}/sprints`, body),
		updateSprint: (planID: string, cpID: string, sID: string, body: Partial<Sprint>) => put<CapacityPlan>(`/plans/${planID}/capacity/${cpID}/sprints/${sID}`, body),
		deleteSprint: (planID: string, cpID: string, sID: string) => del<CapacityPlan>(`/plans/${planID}/capacity/${cpID}/sprints/${sID}`),
		upsertLeave: (planID: string, cpID: string, sID: string, member_id: string, leaves: number) =>
			post<CapacityPlan>(`/plans/${planID}/capacity/${cpID}/sprints/${sID}/leaves`, { member_id, leaves }),
	},
	presentations: {
		list: (planID: string) => get<Presentation[]>(`/plans/${planID}/presentations`),
		create: (planID: string, body: { type: string; title: string; sprint_name: string }) => post<Presentation>(`/plans/${planID}/presentations`, body),
		get: (planID: string, presID: string) => get<Presentation>(`/plans/${planID}/presentations/${presID}`),
		update: (planID: string, presID: string, body: Partial<Presentation> & { content?: unknown }) => put<Presentation>(`/plans/${planID}/presentations/${presID}`, body),
		delete: (planID: string, presID: string) => del<{ deleted: boolean }>(`/plans/${planID}/presentations/${presID}`),
		publish: (planID: string, presID: string) => post<Presentation>(`/plans/${planID}/presentations/${presID}/publish`),
		unpublish: (planID: string, presID: string) => post<Presentation>(`/plans/${planID}/presentations/${presID}/unpublish`),
		addFeedback: (planID: string, presID: string, item: string) => post<Presentation>(`/plans/${planID}/presentations/${presID}/feedback`, { item }),
	},
};
