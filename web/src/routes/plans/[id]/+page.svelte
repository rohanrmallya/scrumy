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
  let jiraInsecure = $state(false);
  let jiraTokenSet = $state(false);
  let testingConnection = $state(false);
  let testResult = $state<{ ok: boolean; message: string } | null>(null);
  let savingSettings = $state(false);

  // Snapshots Form
  let showCreateSnapshotForm = $state(false);
  let newSnapshotName = $state('');
  let newSnapshotStart = $state('');
  let newSnapshotEnd = $state('');
  let newSnapshotAllWorklogs = $state(false);
  let creatingSnapshot = $state(false);

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
      jiraInsecure = !!p.jira_insecure;
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
        jira_insecure: jiraInsecure,
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
        jira_insecure: jiraInsecure,
      });
      plan = await api.plans.get(planID);
      jiraTokenSet = !!plan.jira_token_set;
      jiraInsecure = !!plan.jira_insecure;
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
        all_worklogs: newSnapshotAllWorklogs,
      });
      snapshots = [snap, ...snapshots];
      showCreateSnapshotForm = false;
      newSnapshotName = '';
      newSnapshotStart = '';
      newSnapshotEnd = '';
      newSnapshotAllWorklogs = false;
      
      // Navigate to the dedicated snapshot page!
      goto(`/plans/${planID}/snapshots/${snap.id}`);
    } catch (e: any) {
      alert(`Failed to create snapshot: ${e.message}`);
    } finally {
      creatingSnapshot = false;
    }
  }

  async function deleteSnapshot(id: string) {
    if (!confirm('Are you sure you want to delete this snapshot?')) return;
    try {
      await api.jira.deleteSnapshot(planID, id);
      snapshots = snapshots.filter(s => s.id !== id);
    } catch (e: any) {
      alert(`Failed to delete: ${e.message}`);
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
        Jira Integration
      </button>
    </div>

    {#if activeTab === 'overview'}
      <div style="display:grid; grid-template-columns: repeat(auto-fit, minmax(320px, 1fr)); gap:20px; align-items:start;">
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

        <!-- Jira Snapshots -->
        <div class="card">
          <div class="card-header" style="display:flex; justify-content:space-between; align-items:center;">
            <div class="flex items-center gap-2">
              <span style="font-size:18px;">📸</span>
              <h2 class="font-semibold" style="font-size:16px;">Jira Snapshots</h2>
            </div>
            {#if plan?.is_admin && plan?.jira_url}
              <button class="btn btn-primary btn-xs" onclick={() => showCreateSnapshotForm = !showCreateSnapshotForm}>
                {showCreateSnapshotForm ? 'Cancel' : '+ Create'}
              </button>
            {/if}
          </div>

          <div>
            {#if showCreateSnapshotForm}
              <div style="background:var(--c-bg); padding:16px; display:flex; flex-direction:column; gap:12px; border-bottom:1px solid var(--c-border);">
                <p class="font-bold text-xs" style="color:var(--c-primary); margin-bottom: 4px;">Generate Date-Bounded Snapshot</p>
                
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
                <div class="form-group flex items-center gap-2" style="margin-top: 4px; margin-bottom: 6px;">
                  <input type="checkbox" id="newSnapshotAllWorklogs" bind:checked={newSnapshotAllWorklogs} />
                  <label for="newSnapshotAllWorklogs" class="label" style="font-size:11px; margin: 0; cursor: pointer;">Include all worklogs (no date filter)</label>
                </div>
                
                <button class="btn btn-primary btn-sm" onclick={createSnapshot} disabled={creatingSnapshot} style="margin-top:6px; font-size:11px; width: 100%;">
                  {creatingSnapshot ? 'Generating...' : '✓ Generate'}
                </button>
              </div>
            {/if}

            {#if snapshots.length === 0}
              <div class="empty-state">
                <p>No snapshots generated yet.</p>
              </div>
            {:else}
              {#each snapshots as s (s.id)}
                <a
                  href="/plans/{planID}/snapshots/{s.id}"
                  style="display:block; padding:16px 20px; border-bottom:1px solid var(--c-border); text-decoration:none; transition:background 150ms ease; cursor:pointer;"
                  onmouseover={e => (e.currentTarget as HTMLElement).style.background = 'var(--c-bg)'}
                  onmouseout={e => (e.currentTarget as HTMLElement).style.background = ''}
                  onfocus={e => (e.currentTarget as HTMLElement).style.background = 'var(--c-bg)'}
                  onblur={e => (e.currentTarget as HTMLElement).style.background = ''}
                >
                  <div class="flex justify-between items-center" style="margin-bottom:8px;">
                    <span class="font-semibold" style="font-size:14px; color:var(--c-primary);">{s.name}</span>
                    {#if plan?.is_admin}
                      <button
                        class="btn-icon"
                        style="color:var(--c-danger); font-size:14px;"
                        onclick={(e) => { e.preventDefault(); e.stopPropagation(); deleteSnapshot(s.id); }}
                        title="Delete"
                      >✕</button>
                    {/if}
                  </div>
                  <p class="text-xs text-muted">{s.start_date} to {s.end_date}</p>
                </a>
              {/each}
            {/if}
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
      <!-- Jira settings -->
      <div style="max-width:600px; margin:0 auto;">
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
            <div class="form-group flex items-center gap-2" style="margin-top: 4px;">
              <input type="checkbox" id="jiraInsecure" bind:checked={jiraInsecure} disabled={!plan?.is_admin} />
              <label for="jiraInsecure" class="label" style="font-size:11px; margin: 0; cursor: pointer;">Skip TLS Verification (Insecure)</label>
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

  @keyframes slideUp {
    from {
      opacity: 0;
      transform: translateY(20px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }
</style>
