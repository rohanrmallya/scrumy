<script lang="ts">
  import { page } from '$app/stores';
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { api, type JiraSnapshot, type Plan } from '$lib/api';

  const planID = $derived($page.params.id);
  const snapshotID = $derived($page.params.snapid);

  let plan = $state<Plan | null>(null);
  let snapshot = $state<JiraSnapshot | null>(null);
  let loading = $state(true);
  let error = $state('');

  let leaderboardSearchQuery = $state('');
  let expandedDevelopers = $state<Record<string, boolean>>({});
  let refreshing = $state(false);
  let deleting = $state(false);

  let showAllWorklogs = $state(false);

  async function loadData() {
    loading = true;
    error = '';
    try {
      const [p, s] = await Promise.all([
        api.plans.get(planID),
        api.jira.getSnapshot(planID, snapshotID)
      ]);
      plan = p;
      snapshot = s;
      if (s) {
        showAllWorklogs = s.all_worklogs;
      }
    } catch (e: any) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  onMount(loadData);

  function formatDateTime(utcString: string) {
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

  let doneIssuesOnly = $state(false);

  // Derived filtered snapshot totals and leaderboard based on whether we only want worklog data of done tasks
  const computedData = $derived.by(() => {
    if (!snapshot || !snapshot.data) return {
      leaderboard: [],
      totalHoursLogged: 0,
      totalWorkLogs: 0,
      avgHoursPerSP: 0
    };

    const issues = snapshot.data.issues || [];
    
    // Choose correct source of totals & leaderboard based on showAllWorklogs toggle
    // Support fallback to window metrics if totals_all or leaderboard_all is missing (for older snapshots)
    const hasAllMetrics = !!snapshot.data.totals_all && !!snapshot.data.leaderboard_all;
    const activeShowAll = showAllWorklogs && hasAllMetrics;

    const baseLeaderboard = activeShowAll 
      ? (snapshot.data.leaderboard_all || []) 
      : (snapshot.data.leaderboard || []);
      
    const baseTotals = activeShowAll 
      ? (snapshot.data.totals_all || snapshot.data.totals) 
      : snapshot.data.totals;

    // Create lookup map for issue category/status
    const doneStatusMap: Record<string, boolean> = {};
    for (const issue of issues) {
      const isDone = (issue.status_category_key === 'done' || 
                      issue.status?.toLowerCase() === 'done');
      doneStatusMap[issue.key] = isDone;
    }

    if (!doneIssuesOnly) {
      return {
        leaderboard: baseLeaderboard,
        totalHoursLogged: baseTotals.total_hours_logged,
        totalWorkLogs: baseTotals.total_work_logs,
        avgHoursPerSP: baseTotals.avg_hours_per_sp,
      };
    }

    // Filtered leaderboard
    let totalHoursLogged = 0;
    let totalWorkLogs = 0;
    const newLeaderboard = baseLeaderboard.map(entry => {
      const filteredWorklogs = (entry.worklogs || []).filter(wl => doneStatusMap[wl.issue_key]);
      const hoursLogged = filteredWorklogs.reduce((sum, wl) => sum + wl.hours_logged, 0);
      
      totalHoursLogged += hoursLogged;
      totalWorkLogs += filteredWorklogs.length;

      return {
        ...entry,
        hours_logged: hoursLogged,
        worklogs: filteredWorklogs,
      };
    }).filter(entry => entry.hours_logged > 0);

    // Re-calculate percentages
    for (const entry of newLeaderboard) {
      entry.percentage = totalHoursLogged > 0 ? (entry.hours_logged / totalHoursLogged) * 100 : 0;
    }

    // Sort leaderboard descending by hours
    newLeaderboard.sort((a, b) => b.hours_logged - a.hours_logged);

    // Re-calculate average hours per SP
    const totalSP = baseTotals.total_story_points;
    const avgHoursPerSP = totalSP > 0 ? totalHoursLogged / totalSP : 0;

    return {
      leaderboard: newLeaderboard,
      totalHoursLogged: Math.round(totalHoursLogged * 100) / 100,
      totalWorkLogs,
      avgHoursPerSP: Math.round(avgHoursPerSP * 100) / 100,
    };
  });

  const filteredLeaderboard = $derived(
    (computedData?.leaderboard || []).filter(entry =>
      entry.author_name.toLowerCase().includes(leaderboardSearchQuery.trim().toLowerCase())
    )
  );

  // Edit Snapshot Modal state
  let showEditModal = $state(false);
  let editSnapshotName = $state('');
  let editSnapshotStart = $state('');
  let editSnapshotEnd = $state('');
  let updatingSnapshot = $state(false);
  let editSnapshotError = $state('');

  function openEditSnapshot() {
    if (!snapshot) return;
    editSnapshotName = snapshot.name;
    editSnapshotStart = snapshot.start_date;
    editSnapshotEnd = snapshot.end_date;
    editSnapshotError = '';
    showEditModal = true;
  }

  async function saveEditedSnapshot() {
    if (!snapshot) return;
    if (!editSnapshotName.trim() || !editSnapshotStart || !editSnapshotEnd) {
      editSnapshotError = 'Name, start date, and end date are required';
      return;
    }
    updatingSnapshot = true;
    editSnapshotError = '';
    try {
      const updated = await api.jira.updateSnapshot(planID, snapshot.id, {
        name: editSnapshotName.trim(),
        start_date: editSnapshotStart,
        end_date: editSnapshotEnd,
      });
      snapshot = updated;
      showEditModal = false;
    } catch (e: any) {
      editSnapshotError = e.message;
    } finally {
      updatingSnapshot = false;
    }
  }

  async function refreshSnapshot() {
    if (!confirm('Re-fetch snapshot data from Jira?')) return;
    refreshing = true;
    try {
      const updated = await api.jira.refreshSnapshot(planID, snapshotID);
      snapshot = updated;
      if (updated) {
        showAllWorklogs = updated.all_worklogs;
      }
      leaderboardSearchQuery = '';
      expandedDevelopers = {};
      alert('Snapshot refreshed successfully!');
    } catch (e: any) {
      alert(`Failed to refresh: ${e.message}`);
    } finally {
      refreshing = false;
    }
  }

  async function deleteSnapshot() {
    if (!confirm('Are you sure you want to delete this snapshot?')) return;
    deleting = true;
    try {
      await api.jira.deleteSnapshot(planID, snapshotID);
      goto(`/plans/${planID}`);
    } catch (e: any) {
      alert(`Failed to delete: ${e.message}`);
      deleting = false;
    }
  }

  async function createRetroFromSnapshot() {
    if (!snapshot) return;
    try {
      const title = `${snapshot.name} Retro`;
      const pres = await api.presentations.create(planID, {
        type: 'retro',
        template_id: 'default',
        title: title,
        sprint_name: snapshot.name,
      });

      const content = {
        previous_data: {
          total_sp_delivered: snapshot.data.totals.total_story_points,
          total_hours_logged: computedData.totalHoursLogged,
          total_work_logs: computedData.totalWorkLogs,
          avg_hours_per_sp: computedData.avgHoursPerSP,
          planned_sp: 0,
          executed_sp: snapshot.data.totals.total_story_points,
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
        sprint_name: snapshot.name,
        content: content,
      });

      goto(`/plans/${planID}/presentations/${pres.id}/edit`);
    } catch (e: any) {
      alert(`Failed to create presentation: ${e.message}`);
    }
  }

  function exportToCSV() {
    if (!snapshot || !computedData || !computedData.leaderboard) return;

    // Create lookup map for issue details to enrich worklogs
    const issueMap: Record<string, { story_points: number; status: string; status_changed: string }> = {};
    if (snapshot.data.issues) {
      for (const issue of snapshot.data.issues) {
        issueMap[issue.key] = {
          story_points: issue.story_points,
          status: issue.status,
          status_changed: issue.status_category_changed_date
        };
      }
    }

    // CSV Headers
    const headers = [
      'Developer',
      'Issue Key',
      'Issue Summary',
      'Story Points',
      'Issue Status',
      'Status Changed Date',
      'Logged Hours'
    ];

    const rows = [headers];

    for (const entry of computedData.leaderboard) {
      if (!entry.worklogs || entry.worklogs.length === 0) continue;
      for (const wl of entry.worklogs) {
        const issueDetails = issueMap[wl.issue_key] || { story_points: 0, status: 'Unknown', status_changed: '—' };
        
        let formattedDate = '—';
        if (issueDetails.status_changed && issueDetails.status_changed !== '—') {
          try {
            formattedDate = issueDetails.status_changed.split('T')[0] || issueDetails.status_changed;
          } catch {
            formattedDate = issueDetails.status_changed;
          }
        }

        rows.push([
          entry.author_name,
          wl.issue_key,
          wl.issue_summary || '',
          issueDetails.story_points > 0 ? issueDetails.story_points.toString() : '—',
          issueDetails.status || '—',
          formattedDate,
          wl.hours_logged.toFixed(1)
        ]);
      }
    }

    // Helper to escape CSV cell values according to RFC 4180
    const csvContent = rows
      .map(row => 
        row
          .map(val => {
            const escaped = val.replace(/"/g, '""');
            // Check if wrapping is needed
            if (escaped.includes(',') || escaped.includes('\n') || escaped.includes('\r') || escaped.includes('"')) {
              return `"${escaped}"`;
            }
            return escaped;
          })
          .join(',')
      )
      .join('\r\n'); // Use Windows-style line breaks for maximum compatibility in Excel

    // Create download link and trigger click
    const blob = new Blob([new Uint8Array([0xEF, 0xBB, 0xBF]), csvContent], { type: 'text/csv;charset=utf-8;' }); // Include UTF-8 BOM for Excel compatibility
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    
    // Construct meaningful filename e.g. "scrumy_worklogs_sprint_name.csv"
    const cleanedSnapshotName = snapshot.name.replace(/[^a-z0-9]/gi, '_').toLowerCase();
    link.setAttribute('href', url);
    link.setAttribute('download', `scrumy_worklogs_${cleanedSnapshotName}.csv`);
    link.style.visibility = 'hidden';
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    URL.revokeObjectURL(url);
  }
</script>

<svelte:head>
  <title>{snapshot?.name ?? 'Snapshot Details'} – Scrumy</title>
</svelte:head>

<div class="page-content fade-in">
  {#if loading}
    <div class="flex items-center gap-3" style="padding:40px 0">
      <span class="spinner"></span>
      <span class="text-muted">Loading snapshot…</span>
    </div>
  {:else if error}
    <div class="badge badge-danger" style="padding:12px 20px; border-radius:8px; font-size:14px; text-transform:none; letter-spacing:0; font-weight:400;">
      ⚠️ {error}
    </div>
    <div style="margin-top:20px;">
      <a href="/plans/{planID}" class="btn btn-secondary">← Back to Workspace</a>
    </div>
  {:else if snapshot}
    <!-- Back Navigation Link -->
    <div style="margin-bottom: 20px;">
      <a href="/plans/{planID}" class="btn btn-secondary btn-sm" style="display:inline-flex; align-items:center; gap:6px;">
        ← Back to Plan Workspace
      </a>
    </div>

    <!-- Header Actions Panel -->
    <div class="flex justify-between items-center" style="margin-bottom: 24px; gap: 16px; flex-wrap: wrap;">
      <div>
        <h1 class="text-2xl font-bold">{snapshot.name}</h1>
        <p class="text-xs text-muted" style="margin-top:4px; display:flex; align-items:center; gap:6px;">
          Range: {snapshot.start_date} to {snapshot.end_date}
          {#if showAllWorklogs && snapshot.data.totals_all}
            <span class="badge badge-warning" style="font-size:9px; padding:1px 6px; text-transform:none; font-weight:normal; letter-spacing:0;">all worklogs</span>
          {:else}
            <span class="badge badge-default" style="font-size:9px; padding:1px 6px; text-transform:none; font-weight:normal; letter-spacing:0;">filtered worklogs</span>
          {/if}
        </p>
      </div>
      <div class="flex gap-2">
        <button class="btn btn-secondary btn-sm" onclick={exportToCSV} style="display:inline-flex; align-items:center; gap:6px;">
          📥 Export to CSV
        </button>
        {#if plan?.is_admin}
          <button class="btn btn-secondary btn-sm" onclick={openEditSnapshot}>
            ✎ Edit Settings
          </button>
          <button class="btn btn-secondary btn-sm" onclick={refreshSnapshot} disabled={refreshing}>
            {refreshing ? 'Refreshing...' : '🔄 Refresh'}
          </button>
          <button class="btn btn-outline-danger btn-sm" onclick={deleteSnapshot} disabled={deleting}>
            {deleting ? 'Deleting...' : '✕ Delete'}
          </button>
        {/if}
        <button class="btn btn-primary btn-sm" onclick={createRetroFromSnapshot}>
          🎤 Create Retro Presentation
        </button>
      </div>
    </div>

    <!-- Layout Container -->
    <div style="display:flex; flex-direction:column; gap:24px;">
      
      <!-- Metrics Summary Cards -->
      <div style="display:grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap:16px;">
        <div style="background:var(--c-surface); padding:20px; border-radius:10px; border:1px solid var(--c-border); box-shadow:var(--shadow-sm);">
          <p class="text-xs text-muted font-semibold" style="text-transform:uppercase; letter-spacing:0.05em; font-size:10px;">SP Burned</p>
          <p class="text-3xl font-bold" style="margin-top:6px; color:var(--c-success);">{snapshot.data.totals.total_story_points}</p>
        </div>
        <div style="background:var(--c-surface); padding:20px; border-radius:10px; border:1px solid var(--c-border); box-shadow:var(--shadow-sm);">
          <p class="text-xs text-muted font-semibold" style="text-transform:uppercase; letter-spacing:0.05em; font-size:10px;">Hours Logged</p>
          <p class="text-3xl font-bold" style="margin-top:6px; color:var(--c-primary);">{computedData.totalHoursLogged} <span class="text-sm font-normal text-muted">hrs</span></p>
        </div>
        <div style="background:var(--c-surface); padding:20px; border-radius:10px; border:1px solid var(--c-border); box-shadow:var(--shadow-sm);">
          <p class="text-xs text-muted font-semibold" style="text-transform:uppercase; letter-spacing:0.05em; font-size:10px;">Worklogs Count</p>
          <p class="text-3xl font-bold" style="margin-top:6px;">{computedData.totalWorkLogs}</p>
        </div>
        <div style="background:var(--c-surface); padding:20px; border-radius:10px; border:1px solid var(--c-border); box-shadow:var(--shadow-sm);">
          <p class="text-xs text-muted font-semibold" style="text-transform:uppercase; letter-spacing:0.05em; font-size:10px;">Hours / SP</p>
          <p class="text-3xl font-bold" style="margin-top:6px;">{computedData.avgHoursPerSP} <span class="text-sm font-normal text-muted">avg</span></p>
        </div>
      </div>

      <!-- Leaderboard and Insights Block -->
      <div style="display:grid; grid-template-columns: 1fr; gap:24px; align-items:start;" class="grid-layout">
        
        <!-- Logged Hours Leaderboard Accordion Card -->
        <div class="card">
          <div class="card-header" style="display:flex; flex-direction:column; gap:12px; align-items:stretch; padding:16px 20px;">
            <div class="flex justify-between items-center">
              <h2 class="font-semibold text-base" style="display:flex; align-items:center; gap:8px; margin:0;">
                <span>🏆</span> Logged Hours Leaderboard
              </h2>
              <div class="flex items-center gap-3" style="flex-wrap: wrap;">
                <!-- Worklog Filter Switch -->
                <label 
                  style="display:inline-flex; align-items:center; gap:6px; font-size:12px; cursor:pointer; font-weight:normal; user-select:none; margin:0;"
                  title={!snapshot.data.totals_all ? "Click Refresh to enable this toggle for older snapshots" : ""}
                >
                  <input type="checkbox" bind:checked={showAllWorklogs} disabled={!snapshot.data.totals_all} />
                  <span>Include all worklogs (no date filter)</span>
                </label>

                <label style="display:inline-flex; align-items:center; gap:6px; font-size:12px; cursor:pointer; font-weight:normal; user-select:none; margin:0;">
                  <input type="checkbox" bind:checked={doneIssuesOnly} />
                  <span>Done Tasks Only</span>
                </label>
                <span class="badge badge-primary">{filteredLeaderboard.length} Developers</span>
              </div>
            </div>
            
            <!-- Search bar inline -->
            <div style="position:relative; width:100%;">
              <span style="position:absolute; left:12px; top:50%; transform:translateY(-50%); font-size:14px; color:var(--c-text-3);">🔍</span>
              <input 
                type="text" 
                class="input" 
                placeholder="Search contributors..." 
                bind:value={leaderboardSearchQuery} 
                style="padding-left:36px; background:var(--c-bg);"
              />
              {#if leaderboardSearchQuery}
                <button 
                  onclick={() => leaderboardSearchQuery = ''} 
                  style="position:absolute; right:12px; top:50%; transform:translateY(-50%); background:none; border:none; font-size:12px; color:var(--c-text-3); cursor:pointer;"
                >
                  ✕
                </button>
              {/if}
            </div>
          </div>

          <div style="display:flex; flex-direction:column; padding:12px 20px; gap:10px; background:var(--c-bg);">
            {#if !snapshot.data.leaderboard || snapshot.data.leaderboard.length === 0}
              <p class="text-xs text-muted" style="padding:20px 0; text-align:center;">No work logged within this period.</p>
            {:else if filteredLeaderboard.length === 0}
              <p class="text-xs text-muted" style="padding:20px 0; text-align:center;">No developers match your query.</p>
            {:else}
              {#each filteredLeaderboard as entry, idx}
                {@const isExpanded = !!expandedDevelopers[entry.author_name]}
                <div 
                  style="background:var(--c-surface); border:1px solid var(--c-border); border-radius:8px; overflow:hidden; transition:all 150ms ease; box-shadow:var(--shadow-sm);"
                >
                  <!-- Row Trigger Button -->
                  <button 
                    onclick={() => expandedDevelopers[entry.author_name] = !isExpanded}
                    style="width:100%; display:flex; align-items:center; justify-content:space-between; padding:16px 20px; background:none; border:none; text-align:left; cursor:pointer; gap:16px; outline:none;"
                    onmouseover={e => e.currentTarget.style.background = 'rgba(7C, 110, 245, 0.03)'}
                    onmouseout={e => e.currentTarget.style.background = 'none'}
                  >
                    <div style="display:flex; align-items:center; gap:14px; flex:1;">
                      <span 
                        style="font-size:12px; font-weight:700; color:var(--c-text-2); width:24px; height:24px; border-radius:50%; background:var(--c-surface-2); display:flex; align-items:center; justify-content:center;"
                      >
                        {idx + 1}
                      </span>
                      <div style="flex:1;">
                        <div style="display:flex; justify-content:between; align-items:center; margin-bottom:6px;">
                          <span class="font-bold text-sm" style="color:var(--c-text);">{entry.author_name}</span>
                          <span class="font-semibold text-sm" style="color:var(--c-primary);">{entry.hours_logged.toFixed(1)} hrs <span class="text-xs text-muted font-normal">({entry.percentage.toFixed(0)}%)</span></span>
                        </div>
                        <div style="width:100%; height:6px; background:var(--c-surface-2); border-radius:3px; overflow:hidden;">
                          <div style="height:100%; width:{entry.percentage}%; background:var(--c-primary); border-radius:3px;"></div>
                        </div>
                      </div>
                    </div>
                    <!-- Accordion Indicator -->
                    <span style="font-size:10px; color:var(--c-text-3); transition:transform 200ms ease; transform:rotate({isExpanded ? '90deg' : '0deg'});">
                      ▶
                    </span>
                  </button>

                  <!-- Expanded Tasks List -->
                  {#if isExpanded}
                    <div style="border-top:1px solid var(--c-border); padding:16px 20px; background:var(--c-surface);">
                      {#if !entry.worklogs || entry.worklogs.length === 0}
                        <div style="padding:10px 0; text-align:center; color:var(--c-text-3); font-size:11px;">
                          <p>No per-task worklogs recorded in snapshot.</p>
                          <p style="font-size:9px; margin-top:2px;">Re-generate or click "Refresh" to fetch worklogs detail from Jira.</p>
                        </div>
                      {:else}
                        <div style="overflow-x:auto;">
                          <table style="width:100%; border-collapse:collapse; text-align:left; font-size:12px;">
                            <thead>
                              <tr style="border-bottom:1px solid var(--c-border); color:var(--c-text-3); font-size:11px;">
                                <th style="padding:6px 8px; font-weight:600; width:110px;">Issue Key</th>
                                <th style="padding:6px 8px; font-weight:600;">Summary</th>
                                <th style="padding:6px 8px; font-weight:600; text-align:right; width:100px;">Logged Hours</th>
                              </tr>
                            </thead>
                            <tbody>
                              {#each entry.worklogs as wl}
                                <tr style="border-bottom:1px solid var(--c-border-2);">
                                  <td style="padding:8px; font-weight:600; white-space:nowrap;">
                                    {#if plan?.jira_url}
                                      <a href="{plan.jira_url}/browse/{wl.issue_key}" target="_blank" class="text-primary" style="text-decoration:none;">{wl.issue_key}</a>
                                    {:else}
                                      {wl.issue_key}
                                    {/if}
                                  </td>
                                  <td style="padding:8px; max-width:320px; overflow:hidden; text-overflow:ellipsis; white-space:nowrap;" title={wl.issue_summary}>
                                    {wl.issue_summary || '—'}
                                  </td>
                                  <td style="padding:8px; text-align:right; font-weight:600; color:var(--c-primary);">
                                    {wl.hours_logged.toFixed(1)} hrs
                                  </td>
                                </tr>
                              {/each}
                            </tbody>
                          </table>
                        </div>
                      {/if}
                    </div>
                  {/if}
                </div>
              {/each}
            {/if}
          </div>
        </div>

        <!-- Right Column Panel (Insights & History) -->
        <div style="display:flex; flex-direction:column; gap:24px;">
          <!-- Info Insights card below or adjacent -->
          <div style="background:var(--c-surface); border-radius:10px; padding:20px; border:1px solid var(--c-border); box-shadow:var(--shadow-sm); display:flex; flex-direction:column; gap:12px;">
            <h3 class="font-semibold text-sm" style="display:flex; align-items:center; gap:6px;">
              <span>ℹ️</span> Snapshot Insights
            </h3>
            <p class="text-xs text-muted" style="line-height:1.6; margin:0;">
              This analysis is generated dynamically by querying Jira Cloud. Sprints metrics include issues matching JQL that were set to a completed state category during the snapshot dates.
            </p>
            <p class="text-xs text-muted" style="line-height:1.6; margin:0;">
              The leaderboard highlights time logs created <strong>strictly within the start/end dates</strong> of the snapshot on the retrieved issues.
            </p>
            <div style="border-top:1px solid var(--c-border); padding-top:12px; font-size:11px; display:flex; flex-direction:column; gap:8px;">
              <div class="flex justify-between">
                <span class="text-muted">Closed Issues count:</span>
                <span class="font-semibold">{snapshot.data.issues?.length || 0}</span>
              </div>
              <div class="flex justify-between">
                <span class="text-muted">Total story points delivered:</span>
                <span class="font-semibold text-success">{snapshot.data.totals.total_story_points}</span>
              </div>
            </div>
          </div>

          <!-- Refresh History Card -->
          <div class="card">
            <div class="card-header" style="padding:16px 20px; border-bottom: 1px solid var(--c-border-2);">
              <h3 class="font-semibold text-sm" style="margin:0; display:flex; align-items:center; gap:8px;">
                <span>🔄</span> Refresh History
              </h3>
            </div>
            <div class="card-body" style="padding: 16px 20px; display:flex; flex-direction:column; gap:16px; max-height:380px; overflow-y:auto; background:var(--c-bg);">
              {#if !snapshot.refresh_history || snapshot.refresh_history.length === 0}
                <p class="text-xs text-muted" style="text-align:center; padding:12px 0; margin:0;">No refreshes recorded yet.</p>
              {:else}
                <div style="display:flex; flex-direction:column; gap:16px;">
                  {#each snapshot.refresh_history as log}
                    {@const spDiff = log.new_story_points - log.prev_story_points}
                    {@const hoursDiff = log.new_hours_logged - log.prev_hours_logged}
                    {@const worklogsDiff = log.new_worklogs_count - log.prev_worklogs_count}
                    {@const issuesDiff = log.new_issues_count - log.prev_issues_count}
                    {@const hasChanges = spDiff !== 0 || hoursDiff !== 0 || worklogsDiff !== 0 || issuesDiff !== 0}

                    <div style="background:var(--c-surface); border:1px solid var(--c-border); border-radius:8px; padding:14px; box-shadow:var(--shadow-sm); display:flex; flex-direction:column; gap:10px;">
                      <div style="display:flex; justify-content:space-between; align-items:center;">
                        <span class="font-bold text-xs" style="color:var(--c-text);">{formatDateTime(log.refreshed_at)}</span>
                        {#if hasChanges}
                          <span class="badge badge-success" style="font-size:9px; padding:1px 6px;">Changes</span>
                        {:else}
                          <span class="badge badge-default" style="font-size:9px; padding:1px 6px; background:var(--c-surface-2); color:var(--c-text-3);">No changes</span>
                        {/if}
                      </div>

                      <div style="display:grid; grid-template-columns: 1fr 1fr; gap:8px; font-size:11px; border-top:1px solid var(--c-border-2); padding-top:8px;">
                        
                        <!-- Story Points -->
                        <div style="display:flex; flex-direction:column; gap:2px;">
                          <span class="text-muted">Story Points:</span>
                          <span style="font-weight:600; display:inline-flex; align-items:center; gap:4px;">
                            {log.new_story_points} SP
                            {#if spDiff > 0}
                              <span style="color:var(--c-success); font-weight:700;">(+{spDiff.toFixed(1)})</span>
                            {:else if spDiff < 0}
                              <span style="color:var(--c-danger); font-weight:700;">({spDiff.toFixed(1)})</span>
                            {/if}
                          </span>
                        </div>

                        <!-- Logged Hours -->
                        <div style="display:flex; flex-direction:column; gap:2px;">
                          <span class="text-muted">Hours Logged:</span>
                          <span style="font-weight:600; display:inline-flex; align-items:center; gap:4px;">
                            {log.new_hours_logged.toFixed(1)} hrs
                            {#if hoursDiff > 0}
                              <span style="color:var(--c-success); font-weight:700;">(+{hoursDiff.toFixed(1)})</span>
                            {:else if hoursDiff < 0}
                              <span style="color:var(--c-danger); font-weight:700;">({hoursDiff.toFixed(1)})</span>
                            {/if}
                          </span>
                        </div>

                        <!-- Closed Issues -->
                        <div style="display:flex; flex-direction:column; gap:2px;">
                          <span class="text-muted">Issues Closed:</span>
                          <span style="font-weight:600; display:inline-flex; align-items:center; gap:4px;">
                            {log.new_issues_count}
                            {#if issuesDiff > 0}
                              <span style="color:var(--c-success); font-weight:700;">(+{issuesDiff})</span>
                            {:else if issuesDiff < 0}
                              <span style="color:var(--c-danger); font-weight:700;">({issuesDiff})</span>
                            {/if}
                          </span>
                        </div>

                        <!-- Worklogs count -->
                        <div style="display:flex; flex-direction:column; gap:2px;">
                          <span class="text-muted">Worklogs count:</span>
                          <span style="font-weight:600; display:inline-flex; align-items:center; gap:4px;">
                            {log.new_worklogs_count}
                            {#if worklogsDiff > 0}
                              <span style="color:var(--c-success); font-weight:700;">(+{worklogsDiff})</span>
                            {:else if worklogsDiff < 0}
                              <span style="color:var(--c-danger); font-weight:700;">({worklogsDiff})</span>
                            {/if}
                          </span>
                        </div>

                      </div>
                    </div>
                  {/each}
                </div>
              {/if}
            </div>
          </div>
        </div>
      </div>

      <!-- Issue List Table Card -->
      <div class="card">
        <div class="card-header" style="padding:16px 20px;">
          <h3 class="font-semibold text-sm" style="margin:0; display:flex; align-items:center; gap:8px;">
            <span>📋</span> Issues in Snapshot ({snapshot.data.issues?.length || 0})
          </h3>
        </div>
        
        <div class="card-body" style="padding:0;">
          {#if !snapshot.data.issues || snapshot.data.issues.length === 0}
            <p class="text-xs text-muted" style="padding:30px 0; text-align:center;">No closed issues found in this range.</p>
          {:else}
            <div style="max-height:400px; overflow-y:auto;">
              <table style="width:100%; border-collapse:collapse; text-align:left; font-size:12px;">
                <thead>
                  <tr style="border-bottom:1px solid var(--c-border); background:var(--c-bg); color:var(--c-text-3); position:sticky; top:0; z-index:1;">
                    <th style="padding:12px 16px; font-weight:600;">Key</th>
                    <th style="padding:12px 16px; font-weight:600;">Summary</th>
                    <th style="padding:12px 16px; font-weight:600;">Status</th>
                    <th style="padding:12px 16px; font-weight:600; text-align:right;">Story Points</th>
                    <th style="padding:12px 16px; font-weight:600; text-align:right;">Logged Hours</th>
                  </tr>
                </thead>
                <tbody>
                  {#each snapshot.data.issues as issue}
                    <tr style="border-bottom:1px solid var(--c-border); background:var(--c-surface);">
                      <td style="padding:12px 16px; font-weight:600; white-space:nowrap;">
                        {#if plan?.jira_url}
                          <a href="{plan.jira_url}/browse/{issue.key}" target="_blank" class="text-primary" style="text-decoration:none;">{issue.key}</a>
                        {:else}
                          {issue.key}
                        {/if}
                      </td>
                      <td style="padding:12px 16px; max-width:300px; overflow:hidden; text-overflow:ellipsis; white-space:nowrap;" title={issue.summary}>{issue.summary}</td>
                      <td style="padding:12px 16px; white-space:nowrap;">
                        <span class="badge badge-default" style="font-size:10px; padding:2px 6px;">{issue.status}</span>
                      </td>
                      <td style="padding:12px 16px; text-align:right; font-weight:500;">{issue.story_points > 0 ? issue.story_points : '—'}</td>
                      <td style="padding:12px 16px; text-align:right; font-weight:500; color:var(--c-primary);">
                        {(showAllWorklogs && issue.time_spent_hours_all !== undefined ? issue.time_spent_hours_all : issue.time_spent_hours).toFixed(1)} hrs
                      </td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>
          {/if}
        </div>
      </div>

    </div>
  {/if}

  {#if showEditModal}
    <div style="position:fixed; top:0; left:0; right:0; bottom:0; background:rgba(0,0,0,0.55); display:flex; align-items:center; justify-content:center; z-index:1000; backdrop-filter:blur(4px); transition: opacity 0.2s ease;">
      <div style="background:var(--c-surface); border:1px solid var(--c-border); border-radius:12px; width:480px; max-width:90%; padding:24px; box-shadow:var(--shadow-lg);">
        <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:20px; border-bottom:1px solid var(--c-border); padding-bottom:12px;">
          <h3 class="font-semibold text-base" style="color:var(--c-primary); margin:0;">Edit Snapshot Settings</h3>
          <button class="btn-icon" onclick={() => showEditModal = false} style="font-size:14px; width:28px; height:28px; border-radius:50%; border:none; display:flex; align-items:center; justify-content:center; background:transparent;">✕</button>
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
        </div>

        {#if editSnapshotError}
          <div class="badge badge-danger" style="margin-bottom:16px; padding:10px; border-radius:6px; font-size:12px; text-transform:none; letter-spacing:0; text-align:center; display:block; font-weight:400;">
            ⚠️ {editSnapshotError}
          </div>
        {/if}

        <div style="display:flex; justify-content:flex-end; gap:12px; border-top:1px solid var(--c-border); padding-top:16px;">
          <button class="btn btn-secondary" onclick={() => showEditModal = false} disabled={updatingSnapshot}>Cancel</button>
          <button class="btn btn-primary" onclick={saveEditedSnapshot} disabled={updatingSnapshot}>
            {updatingSnapshot ? 'Saving...' : 'Save'}
          </button>
        </div>
      </div>
    </div>
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

  .grid-layout {
    grid-template-columns: 2fr 1fr;
  }

  @media (max-width: 768px) {
    .grid-layout {
      grid-template-columns: 1fr;
    }
  }
</style>
