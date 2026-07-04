<script lang="ts">
  import { page } from '$app/stores';
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { api, type Presentation, type IntroContent, type RetroContent, type PreviousData, type Epic, type Learning, type Change, type Contributor, type JiraSnapshot } from '$lib/api';

  const planID = $derived($page.params.id);
  const presID = $derived($page.params.presid);

  let pres = $state<Presentation | null>(null);
  let loading = $state(true);
  let saving = $state(false);
  let publishing = $state(false);
  let error = $state('');

  // Intro state
  let learnings = $state<Learning[]>([{ title: '', content: '', tags: [] }]);
  let changes = $state<Change[]>([{ title: '', content: '', tags: [] }]);
  let prevData = $state<PreviousData>({ total_sp_delivered:0, total_hours_logged:0, total_work_logs:0, avg_hours_per_sp:0, planned_sp:0, executed_sp:0, spillovers:0, total_epics_delivered:0 });
  let epics = $state<Epic[]>([{ id:'', title:'', summary:'', why_needed:'', when_doing:'', audience:'', total_sp:0 }]);

  // Shared state
  let contributors = $state<Contributor[]>([{ name: '', contribution: '' }]);
  let closingText = $state('');

  // Retro state
  let retroFeedback = $state<string[]>(['']);

  let title = $state('');
  let templateID = $state('default');
  let sprintName = $state('');

  // Jira Snapshot import state
  let snapshots = $state<JiraSnapshot[]>([]);
  let selectedSnapshotID = $state('');

  onMount(async () => {
    try {
      // Load both presentation details and plan snapshots
      const [pRes, sRes] = await Promise.all([
        api.presentations.get(planID, presID),
        api.jira.listSnapshots(planID)
      ]);
      
      pres = pRes;
      snapshots = sRes;

      title = pres.title;
      templateID = pres.template_id || 'default';
      sprintName = pres.sprint_name;
      if (pres.type === 'intro') {
        const c = pres.content as IntroContent;
        if (c) {
          // Migrate old string data to new Learning/Change objects
          learnings = (c.learnings ?? []).map((l: any) => typeof l === 'string' ? { title: '', content: l, tags: [] } : l);
          if (learnings.length === 0) learnings = [{ title: '', content: '', tags: [] }];
          
          changes = (c.changes ?? []).map((ch: any) => typeof ch === 'string' ? { title: '', content: ch, tags: [] } : ch);
          if (changes.length === 0) changes = [{ title: '', content: '', tags: [] }];
          
          prevData = c.previous_data ?? prevData;
          epics = c.epics?.length ? c.epics : [{ id:'', title:'', summary:'', why_needed:'', when_doing:'', audience:'', total_sp:0 }];
          contributors = c.contributors?.length ? c.contributors : [{ name: '', contribution: '' }];
          closingText = c.closing_text ?? '';
        }
      } else {
        const c = pres.content as RetroContent;
        if (c) {
          retroFeedback = c.feedback?.length ? c.feedback : [''];
          prevData = c.previous_data ?? prevData;
          contributors = c.contributors?.length ? c.contributors : [{ name: '', contribution: '' }];
          closingText = c.closing_text ?? '';
        }
      }
    } catch (e: any) { error = e.message; }
    finally { loading = false; }
  });

  async function importSnapshot() {
    if (!selectedSnapshotID) return;
    try {
      const snap = await api.jira.getSnapshot(planID, selectedSnapshotID);
      prevData = {
        total_sp_delivered: snap.data.totals.total_story_points,
        total_hours_logged: snap.data.totals.total_hours_logged,
        total_work_logs: snap.data.totals.total_work_logs,
        avg_hours_per_sp: snap.data.totals.avg_hours_per_sp,
        planned_sp: prevData.planned_sp,
        executed_sp: snap.data.totals.total_story_points,
        spillovers: prevData.spillovers,
        total_epics_delivered: prevData.total_epics_delivered,
      };

      if (snap.data.leaderboard && snap.data.leaderboard.length > 0) {
        if (confirm("Would you like to import authors from the Jira leaderboard as contributors?")) {
          contributors = snap.data.leaderboard.map(entry => ({
            name: entry.author_name,
            contribution: `Logged ${entry.hours_logged.toFixed(1)} hrs`
          }));
        }
      }

      alert('Jira snapshot data imported successfully!');
    } catch (e: any) {
      alert(`Import failed: ${e.message}`);
    }
  }

  async function save() {
    saving = true;
    try {
      const content = pres?.type === 'intro'
        ? { learnings: learnings.filter(l => l.content.trim()), changes: changes.filter(ch => ch.content.trim()), previous_data: prevData, epics, contributors: contributors.filter(c => c.name.trim()), closing_text: closingText } as IntroContent
        : { previous_data: prevData, feedback: retroFeedback.filter(Boolean), contributors: contributors.filter(c => c.name.trim()), closing_text: closingText } as RetroContent;
      pres = await api.presentations.update(planID, presID, { 
        title, 
        template_id: templateID,
        sprint_name: sprintName, 
        content 
      });
    } catch (e: any) { error = e.message; }
    finally { saving = false; }
  }

  async function publishPres() {
    publishing = true;
    await save();
    pres = await api.presentations.publish(planID, presID);
    publishing = false;
  }

  async function unpublishPres() {
    pres = await api.presentations.unpublish(planID, presID);
  }

  function addLearning() { learnings = [...learnings, { title: '', content: '', tags: [] }]; }
  function removeLearning(i: number) { learnings = learnings.filter((_,j) => j !== i); }
  function addChange() { changes = [...changes, { title: '', content: '', tags: [] }]; }
  function removeChange(i: number) { changes = changes.filter((_,j) => j !== i); }
  function addEpic() { epics = [...epics, { id:'', title:'', summary:'', why_needed:'', when_doing:'', audience:'', total_sp:0 }]; }
  function removeEpic(i: number) { epics = epics.filter((_,j) => j !== i); }
  function addFeedback() { retroFeedback = [...retroFeedback, '']; }
  function removeFeedback(i: number) { retroFeedback = retroFeedback.filter((_,j) => j !== i); }
  function addContributor() { contributors = [...contributors, { name: '', contribution: '' }]; }
  function removeContributor(i: number) { contributors = contributors.filter((_,j) => j !== i); }
</script>

<svelte:head>
  <title>{title || 'Edit Presentation'} – Scrumy</title>
</svelte:head>

<div style="background:var(--c-surface);border-bottom:1px solid var(--c-border);padding:12px 24px;display:flex;align-items:center;justify-content:space-between;position:sticky;top:56px;z-index:99;">
  <div class="flex items-center gap-3">
    <a href="/plans/{planID}" class="btn-icon" style="font-size:18px;">←</a>
    <div>
      <h1 class="font-semibold" style="font-size:16px;">{title || 'Edit Presentation'}</h1>
      {#if pres}<span class="badge {pres.status === 'published' ? 'badge-success' : 'badge-warning'}">{pres.status}</span>{/if}
    </div>
  </div>
  <div class="flex gap-2">
    {#if pres?.status === 'published'}
      <a href="/plans/{planID}/presentations/{presID}/view" class="btn btn-secondary">▶ Preview</a>
      <button class="btn btn-ghost btn-sm" onclick={unpublishPres}>Unpublish</button>
    {:else}
      <button class="btn btn-secondary" onclick={save} disabled={saving}>{saving ? 'Saving…' : 'Save Draft'}</button>
      <button class="btn btn-primary" onclick={publishPres} disabled={publishing}>
        {publishing ? 'Publishing…' : '✓ Publish'}
      </button>
    {/if}
  </div>
</div>

{#if error}
  <div style="padding:8px 24px;background:var(--c-danger-lt);color:var(--c-danger);font-size:13px;">⚠️ {error}</div>
{/if}

{#if loading}
  <div class="flex items-center gap-3" style="padding:40px 24px"><span class="spinner"></span><span class="text-muted">Loading…</span></div>
{:else if pres}
<div style="max-width:900px;margin:0 auto;padding:24px 24px 80px;display:flex;flex-direction:column;gap:20px;">

  <div class="card">
    <div class="card-body grid-3">
      <div class="form-group">
        <label class="label">Presentation Title</label>
        <input class="input" bind:value={title} placeholder="e.g. Sprint 24 Intro" />
      </div>
      <div class="form-group">
        <label class="label">Template</label>
        <select class="input" bind:value={templateID}>
          <option value="default">Default (Dark/Modern)</option>
          <option value="minimalist">Minimalist (Light/Simple)</option>
          <option value="fire">Fire (Dark)</option>
        </select>
      </div>
      <div class="form-group">
        <label class="label">Sprint Name</label>
        <input class="input" bind:value={sprintName} placeholder="e.g. Sprint 24 – 25" />
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
    <div class="card-header" style="display:flex; justify-content:space-between; align-items:center;">
      <div class="flex items-center gap-2"><span style="font-size:18px;">📊</span><h2 class="font-semibold">Data from previous sprint</h2></div>
      
      {#if snapshots.length > 0}
        <div class="flex items-center gap-2" style="font-size:13px; font-weight:normal; text-transform:none; letter-spacing:0;">
          <span style="color:var(--c-text-3);">Import:</span>
          <select class="input" style="padding:4px 8px; font-size:12px; width:180px; height:auto;" bind:value={selectedSnapshotID}>
            <option value="">-- Select Jira Snapshot --</option>
            {#each snapshots as s}
              <option value={s.id}>{s.name}</option>
            {/each}
          </select>
          <button class="btn btn-secondary btn-sm" onclick={importSnapshot} disabled={!selectedSnapshotID} style="padding:4px 10px;">Import</button>
        </div>
      {/if}
    </div>
    <div class="card-body" style="display:flex;flex-direction:column;gap:16px;">
      <div class="grid-3">
        <div class="form-group">
          <label class="label">Total Story Points Delivered</label>
          <input type="number" class="input" bind:value={prevData.total_sp_delivered} min="0" />
        </div>
        {#if pres.type === 'intro'}
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
          <label class="label">Spillovers</label>
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

  {#if pres.type === 'intro'}
    <div style="display:grid;grid-template-columns:1fr 1fr;gap:16px;">
      <div class="card">
        <div class="card-header">
          <div class="flex items-center gap-2"><span>💡</span><h2 class="font-semibold">Learnings</h2></div>
          <button class="btn btn-ghost btn-sm" onclick={addLearning}>+ Add</button>
        </div>
        <div class="card-body" style="display:flex;flex-direction:column;gap:20px;">
          {#each learnings as l, i (i)}
            <div style="border-left:3px solid var(--c-purple);padding-left:16px;display:flex;flex-direction:column;gap:10px;position:relative;">
              <div class="flex justify-between items-center">
                <input class="input grow" bind:value={learnings[i].title} placeholder="Heading (e.g. Deployment Velocity)" style="font-weight:600;" />
                {#if learnings.length > 1}<button class="btn-icon" style="color:var(--c-danger);" onclick={() => removeLearning(i)}>🗑</button>{/if}
              </div>
              <textarea class="input" bind:value={learnings[i].content} placeholder="What did we learn? (Content)" rows="2"></textarea>
              <div class="flex items-center gap-2">
                <span style="font-size:11px;color:var(--c-text-3);text-transform:uppercase;font-weight:600;">Tags:</span>
                <input class="input" 
                  value={learnings[i].tags.join(', ')} 
                  oninput={e => learnings[i].tags = e.currentTarget.value.split(',').map(t => t.trim()).filter(Boolean)} 
                  placeholder="e.g. DEVOPS, PROCESS" 
                  style="font-size:12px;padding:4px 8px;" 
                />
              </div>
            </div>
          {/each}
        </div>

      </div>
      <div class="card">
        <div class="card-header">
          <div class="flex items-center gap-2"><span>🔄</span><h2 class="font-semibold">Changes</h2></div>
          <button class="btn btn-ghost btn-sm" onclick={addChange}>+ Add</button>
        </div>
        <div class="card-body" style="display:flex;flex-direction:column;gap:20px;">
          {#each changes as ch, i (i)}
            <div style="border-left:3px solid var(--c-success);padding-left:16px;display:flex;flex-direction:column;gap:10px;position:relative;">
              <div class="flex justify-between items-center">
                <input class="input grow" bind:value={changes[i].title} placeholder="Heading (e.g. CI/CD Pipeline)" style="font-weight:600;" />
                {#if changes.length > 1}<button class="btn-icon" style="color:var(--c-danger);" onclick={() => removeChange(i)}>🗑</button>{/if}
              </div>
              <textarea class="input" bind:value={changes[i].content} placeholder="Describe the change…" rows="2"></textarea>
              <div class="flex items-center gap-2">
                <span style="font-size:11px;color:var(--c-text-3);text-transform:uppercase;font-weight:600;">Tags:</span>
                <input class="input" 
                  value={changes[i].tags.join(', ')} 
                  oninput={e => changes[i].tags = e.currentTarget.value.split(',').map(t => t.trim()).filter(Boolean)} 
                  placeholder="e.g. AUTOMATION, INFRA" 
                  style="font-size:12px;padding:4px 8px;" 
                />
              </div>
            </div>
          {/each}
        </div>

      </div>
    </div>

    <div class="card">
      <div class="card-header">
        <div class="flex items-center gap-2"><span style="font-size:18px;">🗂</span><h2 class="font-semibold">Current Epics</h2></div>
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
                  <input class="input grow" bind:value={epics[i].title} placeholder="Epic Title" style="font-size:15px;font-weight:500;" />
                </div>
              </div>
              {#if epics.length > 1}<button class="btn-icon" style="color:var(--c-danger);" onclick={() => removeEpic(i)}>🗑</button>{/if}
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
          style="width:100%;padding:12px;border:2px dashed var(--c-border-2);border-radius:var(--r);background:none;cursor:pointer;color:var(--c-primary);font-size:14px;font-family:inherit;"
          onclick={addEpic}
        >+ Add Another Epic</button>
      </div>
    </div>
  {:else}
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
            {#if retroFeedback.length > 1}<button class="btn-icon" style="color:var(--c-danger);margin-top:4px;" onclick={() => removeFeedback(i)}>✕</button>{/if}
          </div>
        {/each}
      </div>
    </div>
  {/if}

  <!-- Closing Slide Customization -->
  <div class="card">
    <div class="card-header">
      <div class="flex items-center gap-2"><span style="font-size:18px;">🎬</span><h2 class="font-semibold">Closing Slide</h2></div>
    </div>
    <div class="card-body">
      <div class="form-group">
        <label class="label">Custom Closing Text</label>
        <input class="input" bind:value={closingText} placeholder={pres.type === 'intro' ? "Let's Ship It!" : "Thank You!"} />
        <p class="text-muted" style="font-size:12px;margin-top:6px;">Leave empty to use the default text and emoji.</p>
      </div>
    </div>
  </div>

</div>
{/if}
