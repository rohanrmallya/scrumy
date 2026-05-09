<script lang="ts">
  import { page } from '$app/stores';
  import { onMount, onDestroy } from 'svelte';
  import { goto } from '$app/navigation';
  import { api, type Presentation, type IntroContent, type RetroContent, type Epic, type PreviousData, type Contributor } from '$lib/api';

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
      // Title slide
      s.push({ type: 'title', data: { title: pres.title, sprint: pres.sprint_name } });
      // Contributors slide
      if (c?.contributors?.length) {
        s.push({ type: 'contributors', data: { items: c.contributors } });
      }
      // Metrics from previous sprint
      if (c?.previous_data) s.push({ type: 'metrics', data: c.previous_data });
      // Learnings slide
      if (c?.learnings?.length) {
        const items = c.learnings.map((l: any) => typeof l === 'string' ? { title: '', content: l, tags: [] } : l);
        s.push({ type: 'learnings', data: { items } });
      }
      // Changes slide
      if (c?.changes?.length) {
        const items = c.changes.map((ch: any) => typeof ch === 'string' ? { title: '', content: ch, tags: [] } : ch);
        s.push({ type: 'changes', data: { items } });
      }
      // Epic slides
      for (const epic of (c?.epics ?? [])) {
        s.push({ type: 'epic', data: epic });
      }
      // Closing slide
      s.push({ type: 'closing', data: { sprint: pres.sprint_name, closing_text: c?.closing_text } });
    } else {
      const c = pres.content as RetroContent;
      s.push({ type: 'title', data: { title: pres.title, sprint: pres.sprint_name, isRetro: true } });
      if (c?.contributors?.length) {
        s.push({ type: 'contributors', data: { items: c.contributors } });
      }
      if (c?.previous_data) s.push({ type: 'metrics', data: c.previous_data });
      s.push({ type: 'retro-feedback', data: { items: c?.feedback ?? [], presID, planID } });
      s.push({ type: 'closing', data: { sprint: pres.sprint_name, isRetro: true, closing_text: c?.closing_text } });
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
      <div class="slide" style="position:relative; height:100%; background:#0A0C10; overflow:hidden;">
        <!-- Global Dynamic Background -->
        <div class="dynamic-bg" style="opacity: {s.type === 'title' || s.type === 'closing' ? 1 : 0.4}; transition: opacity 1s ease;">
          <div class="blob blob-1"></div>
          <div class="blob blob-2"></div>
          <div class="blob blob-3"></div>
          <div class="blob blob-4"></div>
        </div>

        <div style="position:relative; z-index:2; height:100%; width:100%; overflow:hidden;">
          {#if s.type === 'title'}
            <!-- Title Slide -->
            <div class="slide-content">
              <div class="glass-tag fade-in-up" style="animation-delay: 0.1s;">
                {s.data.isRetro ? 'Sprint Retrospective' : 'Sprint Introduction'}
              </div>
              <h1 class="fade-in-up" style="font-size:clamp(40px, 8vw, 100px); font-weight:900; line-height:1.1; margin-bottom:20px; letter-spacing:-0.04em; animation-delay: 0.2s; background: linear-gradient(180deg, #FFFFFF 0%, rgba(255,255,255,0.7) 100%); -webkit-background-clip: text; -webkit-text-fill-color: transparent;">
                {s.data.title}
              </h1>
              {#if s.data.sprint}
                <p class="fade-in-up" style="font-size:clamp(20px, 4vw, 36px); color:rgba(255,255,255,0.5); font-weight:500; animation-delay: 0.3s; letter-spacing:-0.02em;">
                  {s.data.sprint}
                </p>
              {/if}
            </div>

          {:else if s.type === 'contributors'}
            <!-- Contributors Slide -->
            <div style="height:100%; padding:40px 60px; display:flex; flex-direction:column; font-family:'Inter', sans-serif;">
              <div style="margin-bottom:40px;" class="fade-in-up">
                <h1 style="font-size:42px; font-weight:800; color:#fff; margin-bottom:8px; letter-spacing:-0.02em;">Folks that Contributed</h1>
                <p style="font-size:18px; color:rgba(255,255,255,0.5);">Celebrating the team's efforts in {pres?.sprint_name}</p>
              </div>

              <div style="flex:1; overflow-y:auto; display:grid; grid-template-columns:repeat(auto-fill, minmax(300px, 1fr)); grid-auto-rows:max-content; gap:24px; padding:6px;">
                {#each s.data.items as person, i}
                  <div class="glass-card fade-in-up" style="padding:24px; display:flex; flex-direction:column; gap:16px; animation-delay: {i * 0.05}s;">
                    <div style="display:flex; align-items:center; gap:16px;">
                      <div style="width:50px; height:50px; border-radius:50%; background:linear-gradient(135deg, #818CF8, #C084FC); display:flex; align-items:center; justify-content:center; font-weight:800; color:#fff; font-size:20px; box-shadow:0 4px 12px rgba(129, 140, 248, 0.3);">
                        {person.name.charAt(0).toUpperCase()}
                      </div>
                      <div style="display:flex; flex-direction:column;">
                        <span style="font-size:22px; font-weight:800; color:#fff;">{person.name}</span>
                        <span style="font-size:13px; font-weight:700; color:#818CF8; text-transform:uppercase; letter-spacing:0.05em;">Contributor</span>
                      </div>
                    </div>
                    {#if person.contribution}
                      <div style="background:rgba(255,255,255,0.03); border:1px solid rgba(255,255,255,0.06); border-radius:12px; padding:12px 16px;">
                        <p style="font-size:15px; color:rgba(255,255,255,0.7); line-height:1.5;">{person.contribution}</p>
                      </div>
                    {/if}
                  </div>
                {/each}
              </div>

              <!-- Footer -->
              <div style="border-top:1px solid rgba(255,255,255,0.1); padding-top:20px; display:flex; justify-content:space-between; align-items:center; color:rgba(255,255,255,0.3); font-size:13px; font-weight:600;">
                <div>{pres?.sprint_name ?? 'Sprint'} • Recognition</div>
                <div>Keep up the great work!</div>
              </div>
            </div>

          {:else if s.type === 'metrics'}
            {@const d = s.data as PreviousData}
            {@const sc = d.spillovers === 0 ? { bg:'rgba(22, 163, 74, 0.1)', border:'rgba(22, 163, 74, 0.2)', text:'#4ADE80' } : d.spillovers <= 8 ? { bg:'rgba(217, 119, 6, 0.1)', border:'rgba(217, 119, 6, 0.2)', text:'#FBBF24' } : { bg:'rgba(239, 68, 68, 0.1)', border:'rgba(239, 68, 68, 0.2)', text:'#F87171' }}
            <!-- Metrics Slide -->
            <div style="height:100%; padding:40px 60px; display:flex; flex-direction:column; font-family:'Inter', sans-serif; color:#fff;">
              <div style="margin-bottom:40px;" class="fade-in-up">
                <h1 style="font-size:42px; font-weight:800; color:#fff; margin-bottom:8px; letter-spacing:-0.02em;">Sprint Review: Metrics</h1>
                <p style="font-size:18px; color:rgba(255,255,255,0.5);">Data from previous sprint</p>
              </div>

              <div style="display:grid; grid-template-columns:1.2fr 1fr 1fr; grid-template-rows:repeat(3, minmax(0, 1fr)); gap:24px; flex:1; margin-bottom:40px;">
                
                <!-- Total Story Points Delivered -->
                <div class="glass-card fade-in-up" style="grid-row: span 2; padding:32px; display:flex; flex-direction:column; position:relative; animation-delay: 0.1s;">
                  <span style="font-size:12px; font-weight:700; color:#818CF8; text-transform:uppercase; letter-spacing:0.05em; margin-bottom:auto;">Total Story Points Delivered</span>
                  <div style="position:absolute; top:32px; right:32px; width:48px; height:48px; background:rgba(129, 140, 248, 0.15); border-radius:12px; display:flex; align-items:center; justify-content:center;">
                    <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="#818CF8" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"></polyline></svg>
                  </div>
                  <div style="margin-bottom:12px;">
                    <span style="font-size:120px; font-weight:900; color:#fff; line-height:1; letter-spacing:-0.04em;">{d.total_sp_delivered}</span> 
                    <span style="font-size:24px; color:rgba(255,255,255,0.4); margin-left:8px;">points</span>
                  </div>
                </div>

                <!-- Total Epics Delivered -->
                <div class="glass-card fade-in-up" style="padding:24px; display:flex; flex-direction:column; justify-content:space-between; animation-delay: 0.2s;">
                  <span style="font-size:12px; font-weight:700; color:rgba(255,255,255,0.6); text-transform:uppercase; letter-spacing:0.05em;">Total Epics Delivered</span>
                  <div style="display:flex; align-items:baseline; gap:8px;">
                    <span style="font-size:56px; font-weight:800; color:#fff;">{d.total_epics_delivered}</span>
                    <span style="font-size:18px; color:rgba(255,255,255,0.4);">epics</span>
                  </div>
                </div>

                <!-- Spillovers -->
                <div class="glass-card fade-in-up" style="background:{sc.bg}; border:1px solid {sc.border}; border-radius:24px; backdrop-filter: blur(12px); padding:24px; display:flex; flex-direction:column; justify-content:space-between; position:relative; animation-delay: 0.3s;">
                  <div style="display:flex; justify-content:space-between; align-items:flex-start;">
                    <span style="font-size:12px; font-weight:700; color:{sc.text}; text-transform:uppercase; letter-spacing:0.05em;">Spillovers</span>
                    {#if d.spillovers === 0}
                      <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="{sc.text}" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"></polyline></svg>
                    {:else}
                      <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="{sc.text}" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"></path><line x1="12" y1="9" x2="12" y2="13"></line><line x1="12" y1="17" x2="12.01" y2="17"></line></svg>
                    {/if}
                  </div>
                  <div style="display:flex; align-items:baseline; gap:8px;">
                    <span style="font-size:56px; font-weight:800; color:{sc.text};">{d.spillovers}</span>
                    <span style="font-size:18px; color:{sc.text}; opacity:0.8;">pts</span>
                  </div>
                </div>

                <!-- Total Work Logged -->
                <div class="glass-card fade-in-up" style="grid-column: span 2; padding:24px; display:flex; flex-direction:column; justify-content:space-between; animation-delay: 0.4s;">
                  <span style="font-size:12px; font-weight:700; color:rgba(255,255,255,0.6); text-transform:uppercase; letter-spacing:0.05em;">Total Work Logged</span>
                  <div style="display:flex; align-items:center; justify-content:space-between;">
                    <div style="display:flex; align-items:baseline; gap:8px;">
                      <span style="font-size:56px; font-weight:800; color:#fff;">{d.total_hours_logged}</span>
                      <span style="font-size:18px; color:rgba(255,255,255,0.4);">hours</span>
                    </div>
                    <div style="width:240px; height:12px; background:rgba(255,255,255,0.05); border-radius:6px; overflow:hidden; border:1px solid rgba(255,255,255,0.1);">
                      <div style="width:75%; height:100%; background:linear-gradient(90deg, #818CF8, #C084FC); border-radius:6px;"></div>
                    </div>
                  </div>
                </div>

                <!-- Hrs / Story Point -->
                <div class="glass-card fade-in-up" style="padding:24px; display:flex; flex-direction:column; justify-content:space-between; animation-delay: 0.5s;">
                  <span style="font-size:12px; font-weight:700; color:rgba(255,255,255,0.6); text-transform:uppercase; letter-spacing:0.05em;">Hrs / Story Point</span>
                  <div style="display:flex; align-items:baseline; gap:8px;">
                    <span style="font-size:56px; font-weight:800; color:#fff;">{d.avg_hours_per_sp.toFixed(1)}</span>
                    <span style="font-size:18px; color:rgba(255,255,255,0.4);">avg</span>
                  </div>
                </div>

                <!-- Capacity Comparison -->
                <div class="glass-card fade-in-up" style="grid-column: span 2; padding:32px; display:flex; flex-direction:column; gap:20px; animation-delay: 0.6s;">
                  <div style="display:flex; flex-direction:column; gap:8px;">
                    <div style="display:flex; justify-content:space-between; font-size:12px; font-weight:700; color:rgba(255,255,255,0.5); text-transform:uppercase; letter-spacing:0.05em;">
                      <span>Planned Capacity</span>
                      <span style="color:#fff;">{d.planned_sp} pts</span>
                    </div>
                    <div style="width:100%; height:10px; background:rgba(255,255,255,0.05); border-radius:5px; overflow:hidden;">
                      <div style="width:100%; height:100%; background:rgba(255,255,255,0.2);"></div>
                    </div>
                  </div>
                  <div style="display:flex; flex-direction:column; gap:8px;">
                    <div style="display:flex; justify-content:space-between; font-size:12px; font-weight:700; color:#818CF8; text-transform:uppercase; letter-spacing:0.05em;">
                      <span>Executed Capacity</span>
                      <span style="color:#fff;">{d.executed_sp} pts</span>
                    </div>
                    <div style="width:100%; height:10px; background:rgba(255,255,255,0.05); border-radius:5px; overflow:hidden;">
                      <div style="width:{(d.executed_sp / d.planned_sp) * 100}%; height:100%; background:linear-gradient(90deg, #4F46E5, #818CF8);"></div>
                    </div>
                  </div>
                </div>
              </div>

              <!-- Footer -->
              <div style="border-top:1px solid rgba(255,255,255,0.1); padding-top:20px; display:flex; justify-content:space-between; align-items:center; color:rgba(255,255,255,0.3); font-size:13px; font-weight:600;">
                <div>{pres?.sprint_name ?? 'Sprint 42'} • Q3</div>
                <div>Generated: Today, 09:00 AM</div>
              </div>
            </div>

          {:else if s.type === 'learnings' || s.type === 'changes'}
            <!-- Learnings/Changes Slide -->
            <div style="height:100%; padding:40px 60px; display:flex; flex-direction:column; font-family:'Inter', sans-serif;">
              <div style="display:flex; justify-content:space-between; align-items:flex-start; margin-bottom:40px;" class="fade-in-up">
                <div>
                  <h1 style="font-size:42px; font-weight:800; color:#fff; margin-bottom:8px; letter-spacing:-0.02em;">
                    {s.type === 'learnings' ? 'Team Learnings' : 'Process Changes'}
                  </h1>
                  <p style="font-size:18px; color:rgba(255,255,255,0.5);">
                    {s.type === 'learnings' ? `${pres?.sprint_name ?? 'Sprint'} Retrospective Insights` : `Strategic adjustments for ${pres?.sprint_name ?? 'next sprint'}`}
                  </p>
                </div>
                <div style="background:{s.type === 'learnings' ? 'rgba(79, 70, 229, 0.2)' : 'rgba(16, 185, 129, 0.2)'}; color:{s.type === 'learnings' ? '#818CF8' : '#34D399'}; padding:8px 20px; border-radius:100px; font-size:14px; font-weight:700; display:flex; align-items:center; gap:8px; border:1px solid {s.type === 'learnings' ? 'rgba(79, 70, 229, 0.3)' : 'rgba(16, 185, 129, 0.3)'};">
                  <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"></path></svg>
                  {s.type === 'learnings' ? pres?.sprint_name : 'IMPROVEMENT'}
                </div>
              </div>

              <div style="display:grid; grid-template-columns:1fr 1fr; grid-template-rows:repeat(auto-fill, minmax(140px, 1fr)); gap:24px; flex:1; overflow-y:auto; padding:4px;">
                {#each s.data.items as item, i}
                  {@const gradients = [
                    'linear-gradient(135deg, #6366F1 0%, #4F46E5 100%)',
                    'linear-gradient(135deg, #EC4899 0%, #DB2777 100%)',
                    'linear-gradient(135deg, #10B981 0%, #059669 100%)',
                    'linear-gradient(135deg, #F59E0B 0%, #D97706 100%)',
                    'linear-gradient(135deg, #3B82F6 0%, #2563EB 100%)',
                    'linear-gradient(135deg, #8B5CF6 0%, #7C3AED 100%)'
                  ]}
                  {@const gradient = gradients[i % gradients.length]}
                  <div class="glass-card fade-in-up" style="padding:28px; display:flex; gap:24px; animation-delay: {i * 0.05}s;">
                    <div style="width:56px; height:56px; border-radius:16px; flex-shrink:0; background:{gradient}; display:flex; align-items:center; justify-content:center; box-shadow: 0 8px 16px rgba(0,0,0,0.2);">
                      <span style="color:#fff; font-weight:900; font-size:20px;">{i+1}</span>
                    </div>
                    <div style="display:flex; flex-direction:column; gap:12px;">
                      <h3 style="font-size:22px; font-weight:700; color:#fff; line-height:1.2;">{item.title || (s.type === 'learnings' ? 'Key Insight' : 'Process Update')}</h3>
                      <p style="font-size:16px; color:rgba(255,255,255,0.6); line-height:1.6;">{item.content}</p>
                      <div style="display:flex; flex-wrap:wrap; gap:8px; margin-top:4px;">
                        {#each (item.tags ?? []) as tag}
                          <span style="background:rgba(255,255,255,0.05); color:rgba(255,255,255,0.5); padding:4px 10px; border-radius:6px; font-size:11px; font-weight:700; text-transform:uppercase; letter-spacing:0.05em; border:1px solid rgba(255,255,255,0.1);">{tag}</span>
                        {/each}
                      </div>
                    </div>
                  </div>
                {/each}
              </div>
            </div>

          {:else if s.type === 'epic'}
            {@const e = s.data as Epic}
            <!-- Redesigned Epic Slide (Dark) -->
            <div style="height:100%; display:grid; grid-template-columns: 1fr 380px; font-family:'Inter', sans-serif;">
              
              <!-- Left Side: Main Content -->
              <div style="padding: 60px 80px; overflow-y: auto; display: flex; flex-direction: column; gap: 48px; border-right: 1px solid rgba(255,255,255,0.1);">
                
                <div class="fade-in-up">
                  <div style="display: flex; gap: 12px; margin-bottom: 24px;">
                    {#if e.id}
                      <span style="background: rgba(234, 88, 12, 0.15); color: #FB923C; padding: 6px 12px; border-radius: 6px; font-size: 13px; font-weight: 700; border: 1px solid rgba(234, 88, 12, 0.3); text-transform: uppercase; letter-spacing: 0.05em;">{e.id}</span>
                    {/if}
                    <span style="background: rgba(79, 70, 229, 0.15); color: #818CF8; padding: 6px 12px; border-radius: 6px; font-size: 13px; font-weight: 700; border: 1px solid rgba(79, 70, 229, 0.3); text-transform: uppercase; letter-spacing: 0.05em;">{pres?.sprint_name?.includes('Q') ? pres.sprint_name.split(' ')[0] : ''} INITIATIVE</span>
                  </div>
                  <h1 style="font-size: 68px; font-weight: 900; color: #fff; line-height: 1.1; letter-spacing: -0.03em;">{e.title}</h1>
                </div>

                <div class="fade-in-up" style="animation-delay: 0.1s;">
                  <div style="display: flex; align-items: center; gap: 12px; color: #818CF8; margin-bottom: 20px;">
                    <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"><line x1="8" y1="6" x2="21" y2="6"></line><line x1="8" y1="12" x2="21" y2="12"></line><line x1="8" y1="18" x2="21" y2="18"></line></svg>
                    <h2 style="font-size: 26px; font-weight: 800;">Summary</h2>
                  </div>
                  <p style="font-size: 22px; color: rgba(255,255,255,0.7); line-height: 1.6; font-weight: 400;">{e.summary || 'No summary provided.'}</p>
                </div>

                <div class="fade-in-up" style="animation-delay: 0.2s;">
                  <div style="display: flex; align-items: center; gap: 12px; color: #818CF8; margin-bottom: 24px;">
                    <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"></circle><circle cx="12" cy="12" r="3"></circle></svg>
                    <h2 style="font-size: 26px; font-weight: 800;">Why we need it?</h2>
                  </div>
                  <div style="display: flex; flex-direction: column; gap: 20px;">
                    {#each (e.why_needed || '').split('\n').filter(Boolean) as line}
                      <div class="glass-card" style="padding: 20px 24px; display: flex; gap: 20px; align-items: flex-start;">
                        <div style="width: 28px; height: 28px; background: rgba(79, 70, 229, 0.2); border: 2px solid #6366F1; border-radius: 50%; display: flex; align-items: center; justify-content: center; flex-shrink: 0; margin-top: 2px;">
                          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#6366F1" stroke-width="4" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"></polyline></svg>
                        </div>
                        <p style="font-size: 19px; color: #fff; line-height: 1.5;">
                          {#if line.includes(':')}
                            <strong style="color: #818CF8; font-weight: 800;">{line.split(':')[0]}:</strong>{line.split(':').slice(1).join(':')}
                          {:else}
                            {line}
                          {/if}
                        </p>
                      </div>
                    {/each}
                  </div>
                </div>
              </div>

              <!-- Right Side: Sidebar -->
              <div style="padding: 60px 40px; display: flex; flex-direction: column; gap: 56px;">
                
                <div class="fade-in-up" style="animation-delay: 0.3s;">
                  <div style="display: flex; align-items: center; gap: 12px; color: #818CF8; margin-bottom: 24px;">
                    <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"></path><circle cx="9" cy="7" r="4"></circle></svg>
                    <h2 style="font-size: 24px; font-weight: 800;">Audience</h2>
                  </div>
                  <div style="display: flex; flex-direction: column; gap: 16px;">
                    {#each (e.audience || '').split('\n').filter(Boolean) as aud}
                      <div class="glass-card" style="padding: 18px 24px; display: flex; align-items: center; gap: 20px;">
                        <div style="width: 44px; height: 44px; background: rgba(59, 130, 246, 0.15); border-radius: 12px; display: flex; align-items: center; justify-content: center; color: #60A5FA;">
                          <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"></path><circle cx="12" cy="7" r="4"></circle></svg>
                        </div>
                        <span style="font-size: 18px; font-weight: 700; color: #fff;">{aud}</span>
                      </div>
                    {/each}
                  </div>
                </div>

                <div class="fade-in-up" style="animation-delay: 0.4s;">
                  <div style="display: flex; align-items: center; gap: 12px; color: #818CF8; margin-bottom: 24px;">
                    <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="4" width="18" height="18" rx="2" ry="2"></rect><line x1="16" y1="2" x2="16" y2="6"></line><line x1="8" y1="2" x2="8" y2="6"></line></svg>
                    <h2 style="font-size: 24px; font-weight: 800;">Timeline</h2>
                  </div>
                  <div class="glass-card" style="padding: 28px; position: relative; overflow: hidden;">
                    <div style="position: absolute; left: 0; top: 0; bottom: 0; width: 6px; background: #4F46E5;"></div>
                    <div style="display: flex; flex-direction: column; gap: 6px;">
                      <span style="font-size: 11px; font-weight: 800; color: rgba(255,255,255,0.4); text-transform: uppercase; letter-spacing: 0.1em;">Target Completion</span>
                      <div style="display: flex; align-items: baseline; gap: 10px;">
                        <span style="font-size: 22px; font-weight: 900; color: #818CF8;">{e.when_doing?.split('–')[0] || 'TBD'}</span>
                        <span style="color: rgba(255,255,255,0.2); font-size: 20px;">—</span>
                        <span style="font-size: 22px; font-weight: 700; color: #fff;">{e.when_doing?.split('–')[1] || ''}</span>
                      </div>
                      {#if e.total_sp}
                        <div style="margin-top: 16px; display: flex; align-items: center; gap: 8px; color: rgba(255,255,255,0.5); font-size: 16px; font-weight: 600;">
                          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"></path></svg>
                          <span>{e.total_sp} Story Points</span>
                        </div>
                      {/if}
                    </div>
                  </div>
                </div>
              </div>
            </div>

          {:else if s.type === 'retro-feedback'}
            <!-- Interactive Retro Feedback Slide (Dark) -->
            <div style="height:100%; padding:40px 60px; display:flex; flex-direction:column; gap:32px; overflow:hidden; font-family:'Inter', sans-serif;">
              <div style="display:flex; justify-content:space-between; align-items:center;" class="fade-in-up">
                <div>
                  <h2 style="font-size:42px; font-weight:900; color:#fff; letter-spacing:-0.03em;">Feedback & Learnings</h2>
                  <p style="font-size:18px; color:rgba(255,255,255,0.5); margin-top:4px;">Insights gathered from the team live</p>
                </div>
                <div class="glass-tag" style="background:rgba(79, 70, 229, 0.2); color:#818CF8; border-color:rgba(79, 70, 229, 0.3); margin-bottom:0;">
                  <span style="display:flex; align-items:center; gap:8px;">
                    <div style="width:8px; height:8px; background:#818CF8; border-radius:50%; animation: pulse 2s infinite;"></div>
                    INTERACTIVE
                  </span>
                </div>
              </div>

              <div style="flex:1; overflow-y:auto; display:grid; grid-template-columns:repeat(auto-fill, minmax(360px, 1fr)); grid-auto-rows:max-content; gap:28px; padding:6px;">
                {#each s.data.items as item, i}
                  <div class="glass-card fade-in-up" style="padding:32px; display:flex; flex-direction:column; gap:24px; animation-delay: {i * 0.05}s;">
                    <div style="width:44px; height:44px; background:linear-gradient(135deg, #6366F1 0%, #4F46E5 100%); border-radius:12px; display:flex; align-items:center; justify-content:center; color:#fff; font-weight:900; font-size:18px; box-shadow:0 8px 20px rgba(79, 70, 229, 0.3);">
                      {i+1}
                    </div>
                    <p style="font-size:22px; color:#fff; line-height:1.6; font-weight:500; letter-spacing:-0.01em;">{item}</p>
                  </div>
                {/each}
                {#if s.data.items.length === 0}
                  <div style="grid-column: 1 / -1; height: 240px; display: flex; align-items: center; justify-content: center; background: rgba(255,255,255,0.03); border: 2px dashed rgba(255,255,255,0.1); border-radius: 24px; color: rgba(255,255,255,0.2); font-size: 20px; font-weight: 500;">
                    Waiting for team feedback...
                  </div>
                {/if}
              </div>

              <!-- Live add feedback -->
              <div class="fade-in-up" style="display:flex; gap:16px; background:rgba(255,255,255,0.05); border:1px solid rgba(255,255,255,0.1); backdrop-filter: blur(20px); border-radius:20px; padding:12px 12px 12px 24px; box-shadow:0 20px 40px rgba(0,0,0,0.4); margin-bottom:40px; animation-delay: 0.2s;">
                <input
                  class="input"
                  bind:value={newFeedback}
                  placeholder="Share your thoughts live…"
                  onkeydown={e => e.key === 'Enter' && addFeedbackItem()}
                  style="flex:1; font-size:20px; border:none; background:transparent; outline:none; color:#fff; padding:12px 0;"
                />
                <button 
                  class="btn" 
                  onclick={addFeedbackItem} 
                  disabled={addingFeedback || !newFeedback.trim()}
                  style="background:#4F46E5; color:#fff; padding:0 32px; border-radius:14px; font-weight:800; font-size:16px; border:none; cursor:pointer; transition:all 200ms; height:56px;"
                >
                  {addingFeedback ? 'Adding...' : 'Post Feedback'}
                </button>
              </div>
            </div>

          {:else if s.type === 'closing'}
            <!-- Closing Slide -->
            <div class="slide-content">
              <div style="font-size:80px; margin-bottom:32px; animation: bounce 2s infinite ease-in-out;">
                {s.data.isRetro ? '🙏' : '🚀'}
              </div>
              <h1 class="fade-in-up" style="font-size:clamp(40px, 8vw, 100px); font-weight:900; margin-bottom:16px; letter-spacing:-0.04em; background: linear-gradient(180deg, #FFFFFF 0%, rgba(255,255,255,0.7) 100%); -webkit-background-clip: text; -webkit-text-fill-color: transparent;">
                {s.data.closing_text || (s.data.isRetro ? 'Thank You!' : "Let's Ship It!")}
              </h1>
              <p class="fade-in-up" style="color:rgba(255,255,255,0.5); font-size:clamp(20px, 4vw, 36px); font-weight:500; animation-delay: 0.1s;">
                {s.data.sprint}
              </p>
            </div>
          {/if}
        </div>
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
  .dynamic-bg {
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    overflow: hidden;
    background: #0A0C10;
    z-index: 0;
  }

  .blob {
    position: absolute;
    width: 60vmax;
    height: 60vmax;
    border-radius: 50%;
    filter: blur(80px);
    opacity: 0.35;
    mix-blend-mode: screen;
    animation: move 25s infinite alternate ease-in-out;
  }

  .blob-1 {
    background: radial-gradient(circle, #4F46E5 0%, transparent 70%);
    top: -10%;
    left: -10%;
    animation: liquid-1 12s infinite alternate ease-in-out;
  }

  .blob-2 {
    background: radial-gradient(circle, #7C3AED 0%, transparent 70%);
    bottom: -10%;
    right: -10%;
    animation: liquid-2 15s infinite alternate ease-in-out;
    animation-delay: -2s;
  }

  .blob-3 {
    background: radial-gradient(circle, #DB2777 0%, transparent 70%);
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
    animation: liquid-3 18s infinite alternate ease-in-out;
    animation-delay: -4s;
  }

  .blob-4 {
    background: radial-gradient(circle, #2563EB 0%, transparent 70%);
    bottom: 20%;
    left: 10%;
    animation: liquid-1 10s infinite alternate-reverse ease-in-out;
    animation-delay: -6s;
  }

  @keyframes liquid-1 {
    0% { transform: translate(0, 0) scale(1) rotate(0deg); }
    33% { transform: translate(30%, 20%) scale(1.2) rotate(30deg); }
    66% { transform: translate(-20%, 30%) scale(0.8) rotate(-20deg); }
    100% { transform: translate(0, 0) scale(1) rotate(0deg); }
  }

  @keyframes liquid-2 {
    0% { transform: translate(0, 0) scale(1.1) rotate(0deg); }
    50% { transform: translate(-30%, -25%) scale(1.4) rotate(-40deg); }
    100% { transform: translate(0, 0) scale(1.1) rotate(0deg); }
  }

  @keyframes liquid-3 {
    0% { transform: translate(-50%, -50%) scale(1); }
    33% { transform: translate(-30%, -70%) scale(1.3) rotate(20deg); }
    66% { transform: translate(-70%, -30%) scale(0.7) rotate(-20deg); }
    100% { transform: translate(-50%, -50%) scale(1); }
  }

  .slide-content {
    position: relative;
    z-index: 2;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 100%;
    width: 100%;
    text-align: center;
    padding: 60px;
    color: #fff;
  }

  .fade-in-up {
    animation: fadeInUp 0.8s cubic-bezier(0.16, 1, 0.3, 1) forwards;
    opacity: 0;
  }

  @keyframes fadeInUp {
    from {
      opacity: 0;
      transform: translateY(30px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }

  .glass-tag {
    background: rgba(255, 255, 255, 0.05);
    backdrop-filter: blur(10px);
    border: 1px solid rgba(255, 255, 255, 0.1);
    padding: 8px 16px;
    border-radius: 100px;
    font-size: 12px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.15em;
    color: rgba(255, 255, 255, 0.7);
    margin-bottom: 24px;
  }

  .glass-card {
    background: rgba(255, 255, 255, 0.03);
    backdrop-filter: blur(20px);
    border: 1px solid rgba(255, 255, 255, 0.08);
    border-radius: 24px;
    box-shadow: 0 8px 32px rgba(0,0,0,0.2);
    transition: transform 300ms ease, border-color 300ms ease;
    position: relative;
  }

  .glass-card::before {
    content: '';
    position: absolute;
    inset: -2px;
    padding: 2px;
    border-radius: 24px;
    background: linear-gradient(135deg, #818CF8, #C084FC, #F472B6);
    -webkit-mask: 
      linear-gradient(#fff 0 0) content-box, 
      linear-gradient(#fff 0 0);
    mask: 
      linear-gradient(#fff 0 0) content-box, 
      linear-gradient(#fff 0 0);
    -webkit-mask-composite: xor;
    mask-composite: exclude;
    opacity: 0;
    transition: opacity 0.5s ease;
    pointer-events: none;
  }

  .glass-card:hover {
    border-color: rgba(255, 255, 255, 0.15);
    transform: translateY(-4px);
  }

  .glass-card:hover::before {
    opacity: 1;
  }


  @keyframes pulse {
    0%, 100% { opacity: 1; transform: scale(1); }
    50% { opacity: 0.5; transform: scale(0.8); }
  }

  @keyframes bounce {
    0%, 100% { transform: translateY(0); }
    50% { transform: translateY(-20px); }
  }
</style>

