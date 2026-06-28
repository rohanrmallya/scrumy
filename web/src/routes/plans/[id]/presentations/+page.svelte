<script lang="ts">
  import { page } from '$app/stores';
  import { onMount } from 'svelte';
  import { api, type Presentation, type Plan } from '$lib/api';

  const planID = $derived($page.params.id as string);

  let plan = $state<Plan | null>(null);
  let presentations = $state<Presentation[]>([]);
  let loading = $state(true);
  let error = $state('');

  // Filters State
  let searchQuery = $state('');
  let typeFilter = $state('');
  let statusFilter = $state('');

  async function loadData() {
    loading = true;
    error = '';
    try {
      const [p, pr] = await Promise.all([
        api.plans.get(planID),
        api.presentations.list(planID)
      ]);
      plan = p;
      presentations = pr;
    } catch (e: any) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  onMount(loadData);

  // Derived filtered presentations list
  const filteredPresentations = $derived.by(() => {
    return presentations.filter(pres => {
      if (searchQuery.trim()) {
        const query = searchQuery.trim().toLowerCase();
        const titleMatch = pres.title?.toLowerCase().includes(query);
        const sprintMatch = pres.sprint_name?.toLowerCase().includes(query);
        if (!titleMatch && !sprintMatch) return false;
      }
      if (typeFilter) {
        if (pres.type !== typeFilter) return false;
      }
      if (statusFilter) {
        if (pres.status !== statusFilter) return false;
      }
      return true;
    });
  });

  async function deletePres(presID: string) {
    if (!confirm('Are you sure you want to delete this presentation?')) return;
    try {
      await api.presentations.delete(planID, presID);
      presentations = presentations.filter(p => p.id !== presID);
    } catch (e: any) {
      alert(`Failed to delete presentation: ${e.message}`);
    }
  }

  function clearFilters() {
    searchQuery = '';
    typeFilter = '';
    statusFilter = '';
  }

  function formatRelDate(iso: string) {
    try {
      if (!iso) return '';
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
  <title>Presentations – Scrumy</title>
</svelte:head>

<div class="page-content fade-in">
  <!-- Back Link -->
  <div style="margin-bottom: 20px;">
    <a href="/plans/{planID}" class="btn btn-secondary btn-sm" style="display:inline-flex; align-items:center; gap:6px;">
      ← Back to Plan Workspace
    </a>
  </div>

  {#if loading}
    <div class="flex items-center gap-3" style="padding:40px 0">
      <span class="spinner"></span>
      <span class="text-muted">Loading presentations…</span>
    </div>
  {:else if error}
    <div class="badge badge-danger" style="padding:12px 20px; border-radius:8px; font-size:14px; text-transform:none; letter-spacing:0; font-weight:400;">
      ⚠️ {error}
    </div>
  {:else if plan}
    <!-- Title & Description -->
    <div style="margin-bottom: 24px; display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 16px;">
      <div>
        <h1 class="text-2xl font-bold">🎤 Presentations</h1>
        <p class="text-sm text-muted" style="margin-top: 4px;">
          Browse, filter, and view presentations created for <strong>{plan.name}</strong>.
        </p>
      </div>
      {#if plan.is_admin}
        <div class="flex gap-2">
          <a href="/plans/{planID}/presentations/new?type=intro" class="btn btn-secondary btn-sm">+ Intro</a>
          <a href="/plans/{planID}/presentations/new?type=retro" class="btn btn-secondary btn-sm">+ Retro</a>
        </div>
      {/if}
    </div>

    <!-- Filters Panel -->
    <div class="card" style="margin-bottom: 24px; padding: 20px;">
      <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 16px; align-items: flex-end;">
        <div class="form-group" style="margin-bottom: 0;">
          <label class="label" style="font-size: 11px; margin-bottom: 6px;">Search Title / Sprint</label>
          <input
            class="input"
            type="text"
            placeholder="e.g. Sprint 24"
            bind:value={searchQuery}
            style="padding: 8px; font-size: 13px;"
          />
        </div>

        <div class="form-group" style="margin-bottom: 0;">
          <label class="label" style="font-size: 11px; margin-bottom: 6px;">Filter by Type</label>
          <select class="input" bind:value={typeFilter} style="padding: 8px; font-size: 13px; height: 38px;">
            <option value="">All Types</option>
            <option value="intro">Sprint Intro</option>
            <option value="retro">Sprint Retro</option>
          </select>
        </div>

        <div class="form-group" style="margin-bottom: 0;">
          <label class="label" style="font-size: 11px; margin-bottom: 6px;">Filter by Status</label>
          <select class="input" bind:value={statusFilter} style="padding: 8px; font-size: 13px; height: 38px;">
            <option value="">All Statuses</option>
            <option value="draft">Draft</option>
            <option value="published">Published</option>
          </select>
        </div>

        <div style="display: flex; gap: 8px;">
          {#if searchQuery || typeFilter || statusFilter}
            <button class="btn btn-secondary btn-sm" onclick={clearFilters} style="flex: 1; justify-content: center; height: 38px;">
              Clear Filters
            </button>
          {/if}
        </div>
      </div>
    </div>

    <!-- Presentations List Section -->
    <div class="card" style="overflow: hidden;">
      <div class="card-header flex justify-between items-center" style="padding: 16px 20px;">
        <h2 class="font-semibold" style="font-size: 15px;">
          {#if filteredPresentations.length === presentations.length}
            All Presentations ({presentations.length})
          {:else}
            Filtered Presentations ({filteredPresentations.length} of {presentations.length})
          {/if}
        </h2>
      </div>

      <div>
        {#if filteredPresentations.length === 0}
          <div class="empty-state" style="padding: 40px; text-align: center;">
            <p style="font-size: 15px; color: var(--c-text-2);">No presentations match your filter criteria.</p>
            {#if searchQuery || typeFilter || statusFilter}
              <button class="btn btn-secondary btn-sm" onclick={clearFilters} style="margin-top: 12px;">
                Reset Filters
              </button>
            {/if}
          </div>
        {:else}
          {#each filteredPresentations as pres (pres.id)}
            <div
              style="display:flex; justify-content:space-between; align-items:center; padding:16px 20px; border-bottom:1px solid var(--c-border); transition:background 150ms ease;"
              onmouseover={e => (e.currentTarget as HTMLElement).style.background = 'var(--c-bg)'}
              onmouseout={e => (e.currentTarget as HTMLElement).style.background = ''}
            >
              <div style="display:flex; align-items:center; gap:14px; flex: 1; min-width: 0; margin-right: 16px;">
                <!-- Type icon and play trigger -->
                {#if pres.status === 'published'}
                  <a
                    href="/plans/{planID}/presentations/{pres.id}/view"
                    style="width:36px; height:36px; border-radius:8px; display:flex; align-items:center; justify-content:center; font-size:16px; font-weight:bold; transition:transform 100ms ease;"
                    class="play-btn {pres.type === 'intro' ? 'intro-play' : 'retro-play'}"
                    title="Play Presentation"
                  >
                    ▶
                  </a>
                {:else}
                  <div
                    style="width:36px; height:36px; background:var(--c-surface-2); border-radius:8px; display:flex; align-items:center; justify-content:center; color:var(--c-text-3); font-size:14px;"
                    title="Draft"
                  >
                    ✎
                  </div>
                {/if}

                <div style="flex:1; min-width: 0;">
                  <span style="font-weight: 600; font-size: 15px; display: block; margin-bottom: 2px;">
                    {pres.title}
                  </span>
                  <div class="flex items-center gap-2 text-xs text-muted" style="flex-wrap: wrap;">
                    <span class="badge badge-type {pres.type === 'intro' ? 'badge-intro' : 'badge-retro'}">
                      {pres.type === 'intro' ? 'Sprint Intro' : 'Sprint Retro'}
                    </span>
                    <span>•</span>
                    <span style="text-transform: capitalize;">{pres.status}</span>
                    <span>•</span>
                    {#if pres.sprint_name}
                      <span>Sprint: <strong>{pres.sprint_name}</strong></span>
                      <span>•</span>
                    {/if}
                    <span>🕒 Updated {formatRelDate(pres.updated_at)}</span>
                  </div>
                </div>
              </div>

              <div class="flex gap-2">
                {#if pres.status === 'published'}
                  <a href="/plans/{planID}/presentations/{pres.id}/view" class="btn btn-secondary btn-sm">
                    View
                  </a>
                {/if}
                {#if plan.is_admin}
                  <a href="/plans/{planID}/presentations/{pres.id}/edit" class="btn btn-secondary btn-sm">
                    Edit
                  </a>
                  <button
                    class="btn btn-danger btn-sm btn-icon"
                    style="color: var(--c-danger); width: 32px; height: 32px; padding: 0; display: inline-flex; align-items: center; justify-content: center;"
                    onclick={() => deletePres(pres.id)}
                    title="Delete Presentation"
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
  {/if}
</div>

<style>
  .play-btn:hover {
    transform: scale(1.05);
  }
  .intro-play {
    background: var(--c-primary-lt);
    color: var(--c-primary);
  }
  .retro-play {
    background: var(--c-purple-lt);
    color: var(--c-purple);
  }
  .badge-type {
    font-size: 10px;
    padding: 1px 6px;
    border-radius: 4px;
    font-weight: 500;
  }
  .badge-intro {
    background: var(--c-primary-lt);
    color: var(--c-primary);
  }
  .badge-retro {
    background: var(--c-purple-lt);
    color: var(--c-purple);
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
