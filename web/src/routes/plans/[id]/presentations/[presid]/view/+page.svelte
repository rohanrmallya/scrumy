<script lang="ts">
  import { page } from '$app/stores';
  import { onMount, onDestroy } from 'svelte';
  import { goto } from '$app/navigation';
  import { api, type Presentation, type IntroContent, type RetroContent, type Epic, type PreviousData } from '$lib/api';

  const planID = $derived($page.params.id);
  const presID = $derived($page.params.presid);

  let pres = $state<Presentation | null>(null);
  let loading = $state(true);
  let currentSlide = $state(0);
  let fullscreen = $state(false);
  let newFeedback = $state('');
  let addingFeedback = $state(false);

  // Build slides from content
  let slides = $state<{ type: string; data: any }[]>([]);

  onMount(async () => {
    pres = await api.presentations.get(planID, presID);
    buildSlides();
    loading = false;

    // Keyboard nav
    window.addEventListener('keydown', onKey);
    document.addEventListener('fullscreenchange', handleFSChange);
  });

  onDestroy(() => {
    window.removeEventListener('keydown', onKey);
    document.removeEventListener('fullscreenchange', handleFSChange);
  });

  function handleFSChange() {
    fullscreen = !!document.fullscreenElement;
  }

  function onKey(e: KeyboardEvent) {
    if (e.key === 'ArrowRight' || e.key === 'ArrowDown') next();
    if (e.key === 'ArrowLeft' || e.key === 'ArrowUp') prev();
    if (e.key === 'Escape') {
      if (fullscreen) document.exitFullscreen?.().catch(() => {});
      else goto(`/plans/${planID}`);
    }
  }

  function buildSlides() {
    if (!pres) return;
    const s: { type: string; data: any }[] = [];

    if (pres.type === 'intro') {
      const c = pres.content as IntroContent;
      // Title slide
      s.push({ type: 'title', data: { title: pres.title, sprint: pres.sprint_name } });
      // Metrics from previous sprint
      if (c?.previous_data) s.push({ type: 'metrics', data: c.previous_data });
      // Learnings slide
      if (c?.learnings?.length) s.push({ type: 'learnings', data: { items: c.learnings, changes: c.changes } });
      // Epic slides
      for (const epic of (c?.epics ?? [])) {
        s.push({ type: 'epic', data: epic });
      }
      // Closing slide
      s.push({ type: 'closing', data: { sprint: pres.sprint_name } });
    } else {
      const c = pres.content as RetroContent;
      s.push({ type: 'title', data: { title: pres.title, sprint: pres.sprint_name, isRetro: true } });
      if (c?.previous_data) s.push({ type: 'metrics', data: c.previous_data });
      s.push({ type: 'retro-feedback', data: { items: c?.feedback ?? [], presID, planID } });
      s.push({ type: 'closing', data: { sprint: pres.sprint_name, isRetro: true } });
    }
    slides = s;
  }

  function next() { if (currentSlide < slides.length - 1) currentSlide++; }
  function prev() { if (currentSlide > 0) currentSlide--; }

  async function toggleFullscreen() {
    if (!fullscreen) {
      await document.documentElement.requestFullscreen?.();
    } else {
      await document.exitFullscreen?.();
    }
  }

  async function addFeedbackItem() {
    if (!newFeedback.trim()) return;
    addingFeedback = true;
    pres = await api.presentations.addFeedback(planID, presID, newFeedback.trim());
    buildSlides();
    newFeedback = '';
    addingFeedback = false;
  }
</script>

<svelte:head>
  <title>{pres?.title ?? 'Presentation'} – Scrumy</title>
</svelte:head>

{#if loading}
  <div style="height:100vh;display:flex;align-items:center;justify-content:center;background:#0F1117;">
    <span class="spinner" style="width:32px;height:32px;border-width:3px;border-color:rgba(255,255,255,0.2);border-top-color:var(--c-primary);"></span>
  </div>
{:else}
<div class="slide-viewer" class:is-fullscreen={fullscreen}>
  <!-- Controls bar -->
  <div class="slide-controls">
    <a href="/plans/{planID}" style="color:rgba(255,255,255,0.6);font-size:13px;text-decoration:none;display:flex;align-items:center;gap:6px;">
      ← Back to plan
    </a>
    <div style="color:rgba(255,255,255,0.7);font-size:13px;">{pres?.title} · {currentSlide + 1} / {slides.length}</div>
    <button
      style="background:rgba(255,255,255,0.1);border:1px solid rgba(255,255,255,0.15);color:rgba(255,255,255,0.8);padding:6px 14px;border-radius:6px;font-size:12px;cursor:pointer;"
      onclick={toggleFullscreen}
    >{fullscreen ? 'Exit Fullscreen' : '⛶ Fullscreen'}</button>
  </div>

  <!-- Slide area -->
  <div class="slide-area">
    {#if slides[currentSlide]}
      {@const s = slides[currentSlide]}
      <div class="slide" style="position:relative;">
        {#if s.type === 'title'}
          <!-- Title Slide -->
          <div style="background:linear-gradient(135deg,#1A1B2E 0%,#2D3561 100%);height:100%;display:flex;flex-direction:column;align-items:center;justify-content:center;color:#fff;padding:60px;text-align:center;">
            <div style="font-size:11px;text-transform:uppercase;letter-spacing:0.15em;color:rgba(255,255,255,0.5);margin-bottom:20px;">
              {s.data.isRetro ? 'Sprint Retrospective' : 'Sprint Introduction'}
            </div>
            <h1 style="font-size:clamp(32px,6vw,84px);font-weight:700;line-height:1.2;margin-bottom:16px;">{s.data.title}</h1>
            {#if s.data.sprint}
              <p style="font-size:clamp(18px,3vw,32px);color:rgba(255,255,255,0.6);">{s.data.sprint}</p>
            {/if}
            <div style="margin-top:40px;display:flex;gap:8px;">
              {#each slides as _, i}
                <div style="width:{i === currentSlide ? 24 : 8}px;height:8px;border-radius:4px;background:{i === currentSlide ? 'var(--c-primary)' : 'rgba(255,255,255,0.2)'};transition:all 300ms;"></div>
              {/each}
            </div>
          </div>

        {:else if s.type === 'metrics'}
          {@const d = s.data as PreviousData}
          <!-- Metrics Slide -->
          <div style="background:#fff;height:100%;padding:40px;display:flex;flex-direction:column;">
            <h2 style="font-size:clamp(24px,3.5vw,48px);font-weight:700;color:#1A1B2E;margin-bottom:28px;">📊 Previous Sprint Metrics</h2>
            <div style="display:grid;grid-template-columns:repeat(4,1fr);gap:16px;flex:1;">
              {#each [
                { label:'Story Points', value: d.total_sp_delivered, color: '#4C6EF5' },
                { label:'Hours Logged', value: d.total_hours_logged, color: '#10B981' },
                { label:'Work Logs', value: d.total_work_logs, color: '#F59E0B' },
                { label:'Hrs / SP', value: d.avg_hours_per_sp.toFixed(1), color: '#8B5CF6' },
                { label:'Planned SP', value: d.planned_sp, color: '#4C6EF5' },
                { label:'Executed SP', value: d.executed_sp, color: '#10B981' },
                { label:'Spillovers', value: d.spillovers, color: '#EF4444' },
              ] as stat}
                <div style="background:var(--c-bg);border-radius:12px;padding:20px;display:flex;flex-direction:column;gap:8px;">
                  <span style="font-size:11px;text-transform:uppercase;letter-spacing:0.08em;color:var(--c-text-3);">{stat.label}</span>
                  <span style="font-size:clamp(28px,4.5vw,64px);font-weight:700;color:{stat.color};">{stat.value}</span>
                </div>
              {/each}
            </div>
          </div>

        {:else if s.type === 'learnings'}
          <!-- Learnings + Changes Slide -->
          <div style="background:#fff;height:100%;padding:40px;display:flex;flex-direction:column;gap:20px;">
            <h2 style="font-size:clamp(24px,3.5vw,48px);font-weight:700;color:#1A1B2E;">💡 Learnings & Changes</h2>
            <div style="display:grid;grid-template-columns:1fr 1fr;gap:20px;flex:1;">
              <div>
                <h3 style="font-size:13px;text-transform:uppercase;letter-spacing:0.1em;color:var(--c-text-3);margin-bottom:12px;">Key Learnings</h3>
                <div style="display:flex;flex-direction:column;gap:10px;">
                  {#each s.data.items as item, i}
                    <div style="display:flex;gap:10px;align-items:flex-start;">
                      <span style="width:22px;height:22px;background:var(--c-primary-lt);border-radius:50%;display:flex;align-items:center;justify-content:center;font-size:11px;font-weight:700;color:var(--c-primary);flex-shrink:0;">{i+1}</span>
                      <p style="font-size:clamp(14px,1.8vw,22px);color:#374151;line-height:1.5;">{item}</p>
                    </div>
                  {/each}
                </div>
              </div>
              <div>
                <h3 style="font-size:13px;text-transform:uppercase;letter-spacing:0.1em;color:var(--c-text-3);margin-bottom:12px;">Changes This Sprint</h3>
                <div style="display:flex;flex-direction:column;gap:10px;">
                  {#each (s.data.changes ?? []) as item}
                    <div style="display:flex;gap:10px;align-items:flex-start;">
                      <span style="width:8px;height:8px;background:var(--c-success);border-radius:50%;margin-top:6px;flex-shrink:0;"></span>
                      <p style="font-size:clamp(14px,1.8vw,22px);color:#374151;line-height:1.5;">{item}</p>
                    </div>
                  {/each}
                </div>
              </div>
            </div>
          </div>

        {:else if s.type === 'epic'}
          {@const e = s.data as Epic}
          <!-- Epic Slide -->
          <div style="background:#fff;height:100%;padding:40px;display:flex;flex-direction:column;">
            <div style="display:flex;align-items:center;gap:10px;margin-bottom:24px;">
              <div style="width:36px;height:36px;background:var(--c-primary);border-radius:8px;display:flex;align-items:center;justify-content:center;color:#fff;font-size:18px;">🗂</div>
              <div>
                <p style="font-size:11px;text-transform:uppercase;letter-spacing:0.1em;color:var(--c-text-3);">Epic</p>
                <h2 style="font-size:clamp(24px,3.5vw,48px);font-weight:700;color:#1A1B2E;">{e.title}</h2>
              </div>
            </div>
            {#if e.summary}
              <p style="font-size:clamp(16px,2vw,24px);color:#4B5563;margin-bottom:20px;line-height:1.6;">{e.summary}</p>
            {/if}
            <div style="display:grid;grid-template-columns:1fr 1fr;gap:16px;flex:1;">
              {#if e.why_needed}
                <div style="background:var(--c-primary-lt);padding:20px;border-radius:12px;">
                  <p style="font-size:11px;text-transform:uppercase;letter-spacing:0.08em;color:var(--c-primary);margin-bottom:8px;font-weight:600;">Why we need it?</p>
                  <p style="font-size:clamp(14px,1.8vw,22px);color:#374151;line-height:1.5;">{e.why_needed}</p>
                </div>
              {/if}
              {#if e.audience}
                <div style="background:var(--c-success-lt);padding:20px;border-radius:12px;">
                  <p style="font-size:11px;text-transform:uppercase;letter-spacing:0.08em;color:var(--c-success);margin-bottom:8px;font-weight:600;">Who consumes it?</p>
                  <p style="font-size:clamp(14px,1.8vw,22px);color:#374151;line-height:1.5;">{e.audience}</p>
                </div>
              {/if}
              {#if e.when_doing}
                <div style="background:var(--c-warning-lt);padding:20px;border-radius:12px;">
                  <p style="font-size:11px;text-transform:uppercase;letter-spacing:0.08em;color:var(--c-warning);margin-bottom:8px;font-weight:600;">When?</p>
                  <p style="font-size:clamp(14px,1.8vw,22px);color:#374151;">{e.when_doing}</p>
                </div>
              {/if}
              {#if e.total_sp}
                <div style="background:var(--c-purple-lt);padding:20px;border-radius:12px;">
                  <p style="font-size:11px;text-transform:uppercase;letter-spacing:0.08em;color:var(--c-purple);margin-bottom:8px;font-weight:600;">Story Points</p>
                  <p style="font-size:clamp(32px,5vw,72px);font-weight:700;color:var(--c-purple);">{e.total_sp}</p>
                </div>
              {/if}
            </div>
          </div>

        {:else if s.type === 'retro-feedback'}
          <!-- Interactive Retro Feedback Slide -->
          <div style="background:#fff;height:100%;padding:32px;display:flex;flex-direction:column;gap:16px;overflow:hidden;">
            <div style="display:flex;justify-content:space-between;align-items:center;">
              <h2 style="font-size:clamp(24px,3.5vw,48px);font-weight:700;color:#1A1B2E;">💬 Feedback & Learnings</h2>
              <span class="badge badge-purple" style="font-size:11px;">Interactive</span>
            </div>
            <div style="flex:1;overflow-y:auto;display:flex;flex-direction:column;gap:8px;">
              {#each s.data.items as item, i}
                <div style="display:flex;gap:10px;align-items:flex-start;animation:fadeIn 300ms ease;">
                  <span style="width:22px;height:22px;background:var(--c-purple-lt);border-radius:50%;display:flex;align-items:center;justify-content:center;font-size:11px;font-weight:700;color:var(--c-purple);flex-shrink:0;">{i+1}</span>
                  <p style="font-size:clamp(14px,1.8vw,22px);color:#374151;line-height:1.5;">{item}</p>
                </div>
              {/each}
              {#if s.data.items.length === 0}
                <p style="color:var(--c-text-3);font-size:14px;">No feedback yet. Use the form below to add items live.</p>
              {/if}
            </div>
            <!-- Live add feedback -->
            <div style="display:flex;gap:10px;border-top:1px solid var(--c-border);padding-top:12px;padding-bottom:80px;">
              <input
                class="input"
                bind:value={newFeedback}
                placeholder="Add a feedback item live…"
                onkeydown={e => e.key === 'Enter' && addFeedbackItem()}
                style="flex:1;font-size:18px;padding:12px 16px;"
              />
              <button class="btn btn-primary btn-sm" onclick={addFeedbackItem} disabled={addingFeedback || !newFeedback.trim()}>
                {addingFeedback ? '…' : '+ Add'}
              </button>
            </div>
          </div>

        {:else if s.type === 'closing'}
          <!-- Closing Slide -->
          <div style="background:linear-gradient(135deg,#1A1B2E 0%,#2D3561 100%);height:100%;display:flex;flex-direction:column;align-items:center;justify-content:center;color:#fff;padding:60px;text-align:center;">
            <div style="font-size:48px;margin-bottom:20px;">{s.data.isRetro ? '🙏' : '🚀'}</div>
            <h1 style="font-size:clamp(32px,5vw,84px);font-weight:700;margin-bottom:12px;">
              {s.data.isRetro ? 'Thank You!' : "Let's Ship It!"}
            </h1>
            <p style="color:rgba(255,255,255,0.6);font-size:clamp(18px,3vw,32px);">
              {s.data.sprint}
            </p>
          </div>
        {/if}
      </div>
    {/if}
  </div>

  <!-- Navigation -->
  <div class="slide-nav">
    <button
      onclick={prev}
      disabled={currentSlide === 0}
      style="width:40px;height:40px;border-radius:50%;background:rgba(255,255,255,0.1);border:1px solid rgba(255,255,255,0.15);color:#fff;font-size:16px;cursor:pointer;disabled:opacity(0.4);transition:all 150ms;"
    >←</button>
    <div style="display:flex;gap:6px;align-items:center;">
      {#each slides as _, i}
        <button
          onclick={() => currentSlide = i}
          style="width:{i === currentSlide ? 20 : 8}px;height:8px;border-radius:4px;background:{i === currentSlide ? 'var(--c-primary)' : 'rgba(255,255,255,0.25)'};border:none;cursor:pointer;transition:all 300ms;padding:0;"
        ></button>
      {/each}
    </div>
    <button
      onclick={next}
      disabled={currentSlide === slides.length - 1}
      style="width:40px;height:40px;border-radius:50%;background:rgba(255,255,255,0.1);border:1px solid rgba(255,255,255,0.15);color:#fff;font-size:16px;cursor:pointer;transition:all 150ms;"
    >→</button>
  </div>
</div>
{/if}

<style>
  :global(body) {
    overflow: hidden;
  }
</style>
