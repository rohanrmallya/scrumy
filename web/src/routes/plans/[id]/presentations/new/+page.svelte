<script lang="ts">
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { api, type IntroContent, type RetroContent, type PreviousData, type Epic, type Contributor } from '$lib/api';

  const planID = $derived($page.params.id);
  const type = $derived(($page.url.searchParams.get('type') ?? 'intro') as 'intro' | 'retro');

  let title = $state('');
  let templateID = $state('default');
  let sprintName = $state('');
  let saving = $state(false);
  let error = $state('');

  // Intro content
  interface Learning { title: string; content: string; tags: string[] }
  interface Change { title: string; content: string; tags: string[] }
  let learnings = $state<Learning[]>([{ title: '', content: '', tags: [] }, { title: '', content: '', tags: [] }]);
  let changes = $state<Change[]>([{ title: '', content: '', tags: [] }]);
  let prevData = $state<PreviousData>({
    total_sp_delivered: 0, total_hours_logged: 0, total_work_logs: 0,
    avg_hours_per_sp: 0, planned_sp: 0, executed_sp: 0, spillovers: 0, total_epics_delivered: 0
  });
  let epics = $state<Epic[]>([{
    id: '', title: '', summary: '', why_needed: '', when_doing: '', audience: '', total_sp: 0
  }]);

  // Shared state
  let contributors = $state<Contributor[]>([{ name: '', contribution: '' }]);

  // Retro content
  let retroFeedback = $state<string[]>(['', '']);

  function addLearning() { learnings = [...learnings, { title: '', content: '', tags: [] }]; }
  function removeLearning(i: number) { learnings = learnings.filter((_, idx) => idx !== i); }
  function addChange() { changes = [...changes, { title: '', content: '', tags: [] }]; }
  function removeChange(i: number) { changes = changes.filter((_, idx) => idx !== i); }
  function addEpic() {
    epics = [...epics, { id: '', title: '', summary: '', why_needed: '', when_doing: '', audience: '', total_sp: 0 }];
  }
  function removeEpic(i: number) { epics = epics.filter((_, idx) => idx !== i); }
  function addFeedback() { retroFeedback = [...retroFeedback, '']; }
  function removeFeedback(i: number) { retroFeedback = retroFeedback.filter((_, idx) => idx !== i); }
  function addContributor() { contributors = [...contributors, { name: '', contribution: '' }]; }
  function removeContributor(i: number) { contributors = contributors.filter((_, idx) => idx !== i); }

  async function saveDraft() {
    if (!title.trim()) { error = 'Title is required'; return; }
    saving = true;
    try {
      const pres = await api.presentations.create(planID, {
        type, 
        template_id: templateID,
        title: title.trim(), 
        sprint_name: sprintName.trim()
      });
      const content = type === 'intro'
        ? { 
            learnings: learnings.filter(l => l.content.trim() || l.title.trim()), 
            changes: changes.filter(c => c.content.trim() || c.title.trim()), 
            previous_data: prevData, 
            epics,
            contributors: contributors.filter(c => c.name.trim()),
            closing_text: ""
          } as IntroContent
        : { 
            previous_data: prevData, 
            feedback: retroFeedback.filter(Boolean),
            contributors: contributors.filter(c => c.name.trim()),
            closing_text: ""
          } as RetroContent;
      
      await api.presentations.update(planID, pres.id, { 
        title: title.trim(), 
        template_id: templateID,
        sprint_name: sprintName.trim(), 
        content 
      });
      goto(`/plans/${planID}/presentations/${pres.id}/edit`);
    } catch (e: any) {
      error = e.message;
    } finally { saving = false; }
  }
</script>

<svelte:head>
  <title>New {type === 'intro' ? 'Sprint Intro' : 'Sprint Retro'} – Scrumy</title>
</svelte:head>

<!-- Top bar -->
<div style="background:var(--c-surface);border-bottom:1px solid var(--c-border);padding:12px 24px;display:flex;align-items:center;justify-content:space-between;position:sticky;top:56px;z-index:99;">
  <div class="flex items-center gap-3">
    <a href="/plans/{planID}" class="btn-icon" style="font-size:18px;" title="Back">←</a>
    <div>
      <h1 class="font-semibold" style="font-size:16px;">{type === 'intro' ? 'Scrum Intro Form' : 'Sprint Retro Form'}</h1>
    </div>
  </div>
  <div class="flex gap-2">
    <a href="/plans/{planID}" class="btn btn-secondary">Cancel</a>
    <button class="btn btn-primary" onclick={saveDraft} disabled={saving}>
      {saving ? 'Saving…' : 'Save Presentation Data'}
    </button>
  </div>
</div>

{#if error}
  <div style="padding:8px 24px;background:var(--c-danger-lt);color:var(--c-danger);font-size:13px;">⚠️ {error}</div>
{/if}

<div style="max-width:900px;margin:0 auto;padding:24px 24px 80px;display:flex;flex-direction:column;gap:20px;">

  <!-- Header info -->
  <div>
    <h2 class="text-xl font-bold">{type === 'intro' ? 'New Sprint Intro' : 'New Sprint Retro'}</h2>
    <p class="text-sm text-muted" style="margin-top:4px;">Enter the raw data required for the presentation. Styling and slide layouts will be applied automatically.</p>
  </div>

  <!-- Title + Template + Sprint -->
  <div class="card">
    <div class="card-body grid-3">
      <div class="form-group">
        <label class="label" for="pres-title">Presentation Title</label>
        <input id="pres-title" class="input" bind:value={title} placeholder="e.g. Sprint 24 Intro" />
      </div>
      <div class="form-group">
        <label class="label" for="pres-template">Template</label>
        <select id="pres-template" class="input" bind:value={templateID}>
          <option value="default">Default (Dark/Modern)</option>
          <option value="minimalist">Minimalist (Light/Simple)</option>
          <option value="fire">Fire (Dark)</option>
        </select>
      </div>
      <div class="form-group">
        <label class="label" for="sprint-name">Sprint Name</label>
        <input id="sprint-name" class="input" bind:value={sprintName} placeholder="e.g. Sprint 24 – 25" />
      </div>
    </div>
  </div>

  <div class="card">
    <div class="card-header">
      <div class="flex items-center gap-2"><span style="font-size:18px;">👥</span><h2 class="font-semibold">Folks that contributed</h2></div>
      <button class="btn btn-ghost btn-sm" onclick={addContributor}>+ Add Person</button>
    </div>
    <div class="card-body" style="display:flex;flex-direction:column;gap:12px;">
      {#each contributors as c, i (i)}
        <div class="flex gap-2 items-start">
          <div class="grow grid-2" style="gap:10px;">
            <input class="input" bind:value={contributors[i].name} placeholder="Name" />
            <input class="input" bind:value={contributors[i].contribution} placeholder="Contribution (e.g. Developed API)" />
          </div>
          {#if contributors.length > 1}
            <button class="btn-icon" style="color:var(--c-danger);margin-top:8px;" onclick={() => removeContributor(i)}>✕</button>
          {/if}
        </div>
      {/each}
      {#if contributors.length === 0 || (contributors.length === 1 && !contributors[0].name)}
        <p class="text-muted" style="font-size:13px;text-align:center;padding:12px;">Slide will be skipped if no names are provided.</p>
      {/if}
    </div>
  </div>

  <!-- Data from previous sprint -->
  <div class="card">
    <div class="card-header">
      <div class="flex items-center gap-2">
        <span style="font-size:18px;">📊</span>
        <h2 class="font-semibold">Data from previous sprint</h2>
      </div>
    </div>
    <div class="card-body" style="display:flex;flex-direction:column;gap:16px;">
      <div class="grid-3">
        <div class="form-group">
          <label class="label">Total Story Points Delivered</label>
          <input type="number" class="input" bind:value={prevData.total_sp_delivered} min="0" />
        </div>
        {#if type === 'intro'}
          <div class="form-group">
            <label class="label">Total Epics Delivered</label>
            <input type="number" class="input" bind:value={prevData.total_epics_delivered} min="0" />
          </div>
        {/if}
        <div class="form-group">
          <label class="label">Total Work Logged (hrs)</label>
          <input type="number" class="input" bind:value={prevData.total_hours_logged} min="0" />
        </div>
      </div>
      <div class="grid-3">
        <div class="form-group">
          <label class="label">Hours per Story Point</label>
          <input type="number" class="input" bind:value={prevData.avg_hours_per_sp} min="0" step="0.1" />
        </div>
        <div class="form-group">
          <label class="label">Spillovers (if any)</label>
          <input type="number" class="input" bind:value={prevData.spillovers} min="0" />
        </div>
        <div class="form-group">
          <label class="label">Total Work Logs</label>
          <input type="number" class="input" bind:value={prevData.total_work_logs} min="0" />
        </div>
      </div>
      <div class="grid-2">
        <div class="form-group">
          <label class="label">Planned Capacity (pts)</label>
          <input type="number" class="input" bind:value={prevData.planned_sp} min="0" />
        </div>
        <div class="form-group">
          <label class="label">Executed Capacity (pts)</label>
          <input type="number" class="input" bind:value={prevData.executed_sp} min="0" />
        </div>
      </div>
    </div>
  </div>

  {#if type === 'intro'}
    <!-- Learnings + Changes -->
    <div style="display:grid;grid-template-columns:1fr 1fr;gap:16px;">
      <div class="card">
        <div class="card-header">
          <div class="flex items-center gap-2"><span>💡</span><h2 class="font-semibold">Learnings</h2></div>
          <button class="btn btn-ghost btn-sm" onclick={addLearning}>+ Add Learning</button>
        </div>
        <div class="card-body" style="display:flex;flex-direction:column;gap:16px;">
          {#each learnings as l, i (i)}
            <div style="border-left:3px solid var(--c-purple);padding-left:16px;display:flex;flex-direction:column;gap:10px;position:relative;">
              <div class="flex justify-between items-center">
                <input class="input grow" bind:value={learnings[i].title} placeholder="Heading (e.g. Deployment Velocity)" style="font-weight:600;" />
                {#if learnings.length > 1}
                  <button class="btn-icon" style="color:var(--c-danger);" onclick={() => removeLearning(i)}>✕</button>
                {/if}
              </div>
              <textarea class="input" bind:value={learnings[i].content} placeholder="What did we learn? (Content)" rows="2"></textarea>
            </div>
          {/each}
        </div>
      </div>

      <div class="card">
        <div class="card-header">
          <div class="flex items-center gap-2"><span>🔄</span><h2 class="font-semibold">Changes to this sprint</h2></div>
          <button class="btn btn-ghost btn-sm" onclick={addChange}>+ Add Change</button>
        </div>
        <div class="card-body" style="display:flex;flex-direction:column;gap:16px;">
          {#each changes as c, i (i)}
            <div style="border-left:3px solid var(--c-success);padding-left:16px;display:flex;flex-direction:column;gap:10px;position:relative;">
              <div class="flex justify-between items-center">
                <input class="input grow" bind:value={changes[i].title} placeholder="Heading (e.g. CI/CD Pipeline)" style="font-weight:600;" />
                {#if changes.length > 1}
                  <button class="btn-icon" style="color:var(--c-danger);" onclick={() => removeChange(i)}>✕</button>
                {/if}
              </div>
              <textarea class="input" bind:value={changes[i].content} placeholder="Describe the change…" rows="2"></textarea>
            </div>
          {/each}
        </div>
      </div>
    </div>

    <!-- Epics -->
    <div class="card">
      <div class="card-header">
        <div class="flex items-center gap-2">
          <span style="font-size:18px;">🗂</span>
          <h2 class="font-semibold">Current Epics</h2>
        </div>
        <span class="badge badge-default">{epics.length} {epics.length === 1 ? 'Entry' : 'Entries'}</span>
      </div>
      <div class="card-body" style="display:flex;flex-direction:column;gap:20px;">
        {#each epics as epic, i (i)}
          <div style="border-left:3px solid var(--c-primary);padding-left:16px;display:flex;flex-direction:column;gap:12px;">
            <div class="flex gap-3 justify-between items-center">
              <div class="flex gap-2 grow items-center">
                <div style="width:100px;">
                  <label class="label" style="font-size:10px;margin-bottom:0;">Epic ID</label>
                  <input class="input" bind:value={epics[i].id} placeholder="EPIC-123" style="font-size:13px;font-weight:600;padding:4px 8px;text-transform:uppercase;" />
                </div>
                <div class="grow">
                  <label class="label" style="font-size:10px;margin-bottom:0;">Epic Title</label>
                  <input class="input" bind:value={epics[i].title} placeholder="Epic Title" style="font-size:15px;font-weight:500;border:none;border-bottom:2px solid var(--c-border-2);border-radius:0;padding-left:0;background:transparent;" />
                </div>
              </div>
              {#if epics.length > 1}
                <button class="btn-icon" style="color:var(--c-danger);" onclick={() => removeEpic(i)}>🗑</button>
              {/if}
            </div>
            <div class="form-group">
              <label class="label">Summary</label>
              <textarea class="input" bind:value={epics[i].summary} placeholder="Brief overview of the Epic…" rows="2"></textarea>
            </div>
            <div class="grid-2">
              <div class="form-group">
                <label class="label">Why we need it?</label>
                <textarea class="input" bind:value={epics[i].why_needed} placeholder="Business value or problem solved" rows="2"></textarea>
              </div>
              <div class="form-group">
                <label class="label">Who is going to consume it?</label>
                <textarea class="input" bind:value={epics[i].audience} placeholder="Target audience / stakeholder" rows="2"></textarea>
              </div>
            </div>
            <div class="grid-2">
              <div class="form-group">
                <label class="label">When are we doing it?</label>
                <input class="input" bind:value={epics[i].when_doing} placeholder="e.g., Sprint 25 – 26" />
              </div>
              <div class="form-group">
                <label class="label">Total Story Points Planned</label>
                <input type="number" class="input" bind:value={epics[i].total_sp} min="0" />
              </div>
            </div>
          </div>
        {/each}
        <button
          style="width:100%;padding:12px;border:2px dashed var(--c-border-2);border-radius:var(--r);background:none;cursor:pointer;color:var(--c-primary);font-size:14px;font-family:inherit;transition:all 150ms ease;"
          onmouseover={e => (e.currentTarget as HTMLElement).style.borderColor='var(--c-primary)'}
          onmouseout={e => (e.currentTarget as HTMLElement).style.borderColor='var(--c-border-2)'}
          onclick={addEpic}
        >+ Add Another Epic</button>
      </div>
    </div>
  {:else}
    <!-- Retro: Feedback & Learnings -->
    <div class="card">
      <div class="card-header">
        <div class="flex items-center gap-2"><span>💬</span><h2 class="font-semibold">Feedback & Learnings</h2></div>
        <button class="btn btn-ghost btn-sm" onclick={addFeedback}>+ Add Item</button>
      </div>
      <div class="card-body" style="display:flex;flex-direction:column;gap:8px;">
        {#each retroFeedback as f, i (i)}
          <div class="flex gap-2 items-start">
            <span class="text-xs font-semibold text-muted" style="width:18px;text-align:right;padding-top:9px;">{i+1}</span>
            <textarea class="input grow" bind:value={retroFeedback[i]} placeholder="What went well, what can be improved, action items…" rows="2"></textarea>
            {#if retroFeedback.length > 1}
              <button class="btn-icon" style="color:var(--c-danger);margin-top:4px;" onclick={() => removeFeedback(i)}>✕</button>
            {/if}
          </div>
        {/each}
      </div>
    </div>
  {/if}

</div>
