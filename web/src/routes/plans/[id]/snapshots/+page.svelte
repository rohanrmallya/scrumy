<script lang="ts">
  import { page } from '$app/stores';
  import { onMount } from 'svelte';
  import { api, type JiraSnapshot, type Plan } from '$lib/api';

  const planID = $derived($page.params.id as string);

  let plan = $state<Plan | null>(null);
  let snapshots = $state<JiraSnapshot[]>([]);
  let loading = $state(true);
  let error = $state('');

  // Filters State
  let searchQuery = $state('');
  let startDateFilter = $state('');
  let endDateFilter = $state('');

  async function loadData() {
    loading = true;
    error = '';
    try {
      const [p, sn] = await Promise.all([
        api.plans.get(planID),
        api.jira.listSnapshots(planID)
      ]);
      plan = p;
      snapshots = sn;
    } catch (e: any) {
      error = e.message;
    } finally {
      loading = false;
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

  async function deleteSnapshot(id: string) {
    if (!confirm('Are you sure you want to delete this snapshot?')) return;
    try {
      await api.jira.deleteSnapshot(planID, id);
      snapshots = snapshots.filter(s => s.id !== id);
    } catch (e: any) {
      alert(`Failed to delete snapshot: ${e.message}`);
    }
  }

  function clearFilters() {
    searchQuery = '';
    startDateFilter = '';
    endDateFilter = '';
  }

  function formatDate(iso: string) {
    try {
      if (!iso) return '';
      const datePart = iso.includes('T') ? iso.split('T')[0] : iso;
      const parts = datePart.split('-');
      if (parts.length !== 3) return iso;
      const [year, month, day] = parts;
      const date = new Date(Number(year), Number(month) - 1, Number(day));
      return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' });
    } catch { return iso; }
  }
</script>

<svelte:head>
  <title>Jira Snapshots – Scrumy</title>
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
      <span class="text-muted">Loading snapshots…</span>
    </div>
  {:else if error}
    <div class="badge badge-danger" style="padding:12px 20px; border-radius:8px; font-size:14px; text-transform:none; letter-spacing:0; font-weight:400;">
      ⚠️ {error}
    </div>
  {:else if plan}
    <!-- Title & Description -->
    <div style="margin-bottom: 24px;">
      <h1 class="text-2xl font-bold">📸 Jira Snapshots</h1>
      <p class="text-sm text-muted" style="margin-top: 4px;">
        Browse, filter, and manage all Jira snapshots taken for <strong>{plan.name}</strong>.
      </p>
    </div>

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
                <a
                  href="/plans/{planID}/snapshots/{s.id}"
                  style="display: inline-block; font-weight: 600; font-size: 15px; color: var(--c-primary); text-decoration: none; margin-bottom: 4px;"
                  class="hover-underline"
                >
                  {s.name}
                </a>
                <div class="flex items-center gap-2 text-xs text-muted" style="flex-wrap: wrap;">
                  <span>📅 Range: <strong>{formatDate(s.start_date)}</strong> to <strong>{formatDate(s.end_date)}</strong></span>
                  <span>•</span>
                  <span>⚙️ Mode: <strong>{s.all_worklogs ? 'All worklogs' : 'Filtered worklogs'}</strong></span>
                  <span>•</span>
                  <span>🕒 Created: {formatDate(s.created_at?.toString())}</span>
                </div>
              </div>

              <div class="flex gap-2">
                <a href="/plans/{planID}/snapshots/{s.id}" class="btn btn-secondary btn-sm">
                  View Details
                </a>
                {#if plan.is_admin}
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
