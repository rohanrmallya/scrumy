<script lang="ts">
  import { page } from '$app/stores';
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { api, type CapacityPlan, type TeamMember, type Sprint, type CapacitySummary } from '$lib/api';

  const teamID = $derived($page.params.id);
  const cpID = $derived($page.params.cpid);
  const isNew = $derived(cpID === 'new');

  let cp = $state<CapacityPlan>({
    id: '', team_id: teamID, name: '', status: 'draft',
    hours_per_sp: 8, productive_hours: 6, loading_factor: 0.75,
    created_at: '', updated_at: '', members: [], sprints: []
  });
  let summary = $state<CapacitySummary | null>(null);
  let loading = $state(!isNew);
  let saving = $state(false);
  let summaryLoading = $state(false);
  let error = $state('');
  let expandedSprints = $state<Set<string>>(new Set());

  // Add person form
  let newMemberName = $state('');
  let newMemberRole = $state('');
  let newMemberUtil = $state(100);

  // Add sprint form  
  let newSprintName = $state('');
  let newSprintStart = $state('');
  let newSprintEnd = $state('');
  let showAddSprint = $state(false);
  let showAddMember = $state(false);

  // Percentage display helper for loading factor (stored internally as decimal e.g. 0.75)
  let loadingFactorPct = $derived(Math.round((cp.loading_factor || 0.75) * 100));

  function updateLoadingFactorPct(val: number) {
    if (isNaN(val) || val <= 0) val = 75;
    if (val > 100) val = 100;
    cp.loading_factor = val / 100.0;
  }
  // Edit sprint state
  let editingSprintId = $state<string | null>(null);
  let editSprintName = $state('');
  let editSprintStart = $state('');
  let editSprintEnd = $state('');
  let savingSprint = $state(false);

  onMount(async () => {
    if (!isNew) {
      try {
        cp = await api.capacity.get(teamID, cpID);
        // Expand first sprint by default
        if (cp.sprints?.length) expandedSprints = new Set([cp.sprints[0].id]);
        await refreshSummary();
      } catch (e: any) { error = e.message; }
      finally { loading = false; }
    }
  });

  async function refreshSummary() {
    if (isNew || !cp.id) return;
    summaryLoading = true;
    try {
      summary = await api.capacity.summary(teamID, cp.id);
    } catch {}
    finally { summaryLoading = false; }
  }

  async function saveVelocity() {
    if (!cp.id) return;
    await api.capacity.update(teamID, cp.id, {
      name: cp.name, status: cp.status,
      hours_per_sp: cp.hours_per_sp,
      productive_hours: cp.productive_hours,
      loading_factor: cp.loading_factor
    });
    await refreshSummary();
  }

  async function finalize() {
    saving = true;
    try {
      if (isNew) {
        const created = await api.capacity.create(teamID, {
          name: cp.name || 'New Capacity Plan',
          hours_per_sp: cp.hours_per_sp,
          productive_hours: cp.productive_hours,
          loading_factor: cp.loading_factor
        });
        goto(`/teams/${teamID}/capacity/${created.id}`);
      } else {
        await api.capacity.update(teamID, cp.id, { ...cp, status: 'active' });
        goto(`/teams/${teamID}`);
      }
    } catch (e: any) {
      error = e.message;
    } finally {
      saving = false;
    }
  }

  async function addMember() {
    console.log("Adding member: ", teamID, cp.id, newMemberName, newMemberRole, newMemberUtil);
    if (!newMemberName.trim() || !cp.id) return;
    let util = Number(newMemberUtil);
    if (isNaN(util) || util < 0) util = 0;
    if (util > 100) util = 100;
    cp = await api.capacity.addMember(teamID, cp.id, {
      name: newMemberName.trim(), role: newMemberRole.trim(), utilization_pct: util
    });
    newMemberName = ''; newMemberRole = ''; newMemberUtil = 100;
    showAddMember = false;
    await refreshSummary();
  }

  async function updateMemberUtil(m: TeamMember, val: number) {
    if (isNaN(val) || val < 0) val = 0;
    if (val > 100) val = 100;
    m.utilization_pct = val;
    await api.capacity.updateMember(teamID, cp.id, m.id, m);
    await refreshSummary();
  }

  async function deleteMember(mID: string) {
    cp = await api.capacity.deleteMember(teamID, cp.id, mID) as CapacityPlan;
    await refreshSummary();
  }

  async function addSprint() {
    console.log("Adding sprint: ", teamID, cp.id, newSprintName, newSprintStart, newSprintEnd);
    if (!newSprintStart || !newSprintEnd || !cp.id) return;
    if (newSprintEnd <= newSprintStart) {
      error = 'Sprint end date must be after start date';
      return;
    }
    error = '';
    cp = await api.capacity.addSprint(teamID, cp.id, {
      name: newSprintName || undefined,
      start_date: newSprintStart, end_date: newSprintEnd
    });
    // Ensure sorted in client as well
    if (cp.sprints) {
      cp.sprints.sort((a, b) => (a.start_date || '').localeCompare(b.start_date || ''));
    }
    const addedSprint = cp.sprints?.find(s => s.start_date === newSprintStart && (!newSprintName || s.name === newSprintName));
    if (addedSprint) {
      expandedSprints = new Set([...expandedSprints, addedSprint.id]);
    }
    newSprintName = ''; newSprintStart = ''; newSprintEnd = '';
    showAddSprint = false;
    await refreshSummary();
  }

  function startEditSprint(sprint: Sprint, e?: Event) {
    if (e) e.stopPropagation();
    editingSprintId = sprint.id;
    editSprintName = sprint.name;
    editSprintStart = sprint.start_date;
    editSprintEnd = sprint.end_date;
  }

  function cancelEditSprint(e?: Event) {
    if (e) e.stopPropagation();
    editingSprintId = null;
    editSprintName = '';
    editSprintStart = '';
    editSprintEnd = '';
  }

  async function saveSprint(sID: string, e?: Event) {
    if (e) e.stopPropagation();
    if (!editSprintStart || !editSprintEnd || !cp.id) return;
    if (editSprintEnd <= editSprintStart) {
      error = 'Sprint end date must be after start date';
      return;
    }
    error = '';
    savingSprint = true;
    try {
      cp = await api.capacity.updateSprint(teamID, cp.id, sID, {
        name: editSprintName.trim() || undefined,
        start_date: editSprintStart,
        end_date: editSprintEnd
      });
      if (cp.sprints) {
        cp.sprints.sort((a, b) => (a.start_date || '').localeCompare(b.start_date || ''));
      }
      editingSprintId = null;
      await refreshSummary();
    } catch (err: any) {
      error = err.message;
    } finally {
      savingSprint = false;
    }
  }

  async function deleteSprint(sID: string) {
    cp = await api.capacity.deleteSprint(teamID, cp.id, sID) as CapacityPlan;
    await refreshSummary();
  }

  async function upsertLeave(sID: string, mID: string, val: number) {
    await api.capacity.upsertLeave(teamID, cp.id, sID, mID, val);
    await refreshSummary();
  }

  function getLeave(sprint: Sprint, mID: string): number {
    return sprint.leaves?.find(l => l.member_id === mID)?.leaves ?? 0;
  }

  function toggleSprint(id: string) {
    const s = new Set(expandedSprints);
    s.has(id) ? s.delete(id) : s.add(id);
    expandedSprints = s;
  }

  async function createAndContinue() {
    if (!cp.name.trim()) { error = 'Plan name is required'; return; }
    saving = true;
    try {
      const created = await api.capacity.create(teamID, {
        name: cp.name, hours_per_sp: cp.hours_per_sp,
        productive_hours: cp.productive_hours, loading_factor: cp.loading_factor
      });
      goto(`/teams/${teamID}/capacity/${created.id}`);
    } catch (e: any) {
      error = e.message;
    } finally { saving = false; }
  }
</script>

<svelte:head>
  <title>{isNew ? 'New Capacity Plan' : cp.name} – Scrumy</title>
</svelte:head>

<!-- Top bar -->
<div style="background:var(--c-surface);border-bottom:1px solid var(--c-border);padding:12px 24px;display:flex;align-items:center;justify-content:space-between;position:sticky;top:56px;z-index:99;">
  <div class="flex items-center gap-3">
    <a href="/teams/{teamID}" class="btn-icon" style="font-size:18px;" title="Back">←</a>
    <h1 class="font-semibold" style="font-size:16px;">{isNew ? 'New Capacity Plan' : cp.name}</h1>
  </div>
  <div class="flex gap-2">
    <a href="/teams/{teamID}" class="btn btn-secondary">Cancel</a>
    {#if isNew}
      <button type="button" class="btn btn-primary" onclick={createAndContinue} disabled={saving || !cp.name.trim()}>
        {saving ? 'Creating…' : 'Continue →'}
      </button>
    {:else}
      <button type="button" class="btn btn-primary" onclick={finalize} disabled={saving}>
        <span>✓</span> {saving ? 'Saving…' : 'Finalize Capacity Plan'}
      </button>
    {/if}
  </div>
</div>

{#if error}
  <div style="padding:8px 24px;background:var(--c-danger-lt);color:var(--c-danger);font-size:13px;">⚠️ {error}</div>
{/if}

<div style="max-width:1100px;margin:0 auto;padding:24px 24px 80px;display:flex;flex-direction:column;gap:20px;">

  {#if isNew}
    <!-- Name + Velocity for new teams -->
    <div class="card">
      <div class="card-header"><h2 class="font-semibold">Capacity Plan Details</h2></div>
      <div class="card-body" style="display:flex;flex-direction:column;gap:16px;">
        <div class="form-group">
          <label class="label" for="team-name">Capacity Plan Name</label>
          <input id="team-name" class="input" bind:value={cp.name} placeholder="e.g. Mid-Quarter Revision" />
        </div>
        <div class="grid-3">
          <div class="form-group">
            <label class="label" for="hpsp">Hours Per Story Point</label>
            <input id="hpsp" type="number" class="input" bind:value={cp.hours_per_sp} min="0.5" step="0.5" />
          </div>
          <div class="form-group">
            <label class="label" for="proh">Avg. Productive Hours / Day</label>
            <input id="proh" type="number" class="input" bind:value={cp.productive_hours} min="1" step="0.5" />
          </div>
          <div class="form-group">
            <label class="label" for="lf">Loading Factor (%)</label>
            <input
              id="lf"
              type="number"
              class="input"
              value={loadingFactorPct}
              min="1"
              max="100"
              step="1"
              oninput={e => updateLoadingFactorPct(Number((e.target as HTMLInputElement).value))}
            />
            <span class="text-xs text-muted">e.g. 75 = 75%</span>
          </div>
        </div>
      </div>
    </div>
  {:else if loading}
    <div class="flex items-center gap-3" style="padding:40px 0"><span class="spinner"></span><span class="text-muted">Loading…</span></div>
  {:else}
    <!-- LEFT/RIGHT layout for edit mode -->
    <div style="display:grid;grid-template-columns:1fr 320px;gap:20px;align-items:start;">
      <div style="display:flex;flex-direction:column;gap:20px;">

        <!-- People Section -->
        <div class="card">
          <div class="card-header">
            <h2 class="font-semibold">People</h2>
            <button type="button" class="btn btn-ghost btn-sm" onclick={() => showAddMember = !showAddMember}>+ Add Person</button>
          </div>
          {#if showAddMember}
            <div style="padding:12px 20px;background:var(--c-bg);border-bottom:1px solid var(--c-border);display:flex;gap:10px;align-items:flex-end;flex-wrap:wrap;">
              <div class="form-group" style="flex:2;min-width:150px;">
                <label class="label">Name</label>
                <input class="input" bind:value={newMemberName} placeholder="e.g. Sarah Connor" />
              </div>
              <div class="form-group" style="flex:1;min-width:100px;">
                <label class="label">Role</label>
                <input class="input" bind:value={newMemberRole} placeholder="Developer" />
              </div>
              <div class="form-group" style="width:100px;">
                <label class="label">Utilization %</label>
                <input type="number" class="input" bind:value={newMemberUtil} min="0" max="100" />
              </div>
              <div class="flex gap-2">
                <button type="button" class="btn btn-secondary btn-sm" onclick={() => { showAddMember=false; newMemberName=''; }}>Cancel</button>
                <button type="button" class="btn btn-primary btn-sm" onclick={addMember} disabled={!newMemberName.trim()}>Add</button>
              </div>
            </div>
          {/if}
          <table class="table">
            <thead>
              <tr>
                <th>Name</th>
                <th>Role</th>
                <th style="width:140px;">Utilization (%)</th>
                <th style="width:50px;"></th>
              </tr>
            </thead>
            <tbody>
              {#if !cp.members?.length}
                <tr><td colspan="4" style="text-align:center;color:var(--c-text-3);padding:20px;">No members yet. Add a person above.</td></tr>
              {:else}
                {#each cp.members as m (m.id)}
                  <tr>
                    <td class="font-medium">{m.name}</td>
                    <td class="text-muted">{m.role || '—'}</td>
                    <td>
                      <input
                        type="number"
                        class="input"
                        style="width:80px;text-align:center;"
                        value={m.utilization_pct}
                        min="0" max="100"
                        onchange={e => updateMemberUtil(m, Number((e.target as HTMLInputElement).value))}
                      />
                    </td>
                    <td>
                      <button class="btn-icon" style="color:var(--c-danger);font-size:13px;" onclick={() => deleteMember(m.id)} title="Remove">✕</button>
                    </td>
                  </tr>
                {/each}
              {/if}
            </tbody>
          </table>
        </div>

        <!-- Sprints & Leaves -->
        <div class="card">
          <div class="card-header">
            <h2 class="font-semibold">Sprints & Leaves</h2>
            <button type="button" class="btn btn-ghost btn-sm" onclick={() => showAddSprint = !showAddSprint}>+ Add Sprint</button>
          </div>

          {#if showAddSprint}
            <div style="padding:12px 20px;background:var(--c-bg);border-bottom:1px solid var(--c-border);display:flex;gap:10px;align-items:flex-end;flex-wrap:wrap;">
              <div class="form-group" style="flex:1;min-width:140px;">
                <label class="label">Sprint Name (optional)</label>
                <input class="input" bind:value={newSprintName} placeholder="Sprint 24" />
              </div>
              <div class="form-group">
                <label class="label">Start Date</label>
                <input type="date" class="input" bind:value={newSprintStart} />
              </div>
              <div class="form-group">
                <label class="label">End Date</label>
                <input type="date" class="input" bind:value={newSprintEnd} />
              </div>
              <div class="flex gap-2">
                <button type="button" class="btn btn-secondary btn-sm" onclick={() => { showAddSprint=false; }}>Cancel</button>
                <button type="button" class="btn btn-primary btn-sm" onclick={addSprint} disabled={!newSprintStart || !newSprintEnd}>Add</button>
              </div>
            </div>
          {/if}

          <div style="display:flex;flex-direction:column;gap:0;">
            {#if !cp.sprints?.length}
              <div class="empty-state"><p>No sprints yet. Add a sprint above.</p></div>
            {:else}
              {#each cp.sprints as sprint (sprint.id)}
                <div style="border-bottom:1px solid var(--c-border);">
                  {#if editingSprintId === sprint.id}
                    <!-- Edit sprint inline form -->
                    <div style="padding:14px 20px;background:var(--c-bg);display:flex;gap:10px;align-items:flex-end;flex-wrap:wrap;">
                      <div class="form-group" style="flex:1;min-width:140px;margin-bottom:0;">
                        <label class="label" style="font-size:11px;margin-bottom:4px;">Sprint Name</label>
                        <input class="input" bind:value={editSprintName} placeholder="e.g. Sprint 24" />
                      </div>
                      <div class="form-group" style="margin-bottom:0;">
                        <label class="label" style="font-size:11px;margin-bottom:4px;">Start Date</label>
                        <input type="date" class="input" bind:value={editSprintStart} />
                      </div>
                      <div class="form-group" style="margin-bottom:0;">
                        <label class="label" style="font-size:11px;margin-bottom:4px;">End Date</label>
                        <input type="date" class="input" bind:value={editSprintEnd} />
                      </div>
                      <div class="flex gap-2">
                        <button type="button" class="btn btn-secondary btn-sm" onclick={cancelEditSprint} disabled={savingSprint}>Cancel</button>
                        <button type="button" class="btn btn-primary btn-sm" onclick={(e) => saveSprint(sprint.id, e)} disabled={savingSprint || !editSprintStart || !editSprintEnd}>
                          {savingSprint ? 'Saving…' : 'Save'}
                        </button>
                      </div>
                    </div>
                  {:else}
                    <!-- Sprint header -->
                    <div
                      role="button"
                      tabindex="0"
                      style="width:100%;display:flex;align-items:center;justify-content:space-between;padding:14px 20px;cursor:pointer;user-select:none;"
                      onclick={() => toggleSprint(sprint.id)}
                      onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); toggleSprint(sprint.id); } }}
                    >
                      <div class="flex items-center gap-3">
                        <span style="font-size:13px;color:var(--c-text-3);transition:transform 200ms ease;transform:rotate({expandedSprints.has(sprint.id) ? '0' : '-90'}deg)">▾</span>
                        <span class="font-semibold" style="font-size:16px;">{sprint.name}</span>
                      </div>
                      <div class="flex items-center gap-3">
                        <span class="text-sm text-muted">Start: {sprint.start_date} · End: {sprint.end_date}</span>
                        <button
                          type="button"
                          class="btn btn-ghost btn-sm"
                          style="font-size:12px;padding:2px 8px;"
                          onclick={(e) => startEditSprint(sprint, e)}
                          title="Edit Sprint"
                        >
                          ✎ Edit
                        </button>
                      </div>
                    </div>
                  {/if}

                  {#if expandedSprints.has(sprint.id)}
                    <div style="padding:0 20px 16px;">
                      <table class="table" style="background:var(--c-bg);border-radius:var(--r-sm);overflow:hidden;">
                        <thead>
                          <tr>
                            <th>Name</th>
                            <th style="width:140px;text-align:right;">Leaves (Days)</th>
                          </tr>
                        </thead>
                        <tbody>
                          {#if !cp.members?.length}
                            <tr><td colspan="2" style="color:var(--c-text-3);font-size:13px;">Add team members first.</td></tr>
                          {:else}
                            {#each cp.members as m (m.id)}
                              <tr>
                                <td>{m.name}</td>
                                <td style="text-align:right;">
                                  <input
                                    type="number"
                                    class="input"
                                    style="width:80px;text-align:center;"
                                    value={getLeave(sprint, m.id)}
                                    min="0" step="0.5"
                                    onchange={e => upsertLeave(sprint.id, m.id, Number((e.target as HTMLInputElement).value))}
                                  />
                                </td>
                              </tr>
                            {/each}
                          {/if}
                        </tbody>
                      </table>
                      <div class="flex justify-end gap-2" style="margin-top:8px;">
                        {#if editingSprintId !== sprint.id}
                          <button type="button" class="btn btn-secondary btn-sm" onclick={(e) => startEditSprint(sprint, e)}>Edit Sprint</button>
                        {/if}
                        <button type="button" class="btn btn-danger btn-sm" onclick={() => deleteSprint(sprint.id)}>Remove Sprint</button>
                      </div>
                    </div>
                  {/if}
                </div>
              {/each}
            {/if}
          </div>
        </div>

        <!-- Output Summary -->
        {#if summary}
          <div class="card">
            <div class="card-header">
              <h2 class="font-semibold">Output Summary</h2>
              <span class="badge badge-primary">LIVE CALC</span>
            </div>
            <div style="overflow-x:auto;">
              <table class="table">
                <thead>
                  <tr>
                    <th>Sprint</th>
                    <th>Gross Person Days</th>
                    <th>Leaves</th>
                    <th>Net Person Days</th>
                    <th>Loaded Person Days</th>
                    <th>Target Story Points</th>
                    <th>Thin Target</th>
                    <th>Stretch Target</th>
                  </tr>
                </thead>
                <tbody>
                  {#each summary.sprints as ss (ss.sprint_id)}
                    <tr>
                      <td class="font-medium">{ss.sprint_name}</td>
                      <td>{ss.gross_person_days}</td>
                      <td>{ss.leaves}</td>
                      <td>{ss.net_person_days}</td>
                      <td>{ss.loaded_person_days}</td>
                      <td style="color:var(--c-primary);font-weight:600;">{ss.target_sp}</td>
                      <td>{ss.thin_target}</td>
                      <td>{ss.stretch_target}</td>
                    </tr>
                  {/each}
                  {#if summary.totals}
                    <tr class="total-row">
                      <td>TOTAL</td>
                      <td>{summary.totals.gross_person_days}</td>
                      <td>{summary.totals.leaves}</td>
                      <td>{summary.totals.net_person_days}</td>
                      <td>{summary.totals.loaded_person_days}</td>
                      <td style="color:var(--c-primary);">{summary.totals.target_sp}</td>
                      <td>{summary.totals.thin_target}</td>
                      <td>{summary.totals.stretch_target}</td>
                    </tr>
                  {/if}
                </tbody>
              </table>
            </div>
          </div>
        {/if}

      </div>

      <!-- Velocity Parameters Sidebar -->
      <div class="card" style="position:sticky;top:120px;">
        <div class="card-header"><h2 class="font-semibold">Velocity Parameters</h2></div>
        <div class="card-body" style="display:flex;flex-direction:column;gap:16px;">
          <div class="form-group">
            <label class="label" for="prhd">Avg. Productive Hours / Day</label>
            <input id="prhd" type="number" class="input" bind:value={cp.productive_hours} min="1" max="24" step="0.5" onchange={saveVelocity} />
          </div>
          <div class="form-group">
            <label class="label" for="hpsp2">Hours per Story Point</label>
            <input id="hpsp2" type="number" class="input" bind:value={cp.hours_per_sp} min="0.5" step="0.5" onchange={saveVelocity} />
          </div>
          <div class="form-group">
            <label class="label" for="lf2">Loading Factor (%)</label>
            <input
              id="lf2"
              type="number"
              class="input"
              value={loadingFactorPct}
              min="1"
              max="100"
              step="1"
              oninput={e => updateLoadingFactorPct(Number((e.target as HTMLInputElement).value))}
              onchange={saveVelocity}
            />
            <span class="text-xs text-muted">Accounts for meetings, ceremonies, and standard overhead (e.g. 75 = 75%).</span>
          </div>
        </div>
      </div>
    </div>
  {/if}
</div>
