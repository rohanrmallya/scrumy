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
          <div style="height:100%; position:relative; overflow:hidden;">
            <div class="dynamic-bg">
              <div class="blob blob-1"></div>
              <div class="blob blob-2"></div>
              <div class="blob blob-3"></div>
              <div class="blob blob-4"></div>
            </div>
            
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
          </div>

        {:else if s.type === 'metrics'}
          {@const d = s.data as PreviousData}
          {@const sc = d.spillovers === 0 ? { bg:'#F0FDF4', border:'#DCFCE7', text:'#16A34A' } : d.spillovers <= 8 ? { bg:'#FFFBEB', border:'#FEF3C7', text:'#D97706' } : { bg:'#FFFBFB', border:'#FEE2E2', text:'#EF4444' }}
          <!-- Metrics Slide -->
          <div style="background:#fff;height:100%;padding:40px 60px;display:flex;flex-direction:column;font-family:'Inter', sans-serif;">
            <div style="margin-bottom:40px;">
              <h1 style="font-size:42px;font-weight:700;color:#1A1B2E;margin-bottom:8px;letter-spacing:-0.02em;">Sprint Review: Metrics</h1>
              <p style="font-size:18px;color:#6B7280;">Data from previous sprint</p>
            </div>

            <div style="display:grid;grid-template-columns:1.2fr 1fr 1fr;grid-template-rows:repeat(3, minmax(0, 1fr));gap:24px;flex:1;margin-bottom:40px;">
              
              <!-- Total Story Points Delivered (Span 2 rows) -->
              <div style="grid-row: span 2; background:#F8FAFF; border:1px solid #E0E7FF; border-radius:12px; padding:32px; display:flex; flex-direction:column; position:relative;">
                <span style="font-size:12px; font-weight:700; color:#4C6EF5; text-transform:uppercase; letter-spacing:0.05em; margin-bottom:auto;">Total Story Points Delivered</span>
                <div style="position:absolute; top:32px; right:32px; width:40px; height:40px; background:#DDE6FF; border-radius:50%; display:flex; align-items:center; justify-content:center;">
                  <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="#4C6EF5" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"></polyline></svg>
                </div>
                <div style="margin-bottom:12px;">
                  <span style="font-size:120px; font-weight:800; color:#1A1B2E; line-height:1;">{d.total_sp_delivered}</span> story points
                </div>
              </div>

              <!-- Total Epics Delivered -->
              <div style="background:#fff; border:1px solid #E5E7EB; border-radius:12px; padding:24px; display:flex; flex-direction:column; justify-content:space-between;">
                <span style="font-size:12px; font-weight:700; color:#374151; text-transform:uppercase; letter-spacing:0.05em;">Total Epics Delivered</span>
                <div style="display:flex; align-items:baseline; gap:8px;">
                  <span style="font-size:48px; font-weight:800; color:#1A1B2E;">{d.total_epics_delivered}</span>
                  <span style="font-size:18px; color:#6B7280;">epics</span>
                </div>
              </div>

              <!-- Spillovers -->
              <div style="background:{sc.bg}; border:1px solid {sc.border}; border-radius:12px; padding:24px; display:flex; flex-direction:column; justify-content:space-between; position:relative;">
                <div style="display:flex; justify-content:space-between; align-items:flex-start;">
                  <span style="font-size:12px; font-weight:700; color:{sc.text}; text-transform:uppercase; letter-spacing:0.05em;">Spillovers</span>
                  {#if d.spillovers === 0}
                    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="{sc.text}" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"></polyline></svg>
                  {:else}
                    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="{sc.text}" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"></path><line x1="12" y1="9" x2="12" y2="13"></line><line x1="12" y1="17" x2="12.01" y2="17"></line></svg>
                  {/if}
                </div>
                <div style="display:flex; align-items:baseline; gap:8px;">
                  <span style="font-size:48px; font-weight:800; color:{sc.text};">{d.spillovers}</span>
                  <span style="font-size:18px; color:{sc.text}; opacity:0.8;">pts</span>
                </div>
              </div>


              <!-- Total Work Logged (Span 2 columns) -->
              <div style="grid-column: span 2; background:#F9FAFB; border:1px solid #E5E7EB; border-radius:12px; padding:24px; display:flex; flex-direction:column; justify-content:space-between;">
                <span style="font-size:12px; font-weight:700; color:#374151; text-transform:uppercase; letter-spacing:0.05em;">Total Work Logged</span>
                <div style="display:flex; align-items:center; justify-content:space-between;">
                  <div style="display:flex; align-items:baseline; gap:8px;">
                    <span style="font-size:48px; font-weight:800; color:#1A1B2E;">{d.total_hours_logged}</span>
                    <span style="font-size:18px; color:#6B7280;">hours</span>
                  </div>
                  <div style="width:240px; height:12px; background:#E5E7EB; border-radius:6px; overflow:hidden;">
                    <div style="width:75%; height:100%; background:#4B5563; border-radius:6px;"></div>
                  </div>
                </div>
              </div>

              <!-- Hrs / Story Point -->
              <div style="background:#fff; border:1px solid #E5E7EB; border-radius:12px; padding:24px; display:flex; flex-direction:column; justify-content:space-between;">
                <span style="font-size:12px; font-weight:700; color:#374151; text-transform:uppercase; letter-spacing:0.05em;">Hrs / Story Point</span>
                <div style="display:flex; align-items:baseline; gap:8px;">
                  <span style="font-size:48px; font-weight:800; color:#1A1B2E;">{d.avg_hours_per_sp.toFixed(1)}</span>
                  <span style="font-size:18px; color:#6B7280;">avg</span>
                </div>
              </div>

              <!-- Capacity Comparison (Span 2 columns) -->
              <div style="grid-column: span 2; background:#F9FAFB; border:1px solid #E5E7EB; border-radius:12px; padding:32px; display:flex; flex-direction:column; gap:20px;">
                <div style="display:flex; flex-direction:column; gap:8px;">
                  <div style="display:flex; justify-content:space-between; font-size:12px; font-weight:700; color:#4B5563; text-transform:uppercase; letter-spacing:0.05em;">
                    <span>Planned Capacity</span>
                    <span>{d.planned_sp} pts</span>
                  </div>
                  <div style="width:100%; height:10px; background:#E5E7EB; border-radius:5px; overflow:hidden;">
                    <div style="width:100%; height:100%; background:#6B7280;"></div>
                  </div>
                </div>
                <div style="display:flex; flex-direction:column; gap:8px;">
                  <div style="display:flex; justify-content:space-between; font-size:12px; font-weight:700; color:#4C6EF5; text-transform:uppercase; letter-spacing:0.05em;">
                    <span>Executed Capacity</span>
                    <span>{d.executed_sp} pts</span>
                  </div>
                  <div style="width:100%; height:10px; background:#E5E7EB; border-radius:5px; overflow:hidden;">
                    <div style="width:{(d.executed_sp / d.planned_sp) * 100}%; height:100%; background:#4C6EF5;"></div>
                  </div>
                </div>
              </div>
            </div>

            <!-- Footer -->
            <div style="border-top:1px solid #E5E7EB; padding-top:20px; display:flex; justify-content:space-between; align-items:center; color:#9CA3AF; font-size:13px; font-weight:600;">
              <div>{pres?.sprint_name ?? 'Sprint 42'} • Q3</div>
              <div>Generated: Today, 09:00 AM</div>
            </div>
          </div>


        {:else if s.type === 'learnings'}
          <!-- Team Learnings Slide -->
          <div style="background:#F3F4F6;height:100%;padding:40px 60px;display:flex;flex-direction:column;font-family:'Inter', sans-serif;">
            <div style="display:flex; justify-content:space-between; align-items:flex-start; margin-bottom:40px;">
              <div>
                <h1 style="font-size:42px;font-weight:700;color:#1A1B2E;margin-bottom:8px;letter-spacing:-0.02em;">Team Learnings</h1>
                <p style="font-size:18px;color:#6B7280;">{pres?.sprint_name ?? 'Sprint 42'} Retrospective Insights</p>
              </div>
              <div style="background:#0066FF; color:#fff; padding:6px 16px; border-radius:20px; font-size:14px; font-weight:700; display:flex; align-items:center; gap:8px;">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"></path></svg>
                {pres?.sprint_name ?? 'Sprint 42'}
              </div>
            </div>

            <div style="display:grid; grid-template-columns:1fr 1fr; grid-template-rows:repeat(auto-fill, minmax(140px, 1fr)); gap:24px; flex:1;">
              {#each s.data.items as item, i}
                {@const gradients = [
                  'linear-gradient(135deg, #667eea 0%, #764ba2 100%)',
                  'linear-gradient(135deg, #ff9a9e 0%, #fecfef 100%)',
                  'linear-gradient(135deg, #a1c4fd 0%, #c2e9fb 100%)',
                  'linear-gradient(135deg, #84fab0 0%, #8fd3f4 100%)',
                  'linear-gradient(135deg, #fccb90 0%, #d57eeb 100%)',
                  'linear-gradient(135deg, #e0c3fc 0%, #8ec5fc 100%)',
                  'linear-gradient(135deg, #f093fb 0%, #f5576c 100%)',
                  'linear-gradient(135deg, #4facfe 0%, #00f2fe 100%)'
                ]}
                {@const gradient = gradients[i % gradients.length]}
                <div style="background:#fff; border:1px solid #E5E7EB; border-radius:12px; padding:24px; display:flex; gap:20px; box-shadow:0 1px 3px rgba(0,0,0,0.05);">
                  <div style="width:48px; height:48px; border-radius:12px; flex-shrink:0; background:{gradient};"></div>
                  <div style="display:flex; flex-direction:column; gap:8px;">

                    <h3 style="font-size:20px; font-weight:700; color:#1A1B2E; line-height:1.2;">{item.title || 'Key Insight'}</h3>
                    <p style="font-size:14px; color:#4B5563; line-height:1.5; margin-bottom:12px;">{item.content}</p>
                    <div style="display:flex; flex-wrap:wrap; gap:6px;">
                      {#each (item.tags ?? []) as tag}
                        <span style="background:#F3F4F6; color:#6B7280; padding:3px 8px; border-radius:4px; font-size:10px; font-weight:700; text-transform:uppercase; letter-spacing:0.05em;">{tag}</span>
                      {/each}
                    </div>
                  </div>
                </div>
              {/each}
            </div>
          </div>
        {:else if s.type === 'changes'}
          <!-- Process Changes Slide -->
          <div style="background:#FDFCFB;height:100%;padding:40px 60px;display:flex;flex-direction:column;font-family:'Inter', sans-serif;">
            <div style="display:flex; justify-content:space-between; align-items:flex-start; margin-bottom:40px;">
              <div>
                <h1 style="font-size:42px;font-weight:700;color:#1A1B2E;margin-bottom:8px;letter-spacing:-0.02em;">Process Changes</h1>
                <p style="font-size:18px;color:#6B7280;">Strategic adjustments for {pres?.sprint_name ?? 'the next sprint'}</p>
              </div>
              <div style="background:#10B981; color:#fff; padding:6px 16px; border-radius:20px; font-size:14px; font-weight:700; display:flex; align-items:center; gap:8px;">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"></path></svg>
                IMPROVEMENT
              </div>
            </div>

            <div style="display:grid; grid-template-columns:1fr 1fr; grid-template-rows:repeat(auto-fill, minmax(140px, 1fr)); gap:24px; flex:1;">
              {#each s.data.items as item, i}
                {@const gradients = [
                  'linear-gradient(135deg, #f093fb 0%, #f5576c 100%)',
                  'linear-gradient(135deg, #5ee7df 0%, #b490ca 100%)',
                  'linear-gradient(135deg, #c31432 0%, #240b36 100%)',
                  'linear-gradient(135deg, #11998e 0%, #38ef7d 100%)',
                  'linear-gradient(135deg, #ee9ca7 0%, #ffdde1 100%)',
                  'linear-gradient(135deg, #6a11cb 0%, #2575fc 100%)'
                ]}
                {@const gradient = gradients[i % gradients.length]}
                <div style="background:#fff; border:1px solid #E5E7EB; border-radius:12px; padding:24px; display:flex; gap:20px; box-shadow:0 1px 3px rgba(0,0,0,0.05);">
                  <div style="width:48px; height:48px; border-radius:12px; flex-shrink:0; background:{gradient};"></div>
                  <div style="display:flex; flex-direction:column; gap:8px;">
                    <h3 style="font-size:20px; font-weight:700; color:#1A1B2E; line-height:1.2;">{item.title || 'Process Update'}</h3>
                    <p style="font-size:14px; color:#4B5563; line-height:1.5; margin-bottom:12px;">{item.content}</p>
                    <div style="display:flex; flex-wrap:wrap; gap:6px;">
                      {#each (item.tags ?? []) as tag}
                        <span style="background:#ECFDF5; color:#059669; padding:3px 8px; border-radius:4px; font-size:10px; font-weight:700; text-transform:uppercase; letter-spacing:0.05em;">{tag}</span>
                      {/each}
                    </div>
                  </div>
                </div>
              {/each}
            </div>
          </div>


        {:else if s.type === 'epic'}
          {@const e = s.data as Epic}
          <!-- Redesigned Epic Slide -->
          <div style="background:#fff; height:100%; display:grid; grid-template-columns: 1fr 340px; font-family:'Inter', sans-serif;">
            
            <!-- Left Side: Main Content -->
            <div style="padding: 60px 80px; overflow-y: auto; display: flex; flex-direction: column; gap: 40px; border-right: 1px solid #F3F4F6;">
              
              <!-- Header & Tags -->
              <div>
                <div style="display: flex; gap: 12px; margin-bottom: 24px;">
                  {#if e.id}
                    <span style="background: #FFF7ED; color: #EA580C; padding: 4px 10px; border-radius: 4px; font-size: 12px; font-weight: 700; border: 1px solid #FFEDD5; text-transform: uppercase; letter-spacing: 0.05em;">{e.id}</span>
                  {/if}
                  <span style="background: #EFF6FF; color: #2563EB; padding: 4px 10px; border-radius: 4px; font-size: 12px; font-weight: 700; border: 1px solid #DBEAFE; text-transform: uppercase; letter-spacing: 0.05em;">{pres?.sprint_name?.includes('Q') ? pres.sprint_name.split(' ')[0] : ''} INITIATIVE</span>
                </div>
                <h1 style="font-size: 64px; font-weight: 800; color: #111827; line-height: 1.1; letter-spacing: -0.02em;">{e.title}</h1>
              </div>

              <!-- Summary Section -->
              <div style="display: flex; flex-direction: column; gap: 16px;">
                <div style="display: flex; align-items: center; gap: 12px; color: #4F46E5;">
                  <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><line x1="8" y1="6" x2="21" y2="6"></line><line x1="8" y1="12" x2="21" y2="12"></line><line x1="8" y1="18" x2="21" y2="18"></line><line x1="3" y1="6" x2="3.01" y2="6"></line><line x1="3" y1="12" x2="3.01" y2="12"></line><line x1="3" y1="18" x2="3.01" y2="18"></line></svg>
                  <h2 style="font-size: 24px; font-weight: 700;">Summary</h2>
                </div>
                <p style="font-size: 20px; color: #4B5563; line-height: 1.6;">{e.summary || 'No summary provided.'}</p>
              </div>

              <!-- Why We Need It Section -->
              <div style="display: flex; flex-direction: column; gap: 20px;">
                <div style="display: flex; align-items: center; gap: 12px; color: #4F46E5;">
                  <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"></circle><circle cx="12" cy="12" r="3"></circle></svg>
                  <h2 style="font-size: 24px; font-weight: 700;">Why we need it?</h2>
                </div>
                <div style="display: flex; flex-direction: column; gap: 16px;">
                  {#each (e.why_needed || '').split('\n').filter(Boolean) as line}
                    <div style="display: flex; gap: 16px; align-items: flex-start;">
                      <div style="width: 24px; height: 24px; background: #EEF2FF; border: 1.5px solid #6366F1; border-radius: 50%; display: flex; align-items: center; justify-content: center; flex-shrink: 0; margin-top: 2px;">
                        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="#6366F1" stroke-width="4" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"></polyline></svg>
                      </div>
                      <p style="font-size: 18px; color: #374151; line-height: 1.4;">
                        {#if line.includes(':')}
                          <strong style="color: #111827;">{line.split(':')[0]}:</strong>{line.split(':').slice(1).join(':')}
                        {:else}
                          {line}
                        {/if}
                      </p>
                    </div>
                  {/each}
                </div>
              </div>
            </div>

            <!-- Right Side: Metadata / Sidebar -->
            <div style="background: #F9FAFB; padding: 60px 40px; display: flex; flex-direction: column; gap: 48px;">
              
              <!-- Audience Section -->
              <div style="display: flex; flex-direction: column; gap: 24px;">
                <div style="display: flex; align-items: center; gap: 12px; color: #4F46E5;">
                  <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"></path><circle cx="9" cy="7" r="4"></circle><path d="M23 21v-2a4 4 0 0 0-3-3.87"></path><path d="M16 3.13a4 4 0 0 1 0 7.75"></path></svg>
                  <h2 style="font-size: 24px; font-weight: 700;">Audience</h2>
                </div>
                <div style="display: flex; flex-direction: column; gap: 12px;">
                  {#each (e.audience || '').split('\n').filter(Boolean) as aud}
                    <div style="background: #fff; border: 1px solid #E5E7EB; border-radius: 12px; padding: 16px 20px; display: flex; align-items: center; gap: 16px; box-shadow: 0 1px 2px rgba(0,0,0,0.05);">
                      <div style="width: 40px; height: 40px; background: #EFF6FF; border-radius: 8px; display: flex; align-items: center; justify-content: center; color: #3B82F6;">
                        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"></path><circle cx="12" cy="7" r="4"></circle></svg>
                      </div>
                      <span style="font-size: 16px; font-weight: 600; color: #1F2937;">{aud}</span>
                    </div>
                  {/each}
                </div>
              </div>

              <!-- Timeline Section -->
              <div style="display: flex; flex-direction: column; gap: 24px;">
                <div style="display: flex; align-items: center; gap: 12px; color: #4F46E5;">
                  <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="4" width="18" height="18" rx="2" ry="2"></rect><line x1="16" y1="2" x2="16" y2="6"></line><line x1="8" y1="2" x2="8" y2="6"></line><line x1="3" y1="10" x2="21" y2="10"></line></svg>
                  <h2 style="font-size: 24px; font-weight: 700;">Timeline</h2>
                </div>
                <div style="background: #fff; border: 1px solid #E5E7EB; border-radius: 12px; padding: 24px; display: flex; gap: 16px; position: relative; overflow: hidden; box-shadow: 0 1px 2px rgba(0,0,0,0.05);">
                  <div style="position: absolute; left: 0; top: 0; bottom: 0; width: 4px; background: #3B82F6;"></div>
                  <div style="display: flex; flex-direction: column; gap: 4px;">
                    <span style="font-size: 11px; font-weight: 700; color: #9CA3AF; text-transform: uppercase; letter-spacing: 0.05em;">Target Completion</span>
                    <div style="display: flex; align-items: baseline; gap: 8px;">
                      <span style="font-size: 20px; font-weight: 800; color: #3B82F6;">{e.when_doing?.split('–')[0] || 'TBD'}</span>
                      <span style="color: #9CA3AF;">—</span>
                      <span style="font-size: 20px; font-weight: 700; color: #1F2937;">{e.when_doing?.split('–')[1] || ''}</span>
                    </div>
                    {#if e.total_sp}
                      <div style="margin-top: 12px; display: flex; align-items: center; gap: 6px; color: #6B7280; font-size: 14px;">
                        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"></path><polyline points="3.27 6.96 12 12.01 20.73 6.96"></polyline><line x1="12" y1="22.08" x2="12" y2="12"></line></svg>
                        <span>{e.total_sp} Story Points</span>
                      </div>
                    {/if}
                  </div>
                </div>
              </div>
            </div>
          </div>

        {:else if s.type === 'retro-feedback'}
          <!-- Interactive Retro Feedback Slide -->
          <div style="background:#F9FAFB;height:100%;padding:40px 60px;display:flex;flex-direction:column;gap:32px;overflow:hidden;font-family:'Inter', sans-serif;">
            <div style="display:flex;justify-content:space-between;align-items:center;">
              <div>
                <h2 style="font-size:42px;font-weight:800;color:#1A1B2E;letter-spacing:-0.02em;">Feedback & Learnings</h2>
                <p style="font-size:18px;color:#6B7280;margin-top:4px;">Insights gathered from the team during retrospective</p>
              </div>
              <span style="background:#EEF2FF; color:#4F46E5; padding:8px 16px; border-radius:20px; font-size:14px; font-weight:700; display:flex; align-items:center; gap:8px; border:1px solid #E0E7FF;">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"></path></svg>
                INTERACTIVE
              </span>
            </div>

            <div style="flex:1;overflow-y:auto;display:grid;grid-template-columns:repeat(auto-fill, minmax(340px, 1fr));grid-auto-rows:max-content;gap:24px;padding:4px;">
              {#each s.data.items as item, i}
                <div style="background:#fff; border:1px solid #E5E7EB; border-radius:16px; padding:28px; display:flex; flex-direction:column; gap:20px; position:relative; box-shadow:0 4px 6px -1px rgba(0,0,0,0.05); animation:fadeIn 400ms ease forwards;">
                  <div style="width:40px; height:40px; background:linear-gradient(135deg, #6366F1 0%, #4F46E5 100%); border-radius:10px; display:flex; align-items:center; justify-content:center; color:#fff; font-weight:800; font-size:16px; box-shadow:0 4px 12px rgba(79, 70, 229, 0.25);">
                    {i+1}
                  </div>
                  <p style="font-size:20px; color:#374151; line-height:1.6; font-weight:500;">{item}</p>
                </div>
              {/each}
              {#if s.data.items.length === 0}
                <div style="grid-column: 1 / -1; height: 200px; display: flex; align-items: center; justify-content: center; background: rgba(255,255,255,0.5); border: 2px dashed #E5E7EB; border-radius: 16px; color: #9CA3AF; font-size: 18px;">
                  Waiting for team feedback...
                </div>
              {/if}
            </div>

            <!-- Live add feedback -->
            <div style="display:flex;gap:16px;background:#fff;border:1px solid #E5E7EB;border-radius:16px;padding:16px 20px;box-shadow:0 10px 15px -3px rgba(0,0,0,0.1);margin-bottom:60px;">
              <input
                class="input"
                bind:value={newFeedback}
                placeholder="Share your thoughts live…"
                onkeydown={e => e.key === 'Enter' && addFeedbackItem()}
                style="flex:1;font-size:18px;border:none;background:transparent;outline:none;"
              />
              <button 
                class="btn btn-primary" 
                onclick={addFeedbackItem} 
                disabled={addingFeedback || !newFeedback.trim()}
                style="padding:10px 24px;border-radius:10px;font-weight:700;font-size:16px;"
              >
                {addingFeedback ? 'Adding...' : 'Post Feedback'}
              </button>
            </div>
          </div>

        {:else if s.type === 'closing'}
          <!-- Closing Slide -->
          <div style="height:100%; position:relative; overflow:hidden;">
            <div class="dynamic-bg">
              <div class="blob blob-1"></div>
              <div class="blob blob-2" style="animation-delay: -12s;"></div>
              <div class="blob blob-3" style="animation-delay: -20s;"></div>
              <div class="blob blob-4" style="animation-delay: -25s;"></div>
            </div>
            
            <div class="slide-content">
              <div style="font-size:80px; margin-bottom:32px; animation: bounce 2s infinite ease-in-out;">
                {s.data.isRetro ? '🙏' : '🚀'}
              </div>
              <h1 class="fade-in-up" style="font-size:clamp(40px, 8vw, 100px); font-weight:900; margin-bottom:16px; letter-spacing:-0.04em; background: linear-gradient(180deg, #FFFFFF 0%, rgba(255,255,255,0.7) 100%); -webkit-background-clip: text; -webkit-text-fill-color: transparent;">
                {s.data.isRetro ? 'Thank You!' : "Let's Ship It!"}
              </h1>
              <p class="fade-in-up" style="color:rgba(255,255,255,0.5); font-size:clamp(20px, 4vw, 36px); font-weight:500; animation-delay: 0.1s;">
                {s.data.sprint}
              </p>
            </div>
          </div>

          <style>
            @keyframes bounce {
              0%, 100% { transform: translateY(0); }
              50% { transform: translateY(-20px); }
            }
          </style>
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
</style>

