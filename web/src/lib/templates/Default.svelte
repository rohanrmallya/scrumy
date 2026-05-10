<script lang="ts">
  import type { Epic, PreviousData } from '$lib/api';

  let { slide, pres, addFeedbackItem, addingFeedback, newFeedback = $bindable() } = $props<{
    slide: { type: string; data: any };
    pres: any;
    addFeedbackItem: () => Promise<void>;
    addingFeedback: boolean;
    newFeedback: string;
  }>();
</script>

<div class="slide" style="position:relative; height:100%; background:#0A0C10; overflow:hidden; color: #fff;">
  <!-- Global Dynamic Background -->
  <div class="dynamic-bg" style="opacity: {slide.type === 'title' || slide.type === 'closing' ? 1 : 0.4}; transition: opacity 1s ease;">
    <div class="blob blob-1"></div>
    <div class="blob blob-2"></div>
    <div class="blob blob-3"></div>
    <div class="blob blob-4"></div>
  </div>

  <div style="position:relative; z-index:2; height:100%; width:100%; overflow:hidden;">
    {#if slide.type === 'title'}
      <div class="slide-content">
        <div class="glass-tag fade-in-up" style="animation-delay: 0.1s;">
          {slide.data.isRetro ? 'Sprint Retrospective' : 'Sprint Introduction'}
        </div>
        <h1 class="fade-in-up" style="font-size:clamp(40px, 8vw, 100px); font-weight:900; line-height:1.1; margin-bottom:20px; letter-spacing:-0.04em; animation-delay: 0.2s; background: linear-gradient(180deg, #FFFFFF 0%, rgba(255,255,255,0.7) 100%); -webkit-background-clip: text; -webkit-text-fill-color: transparent;">
          {slide.data.title}
        </h1>
        {#if slide.data.sprint}
          <p class="fade-in-up" style="font-size:clamp(20px, 4vw, 36px); color:rgba(255,255,255,0.5); font-weight:500; animation-delay: 0.3s; letter-spacing:-0.02em;">
            {slide.data.sprint}
          </p>
        {/if}
      </div>

    {:else if slide.type === 'contributors'}
      <div style="height:100%; padding:40px 60px; display:flex; flex-direction:column; font-family:'Inter', sans-serif;">
        <div style="margin-bottom:40px;" class="fade-in-up">
          <h1 style="font-size:42px; font-weight:800; color:#fff; margin-bottom:8px; letter-spacing:-0.02em;">Folks that Contributed</h1>
          <p style="font-size:18px; color:rgba(255,255,255,0.5);">Celebrating the team's efforts in {pres?.sprint_name}</p>
        </div>
        <div style="flex:1; overflow-y:auto; display:grid; grid-template-columns:repeat(auto-fill, minmax(300px, 1fr)); grid-auto-rows:max-content; gap:24px; padding:6px;">
          {#each slide.data.items as person, i}
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
      </div>

    {:else if slide.type === 'metrics'}
      {@const d = slide.data as PreviousData}
      {@const sc = d.spillovers === 0 ? { bg:'rgba(22, 163, 74, 0.1)', border:'rgba(22, 163, 74, 0.2)', text:'#4ADE80' } : d.spillovers <= 8 ? { bg:'rgba(217, 119, 6, 0.1)', border:'rgba(217, 119, 6, 0.2)', text:'#FBBF24' } : { bg:'rgba(239, 68, 68, 0.1)', border:'rgba(239, 68, 68, 0.2)', text:'#F87171' }}
      <div style="height:100%; padding:40px 60px; display:flex; flex-direction:column; font-family:'Inter', sans-serif;">
        <div style="margin-bottom:40px;" class="fade-in-up">
          <h1 style="font-size:42px; font-weight:800; color:#fff; margin-bottom:8px; letter-spacing:-0.02em;">Sprint Review: Metrics</h1>
          <p style="font-size:18px; color:rgba(255,255,255,0.5);">Data from previous sprint</p>
        </div>
        <div style="display:grid; grid-template-columns:1.2fr 1fr 1fr; grid-template-rows:repeat(3, minmax(0, 1fr)); gap:24px; flex:1; margin-bottom:40px;">
          <div class="glass-card fade-in-up" style="grid-row: span 2; padding:32px; display:flex; flex-direction:column; position:relative; animation-delay: 0.1s;">
            <span style="font-size:12px; font-weight:700; color:#818CF8; text-transform:uppercase; letter-spacing:0.05em; margin-bottom:auto;">Total Story Points Delivered</span>
            <div style="margin-bottom:12px;">
              <span style="font-size:120px; font-weight:900; color:#fff; line-height:1; letter-spacing:-0.04em;">{d.total_sp_delivered}</span> 
              <span style="font-size:24px; color:rgba(255,255,255,0.4); margin-left:8px;">points</span>
            </div>
          </div>
          <div class="glass-card fade-in-up" style="padding:24px; display:flex; flex-direction:column; justify-content:space-between; animation-delay: 0.2s;">
            <span style="font-size:12px; font-weight:700; color:rgba(255,255,255,0.6); text-transform:uppercase; letter-spacing:0.05em;">Total Epics Delivered</span>
            <div style="display:flex; align-items:baseline; gap:8px;">
              <span style="font-size:56px; font-weight:800; color:#fff;">{d.total_epics_delivered}</span>
              <span style="font-size:18px; color:rgba(255,255,255,0.4);">epics</span>
            </div>
          </div>
          <div class="glass-card fade-in-up" style="background:{sc.bg}; border:1px solid {sc.border}; border-radius:24px; backdrop-filter: blur(12px); padding:24px; display:flex; flex-direction:column; justify-content:space-between; position:relative; animation-delay: 0.3s;">
            <div style="display:flex; justify-content:space-between; align-items:flex-start;">
              <span style="font-size:12px; font-weight:700; color:{sc.text}; text-transform:uppercase; letter-spacing:0.05em;">Spillovers</span>
            </div>
            <div style="display:flex; align-items:baseline; gap:8px;">
              <span style="font-size:56px; font-weight:800; color:{sc.text};">{d.spillovers}</span>
              <span style="font-size:18px; color:{sc.text}; opacity:0.8;">pts</span>
            </div>
          </div>
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
          <div class="glass-card fade-in-up" style="padding:24px; display:flex; flex-direction:column; justify-content:space-between; animation-delay: 0.5s;">
            <span style="font-size:12px; font-weight:700; color:rgba(255,255,255,0.6); text-transform:uppercase; letter-spacing:0.05em;">Hrs / Story Point</span>
            <div style="display:flex; align-items:baseline; gap:8px;">
              <span style="font-size:56px; font-weight:800; color:#fff;">{d.avg_hours_per_sp.toFixed(1)}</span>
            </div>
          </div>
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
                <div style="width:{(d.executed_sp / (d.planned_sp || 1)) * 100}%; height:100%; background:linear-gradient(90deg, #4F46E5, #818CF8);"></div>
              </div>
            </div>
          </div>
        </div>
      </div>

    {:else if slide.type === 'learnings' || slide.type === 'changes'}
      <div style="height:100%; padding:40px 60px; display:flex; flex-direction:column; font-family:'Inter', sans-serif;">
        <div style="display:flex; justify-content:space-between; align-items:flex-start; margin-bottom:40px;" class="fade-in-up">
          <div>
            <h1 style="font-size:42px; font-weight:800; color:#fff; margin-bottom:8px; letter-spacing:-0.02em;">
              {slide.type === 'learnings' ? 'Team Learnings' : 'Process Changes'}
            </h1>
          </div>
        </div>
        <div style="display:grid; grid-template-columns:1fr 1fr; grid-template-rows:repeat(auto-fill, minmax(140px, 1fr)); gap:24px; flex:1; overflow-y:auto; padding:4px;">
          {#each slide.data.items as item, i}
            <div class="glass-card fade-in-up" style="padding:28px; display:flex; gap:24px; animation-delay: {i * 0.05}s;">
              <div style="width:56px; height:56px; border-radius:16px; flex-shrink:0; background:var(--c-primary); display:flex; align-items:center; justify-content:center;">
                <span style="color:#fff; font-weight:900; font-size:20px;">{i+1}</span>
              </div>
              <div style="display:flex; flex-direction:column; gap:12px;">
                <h3 style="font-size:22px; font-weight:700; color:#fff; line-height:1.2;">{item.title || 'Entry'}</h3>
                <p style="font-size:16px; color:rgba(255,255,255,0.6); line-height:1.6;">{item.content}</p>
              </div>
            </div>
          {/each}
        </div>
      </div>

    {:else if slide.type === 'epic'}
      {@const e = slide.data as Epic}
      <div style="height:100%; display:grid; grid-template-columns: 1fr 380px; font-family:'Inter', sans-serif;">
        <div style="padding: 60px 80px; overflow-y: auto; display: flex; flex-direction: column; gap: 48px; border-right: 1px solid rgba(255,255,255,0.1);">
          <div class="fade-in-up">
            <h1 style="font-size: 68px; font-weight: 900; color: #fff; line-height: 1.1;">{e.title}</h1>
          </div>
          <div class="fade-in-up" style="animation-delay: 0.1s;">
            <h2 style="font-size: 26px; font-weight: 800; color: #818CF8; margin-bottom: 20px;">Summary</h2>
            <p style="font-size: 22px; color: rgba(255,255,255,0.7); line-height: 1.6;">{e.summary}</p>
          </div>
          <div class="fade-in-up" style="animation-delay: 0.2s;">
            <h2 style="font-size: 26px; font-weight: 800; color: #818CF8; margin-bottom: 24px;">Why we need it?</h2>
            <div style="display: flex; flex-direction: column; gap: 20px;">
              {#each (e.why_needed || '').split('\n').filter(Boolean) as line}
                <div class="glass-card" style="padding: 20px 24px; display: flex; gap: 20px; align-items: flex-start;">
                  <div style="width: 28px; height: 28px; background: rgba(79, 70, 229, 0.2); border: 2px solid #6366F1; border-radius: 50%; display: flex; align-items: center; justify-content: center; flex-shrink: 0; margin-top: 2px;">
                    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#6366F1" stroke-width="4" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"></polyline></svg>
                  </div>
                  <p style="font-size: 19px; color: #fff; line-height: 1.5;">{line}</p>
                </div>
              {/each}
            </div>
          </div>
        </div>
        <div style="padding: 60px 40px; display: flex; flex-direction: column; gap: 56px;">
          <div class="fade-in-up" style="animation-delay: 0.3s;">
            <h2 style="font-size: 24px; font-weight: 800; color: #818CF8; margin-bottom: 24px;">Audience</h2>
            <div style="display: flex; flex-direction: column; gap: 16px;">
              {#each (e.audience || '').split('\n').filter(Boolean) as aud}
                <div class="glass-card" style="padding: 18px 24px; display: flex; align-items: center; gap: 20px;">
                  <span style="font-size: 18px; font-weight: 700; color: #fff;">{aud}</span>
                </div>
              {/each}
            </div>
          </div>
          <div class="fade-in-up" style="animation-delay: 0.4s;">
            <h2 style="font-size: 24px; font-weight: 800; color: #818CF8; margin-bottom: 24px;">Timeline</h2>
            <div class="glass-card" style="padding: 28px; position: relative; overflow: hidden;">
              <div style="position: absolute; left: 0; top: 0; bottom: 0; width: 6px; background: #4F46E5;"></div>
              <div style="display: flex; flex-direction: column; gap: 6px;">
                <span style="font-size: 11px; font-weight: 800; color: rgba(255,255,255,0.4); text-transform: uppercase; letter-spacing: 0.1em;">Target Completion</span>
                <span style="font-size: 22px; font-weight: 900; color: #818CF8;">{e.when_doing || 'TBD'}</span>
                {#if e.total_sp}
                  <div style="margin-top: 16px; display: flex; align-items: center; gap: 8px; color: rgba(255,255,255,0.5); font-size: 16px; font-weight: 600;">
                    <span>{e.total_sp} Story Points</span>
                  </div>
                {/if}
              </div>
            </div>
          </div>
        </div>
      </div>

    {:else if slide.type === 'retro-feedback'}
      <div style="height:100%; padding:40px 60px; display:flex; flex-direction:column; gap:32px; overflow:hidden; font-family:'Inter', sans-serif;">
        <h2 style="font-size:42px; font-weight:900; color:#fff;">Feedback & Learnings</h2>
        <div style="flex:1; overflow-y:auto; display:grid; grid-template-columns:repeat(auto-fill, minmax(360px, 1fr)); grid-auto-rows:max-content; gap:28px; padding:6px;">
          {#each slide.data.items as item, i}
            <div class="glass-card fade-in-up" style="padding:32px; display:flex; flex-direction:column; gap:24px;">
              <p style="font-size:22px; color:#fff; line-height:1.6;">{item}</p>
            </div>
          {/each}
        </div>
        <div class="fade-in-up" style="display:flex; gap:16px; background:rgba(255,255,255,0.05); border:1px solid rgba(255,255,255,0.1); border-radius:20px; padding:12px;">
          <input class="input" bind:value={newFeedback} placeholder="Share thoughts live…" onkeydown={e => e.key === 'Enter' && addFeedbackItem()} style="flex:1; background:transparent; color:#fff; border:none;" />
          <button class="btn btn-primary" onclick={addFeedbackItem} disabled={addingFeedback}>Post</button>
        </div>
      </div>

    {:else if slide.type === 'closing'}
      <div class="slide-content">
        <h1 class="fade-in-up" style="font-size:clamp(40px, 8vw, 100px); font-weight:900;">
          {slide.data.closing_text || (slide.data.isRetro ? 'Thank You!' : "Let's Ship It!")}
        </h1>
        <p class="fade-in-up" style="color:rgba(255,255,255,0.5); font-size:clamp(20px, 4vw, 36px);">{slide.data.sprint}</p>
      </div>
    {/if}
  </div>
</div>

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

  .slide-content { position:relative; z-index:2; display:flex; flex-direction:column; align-items:center; justify-content:center; height:100%; text-align:center; padding:60px; }
  .fade-in-up { animation: fadeInUp 0.8s forwards; opacity:0; }
  @keyframes fadeInUp { from { opacity:0; transform:translateY(30px); } to { opacity:1; transform:translateY(0); } }
  .glass-tag { background:rgba(255,255,255,0.05); backdrop-filter:blur(10px); border:1px solid rgba(255,255,255,0.1); padding:8px 16px; border-radius:100px; font-size:12px; font-weight:600; text-transform:uppercase; color:rgba(255,255,255,0.7); margin-bottom:24px; }
  
  .glass-card { 
    background: rgba(255, 255, 255, 0.03); 
    backdrop-filter: blur(20px); 
    border: 1px solid rgba(255, 255, 255, 0.08); 
    border-radius: 24px; 
    position: relative; 
    transition: transform 300ms ease, border-color 300ms ease;
  }
  .glass-card::before {
    content: '';
    position: absolute;
    inset: -2px;
    padding: 2px;
    border-radius: 24px;
    background: linear-gradient(135deg, #818CF8, #C084FC, #F472B6);
    -webkit-mask: linear-gradient(#fff 0 0) content-box, linear-gradient(#fff 0 0);
    mask: linear-gradient(#fff 0 0) content-box, linear-gradient(#fff 0 0);
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
</style>
