<script lang="ts">
  import { page } from '$app/stores';
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { api, type Plan, type CapacityPlan, type Presentation, type JiraSnapshot, type JiraSettings, type Sprint } from '$lib/api';

  const planID = $derived($page.params.id);

  let plan = $state<Plan | null>(null);
  let capacityPlans = $state<CapacityPlan[]>([]);
  let presentations = $state<Presentation[]>([]);
  let snapshots = $state<JiraSnapshot[]>([]);
  let loading = $state(true);
  let error = $state('');

  let newAdminUsername = $state('');
  let addingAdmin = $state(false);

  // Tab switcher
  let activeTab = $state<'overview' | 'jira'>('overview');

  // Jira Config Form
  let jiraURL = $state('');
  let jiraUser = $state('');
  let jiraToken = $state('');
  let jiraJQL = $state('');
  let jiraSPField = $state('');
  let jiraTokenSet = $state(false);
  let testingConnection = $state(false);
  let testResult = $state<{ ok: boolean; message: string } | null>(null);
  let savingSettings = $state(false);

  // Snapshots Form / Detail Form
  let selectedSnapshot = $state<JiraSnapshot | null>(null);
  let showCreateSnapshotForm = $state(false);
  let newSnapshotName = $state('');
  let newSnapshotStart = $state('');
  let newSnapshotEnd = $state('');
  let creatingSnapshot = $state(false);
  let loadingSnapshotDetails = $state(false);

  onMount(async () => {
    try {
      const [p, c, pr, sn] = await Promise.all([
        api.plans.get(planID),
        api.capacity.list(planID),
        api.presentations.list(planID),
        api.jira.listSnapshots(planID),
      ]);
      plan = p;
      capacityPlans = c;
      presentations = pr;
      snapshots = sn;

      jiraURL = p.jira_url || '';
      jiraUser = p.jira_user || '';
      jiraJQL = p.jira_jql || '';
      jiraSPField = p.jira_sp_field || '';
      jiraTokenSet = !!p.jira_token_set;
    } catch (e: any) {
      error = e.message;
    } finally {
      loading = false;
    }
  });

  const intros = $derived(presentations.filter(p => p.type === 'intro'));
  const retros = $derived(presentations.filter(p => p.type === 'retro'));

  const allSprints = $derived(capacityPlans.flatMap(cp => cp.sprints || []));

  function prefillSnapshotFromSprint(sprintID: string) {
    const s = allSprints.find(sp => sp.id === sprintID);
    if (s) {
      newSnapshotName = `${s.name}`;
      newSnapshotStart = s.start_date;
      newSnapshotEnd = s.end_date;
    }
  }

  async function testJiraConnection() {
    testingConnection = true;
    testResult = null;
    try {
      await api.jira.testConnection(planID, {
        jira_url: jiraURL,
        jira_user: jiraUser,
        jira_token: jiraToken || undefined,
      });
      testResult = { ok: true, message: 'Connection successful!' };
    } catch (e: any) {
      testResult = { ok: false, message: e.message };
    } finally {
      testingConnection = false;
    }
  }

  async function saveJiraSettings() {
    savingSettings = true;
    try {
      await api.jira.saveSettings(planID, {
        jira_url: jiraURL.trim(),
        jira_user: jiraUser.trim(),
        jira_token: jiraToken ? jiraToken.trim() : undefined,
        jira_jql: jiraJQL.trim(),
        jira_sp_field: jiraSPField.trim(),
      });
      plan = await api.plans.get(planID);
      jiraTokenSet = !!plan.jira_token_set;
      jiraToken = ''; // Reset input after saving
      alert('Jira settings saved successfully!');
    } catch (e: any) {
      alert(`Failed to save settings: ${e.message}`);
    } finally {
      savingSettings = false;
    }
  }

  async function createSnapshot() {
    if (!newSnapshotName.trim() || !newSnapshotStart || !newSnapshotEnd) {
      alert('Please fill out Name, Start Date, and End Date');
      return;
    }
    creatingSnapshot = true;
    try {
      const snap = await api.jira.createSnapshot(planID, {
        name: newSnapshotName.trim(),
        start_date: newSnapshotStart,
        end_date: newSnapshotEnd,
      });
      snapshots = [snap, ...snapshots];
      selectedSnapshot = snap;
      showCreateSnapshotForm = false;
      newSnapshotName = '';
      newSnapshotStart = '';
      newSnapshotEnd = '';
    } catch (e: any) {
      alert(`Failed to create snapshot: ${e.message}`);
    } finally {
      creatingSnapshot = false;
    }
  }

  async function viewSnapshot(id: string) {
    loadingSnapshotDetails = true;
    try {
      selectedSnapshot = await api.jira.getSnapshot(planID, id);
    } catch (e: any) {
      alert(`Failed to load snapshot: ${e.message}`);
    } finally {
      loadingSnapshotDetails = false;
    }
  }

  async function refreshSnapshot(id: string) {
    if (!confirm('Re-fetch snapshot data from Jira?')) return;
    loadingSnapshotDetails = true;
    try {
      const updated = await api.jira.refreshSnapshot(planID, id);
      snapshots = snapshots.map(s => s.id === id ? updated : s);
      selectedSnapshot = updated;
    } catch (e: any) {
      alert(`Failed to refresh snapshot: ${e.message}`);
    } finally {
      loadingSnapshotDetails = false;
    }
  }

  async function deleteSnapshot(id: string) {
    if (!confirm('Are you sure you want to delete this snapshot?')) return;
    try {
      await api.jira.deleteSnapshot(planID, id);
      snapshots = snapshots.filter(s => s.id !== id);
      if (selectedSnapshot?.id === id) {
        selectedSnapshot = null;
      }
    } catch (e: any) {
      alert(`Failed to delete: ${e.message}`);
    }
  }

  async function createRetroFromSnapshot(snap: JiraSnapshot) {
    try {
      const title = `${snap.name} Retro`;
      const pres = await api.presentations.create(planID, {
        type: 'retro',
        template_id: 'default',
        title: title,
        sprint_name: snap.name,
      });

      const content = {
        previous_data: {
          total_sp_delivered: snap.data.totals.total_story_points,
          total_hours_logged: snap.data.totals.total_hours_logged,
          total_work_logs: snap.data.totals.total_work_logs,
          avg_hours_per_sp: snap.data.totals.avg_hours_per_sp,
          planned_sp: 0,
          executed_sp: snap.data.totals.total_story_points,
          spillovers: 0,
          total_epics_delivered: 0,
        },
        feedback: ['', ''],
        contributors: [],
        closing_text: '',
      };

      await api.presentations.update(planID, pres.id, {
        title: title,
        template_id: 'default',
        sprint_name: snap.name,
        content: content,
      });

      goto(`/plans/${planID}/presentations/${pres.id}/edit`);
    } catch (e: any) {
      alert(`Failed to create presentation: ${e.message}`);
    }
  }

  function statusBadge(s: string) {
    if (s === 'active') return 'badge-success';
    if (s === 'archived') return 'badge-default';
    return 'badge-warning';
  }

  async function deleteCapacity(cpID: string) {
    if (!confirm('Delete this capacity plan?')) return;
    await api.capacity.delete(planID, cpID);
    capacityPlans = await api.capacity.list(planID);
    plan = await api.plans.get(planID);
  }

  async function deletePres(presID: string) {
    if (!confirm('Delete this presentation?')) return;
    await api.presentations.delete(planID, presID);
    presentations = await api.presentations.list(planID);
    plan = await api.plans.get(planID);
  }

  async function addAdmin() {
    if (!newAdminUsername.trim()) return;
    addingAdmin = true;
    try {
      await api.plans.addAdmin(planID, newAdminUsername.trim());
      newAdminUsername = '';
      plan = await api.plans.get(planID);
    } catch (e: any) {
      alert(e.message);
    } finally {
      addingAdmin = false;
    }
  }

  async function removeAdmin(username: string) {
    if (!confirm(`Remove ${username} as admin?`)) return;
    try {
      const res = await fetch(`/api/plans/${planID}/admins`, {
        method: 'DELETE',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username })
      });
      if (!res.ok) throw new Error(await res.text());
      plan = await api.plans.get(planID);
    } catch (e: any) {
      alert(e.message);
    }
  }

  async function deletePlan() {
    if (!confirm('PERMANENTLY DELETE THIS ENTIRE PLAN AND ALL ITS DATA?')) return;
    try {
      await api.plans.delete(planID);
      goto('/');
    } catch (e: any) {
      alert(e.message);
    }
  }

  function formatRelDate(iso: string) {
    try {
      const d = new Date(iso);
      const now = new Date();
      const diff = Math.floor((now.getTime() - d.getTime()) / 86400000);
      if (diff === 0) return 'today';
      if (diff === 1) return 'yesterday';
      if (diff < 7) return `${diff} days ago`;
      return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
    } catch { return ''; }
  }
</script>

<svelte:head>
  <title>{plan?.name ?? 'Plan'} – Scrumy</title>
</svelte:head>

<div class="page-content fade-in">
  {#if loading}
    <div class="flex items-center gap-3" style="padding:40px 0">
      <span class="spinner"></span>
      <span class="text-muted">Loading plan…</span>
    </div>
  {:else if error}
    <div class="badge badge-danger" style="padding:12px 20px; border-radius:8px; font-size:14px; text-transform:none; letter-spacing:0; font-weight:400;">
      ⚠️ {error}
    </div>
  {:else}
    <!-- Header -->
    <div class="flex items-center justify-between" style="margin-bottom:20px;">
      <div>
        <h1 class="text-2xl font-bold">{plan?.name}</h1>
        <p class="text-sm text-muted" style="margin-top:2px;">Plan Workspace</p>
      </div>
      {#if plan?.is_admin}
        <button class="btn btn-outline-danger btn-sm" onclick={deletePlan}>Delete Plan</button>
      {/if}
    </div>

    <!-- Tabs switcher -->
    <div class="flex gap-4" style="border-bottom:1px solid var(--c-border); margin-bottom:24px; padding-bottom:2px;">
      <button 
        class="tab-btn {activeTab === 'overview' ? 'active' : ''}" 
        onclick={() => activeTab = 'overview'}
      >
        Sprint Overview
      </button>
      <button 
        class="tab-btn {activeTab === 'jira' ? 'active' : ''}" 
        onclick={() => activeTab = 'jira'}
      >
        Jira Integration & Snapshots
      </button>
    </div>

    {#if activeTab === 'overview'}
      <div style="display:grid; grid-template-columns: 1fr 1fr; gap:20px; align-items:start;">
        <!-- Capacity Plans -->
        <div class="card">
          <div class="card-header">
            <div class="flex items-center gap-2">
              <span style="font-size:18px;">📊</span>
              <h2 class="font-semibold" style="font-size:16px;">Capacity Plans</h2>
            </div>
            {#if plan?.is_admin}
              <a href="/plans/{planID}/capacity/new" class="btn btn-primary btn-sm">+ Create New</a>
            {/if}
          </div>

          <div>
            {#if capacityPlans.length === 0}
              <div class="empty-state">
                <p>No capacity plans yet.</p>
                {#if plan?.is_admin}
                  <a href="/plans/{planID}/capacity/new" class="btn btn-primary btn-sm" style="margin-top:12px;">Create First Plan</a>
                {/if}
              </div>
            {:else}
              {#each capacityPlans as cp (cp.id)}
                <a
                  href="/plans/{planID}/capacity/{cp.id}"
                  style="display:block; padding:16px 20px; border-bottom:1px solid var(--c-border); text-decoration:none; transition:background 150ms ease; cursor:pointer;"
                  onmouseover={e => (e.currentTarget as HTMLElement).style.background = 'var(--c-bg)'}
                  onmouseout={e => (e.currentTarget as HTMLElement).style.background = ''}
                  onfocus={e => (e.currentTarget as HTMLElement).style.background = 'var(--c-bg)'}
                  onblur={e => (e.currentTarget as HTMLElement).style.background = ''}
                >
                  <div class="flex justify-between items-center" style="margin-bottom:8px;">
                    <span class="font-semibold" style="font-size:14px;">{cp.name}</span>
                    <div class="flex items-center gap-2">
                      <span class="badge {statusBadge(cp.status)}">{cp.status}</span>
                      {#if plan?.is_admin}
                        <button
                          class="btn-icon"
                          style="color:var(--c-danger); font-size:14px;"
                          onclick={(e) => { e.preventDefault(); e.stopPropagation(); deleteCapacity(cp.id); }}
                          title="Delete"
                        >✕</button>
                      {/if}
                    </div>
                  </div>
                  <p class="text-xs text-muted">Updated {formatRelDate(cp.updated_at)}</p>
                </a>
              {/each}
            {/if}
          </div>
        </div>

        <!-- Presentations -->
        <div class="card">
          <div class="card-header">
            <div class="flex items-center gap-2">
              <span style="font-size:18px;">🎤</span>
              <h2 class="font-semibold" style="font-size:16px;">Presentations</h2>
            </div>
            {#if plan?.is_admin}
              <div class="flex gap-2">
                <a href="/plans/{planID}/presentations/new?type=intro" class="btn btn-secondary btn-sm">+ Intro</a>
                <a href="/plans/{planID}/presentations/new?type=retro" class="btn btn-secondary btn-sm">+ Retro</a>
              </div>
            {/if}
          </div>

          <div class="card-body" style="display:flex;flex-direction:column;gap:20px;padding-top:16px;">
            <!-- Sprint Intros -->
            <div>
              <p class="text-xs font-semibold text-muted" style="text-transform:uppercase;letter-spacing:0.06em;margin-bottom:10px;">Sprint Intros</p>
              {#if intros.length === 0}
                <p class="text-sm text-muted">No sprint intros yet.</p>
              {:else}
                <div style="display:flex;flex-direction:column;gap:8px;">
                  {#each intros as pres (pres.id)}
                    <div class="flex items-center justify-between" style="padding:10px 12px;background:var(--c-bg);border-radius:8px;">
                      <div class="flex items-center gap-3">
                        {#if pres.status === 'published'}
                          <a href="/plans/{planID}/presentations/{pres.id}/view"
                             style="width:30px;height:30px;background:var(--c-primary-lt);border-radius:8px;display:flex;align-items:center;justify-content:center;color:var(--c-primary);" title="Play">▶</a>
                        {:else}
                          <div style="width:30px;height:30px;background:var(--c-surface-2);border-radius:8px;display:flex;align-items:center;justify-content:center;color:var(--c-text-3);font-size:12px;">✎</div>
                        {/if}
                        <div>
                          <p class="font-medium" style="font-size:13px;">{pres.title}</p>
                          <p class="text-xs text-muted">{pres.status === 'published' ? 'Published' : 'Draft'} · {formatRelDate(pres.updated_at)}</p>
                        </div>
                      </div>
                      <div class="flex gap-1">
                        {#if plan?.is_admin}
                          <a href="/plans/{planID}/presentations/{pres.id}/edit" class="btn-icon" title="Edit" style="font-size:14px;">✎</a>
                          <button class="btn-icon" style="color:var(--c-danger);font-size:14px;" onclick={() => deletePres(pres.id)} title="Delete">✕</button>
                        {/if}
                      </div>
                    </div>
                  {/each}
                </div>
              {/if}
            </div>

            <div class="divider" style="margin:0;"></div>

            <!-- Sprint Retros -->
            <div>
              <p class="text-xs font-semibold text-muted" style="text-transform:uppercase;letter-spacing:0.06em;margin-bottom:10px;">Sprint Retros</p>
              {#if retros.length === 0}
                <p class="text-sm text-muted">No sprint retros yet.</p>
              {:else}
                <div style="display:flex;flex-direction:column;gap:8px;">
                  {#each retros as pres (pres.id)}
                    <div class="flex items-center justify-between" style="padding:10px 12px;background:var(--c-bg);border-radius:8px;">
                      <div class="flex items-center gap-3">
                        {#if pres.status === 'published'}
                          <a href="/plans/{planID}/presentations/{pres.id}/view"
                             style="width:30px;height:30px;background:var(--c-purple-lt);border-radius:8px;display:flex;align-items:center;justify-content:center;color:var(--c-purple);" title="Play">▶</a>
                        {:else}
                          <div style="width:30px;height:30px;background:var(--c-surface-2);border-radius:8px;display:flex;align-items:center;justify-content:center;color:var(--c-text-3);font-size:12px;">✎</div>
                        {/if}
                        <div>
                          <p class="font-medium" style="font-size:13px;">{pres.title}</p>
                          <p class="text-xs text-muted">{pres.status === 'published' ? 'Published' : 'Draft'} · {formatRelDate(pres.updated_at)}</p>
                        </div>
                      </div>
                      <div class="flex gap-1">
                        {#if plan?.is_admin}
                          <a href="/plans/{planID}/presentations/{pres.id}/edit" class="btn-icon" title="Edit" style="font-size:14px;">✎</a>
                          <button class="btn-icon" style="color:var(--c-danger);font-size:14px;" onclick={() => deletePres(pres.id)} title="Delete">✕</button>
                        {/if}
                      </div>
                    </div>
                  {/each}
                </div>
              {/if}
            </div>
          </div>
        </div>

        <!-- Admins Management -->
        {#if plan?.is_admin}
          <div class="card" style="grid-column: 1 / -1;">
            <div class="card-header">
              <h2 class="font-semibold">Manage Admins</h2>
            </div>
            <div class="card-body">
              <div class="flex gap-2" style="margin-bottom:16px;">
                <input class="input" bind:value={newAdminUsername} placeholder="Username" style="max-width:200px;" />
                <button class="btn btn-primary btn-sm" onclick={addAdmin} disabled={addingAdmin}>Add Admin</button>
              </div>
              <div class="flex flex-wrap gap-2">
                {#each plan.admins || [] as admin}
                  <div class="badge badge-default flex items-center gap-2" style="padding:6px 10px;">
                    {admin}
                    <button class="btn-icon" style="font-size:12px;" onclick={() => removeAdmin(admin)}>✕</button>
                  </div>
                {/each}
              </div>
            </div>
          </div>
        {/if}
      </div>
    {:else if activeTab === 'jira'}
      <!-- Jira snapshots and settings -->
      <div style="display:grid; grid-template-columns: 320px 1fr; gap:20px; align-items:start;">
        <!-- Left Side: Config and Snapshots List -->
        <div style="display:flex; flex-direction:column; gap:20px;">
          
          <!-- Jira Credentials Settings -->
          <div class="card">
            <div class="card-header">
              <div class="flex items-center gap-2">
                <span>⚙️</span>
                <h2 class="font-semibold" style="font-size:15px;">Jira Configuration</h2>
              </div>
            </div>
            <div class="card-body" style="display:flex; flex-direction:column; gap:12px; padding-top:16px;">
              <div class="form-group">
                <label class="label" style="font-size:11px;">Jira URL</label>
                <input class="input" bind:value={jiraURL} placeholder="https://your-domain.atlassian.net" disabled={!plan?.is_admin} />
              </div>
              <div class="form-group">
                <label class="label" style="font-size:11px;">Email / Username</label>
                <input class="input" bind:value={jiraUser} placeholder="email@company.com" disabled={!plan?.is_admin} />
              </div>
              <div class="form-group">
                <label class="label" style="font-size:11px;">API Token</label>
                <input type="password" class="input" bind:value={jiraToken} placeholder={jiraTokenSet ? "•••••••• (Configured)" : "Enter API Token"} disabled={!plan?.is_admin} />
              </div>
              <div class="form-group">
                <label class="label" style="font-size:11px;">Base JQL Filter</label>
                <input class="input" bind:value={jiraJQL} placeholder="project = PROJ AND statusCategory = Done" disabled={!plan?.is_admin} />
              </div>
              <div class="form-group">
                <label class="label" style="font-size:11px;">Story Points Field (Optional)</label>
                <input class="input" bind:value={jiraSPField} placeholder="e.g. customfield_10016" disabled={!plan?.is_admin} />
                <p class="text-xs text-muted" style="margin-top:2px; font-size:10px;">Left blank to auto-detect "Story Points"</p>
              </div>

              {#if plan?.is_admin}
                <div class="flex gap-2" style="margin-top:8px;">
                  <button class="btn btn-secondary btn-sm grow" onclick={testJiraConnection} disabled={testingConnection}>
                    {testingConnection ? 'Testing...' : 'Test Connection'}
                  </button>
                  <button class="btn btn-primary btn-sm grow" onclick={saveJiraSettings} disabled={savingSettings}>
                    {savingSettings ? 'Saving...' : 'Save Settings'}
                  </button>
                </div>
              {/if}

              {#if testResult}
                <div class="badge {testResult.ok ? 'badge-success' : 'badge-danger'}" style="margin-top:8px; padding:10px; border-radius:6px; font-size:11px; text-transform:none; letter-spacing:0; text-align:left; display:block; font-weight:400; line-height:1.4;">
                  {testResult.message}
                </div>
              {/if}
            </div>
          </div>

          <!-- Snapshots List -->
          <div class="card">
            <div class="card-header" style="display:flex; justify-content:space-between; align-items:center;">
              <div class="flex items-center gap-2">
                <span>📸</span>
                <h2 class="font-semibold" style="font-size:15px;">Jira Snapshots</h2>
              </div>
              {#if plan?.is_admin && plan?.jira_url}
                <button class="btn btn-primary btn-xs" onclick={() => showCreateSnapshotForm = !showCreateSnapshotForm}>
                  {showCreateSnapshotForm ? 'Cancel' : '+ Create'}
                </button>
              {/if}
            </div>
            
            <div class="card-body" style="display:flex; flex-direction:column; gap:8px; padding-top:12px;">
              {#if showCreateSnapshotForm}
                <div style="background:var(--c-bg); border-radius:8px; padding:12px; display:flex; flex-direction:column; gap:10px; border:1px solid var(--c-border); margin-bottom:10px;">
                  <p class="font-bold text-xs" style="color:var(--c-primary);">Create Date-Bounded Snapshot</p>
                  
                  {#if allSprints.length > 0}
                    <div class="form-group">
                      <label class="label" style="font-size:10px;">Pre-fill from Sprint</label>
                      <select class="input" style="padding:4px; font-size:11px;" onchange={e => prefillSnapshotFromSprint(e.currentTarget.value)}>
                        <option value="">-- Select a Sprint --</option>
                        {#each allSprints as s}
                          <option value={s.id}>{s.name} ({s.start_date} to {s.end_date})</option>
                        {/each}
                      </select>
                    </div>
                  {/if}

                  <div class="form-group">
                    <label class="label" style="font-size:10px;">Snapshot Name</label>
                    <input class="input" style="padding:6px; font-size:12px;" bind:value={newSnapshotName} placeholder="e.g. Sprint 24 Retro" />
                  </div>
                  <div class="form-group">
                    <label class="label" style="font-size:10px;">Start Date</label>
                    <input type="date" class="input" style="padding:4px; font-size:12px;" bind:value={newSnapshotStart} />
                  </div>
                  <div class="form-group">
                    <label class="label" style="font-size:10px;">End Date</label>
                    <input type="date" class="input" style="padding:4px; font-size:12px;" bind:value={newSnapshotEnd} />
                  </div>
                  
                  <button class="btn btn-primary btn-sm" onclick={createSnapshot} disabled={creatingSnapshot} style="margin-top:6px; font-size:11px;">
                    {creatingSnapshot ? 'Fetching & Saving...' : '✓ Generate'}
                  </button>
                </div>
              {/if}

              {#if snapshots.length === 0}
                <div class="empty-state" style="padding:20px 0;">
                  <p style="font-size:12px;">No snapshots yet.</p>
                </div>
              {:else}
                <div style="display:flex; flex-direction:column; gap:6px;">
                  {#each snapshots as s (s.id)}
                    <button
                      class="flex flex-col items-start gap-1"
                      style="width:100%; text-align:left; padding:12px 14px; border-radius:8px; border:1px solid {selectedSnapshot?.id === s.id ? 'var(--c-primary)' : 'var(--c-border)'}; background:{selectedSnapshot?.id === s.id ? 'var(--c-primary-lt)' : 'var(--c-bg)'}; cursor:pointer; transition:all 150ms ease;"
                      onclick={() => viewSnapshot(s.id)}
                    >
                      <span class="font-semibold text-sm" style="color:{selectedSnapshot?.id === s.id ? 'var(--c-primary)' : 'var(--c-text)'}">{s.name}</span>
                      <span class="text-xs text-muted">{s.start_date} to {s.end_date}</span>
                    </button>
                  {/each}
                </div>
              {/if}
            </div>
          </div>
        </div>

        <!-- Right Side: Detailed Analytics Viewer -->
        <div class="card" style="min-height:500px;">
          {#if loadingSnapshotDetails}
            <div class="flex items-center justify-center gap-3" style="padding:150px 0;">
              <span class="spinner"></span>
              <span class="text-muted">Analyzing Jira data…</span>
            </div>
          {:else if selectedSnapshot}
            <div class="card-header" style="display:flex; justify-content:space-between; align-items:center;">
              <div>
                <h2 class="font-bold text-lg">{selectedSnapshot.name}</h2>
                <p class="text-xs text-muted" style="margin-top:3px;">Analyzed: {selectedSnapshot.start_date} to {selectedSnapshot.end_date}</p>
              </div>
              <div class="flex gap-2">
                {#if plan?.is_admin}
                  <button class="btn btn-secondary btn-sm" onclick={() => refreshSnapshot(selectedSnapshot!.id)}>🔄 Refresh</button>
                  <button class="btn btn-outline-danger btn-sm" onclick={() => deleteSnapshot(selectedSnapshot!.id)}>✕ Delete</button>
                {/if}
                <button class="btn btn-primary btn-sm" onclick={() => createRetroFromSnapshot(selectedSnapshot!)}>🎤 Create Retro Presentation</button>
              </div>
            </div>

            <div class="card-body" style="display:flex; flex-direction:column; gap:24px; padding-top:20px;">
              <!-- Metrics Grid -->
              <div style="display:grid; grid-template-columns: repeat(4, 1fr); gap:16px;">
                <div style="background:var(--c-bg); padding:16px; border-radius:8px; border:1px solid var(--c-border);">
                  <p class="text-xs text-muted font-semibold" style="text-transform:uppercase; letter-spacing:0.05em; font-size:10px;">SP Burned</p>
                  <p class="text-2xl font-bold" style="margin-top:6px; color:var(--c-success);">{selectedSnapshot.data.totals.total_story_points}</p>
                </div>
                <div style="background:var(--c-bg); padding:16px; border-radius:8px; border:1px solid var(--c-border);">
                  <p class="text-xs text-muted font-semibold" style="text-transform:uppercase; letter-spacing:0.05em; font-size:10px;">Hours Logged</p>
                  <p class="text-2xl font-bold" style="margin-top:6px; color:var(--c-primary);">{selectedSnapshot.data.totals.total_hours_logged} <span class="text-sm font-normal text-muted">hrs</span></p>
                </div>
                <div style="background:var(--c-bg); padding:16px; border-radius:8px; border:1px solid var(--c-border);">
                  <p class="text-xs text-muted font-semibold" style="text-transform:uppercase; letter-spacing:0.05em; font-size:10px;">Worklogs Count</p>
                  <p class="text-2xl font-bold" style="margin-top:6px;">{selectedSnapshot.data.totals.total_work_logs}</p>
                </div>
                <div style="background:var(--c-bg); padding:16px; border-radius:8px; border:1px solid var(--c-border);">
                  <p class="text-xs text-muted font-semibold" style="text-transform:uppercase; letter-spacing:0.05em; font-size:10px;">Hours / SP</p>
                  <p class="text-2xl font-bold" style="margin-top:6px;">{selectedSnapshot.data.totals.avg_hours_per_sp} <span class="text-sm font-normal text-muted">avg</span></p>
                </div>
              </div>

              <!-- Leaderboard and Details -->
              <div style="display:grid; grid-template-columns: 1fr 1fr; gap:24px; align-items:start;">
                
                <!-- Logged Hours Leaderboard -->
                <div style="background:var(--c-bg); border-radius:8px; padding:16px; border:1px solid var(--c-border);">
                  <h3 class="font-semibold text-sm" style="margin-bottom:14px; display:flex; align-items:center; gap:8px;">
                    <span>🏆</span> Logged Hours Leaderboard
                  </h3>
                  {#if !selectedSnapshot.data.leaderboard || selectedSnapshot.data.leaderboard.length === 0}
                    <p class="text-xs text-muted" style="padding:16px 0; text-align:center;">No work logged within this period.</p>
                  {:else}
                    <div style="display:flex; flex-direction:column; gap:12px;">
                      {#each selectedSnapshot.data.leaderboard as entry, idx}
                        <div>
                          <div class="flex justify-between items-center text-xs" style="margin-bottom:6px;">
                            <span class="font-semibold">{idx + 1}. {entry.author_name}</span>
                            <span class="text-muted font-medium">{entry.hours_logged.toFixed(1)} hrs ({entry.percentage.toFixed(0)}%)</span>
                          </div>
                          <div style="width:100%; height:8px; background:var(--c-border); border-radius:4px; overflow:hidden;">
                            <div style="height:100%; width:{entry.percentage}%; background:var(--c-primary); border-radius:4px;"></div>
                          </div>
                        </div>
                      {/each}
                    </div>
                  {/if}
                </div>

                <!-- Info Box -->
                <div style="background:var(--c-bg); border-radius:8px; padding:16px; border:1px solid var(--c-border); height: 100%;">
                  <h3 class="font-semibold text-sm" style="margin-bottom:10px;">Snapshot Insights</h3>
                  <p class="text-xs text-muted" style="line-height:1.6; margin-bottom:12px;">
                    This analysis is generated dynamically by querying Jira Cloud. Sprints metrics include issues matching JQL that were set to a completed state category during the snapshot dates.
                  </p>
                  <p class="text-xs text-muted" style="line-height:1.6;">
                    The leaderboard highlights time logs created <strong>strictly within the start/end dates</strong> of the snapshot on the retrieved issues.
                  </p>
                  <div style="margin-top:16px; border-top:1px solid var(--c-border); padding-top:12px; font-size:11px; display:flex; flex-direction:column; gap:8px;">
                    <div class="flex justify-between">
                      <span class="text-muted">Closed Issues count:</span>
                      <span class="font-semibold">{selectedSnapshot.data.issues?.length || 0}</span>
                    </div>
                    <div class="flex justify-between">
                      <span class="text-muted">Total story points delivered:</span>
                      <span class="font-semibold text-success">{selectedSnapshot.data.totals.total_story_points}</span>
                    </div>
                  </div>
                </div>
              </div>

              <!-- Issue List Table -->
              <div>
                <h3 class="font-semibold text-sm" style="margin-bottom:12px; display:flex; align-items:center; gap:8px;">
                  <span>📋</span> Issues in Snapshot ({selectedSnapshot.data.issues?.length || 0})
                </h3>
                {#if !selectedSnapshot.data.issues || selectedSnapshot.data.issues.length === 0}
                  <p class="text-xs text-muted" style="padding:20px 0; text-align:center; background:var(--c-bg); border-radius:8px; border:1px dashed var(--c-border);">No closed issues found in this range.</p>
                {:else}
                  <div style="max-height:300px; overflow-y:auto; border:1px solid var(--c-border); border-radius:8px;">
                    <table style="width:100%; border-collapse:collapse; text-align:left; font-size:12px;">
                      <thead>
                        <tr style="border-bottom:1px solid var(--c-border); background:var(--c-bg); color:var(--c-text-3); position:sticky; top:0; z-index:1;">
                          <th style="padding:10px; font-weight:600;">Key</th>
                          <th style="padding:10px; font-weight:600;">Summary</th>
                          <th style="padding:10px; font-weight:600;">Status</th>
                          <th style="padding:10px; font-weight:600; text-align:right;">Story Points</th>
                          <th style="padding:10px; font-weight:600; text-align:right;">Logged Hours</th>
                        </tr>
                      </thead>
                      <tbody>
                        {#each selectedSnapshot.data.issues as issue}
                          <tr style="border-bottom:1px solid var(--c-border); background:var(--c-surface);">
                            <td style="padding:10px; font-weight:600; white-space:nowrap;">
                              {#if plan?.jira_url}
                                <a href="{plan.jira_url}/browse/{issue.key}" target="_blank" class="text-primary" style="text-decoration:none;">{issue.key}</a>
                              {:else}
                                {issue.key}
                              {/if}
                            </td>
                            <td style="padding:10px; max-width:260px; overflow:hidden; text-overflow:ellipsis; white-space:nowrap;" title={issue.summary}>{issue.summary}</td>
                            <td style="padding:10px; white-space:nowrap;">
                              <span class="badge badge-default" style="font-size:10px; padding:2px 6px;">{issue.status}</span>
                            </td>
                            <td style="padding:10px; text-align:right; font-weight:500;">{issue.story_points > 0 ? issue.story_points : '—'}</td>
                            <td style="padding:10px; text-align:right; font-weight:500; color:var(--c-primary);">{issue.time_spent_hours.toFixed(1)} hrs</td>
                          </tr>
                        {/each}
                      </tbody>
                    </table>
                  </div>
                {/if}
              </div>
            </div>
          {:else}
            <div style="display:flex; flex-direction:column; align-items:center; justify-content:center; height:100%; color:var(--c-text-3); padding:100px 24px; text-align:center;">
              <span style="font-size:54px; margin-bottom:16px;">📊</span>
              <h3 class="font-bold text-lg" style="color:var(--c-text-2);">No Snapshot Selected</h3>
              <p class="text-xs text-muted" style="max-width:340px; margin-top:8px; line-height:1.5;">
                Choose an existing snapshot on the left, or create a new date-bounded snapshot to pull sprint metrics and worklog statistics from Jira Cloud.
              </p>
            </div>
          {/if}
        </div>
      </div>
    {/if}
  {/if}
</div>

<style>
  .btn-outline-danger {
    background: transparent;
    border: 1px solid var(--c-danger);
    color: var(--c-danger);
  }
  .btn-outline-danger:hover {
    background: var(--c-danger-lt);
  }

  .tab-btn {
    background: transparent;
    border: none;
    padding: 8px 16px;
    font-size: 14px;
    font-weight: 500;
    color: var(--c-text-3);
    cursor: pointer;
    border-bottom: 2px solid transparent;
    transition: all 150ms ease;
    outline: none;
  }
  .tab-btn:hover {
    color: var(--c-text-2);
  }
  .tab-btn.active {
    color: var(--c-primary);
    border-bottom-color: var(--c-primary);
    font-weight: 600;
  }
  
  .btn-xs {
    padding: 4px 8px;
    font-size: 11px;
    border-radius: 4px;
  }
</style>
