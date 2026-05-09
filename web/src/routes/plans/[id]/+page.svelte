<script lang="ts">
  import { page } from '$app/stores';
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { api, type Plan, type CapacityPlan, type Presentation } from '$lib/api';

  const planID = $derived($page.params.id);

  let plan = $state<Plan | null>(null);
  let capacityPlans = $state<CapacityPlan[]>([]);
  let presentations = $state<Presentation[]>([]);
  let loading = $state(true);
  let error = $state('');

  onMount(async () => {
    try {
      [plan, capacityPlans, presentations] = await Promise.all([
        api.plans.get(planID),
        api.capacity.list(planID),
        api.presentations.list(planID),
      ]);
    } catch (e: any) {
      error = e.message;
    } finally {
      loading = false;
    }
  });

  const intros = $derived(presentations.filter(p => p.type === 'intro'));
  const retros = $derived(presentations.filter(p => p.type === 'retro'));

  function statusBadge(s: string) {
    if (s === 'active') return 'badge-success';
    if (s === 'archived') return 'badge-default';
    return 'badge-warning';
  }

  function presBadge(s: string) {
    if (s === 'published') return 'badge-success';
    return 'badge-warning';
  }

  async function deleteCapacity(cpID: string) {
    if (!confirm('Delete this capacity plan?')) return;
    await api.capacity.delete(planID, cpID);
    capacityPlans = await api.capacity.list(planID);
    plan = await api.plans.get(planID);
  }

  async function deletePres(presID: string) {
    if (!confirm('Delete this presentation?')) return;
    await api.presentations.delete(planID, presID);
    presentations = await api.presentations.list(planID);
    plan = await api.plans.get(planID);
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
  <title>{plan?.name ?? 'Plan'} – Scrumy</title>
</svelte:head>

<div class="page-content fade-in">
  {#if loading}
    <div class="flex items-center gap-3" style="padding:40px 0">
      <span class="spinner"></span>
      <span class="text-muted">Loading plan…</span>
    </div>
  {:else if error}
    <div class="badge badge-danger" style="padding:12px 20px; border-radius:8px; font-size:14px; text-transform:none; letter-spacing:0; font-weight:400;">
      ⚠️ {error}
    </div>
  {:else}
    <!-- Header -->
    <div class="flex items-center justify-between" style="margin-bottom:28px;">
      <div>
        <h1 class="text-2xl font-bold">{plan?.name}</h1>
        <p class="text-sm text-muted" style="margin-top:2px;">Plan Workspace</p>
      </div>
    </div>

    <div style="display:grid; grid-template-columns: 1fr 1fr; gap:20px; align-items:start;">
      <!-- Capacity Plans -->
      <div class="card">
        <div class="card-header">
          <div class="flex items-center gap-2">
            <span style="font-size:18px;">📊</span>
            <h2 class="font-semibold" style="font-size:16px;">Capacity Plans</h2>
          </div>
          <a href="/plans/{planID}/capacity/new" class="btn btn-primary btn-sm">+ Create New</a>
        </div>

        <div>
          {#if capacityPlans.length === 0}
            <div class="empty-state">
              <p>No capacity plans yet.</p>
              <a href="/plans/{planID}/capacity/new" class="btn btn-primary btn-sm" style="margin-top:12px;">Create First Plan</a>
            </div>
          {:else}
            {#each capacityPlans as cp (cp.id)}
              <a
                href="/plans/{planID}/capacity/{cp.id}"
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
                    <button
                      class="btn-icon"
                      style="color:var(--c-danger); font-size:14px;"
                      onclick={(e) => { e.stopPropagation(); deleteCapacity(cp.id); }}
                      title="Delete"
                    >✕</button>
                  </div>
                </div>
                <p class="text-xs text-muted">Updated {formatRelDate(cp.updated_at)}</p>
              </a>
            {/each}
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
          <div class="flex gap-2">
            <a href="/plans/{planID}/presentations/new?type=intro" class="btn btn-secondary btn-sm">+ Intro</a>
            <a href="/plans/{planID}/presentations/new?type=retro" class="btn btn-secondary btn-sm">+ Retro</a>
          </div>
        </div>

        <div class="card-body" style="display:flex;flex-direction:column;gap:20px;padding-top:16px;">
          <!-- Sprint Intros -->
          <div>
            <p class="text-xs font-semibold text-muted" style="text-transform:uppercase;letter-spacing:0.06em;margin-bottom:10px;">Sprint Intros</p>
            {#if intros.length === 0}
              <p class="text-sm text-muted">No sprint intros yet.</p>
            {:else}
              <div style="display:flex;flex-direction:column;gap:8px;">
                {#each intros as pres (pres.id)}
                  <div class="flex items-center justify-between" style="padding:10px 12px;background:var(--c-bg);border-radius:8px;">
                    <div class="flex items-center gap-3">
                      {#if pres.status === 'published'}
                        <a href="/plans/{planID}/presentations/{pres.id}/view"
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
                      <a href="/plans/{planID}/presentations/{pres.id}/edit" class="btn-icon" title="Edit" style="font-size:14px;">✎</a>
                      <button class="btn-icon" style="color:var(--c-danger);font-size:14px;" onclick={() => deletePres(pres.id)} title="Delete">✕</button>
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
                {#each retros as pres (pres.id)}
                  <div class="flex items-center justify-between" style="padding:10px 12px;background:var(--c-bg);border-radius:8px;">
                    <div class="flex items-center gap-3">
                      {#if pres.status === 'published'}
                        <a href="/plans/{planID}/presentations/{pres.id}/view"
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
                      <a href="/plans/{planID}/presentations/{pres.id}/edit" class="btn-icon" title="Edit" style="font-size:14px;">✎</a>
                      <button class="btn-icon" style="color:var(--c-danger);font-size:14px;" onclick={() => deletePres(pres.id)} title="Delete">✕</button>
                    </div>
                  </div>
                {/each}
              </div>
            {/if}
          </div>
        </div>
      </div>
    </div>
  {/if}
</div>
