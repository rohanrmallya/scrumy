<script lang="ts">
  import { page } from '$app/stores';
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { api, type Team, type TeamMate, type CapacityPlan, type Presentation, type JiraSnapshot, type JiraSettings, type Sprint } from '$lib/api';
  import { user } from '$lib/auth';

  const teamID = $derived($page.params.id);

  let team = $state<Team | null>(null);
  let capacityPlans = $state<CapacityPlan[]>([]);
  let presentations = $state<Presentation[]>([]);
  let snapshots = $state<JiraSnapshot[]>([]);
  let mates = $state<TeamMate[]>([]);
  let loading = $state(true);
  let error = $state('');

  let newAdminUsername = $state('');
  let addingAdmin = $state(false);
  let addingUsername = $state<string | null>(null);
  let removingUsername = $state<string | null>(null);
  let adminError = $state('');
  let adminNotice = $state('');

  // Admin user search (add flow) + client-side filter for current admins
  let adminSearchQuery = $state('');
  let adminSearchResults = $state<{ id: string; username: string }[]>([]);
  let searchingUsers = $state(false);
  let showUserDropdown = $state(false);
  let searchTimer: ReturnType<typeof setTimeout> | null = null;
  let adminFilter = $state('');

  const currentAdmins = $derived(team?.admins ?? []);
  const adminSet = $derived(new Set(currentAdmins));
  const filteredAdmins = $derived(
    adminFilter.trim()
      ? currentAdmins.filter(a => a.toLowerCase().includes(adminFilter.trim().toLowerCase()))
      : currentAdmins
  );

  // Tab switcher
  let activeTab = $state<'overview' | 'mates' | 'settings'>('overview');

  // Team Mates state
  let mateFilter = $state('');
  let showAddMateModal = $state(false);
  let editingMate = $state<TeamMate | null>(null);
  let mateName = $state('');
  let mateEmail = $state('');
  let mateRole = $state('');
  let mateJiraID = $state('');
  let savingMate = $state(false);
  let mateFormError = $state('');
  let bulkAddingMates = $state(false);

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

  const filteredMates = $derived(
    mateFilter.trim()
      ? mates.filter(m =>
          m.name.toLowerCase().includes(mateFilter.trim().toLowerCase()) ||
          m.email.toLowerCase().includes(mateFilter.trim().toLowerCase()) ||
          (m.role && m.role.toLowerCase().includes(mateFilter.trim().toLowerCase())) ||
          (m.username && m.username.toLowerCase().includes(mateFilter.trim().toLowerCase()))
        )
      : mates
  );

  const snapshotContributors = $derived.by(() => {
    const map = new Map<string, { name: string; email: string; jira_account_id: string }>();
    for (const s of snapshots) {
      const list = [...(s.data?.leaderboard || []), ...(s.data?.leaderboard_all || [])];
      for (const item of list) {
        if (!item.author_name) continue;
        const key = (item.author_email || item.author_name).toLowerCase().trim();
        if (!map.has(key)) {
          map.set(key, {
            name: item.author_name,
            email: item.author_email || '',
            jira_account_id: item.author_account_id || '',
          });
        }
      }
    }
    return Array.from(map.values());
  });

  const unaddedContributors = $derived.by(() => {
    const mateEmails = new Set(mates.map(m => m.email.toLowerCase().trim()).filter(Boolean));
    const mateNames = new Set(mates.map(m => m.name.toLowerCase().trim()));
    return snapshotContributors.filter(c => {
      if (c.email && mateEmails.has(c.email.toLowerCase().trim())) return false;
      if (mateNames.has(c.name.toLowerCase().trim())) return false;
      return true;
    });
  });

  function openAddMateModal(prefill?: { name: string; email: string; jira_account_id: string }) {
    editingMate = null;
    mateName = prefill?.name || '';
    mateEmail = prefill?.email || '';
    mateRole = '';
    mateJiraID = prefill?.jira_account_id || '';
    mateFormError = '';
    showAddMateModal = true;
  }

  function openEditMateModal(m: TeamMate) {
    editingMate = m;
    mateName = m.name;
    mateEmail = m.email || '';
    mateRole = m.role || '';
    mateJiraID = m.jira_account_id || '';
    mateFormError = '';
    showAddMateModal = true;
  }

  async function saveMate() {
    if (!mateName.trim()) {
      mateFormError = 'Name is required';
      return;
    }
    savingMate = true;
    mateFormError = '';
    try {
      if (editingMate) {
        const updated = await api.teams.mates.update(teamID, editingMate.id, {
          name: mateName.trim(),
          email: mateEmail.trim(),
          role: mateRole.trim(),
          jira_account_id: mateJiraID.trim(),
        });
        mates = mates.map(m => m.id === updated.id ? updated : m);
      } else {
        const created = await api.teams.mates.add(teamID, {
          name: mateName.trim(),
          email: mateEmail.trim(),
          role: mateRole.trim(),
          jira_account_id: mateJiraID.trim(),
        });
        mates = [...mates, created];
      }
      showAddMateModal = false;
    } catch (e: any) {
      mateFormError = e.message;
    } finally {
      savingMate = false;
    }
  }

  async function deleteMate(m: TeamMate) {
    if (!confirm(`Are you sure you want to remove ${m.name} from this team?`)) return;
    try {
      await api.teams.mates.delete(teamID, m.id);
      mates = mates.filter(x => x.id !== m.id);
    } catch (e: any) {
      alert(`Failed to remove team mate: ${e.message}`);
    }
  }

  async function addAllSnapshotContributors() {
    if (unaddedContributors.length === 0) return;
    bulkAddingMates = true;
    try {
      await api.teams.mates.bulkAdd(teamID, unaddedContributors.map(c => ({
        name: c.name,
        email: c.email,
        jira_account_id: c.jira_account_id,
      })));
      mates = await api.teams.mates.list(teamID);
    } catch (e: any) {
      alert(`Failed to add contributors: ${e.message}`);
    } finally {
      bulkAddingMates = false;
    }
  }

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
  let newSnapshotAutoRefresh = $state(false);
  let newSnapshotAutoRefreshTime = $state('09:00');
  let creatingSnapshot = $state(false);

  onMount(async () => {
    try {
      const [p, c, pr, sn, tm] = await Promise.all([
        api.teams.get(teamID),
        api.capacity.list(teamID),
        api.presentations.list(teamID),
        api.jira.listSnapshots(teamID),
        api.teams.mates.list(teamID),
      ]);
      team = p;
      teamNameDraft = p.name ?? '';
      capacityPlans = c;
      presentations = pr;
      snapshots = sn;
      mates = tm;

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
      await api.jira.testConnection(teamID, {
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
      await api.jira.saveSettings(teamID, {
        jira_url: jiraURL.trim(),
        jira_user: jiraUser.trim(),
        jira_token: jiraToken ? jiraToken.trim() : undefined,
        jira_jql: jiraJQL.trim(),
        jira_sp_field: jiraSPField.trim(),
        jira_insecure: jiraInsecure,
      });
      team = await api.teams.get(teamID);
      jiraTokenSet = !!team.jira_token_set;
      jiraInsecure = !!team.jira_insecure;
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
      const snap = await api.jira.createSnapshot(teamID, {
        name: newSnapshotName.trim(),
        start_date: newSnapshotStart,
        end_date: newSnapshotEnd,
        all_worklogs: false,
        auto_refresh_enabled: newSnapshotAutoRefresh,
        auto_refresh_time: newSnapshotAutoRefreshTime,
      });
      snapshots = [snap, ...snapshots];
      showCreateSnapshotForm = false;
      newSnapshotName = '';
      newSnapshotStart = '';
      newSnapshotEnd = '';
      newSnapshotAutoRefresh = false;
      newSnapshotAutoRefreshTime = '09:00';
      
      // Navigate to the dedicated snapshot page!
      goto(`/teams/${teamID}/snapshots/${snap.id}`);
    } catch (e: any) {
      alert(`Failed to create snapshot: ${e.message}`);
    } finally {
      creatingSnapshot = false;
    }
  }

  async function deleteSnapshot(id: string) {
    if (!confirm('Are you sure you want to delete this snapshot?')) return;
    try {
      await api.jira.deleteSnapshot(teamID, id);
      snapshots = snapshots.filter(s => s.id !== id);
    } catch (e: any) {
      alert(`Failed to delete: ${e.message}`);
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
    } catch (e: any) {
      editSnapshotError = e.message;
    } finally {
      updatingSnapshot = false;
    }
  }


  function statusBadge(s: string) {
    if (s === 'active') return 'badge-success';
    if (s === 'archived') return 'badge-default';
    return 'badge-warning';
  }

  async function deleteCapacity(cpID: string) {
    if (!confirm('Delete this capacity team?')) return;
    await api.capacity.delete(teamID, cpID);
    capacityPlans = await api.capacity.list(teamID);
    team = await api.teams.get(teamID);
  }

  async function deletePres(presID: string) {
    if (!confirm('Delete this presentation?')) return;
    await api.presentations.delete(teamID, presID);
    presentations = await api.presentations.list(teamID);
    team = await api.teams.get(teamID);
  }

  async function addAdminByUsername(username: string, allProjects: boolean = false) {
    const name = username.trim();
    if (!name || addingUsername) return;
    addingUsername = name;
    addingAdmin = true;
    adminError = '';
    adminNotice = '';
    try {
      if (allProjects) {
        const res = await api.admin.addAdminToAllTeams(name);
        adminNotice = `"${name}" added as admin to all ${res.total_teams} projects (${res.added} newly added)`;
      } else {
        await api.teams.addAdmin(teamID, name);
        adminNotice = `"${name}" added as admin`;
      }
      team = await api.teams.get(teamID);
      newAdminUsername = '';
      adminSearchQuery = '';
      adminSearchResults = [];
      showUserDropdown = false;
    } catch (e: any) {
      adminError = e.message;
    } finally {
      addingUsername = null;
      addingAdmin = false;
    }
  }

  async function addAdmin(allProjects: boolean = false) {
    const name = (adminSearchQuery.trim() || newAdminUsername.trim());
    if (!name) return;
    await addAdminByUsername(name, allProjects);
  }

  function onAdminSearchInput(value: string) {
    adminSearchQuery = value;
    adminError = '';
    if (searchTimer) clearTimeout(searchTimer);
    const q = value.trim();
    if (q.length < 1) {
      adminSearchResults = [];
      searchingUsers = false;
      showUserDropdown = false;
      return;
    }
    showUserDropdown = true;
    searchingUsers = true;
    searchTimer = setTimeout(async () => {
      try {
        adminSearchResults = await api.users.search(q);
      } catch (e: any) {
        adminSearchResults = [];
      } finally {
        searchingUsers = false;
      }
    }, 250);
  }

  async function removeAdmin(username: string) {
    if (!confirm(`Remove ${username} as admin?`)) return;
    removingUsername = username;
    adminError = '';
    adminNotice = '';
    try {
      await api.teams.removeAdmin(teamID, username);
      team = await api.teams.get(teamID);
      adminNotice = `"${username}" removed`;
    } catch (e: any) {
      adminError = e.message;
    } finally {
      removingUsername = null;
    }
  }

  async function deleteTeam() {
    if (!confirm('PERMANENTLY DELETE THIS ENTIRE TEAM AND ALL ITS DATA?')) return;
    try {
      await api.teams.delete(teamID);
      goto('/');
    } catch (e: any) {
      alert(e.message);
    }
  }

  let teamNameDraft = $state('');
  let renamingTeam = $state(false);
  let renameError = $state('');
  let renameSaved = $state(false);

  async function saveTeamName() {
    const name = teamNameDraft.trim();
    if (!name) {
      renameError = 'Name is required';
      return;
    }
    renamingTeam = true;
    renameError = '';
    renameSaved = false;
    try {
      team = await api.teams.update(teamID, name);
      renameSaved = true;
    } catch (e: any) {
      renameError = e.message;
    } finally {
      renamingTeam = false;
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
  <title>{team?.name ?? 'Team'} – Scrumy</title>
</svelte:head>

<div class="page-content fade-in">
  {#if loading}
    <div class="flex items-center gap-3" style="padding:40px 0">
      <span class="spinner"></span>
      <span class="text-muted">Loading team…</span>
    </div>
  {:else if error}
    <div class="badge badge-danger" style="padding:12px 20px; border-radius:8px; font-size:14px; text-transform:none; letter-spacing:0; font-weight:400;">
      ⚠️ {error}
    </div>
  {:else}
    <!-- Header -->
    <div style="margin-bottom:20px;">
      <h1 class="text-2xl font-bold">{team?.name}</h1>
      <p class="text-sm text-muted" style="margin-top:2px;">Team Workspace</p>
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
        class="tab-btn {activeTab === 'mates' ? 'active' : ''}" 
        onclick={() => activeTab = 'mates'}
      >
        👥 Team Mates ({mates.length})
      </button>
      <button 
        class="tab-btn {activeTab === 'settings' ? 'active' : ''}" 
        onclick={() => { activeTab = 'settings'; teamNameDraft = team?.name ?? ''; renameError = ''; renameSaved = false; }}
      >
        ⚙️ Settings
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
            {#if team?.is_admin}
              <a href="/teams/{teamID}/capacity/new" class="btn btn-primary btn-sm">+ Create New</a>
            {/if}
          </div>

          <div>
            {#if capacityPlans.length === 0}
              <div class="empty-state">
                <p>No capacity plans yet.</p>
                {#if team?.is_admin}
                  <a href="/teams/{teamID}/capacity/new" class="btn btn-primary btn-sm" style="margin-top:12px;">Create First Plan</a>
                {/if}
              </div>
            {:else}
              {#each capacityPlans.slice(0, 3) as cp (cp.id)}
                <a
                  href="/teams/{teamID}/capacity/{cp.id}"
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
                      {#if team?.is_admin}
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
              <div style="padding:12px; text-align:center;">
                <a
                  href="/teams/{teamID}/capacity"
                  class="btn btn-secondary btn-sm"
                  style="display:flex; justify-content:center; width:100%;"
                >
                  View All Capacity Plans ({capacityPlans.length})
                </a>
              </div>
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
            {#if team?.is_admin}
              <div class="flex gap-2">
                <a href="/teams/{teamID}/presentations/new?type=intro" class="btn btn-secondary btn-sm">+ Intro</a>
                <a href="/teams/{teamID}/presentations/new?type=retro" class="btn btn-secondary btn-sm">+ Retro</a>
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
                  {#each intros.slice(0, 3) as pres (pres.id)}
                    <div class="flex items-center justify-between" style="padding:10px 12px;background:var(--c-bg);border-radius:8px;">
                      <div class="flex items-center gap-3">
                        {#if pres.status === 'published'}
                          <a href="/teams/{teamID}/presentations/{pres.id}/view"
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
                        {#if team?.is_admin}
                          <a href="/teams/{teamID}/presentations/{pres.id}/edit" class="btn-icon" title="Edit" style="font-size:14px;">✎</a>
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
                  {#each retros.slice(0, 3) as pres (pres.id)}
                    <div class="flex items-center justify-between" style="padding:10px 12px;background:var(--c-bg);border-radius:8px;">
                      <div class="flex items-center gap-3">
                        {#if pres.status === 'published'}
                          <a href="/teams/{teamID}/presentations/{pres.id}/view"
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
                        {#if team?.is_admin}
                          <a href="/teams/{teamID}/presentations/{pres.id}/edit" class="btn-icon" title="Edit" style="font-size:14px;">✎</a>
                          <button class="btn-icon" style="color:var(--c-danger);font-size:14px;" onclick={() => deletePres(pres.id)} title="Delete">✕</button>
                        {/if}
                      </div>
                    </div>
                  {/each}
                </div>
              {/if}
            </div>
          </div>
          {#if presentations.length > 0}
            <div style="padding:12px; text-align:center; border-top:1px solid var(--c-border);">
              <a
                href="/teams/{teamID}/presentations"
                class="btn btn-secondary btn-sm"
                style="display:flex; justify-content:center; width:100%;"
              >
                View All Presentations ({presentations.length})
              </a>
            </div>
          {/if}
        </div>

        <!-- Jira Snapshots -->
        <div class="card">
          <div class="card-header" style="display:flex; justify-content:space-between; align-items:center;">
            <div class="flex items-center gap-2">
              <span style="font-size:18px;">📸</span>
              <h2 class="font-semibold" style="font-size:16px;">Jira Snapshots</h2>
            </div>
            {#if team?.is_admin && team?.jira_url}
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

                <div style="background:var(--c-surface); border:1px solid var(--c-border); border-radius:6px; padding:8px 10px; margin-top:2px;">
                  <label style="display:flex; align-items:center; gap:8px; cursor:pointer; font-size:11px; font-weight:500;">
                    <input type="checkbox" bind:checked={newSnapshotAutoRefresh} />
                    <span>⏰ Enable Daily Auto-Refresh</span>
                  </label>
                  {#if newSnapshotAutoRefresh}
                    <div style="margin-top:8px; display:flex; align-items:center; gap:8px;">
                      <label class="label" style="font-size:10px; margin:0; white-space:nowrap;">Daily Time</label>
                      <input type="time" class="input" style="padding:4px 8px; font-size:11px; width:auto;" bind:value={newSnapshotAutoRefreshTime} />
                    </div>
                    <p class="text-xs text-muted" style="margin-top:4px; font-size:10px;">
                      Automatically refreshes daily at this time between start and end dates.
                    </p>
                  {/if}
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
              {#each snapshots.slice(0, 3) as s (s.id)}
                <a
                  href="/teams/{teamID}/snapshots/{s.id}"
                  style="display:block; padding:16px 20px; border-bottom:1px solid var(--c-border); text-decoration:none; transition:background 150ms ease; cursor:pointer;"
                  onmouseover={e => (e.currentTarget as HTMLElement).style.background = 'var(--c-bg)'}
                  onmouseout={e => (e.currentTarget as HTMLElement).style.background = ''}
                  onfocus={e => (e.currentTarget as HTMLElement).style.background = 'var(--c-bg)'}
                  onblur={e => (e.currentTarget as HTMLElement).style.background = ''}
                >
                  <div class="flex justify-between items-center" style="margin-bottom:8px;">
                    <span class="font-semibold" style="font-size:14px; color:var(--c-primary);">{s.name}</span>
                    {#if team?.is_admin}
                      <div class="flex items-center gap-1" style="display: flex; align-items: center; gap: 4px;">
                        <button
                          class="btn-icon"
                          style="color:var(--c-primary); font-size:13px; display:inline-flex; align-items:center; justify-content:center; width:22px; height:22px; padding:0; border:none; background:transparent;"
                          onclick={(e) => { e.preventDefault(); e.stopPropagation(); openEditSnapshot(s); }}
                          title="Edit"
                        >✎</button>
                        <button
                          class="btn-icon"
                          style="color:var(--c-danger); font-size:13px; display:inline-flex; align-items:center; justify-content:center; width:22px; height:22px; padding:0; border:none; background:transparent;"
                          onclick={(e) => { e.preventDefault(); e.stopPropagation(); deleteSnapshot(s.id); }}
                          title="Delete"
                        >✕</button>
                      </div>
                    {/if}
                  <div style="display:flex; justify-content:space-between; align-items:center;">
                    <p class="text-xs text-muted" style="margin:0;">{s.start_date} to {s.end_date}</p>
                    {#if s.auto_refresh_enabled}
                      <span class="badge badge-primary" style="font-size:10px; padding:2px 6px;">⏰ Auto: {s.auto_refresh_time || '09:00'}</span>
                    {/if}
                  </div>
                </a>
              {/each}
              <div style="padding:12px; text-align:center;">
                <a
                  href="/teams/{teamID}/snapshots"
                  class="btn btn-secondary btn-sm"
                  style="display:flex; justify-content:center; width:100%;"
                >
                  View All Snapshots ({snapshots.length})
                </a>
              </div>
            {/if}
          </div>
        </div>

      </div>
    {:else if activeTab === 'mates'}
      <div style="display:flex; flex-direction:column; gap:20px;">
        <!-- Header banner / actions -->
        <div class="card" style="padding:16px 20px;">
          <div style="display:flex; justify-content:space-between; align-items:center; flex-wrap:wrap; gap:16px;">
            <div>
              <h2 class="font-bold text-lg" style="margin:0; display:flex; align-items:center; gap:8px;">
                <span>👥</span> Team Mates ({mates.length})
              </h2>
              <p class="text-xs text-muted" style="margin:4px 0 0 0;">
                Members belonging to this team. Jira worklogs are attributed to these team mates, and corporate emails link to Scrumy accounts.
              </p>
            </div>
            <div style="display:flex; align-items:center; gap:10px; flex-wrap:wrap;">
              <div style="position:relative; width:220px;">
                <span style="position:absolute; left:10px; top:50%; transform:translateY(-50%); font-size:12px; color:var(--c-text-3);">🔍</span>
                <input
                  type="text"
                  class="input"
                  placeholder="Filter mates..."
                  bind:value={mateFilter}
                  style="padding-left:30px; font-size:12px; height:34px;"
                />
                {#if mateFilter}
                  <button
                    onclick={() => mateFilter = ''}
                    style="position:absolute; right:8px; top:50%; transform:translateY(-50%); background:none; border:none; font-size:11px; color:var(--c-text-3); cursor:pointer;"
                  >✕</button>
                {/if}
              </div>
              {#if team?.is_admin}
                <button class="btn btn-primary btn-sm" onclick={() => openAddMateModal()}>
                  + Add Team Mate
                </button>
              {/if}
            </div>
          </div>
        </div>

        <!-- Suggestion box for unadded contributors -->
        {#if team?.is_admin && unaddedContributors.length > 0}
          <div style="background:#eff6ff; border:1px solid #bfdbfe; border-radius:10px; padding:16px 20px; display:flex; justify-content:space-between; align-items:center; gap:16px; flex-wrap:wrap;">
            <div>
              <div style="font-size:14px; font-weight:700; color:#1e40af; display:flex; align-items:center; gap:6px;">
                <span>💡</span> {unaddedContributors.length} contributor{unaddedContributors.length > 1 ? 's' : ''} found in snapshots not yet in team
              </div>
              <div style="font-size:12px; color:#3b82f6; margin-top:4px;">
                {unaddedContributors.slice(0, 5).map(c => c.name).join(', ')}{unaddedContributors.length > 5 ? ` and ${unaddedContributors.length - 5} more` : ''}
              </div>
            </div>
            <div style="display:flex; gap:8px;">
              <button class="btn btn-primary btn-sm" onclick={addAllSnapshotContributors} disabled={bulkAddingMates}>
                {bulkAddingMates ? 'Adding...' : `+ Add All to Team (${unaddedContributors.length})`}
              </button>
            </div>
          </div>
        {/if}

        <!-- Team Mates Table Card -->
        <div class="card" style="overflow:hidden;">
          {#if filteredMates.length === 0}
            <div style="padding:48px 20px; text-align:center;">
              <span style="font-size:36px; display:block; margin-bottom:10px;">👥</span>
              <p class="font-semibold text-base" style="margin:0 0 6px 0; color:var(--c-text);">No team mates found</p>
              <p class="text-xs text-muted" style="margin:0 0 16px 0;">
                {mateFilter ? 'Try a different filter query.' : 'Add your first team mate from snapshots or manually.'}
              </p>
              {#if team?.is_admin}
                {#if unaddedContributors.length > 0}
                  <button class="btn btn-primary btn-sm" onclick={addAllSnapshotContributors} disabled={bulkAddingMates}>
                    Add All Contributors from Snapshots ({unaddedContributors.length})
                  </button>
                {:else}
                  <button class="btn btn-primary btn-sm" onclick={() => openAddMateModal()}>
                    + Add Team Mate
                  </button>
                {/if}
              {/if}
            </div>
          {:else}
            <div style="overflow-x:auto;">
              <table style="width:100%; border-collapse:collapse; font-size:13px; text-align:left;">
                <thead>
                  <tr style="border-bottom:1px solid var(--c-border); background:var(--c-surface-2); color:var(--c-text-2); font-size:11px; text-transform:uppercase; letter-spacing:0.5px;">
                    <th style="padding:12px 18px; font-weight:600;">Member</th>
                    <th style="padding:12px 18px; font-weight:600;">Email</th>
                    <th style="padding:12px 18px; font-weight:600;">Role</th>
                    <th style="padding:12px 18px; font-weight:600;">Scrumy User</th>
                    <th style="padding:12px 18px; font-weight:600;">Jira ID</th>
                    {#if team?.is_admin}
                      <th style="padding:12px 18px; font-weight:600; text-align:right;">Actions</th>
                    {/if}
                  </tr>
                </thead>
                <tbody>
                  {#each filteredMates as m}
                    {@const av = avatarBg(m.name)}
                    <tr style="border-bottom:1px solid var(--c-border); background:var(--c-surface);">
                      <td style="padding:12px 18px; font-weight:600; color:var(--c-text); white-space:nowrap;">
                        <div style="display:flex; align-items:center; gap:10px;">
                          <span style="width:32px; height:32px; border-radius:50%; background:{av[0]}; color:{av[1]}; font-size:11px; font-weight:700; display:inline-flex; align-items:center; justify-content:center; flex-shrink:0;">
                            {initials(m.name)}
                          </span>
                          <div>
                            <div>{m.name}</div>
                          </div>
                        </div>
                      </td>
                      <td style="padding:12px 18px; color:var(--c-text-2); white-space:nowrap;">
                        {#if m.email}
                          <a href="mailto:{m.email}" style="color:var(--c-primary); text-decoration:none;">{m.email}</a>
                        {:else}
                          <span class="text-muted" style="font-size:12px;">—</span>
                        {/if}
                      </td>
                      <td style="padding:12px 18px; white-space:nowrap;">
                        {#if m.role}
                          <span class="badge badge-default" style="font-size:11px; padding:2px 8px;">{m.role}</span>
                        {:else}
                          <span class="text-muted" style="font-size:12px;">Member</span>
                        {/if}
                      </td>
                      <td style="padding:12px 18px; white-space:nowrap;">
                        {#if m.username}
                          <span class="badge badge-success" style="font-size:11px; padding:3px 8px; display:inline-flex; align-items:center; gap:4px;" title="Linked to Scrumy user: @{m.username}">
                            <span>✓</span> @{m.username}
                          </span>
                        {:else}
                          <span class="text-muted" style="font-size:11px;" title="No Scrumy user registered with this email yet">Unlinked</span>
                        {/if}
                      </td>
                      <td style="padding:12px 18px; color:var(--c-text-3); font-size:11px; font-family:monospace; white-space:nowrap;">
                        {m.jira_account_id ? m.jira_account_id.slice(0, 16) + (m.jira_account_id.length > 16 ? '...' : '') : '—'}
                      </td>
                      {#if team?.is_admin}
                        <td style="padding:12px 18px; text-align:right; white-space:nowrap;">
                          <button class="btn btn-secondary btn-sm" style="font-size:11px; padding:3px 8px; margin-right:6px;" onclick={() => openEditMateModal(m)}>
                            Edit
                          </button>
                          <button class="btn btn-outline-danger btn-sm" style="font-size:11px; padding:3px 8px;" onclick={() => deleteMate(m)}>
                            Remove
                          </button>
                        </td>
                      {/if}
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>
          {/if}
        </div>
      </div>
    {:else if activeTab === 'settings'}
      <!-- Team Settings -->
      <div style="max-width:720px; margin:0 auto; display:flex; flex-direction:column; gap:20px;">
        <!-- General -->
        <div class="card">
          <div class="card-header">
            <div class="flex items-center gap-2">
              <span>👥</span>
              <h2 class="font-semibold" style="font-size:15px;">General</h2>
            </div>
          </div>
          <div class="card-body" style="display:flex; flex-direction:column; gap:12px; padding-top:16px;">
            <div class="form-group">
              <label class="label" style="font-size:11px;" for="team-name">Team name</label>
              <div class="flex gap-2">
                <input
                  id="team-name"
                  class="input"
                  bind:value={teamNameDraft}
                  placeholder="e.g. Phoenix"
                  disabled={!team?.is_admin || renamingTeam}
                  onkeydown={(e) => { if (e.key === 'Enter') saveTeamName(); }}
                />
                {#if team?.is_admin}
                  <button class="btn btn-primary btn-sm" style="white-space:nowrap;" onclick={saveTeamName} disabled={renamingTeam || !teamNameDraft.trim()}>
                    {renamingTeam ? 'Saving…' : 'Save'}
                  </button>
                {/if}
              </div>
              {#if renameError}
                <p class="text-xs" style="color:var(--c-danger); margin-top:6px;">⚠️ {renameError}</p>
              {/if}
              {#if renameSaved}
                <p class="text-xs" style="color:var(--c-success); margin-top:6px;">✓ Team name saved</p>
              {/if}
              {#if !team?.is_admin}
                <p class="text-xs text-muted" style="font-size:10px;">Only team admins can rename this team.</p>
              {/if}
            </div>
          </div>
        </div>
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
              <input class="input" bind:value={jiraURL} placeholder="https://your-domain.atlassian.net" disabled={!team?.is_admin} />
            </div>
            <div class="form-group">
              <label class="label" style="font-size:11px;">Email / Username</label>
              <input class="input" bind:value={jiraUser} placeholder="email@company.com" disabled={!team?.is_admin} />
            </div>
            <div class="form-group">
              <label class="label" style="font-size:11px;">API Token</label>
              <input type="password" class="input" bind:value={jiraToken} placeholder={jiraTokenSet ? "•••••••• (Configured)" : "Enter API Token"} disabled={!team?.is_admin} />
            </div>
            <div class="form-group">
              <label class="label" style="font-size:11px;">Base JQL Filter</label>
              <input class="input" bind:value={jiraJQL} placeholder="project = PROJ AND statusCategory = Done" disabled={!team?.is_admin} />
            </div>
            <div class="form-group">
              <label class="label" style="font-size:11px;">Story Points Field (Optional)</label>
              <input class="input" bind:value={jiraSPField} placeholder="e.g. customfield_10016" disabled={!team?.is_admin} />
              <p class="text-xs text-muted" style="margin-top:2px; font-size:10px;">Left blank to auto-detect "Story Points"</p>
            </div>
            <div class="form-group flex items-center gap-2" style="margin-top: 4px;">
              <input type="checkbox" id="jiraInsecure" bind:checked={jiraInsecure} disabled={!team?.is_admin} />
              <label for="jiraInsecure" class="label" style="font-size:11px; margin: 0; cursor: pointer;">Skip TLS Verification (Insecure)</label>
            </div>

            {#if team?.is_admin}
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
        {#if team?.is_admin}
        <!-- Admins Management -->
        <div class="card">
          <div class="card-header">
            <div class="flex items-center gap-2">
              <span>👥</span>
              <h2 class="font-semibold" style="font-size:15px;">Manage Admins ({currentAdmins.length})</h2>
            </div>
            <input
              class="input"
              style="max-width:220px; padding:6px 10px; font-size:12px;"
              placeholder="Filter admins…"
              bind:value={adminFilter}
            />
          </div>
          <div class="card-body" style="display:flex; flex-direction:column; gap:16px;">
            {#if adminError}
              <div class="badge badge-danger" style="padding:8px 12px; border-radius:6px; font-size:12px; text-transform:none; letter-spacing:0; font-weight:400;">
                ⚠️ {adminError}
              </div>
            {/if}
            {#if adminNotice}
              <div class="badge badge-success" style="padding:8px 12px; border-radius:6px; font-size:12px; text-transform:none; letter-spacing:0; font-weight:400;">
                ✓ {adminNotice}
              </div>
            {/if}

            <!-- Add admin via user search -->
            <div class="form-group" style="position:relative;">
              <label class="label" style="font-size:11px;" for="admin-search">Add admin — search users</label>
              <div class="flex gap-2">
                <input
                  id="admin-search"
                  class="input"
                  placeholder="Type a username to search…"
                  value={adminSearchQuery}
                  oninput={(e) => onAdminSearchInput(e.currentTarget.value)}
                  onfocus={() => { if (adminSearchQuery.trim()) showUserDropdown = true; }}
                  onkeydown={(e) => { if (e.key === 'Enter') addAdmin(); if (e.key === 'Escape') showUserDropdown = false; }}
                  autocomplete="off"
                />
                <button class="btn btn-primary btn-sm" style="white-space:nowrap;" onclick={() => addAdmin(false)} disabled={addingAdmin || !adminSearchQuery.trim()}>
                  {addingAdmin ? 'Adding…' : 'Add Admin'}
                </button>
                {#if $user?.role === 'admin'}
                  <button class="btn btn-secondary btn-sm" style="white-space:nowrap;" onclick={() => addAdmin(true)} disabled={addingAdmin || !adminSearchQuery.trim()} title="Add this admin to all projects">
                    Add to All Projects
                  </button>
                {/if}
              </div>
              {#if showUserDropdown && adminSearchQuery.trim()}
                <div style="position:absolute; top:100%; left:0; right:0; z-index:30; background:var(--c-surface); border:1px solid var(--c-border); border-radius:8px; margin-top:4px; box-shadow:var(--shadow-lg); overflow:hidden; max-height:260px; overflow-y:auto;">
                  {#if searchingUsers}
                    <div class="flex items-center gap-2" style="padding:12px 14px;">
                      <span class="spinner"></span>
                      <span class="text-sm text-muted">Searching…</span>
                    </div>
                  {:else if adminSearchResults.length === 0}
                    <div style="padding:12px 14px;">
                      <p class="text-sm text-muted">No users found for “{adminSearchQuery.trim()}”. Press Add to try the exact username.</p>
                    </div>
                  {:else}
                    <table class="table">
                      <tbody>
                        {#each adminSearchResults as u (u.id)}
                          <tr>
                            <td class="font-medium" style="font-size:13px;">{u.username}</td>
                            <td style="text-align:right;">
                              {#if adminSet.has(u.username)}
                                <span class="badge badge-success" style="font-size:10px;">Admin</span>
                              {:else}
                                <button
                                  class="btn btn-primary btn-sm"
                                  style="padding:4px 10px; font-size:12px;"
                                  onclick={() => addAdminByUsername(u.username)}
                                  disabled={addingUsername === u.username}
                                >
                                  {addingUsername === u.username ? 'Adding…' : 'Add'}
                                </button>
                              {/if}
                            </td>
                          </tr>
                        {/each}
                      </tbody>
                    </table>
                  {/if}
                </div>
              {/if}
            </div>

            <!-- Current admins table -->
            <div>
              <p class="text-xs font-semibold text-muted" style="text-transform:uppercase; letter-spacing:0.06em; margin-bottom:8px;">Current admins</p>
              {#if currentAdmins.length === 0}
                <div class="empty-state"><p>No admins yet.</p></div>
              {:else if filteredAdmins.length === 0}
                <div class="empty-state"><p>No admins match “{adminFilter}”.</p></div>
              {:else}
                <table class="table">
                  <thead>
                    <tr>
                      <th style="width:48px;">#</th>
                      <th>Username</th>
                      <th style="text-align:right;">Action</th>
                    </tr>
                  </thead>
                  <tbody>
                    {#each filteredAdmins as admin, i (admin)}
                      <tr>
                        <td class="text-sm text-muted">{i + 1}</td>
                        <td class="font-medium" style="font-size:13px;">{admin}</td>
                        <td style="text-align:right;">
                          <div style="display:inline-flex; align-items:center; gap:6px;">
                            {#if $user?.role === 'admin'}
                              <button
                                class="btn btn-secondary btn-sm"
                                style="padding:4px 8px; font-size:11px;"
                                onclick={() => addAdminByUsername(admin, true)}
                                disabled={addingUsername === admin}
                                title="Ensure {admin} is an admin on all projects"
                              >
                                Apply to All Projects
                              </button>
                            {/if}
                            <button
                              class="btn btn-secondary btn-sm"
                              style="padding:4px 10px; font-size:12px; color:var(--c-danger);"
                              onclick={() => removeAdmin(admin)}
                              disabled={removingUsername === admin || currentAdmins.length <= 1}
                              title={currentAdmins.length <= 1 ? 'Cannot remove the last admin' : `Remove ${admin}`}
                            >
                              {removingUsername === admin ? 'Removing…' : 'Remove'}
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
        </div>
        {/if}

        {#if team?.is_admin}
        <!-- Danger zone -->
        <div class="card" style="border-color:#fecaca;">
          <div class="card-body" style="display:flex; align-items:center; justify-content:space-between; gap:12px; flex-wrap:wrap;">
            <div>
              <p class="font-semibold" style="font-size:14px; color:var(--c-danger);">Delete this team</p>
              <p class="text-xs text-muted" style="margin-top:2px;">Permanently deletes the team and all its capacity plans, snapshots, and presentations.</p>
            </div>
            <button class="btn btn-outline-danger btn-sm" onclick={deleteTeam}>Delete Team</button>
          </div>
        </div>
        {/if}
      </div>
    {/if}
  {/if}
  {#if activeEditSnapshot}
    <div style="position:fixed; top:0; left:0; right:0; bottom:0; background:rgba(0,0,0,0.55); display:flex; align-items:center; justify-content:center; z-index:1000; backdrop-filter:blur(4px); transition: opacity 0.2s ease;">
      <div style="background:var(--c-surface); border:1px solid var(--c-border); border-radius:12px; width:480px; max-width:90%; padding:24px; box-shadow:var(--shadow-lg);">
        <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:20px; border-bottom:1px solid var(--c-border); padding-bottom:12px;">
          <h3 class="font-semibold text-base" style="color:var(--c-primary); margin:0;">Edit Snapshot Settings</h3>
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
            {updatingSnapshot ? 'Saving...' : 'Save'}
          </button>
        </div>
      </div>
    </div>
  {/if}

  {#if showAddMateModal}
    <div style="position:fixed; top:0; left:0; right:0; bottom:0; background:rgba(0,0,0,0.55); display:flex; align-items:center; justify-content:center; z-index:1000; backdrop-filter:blur(4px); transition: opacity 0.2s ease;">
      <div style="background:var(--c-surface); border:1px solid var(--c-border); border-radius:12px; width:480px; max-width:90%; padding:24px; box-shadow:var(--shadow-lg);">
        <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:20px; border-bottom:1px solid var(--c-border); padding-bottom:12px;">
          <h3 class="font-semibold text-base" style="color:var(--c-primary); margin:0;">
            {editingMate ? 'Edit Team Mate' : 'Add Team Mate'}
          </h3>
          <button class="btn-icon" onclick={() => showAddMateModal = false} style="font-size:14px; width:28px; height:28px; border-radius:50%; border:none; display:flex; align-items:center; justify-content:center; background:transparent; cursor:pointer;">✕</button>
        </div>

        {#if !editingMate && unaddedContributors.length > 0}
          <div style="margin-bottom:16px; background:var(--c-surface-2); padding:10px 14px; border-radius:8px; border:1px solid var(--c-border);">
            <label class="label" style="font-size:11px; margin-bottom:6px;" for="quick-pick-contrib">Quick-pick from snapshot contributor:</label>
            <select
              id="quick-pick-contrib"
              class="input"
              style="padding:6px; font-size:12px;"
              onchange={(e) => {
                const selected = unaddedContributors.find(c => c.name === e.currentTarget.value);
                if (selected) {
                  mateName = selected.name;
                  mateEmail = selected.email;
                  mateJiraID = selected.jira_account_id;
                }
              }}
            >
              <option value="">-- Select a contributor --</option>
              {#each unaddedContributors as c}
                <option value={c.name}>{c.name}{c.email ? ` (${c.email})` : ''}</option>
              {/each}
            </select>
          </div>
        {/if}

        <div style="display:flex; flex-direction:column; gap:14px; margin-bottom:20px;">
          <div class="form-group">
            <label class="label" style="font-size:11px; margin-bottom:6px;" for="mate-name">Name <span style="color:var(--c-danger);">*</span></label>
            <input id="mate-name" class="input" style="padding:10px; font-size:13px;" bind:value={mateName} placeholder="e.g. Alice Cooper" />
          </div>
          <div class="form-group">
            <label class="label" style="font-size:11px; margin-bottom:6px;" for="mate-email">Corporate Email (links to Scrumy account)</label>
            <input id="mate-email" type="email" class="input" style="padding:10px; font-size:13px;" bind:value={mateEmail} placeholder="e.g. alice@company.com" />
          </div>
          <div class="form-group">
            <label class="label" style="font-size:11px; margin-bottom:6px;" for="mate-role">Role / Designation</label>
            <input id="mate-role" class="input" style="padding:10px; font-size:13px;" bind:value={mateRole} placeholder="e.g. Frontend Engineer, Scrum Master" />
          </div>
          <div class="form-group">
            <label class="label" style="font-size:11px; margin-bottom:6px;" for="mate-jira-id">Jira Account ID / Key (optional, synced from Jira)</label>
            <input id="mate-jira-id" class="input" style="padding:10px; font-size:13px; font-family:monospace;" bind:value={mateJiraID} placeholder="Auto-captured or optional" />
          </div>
        </div>

        {#if mateFormError}
          <div class="badge badge-danger" style="margin-bottom:16px; padding:10px; border-radius:6px; font-size:12px; text-transform:none; letter-spacing:0; text-align:center; display:block; font-weight:400;">
            ⚠️ {mateFormError}
          </div>
        {/if}

        <div style="display:flex; justify-content:flex-end; gap:12px; border-top:1px solid var(--c-border); padding-top:16px;">
          <button class="btn btn-secondary" onclick={() => showAddMateModal = false} disabled={savingMate}>Cancel</button>
          <button class="btn btn-primary" onclick={saveMate} disabled={savingMate}>
            {savingMate ? 'Saving...' : (editingMate ? 'Save Changes' : 'Add Team Mate')}
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
