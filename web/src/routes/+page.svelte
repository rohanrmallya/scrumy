<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { api, type Plan } from '$lib/api';

  let plans = $state<Plan[]>([]);
  let loading = $state(true);
  let creating = $state(false);
  let newPlanName = $state('');
  let showCreate = $state(false);
  let error = $state('');

  onMount(async () => {
    try {
      plans = await api.plans.list();
    } catch (e: any) {
      error = e.message;
    } finally {
      loading = false;
    }
  });

  async function createPlan() {
    if (!newPlanName.trim()) return;
    creating = true;
    try {
      const plan = await api.plans.create(newPlanName.trim());
      goto(`/plans/${plan.id}`);
    } catch (e: any) {
      error = e.message;
      creating = false;
    }
  }

  function onKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter') createPlan();
    if (e.key === 'Escape') { showCreate = false; newPlanName = ''; }
  }

  function formatDate(iso: string) {
    try {
      return new Date(iso).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' });
    } catch { return ''; }
  }

  const icons = ['📊', '🚀', '🎯', '⚡', '💡', '🔮', '🛠️', '📈'];
  function planIcon(id: string) {
    const n = id.charCodeAt(0) % icons.length;
    return icons[n];
  }
</script>

<svelte:head>
  <title>Scrumy – Your Agile Plans</title>
</svelte:head>

<div class="page-content fade-in">
  <div style="text-align:center; margin-bottom:40px;">
    <h1 class="text-2xl font-bold" style="margin-bottom:8px;">Your Agile Plans</h1>
    <p class="text-muted">Select an existing plan or start a new one to begin planning your capacity.</p>
  </div>

  {#if error}
    <div class="badge badge-danger" style="margin-bottom:16px; padding:8px 16px; border-radius:8px; font-size:13px; text-transform:none; letter-spacing:0; font-weight:400;">
      ⚠️ {error}
    </div>
  {/if}

  <div style="display:grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap:16px;">
    <!-- Create New Card -->
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
          <span class="font-semibold" style="font-size:14px;">New Plan Name</span>
          <input
            class="input"
            bind:value={newPlanName}
            placeholder="e.g. Q3 Phoenix Release"
            onkeydown={onKeydown}
            autofocus
          />
          <div class="flex gap-2" style="justify-content:flex-end;">
            <button class="btn btn-secondary btn-sm" onclick={() => { showCreate = false; newPlanName = ''; }}>Cancel</button>
            <button class="btn btn-primary btn-sm" onclick={createPlan} disabled={creating || !newPlanName.trim()}>
              {creating ? 'Creating…' : 'Create Plan'}
            </button>
          </div>
        </div>
      {:else}
        <div style="width:48px;height:48px;background:var(--c-primary-lt);border-radius:12px;display:flex;align-items:center;justify-content:center;font-size:22px;color:var(--c-primary);">+</div>
        <div style="text-align:center;">
          <p class="font-semibold">Create New Plan</p>
          <p class="text-sm text-muted" style="margin-top:4px;">Start a new agile plan</p>
        </div>
      {/if}
    </div>

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
      {#each plans as plan (plan.id)}
        <a
          href="/plans/{plan.id}"
          class="card"
          style="display:block; text-decoration:none; cursor:pointer; transition:all 200ms ease; min-height:160px;"
          onmouseover={e => (e.currentTarget as HTMLElement).style.transform = 'translateY(-2px)'}
          onmouseout={e => (e.currentTarget as HTMLElement).style.transform = ''}
          onfocus={e => (e.currentTarget as HTMLElement).style.transform = 'translateY(-2px)'}
          onblur={e => (e.currentTarget as HTMLElement).style.transform = ''}
        >
          <div class="card-body" style="display:flex;flex-direction:column;gap:12px;height:100%;">
            <div style="width:42px;height:42px;background:var(--c-surface-2);border-radius:10px;display:flex;align-items:center;justify-content:center;font-size:20px;">
              {planIcon(plan.id)}
            </div>
            <div style="flex:1;">
              <p class="font-semibold" style="font-size:16px; margin-bottom:4px;">{plan.name}</p>
              <p class="text-sm text-muted">
                {plan.capacity_plan_count} capacity plan{plan.capacity_plan_count !== 1 ? 's' : ''} &bull; {plan.presentation_count} presentation{plan.presentation_count !== 1 ? 's' : ''}
              </p>
            </div>
            <p class="text-xs text-muted">Updated {formatDate(plan.updated_at)}</p>
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
</style>
