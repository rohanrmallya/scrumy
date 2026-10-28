<script lang="ts">
  import { page } from '$app/stores';
  import { onMount } from 'svelte';
  import { api, type JiraSnapshot, type Team, type AutoRefreshJobInfo, type JiraAutoRefreshAudit } from '$lib/api';

  const teamID = $derived($page.params.id as string);

  let team = $state<Team | null>(null);
  let snapshots = $state<JiraSnapshot[]>([]);
  let autoRefreshJobs = $state<AutoRefreshJobInfo[]>([]);
  let audits = $state<JiraAutoRefreshAudit[]>([]);
  let loading = $state(true);
  let error = $state('');
  let notice = $state('');

  // Active Tab: 'snapshots' | 'jobs' | 'audits'
  let activeTab = $state<'snapshots' | 'jobs' | 'audits'>('snapshots');

  // Filters State for Snapshots
  let searchQuery = $state('');
  let startDateFilter = $state('');
  let endDateFilter = $state('');

  // Filters State for Audits
  let auditSnapshotFilter = $state('');
  let auditStatusFilter = $state('all');

  // Action states
  let runningJobId = $state<string | null>(null);
  let togglingJobId = $state<string | null>(null);

  async function loadData() {
    loading = true;
    error = '';
    try {
      const [p, sn] = await Promise.all([
        api.teams.get(teamID),
        api.jira.listSnapshots(teamID)
      ]);
      team = p;
      snapshots = sn;

      // If user is admin, load jobs and audits
      if (p.is_admin) {
        await refreshJobsAndAudits();
      }
    } catch (e: any) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  async function refreshJobsAndAudits() {
    try {
      const [jobsData, auditsData] = await Promise.all([
        api.jira.getAutoRefreshJobs(teamID),
        api.jira.getAutoRefreshAudits(teamID)
      ]);
      autoRefreshJobs = jobsData;
      audits = auditsData;
    } catch (e: any) {
      console.error('Failed to load auto-refresh data:', e);
    }
  }

  onMount(loadData);

  // Derived filtered snapshots list
  const filteredSnapshots = $derived.by(() => {
    return snapshots.filter(s => {
      if (searchQuery.trim()) {
        const query = searchQuery.trim().toLowerCase();
        if (!s.name.toLowerCase().includes(query)) return false;
      }
      if (startDateFilter) {
        if (s.start_date < startDateFilter) return false;
      }
      if (endDateFilter) {
        if (s.end_date > endDateFilter) return false;
      }
      return true;
    });
  });

  // Derived filtered audits list
  const filteredAudits = $derived.by(() => {
    return audits.filter(a => {
      if (auditSnapshotFilter && a.snapshot_id !== auditSnapshotFilter) return false;
      if (auditStatusFilter !== 'all' && a.status !== auditStatusFilter) return false;
      return true;
    });
  });

  const enabledJobsCount = $derived(autoRefreshJobs.filter(j => j.auto_refresh_enabled).length);
  const activeJobsCount = $derived(autoRefreshJobs.filter(j => j.auto_refresh_enabled && j.date_status === 'active').length);

  async function deleteSnapshot(id: string) {
    if (!confirm('Are you sure you want to delete this snapshot?')) return;
    try {
      await api.jira.deleteSnapshot(teamID, id);
      snapshots = snapshots.filter(s => s.id !== id);
      autoRefreshJobs = autoRefreshJobs.filter(j => j.snapshot_id !== id);
      notice = 'Snapshot deleted successfully.';
    } catch (e: any) {
      alert(`Failed to delete snapshot: ${e.message}`);
    }
  }

  // Edit Snapshot Modal state
  let activeEditSnapshot = $state<JiraSnapshot | null>(null);
  let editSnapshotName = $state('');
  let editSnapshotStart = $state('');
  let editSnapshotEnd = $state('');
  let editSnapshotAutoRefresh = $state(false);
  let editSnapshotAutoRefreshTime = $state('09:00');
  let updatingSnapshot = $state(false);
  let editSnapshotError = $state('');

  function openEditSnapshot(s: JiraSnapshot) {
    activeEditSnapshot = s;
    editSnapshotName = s.name;
    editSnapshotStart = s.start_date;
    editSnapshotEnd = s.end_date;
    editSnapshotAutoRefresh = !!s.auto_refresh_enabled;
    editSnapshotAutoRefreshTime = s.auto_refresh_time || '09:00';
    editSnapshotError = '';
  }

  async function saveEditedSnapshot() {
    if (!activeEditSnapshot) return;
    if (!editSnapshotName.trim() || !editSnapshotStart || !editSnapshotEnd) {
      editSnapshotError = 'Name, start date, and end date are required';
      return;
    }
    updatingSnapshot = true;
    editSnapshotError = '';
    try {
      const updated = await api.jira.updateSnapshot(teamID, activeEditSnapshot.id, {
        name: editSnapshotName.trim(),
        start_date: editSnapshotStart,
        end_date: editSnapshotEnd,
        auto_refresh_enabled: editSnapshotAutoRefresh,
        auto_refresh_time: editSnapshotAutoRefreshTime,
      });
      snapshots = snapshots.map(s => s.id === updated.id ? updated : s);
      activeEditSnapshot = null;
      await refreshJobsAndAudits();
      notice = 'Snapshot and auto-refresh settings saved.';
    } catch (e: any) {
      editSnapshotError = e.message;
    } finally {
      updatingSnapshot = false;
    }
  }

  // Quick toggle/disable auto-refresh from Jobs view
  async function toggleAutoRefreshJob(job: AutoRefreshJobInfo) {
    togglingJobId = job.snapshot_id;
    error = '';
    notice = '';
    try {
      if (job.auto_refresh_enabled) {
        await api.jira.disableAutoRefresh(teamID, job.snapshot_id);
        notice = `Auto-refresh disabled for "${job.snapshot_name}".`;
      } else {
        await api.jira.updateAutoRefresh(teamID, job.snapshot_id, {
          auto_refresh_enabled: true,
          auto_refresh_time: job.auto_refresh_time || '09:00',
        });
        notice = `Auto-refresh enabled for "${job.snapshot_name}" (daily at ${job.auto_refresh_time || '09:00'}).`;
      }
      await refreshJobsAndAudits();
      const sn = await api.jira.listSnapshots(teamID);
      snapshots = sn;
    } catch (e: any) {
      error = e.message;
    } finally {
      togglingJobId = null;
    }
  }

  // Trigger immediate run for an auto-refresh job
  async function runJobNow(job: AutoRefreshJobInfo) {
    runningJobId = job.snapshot_id;
    error = '';
    notice = '';
    try {
      const res = await api.jira.runAutoRefresh(teamID, job.snapshot_id);
      notice = `✓ Refreshed "${job.snapshot_name}": ${res.audit.message}`;
      await refreshJobsAndAudits();
      const sn = await api.jira.listSnapshots(teamID);
      snapshots = sn;
    } catch (e: any) {
      error = `Run failed for "${job.snapshot_name}": ${e.message}`;
    } finally {
      runningJobId = null;
    }
  }

  function viewSnapshotAudits(snapshotId: string) {
    auditSnapshotFilter = snapshotId;
    auditStatusFilter = 'all';
    activeTab = 'audits';
  }

  function clearFilters() {
    searchQuery = '';
    startDateFilter = '';
    endDateFilter = '';
  }

  function formatDate(iso?: string) {
    try {
      if (!iso) return '—';
      const datePart = iso.includes('T') ? iso.split('T')[0] : iso.split(' ')[0];
      const parts = datePart.split('-');
      if (parts.length !== 3) return iso;
      const [year, month, day] = parts;
      const date = new Date(Number(year), Number(month) - 1, Number(day));
      return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' });
    } catch { return iso || '—'; }
  }

  function formatDateTime(iso?: string) {
    try {
      if (!iso) return '—';
      const date = new Date(iso);
      if (isNaN(date.getTime())) return iso;
      return date.toLocaleString('en-US', {
        month: 'short',
        day: 'numeric',
        year: 'numeric',
        hour: 'numeric',
        minute: '2-digit',
        hour12: true
      });
    } catch { return iso || '—'; }
  }
</script>

<svelte:head>
  <title>Jira Snapshots & Auto-Refresh – Scrumy</title>
</svelte:head>

<div class="page-content fade-in">
  <!-- Back Link -->
  <div style="margin-bottom: 20px;">
    <a href="/teams/{teamID}" class="btn btn-secondary btn-sm" style="display:inline-flex; align-items:center; gap:6px;">
      ← Back to Team Workspace
    </a>
  </div>

  {#if loading}
    <div class="flex items-center gap-3" style="padding:40px 0">
      <span class="spinner"></span>
      <span class="text-muted">Loading snapshots & auto-refresh settings…</span>
    </div>
  {:else if error}
    <div class="badge badge-danger" style="padding:12px 20px; border-radius:8px; font-size:14px; text-transform:none; letter-spacing:0; font-weight:400; margin-bottom:16px; display:block;">
      ⚠️ {error}
    </div>
  {/if}

  {#if notice}
    <div class="badge badge-success" style="padding:12px 20px; border-radius:8px; font-size:14px; text-transform:none; letter-spacing:0; font-weight:400; margin-bottom:16px; display:block;">
      {notice}
    </div>
  {/if}

  {#if team}
    <!-- Header Title -->
    <div style="margin-bottom: 20px; display:flex; justify-content:space-between; align-items:flex-start; flex-wrap:wrap; gap:12px;">
      <div>
        <h1 class="text-2xl font-bold">📸 Jira Snapshots & Auto-Refresh</h1>
        <p class="text-sm text-muted" style="margin-top: 4px;">
          Manage snapshots, automated daily refresh jobs, and audit logs for <strong>{team.name}</strong>.
        </p>
      </div>
      {#if team.is_admin}
        <div class="flex gap-2">
          <a href="/teams/{teamID}?create_snapshot=true" class="btn btn-primary btn-sm">
            + Create Snapshot
          </a>
        </div>
      {/if}
    </div>

    <!-- Navigation Tabs -->
    <div style="display:flex; gap:8px; margin-bottom:24px; border-bottom:1px solid var(--c-border); padding-bottom:8px;">
      <button
        class="btn btn-sm {activeTab === 'snapshots' ? 'btn-primary' : 'btn-ghost'}"
        onclick={() => activeTab = 'snapshots'}
        style="display:flex; align-items:center; gap:6px;"
      >
        <span>📸</span> All Snapshots ({snapshots.length})
      </button>
      {#if team.is_admin}
        <button
          class="btn btn-sm {activeTab === 'jobs' ? 'btn-primary' : 'btn-ghost'}"
          onclick={() => activeTab = 'jobs'}
          style="display:flex; align-items:center; gap:6px;"
        >
          <span>⏰</span> Auto-Refresh Jobs ({enabledJobsCount})
        </button>
        <button
          class="btn btn-sm {activeTab === 'audits' ? 'btn-primary' : 'btn-ghost'}"
          onclick={() => activeTab = 'audits'}
          style="display:flex; align-items:center; gap:6px;"
        >
          <span>📜</span> Refresh Audit Logs ({audits.length})
        </button>
      {/if}
    </div>

    <!-- TAB 1: ALL SNAPSHOTS -->
    {#if activeTab === 'snapshots'}
      <!-- Filters Panel -->
      <div class="card" style="margin-bottom: 24px; padding: 20px;">
        <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 16px; align-items: flex-end;">
          <div class="form-group" style="margin-bottom: 0;">
            <label class="label" style="font-size: 11px; margin-bottom: 6px;">Search by Name</label>
            <input
              class="input"
              type="text"
              placeholder="e.g. Sprint 24 Retro"
              bind:value={searchQuery}
              style="padding: 8px; font-size: 13px;"
            />
          </div>

          <div class="form-group" style="margin-bottom: 0;">
            <label class="label" style="font-size: 11px; margin-bottom: 6px;">Start Date (From)</label>
            <input
              class="input"
              type="date"
              bind:value={startDateFilter}
              style="padding: 8px; font-size: 13px;"
            />
          </div>

          <div class="form-group" style="margin-bottom: 0;">
            <label class="label" style="font-size: 11px; margin-bottom: 6px;">End Date (To)</label>
            <input
              class="input"
              type="date"
              bind:value={endDateFilter}
              style="padding: 8px; font-size: 13px;"
            />
          </div>

          <div style="display: flex; gap: 8px;">
            {#if searchQuery || startDateFilter || endDateFilter}
              <button class="btn btn-secondary btn-sm" onclick={clearFilters} style="flex: 1; justify-content: center; height: 38px;">
                Clear Filters
              </button>
            {/if}
          </div>
        </div>
      </div>

      <!-- Snapshots List Section -->
      <div class="card" style="overflow: hidden;">
        <div class="card-header flex justify-between items-center" style="padding: 16px 20px;">
          <h2 class="font-semibold" style="font-size: 15px;">
            {#if filteredSnapshots.length === snapshots.length}
              All Snapshots ({snapshots.length})
            {:else}
              Filtered Snapshots ({filteredSnapshots.length} of {snapshots.length})
            {/if}
          </h2>
        </div>

        <div>
          {#if filteredSnapshots.length === 0}
            <div class="empty-state" style="padding: 40px; text-align: center;">
              <p style="font-size: 15px; color: var(--c-text-2);">No snapshots match your filter criteria.</p>
              {#if searchQuery || startDateFilter || endDateFilter}
                <button class="btn btn-secondary btn-sm" onclick={clearFilters} style="margin-top: 12px;">
                  Reset Filters
                </button>
              {/if}
            </div>
          {:else}
            {#each filteredSnapshots as s (s.id)}
              <div
                style="display:flex; justify-content:space-between; align-items:center; padding:16px 20px; border-bottom:1px solid var(--c-border); transition:background 150ms ease;"
                onmouseover={e => (e.currentTarget as HTMLElement).style.background = 'var(--c-bg)'}
                onmouseout={e => (e.currentTarget as HTMLElement).style.background = ''}
              >
                <div style="flex: 1; min-width: 0; margin-right: 16px;">
                  <div style="display:flex; align-items:center; gap:10px; margin-bottom:4px; flex-wrap:wrap;">
                    <a
                      href="/teams/{teamID}/snapshots/{s.id}"
                      style="font-weight: 600; font-size: 15px; color: var(--c-primary); text-decoration: none;"
                      class="hover-underline"
                    >
                      {s.name}
                    </a>
                    {#if s.auto_refresh_enabled}
                      <span class="badge badge-primary" style="font-size:11px; padding:3px 8px; font-weight:500;">
                        ⏰ Daily at {s.auto_refresh_time || '09:00'}
                      </span>
                    {:else}
                      <span class="badge" style="background:var(--c-bg-2); color:var(--c-text-muted); font-size:11px; padding:3px 8px;">
                        Auto-refresh: Off
                      </span>
                    {/if}
                  </div>
                  <div class="flex items-center gap-2 text-xs text-muted" style="flex-wrap: wrap;">
                    <span>📅 Sprint: <strong>{formatDate(s.start_date)}</strong> to <strong>{formatDate(s.end_date)}</strong></span>
                    <span>•</span>
                    <span>⚙️ Mode: <strong>{s.all_worklogs ? 'All worklogs' : 'Filtered worklogs'}</strong></span>
                    {#if s.last_auto_refreshed_at}
                      <span>•</span>
                      <span>🔄 Last Auto-Refresh: {formatDateTime(s.last_auto_refreshed_at)}</span>
                    {/if}
                  </div>
                </div>

                <div class="flex gap-2 items-center">
                  <a href="/teams/{teamID}/snapshots/{s.id}" class="btn btn-secondary btn-sm">
                    View Details
                  </a>
                  {#if team.is_admin}
                    <button
                      class="btn btn-secondary btn-sm btn-icon"
                      style="color: var(--c-primary); width: 32px; height: 32px; padding: 0; display: inline-flex; align-items: center; justify-content: center;"
                      onclick={() => openEditSnapshot(s)}
                      title="Edit Snapshot & Auto-Refresh Settings"
                    >
                      ✎
                    </button>
                    <button
                      class="btn btn-danger btn-sm btn-icon"
                      style="color: var(--c-danger); width: 32px; height: 32px; padding: 0; display: inline-flex; align-items: center; justify-content: center;"
                      onclick={() => deleteSnapshot(s.id)}
                      title="Delete Snapshot"
                    >
                      ✕
                    </button>
                  {/if}
                </div>
              </div>
            {/each}
          {/if}
        </div>
      </div>

    <!-- TAB 2: AUTO-REFRESH JOBS (ADMIN VIEW) -->
    {:else if activeTab === 'jobs'}
      <!-- Overview Metric Cards -->
      <div style="display:grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap:16px; margin-bottom:24px;">
        <div class="card" style="padding:16px;">
          <p class="text-xs text-muted font-medium" style="text-transform:uppercase; letter-spacing:0.5px;">Auto-Refresh Enabled</p>
          <div style="font-size:24px; font-weight:700; margin-top:4px; color:var(--c-primary);">
            {enabledJobsCount} <span style="font-size:13px; font-weight:400; color:var(--c-text-muted);">/ {autoRefreshJobs.length} snapshots</span>
          </div>
        </div>
        <div class="card" style="padding:16px;">
          <p class="text-xs text-muted font-medium" style="text-transform:uppercase; letter-spacing:0.5px;">Active Daily Runs</p>
          <div style="font-size:24px; font-weight:700; margin-top:4px; color:var(--c-success);">
            {activeJobsCount}
            <span style="font-size:12px; font-weight:400; color:var(--c-text-muted); display:block; margin-top:2px;">Currently inside sprint active window</span>
          </div>
        </div>
        <div class="card" style="padding:16px;">
          <p class="text-xs text-muted font-medium" style="text-transform:uppercase; letter-spacing:0.5px;">Window Policy</p>
          <div style="font-size:13px; font-weight:500; margin-top:6px; color:var(--c-text);">
            🔒 Refreshes only between snapshot start & end dates; automatically stops afterwards.
          </div>
        </div>
      </div>

      <!-- Jobs Table Card -->
      <div class="card" style="overflow:hidden;">
        <div class="card-header flex justify-between items-center" style="padding:16px 20px;">
          <h2 class="font-semibold" style="font-size:15px;">Auto-Refresh Jobs & Schedules</h2>
          <button class="btn btn-secondary btn-xs" onclick={refreshJobsAndAudits}>🔄 Refresh List</button>
        </div>

        <div style="overflow-x:auto;">
          {#if autoRefreshJobs.length === 0}
            <div class="empty-state" style="padding:40px; text-align:center;">
              <p>No snapshots found for this team.</p>
            </div>
          {:else}
            <table class="table" style="width:100%; text-align:left;">
              <thead>
                <tr>
                  <th>Snapshot</th>
                  <th>Active Window</th>
                  <th>Schedule</th>
                  <th>Next Run</th>
                  <th>Last Auto-Refresh</th>
                  <th style="text-align:right;">Actions</th>
                </tr>
              </thead>
              <tbody>
                {#each autoRefreshJobs as job (job.snapshot_id)}
                  <tr>
                    <td>
                      <a href="/teams/{teamID}/snapshots/{job.snapshot_id}" class="font-semibold" style="color:var(--c-primary); text-decoration:none;">
                        {job.snapshot_name}
                      </a>
                    </td>
                    <td>
                      <div style="display:flex; flex-direction:column; gap:4px;">
                        <span class="text-xs">{formatDate(job.start_date)} – {formatDate(job.end_date)}</span>
                        <div>
                          {#if job.date_status === 'active'}
                            <span class="badge badge-success" style="font-size:10px; padding:2px 6px;">🟢 Active Sprint</span>
                          {:else if job.date_status === 'upcoming'}
                            <span class="badge badge-warning" style="font-size:10px; padding:2px 6px;">🟡 Upcoming</span>
                          {:else}
                            <span class="badge" style="background:var(--c-bg-2); color:var(--c-text-muted); font-size:10px; padding:2px 6px;">⚪ Ended (No refresh)</span>
                          {/if}
                        </div>
                      </div>
                    </td>
                    <td>
                      {#if job.auto_refresh_enabled}
                        <div style="display:flex; align-items:center; gap:6px;">
                          <span class="badge badge-primary" style="font-size:11px;">⏰ Daily at {job.auto_refresh_time}</span>
                        </div>
                      {:else}
                        <span class="badge" style="background:var(--c-bg-2); color:var(--c-text-muted); font-size:11px;">Disabled</span>
                      {/if}
                    </td>
                    <td>
                      {#if job.auto_refresh_enabled}
                        {#if job.date_status === 'expired'}
                          <span class="text-xs text-muted">Sprint ended (Stopped)</span>
                        {:else if job.next_run_at}
                          <span class="text-xs font-medium">{formatDateTime(job.next_run_at?.toString())}</span>
                        {:else}
                          <span class="text-xs text-muted">—</span>
                        {/if}
                      {:else}
                        <span class="text-xs text-muted">—</span>
                      {/if}
                    </td>
                    <td>
                      {#if job.last_auto_refreshed_at}
                        <div style="display:flex; flex-direction:column; gap:2px;">
                          <span class="text-xs">{formatDateTime(job.last_auto_refreshed_at?.toString())}</span>
                          <div>
                            {#if job.last_auto_refresh_status === 'success'}
                              <span class="badge badge-success" style="font-size:9px; padding:1px 5px;">Success</span>
                            {:else if job.last_auto_refresh_status === 'failed'}
                              <span class="badge badge-danger" style="font-size:9px; padding:1px 5px;" title={job.last_auto_refresh_msg}>Failed</span>
                            {:else if job.last_auto_refresh_status === 'skipped'}
                              <span class="badge" style="background:var(--c-bg-2); color:var(--c-text-muted); font-size:9px; padding:1px 5px;">Skipped</span>
                            {/if}
                          </div>
                        </div>
                      {:else}
                        <span class="text-xs text-muted">Never run</span>
                      {/if}
                    </td>
                    <td style="text-align:right;">
                      <div class="flex gap-2" style="justify-content:flex-end;">
                        <!-- Run Now Button -->
                        <button
                          class="btn btn-secondary btn-xs"
                          onclick={() => runJobNow(job)}
                          disabled={runningJobId === job.snapshot_id}
                          title="Run Jira refresh now"
                        >
                          {runningJobId === job.snapshot_id ? 'Running…' : '⚡ Run Now'}
                        </button>

                        <!-- Toggle Enable / Disable Button -->
                        <button
                          class="btn {job.auto_refresh_enabled ? 'btn-secondary' : 'btn-primary'} btn-xs"
                          onclick={() => toggleAutoRefreshJob(job)}
                          disabled={togglingJobId === job.snapshot_id}
                        >
                          {job.auto_refresh_enabled ? 'Disable' : 'Enable'}
                        </button>

                        <!-- Audits Shortcut Button -->
                        <button
                          class="btn btn-ghost btn-xs"
                          onclick={() => viewSnapshotAudits(job.snapshot_id)}
                          title="View audit logs for this snapshot"
                        >
                          📜 Audits
                        </button>
                      </div>
                    </td>
                  </tr>
                {/each}
              </tbody>
            </table>
          {/if}
        </div>
      </div>

    <!-- TAB 3: AUDIT LOGS (ADMIN VIEW) -->
    {:else if activeTab === 'audits'}
      <!-- Audit Filter Bar -->
      <div class="card" style="margin-bottom:20px; padding:16px;">
        <div style="display:flex; gap:16px; align-items:center; flex-wrap:wrap;">
          <div class="form-group" style="margin-bottom:0; min-width:200px;">
            <label class="label" style="font-size:11px; margin-bottom:4px;">Filter by Snapshot</label>
            <select class="input" style="padding:6px; font-size:12px;" bind:value={auditSnapshotFilter}>
              <option value="">All Snapshots ({snapshots.length})</option>
              {#each snapshots as s}
                <option value={s.id}>{s.name}</option>
              {/each}
            </select>
          </div>
          <div class="form-group" style="margin-bottom:0; min-width:160px;">
            <label class="label" style="font-size:11px; margin-bottom:4px;">Filter by Status</label>
            <select class="input" style="padding:6px; font-size:12px;" bind:value={auditStatusFilter}>
              <option value="all">All Statuses</option>
              <option value="success">Success only</option>
              <option value="failed">Failed only</option>
              <option value="skipped">Skipped only</option>
            </select>
          </div>
          {#if auditSnapshotFilter || auditStatusFilter !== 'all'}
            <button class="btn btn-secondary btn-sm" onclick={() => { auditSnapshotFilter = ''; auditStatusFilter = 'all'; }} style="margin-top:16px;">
              Reset Filter
            </button>
          {/if}
        </div>
      </div>

      <!-- Audits Table -->
      <div class="card" style="overflow:hidden;">
        <div class="card-header flex justify-between items-center" style="padding:16px 20px;">
          <h2 class="font-semibold" style="font-size:15px;">Auto-Refresh Audit Log ({filteredAudits.length})</h2>
          <button class="btn btn-secondary btn-xs" onclick={refreshJobsAndAudits}>🔄 Refresh</button>
        </div>

        <div style="overflow-x:auto;">
          {#if filteredAudits.length === 0}
            <div class="empty-state" style="padding:40px; text-align:center;">
              <p>No audit records found matching criteria.</p>
            </div>
          {:else}
            <table class="table" style="width:100%; text-align:left;">
              <thead>
                <tr>
                  <th>Timestamp</th>
                  <th>Snapshot</th>
                  <th>Trigger Mode</th>
                  <th>Status</th>
                  <th>Result / Changes</th>
                  <th>Duration</th>
                </tr>
              </thead>
              <tbody>
                {#each filteredAudits as audit (audit.id)}
                  <tr>
                    <td class="text-xs" style="white-space:nowrap;">
                      {formatDateTime(audit.triggered_at)}
                    </td>
                    <td>
                      <a href="/teams/{teamID}/snapshots/{audit.snapshot_id}" class="font-medium" style="color:var(--c-primary); text-decoration:none;">
                        {audit.snapshot_name || 'Snapshot'}
                      </a>
                    </td>
                    <td>
                      {#if audit.trigger_type === 'scheduled'}
                        <span class="badge" style="background:#e0f2fe; color:#0369a1; font-size:10px;">⏰ Scheduled Auto</span>
                      {:else if audit.trigger_type === 'manual'}
                        <span class="badge" style="background:#fef3c7; color:#92400e; font-size:10px;">⚡ Manual Run</span>
                      {:else if audit.trigger_type === 'disabled'}
                        <span class="badge" style="background:#f1f5f9; color:#475569; font-size:10px;">🔒 Disabled</span>
                      {:else}
                        <span class="badge" style="background:#f1f5f9; color:#475569; font-size:10px;">{audit.trigger_type}</span>
                      {/if}
                    </td>
                    <td>
                      {#if audit.status === 'success'}
                        <span class="badge badge-success" style="font-size:10px; padding:2px 6px;">✓ Success</span>
                      {:else if audit.status === 'failed'}
                        <span class="badge badge-danger" style="font-size:10px; padding:2px 6px;">✕ Failed</span>
                      {:else}
                        <span class="badge" style="background:var(--c-bg-2); color:var(--c-text-muted); font-size:10px; padding:2px 6px;">⚪ Skipped</span>
                      {/if}
                    </td>
                    <td>
                      <div style="font-size:12px;">
                        <span class="text-xs">{audit.message}</span>
                        {#if audit.status === 'success'}
                          <div class="text-xs text-muted" style="margin-top:2px;">
                            SP: {audit.prev_story_points} → {audit.new_story_points}
                            ({audit.new_story_points - audit.prev_story_points >= 0 ? '+' : ''}{(audit.new_story_points - audit.prev_story_points).toFixed(1)} SP)
                            • Hours: {audit.prev_hours_logged.toFixed(1)}h → {audit.new_hours_logged.toFixed(1)}h
                          </div>
                        {/if}
                      </div>
                    </td>
                    <td class="text-xs text-muted">
                      {audit.duration_ms ? `${audit.duration_ms}ms` : '—'}
                    </td>
                  </tr>
                {/each}
              </tbody>
            </table>
          {/if}
        </div>
      </div>
    {/if}
  {/if}

  <!-- Edit Snapshot & Auto-Refresh Modal -->
  {#if activeEditSnapshot}
    <div style="position:fixed; top:0; left:0; right:0; bottom:0; background:rgba(0,0,0,0.55); display:flex; align-items:center; justify-content:center; z-index:1000; backdrop-filter:blur(4px); transition: opacity 0.2s ease;">
      <div style="background:var(--c-surface); border:1px solid var(--c-border); border-radius:12px; width:480px; max-width:90%; padding:24px; box-shadow:var(--shadow-lg);">
        <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:20px; border-bottom:1px solid var(--c-border); padding-bottom:12px;">
          <h3 class="font-semibold text-base" style="color:var(--c-primary); margin:0;">Edit Snapshot & Auto-Refresh</h3>
          <button class="btn-icon" onclick={() => activeEditSnapshot = null} style="font-size:14px; width:28px; height:28px; border-radius:50%; border:none; display:flex; align-items:center; justify-content:center; background:transparent;">✕</button>
        </div>
        
        <div style="display:flex; flex-direction:column; gap:16px; margin-bottom:20px;">
          <div class="form-group">
            <label class="label" style="font-size:11px; margin-bottom:6px;">Snapshot Name</label>
            <input class="input" style="padding:10px; font-size:13px;" bind:value={editSnapshotName} placeholder="e.g. Sprint 24 Retro" />
          </div>
          <div class="form-group">
            <label class="label" style="font-size:11px; margin-bottom:6px;">Start Date</label>
            <input type="date" class="input" style="padding:8px; font-size:13px;" bind:value={editSnapshotStart} />
          </div>
          <div class="form-group">
            <label class="label" style="font-size:11px; margin-bottom:6px;">End Date</label>
            <input type="date" class="input" style="padding:8px; font-size:13px;" bind:value={editSnapshotEnd} />
          </div>

          <!-- Auto-Refresh Settings Section in Modal -->
          <div style="background:var(--c-bg); border:1px solid var(--c-border); border-radius:8px; padding:12px;">
            <label style="display:flex; align-items:center; gap:8px; cursor:pointer; font-size:12px; font-weight:500;">
              <input type="checkbox" bind:checked={editSnapshotAutoRefresh} />
              <span>⏰ Enable Daily Auto-Refresh</span>
            </label>
            {#if editSnapshotAutoRefresh}
              <div style="margin-top:10px; display:flex; align-items:center; gap:10px;">
                <label class="label" style="font-size:11px; margin:0; white-space:nowrap;">Daily Time (24h)</label>
                <input type="time" class="input" style="padding:6px 10px; font-size:12px; width:auto;" bind:value={editSnapshotAutoRefreshTime} />
              </div>
              <p class="text-xs text-muted" style="margin-top:6px; font-size:11px;">
                Jira data will automatically refresh every day at this time between {editSnapshotStart || 'start'} and {editSnapshotEnd || 'end'}.
              </p>
            {/if}
          </div>
        </div>

        {#if editSnapshotError}
          <div class="badge badge-danger" style="margin-bottom:16px; padding:10px; border-radius:6px; font-size:12px; text-transform:none; letter-spacing:0; text-align:center; display:block; font-weight:400;">
            ⚠️ {editSnapshotError}
          </div>
        {/if}

        <div style="display:flex; justify-content:flex-end; gap:12px; border-top:1px solid var(--c-border); padding-top:16px;">
          <button class="btn btn-secondary" onclick={() => activeEditSnapshot = null} disabled={updatingSnapshot}>Cancel</button>
          <button class="btn btn-primary" onclick={saveEditedSnapshot} disabled={updatingSnapshot}>
            {updatingSnapshot ? 'Saving...' : 'Save Changes'}
          </button>
        </div>
      </div>
    </div>
  {/if}
</div>

<style>
  .hover-underline:hover {
    text-decoration: underline;
  }
  .spinner {
    width: 20px;
    height: 20px;
    border: 2px solid var(--c-border);
    border-top-color: var(--c-primary);
    border-radius: 50%;
    animation: spin 1s linear infinite;
  }
  @keyframes spin {
    to { transform: rotate(360deg); }
  }
</style>
