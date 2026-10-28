<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import {
    api,
    type ManagedUser,
    type BulkRefreshResponse,
    type SnapshotRefreshResult,
    type AutoRefreshJobInfo,
    type JiraAutoRefreshAudit,
  } from '$lib/api';
  import { user } from '$lib/auth';

  let activeTab = $state<'users' | 'operations' | 'autorefresh'>('users');
  let users = $state<ManagedUser[]>([]);
  let loading = $state(true);
  let error = $state('');
  let notice = $state('');

  // Auto-Refresh state
  let autoRefreshJobs = $state<AutoRefreshJobInfo[]>([]);
  let autoRefreshAudits = $state<JiraAutoRefreshAudit[]>([]);
  let loadingAutoRefresh = $state(false);
  let autoRefreshFilterStatus = $state<string>('all');
  let autoRefreshSearch = $state<string>('');
  let disablingJobId = $state<string>('');
  let runningJobId = $state<string>('');

  function formatDateTime(utcString?: string) {
    if (!utcString) return '—';
    try {
      const date = new Date(utcString);
      if (isNaN(date.getTime())) return utcString;
      return date.toLocaleString(undefined, {
        month: 'short',
        day: 'numeric',
        year: 'numeric',
        hour: '2-digit',
        minute: '2-digit'
      });
    } catch {
      return utcString;
    }
  }

  async function loadAutoRefreshData() {
    loadingAutoRefresh = true;
    try {
      const [jobs, audits] = await Promise.all([
        api.admin.getAutoRefreshJobs(),
        api.admin.getAutoRefreshAudits({ limit: 100 })
      ]);
      autoRefreshJobs = jobs || [];
      autoRefreshAudits = audits || [];
    } catch (e: any) {
      error = `Failed to load auto-refresh data: ${e.message}`;
    } finally {
      loadingAutoRefresh = false;
    }
  }

  async function disableJob(job: AutoRefreshJobInfo) {
    if (!confirm(`Disable daily auto-refresh for "${job.snapshot_name}" in project "${job.team_name}"?`)) return;
    disablingJobId = job.snapshot_id;
    error = '';
    notice = '';
    try {
      await api.admin.disableAutoRefresh(job.team_id, job.snapshot_id);
      notice = `✓ Auto-refresh disabled for "${job.snapshot_name}" (${job.team_name})`;
      await loadAutoRefreshData();
    } catch (e: any) {
      error = `Failed to disable auto-refresh: ${e.message}`;
    } finally {
      disablingJobId = '';
    }
  }

  async function runJobNow(job: AutoRefreshJobInfo) {
    if (!confirm(`Trigger immediate auto-refresh for "${job.snapshot_name}" (${job.team_name})?`)) return;
    runningJobId = job.snapshot_id;
    error = '';
    notice = '';
    try {
      const res = await api.admin.runAutoRefresh(job.team_id, job.snapshot_id);
      if (res.status === 'success') {
        notice = `✓ Auto-refresh executed successfully for "${job.snapshot_name}" (${res.duration_ms}ms).`;
      } else if (res.status === 'skipped') {
        notice = `ℹ️ Auto-refresh skipped for "${job.snapshot_name}": ${res.message}`;
      } else {
        error = `Auto-refresh ${res.status} for "${job.snapshot_name}": ${res.message || 'Error occurred'}`;
      }
      await loadAutoRefreshData();
    } catch (e: any) {
      error = `Failed to run auto-refresh: ${e.message}`;
    } finally {
      runningJobId = '';
    }
  }

  const filteredJobs = $derived.by(() => {
    let list = autoRefreshJobs;
    if (autoRefreshSearch.trim()) {
      const q = autoRefreshSearch.trim().toLowerCase();
      list = list.filter(j =>
        (j.snapshot_name || '').toLowerCase().includes(q) ||
        (j.team_name || '').toLowerCase().includes(q)
      );
    }
    return list;
  });

  const filteredAudits = $derived.by(() => {
    let list = autoRefreshAudits;
    if (autoRefreshFilterStatus !== 'all') {
      list = list.filter(a => a.status === autoRefreshFilterStatus);
    }
    if (autoRefreshSearch.trim()) {
      const q = autoRefreshSearch.trim().toLowerCase();
      list = list.filter(a =>
        (a.snapshot_name || '').toLowerCase().includes(q) ||
        (a.team_name || '').toLowerCase().includes(q) ||
        (a.message || '').toLowerCase().includes(q)
      );
    }
    return list;
  });

  const activeJobsCount = $derived(autoRefreshJobs.filter(j => j.active_status === 'active').length);
  const upcomingJobsCount = $derived(autoRefreshJobs.filter(j => j.active_status === 'upcoming').length);
  const expiredJobsCount = $derived(autoRefreshJobs.filter(j => j.active_status === 'expired').length);

  // Create form
  let newUsername = $state('');
  let newPassword = $state('');
  let newRole = $state('user');
  let creating = $state(false);

  // Reset-password inline state: userID -> new password
  let resetPasswords: Record<string, string> = $state({});
  let resetting: Record<string, boolean> = $state({});

  // Adding user to all teams state
  let addingToAll: Record<string, boolean> = $state({});

  // Bulk operations tab state
  let bulkSelectedUsername = $state('');
  let bulkAddingAdmin = $state(false);

  let bulkRefreshing = $state(false);
  let bulkRefreshResult = $state<BulkRefreshResponse | null>(null);

  async function load() {
    loading = true;
    error = '';
    try {
      users = await api.users.list();
      if (users.length > 0 && !bulkSelectedUsername) {
        bulkSelectedUsername = users[0].username;
      }
      api.admin.getAutoRefreshJobs().then(jobs => { autoRefreshJobs = jobs || []; }).catch(() => {});
    } catch (e: any) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  onMount(load);

  async function createUser(e: Event) {
    e.preventDefault();
    error = '';
    notice = '';
    if (!newUsername.trim() || !newPassword) return;
    creating = true;
    try {
      const u = await api.users.create({ username: newUsername.trim(), password: newPassword, role: newRole });
      users = [...users, u].sort((a, b) => a.username.localeCompare(b.username));
      newUsername = '';
      newPassword = '';
      newRole = 'user';
      notice = `User "${u.username}" created`;
    } catch (e: any) {
      error = e.message;
    } finally {
      creating = false;
    }
  }

  async function resetPassword(id: string) {
    const pw = (resetPasswords[id] || '').trim();
    error = '';
    notice = '';
    if (!pw) {
      error = 'Enter a new password first';
      return;
    }
    resetting[id] = true;
    try {
      await api.users.resetPassword(id, pw);
      resetPasswords[id] = '';
      notice = 'Password updated — user sessions were signed out';
    } catch (e: any) {
      error = e.message;
    } finally {
      resetting[id] = false;
    }
  }

  async function changeRole(u: ManagedUser, role: string) {
    if (u.role === role) return;
    error = '';
    notice = '';
    try {
      const updated = await api.users.updateRole(u.id, role);
      users = users.map(x => x.id === u.id ? updated : x);
      notice = `Role for "${u.username}" set to ${role}`;
    } catch (e: any) {
      error = e.message;
    }
  }

  async function deleteUser(u: ManagedUser) {
    if (!confirm(`Delete user "${u.username}"?`)) return;
    error = '';
    notice = '';
    try {
      await api.users.delete(u.id);
      users = users.filter(x => x.id !== u.id);
      notice = `User "${u.username}" deleted`;
    } catch (e: any) {
      error = e.message;
    }
  }

  async function addAdminToAllProjects(username: string, userId?: string) {
    error = '';
    notice = '';
    if (userId) addingToAll[userId] = true;
    bulkAddingAdmin = true;
    try {
      const res = await api.admin.addAdminToAllTeams(username);
      notice = `✓ Successfully added "${username}" as admin to all ${res.total_teams} project(s) (${res.added} newly added).`;
    } catch (e: any) {
      error = `Failed to add admin: ${e.message}`;
    } finally {
      if (userId) addingToAll[userId] = false;
      bulkAddingAdmin = false;
    }
  }

  async function refreshAllSnapshots() {
    if (!confirm('Refresh snapshots across all projects now? This will query Jira for each configured project.')) return;
    error = '';
    notice = '';
    bulkRefreshing = true;
    try {
      const res = await api.admin.refreshAllSnapshots();
      bulkRefreshResult = res;
      notice = `Snapshot refresh completed: ${res.refreshed} refreshed, ${res.skipped} skipped, ${res.failed} failed out of ${res.total} total snapshot(s).`;
    } catch (e: any) {
      error = `Failed to refresh snapshots: ${e.message}`;
    } finally {
      bulkRefreshing = false;
    }
  }
</script>

<svelte:head>
  <title>Admin & Operations – Scrumy</title>
</svelte:head>

<div class="page-content fade-in">
  <div class="flex items-center justify-between" style="margin-bottom:20px; flex-wrap:wrap; gap:12px;">
    <div>
      <h1 class="text-2xl font-bold">Superadmin Console</h1>
      <p class="text-sm text-muted" style="margin-top:2px;">Manage user accounts, bulk assign project admins, and refresh snapshots across all projects.</p>
    </div>
    <div class="flex gap-2">
      <button class="btn btn-secondary btn-sm" onclick={() => goto('/')}>← All Teams</button>
    </div>
  </div>

  {#if $user && $user.role !== 'admin'}
    <div class="badge badge-danger" style="padding:12px 20px; border-radius:8px; font-size:14px; text-transform:none; letter-spacing:0; font-weight:400;">
      ⚠️ Super admin access required.
    </div>
  {:else}
    <!-- Tab navigation -->
    <div style="display:flex; gap:8px; margin-bottom:20px; border-bottom:1px solid var(--c-border); padding-bottom:8px;">
      <button
        class="btn btn-sm {activeTab === 'users' ? 'btn-primary' : 'btn-ghost'}"
        onclick={() => activeTab = 'users'}
        style="display:flex; align-items:center; gap:6px;"
      >
        <span>👥</span> User Accounts ({users.length})
      </button>
      <button
        class="btn btn-sm {activeTab === 'operations' ? 'btn-primary' : 'btn-ghost'}"
        onclick={() => activeTab = 'operations'}
        style="display:flex; align-items:center; gap:6px;"
      >
        <span>⚡</span> All Projects Operations
      </button>
      <button
        class="btn btn-sm {activeTab === 'autorefresh' ? 'btn-primary' : 'btn-ghost'}"
        onclick={() => { activeTab = 'autorefresh'; loadAutoRefreshData(); }}
        style="display:flex; align-items:center; gap:6px;"
      >
        <span>⏰</span> Auto-Refresh Jobs ({autoRefreshJobs.length})
      </button>
    </div>

    {#if error}
      <div class="badge badge-danger" style="margin-bottom:16px; padding:10px 16px; border-radius:8px; font-size:13px; text-transform:none; letter-spacing:0; font-weight:400; display:block;">
        ⚠️ {error}
      </div>
    {/if}
    {#if notice}
      <div class="badge badge-success" style="margin-bottom:16px; padding:10px 16px; border-radius:8px; font-size:13px; text-transform:none; letter-spacing:0; font-weight:400; display:block;">
        {notice}
      </div>
    {/if}

    {#if activeTab === 'users'}
      <!-- Add User Form -->
      <div class="card" style="margin-bottom:20px;">
        <div class="card-header">
          <h2 class="font-semibold" style="font-size:15px;">Add user</h2>
        </div>
        <div class="card-body">
          <form onsubmit={createUser} class="flex gap-2" style="flex-wrap:wrap; align-items:flex-end;">
            <div class="form-group" style="min-width:180px; flex:1;">
              <label class="label" for="nu">Username</label>
              <input id="nu" class="input" bind:value={newUsername} placeholder="e.g. jane" required />
            </div>
            <div class="form-group" style="min-width:180px; flex:1;">
              <label class="label" for="np">Password</label>
              <input id="np" type="password" class="input" bind:value={newPassword} placeholder="Min 4 characters" required autocomplete="new-password" />
            </div>
            <div class="form-group">
              <label class="label" for="nr">Role</label>
              <select id="nr" class="input" bind:value={newRole}>
                <option value="user">user</option>
                <option value="admin">admin</option>
              </select>
            </div>
            <button type="submit" class="btn btn-primary btn-sm" disabled={creating}>
              {creating ? 'Adding…' : 'Add user'}
            </button>
          </form>
        </div>
      </div>

      <!-- Users Table -->
      <div class="card">
        <div class="card-header">
          <h2 class="font-semibold" style="font-size:15px;">Users ({users.length})</h2>
        </div>
        <div class="card-body" style="padding:0; overflow-x:auto;">
          {#if loading}
            <div class="flex items-center gap-3" style="padding:24px;">
              <span class="spinner"></span>
              <span class="text-muted">Loading users…</span>
            </div>
          {:else if users.length === 0}
            <div class="empty-state"><p>No users found.</p></div>
          {:else}
            <table class="table">
              <thead>
                <tr>
                  <th>Username</th>
                  <th>Role</th>
                  <th>Created</th>
                  <th>Set new password</th>
                  <th>Project Admin Access</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {#each users as u (u.id)}
                  <tr>
                    <td class="font-medium">
                      {u.username}
                      {#if u.is_super_admin}
                        <span class="badge badge-primary" style="margin-left:8px;">super admin</span>
                      {/if}
                      {#if $user && $user.id === u.id}
                        <span class="badge badge-default" style="margin-left:8px;">you</span>
                      {/if}
                    </td>
                    <td>
                      <select
                        class="input"
                        style="width:auto; padding:4px 8px; font-size:12px;"
                        value={u.role}
                        disabled={u.is_super_admin || ($user && $user.id === u.id)}
                        onchange={(e) => changeRole(u, e.currentTarget.value)}
                      >
                        <option value="user">user</option>
                        <option value="admin">admin</option>
                      </select>
                    </td>
                    <td class="text-sm text-muted">{u.created_at ? new Date(u.created_at).toLocaleDateString() : '—'}</td>
                    <td>
                      <div class="flex gap-2">
                        <input
                          type="password"
                          class="input"
                          style="max-width:160px; padding:4px 8px; font-size:12px;"
                          placeholder="New password"
                          value={resetPasswords[u.id] || ''}
                          oninput={(e) => resetPasswords[u.id] = e.currentTarget.value}
                          autocomplete="new-password"
                        />
                        <button
                          class="btn btn-secondary btn-sm"
                          onclick={() => resetPassword(u.id)}
                          disabled={!!resetting[u.id]}
                        >
                          {resetting[u.id] ? 'Saving…' : 'Set'}
                        </button>
                      </div>
                    </td>
                    <td>
                      <button
                        class="btn btn-secondary btn-sm"
                        style="font-size:11px; padding:4px 10px; white-space:nowrap;"
                        onclick={() => addAdminToAllProjects(u.username, u.id)}
                        disabled={addingToAll[u.id]}
                        title="Add this user as an administrator to all projects"
                      >
                        {addingToAll[u.id] ? 'Adding…' : 'Add to All Projects'}
                      </button>
                    </td>
                    <td style="text-align:right;">
                      <button
                        class="btn btn-danger btn-sm"
                        onclick={() => deleteUser(u)}
                        disabled={u.is_super_admin || ($user && $user.id === u.id)}
                        title={u.is_super_admin ? 'Cannot delete the super admin account' : 'Delete user'}
                      >
                        Delete
                      </button>
                    </td>
                  </tr>
                {/each}
              </tbody>
            </table>
          {/if}
        </div>
      </div>
    {:else}
      <!-- Operations Tab: Bulk Actions -->
      <div style="display:flex; flex-direction:column; gap:24px;">

        <!-- Section 1: Snapshot Refresh for All Projects -->
        <div class="card">
          <div class="card-header" style="display:flex; align-items:center; justify-content:space-between; flex-wrap:wrap; gap:12px;">
            <div class="flex items-center gap-2">
              <span style="font-size:18px;">🔄</span>
              <div>
                <h2 class="font-semibold" style="font-size:16px;">Refresh Snapshots Across All Projects</h2>
                <p class="text-xs text-muted" style="margin-top:2px;">Contacts Jira for every project with configured credentials and updates all snapshot issue and worklog metrics.</p>
              </div>
            </div>
            <button
              class="btn btn-primary"
              style="display:inline-flex; align-items:center; gap:8px;"
              onclick={refreshAllSnapshots}
              disabled={bulkRefreshing}
            >
              {#if bulkRefreshing}
                <span class="spinner" style="width:14px; height:14px;"></span>
                <span>Refreshing all projects…</span>
              {:else}
                <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg>
                <span>Refresh Snapshots of All Projects</span>
              {/if}
            </button>
          </div>
          <div class="card-body">
            {#if bulkRefreshResult}
              <!-- Metric Tiles -->
              <div style="display:grid; grid-template-columns: repeat(auto-fit, minmax(130px, 1fr)); gap:12px; margin-bottom:20px;">
                <div style="background:var(--c-surface-2, #f8fafc); border:1px solid var(--c-border); border-radius:10px; padding:14px 16px;">
                  <div style="font-size:11px; font-weight:600; text-transform:uppercase; color:var(--c-text-3);">Total Snapshots</div>
                  <div style="font-size:24px; font-weight:800; color:var(--c-text); margin-top:4px;">{bulkRefreshResult.total}</div>
                </div>
                <div style="background:rgba(16, 185, 129, 0.08); border:1px solid rgba(16, 185, 129, 0.25); border-radius:10px; padding:14px 16px;">
                  <div style="font-size:11px; font-weight:600; text-transform:uppercase; color:var(--c-success);">Refreshed</div>
                  <div style="font-size:24px; font-weight:800; color:var(--c-success); margin-top:4px;">{bulkRefreshResult.refreshed}</div>
                </div>
                <div style="background:rgba(245, 158, 11, 0.08); border:1px solid rgba(245, 158, 11, 0.25); border-radius:10px; padding:14px 16px;">
                  <div style="font-size:11px; font-weight:600; text-transform:uppercase; color:#d97706;">Skipped</div>
                  <div style="font-size:24px; font-weight:800; color:#d97706; margin-top:4px;">{bulkRefreshResult.skipped}</div>
                </div>
                <div style="background:rgba(239, 68, 68, 0.08); border:1px solid rgba(239, 68, 68, 0.25); border-radius:10px; padding:14px 16px;">
                  <div style="font-size:11px; font-weight:600; text-transform:uppercase; color:var(--c-danger);">Failed</div>
                  <div style="font-size:24px; font-weight:800; color:var(--c-danger); margin-top:4px;">{bulkRefreshResult.failed}</div>
                </div>
              </div>

              <!-- Results Detail Table -->
              <h3 class="font-semibold text-sm" style="margin-bottom:10px;">Results Breakdown</h3>
              <div style="overflow-x:auto;">
                <table class="table">
                  <thead>
                    <tr>
                      <th>Project</th>
                      <th>Snapshot</th>
                      <th>Status</th>
                      <th>Details / Error</th>
                      <th></th>
                    </tr>
                  </thead>
                  <tbody>
                    {#each bulkRefreshResult.results as r}
                      <tr>
                        <td class="font-semibold">{r.team_name}</td>
                        <td>{r.snapshot_name}</td>
                        <td>
                          {#if r.status === 'refreshed'}
                            <span class="badge badge-success">✓ Refreshed</span>
                          {:else if r.status === 'skipped'}
                            <span class="badge badge-warning">Skipped</span>
                          {:else}
                            <span class="badge badge-danger">Failed</span>
                          {/if}
                        </td>
                        <td class="text-xs {r.error ? 'text-danger' : 'text-muted'}">
                          {r.error || 'Successfully synced latest data'}
                        </td>
                        <td style="text-align:right;">
                          <a href="/teams/{r.team_id}/snapshots/{r.snapshot_id}" class="btn btn-secondary btn-sm" style="font-size:11px; padding:3px 8px;">
                            View Snapshot →
                          </a>
                        </td>
                      </tr>
                    {/each}
                  </tbody>
                </table>
              </div>
            {:else}
              <p class="text-sm text-muted">Click the button above to begin refreshing snapshots across all projects. You will see a breakdown of the results here.</p>
            {/if}
          </div>
        </div>

        <!-- Section 2: Add Admins to All Projects -->
        <div class="card">
          <div class="card-header">
            <div class="flex items-center gap-2">
              <span style="font-size:18px;">👥</span>
              <div>
                <h2 class="font-semibold" style="font-size:16px;">Add Admin to All Projects</h2>
                <p class="text-xs text-muted" style="margin-top:2px;">Grant a user administrator privileges across every existing project in a single step.</p>
              </div>
            </div>
          </div>
          <div class="card-body">
            <div class="flex gap-3" style="max-width:540px; align-items:flex-end; flex-wrap:wrap;">
              <div class="form-group" style="flex:1; min-width:220px;">
                <label class="label" for="bulk-user-select" style="font-size:12px;">Select user to make admin of all projects</label>
                <select id="bulk-user-select" class="input" bind:value={bulkSelectedUsername}>
                  {#each users as u}
                    <option value={u.username}>
                      {u.username} ({u.role}{u.is_super_admin ? ', super admin' : ''})
                    </option>
                  {/each}
                </select>
              </div>
              <button
                class="btn btn-primary"
                onclick={() => addAdminToAllProjects(bulkSelectedUsername)}
                disabled={bulkAddingAdmin || !bulkSelectedUsername}
                style="height:38px; display:inline-flex; align-items:center; gap:6px;"
              >
                {#if bulkAddingAdmin}
                  <span class="spinner" style="width:14px; height:14px;"></span>
                  <span>Applying to all projects…</span>
                {:else}
                  <span>Add as Admin to All Projects</span>
                {/if}
              </button>
            </div>
            <p class="text-xs text-muted" style="margin-top:12px;">
              Note: This assigns the selected user to the <code>team_admins</code> table for every project.
            </p>
          </div>
        </div>

      </div>
    {:else if activeTab === 'autorefresh'}
      <!-- Auto-Refresh Jobs & Audit Logs Tab -->
      <div style="display:flex; flex-direction:column; gap:20px;">
        <!-- KPI Summary Cards -->
        <div style="display:grid; grid-template-columns:repeat(auto-fit, minmax(200px, 1fr)); gap:16px;">
          <div class="card" style="padding:16px;">
            <p class="text-xs text-muted font-semibold" style="text-transform:uppercase; letter-spacing:0.05em; font-size:10px;">Total Configured Jobs</p>
            <p class="text-2xl font-bold" style="margin-top:4px; color:var(--c-primary);">{autoRefreshJobs.length}</p>
            <p class="text-xs text-muted" style="margin-top:2px;">Across all projects</p>
          </div>
          <div class="card" style="padding:16px;">
            <p class="text-xs text-muted font-semibold" style="text-transform:uppercase; letter-spacing:0.05em; font-size:10px;">Active Today</p>
            <p class="text-2xl font-bold" style="margin-top:4px; color:var(--c-success);">{activeJobsCount}</p>
            <p class="text-xs text-muted" style="margin-top:2px;">Currently within sprint dates</p>
          </div>
          <div class="card" style="padding:16px;">
            <p class="text-xs text-muted font-semibold" style="text-transform:uppercase; letter-spacing:0.05em; font-size:10px;">Upcoming / Expired</p>
            <p class="text-2xl font-bold" style="margin-top:4px;">
              <span style="color:var(--c-warning);">{upcomingJobsCount}</span>
              <span class="text-muted" style="font-weight:normal; font-size:18px;"> / </span>
              <span style="color:var(--c-text-3);">{expiredJobsCount}</span>
            </p>
            <p class="text-xs text-muted" style="margin-top:2px;">{upcomingJobsCount} upcoming · {expiredJobsCount} ended</p>
          </div>
          <div class="card" style="padding:16px;">
            <p class="text-xs text-muted font-semibold" style="text-transform:uppercase; letter-spacing:0.05em; font-size:10px;">Audit Events</p>
            <p class="text-2xl font-bold" style="margin-top:4px;">{autoRefreshAudits.length}</p>
            <p class="text-xs text-muted" style="margin-top:2px;">Logged execution records</p>
          </div>
        </div>

        <!-- All Projects Auto-Refresh Jobs Card -->
        <div class="card">
          <div class="card-header" style="display:flex; justify-content:space-between; align-items:center; flex-wrap:wrap; gap:12px;">
            <div>
              <h2 class="font-semibold" style="font-size:15px; display:flex; align-items:center; gap:8px;">
                <span>⏰</span> All Projects Auto-Refresh Jobs ({filteredJobs.length})
              </h2>
              <p class="text-xs text-muted" style="margin-top:2px; margin-bottom:0;">
                All snapshot refresh schedules enabled across projects. Superadmins can monitor, test run, or disable any job.
              </p>
            </div>
            <div class="flex gap-2" style="align-items:center;">
              <input
                type="text"
                class="input"
                placeholder="Search project or snapshot…"
                bind:value={autoRefreshSearch}
                style="font-size:12px; padding:6px 12px; width:220px;"
              />
              <button
                class="btn btn-secondary btn-sm"
                onclick={loadAutoRefreshData}
                disabled={loadingAutoRefresh}
                style="display:inline-flex; align-items:center; gap:4px;"
              >
                {#if loadingAutoRefresh}
                  <span class="spinner" style="width:12px; height:12px;"></span>
                {:else}
                  <span>↻</span>
                {/if}
                <span>Refresh</span>
              </button>
            </div>
          </div>
          <div class="card-body" style="padding:0; overflow-x:auto;">
            {#if loadingAutoRefresh && autoRefreshJobs.length === 0}
              <div class="flex items-center gap-3" style="padding:24px;">
                <span class="spinner"></span>
                <span class="text-muted">Loading auto-refresh jobs…</span>
              </div>
            {:else if filteredJobs.length === 0}
              <div class="empty-state" style="padding:32px 20px; text-align:center;">
                <p style="font-size:13px; color:var(--c-text-3); margin:0;">
                  {#if autoRefreshSearch.trim()}
                    No auto-refresh jobs matching "{autoRefreshSearch}".
                  {:else}
                    No snapshot auto-refresh jobs are currently enabled in any project.
                  {/if}
                </p>
              </div>
            {:else}
              <table class="table" style="width:100%; font-size:12px;">
                <thead>
                  <tr style="background:var(--c-bg); border-bottom:1px solid var(--c-border); color:var(--c-text-2);">
                    <th style="padding:10px 16px; text-align:left;">Project</th>
                    <th style="padding:10px 16px; text-align:left;">Snapshot</th>
                    <th style="padding:10px 16px; text-align:left;">Sprint Window & Status</th>
                    <th style="padding:10px 16px; text-align:left;">Daily Time</th>
                    <th style="padding:10px 16px; text-align:left;">Next Scheduled Run</th>
                    <th style="padding:10px 16px; text-align:left;">Last Auto-Refresh</th>
                    <th style="padding:10px 16px; text-align:right;">Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {#each filteredJobs as job}
                    <tr style="border-bottom:1px solid var(--c-border);">
                      <td style="padding:10px 16px;">
                        <a href="/teams/{job.team_id}" class="font-semibold" style="color:var(--c-primary); text-decoration:none;">
                          {job.team_name}
                        </a>
                      </td>
                      <td style="padding:10px 16px;">
                        <a href="/teams/{job.team_id}/snapshots/{job.snapshot_id}" class="font-medium" style="color:var(--c-text); text-decoration:none;">
                          📸 {job.snapshot_name}
                        </a>
                      </td>
                      <td style="padding:10px 16px;">
                        <div style="display:flex; align-items:center; gap:8px;">
                          <span>{job.start_date} – {job.end_date}</span>
                          {#if job.active_status === 'active'}
                            <span class="badge badge-success" style="font-size:10px; padding:2px 6px;">🟢 Active</span>
                          {:else if job.active_status === 'upcoming'}
                            <span class="badge badge-warning" style="font-size:10px; padding:2px 6px;">🟡 Upcoming</span>
                          {:else}
                            <span class="badge badge-secondary" style="font-size:10px; padding:2px 6px;">⚪ Expired</span>
                          {/if}
                        </div>
                      </td>
                      <td style="padding:10px 16px; font-weight:600;">
                        ⏰ {job.auto_refresh_time}
                      </td>
                      <td style="padding:10px 16px; font-family:ui-monospace, monospace; color:var(--c-text-2);">
                        {formatDateTime(job.next_run_at)}
                      </td>
                      <td style="padding:10px 16px;">
                        {#if job.last_auto_refreshed_at}
                          <div style="display:flex; flex-direction:column; gap:2px;">
                            <span style="font-size:11px;">{formatDateTime(job.last_auto_refreshed_at)}</span>
                            <div>
                              {#if job.last_auto_refresh_status === 'success'}
                                <span class="badge badge-success" style="font-size:9px; padding:1px 5px;">Success</span>
                              {:else if job.last_auto_refresh_status === 'skipped'}
                                <span class="badge badge-warning" style="font-size:9px; padding:1px 5px;" title={job.last_auto_refresh_msg}>Skipped</span>
                              {:else}
                                <span class="badge badge-danger" style="font-size:9px; padding:1px 5px;" title={job.last_auto_refresh_msg}>Failed</span>
                              {/if}
                            </div>
                          </div>
                        {:else}
                          <span class="text-muted">—</span>
                        {/if}
                      </td>
                      <td style="padding:10px 16px; text-align:right;">
                        <div class="flex gap-2 justify-end" style="align-items:center;">
                          <button
                            class="btn btn-secondary btn-sm"
                            style="font-size:11px; padding:4px 8px; display:inline-flex; align-items:center; gap:4px;"
                            onclick={() => runJobNow(job)}
                            disabled={runningJobId === job.snapshot_id}
                            title="Trigger test auto-refresh now"
                          >
                            {#if runningJobId === job.snapshot_id}
                              <span class="spinner" style="width:10px; height:10px;"></span>
                              <span>Running…</span>
                            {:else}
                              <span>⚡ Run Now</span>
                            {/if}
                          </button>
                          <button
                            class="btn btn-outline-danger btn-sm"
                            style="font-size:11px; padding:4px 8px; display:inline-flex; align-items:center; gap:4px; color:var(--c-danger); border:1px solid var(--c-danger); background:transparent;"
                            onclick={() => disableJob(job)}
                            disabled={disablingJobId === job.snapshot_id}
                            title="Disable auto-refresh for this snapshot"
                          >
                            {#if disablingJobId === job.snapshot_id}
                              <span class="spinner" style="width:10px; height:10px;"></span>
                              <span>Disabling…</span>
                            {:else}
                              <span>Disable</span>
                            {/if}
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

        <!-- Global Auto-Refresh Audit Logs Card -->
        <div class="card">
          <div class="card-header" style="display:flex; justify-content:space-between; align-items:center; flex-wrap:wrap; gap:12px;">
            <div>
              <h2 class="font-semibold" style="font-size:15px; display:flex; align-items:center; gap:8px;">
                <span>📋</span> Global Auto-Refresh Audit Logs ({filteredAudits.length})
              </h2>
              <p class="text-xs text-muted" style="margin-top:2px; margin-bottom:0;">
                Complete audit history of automated scheduled runs and manual triggers across all projects.
              </p>
            </div>
            <div class="flex gap-2" style="align-items:center; flex-wrap:wrap;">
              <!-- Status filter buttons -->
              <div style="display:flex; gap:4px; background:var(--c-bg); border:1px solid var(--c-border); border-radius:6px; padding:2px;">
                <button
                  class="btn btn-sm {autoRefreshFilterStatus === 'all' ? 'btn-primary' : 'btn-ghost'}"
                  style="font-size:11px; padding:3px 8px; height:auto;"
                  onclick={() => autoRefreshFilterStatus = 'all'}
                >
                  All
                </button>
                <button
                  class="btn btn-sm {autoRefreshFilterStatus === 'success' ? 'btn-primary' : 'btn-ghost'}"
                  style="font-size:11px; padding:3px 8px; height:auto;"
                  onclick={() => autoRefreshFilterStatus = 'success'}
                >
                  Success
                </button>
                <button
                  class="btn btn-sm {autoRefreshFilterStatus === 'skipped' ? 'btn-primary' : 'btn-ghost'}"
                  style="font-size:11px; padding:3px 8px; height:auto;"
                  onclick={() => autoRefreshFilterStatus = 'skipped'}
                >
                  Skipped
                </button>
                <button
                  class="btn btn-sm {autoRefreshFilterStatus === 'failed' ? 'btn-primary' : 'btn-ghost'}"
                  style="font-size:11px; padding:3px 8px; height:auto;"
                  onclick={() => autoRefreshFilterStatus = 'failed'}
                >
                  Failed
                </button>
              </div>
            </div>
          </div>
          <div class="card-body" style="padding:0; overflow-x:auto;">
            {#if loadingAutoRefresh && autoRefreshAudits.length === 0}
              <div class="flex items-center gap-3" style="padding:24px;">
                <span class="spinner"></span>
                <span class="text-muted">Loading audit logs…</span>
              </div>
            {:else if filteredAudits.length === 0}
              <div class="empty-state" style="padding:32px 20px; text-align:center;">
                <p style="font-size:13px; color:var(--c-text-3); margin:0;">
                  No audit log entries found.
                </p>
              </div>
            {:else}
              <table class="table" style="width:100%; font-size:12px;">
                <thead>
                  <tr style="background:var(--c-bg); border-bottom:1px solid var(--c-border); color:var(--c-text-2);">
                    <th style="padding:10px 16px; text-align:left;">Triggered At</th>
                    <th style="padding:10px 16px; text-align:left;">Project</th>
                    <th style="padding:10px 16px; text-align:left;">Snapshot</th>
                    <th style="padding:10px 16px; text-align:left;">Trigger</th>
                    <th style="padding:10px 16px; text-align:left;">Status</th>
                    <th style="padding:10px 16px; text-align:right;">Delta</th>
                    <th style="padding:10px 16px; text-align:right;">Duration</th>
                    <th style="padding:10px 16px; text-align:left;">Details / Message</th>
                  </tr>
                </thead>
                <tbody>
                  {#each filteredAudits as a}
                    <tr style="border-bottom:1px solid var(--c-border);">
                      <td style="padding:10px 16px; font-family:ui-monospace, monospace; color:var(--c-text-2); white-space:nowrap;">
                        {formatDateTime(a.triggered_at)}
                      </td>
                      <td style="padding:10px 16px;">
                        <a href="/teams/{a.team_id}" style="color:var(--c-primary); text-decoration:none; font-weight:500;">
                          {a.team_name || a.team_id}
                        </a>
                      </td>
                      <td style="padding:10px 16px;">
                        <a href="/teams/{a.team_id}/snapshots/{a.snapshot_id}" style="color:var(--c-text); text-decoration:none; font-weight:500;">
                          {a.snapshot_name || a.snapshot_id}
                        </a>
                      </td>
                      <td style="padding:10px 16px;">
                        <span class="badge {a.trigger_type === 'scheduled' ? 'badge-secondary' : 'badge-primary'}" style="font-size:10px; text-transform:capitalize;">
                          {a.trigger_type}
                        </span>
                      </td>
                      <td style="padding:10px 16px;">
                        {#if a.status === 'success'}
                          <span class="badge badge-success" style="font-size:10px;">Success</span>
                        {:else if a.status === 'skipped'}
                          <span class="badge badge-warning" style="font-size:10px;">Skipped</span>
                        {:else}
                          <span class="badge badge-danger" style="font-size:10px;">Failed</span>
                        {/if}
                      </td>
                      <td style="padding:10px 16px; text-align:right; white-space:nowrap;">
                        {#if a.status === 'success'}
                          <span style="font-weight:600; color:{a.hours_delta > 0 ? 'var(--c-success)' : 'var(--c-text)'};">
                            {a.hours_delta > 0 ? '+' : ''}{a.hours_delta.toFixed(1)}h
                          </span>
                          {#if a.story_points_delta !== 0}
                            <span class="text-muted" style="font-size:11px; margin-left:4px;">
                              ({a.story_points_delta > 0 ? '+' : ''}{a.story_points_delta.toFixed(1)} SP)
                            </span>
                          {/if}
                        {:else}
                          <span class="text-muted">—</span>
                        {/if}
                      </td>
                      <td style="padding:10px 16px; text-align:right; font-family:ui-monospace, monospace; color:var(--c-text-2); white-space:nowrap;">
                        {a.duration_ms}ms
                      </td>
                      <td style="padding:10px 16px; color:var(--c-text-2); max-width:320px; overflow:hidden; text-overflow:ellipsis; white-space:nowrap;" title={a.message || ''}>
                        {a.message || '—'}
                      </td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            {/if}
          </div>
        </div>
      </div>
    {/if}
  {/if}
</div>
