<script lang="ts">
  import { page } from '$app/stores';
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { api, type JiraSnapshot, type Team, type JiraUserRefreshDelta } from '$lib/api';
  import { user } from '$lib/auth';

  const teamID = $derived($page.params.id as string);
  const snapshotID = $derived($page.params.snapid as string);

  let team = $state<Team | null>(null);
  let snapshot = $state<JiraSnapshot | null>(null);
  let loading = $state(true);
  let error = $state('');

  let leaderboardSearchQuery = $state('');
  let expandedDevelopers = $state<Record<string, boolean>>({});
  let refreshing = $state(false);
  let deleting = $state(false);

  let showAllWorklogs = $state(false);
  let expandedRefreshContributors = $state<Record<string, boolean>>({});
  let selectedHistoryId = $state<string>('');

  function initials(name: string) {
    if (!name) return '?';
    const parts = name.trim().split(/\s+/);
    if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase();
    return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase();
  }

  function avatarBg(name: string) {
    const palette = [
      ['#DBEAFE', '#2563EB'],
      ['#F3E8FF', '#7C3AED'],
      ['#DCFCE7', '#16A34A'],
      ['#FFEDD5', '#EA580C'],
      ['#FCE7F3', '#DB2777'],
      ['#E0E7FF', '#4F46E5'],
      ['#FEF3C7', '#D97706'],
      ['#ECFDF5', '#0D9488'],
    ];
    let h = 0;
    for (let i = 0; i < name.length; i++) h = (h * 31 + name.charCodeAt(i)) % 997;
    return palette[h % palette.length];
  }

  function formatHistoryDateTime(utcString: string) {
    if (!utcString) return '—';
    try {
      const d = new Date(utcString);
      if (isNaN(d.getTime())) return utcString;
      const day = String(d.getDate()).padStart(2, '0');
      const mon = d.toLocaleString('en-GB', { month: 'short' });
      const year = d.getFullYear();
      const hh = String(d.getHours()).padStart(2, '0');
      const mm = String(d.getMinutes()).padStart(2, '0');
      return `${day} ${mon} ${year}, ${hh}:${mm}`;
    } catch {
      return utcString;
    }
  }

  function formatShortDay(utcString: string) {
    if (!utcString) return '—';
    try {
      const d = new Date(utcString);
      if (isNaN(d.getTime())) return utcString;
      const day = d.getDate();
      const mon = d.toLocaleString('en-GB', { month: 'short' });
      return `${day} ${mon}`;
    } catch {
      return utcString;
    }
  }

  function formatSprintRange(start: string, end: string) {
    try {
      const fmt = (s: string) => {
        const d = new Date(s);
        if (isNaN(d.getTime())) return s;
        return `${String(d.getDate()).padStart(2, '0')} ${d.toLocaleString('en-GB', { month: 'short' })} ${d.getFullYear()}`;
      };
      return `${fmt(start)} – ${fmt(end)}`;
    } catch {
      return `${start} – ${end}`;
    }
  }

  async function copyJql(text: string) {
    try {
      await navigator.clipboard.writeText(text);
    } catch { /* noop */ }
  }

  const historyDesc = $derived((snapshot?.refresh_history || []).slice());
  const historyAsc = $derived(
    (snapshot?.refresh_history || []).slice().sort(
      (a, b) => new Date(a.refreshed_at).getTime() - new Date(b.refreshed_at).getTime()
    )
  );

  const selectedHistory = $derived.by(() => {
    const list = snapshot?.refresh_history || [];
    if (list.length === 0) return null;
    if (selectedHistoryId) {
      const found = list.find((r) => r.id === selectedHistoryId);
      if (found) return found;
    }
    return list[0];
  });

  const selectedHistoryPrev = $derived.by(() => {
    const list = historyAsc;
    if (!selectedHistory || list.length === 0) return null;
    const idx = list.findIndex((r) => r.id === selectedHistory.id);
    if (idx > 0) return list[idx - 1];
    return null;
  });

  const selectedDeltas = $derived.by(() => {
    if (!selectedHistory) return [];
    const hasAll = !!snapshot?.data?.totals_all && !!snapshot?.data?.leaderboard_all;
    const activeShowAll = showAllWorklogs && hasAll;
    const base =
      activeShowAll && selectedHistory.user_deltas_all
        ? selectedHistory.user_deltas_all
        : selectedHistory.user_deltas || [];
    return [...base]
      .filter((d) => d.hours_delta !== 0)
      .sort((a, b) => b.hours_delta - a.hours_delta);
  });

  const historyChart = $derived.by(() => {
    const L = 36, R = 36, T = 12, B = 28, W = 720, H = 235;
    const PW = W - L - R;
    const PH = H - T - B;
    const raw = historyAsc.map((log) => ({
      id: log.id,
      label: formatShortDay(log.refreshed_at),
      full: formatHistoryDateTime(log.refreshed_at),
      total: log.new_hours_logged || 0,
      delta: (log.new_hours_logged || 0) - (log.prev_hours_logged || 0),
    }));
    const maxTotal = Math.max(10, ...raw.map((p) => p.total)) * 1.15;
    const maxDelta = Math.max(2, ...raw.map((p) => Math.abs(p.delta))) * 1.35;
    const n = raw.length;
    const xPos = (i: number) => (n === 1 ? L + PW / 2 : L + (i * PW) / Math.max(1, n - 1));
    const yTot = (v: number) => T + PH - (Math.max(0, v) / maxTotal) * PH;
    const pts = raw.map((p, i) => {
      const x = xPos(i);
      const y = yTot(p.total);
      const bh = (Math.max(0, p.delta) / maxDelta) * PH;
      const by = T + PH - bh;
      return { ...p, x, y, barH: bh, barY: by };
    });
    const linePoints = pts.map((p) => `${p.x},${p.y}`).join(' ');
    const grid = [0, 1, 2, 3, 4, 5].map((g) => {
      const gy = T + (PH * g) / 5;
      return {
        y: gy,
        left: String(Math.round(maxTotal * (1 - g / 5))),
        right: (maxDelta * (1 - g / 5)).toFixed(0),
      };
    });
    return { pts, maxTotal, maxDelta, linePoints, grid, L, R, T, B, W, H, PH };
  });

  // Contributor Refresh Breakdown Modal State
  let showRefreshBreakdownModal = $state(false);
  let selectedRefreshLogId = $state<string>('');
  let modalSearchQuery = $state('');
  let modalOnlyChanges = $state(false);
  let modalSortBy = $state<'delta' | 'hours' | 'name'>('delta');

  async function loadData() {
    loading = true;
    error = '';
    try {
      const [p, s] = await Promise.all([
        api.teams.get(teamID),
        api.jira.getSnapshot(teamID, snapshotID)
      ]);
      team = p;
      snapshot = s;
      if (s) {
        showAllWorklogs = s.all_worklogs;
        editSnapshotName = s.name;
        editSnapshotStart = s.start_date;
        editSnapshotEnd = s.end_date;
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
  let activityFilter = $state<'all' | 'planned' | 'unplanned'>('all');
  let togglingIssueKey = $state<string>('');

  let activeTab = $state<'overview' | 'issues' | 'history' | 'settings'>('overview');
  let issueSearchQuery = $state('');
  let issueClassificationFilter = $state<'all' | 'planned' | 'unplanned'>('all');

  // Lookup map for issues by key
  const issueMap = $derived.by(() => {
    const map: Record<string, any> = {};
    if (snapshot?.data?.issues) {
      for (const issue of snapshot.data.issues) {
        map[issue.key] = issue;
      }
    }
    return map;
  });

  const filteredIssues = $derived.by(() => {
    const issues = snapshot?.data?.issues || [];
    return issues.filter((issue) => {
      if (issueClassificationFilter === 'planned' && issue.is_unplanned) return false;
      if (issueClassificationFilter === 'unplanned' && !issue.is_unplanned) return false;
      if (issueSearchQuery.trim()) {
        const q = issueSearchQuery.trim().toLowerCase();
        const hay = `${issue.key} ${issue.summary || ''} ${issue.status || ''}`.toLowerCase();
        if (!hay.includes(q)) return false;
      }
      return true;
    });
  });

  // Planned vs Unplanned Summary Metrics
  const plannedStats = $derived.by(() => {
    if (!snapshot?.data?.issues) {
      return {
        plannedHours: 0,
        unplannedHours: 0,
        plannedSP: 0,
        unplannedSP: 0,
        plannedPct: 100,
        unplannedPct: 0,
        plannedCount: 0,
        unplannedCount: 0,
        totalHours: 0,
      };
    }

    const issues = snapshot.data.issues;
    let plannedHours = 0;
    let unplannedHours = 0;
    let plannedSP = 0;
    let unplannedSP = 0;
    let plannedCount = 0;
    let unplannedCount = 0;

    for (const issue of issues) {
      const isUnplanned = !!issue.is_unplanned;
      const hours = showAllWorklogs && issue.time_spent_hours_all !== undefined 
        ? (issue.time_spent_hours_all || 0) 
        : (issue.time_spent_hours || 0);
      
      const isDone = (issue.status_category_key === 'done' || issue.status?.toLowerCase() === 'done');
      const sp = isDone ? (issue.story_points || 0) : 0;

      if (isUnplanned) {
        unplannedHours += hours;
        unplannedSP += sp;
        unplannedCount++;
      } else {
        plannedHours += hours;
        plannedSP += sp;
        plannedCount++;
      }
    }

    const totalHours = plannedHours + unplannedHours;
    const plannedPct = totalHours > 0 ? (plannedHours / totalHours) * 100 : 100;
    const unplannedPct = totalHours > 0 ? (unplannedHours / totalHours) * 100 : 0;

    return {
      plannedHours: Math.round(plannedHours * 100) / 100,
      unplannedHours: Math.round(unplannedHours * 100) / 100,
      plannedSP: Math.round(plannedSP * 10) / 10,
      unplannedSP: Math.round(unplannedSP * 10) / 10,
      plannedPct: Math.round(plannedPct),
      unplannedPct: Math.round(unplannedPct),
      plannedCount,
      unplannedCount,
      totalHours: Math.round(totalHours * 100) / 100,
    };
  });

  async function toggleIssuePlanned(issueKey: string, currentUnplanned: boolean) {
    if (!snapshot) return;
    const targetVal = !currentUnplanned;

    // Optimistic local update
    if (snapshot.data?.issues) {
      snapshot.data.issues = snapshot.data.issues.map(iss => 
        iss.key === issueKey ? { ...iss, is_unplanned: targetVal } : iss
      );
    }
    togglingIssueKey = issueKey;
    try {
      const updated = await api.jira.toggleIssuePlanned(teamID, snapshotID, issueKey, targetVal);
      snapshot = updated;
    } catch (e: any) {
      alert(`Failed to update issue: ${e.message}`);
      if (snapshot.data?.issues) {
        snapshot.data.issues = snapshot.data.issues.map(iss => 
          iss.key === issueKey ? { ...iss, is_unplanned: currentUnplanned } : iss
        );
      }
    } finally {
      togglingIssueKey = '';
    }
  }

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

    // Create lookup map for issue category/status & unplanned flag
    const doneStatusMap: Record<string, boolean> = {};
    const unplannedStatusMap: Record<string, boolean> = {};
    for (const issue of issues) {
      const isDone = (issue.status_category_key === 'done' || 
                      issue.status?.toLowerCase() === 'done');
      doneStatusMap[issue.key] = isDone;
      unplannedStatusMap[issue.key] = !!issue.is_unplanned;
    }

    let filteredSP = 0;
    for (const issue of issues) {
      const isDone = (issue.status_category_key === 'done' || issue.status?.toLowerCase() === 'done');
      if (!isDone) continue;
      const isUnplanned = !!issue.is_unplanned;
      if (activityFilter === 'planned' && isUnplanned) continue;
      if (activityFilter === 'unplanned' && !isUnplanned) continue;
      filteredSP += (issue.story_points || 0);
    }

    // Filtered leaderboard
    let totalHoursLogged = 0;
    let totalWorkLogs = 0;
    const newLeaderboard = baseLeaderboard.map(entry => {
      const allEntryWorklogs = entry.worklogs || [];

      // Calculate developer's planned vs unplanned hours
      let devPlannedHours = 0;
      let devUnplannedHours = 0;
      for (const wl of allEntryWorklogs) {
        if (doneIssuesOnly && !doneStatusMap[wl.issue_key]) continue;
        if (unplannedStatusMap[wl.issue_key]) {
          devUnplannedHours += wl.hours_logged;
        } else {
          devPlannedHours += wl.hours_logged;
        }
      }

      // Filter worklogs by active filter
      const filteredWorklogs = allEntryWorklogs.filter(wl => {
        if (doneIssuesOnly && !doneStatusMap[wl.issue_key]) return false;
        if (activityFilter === 'planned' && unplannedStatusMap[wl.issue_key]) return false;
        if (activityFilter === 'unplanned' && !unplannedStatusMap[wl.issue_key]) return false;
        return true;
      });

      const hoursLogged = filteredWorklogs.reduce((sum, wl) => sum + wl.hours_logged, 0);
      totalHoursLogged += hoursLogged;
      totalWorkLogs += filteredWorklogs.length;

      const devTotalHours = devPlannedHours + devUnplannedHours;
      const devPlannedPct = devTotalHours > 0 ? Math.round((devPlannedHours / devTotalHours) * 100) : 100;
      const devUnplannedPct = devTotalHours > 0 ? Math.round((devUnplannedHours / devTotalHours) * 100) : 0;

      return {
        ...entry,
        hours_logged: hoursLogged,
        worklogs: filteredWorklogs,
        planned_hours: Math.round(devPlannedHours * 100) / 100,
        unplanned_hours: Math.round(devUnplannedHours * 100) / 100,
        planned_pct: devPlannedPct,
        unplanned_pct: devUnplannedPct,
      };
    }).filter(entry => entry.hours_logged > 0);

    // Re-calculate percentages
    for (const entry of newLeaderboard) {
      entry.percentage = totalHoursLogged > 0 ? (entry.hours_logged / totalHoursLogged) * 100 : 0;
    }

    // Sort leaderboard descending by hours
    newLeaderboard.sort((a, b) => b.hours_logged - a.hours_logged);

    // Re-calculate average hours per SP
    const targetSP = (activityFilter === 'all' && !doneIssuesOnly) ? baseTotals.total_story_points : filteredSP;
    const avgHoursPerSP = targetSP > 0 ? totalHoursLogged / targetSP : 0;

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

  const latestRefresh = $derived(
    snapshot?.refresh_history && snapshot.refresh_history.length > 0
      ? snapshot.refresh_history[0]
      : null
  );

  const userDeltaMap = $derived.by(() => {
    const map: Record<string, JiraUserRefreshDelta> = {};
    if (!latestRefresh) return map;
    const hasAll = !!snapshot?.data?.totals_all && !!snapshot?.data?.leaderboard_all;
    const activeShowAll = showAllWorklogs && hasAll;
    const deltas = activeShowAll && latestRefresh.user_deltas_all
      ? latestRefresh.user_deltas_all
      : (latestRefresh.user_deltas || []);
    for (const d of deltas) {
      map[d.author_name] = d;
    }
    return map;
  });

  function openRefreshBreakdown(logId?: string) {
    if (logId) {
      selectedRefreshLogId = logId;
    } else if (snapshot?.refresh_history && snapshot.refresh_history.length > 0) {
      selectedRefreshLogId = snapshot.refresh_history[0].id;
    }
    modalSearchQuery = '';
    modalOnlyChanges = false;
    modalSortBy = 'delta';
    showRefreshBreakdownModal = true;
  }

  const selectedRefreshLog = $derived(
    (snapshot?.refresh_history || []).find(r => r.id === selectedRefreshLogId) || 
    (snapshot?.refresh_history?.[0] ?? null)
  );

  const selectedRefreshDeltas = $derived.by(() => {
    if (!selectedRefreshLog) return [];
    const hasAll = !!snapshot?.data?.totals_all && !!snapshot?.data?.leaderboard_all;
    const activeShowAll = showAllWorklogs && hasAll;
    const base = activeShowAll && selectedRefreshLog.user_deltas_all
      ? selectedRefreshLog.user_deltas_all
      : (selectedRefreshLog.user_deltas || []);
    
    let filtered = base.filter(d => {
      if (modalOnlyChanges && d.hours_delta === 0) return false;
      if (modalSearchQuery.trim()) {
        return d.author_name.toLowerCase().includes(modalSearchQuery.trim().toLowerCase());
      }
      return true;
    });

    return [...filtered].sort((a, b) => {
      if (modalSortBy === 'delta') {
        if (a.hours_delta !== b.hours_delta) return b.hours_delta - a.hours_delta;
        return b.new_hours_logged - a.new_hours_logged;
      } else if (modalSortBy === 'hours') {
        return b.new_hours_logged - a.new_hours_logged;
      } else {
        return a.author_name.localeCompare(b.author_name);
      }
    });
  });

  // Snapshot settings form state (Settings tab)
  let editSnapshotName = $state('');
  let editSnapshotStart = $state('');
  let editSnapshotEnd = $state('');
  let updatingSnapshot = $state(false);
  let editSnapshotError = $state('');
  let snapshotSaved = $state(false);

  function resetSnapshotForm() {
    if (!snapshot) return;
    editSnapshotName = snapshot.name;
    editSnapshotStart = snapshot.start_date;
    editSnapshotEnd = snapshot.end_date;
    editSnapshotError = '';
    snapshotSaved = false;
  }

  async function saveEditedSnapshot() {
    if (!snapshot) return;
    if (!editSnapshotName.trim() || !editSnapshotStart || !editSnapshotEnd) {
      editSnapshotError = 'Name, start date, and end date are required';
      return;
    }
    updatingSnapshot = true;
    editSnapshotError = '';
    snapshotSaved = false;
    try {
      const updated = await api.jira.updateSnapshot(teamID, snapshot.id, {
        name: editSnapshotName.trim(),
        start_date: editSnapshotStart,
        end_date: editSnapshotEnd,
      });
      snapshot = updated;
      snapshotSaved = true;
    } catch (e: any) {
      editSnapshotError = e.message;
    } finally {
      updatingSnapshot = false;
    }
  }

  let refreshingAll = $state(false);

  async function refreshSnapshot() {
    if (!confirm('Re-fetch snapshot data from Jira?')) return;
    refreshing = true;
    try {
      const updated = await api.jira.refreshSnapshot(teamID, snapshotID);
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

  async function refreshAllSnapshots() {
    if (!confirm('Refresh snapshots across all projects now?')) return;
    refreshingAll = true;
    try {
      const res = await api.admin.refreshAllSnapshots();
      const updated = await api.jira.getSnapshot(teamID, snapshotID);
      snapshot = updated;
      alert(`All projects snapshot refresh complete: ${res.refreshed} refreshed, ${res.skipped} skipped, ${res.failed} failed out of ${res.total} snapshots.`);
    } catch (e: any) {
      alert(`Failed to refresh all snapshots: ${e.message}`);
    } finally {
      refreshingAll = false;
    }
  }

  async function deleteSnapshot() {
    if (!confirm('Are you sure you want to delete this snapshot?')) return;
    deleting = true;
    try {
      await api.jira.deleteSnapshot(teamID, snapshotID);
      goto(`/teams/${teamID}`);
    } catch (e: any) {
      alert(`Failed to delete: ${e.message}`);
      deleting = false;
    }
  }

  async function createRetroFromSnapshot() {
    if (!snapshot) return;
    try {
      const title = snapshot.name.toLowerCase().endsWith('retro') ? snapshot.name : `${snapshot.name} Retro`;
      const pres = await api.presentations.create(teamID, {
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

      await api.presentations.update(teamID, pres.id, {
        title: title,
        template_id: 'default',
        sprint_name: snapshot.name,
        content: content,
      });

      goto(`/teams/${teamID}/presentations/${pres.id}/edit`);
    } catch (e: any) {
      alert(`Failed to create presentation: ${e.message}`);
    }
  }

  function exportToCSV() {
    if (!snapshot || !computedData || !computedData.leaderboard) return;

    // Create lookup map for issue details to enrich worklogs
    const issueMap: Record<string, { story_points: number; status: string; status_changed: string; is_unplanned: boolean }> = {};
    if (snapshot.data.issues) {
      for (const issue of snapshot.data.issues) {
        issueMap[issue.key] = {
          story_points: issue.story_points,
          status: issue.status,
          status_changed: issue.status_category_changed_date,
          is_unplanned: !!issue.is_unplanned,
        };
      }
    }

    // CSV Headers
    const headers = [
      'Developer',
      'Issue Key',
      'Issue Summary',
      'Classification',
      'Story Points',
      'Issue Status',
      'Status Changed Date',
      'Logged Hours'
    ];

    const rows = [headers];

    for (const entry of computedData.leaderboard) {
      if (!entry.worklogs || entry.worklogs.length === 0) continue;
      for (const wl of entry.worklogs) {
        const issueDetails = issueMap[wl.issue_key] || { story_points: 0, status: 'Unknown', status_changed: '—', is_unplanned: false };
        
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
          issueDetails.is_unplanned ? 'Unplanned' : 'Planned',
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
      <a href="/teams/{teamID}" class="btn btn-secondary">← Back to Workspace</a>
    </div>
  {:else if snapshot}
    <!-- Back Navigation Link -->
    <div style="margin-bottom: 10px;">
      <a href="/teams/{teamID}" style="display:inline-flex; align-items:center; gap:6px; font-size:13px; color:var(--c-text-2); font-weight:500;">
        <span style="font-size:15px;">←</span> Back to Sprints
      </a>
    </div>

    <!-- Header Actions Panel -->
    <div class="flex justify-between items-center" style="margin-bottom: 18px; gap: 16px; flex-wrap: wrap;">
      <div>
        <h1 style="font-size:28px; font-weight:800; letter-spacing:-0.02em; margin:0; color:#11142b;">{snapshot.name}</h1>
        <p style="margin-top:4px; display:flex; align-items:center; gap:8px; font-size:13px; color:var(--c-text-3);">
          {formatSprintRange(snapshot.start_date, snapshot.end_date)}
          {#if showAllWorklogs && snapshot.data.totals_all}
            <span style="font-size:11px; padding:2px 8px; border-radius:999px; background:var(--c-warning-lt); color:#b45309; font-weight:500;">all worklogs</span>
          {:else}
            <span style="font-size:11px; padding:2px 8px; border-radius:999px; background:var(--c-surface-2); color:var(--c-text-2); font-weight:500;">filtered worklogs</span>
          {/if}
        </p>
      </div>
      <div class="flex gap-2" style="flex-wrap:wrap;">
        <button onclick={exportToCSV} style="display:inline-flex; align-items:center; gap:6px; background:#fff; border:1px solid var(--c-border-2); border-radius:8px; padding:8px 14px; font-size:13px; font-weight:500; color:var(--c-text);">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>
          Export to CSV
        </button>
        {#if team?.is_admin}
          <button onclick={refreshSnapshot} disabled={refreshing || refreshingAll} style="display:inline-flex; align-items:center; gap:6px; background:#fff; border:1px solid var(--c-border-2); border-radius:8px; padding:8px 14px; font-size:13px; font-weight:500; color:var(--c-text);">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg>
            {refreshing ? 'Refreshing...' : 'Refresh'}
          </button>
          {#if $user?.role === 'admin'}
            <button onclick={refreshAllSnapshots} disabled={refreshing || refreshingAll} style="display:inline-flex; align-items:center; gap:6px; background:#fff; border:1px solid var(--c-primary); border-radius:8px; padding:8px 14px; font-size:13px; font-weight:500; color:var(--c-primary);" title="Refresh snapshots across all projects">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg>
              {refreshingAll ? 'Refreshing All…' : 'Refresh All Projects'}
            </button>
          {/if}
          <button onclick={deleteSnapshot} disabled={deleting} style="display:inline-flex; align-items:center; gap:6px; background:#fff; border:1px solid #fca5a5; border-radius:8px; padding:8px 18px; font-size:13px; font-weight:500; color:#dc2626;">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>
            {deleting ? 'Deleting...' : 'Delete'}
          </button>
        {/if}
      </div>
    </div>

    <!-- Snapshot Tabs -->
    <div class="flex gap-6" style="border-bottom:1px solid var(--c-border); margin-bottom:20px; padding-bottom:0;">
      <button
        class="tab-btn {activeTab === 'overview' ? 'active' : ''}"
        onclick={() => activeTab = 'overview'}
      >
        <span style="display:inline-flex; align-items:center; gap:6px;"><span style="font-size:13px;">📊</span> Overview</span>
      </button>
      <button
        class="tab-btn {activeTab === 'issues' ? 'active' : ''}"
        onclick={() => activeTab = 'issues'}
      >
        <span style="display:inline-flex; align-items:center; gap:6px;"><span style="font-size:13px;">📋</span> Issues ({snapshot.data.issues?.length || 0})</span>
      </button>
      <button
        class="tab-btn {activeTab === 'history' ? 'active' : ''}"
        onclick={() => { activeTab = 'history'; if (!selectedHistoryId && snapshot?.refresh_history?.[0]) selectedHistoryId = snapshot.refresh_history[0].id; }}
      >
        <span style="display:inline-flex; align-items:center; gap:6px;"><span style="font-size:13px;">🔄</span> Snapshot History ({snapshot.refresh_history?.length || 0})</span>
      </button>
      <button
        class="tab-btn {activeTab === 'settings' ? 'active' : ''}"
        onclick={() => { activeTab = 'settings'; resetSnapshotForm(); }}
      >
        <span style="display:inline-flex; align-items:center; gap:6px;"><span style="font-size:13px;">⚙️</span> Settings</span>
      </button>
    </div>

    <!-- Tab Content -->
    <div style="display:flex; flex-direction:column; gap:24px;">
      {#if activeTab === 'overview'}
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

      
<!-- Planned vs. Unplanned Work Allocation Banner -->
      <div style="background:var(--c-surface); padding:20px; border-radius:10px; border:1px solid var(--c-border); box-shadow:var(--shadow-sm); display:flex; flex-direction:column; gap:14px;">
        <div style="display:flex; justify-content:space-between; align-items:center; flex-wrap:wrap; gap:12px;">
          <div>
            <h3 class="font-semibold text-sm" style="margin:0; display:flex; align-items:center; gap:8px;">
              <span>⚖️</span> Work Allocation (Planned vs. Unplanned)
            </h3>
            <p class="text-xs text-muted" style="margin:4px 0 0 0;">
              Total: <strong>{plannedStats.totalHours} hrs</strong> across {plannedStats.plannedCount + plannedStats.unplannedCount} issues in snapshot
            </p>
          </div>
          <div style="display:flex; align-items:center; gap:16px; font-size:12px; flex-wrap:wrap;">
            <div style="display:flex; align-items:center; gap:6px;">
              <span style="width:10px; height:10px; border-radius:50%; background:var(--c-primary); display:inline-block;"></span>
              <span><strong>Planned:</strong> {plannedStats.plannedHours} hrs ({plannedStats.plannedPct}%) • {plannedStats.plannedSP} SP</span>
            </div>
            <div style="display:flex; align-items:center; gap:6px;">
              <span style="width:10px; height:10px; border-radius:50%; background:#f59f00; display:inline-block;"></span>
              <span><strong>Unplanned:</strong> {plannedStats.unplannedHours} hrs ({plannedStats.unplannedPct}%) • {plannedStats.unplannedSP} SP</span>
            </div>
          </div>
        </div>

        <!-- Split Progress Bar -->
        <div style="width:100%; height:8px; background:var(--c-surface-2); border-radius:4px; overflow:hidden; display:flex;">
          <div 
            style="height:100%; width:{plannedStats.plannedPct}%; background:var(--c-primary); transition:width 300ms ease;" 
            title="Planned: {plannedStats.plannedHours} hrs ({plannedStats.plannedPct}%)"
          ></div>
          <div 
            style="height:100%; width:{plannedStats.unplannedPct}%; background:#f59f00; transition:width 300ms ease;" 
            title="Unplanned: {plannedStats.unplannedHours} hrs ({plannedStats.unplannedPct}%)"
          ></div>
        </div>
      </div>

      
      <!-- Logged Hours Leaderboard Accordion Card -->
        <div class="card">
          <div class="card-header" style="display:flex; flex-direction:column; gap:12px; align-items:stretch; padding:16px 20px;">
            <div class="flex justify-between items-center" style="flex-wrap:wrap; gap:12px;">
              <div>
                <h2 class="font-semibold text-base" style="display:flex; align-items:center; gap:8px; margin:0;">
                  <span>{activityFilter === 'unplanned' ? '⚡' : activityFilter === 'planned' ? '📋' : '🏆'}</span>
                  {#if activityFilter === 'unplanned'}
                    Unplanned Hours Leaderboard
                  {:else if activityFilter === 'planned'}
                    Planned Hours Leaderboard
                  {:else}
                    Logged Hours Leaderboard
                  {/if}
                </h2>
                {#if activityFilter === 'unplanned'}
                  <p class="text-xs text-muted" style="margin:2px 0 0 0; color:#f59f00;">
                    Ranked by developers who logged the most hours on unplanned tickets
                  </p>
                {/if}
              </div>

              <div class="flex items-center gap-3" style="flex-wrap: wrap;">
                <!-- Activity Filter Tabs -->
                <div style="display:inline-flex; background:var(--c-surface-2); border-radius:6px; padding:2px; border:1px solid var(--c-border-2);">
                  <button 
                    type="button"
                    style="padding:4px 10px; font-size:11px; font-weight:600; border-radius:4px; border:none; cursor:pointer; background:{activityFilter === 'all' ? 'var(--c-surface)' : 'transparent'}; color:{activityFilter === 'all' ? 'var(--c-text)' : 'var(--c-text-3)'}; box-shadow:{activityFilter === 'all' ? 'var(--shadow-sm)' : 'none'}; transition:all 150ms ease;"
                    onclick={() => activityFilter = 'all'}
                  >
                    All Work
                  </button>
                  <button 
                    type="button"
                    style="padding:4px 10px; font-size:11px; font-weight:600; border-radius:4px; border:none; cursor:pointer; background:{activityFilter === 'planned' ? 'var(--c-surface)' : 'transparent'}; color:{activityFilter === 'planned' ? 'var(--c-primary)' : 'var(--c-text-3)'}; box-shadow:{activityFilter === 'planned' ? 'var(--shadow-sm)' : 'none'}; transition:all 150ms ease;"
                    onclick={() => activityFilter = 'planned'}
                  >
                    📋 Planned Only
                  </button>
                  <button 
                    type="button"
                    style="padding:4px 10px; font-size:11px; font-weight:600; border-radius:4px; border:none; cursor:pointer; background:{activityFilter === 'unplanned' ? 'var(--c-surface)' : 'transparent'}; color:{activityFilter === 'unplanned' ? '#f59f00' : 'var(--c-text-3)'}; box-shadow:{activityFilter === 'unplanned' ? 'var(--shadow-sm)' : 'none'}; transition:all 150ms ease;"
                    onclick={() => activityFilter = 'unplanned'}
                    title="Rank contributors who spent the most time on unplanned activities"
                  >
                    ⚡ Unplanned Only
                  </button>
                </div>

                <!-- Worklog Filter Switch -->
                <label 
                  style="display:inline-flex; align-items:center; gap:6px; font-size:12px; cursor:pointer; font-weight:normal; user-select:none; margin:0;"
                  title={!snapshot.data.totals_all ? "Click Refresh to enable this toggle for older snapshots" : ""}
                >
                  <input type="checkbox" bind:checked={showAllWorklogs} disabled={!snapshot.data.totals_all} />
                  <span>Include all worklogs</span>
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
                {@const uDelta = userDeltaMap[entry.author_name]}
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
                        <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:6px; flex-wrap:wrap; gap:8px;">
                          <div style="display:flex; align-items:center; gap:8px; flex-wrap:wrap;">
                            <span class="font-bold text-sm" style="color:var(--c-text);">{entry.author_name}</span>
                            {#if entry.planned_hours !== undefined && entry.unplanned_hours !== undefined}
                              <span class="text-xs text-muted" style="font-size:11px;">
                                (Planned: <strong>{entry.planned_hours}h ({entry.planned_pct ?? 0}%)</strong> • Unplanned: <strong style="color:{entry.unplanned_hours > 0 ? '#f59f00' : 'inherit'};">{entry.unplanned_hours}h ({entry.unplanned_pct ?? 0}%)</strong>)
                              </span>
                            {/if}
                            {#if uDelta}
                              {#if uDelta.hours_delta > 0}
                                <span 
                                  class="badge badge-success" 
                                  style="font-size:10px; padding:2px 6px; font-weight:700; display:inline-flex; align-items:center; gap:2px; border-radius:4px;"
                                  title="Added in last refresh: +{uDelta.hours_delta.toFixed(1)} hrs ({uDelta.prev_hours_logged.toFixed(1)} → {uDelta.new_hours_logged.toFixed(1)} hrs)"
                                >
                                  +{uDelta.hours_delta.toFixed(1)} hrs
                                </span>
                              {:else if uDelta.hours_delta < 0}
                                <span 
                                  class="badge badge-danger" 
                                  style="font-size:10px; padding:2px 6px; font-weight:700; display:inline-flex; align-items:center; gap:2px; border-radius:4px;"
                                  title="Reduced in last refresh: {uDelta.hours_delta.toFixed(1)} hrs ({uDelta.prev_hours_logged.toFixed(1)} → {uDelta.new_hours_logged.toFixed(1)} hrs)"
                                >
                                  {uDelta.hours_delta.toFixed(1)} hrs
                                </span>
                              {/if}
                            {/if}
                          </div>
                          <span class="font-semibold text-sm" style="color:{activityFilter === 'unplanned' ? '#f59f00' : 'var(--c-primary)'};">
                            {entry.hours_logged.toFixed(1)} hrs <span class="text-xs text-muted font-normal">({entry.percentage.toFixed(0)}%)</span>
                          </span>
                        </div>
                        <div style="width:100%; height:6px; background:var(--c-surface-2); border-radius:3px; overflow:hidden;">
                          <div style="height:100%; width:{entry.percentage}%; background:{activityFilter === 'unplanned' ? '#f59f00' : 'var(--c-primary)'}; border-radius:3px;"></div>
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
                      {#if uDelta && uDelta.hours_delta !== 0}
                        <div style="margin-bottom:12px; padding:8px 12px; border-radius:6px; background:var(--c-surface-2); border:1px solid var(--c-border); font-size:11px; display:flex; justify-content:space-between; align-items:center;">
                          <span class="text-muted" style="display:inline-flex; align-items:center; gap:6px;">
                            <span>🔄</span> <strong>Last Refresh:</strong> {uDelta.prev_hours_logged.toFixed(1)} hrs → {uDelta.new_hours_logged.toFixed(1)} hrs
                          </span>
                          <span style="font-weight:700; color:{uDelta.hours_delta > 0 ? 'var(--c-success)' : 'var(--c-danger)'};">
                            {uDelta.hours_delta > 0 ? '+' : ''}{uDelta.hours_delta.toFixed(1)} hrs
                          </span>
                        </div>
                      {/if}
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
                                <th style="padding:6px 8px; font-weight:600; width:100px;">Classification</th>
                                <th style="padding:6px 8px; font-weight:600; text-align:right; width:100px;">Logged Hours</th>
                              </tr>
                            </thead>
                            <tbody>
                              {#each entry.worklogs as wl}
                                {@const isWlUnplanned = !!issueMap[wl.issue_key]?.is_unplanned}
                                <tr style="border-bottom:1px solid var(--c-border-2);">
                                  <td style="padding:8px; font-weight:600; white-space:nowrap;">
                                    {#if team?.jira_url}
                                      <a href="{team.jira_url}/browse/{wl.issue_key}" target="_blank" class="text-primary" style="text-decoration:none;">{wl.issue_key}</a>
                                    {:else}
                                      {wl.issue_key}
                                    {/if}
                                  </td>
                                  <td style="padding:8px; max-width:320px; overflow:hidden; text-overflow:ellipsis; white-space:nowrap;" title={wl.issue_summary}>
                                    {wl.issue_summary || '—'}
                                  </td>
                                  <td style="padding:8px; white-space:nowrap;">
                                    {#if isWlUnplanned}
                                      <span class="badge badge-warning" style="font-size:10px; padding:2px 6px;">⚡ Unplanned</span>
                                    {:else}
                                      <span class="badge badge-default" style="font-size:10px; padding:2px 6px;">📋 Planned</span>
                                    {/if}
                                  </td>
                                  <td style="padding:8px; text-align:right; font-weight:600; color:{isWlUnplanned ? '#f59f00' : 'var(--c-primary)'};">
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

          
      {:else if activeTab === 'issues'}
<!-- Issue List Table Card -->
      <div class="card">
        <div class="card-header" style="padding:16px 20px; display:flex; flex-direction:column; gap:12px; align-items:stretch;">
          <div class="flex justify-between items-center" style="flex-wrap:wrap; gap:12px;">
            <h3 class="font-semibold text-sm" style="margin:0; display:flex; align-items:center; gap:8px;">
              <span>📋</span> Issues in Snapshot ({snapshot.data.issues?.length || 0})
            </h3>
            <div class="flex items-center gap-2" style="flex-wrap:wrap;">
              <div style="display:inline-flex; background:var(--c-surface-2); border-radius:6px; padding:2px; border:1px solid var(--c-border-2);">
                <button
                  type="button"
                  style="padding:4px 10px; font-size:11px; font-weight:600; border-radius:4px; border:none; cursor:pointer; background:{issueClassificationFilter === 'all' ? 'var(--c-surface)' : 'transparent'}; color:{issueClassificationFilter === 'all' ? 'var(--c-text)' : 'var(--c-text-3)'}; box-shadow:{issueClassificationFilter === 'all' ? 'var(--shadow-sm)' : 'none'};"
                  onclick={() => issueClassificationFilter = 'all'}
                >
                  All ({snapshot.data.issues?.length || 0})
                </button>
                <button
                  type="button"
                  style="padding:4px 10px; font-size:11px; font-weight:600; border-radius:4px; border:none; cursor:pointer; background:{issueClassificationFilter === 'planned' ? 'var(--c-surface)' : 'transparent'}; color:{issueClassificationFilter === 'planned' ? 'var(--c-primary)' : 'var(--c-text-3)'}; box-shadow:{issueClassificationFilter === 'planned' ? 'var(--shadow-sm)' : 'none'};"
                  onclick={() => issueClassificationFilter = 'planned'}
                >
                  📋 Planned
                </button>
                <button
                  type="button"
                  style="padding:4px 10px; font-size:11px; font-weight:600; border-radius:4px; border:none; cursor:pointer; background:{issueClassificationFilter === 'unplanned' ? 'var(--c-surface)' : 'transparent'}; color:{issueClassificationFilter === 'unplanned' ? '#f59f00' : 'var(--c-text-3)'}; box-shadow:{issueClassificationFilter === 'unplanned' ? 'var(--shadow-sm)' : 'none'};"
                  onclick={() => issueClassificationFilter = 'unplanned'}
                >
                  ⚡ Unplanned
                </button>
              </div>
              <span class="badge badge-primary">{filteredIssues.length} shown</span>
            </div>
          </div>
          <div style="position:relative; width:100%;">
            <span style="position:absolute; left:12px; top:50%; transform:translateY(-50%); font-size:14px; color:var(--c-text-3);">🔍</span>
            <input
              type="text"
              class="input"
              placeholder="Search by key, summary, or status..."
              bind:value={issueSearchQuery}
              style="padding-left:36px; background:var(--c-bg);"
            />
            {#if issueSearchQuery}
              <button
                onclick={() => issueSearchQuery = ''}
                style="position:absolute; right:12px; top:50%; transform:translateY(-50%); background:none; border:none; font-size:12px; color:var(--c-text-3); cursor:pointer;"
              >
                ✕
              </button>
            {/if}
          </div>
        </div>
        
        <div class="card-body" style="padding:0;">
          {#if !snapshot.data.issues || snapshot.data.issues.length === 0}
            <p class="text-xs text-muted" style="padding:30px 0; text-align:center;">No closed issues found in this range.</p>
          {:else if filteredIssues.length === 0}
            <p class="text-xs text-muted" style="padding:30px 0; text-align:center;">No issues match your search / filter.</p>
          {:else}
            <div style="max-height:400px; overflow-y:auto;">
              <table style="width:100%; border-collapse:collapse; text-align:left; font-size:12px;">
                <thead>
                  <tr style="border-bottom:1px solid var(--c-border); background:var(--c-bg); color:var(--c-text-3); position:sticky; top:0; z-index:1;">
                    <th style="padding:12px 16px; font-weight:600;">Key</th>
                    <th style="padding:12px 16px; font-weight:600;">Summary</th>
                    <th style="padding:12px 16px; font-weight:600; width:130px;">Classification</th>
                    <th style="padding:12px 16px; font-weight:600;">Status</th>
                    <th style="padding:12px 16px; font-weight:600; text-align:right;">Story Points</th>
                    <th style="padding:12px 16px; font-weight:600; text-align:right;">Logged Hours</th>
                  </tr>
                </thead>
                <tbody>
                  {#each filteredIssues as issue}
                    <tr style="border-bottom:1px solid var(--c-border); background:var(--c-surface);">
                      <td style="padding:12px 16px; font-weight:600; white-space:nowrap;">
                        {#if team?.jira_url}
                          <a href="{team.jira_url}/browse/{issue.key}" target="_blank" class="text-primary" style="text-decoration:none;">{issue.key}</a>
                        {:else}
                          {issue.key}
                        {/if}
                      </td>
                      <td style="padding:12px 16px; max-width:300px; overflow:hidden; text-overflow:ellipsis; white-space:nowrap;" title={issue.summary}>{issue.summary}</td>
                      <td style="padding:12px 16px; white-space:nowrap;">
                        {#if team?.is_admin}
                          <button 
                            type="button"
                            class="badge {issue.is_unplanned ? 'badge-warning' : 'badge-default'}" 
                            style="cursor:pointer; border:1px solid var(--c-border); font-size:11px; padding:3px 8px; font-weight:600; display:inline-flex; align-items:center; gap:4px; transition:all 150ms ease; background:{issue.is_unplanned ? 'rgba(245, 159, 0, 0.15)' : 'var(--c-surface-2)'}; color:{issue.is_unplanned ? '#f59f00' : 'var(--c-text)'};"
                            onclick={() => toggleIssuePlanned(issue.key, !!issue.is_unplanned)}
                            disabled={togglingIssueKey === issue.key}
                            title="Click to toggle between Planned and Unplanned"
                          >
                            {#if togglingIssueKey === issue.key}
                              <span class="spinner" style="width:10px; height:10px;"></span>
                            {:else if issue.is_unplanned}
                              <span>⚡ Unplanned</span>
                            {:else}
                              <span>📋 Planned</span>
                            {/if}
                          </button>
                        {:else}
                          <span 
                            class="badge {issue.is_unplanned ? 'badge-warning' : 'badge-default'}" 
                            style="font-size:11px; padding:3px 8px; background:{issue.is_unplanned ? 'rgba(245, 159, 0, 0.15)' : 'var(--c-surface-2)'}; color:{issue.is_unplanned ? '#f59f00' : 'var(--c-text)'};"
                          >
                            {issue.is_unplanned ? '⚡ Unplanned' : '📋 Planned'}
                          </span>
                        {/if}
                      </td>
                      <td style="padding:12px 16px; white-space:nowrap;">
                        <span class="badge badge-default" style="font-size:10px; padding:2px 6px;">{issue.status}</span>
                      </td>
                      <td style="padding:12px 16px; text-align:right; font-weight:500;">{issue.story_points > 0 ? issue.story_points : '—'}</td>
                      <td style="padding:12px 16px; text-align:right; font-weight:500; color:{issue.is_unplanned ? '#f59f00' : 'var(--c-primary)'};">
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
      {:else if activeTab === 'history'}
        {@const latestH = historyDesc[0] ?? null}
        {@const latestSpDiff = latestH ? (latestH.new_story_points - latestH.prev_story_points) : 0}
        {@const latestHoursDiff = latestH ? (latestH.new_hours_logged - latestH.prev_hours_logged) : 0}
        {@const latestIssuesDiff = latestH ? (latestH.new_issues_count - latestH.prev_issues_count) : 0}
        {@const latestWlDiff = latestH ? (latestH.new_worklogs_count - latestH.prev_worklogs_count) : 0}
        {@const chartPts = historyChart.pts}
        {@const chartMaxTotal = historyChart.maxTotal}
        {@const chartMaxDelta = historyChart.maxDelta}
        {@const selSpDiff = selectedHistory ? (selectedHistory.new_story_points - selectedHistory.prev_story_points) : 0}
        {@const selHoursDiff = selectedHistory ? (selectedHistory.new_hours_logged - selectedHistory.prev_hours_logged) : 0}
        {@const selIssuesDiff = selectedHistory ? (selectedHistory.new_issues_count - selectedHistory.prev_issues_count) : 0}
        {@const selWlDiff = selectedHistory ? (selectedHistory.new_worklogs_count - selectedHistory.prev_worklogs_count) : 0}
        {@const selMaxDelta = Math.max(0.1, ...selectedDeltas.map(d => Math.abs(d.hours_delta)))}
        <div class="history-layout">
          <!-- LEFT COLUMN -->
          <div style="display:flex; flex-direction:column; gap:16px; min-width:0;">
            <!-- Chart + summary card -->
            <div style="background:#fff; border:1px solid var(--c-border); border-radius:12px; padding:18px 20px 16px 20px; box-shadow:var(--shadow-sm);">
              <div style="display:flex; justify-content:space-between; align-items:flex-start; gap:12px; flex-wrap:wrap;">
                <div>
                  <h3 style="margin:0; font-size:15px; font-weight:700; color:#11142b;">Hours logged per refresh</h3>
                  <p style="margin:3px 0 0 0; font-size:12px; color:var(--c-text-3);">Total hours logged (cumulative) and hours added in each refresh (delta).</p>
                </div>
                <div style="display:flex; align-items:center; gap:16px; font-size:11px; color:var(--c-text-2); padding-top:4px;">
                  <span style="display:inline-flex; align-items:center; gap:6px;">
                    <svg width="28" height="8"><line x1="2" y1="4" x2="20" y2="4" stroke="#2f6bff" stroke-width="2"/><circle cx="6" cy="4" r="3" fill="#2f6bff"/><circle cx="16" cy="4" r="3" fill="#2f6bff"/></svg>
                    Total hours (cumulative)
                  </span>
                  <span style="display:inline-flex; align-items:center; gap:6px;">
                    <span style="width:14px; height:12px; border-radius:2px; background:#bfdbfe; display:inline-block;"></span>
                    Hours added (this refresh)
                  </span>
                </div>
              </div>

              {#if chartPts.length === 0}
                <p class="text-xs text-muted" style="text-align:center; padding:30px 0;">No refreshes recorded yet.</p>
              {:else}
                <div style="position:relative; margin-top:8px;">
                  <span style="position:absolute; left:0; top:0; font-size:11px; color:var(--c-text-3);">Total hours</span>
                  <span style="position:absolute; right:0; top:0; font-size:11px; color:var(--c-text-3);">Hours added</span>
                  <svg viewBox="0 0 {historyChart.W} {historyChart.H}" style="width:100%; height:auto; display:block; margin-top:18px;" role="img" aria-label="Hours logged per refresh chart">
                    <!-- gridlines -->
                    {#each historyChart.grid as g}
                      <line x1={historyChart.L} y1={g.y} x2={historyChart.W - historyChart.R} y2={g.y} stroke="#eef0f6" stroke-width="1" />
                      <text x={historyChart.L - 6} y={g.y + 4} text-anchor="end" font-size="10" fill="#9ca3af">{g.left}</text>
                      <text x={historyChart.W - historyChart.R + 6} y={g.y + 4} text-anchor="start" font-size="10" fill="#9ca3af">{g.right}</text>
                    {/each}
                    <!-- bars (deltas) -->
                    {#each chartPts as p}
                      {#if p.delta > 0.001}
                        <rect x={p.x - 22} y={p.barY} width="44" height={Math.max(2, p.barH)} rx="3" fill="#bfdbfe" opacity="0.95" />
                        <text x={p.x} y={p.barY + 16} text-anchor="middle" font-size="11" font-weight="600" fill="#2563eb">{p.delta.toFixed(1)}</text>
                      {/if}
                    {/each}
                    <!-- cumulative line -->
                    <polyline
                      points={historyChart.linePoints}
                      fill="none" stroke="#2f6bff" stroke-width="2" stroke-linejoin="round" stroke-linecap="round"
                    />
                    {#each chartPts as p}
                      <text x={p.x} y={p.y - 10} text-anchor="middle" font-size="11" font-weight="600" fill="#2f6bff">{p.total.toFixed(1)}</text>
                      <circle cx={p.x} cy={p.y} r="5" fill="#2f6bff" stroke="#fff" stroke-width="2" />
                    {/each}
                    <!-- x labels -->
                    {#each chartPts as p}
                      <text x={p.x} y={historyChart.T + historyChart.PH + 20} text-anchor="middle" font-size="11" fill="#9ca3af">{p.label}</text>
                    {/each}
                  </svg>
                </div>
              {/if}

              <!-- 4 summary tiles (latest refresh) -->
              {#if latestH}
                <div style="display:grid; grid-template-columns:repeat(4, 1fr); gap:12px; margin-top:14px;">
                  <div style="border:1px solid var(--c-border); border-radius:10px; padding:12px 14px; background:#fff;">
                    <div style="display:flex; align-items:center; gap:8px;">
                      <span style="width:32px; height:32px; border-radius:50%; background:#f1f0ff; display:inline-flex; align-items:center; justify-content:center; font-size:16px;">📦</span>
                      <span style="font-size:12px; color:var(--c-text-2);">Story points</span>
                    </div>
                    <div style="font-size:24px; font-weight:800; margin-top:6px; color:#11142b;">{latestH.new_story_points}</div>
                    <div style="font-size:11px; color:var(--c-success); font-weight:600; margin-top:2px;">↑ +{latestSpDiff.toFixed(0)} since last refresh</div>
                  </div>
                  <div style="border:1px solid var(--c-border); border-radius:10px; padding:12px 14px; background:#fff;">
                    <div style="display:flex; align-items:center; gap:8px;">
                      <span style="width:32px; height:32px; border-radius:50%; background:#e8f3ff; display:inline-flex; align-items:center; justify-content:center; font-size:16px;">🕐</span>
                      <span style="font-size:12px; color:var(--c-text-2);">Hours logged</span>
                    </div>
                    <div style="font-size:24px; font-weight:800; margin-top:6px; color:#11142b;">{latestH.new_hours_logged.toFixed(1)}h</div>
                    <div style="font-size:11px; color:var(--c-success); font-weight:600; margin-top:2px;">↑ +{latestHoursDiff.toFixed(1)}h since last refresh</div>
                  </div>
                  <div style="border:1px solid var(--c-border); border-radius:10px; padding:12px 14px; background:#fff;">
                    <div style="display:flex; align-items:center; gap:8px;">
                      <span style="width:32px; height:32px; border-radius:50%; background:#e9f9ef; display:inline-flex; align-items:center; justify-content:center; font-size:16px; color:#16a34a;">✓</span>
                      <span style="font-size:12px; color:var(--c-text-2);">Issues closed</span>
                    </div>
                    <div style="font-size:24px; font-weight:800; margin-top:6px; color:#11142b;">{latestH.new_issues_count}</div>
                    <div style="font-size:11px; color:var(--c-success); font-weight:600; margin-top:2px;">↑ +{latestIssuesDiff} since last refresh</div>
                  </div>
                  <div style="border:1px solid var(--c-border); border-radius:10px; padding:12px 14px; background:#fff;">
                    <div style="display:flex; align-items:center; gap:8px;">
                      <span style="width:32px; height:32px; border-radius:50%; background:#fff6e8; display:inline-flex; align-items:center; justify-content:center; font-size:16px;">📄</span>
                      <span style="font-size:12px; color:var(--c-text-2);">Worklogs</span>
                    </div>
                    <div style="font-size:24px; font-weight:800; margin-top:6px; color:#11142b;">{latestH.new_worklogs_count}</div>
                    <div style="font-size:11px; color:var(--c-success); font-weight:600; margin-top:2px;">↑ +{latestWlDiff} since last refresh</div>
                  </div>
                </div>
              {/if}
            </div>

            <!-- Refresh history table -->
            <div style="background:#fff; border:1px solid var(--c-border); border-radius:12px; box-shadow:var(--shadow-sm); overflow:hidden;">
              <div style="padding:16px 20px 4px 20px;">
                <h3 style="margin:0; font-size:15px; font-weight:700; color:#11142b;">Refresh history</h3>
                <p style="margin:3px 0 0 0; font-size:12px; color:var(--c-text-3);">All snapshot refreshes for this sprint</p>
              </div>
              {#if historyDesc.length === 0}
                <p class="text-xs text-muted" style="text-align:center; padding:24px 0;">No refreshes recorded yet.</p>
              {:else}
                <div style="overflow-x:auto; margin-top:8px;">
                  <table style="width:100%; border-collapse:collapse; font-size:12px; min-width:720px;">
                    <thead>
                      <tr style="color:var(--c-text-3); font-size:11px; font-weight:500; text-align:left;">
                        <th style="padding:8px 12px 8px 20px; font-weight:500;">Date & time</th>
                        <th style="padding:8px 12px; font-weight:500;">Total hours<br/>(cumulative)</th>
                        <th style="padding:8px 12px; font-weight:500;">Hours added<br/>(delta)</th>
                        <th style="padding:8px 12px; font-weight:500;">Story points</th>
                        <th style="padding:8px 12px; font-weight:500;">Issues closed</th>
                        <th style="padding:8px 12px; font-weight:500;">Worklogs</th>
                        <th style="padding:8px 12px; font-weight:500;">Contributors</th>
                        <th style="padding:8px 20px 8px 12px; font-weight:500; text-align:right;">Actions</th>
                      </tr>
                    </thead>
                    <tbody>
                      {#each historyDesc as log}
                        {@const spD = log.new_story_points - log.prev_story_points}
                        {@const hD = log.new_hours_logged - log.prev_hours_logged}
                        {@const wlD = log.new_worklogs_count - log.prev_worklogs_count}
                        {@const isD = log.new_issues_count - log.prev_issues_count}
                        {@const deltas = ((showAllWorklogs && log.user_deltas_all) ? log.user_deltas_all : log.user_deltas) || []}
                        {@const changed = deltas.filter(d => d.hours_delta !== 0)}
                        {@const avatarList = [...deltas].sort((a,b) => b.hours_delta - a.hours_delta).slice(0, 4)}
                        {@const overflow = Math.max(0, deltas.length - 4)}
                        {@const isSel = selectedHistory?.id === log.id}
                        <tr
                          onclick={() => selectedHistoryId = log.id}
                          style="border-top:1px solid var(--c-border); cursor:pointer; background:{isSel ? '#eef4ff' : '#fff'}; box-shadow:{isSel ? 'inset 3px 0 0 #2f6bff' : 'none'};"
                        >
                          <td style="padding:12px 12px 12px 20px; font-weight:700; white-space:nowrap; color:#11142b;">{formatHistoryDateTime(log.refreshed_at)}</td>
                          <td style="padding:12px; font-weight:500; white-space:nowrap;">{log.new_hours_logged.toFixed(1)}h</td>
                          <td style="padding:12px; font-weight:700; color:var(--c-success); white-space:nowrap;">+{hD.toFixed(1)}h</td>
                          <td style="padding:12px; white-space:nowrap;">{log.new_story_points} <span style="color:var(--c-success); font-weight:700; margin-left:4px;">+{spD.toFixed(0)}</span></td>
                          <td style="padding:12px; white-space:nowrap;">{log.new_issues_count} <span style="color:var(--c-success); font-weight:700; margin-left:4px;">+{isD}</span></td>
                          <td style="padding:12px; white-space:nowrap;">{log.new_worklogs_count} <span style="color:var(--c-success); font-weight:700; margin-left:4px;">+{wlD}</span></td>
                          <td style="padding:12px; white-space:nowrap;">
                            {#if deltas.length === 0}
                              <span class="text-muted" style="font-size:11px;">—</span>
                            {:else}
                              <span style="display:inline-flex; align-items:center;">
                                {#each avatarList as d, ai}
                                  {@const av = avatarBg(d.author_name)}
                                  <span title={d.author_name} style="width:24px; height:24px; border-radius:50%; background:{av[0]}; color:{av[1]}; font-size:9px; font-weight:700; display:inline-flex; align-items:center; justify-content:center; border:2px solid #fff; margin-left:{ai === 0 ? '0' : '-8px'};">{initials(d.author_name)}</span>
                                {/each}
                                {#if overflow > 0}
                                  <span style="width:24px; height:24px; border-radius:50%; background:var(--c-surface-2); color:var(--c-text-2); font-size:9px; font-weight:700; display:inline-flex; align-items:center; justify-content:center; border:2px solid #fff; margin-left:-8px;">+{overflow}</span>
                                {/if}
                              </span>
                            {/if}
                          </td>
                          <td style="padding:12px 20px 12px 12px; text-align:right; color:var(--c-text-2);">›</td>
                        </tr>
                      {/each}
                    </tbody>
                  </table>
                </div>
              {/if}
            </div>
          </div>

          <!-- RIGHT DETAIL PANEL -->
          <div style="min-width:0;">
            {#if selectedHistory}
              {@const prevLabel = selectedHistoryPrev ? formatHistoryDateTime(selectedHistoryPrev.refreshed_at) : null}
              <div style="background:#fff; border:1px solid var(--c-border); border-radius:12px; padding:18px 18px 16px 18px; box-shadow:var(--shadow-sm); position:sticky; top:72px;">
                <div style="display:flex; justify-content:space-between; align-items:flex-start; gap:8px;">
                  <div>
                    <div style="font-size:16px; font-weight:800; color:#11142b;">{formatHistoryDateTime(selectedHistory.refreshed_at)}</div>
                    <div style="font-size:12px; color:var(--c-text-3); margin-top:2px;">Compared to {prevLabel ?? formatHistoryDateTime(historyAsc[0]?.refreshed_at ?? selectedHistory.refreshed_at)}</div>
                  </div>
                  <button onclick={() => selectedHistoryId = ''} aria-label="Clear selection" style="background:none; border:none; font-size:16px; color:var(--c-text-3); cursor:pointer; padding:2px 6px;">✕</button>
                </div>

                <div style="border-top:1px solid var(--c-border); margin-top:12px; padding-top:14px;">
                  <div style="font-size:13px; font-weight:700; color:#11142b; margin-bottom:10px;">Change summary</div>
                  <div style="display:grid; grid-template-columns:repeat(4, 1fr); gap:8px;">
                    <div style="border:1px solid var(--c-border); border-radius:8px; padding:10px 8px; text-align:left;">
                      <div style="font-size:16px; color:#8b5cf6;">⬢</div>
                      <div style="font-size:15px; font-weight:800; margin-top:4px;">+{selSpDiff.toFixed(0)}</div>
                      <div style="font-size:10px; color:var(--c-text-2); line-height:1.3;">Story points</div>
                    </div>
                    <div style="border:1px solid var(--c-border); border-radius:8px; padding:10px 8px; text-align:left;">
                      <div style="font-size:16px; color:#2563eb;">🕐</div>
                      <div style="font-size:15px; font-weight:800; margin-top:4px;">+{selHoursDiff.toFixed(1)}h</div>
                      <div style="font-size:10px; color:var(--c-text-2); line-height:1.3;">Hours logged</div>
                    </div>
                    <div style="border:1px solid var(--c-border); border-radius:8px; padding:10px 8px; text-align:left;">
                      <div style="font-size:16px; color:#16a34a;">✓</div>
                      <div style="font-size:15px; font-weight:800; margin-top:4px;">+{selIssuesDiff}</div>
                      <div style="font-size:10px; color:var(--c-text-2); line-height:1.3;">Issues closed</div>
                    </div>
                    <div style="border:1px solid var(--c-border); border-radius:8px; padding:10px 8px; text-align:left;">
                      <div style="font-size:16px; color:#f59e0b;">📄</div>
                      <div style="font-size:15px; font-weight:800; margin-top:4px;">+{selWlDiff}</div>
                      <div style="font-size:10px; color:var(--c-text-2); line-height:1.3;">Worklogs</div>
                    </div>
                  </div>

                  {#if selectedDeltas.length > 0}
                    {@const top1 = selectedDeltas[0]}
                    {@const top2 = selectedDeltas[1]}
                    <div style="margin-top:12px; background:#eef4ff; border:1px solid #dbe7ff; border-radius:8px; padding:10px 12px; display:flex; gap:10px; align-items:flex-start;">
                      <span style="color:#2563eb; font-size:15px; margin-top:1px;">↗</span>
                      <div style="font-size:12px; line-height:1.5;">
                        <div style="font-weight:700; color:#2563eb;">{selectedDeltas.length} contributor{selectedDeltas.length === 1 ? '' : 's'} changed their hours</div>
                        <div style="color:var(--c-text-2);">{top1.author_name}{top2 ? ` and ${top2.author_name}` : ''} had the biggest increase.</div>
                      </div>
                    </div>
                  {/if}
                </div>

                <div style="margin-top:16px;">
                  <div style="font-size:13px; font-weight:700; color:#11142b; margin-bottom:10px;">Contributor changes</div>
                  {#if selectedDeltas.length === 0}
                    <p class="text-xs text-muted">No contributor hour changes in this refresh.</p>
                  {:else}
                    <div style="display:grid; grid-template-columns:1fr auto; font-size:11px; color:var(--c-text-3); padding-bottom:6px;">
                      <span>Contributor</span>
                      <span style="display:flex; gap:24px;"><span>Hours (previous → current)</span><span style="min-width:48px; text-align:right;">Change</span></span>
                    </div>
                    <div style="display:flex; flex-direction:column; gap:12px;">
                      {#each selectedDeltas as d}
                        {@const av2 = avatarBg(d.author_name)}
                        {@const w = Math.max(10, (Math.abs(d.hours_delta) / selMaxDelta) * 100)}
                        <div style="display:flex; justify-content:space-between; align-items:center; gap:10px;">
                          <span style="display:inline-flex; align-items:center; gap:8px; min-width:0;">
                            <span style="width:28px; height:28px; border-radius:50%; background:{av2[0]}; color:{av2[1]}; font-size:10px; font-weight:700; display:inline-flex; align-items:center; justify-content:center; flex-shrink:0;">{initials(d.author_name)}</span>
                            <span style="font-size:12px; font-weight:500; color:#11142b; white-space:nowrap; overflow:hidden; text-overflow:ellipsis;">{d.author_name}</span>
                          </span>
                          <span style="display:flex; align-items:center; gap:12px;">
                            <span style="min-width:150px;">
                              <span style="font-size:12px; color:#11142b; font-weight:500;">{d.prev_hours_logged.toFixed(1)} → {d.new_hours_logged.toFixed(1)}</span>
                              <span style="display:block; height:6px; background:#e9ebf3; border-radius:3px; margin-top:4px; overflow:hidden;">
                                <span style="display:block; height:100%; width:{w}%; background:#2f6bff; border-radius:3px;"></span>
                              </span>
                            </span>
                            <span style="font-size:12px; font-weight:700; color:var(--c-success); min-width:48px; text-align:right;">+{d.hours_delta.toFixed(1)}h</span>
                          </span>
                        </div>
                      {/each}
                    </div>
                  {/if}
                  <button onclick={() => openRefreshBreakdown(selectedHistory.id)} style="margin-top:16px; width:100%; background:#fff; border:1px solid var(--c-border-2); border-radius:8px; padding:9px 12px; font-size:12px; font-weight:500; color:var(--c-text); display:flex; align-items:center; justify-content:center; gap:6px;">
                    View detailed worklogs
                    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/><polyline points="15 3 21 3 21 9"/><line x1="10" y1="14" x2="21" y2="3"/></svg>
                  </button>
                </div>

                <div style="border-top:1px solid var(--c-border); margin-top:16px; padding-top:14px;">
                  <div style="font-size:13px; font-weight:700; color:#11142b; margin-bottom:10px;">What's included</div>
                  <div style="font-size:11px; color:var(--c-text-2); display:flex; align-items:center; gap:6px; margin-bottom:6px;"><span style="color:#2563eb;">⛛</span> JQL filter</div>
                  <div style="position:relative; background:var(--c-bg); border:1px solid var(--c-border); border-radius:8px; padding:8px 30px 8px 10px; font-family:ui-monospace, monospace; font-size:11px; color:var(--c-text-2); white-space:nowrap; overflow:hidden; text-overflow:ellipsis;" title={team?.jira_jql || '—'}>
                    {team?.jira_jql || '—'}
                    <button onclick={() => copyJql(team?.jira_jql || '')} title="Copy JQL" style="position:absolute; right:6px; top:50%; transform:translateY(-50%); background:none; border:none; color:var(--c-text-3); cursor:pointer; font-size:13px;">⧉</button>
                  </div>
                  <div style="display:grid; grid-template-columns:1fr 1fr; gap:8px; margin-top:12px; font-size:11px;">
                    <div>
                      <div style="display:flex; align-items:center; gap:5px; color:var(--c-text-3);"><span>📅</span> Date range</div>
                      <div style="font-weight:600; color:var(--c-text); margin-top:2px;">{formatSprintRange(snapshot.start_date, snapshot.end_date)}</div>
                    </div>
                    <div>
                      <div style="display:flex; align-items:center; gap:5px; color:var(--c-text-3);"><span>🕐</span> Snapshot taken</div>
                      <div style="font-weight:600; color:var(--c-text); margin-top:2px;">{formatHistoryDateTime(selectedHistory.refreshed_at)}</div>
                    </div>
                  </div>
                </div>
              </div>
            {:else}
              <div style="background:#fff; border:1px solid var(--c-border); border-radius:12px; padding:32px 20px; text-align:center; color:var(--c-text-3); font-size:12px;">
                Select a refresh to see details.
              </div>
            {/if}
          </div>
        </div>

      {:else if activeTab === 'settings'}
      <!-- Snapshot Settings -->
      <div style="max-width:720px; margin:0 auto; display:flex; flex-direction:column; gap:20px;">
        <!-- General -->
        <div class="card">
          <div class="card-header">
            <div class="flex items-center gap-2">
              <span>📸</span>
              <h2 class="font-semibold" style="font-size:15px;">General</h2>
            </div>
          </div>
          <div class="card-body" style="display:flex; flex-direction:column; gap:12px; padding-top:16px;">
            <div class="form-group">
              <label class="label" style="font-size:11px;" for="snapshot-name">Snapshot name</label>
              <input
                id="snapshot-name"
                class="input"
                bind:value={editSnapshotName}
                placeholder="e.g. Sprint 25"
                disabled={!team?.is_admin || updatingSnapshot}
              />
            </div>
            <div style="display:grid; grid-template-columns:1fr 1fr; gap:12px;">
              <div class="form-group">
                <label class="label" style="font-size:11px;" for="snapshot-start">Start date</label>
                <input id="snapshot-start" type="date" class="input" bind:value={editSnapshotStart} disabled={!team?.is_admin || updatingSnapshot} />
              </div>
              <div class="form-group">
                <label class="label" style="font-size:11px;" for="snapshot-end">End date</label>
                <input id="snapshot-end" type="date" class="input" bind:value={editSnapshotEnd} disabled={!team?.is_admin || updatingSnapshot} />
              </div>
            </div>
            <p class="text-xs text-muted" style="font-size:10px;">Changing the date range re-fetches the snapshot data from Jira.</p>
            {#if team?.is_admin}
              <div>
                <button class="btn btn-primary btn-sm" onclick={saveEditedSnapshot} disabled={updatingSnapshot}>
                  {updatingSnapshot ? 'Saving…' : 'Save changes'}
                </button>
              </div>
            {:else}
              <p class="text-xs text-muted" style="font-size:10px;">Only team admins can edit this snapshot.</p>
            {/if}
            {#if editSnapshotError}
              <p class="text-xs" style="color:var(--c-danger);">⚠️ {editSnapshotError}</p>
            {/if}
            {#if snapshotSaved}
              <p class="text-xs" style="color:var(--c-success);">✓ Snapshot settings saved</p>
            {/if}
          </div>
        </div>
        {#if team?.is_admin}
        <!-- Danger zone -->
        <div class="card" style="border-color:#fecaca;">
          <div class="card-body" style="display:flex; align-items:center; justify-content:space-between; gap:12px; flex-wrap:wrap;">
            <div>
              <p class="font-semibold" style="font-size:14px; color:var(--c-danger);">Delete this snapshot</p>
              <p class="text-xs text-muted" style="margin-top:2px;">Permanently deletes the snapshot and its refresh history.</p>
            </div>
            <button class="btn btn-outline-danger btn-sm" onclick={deleteSnapshot} disabled={deleting}>
              {deleting ? 'Deleting…' : 'Delete Snapshot'}
            </button>
          </div>
        </div>
        {/if}
      </div>

      {/if}
    </div>
  {/if}

  {#if showRefreshBreakdownModal && selectedRefreshLog}
    {@const totHoursDiff = selectedRefreshLog.new_hours_logged - selectedRefreshLog.prev_hours_logged}
    {@const totSpDiff = selectedRefreshLog.new_story_points - selectedRefreshLog.prev_story_points}
    <div style="position:fixed; top:0; left:0; right:0; bottom:0; background:rgba(0,0,0,0.6); display:flex; align-items:center; justify-content:center; z-index:1000; backdrop-filter:blur(4px); transition: opacity 0.2s ease;">
      <div style="background:var(--c-surface); border:1px solid var(--c-border); border-radius:12px; width:740px; max-width:92%; max-height:90vh; display:flex; flex-direction:column; box-shadow:var(--shadow-lg); overflow:hidden;">
        
        <!-- Modal Header -->
        <div style="display:flex; justify-content:space-between; align-items:center; padding:18px 24px; border-bottom:1px solid var(--c-border);">
          <div>
            <h3 class="font-bold text-lg" style="margin:0; display:flex; align-items:center; gap:8px;">
              <span>🔄</span> Contributor Refresh Breakdown
            </h3>
            <p class="text-xs text-muted" style="margin-top:2px; margin-bottom:0;">
              See who added or changed hours between refreshes.
            </p>
          </div>
          <button class="btn-icon" onclick={() => showRefreshBreakdownModal = false} style="font-size:16px; width:32px; height:32px; border-radius:50%; border:none; display:flex; align-items:center; justify-content:center; background:transparent; cursor:pointer;">✕</button>
        </div>

        <!-- Refresh Picker & Summary Bar -->
        <div style="padding:16px 24px; background:var(--c-bg); border-bottom:1px solid var(--c-border); display:flex; flex-direction:column; gap:14px;">
          <div style="display:flex; justify-content:space-between; align-items:center; flex-wrap:wrap; gap:12px;">
            <div style="display:flex; align-items:center; gap:8px;">
              <span style="font-size:12px; font-weight:600; color:var(--c-text-2);">Select Refresh:</span>
              <select class="input" style="padding:6px 12px; font-size:12px; min-width:240px;" bind:value={selectedRefreshLogId}>
                {#each (snapshot?.refresh_history || []) as r, idx}
                  <option value={r.id}>
                    {formatDateTime(r.refreshed_at)} {idx === 0 ? '(Latest)' : ''}
                  </option>
                {/each}
              </select>
            </div>

            <!-- Total Diff Pills -->
            <div style="display:flex; align-items:center; gap:8px;">
              <span class="badge {totHoursDiff >= 0 ? 'badge-success' : 'badge-danger'}" style="padding:4px 8px; font-size:11px; font-weight:700;">
                Hours: {totHoursDiff > 0 ? '+' : ''}{totHoursDiff.toFixed(1)} hrs
              </span>
              <span class="badge {totSpDiff >= 0 ? 'badge-success' : 'badge-danger'}" style="padding:4px 8px; font-size:11px; font-weight:700;">
                SP: {totSpDiff > 0 ? '+' : ''}{totSpDiff.toFixed(1)} SP
              </span>
            </div>
          </div>

          <!-- Filter Controls -->
          <div style="display:flex; justify-content:space-between; align-items:center; gap:12px; flex-wrap:wrap;">
            <div style="position:relative; flex:1; min-width:200px;">
              <span style="position:absolute; left:10px; top:50%; transform:translateY(-50%); font-size:12px; color:var(--c-text-3);">🔍</span>
              <input 
                type="text" 
                class="input" 
                placeholder="Search contributors..." 
                bind:value={modalSearchQuery} 
                style="padding-left:30px; padding-top:6px; padding-bottom:6px; font-size:12px; width:100%;"
              />
            </div>

            <div style="display:flex; align-items:center; gap:12px;">
              <label style="display:inline-flex; align-items:center; gap:6px; font-size:12px; cursor:pointer; user-select:none; margin:0;">
                <input type="checkbox" bind:checked={modalOnlyChanges} />
                <span>Only changes ({selectedRefreshDeltas.filter(d => d.hours_delta !== 0).length})</span>
              </label>

              <select class="input" style="padding:6px 8px; font-size:12px;" bind:value={modalSortBy}>
                <option value="delta">Sort by Hours Added</option>
                <option value="hours">Sort by Total Hours</option>
                <option value="name">Sort by Name</option>
              </select>
            </div>
          </div>
        </div>

        <!-- Contributor List / Table -->
        <div style="flex:1; overflow-y:auto; padding:0; max-height:420px;">
          {#if selectedRefreshDeltas.length === 0}
            <div style="padding:40px; text-align:center; color:var(--c-text-3); font-size:13px;">
              No contributors found matching current filters.
            </div>
          {:else}
            <table style="width:100%; border-collapse:collapse; text-align:left; font-size:12px;">
              <thead>
                <tr style="background:var(--c-surface-2); border-bottom:1px solid var(--c-border); color:var(--c-text-3); position:sticky; top:0; z-index:1;">
                  <th style="padding:10px 16px; font-weight:600;">Contributor</th>
                  <th style="padding:10px 16px; font-weight:600; text-align:right; width:130px;">Previous Hours</th>
                  <th style="padding:10px 16px; font-weight:600; text-align:right; width:130px;">New Hours</th>
                  <th style="padding:10px 16px; font-weight:600; text-align:right; width:150px;">Hours Added / Delta</th>
                </tr>
              </thead>
              <tbody>
                {#each selectedRefreshDeltas as d}
                  <tr style="border-bottom:1px solid var(--c-border); background:var(--c-surface);">
                    <td style="padding:10px 16px; font-weight:600;">
                      <div style="display:flex; align-items:center; gap:8px;">
                        <span>{d.author_name}</span>
                        {#if d.prev_hours_logged === 0 && d.new_hours_logged > 0}
                          <span class="badge badge-primary" style="font-size:9px; padding:1px 5px;">New</span>
                        {/if}
                      </div>
                    </td>
                    <td style="padding:10px 16px; text-align:right; color:var(--c-text-2);">
                      {d.prev_hours_logged.toFixed(1)} hrs
                    </td>
                    <td style="padding:10px 16px; text-align:right; font-weight:600; color:var(--c-text);">
                      {d.new_hours_logged.toFixed(1)} hrs
                    </td>
                    <td style="padding:10px 16px; text-align:right;">
                      {#if d.hours_delta > 0}
                        <span class="badge badge-success" style="font-size:11px; padding:3px 8px; font-weight:700;">
                          +{d.hours_delta.toFixed(1)} hrs
                        </span>
                      {:else if d.hours_delta < 0}
                        <span class="badge badge-danger" style="font-size:11px; padding:3px 8px; font-weight:700;">
                          {d.hours_delta.toFixed(1)} hrs
                        </span>
                      {:else}
                        <span class="text-muted" style="font-size:11px;">0.0 hrs (No change)</span>
                      {/if}
                    </td>
                  </tr>
                {/each}
              </tbody>
            </table>
          {/if}
        </div>

        <!-- Modal Footer -->
        <div style="padding:14px 24px; border-top:1px solid var(--c-border); display:flex; justify-content:space-between; align-items:center; background:var(--c-surface);">
          <span class="text-xs text-muted">
            Showing {selectedRefreshDeltas.length} contributor{selectedRefreshDeltas.length === 1 ? '' : 's'}
          </span>
          <button class="btn btn-secondary btn-sm" onclick={() => showRefreshBreakdownModal = false}>
            Close
          </button>
        </div>

      </div>
    </div>
  {/if}
</div>

<style>
  .history-layout {
    display: grid;
    grid-template-columns: minmax(0, 1fr) 360px;
    gap: 16px;
    align-items: start;
  }
  @media (max-width: 1024px) {
    .history-layout {
      grid-template-columns: 1fr;
    }
  }
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
</style>
