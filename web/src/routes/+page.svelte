<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { api, type Team } from '$lib/api';
  import { user } from '$lib/auth';

  let teams = $state<Team[]>([]);
  let loading = $state(true);
  let creating = $state(false);
  let newTeamName = $state('');
  let showCreate = $state(false);
  let error = $state('');

  onMount(async () => {
    try {
      teams = await api.teams.list();
    } catch (e: any) {
      error = e.message;
    } finally {
      loading = false;
    }
  });

  async function createTeam() {
    if (!newTeamName.trim()) return;
    creating = true;
    try {
      const team = await api.teams.create(newTeamName.trim());
      goto(`/teams/${team.id}`);
    } catch (e: any) {
      error = e.message;
      creating = false;
    }
  }

  function onKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter') createTeam();
    if (e.key === 'Escape') { showCreate = false; newTeamName = ''; }
  }

  function formatDate(iso: string) {
    try {
      return new Date(iso).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' });
    } catch { return ''; }
  }

  const icons = ['📊', '🚀', '🎯', '⚡', '💡', '🔮', '🛠️', '📈'];
  function teamIcon(id: string) {
    const n = id.charCodeAt(0) % icons.length;
    return icons[n];
  }
</script>

<svelte:head>
  <title>Scrumy – Your Agile Teams</title>
</svelte:head>

<div class="page-content fade-in">
  <div style="text-align:center; margin-bottom:40px;">
    <h1 class="text-2xl font-bold" style="margin-bottom:8px;">Agile Teams</h1>
    <p class="text-muted">Browse public teams or sign in to create and manage your own.</p>
  </div>

  {#if error}
    <div class="badge badge-danger" style="margin-bottom:16px; padding:8px 16px; border-radius:8px; font-size:13px; text-transform:none; letter-spacing:0; font-weight:400;">
      ⚠️ {error}
    </div>
  {/if}

  <div style="display:grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap:16px;">
    <!-- Create New Card (only if logged in) -->
    {#if $user}
      <div
        class="card"
        style="border-style: dashed; border-width:2px; cursor:pointer; transition:all 200ms ease; min-height:160px; display:flex; align-items:center; justify-content:center; flex-direction:column; gap:12px;"
        onclick={() => showCreate = true}
        role="button" tabindex="0"
        onkeydown={(e) => e.key === 'Enter' && (showCreate = true)}
      >
        {#if showCreate}
          <div style="padding:20px; width:100%; display:flex; flex-direction:column; gap:10px;"
               onclick={(e) => e.stopPropagation()} role="none">
            <span class="font-semibold" style="font-size:14px;">New Team Name</span>
            <input
              class="input"
              bind:value={newTeamName}
              placeholder="e.g. Q3 Phoenix Release"
              onkeydown={onKeydown}
              autofocus
            />
            <div class="flex gap-2" style="justify-content:flex-end;">
              <button class="btn btn-secondary btn-sm" onclick={() => { showCreate = false; newTeamName = ''; }}>Cancel</button>
              <button class="btn btn-primary btn-sm" onclick={createTeam} disabled={creating || !newTeamName.trim()}>
                {creating ? 'Creating…' : 'Create Team'}
              </button>
            </div>
          </div>
        {:else}
          <div style="width:48px;height:48px;background:var(--c-primary-lt);border-radius:12px;display:flex;align-items:center;justify-content:center;font-size:22px;color:var(--c-primary);">+</div>
          <div style="text-align:center;">
            <p class="font-semibold">Create New Team</p>
            <p class="text-sm text-muted" style="margin-top:4px;">Start a new agile team</p>
          </div>
        {/if}
      </div>
    {/if}

    {#if loading}
      {#each [1,2,3] as _}
        <div class="card" style="min-height:160px; animation: pulse 1.5s ease infinite;">
          <div class="card-body" style="display:flex;flex-direction:column;gap:12px;">
            <div style="width:40px;height:40px;background:var(--c-surface-2);border-radius:10px;"></div>
            <div style="width:60%;height:16px;background:var(--c-surface-2);border-radius:4px;"></div>
            <div style="width:40%;height:12px;background:var(--c-surface-2);border-radius:4px;"></div>
          </div>
        </div>
      {/each}
    {:else}
      {#each teams as team (team.id)}
        <a
          href="/teams/{team.id}"
          class="card"
          style="display:block; text-decoration:none; cursor:pointer; transition:all 200ms ease; min-height:160px; position:relative;"
          onmouseover={e => (e.currentTarget as HTMLElement).style.transform = 'translateY(-2px)'}
          onmouseout={e => (e.currentTarget as HTMLElement).style.transform = ''}
          onfocus={e => (e.currentTarget as HTMLElement).style.transform = 'translateY(-2px)'}
          onblur={e => (e.currentTarget as HTMLElement).style.transform = ''}
        >
          {#if team.is_admin}
            <div class="admin-badge">Owner</div>
          {/if}
          <div class="card-body" style="display:flex;flex-direction:column;gap:12px;height:100%;">
            <div style="width:42px;height:42px;background:var(--c-surface-2);border-radius:10px;display:flex;align-items:center;justify-content:center;font-size:20px;">
              {teamIcon(team.id)}
            </div>
            <div style="flex:1;">
              <p class="font-semibold" style="font-size:16px; margin-bottom:4px;">{team.name}</p>
              <p class="text-sm text-muted">
                {team.capacity_plan_count} capacity plan{team.capacity_plan_count !== 1 ? 's' : ''} &bull; {team.presentation_count} presentation{team.presentation_count !== 1 ? 's' : ''}
              </p>
            </div>
            <p class="text-xs text-muted">Updated {formatDate(team.updated_at)}</p>
          </div>
        </a>
      {/each}
    {/if}
  </div>
</div>

<style>
  @keyframes pulse {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.5; }
  }
  .admin-badge {
    position: absolute;
    top: 12px;
    right: 12px;
    font-size: 10px;
    text-transform: uppercase;
    font-weight: bold;
    background: var(--c-primary-lt);
    color: var(--c-primary);
    padding: 2px 6px;
    border-radius: 4px;
    letter-spacing: 0.05em;
  }
</style>
