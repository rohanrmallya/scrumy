<script lang="ts">
  import { page } from '$app/stores';
  import { onMount, onDestroy } from 'svelte';
  import { goto } from '$app/navigation';
  import { api, type Presentation, type IntroContent, type RetroContent } from '$lib/api';
  import DefaultTemplate from '$lib/templates/Default.svelte';
  import MinimalistTemplate from '$lib/templates/Minimalist.svelte';
  import FireTemplate from '$lib/templates/Fire.svelte';

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
    document.body.style.overflow = 'hidden';
    pres = await api.presentations.get(planID, presID);
    buildSlides();
    loading = false;

    // Keyboard nav
    window.addEventListener('keydown', onKey);
    document.addEventListener('fullscreenchange', handleFSChange);
  });

  onDestroy(() => {
    document.body.style.overflow = '';
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
      s.push({ type: 'title', data: { title: pres.title, sprint: pres.sprint_name } });
      if (c?.contributors?.length) s.push({ type: 'contributors', data: { items: c.contributors } });
      if (c?.previous_data) s.push({ type: 'metrics', data: c.previous_data });
      if (c?.learnings?.length) s.push({ type: 'learnings', data: { items: c.learnings } });
      if (c?.changes?.length) s.push({ type: 'changes', data: { items: c.changes } });
      for (const epic of (c?.epics ?? [])) s.push({ type: 'epic', data: epic });
      s.push({ type: 'closing', data: { sprint: pres.sprint_name, closing_text: c?.closing_text } });
    } else {
      const c = pres.content as RetroContent;
      s.push({ type: 'title', data: { title: pres.title, sprint: pres.sprint_name, isRetro: true } });
      if (c?.contributors?.length) s.push({ type: 'contributors', data: { items: c.contributors } });
      if (c?.previous_data) s.push({ type: 'metrics', data: c.previous_data });
      s.push({ type: 'retro-feedback', data: { items: c?.feedback ?? [] } });
      s.push({ type: 'closing', data: { sprint: pres.sprint_name, isRetro: true, closing_text: c?.closing_text } });
    }
    slides = s;
  }

  function next() { if (currentSlide < slides.length - 1) currentSlide++; }
  function prev() { if (currentSlide > 0) currentSlide--; }

  async function toggleFullscreen() {
    if (!fullscreen) await document.documentElement.requestFullscreen?.();
    else await document.exitFullscreen?.();
  }

  async function addFeedbackItem() {
    if (!newFeedback.trim()) return;
    addingFeedback = true;
    pres = await api.presentations.addFeedback(planID, presID, newFeedback.trim());
    buildSlides();
    newFeedback = '';
    addingFeedback = false;
  }

  const templateComponent = $derived(
    pres?.template_id === 'fire' ? FireTemplate :
    pres?.template_id === 'minimalist' ? MinimalistTemplate : 
    DefaultTemplate
  );
</script>

<svelte:head>
  <title>{pres?.title ?? 'Presentation'} – Scrumy</title>
</svelte:head>

{#if loading}
  <div style="height:100vh;display:flex;align-items:center;justify-content:center;background:#0F1117;">
    <span class="spinner"></span>
  </div>
{:else}
<div class="slide-viewer" class:is-fullscreen={fullscreen}>
  <div class="slide-controls" style="background: {pres?.template_id === 'minimalist' ? '#f3f4f6' : pres?.template_id === 'fire' ? '#1A0000' : 'rgba(0,0,0,0.5)'}; border-bottom: 1px solid {pres?.template_id === 'minimalist' ? '#e5e7eb' : pres?.template_id === 'fire' ? 'rgba(255,100,50,0.2)' : 'rgba(255,255,255,0.1)'}">
    <a href="/plans/{planID}" style="color: {pres?.template_id === 'minimalist' ? '#4b5563' : pres?.template_id === 'fire' ? '#FF8C42' : 'rgba(255,255,255,0.6)'}; font-size:13px;text-decoration:none;">← Back</a>
    <div style="color: {pres?.template_id === 'minimalist' ? '#111827' : pres?.template_id === 'fire' ? '#FFAA00' : 'rgba(255,255,255,0.7)'}; font-size:13px;">{pres?.title} · {currentSlide + 1} / {slides.length}</div>
    <button class="btn btn-ghost btn-sm" onclick={toggleFullscreen}>{fullscreen ? 'Exit' : '⛶ Fullscreen'}</button>
  </div>

  <div class="slide-area">
    {#if slides[currentSlide]}
      {@const Template = templateComponent}
      <Template 
        slide={slides[currentSlide]} 
        {pres} 
        {addFeedbackItem} 
        {addingFeedback} 
        bind:newFeedback={newFeedback} 
      />
    {/if}
  </div>

  <div class="slide-nav">
    <button onclick={prev} disabled={currentSlide === 0} class="nav-btn">←</button>
    <div class="dots">
      {#each slides as _, i}
        <button onclick={() => currentSlide = i} class="dot" class:active={i === currentSlide}></button>
      {/each}
    </div>
    <button onclick={next} disabled={currentSlide === slides.length - 1} class="nav-btn">→</button>
  </div>
</div>
{/if}

<style>
  .slide-viewer { height: 100vh; display: flex; flex-direction: column; overflow: hidden; background: #000; }
  .slide-controls { height: 56px; padding: 0 24px; display: flex; align-items: center; justify-content: space-between; z-index: 10; }
  .slide-area { flex: 1; position: relative; }
  .slide-nav { position: absolute; bottom: 32px; left: 0; right: 0; display: flex; align-items: center; justify-content: center; gap: 24px; z-index: 10; pointer-events: none; }
  .slide-nav button, .slide-nav .dots { pointer-events: auto; }
  .nav-btn { width: 44px; height: 44px; border-radius: 50%; background: rgba(0,0,0,0.2); border: 1px solid rgba(255,255,255,0.1); color: #fff; cursor: pointer; }
  .dots { display: flex; gap: 8px; }
  .dot { width: 8px; height: 8px; border-radius: 4px; background: rgba(255,255,255,0.3); border: none; cursor: pointer; transition: all 0.3s; }
  .dot.active { width: 24px; background: var(--c-primary); }
  .is-fullscreen .slide-controls { display: none; }
</style>
