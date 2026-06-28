<script lang="ts">
  import { page } from '$app/stores';
  import { onMount } from 'svelte';
  import { api, type CapacityPlan, type Plan } from '$lib/api';

  const planID = $derived($page.params.id as string);

  let plan = $state<Plan | null>(null);
  let capacityPlans = $state<CapacityPlan[]>([]);
  let loading = $state(true);
  let error = $state('');

  // Filters State
  let searchQuery = $state('');
  let statusFilter = $state('');

  async function loadData() {
    loading = true;
    error = '';
    try {
      const [p, c] = await Promise.all([
        api.plans.get(planID),
        api.capacity.list(planID)
      ]);
      plan = p;
      capacityPlans = c;
    } catch (e: any) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  onMount(loadData);

  // Derived filtered capacity plans
  const filteredPlans = $derived.by(() => {
    return capacityPlans.filter(cp => {
      if (searchQuery.trim()) {
        const query = searchQuery.trim().toLowerCase();
        if (!cp.name.toLowerCase().includes(query)) return false;
      }
      if (statusFilter) {
        if (cp.status !== statusFilter) return false;
      }
      return true;
    });
  });

  async function deleteCapacity(cpID: string) {
    if (!confirm('Are you sure you want to delete this capacity plan?')) return;
    try {
      await api.capacity.delete(planID, cpID);
      capacityPlans = capacityPlans.filter(cp => cp.id !== cpID);
    } catch (e: any) {
      alert(`Failed to delete capacity plan: ${e.message}`);
    }
  }

  function clearFilters() {
    searchQuery = '';
    statusFilter = '';
  }

  function statusBadge(s: string) {
    if (s === 'active') return 'badge-success';
    if (s === 'archived') return 'badge-default';
    return 'badge-warning';
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
  <title>Capacity Plans – Scrumy</title>
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
      <span class="text-muted">Loading capacity plans…</span>
    </div>
  {:else if error}
    <div class="badge badge-danger" style="padding:12px 20px; border-radius:8px; font-size:14px; text-transform:none; letter-spacing:0; font-weight:400;">
      ⚠️ {error}
    </div>
  {:else if plan}
    <!-- Title & Description -->
    <div style="margin-bottom: 24px; display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 16px;">
      <div>
        <h1 class="text-2xl font-bold">📊 Capacity Plans</h1>
        <p class="text-sm text-muted" style="margin-top: 4px;">
          Browse, filter, and manage all capacity plans created for <strong>{plan.name}</strong>.
        </p>
      </div>
      {#if plan.is_admin}
        <a href="/plans/{planID}/capacity/new" class="btn btn-primary btn-sm">
          + Create New Plan
        </a>
      {/if}
    </div>

    <!-- Filters Panel -->
    <div class="card" style="margin-bottom: 24px; padding: 20px;">
      <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 16px; align-items: flex-end;">
        <div class="form-group" style="margin-bottom: 0;">
          <label class="label" style="font-size: 11px; margin-bottom: 6px;">Search by Name</label>
          <input
            class="input"
            type="text"
            placeholder="e.g. Q3 Phoenix Sprint Plan"
            bind:value={searchQuery}
            style="padding: 8px; font-size: 13px;"
          />
        </div>

        <div class="form-group" style="margin-bottom: 0;">
          <label class="label" style="font-size: 11px; margin-bottom: 6px;">Filter by Status</label>
          <select class="input" bind:value={statusFilter} style="padding: 8px; font-size: 13px; height: 38px;">
            <option value="">All Statuses</option>
            <option value="active">Active</option>
            <option value="archived">Archived</option>
            <option value="draft">Draft</option>
          </select>
        </div>

        <div style="display: flex; gap: 8px;">
          {#if searchQuery || statusFilter}
            <button class="btn btn-secondary btn-sm" onclick={clearFilters} style="flex: 1; justify-content: center; height: 38px;">
              Clear Filters
            </button>
          {/if}
        </div>
      </div>
    </div>

    <!-- Capacity Plans List Section -->
    <div class="card" style="overflow: hidden;">
      <div class="card-header flex justify-between items-center" style="padding: 16px 20px;">
        <h2 class="font-semibold" style="font-size: 15px;">
          {#if filteredPlans.length === capacityPlans.length}
            All Capacity Plans ({capacityPlans.length})
          {:else}
            Filtered Capacity Plans ({filteredPlans.length} of {capacityPlans.length})
          {/if}
        </h2>
      </div>

      <div>
        {#if filteredPlans.length === 0}
          <div class="empty-state" style="padding: 40px; text-align: center;">
            <p style="font-size: 15px; color: var(--c-text-2);">No capacity plans match your filter criteria.</p>
            {#if searchQuery || statusFilter}
              <button class="btn btn-secondary btn-sm" onclick={clearFilters} style="margin-top: 12px;">
                Reset Filters
              </button>
            {/if}
          </div>
        {:else}
          {#each filteredPlans as cp (cp.id)}
            <div
              style="display:flex; justify-content:space-between; align-items:center; padding:16px 20px; border-bottom:1px solid var(--c-border); transition:background 150ms ease;"
              onmouseover={e => (e.currentTarget as HTMLElement).style.background = 'var(--c-bg)'}
              onmouseout={e => (e.currentTarget as HTMLElement).style.background = ''}
            >
              <div style="flex: 1; min-width: 0; margin-right: 16px;">
                <a
                  href="/plans/{planID}/capacity/{cp.id}"
                  style="display: inline-block; font-weight: 600; font-size: 15px; color: var(--c-primary); text-decoration: none; margin-bottom: 4px;"
                  class="hover-underline"
                >
                  {cp.name}
                </a>
                <div class="flex items-center gap-2 text-xs text-muted">
                  <span class="badge {statusBadge(cp.status)}" style="padding: 2px 8px; font-size: 10px;">{cp.status}</span>
                  <span>•</span>
                  <span>🕒 Updated {formatRelDate(cp.updated_at)}</span>
                </div>
              </div>

              <div class="flex gap-2">
                <a href="/plans/{planID}/capacity/{cp.id}" class="btn btn-secondary btn-sm">
                  View Details
                </a>
                {#if plan.is_admin}
                  <button
                    class="btn btn-danger btn-sm btn-icon"
                    style="color: var(--c-danger); width: 32px; height: 32px; padding: 0; display: inline-flex; align-items: center; justify-content: center;"
                    onclick={() => deleteCapacity(cp.id)}
                    title="Delete Capacity Plan"
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
