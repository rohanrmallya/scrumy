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
