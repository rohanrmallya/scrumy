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

<div class="slide" style="position:relative; height:100%; background:#080000; overflow:hidden; color: #FFC107;">
  <div class="dynamic-bg" style="opacity: {slide.type === 'title' || slide.type === 'closing' ? 1 : 0.5}; transition: opacity 1s ease;">
    <div class="flame flame-1"></div>
    <div class="flame flame-2"></div>
    <div class="flame flame-3"></div>
    <div class="flame flame-4"></div>
    <div class="flame flame-5"></div>
    <div class="ember ember-1"></div>
    <div class="ember ember-2"></div>
    <div class="ember ember-3"></div>
    <div class="ember ember-4"></div>
    <div class="ember ember-5"></div>
    <div class="ember ember-6"></div>
    <div class="ember ember-7"></div>
    <div class="ember ember-8"></div>
  </div>

  <div style="position:relative; z-index:2; height:100%; width:100%; overflow:hidden;">
    {#if slide.type === 'title'}
      <div class="slide-content">
        <div class="fire-tag fade-in-up" style="animation-delay: 0.1s;">
          {slide.data.isRetro ? 'Sprint Retrospective' : 'Sprint Introduction'}
        </div>
        <h1 class="fade-in-up" style="font-size:clamp(40px, 8vw, 100px); font-weight:900; line-height:1.1; margin-bottom:20px; letter-spacing:-0.04em; color:#FFC107;">
          {slide.data.title}
        </h1>
        {#if slide.data.sprint}
          <p class="fade-in-up" style="font-size:clamp(20px, 4vw, 36px); color:rgba(255,150,50,0.6); font-weight:500; animation-delay: 0.3s; letter-spacing:-0.02em;">
            {slide.data.sprint}
          </p>
        {/if}
      </div>

    {:else if slide.type === 'contributors'}
      <div style="height:100%; padding:40px 60px; display:flex; flex-direction:column; font-family:'Inter', sans-serif;">
        <div style="margin-bottom:40px;" class="fade-in-up">
          <h1 style="font-size:42px; font-weight:800; color:#fff; margin-bottom:8px; letter-spacing:-0.02em;">Folks that Contributed</h1>
          <p style="font-size:18px; color:rgba(255,150,50,0.5);">Celebrating the team's efforts in {pres?.sprint_name}</p>
        </div>
        <div style="flex:1; overflow-y:auto; display:grid; grid-template-columns:repeat(auto-fill, minmax(300px, 1fr)); grid-auto-rows:max-content; gap:24px; padding:6px;">
          {#each slide.data.items as person, i}
            <div class="fire-card fade-in-up" style="padding:24px; display:flex; flex-direction:column; gap:16px; animation-delay: {i * 0.05}s;">
              <div style="display:flex; align-items:center; gap:16px;">
                <div style="width:50px; height:50px; border-radius:50%; background:linear-gradient(135deg, #FF6B35, #FFD700); display:flex; align-items:center; justify-content:center; font-weight:800; color:#1A0000; font-size:20px; box-shadow:0 4px 20px rgba(255,107,53,0.4);">
                  {person.name.charAt(0).toUpperCase()}
                </div>
                <div style="display:flex; flex-direction:column;">
                  <span style="font-size:22px; font-weight:800; color:#fff;">{person.name}</span>
                  <span style="font-size:13px; font-weight:700; color:#FF6B35; text-transform:uppercase; letter-spacing:0.05em;">Contributor</span>
                </div>
              </div>
              {#if person.contribution}
                <div style="background:rgba(255,60,30,0.06); border:1px solid rgba(255,100,50,0.15); border-radius:12px; padding:12px 16px;">
                  <p style="font-size:15px; color:rgba(255,200,150,0.8); line-height:1.5;">{person.contribution}</p>
                </div>
              {/if}
            </div>
          {/each}
        </div>
      </div>

    {:else if slide.type === 'metrics'}
      {@const d = slide.data as PreviousData}
      {@const sc = d.spillovers === 0 ? { bg:'rgba(255,100,50, 0.1)', border:'rgba(255,100,50, 0.25)', text:'#FF8C42' } : d.spillovers <= 8 ? { bg:'rgba(255, 160, 0, 0.1)', border:'rgba(255, 160, 0, 0.25)', text:'#FFB347' } : { bg:'rgba(255, 50, 20, 0.15)', border:'rgba(255, 50, 20, 0.3)', text:'#FF4444' }}
      <div style="height:100%; padding:40px 60px; display:flex; flex-direction:column; font-family:'Inter', sans-serif;">
        <div style="margin-bottom:40px;" class="fade-in-up">
          <h1 style="font-size:42px; font-weight:800; color:#fff; margin-bottom:8px; letter-spacing:-0.02em;">Sprint Review: Metrics</h1>
          <p style="font-size:18px; color:rgba(255,150,50,0.5);">Data from previous sprint</p>
        </div>
        <div style="display:grid; grid-template-columns:1.2fr 1fr 1fr; grid-template-rows:repeat(3, minmax(0, 1fr)); gap:24px; flex:1; margin-bottom:40px;">
          <div class="fire-card fade-in-up" style="grid-row: span 2; padding:32px; display:flex; flex-direction:column; position:relative; animation-delay: 0.1s;">
            <span style="font-size:12px; font-weight:700; color:#FF8C42; text-transform:uppercase; letter-spacing:0.05em; margin-bottom:auto;">Total Story Points Delivered</span>
            <div style="margin-bottom:12px;">
              <span style="font-size:120px; font-weight:900; color:#fff; line-height:1; letter-spacing:-0.04em;">{d.total_sp_delivered}</span> 
              <span style="font-size:24px; color:rgba(255,200,150,0.4); margin-left:8px;">points</span>
            </div>
          </div>
          <div class="fire-card fade-in-up" style="padding:24px; display:flex; flex-direction:column; justify-content:space-between; animation-delay: 0.2s;">
            <span style="font-size:12px; font-weight:700; color:rgba(255,200,150,0.6); text-transform:uppercase; letter-spacing:0.05em;">Total Epics Delivered</span>
            <div style="display:flex; align-items:baseline; gap:8px;">
              <span style="font-size:56px; font-weight:800; color:#fff;">{d.total_epics_delivered}</span>
              <span style="font-size:18px; color:rgba(255,200,150,0.4);">epics</span>
            </div>
          </div>
          <div class="fire-card fade-in-up" style="background:{sc.bg}; border:1px solid {sc.border}; border-radius:24px; backdrop-filter: blur(12px); padding:24px; display:flex; flex-direction:column; justify-content:space-between; position:relative; animation-delay: 0.3s;">
            <div style="display:flex; justify-content:space-between; align-items:flex-start;">
              <span style="font-size:12px; font-weight:700; color:{sc.text}; text-transform:uppercase; letter-spacing:0.05em;">Spillovers</span>
            </div>
            <div style="display:flex; align-items:baseline; gap:8px;">
              <span style="font-size:56px; font-weight:800; color:{sc.text};">{d.spillovers}</span>
              <span style="font-size:18px; color:{sc.text}; opacity:0.8;">pts</span>
            </div>
          </div>
          <div class="fire-card fade-in-up" style="grid-column: span 2; padding:24px; display:flex; flex-direction:column; justify-content:space-between; animation-delay: 0.4s;">
            <span style="font-size:12px; font-weight:700; color:rgba(255,200,150,0.6); text-transform:uppercase; letter-spacing:0.05em;">Total Work Logged</span>
            <div style="display:flex; align-items:center; justify-content:space-between;">
              <div style="display:flex; align-items:baseline; gap:8px;">
                <span style="font-size:56px; font-weight:800; color:#fff;">{d.total_hours_logged}</span>
                <span style="font-size:18px; color:rgba(255,200,150,0.4);">hours</span>
              </div>
              <div style="width:240px; height:12px; background:rgba(255,100,50,0.1); border-radius:6px; overflow:hidden; border:1px solid rgba(255,100,50,0.2);">
                <div style="width:75%; height:100%; background:linear-gradient(90deg, #FF6B35, #FFD700); border-radius:6px;"></div>
              </div>
            </div>
          </div>
          <div class="fire-card fade-in-up" style="padding:24px; display:flex; flex-direction:column; justify-content:space-between; animation-delay: 0.5s;">
            <span style="font-size:12px; font-weight:700; color:rgba(255,200,150,0.6); text-transform:uppercase; letter-spacing:0.05em;">Hrs / Story Point</span>
            <div style="display:flex; align-items:baseline; gap:8px;">
              <span style="font-size:56px; font-weight:800; color:#fff;">{d.avg_hours_per_sp.toFixed(1)}</span>
            </div>
          </div>
          <div class="fire-card fade-in-up" style="grid-column: span 2; padding:32px; display:flex; flex-direction:column; gap:20px; animation-delay: 0.6s;">
            <div style="display:flex; flex-direction:column; gap:8px;">
              <div style="display:flex; justify-content:space-between; font-size:12px; font-weight:700; color:rgba(255,200,150,0.5); text-transform:uppercase; letter-spacing:0.05em;">
                <span>Planned Capacity</span>
                <span style="color:#fff;">{d.planned_sp} pts</span>
              </div>
              <div style="width:100%; height:10px; background:rgba(255,100,50,0.08); border-radius:5px; overflow:hidden;">
                <div style="width:100%; height:100%; background:rgba(255,100,50,0.25);"></div>
              </div>
            </div>
            <div style="display:flex; flex-direction:column; gap:8px;">
              <div style="display:flex; justify-content:space-between; font-size:12px; font-weight:700; color:#FF8C42; text-transform:uppercase; letter-spacing:0.05em;">
                <span>Executed Capacity</span>
                <span style="color:#fff;">{d.executed_sp} pts</span>
              </div>
              <div style="width:100%; height:10px; background:rgba(255,100,50,0.08); border-radius:5px; overflow:hidden;">
                <div style="width:{(d.executed_sp / (d.planned_sp || 1)) * 100}%; height:100%; background:linear-gradient(90deg, #FF6B35, #FFD700);"></div>
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
            <div class="fire-card fade-in-up" style="padding:28px; display:flex; gap:24px; animation-delay: {i * 0.05}s;">
              <div style="width:56px; height:56px; border-radius:16px; flex-shrink:0; background:linear-gradient(135deg, #FF6B35, #FFD700); display:flex; align-items:center; justify-content:center; box-shadow:0 4px 16px rgba(255,107,53,0.3);">
                <span style="color:#1A0000; font-weight:900; font-size:20px;">{i+1}</span>
              </div>
              <div style="display:flex; flex-direction:column; gap:12px;">
                <h3 style="font-size:22px; font-weight:700; color:#fff; line-height:1.2;">{item.title || 'Entry'}</h3>
                <p style="font-size:16px; color:rgba(255,200,150,0.6); line-height:1.6;">{item.content}</p>
              </div>
            </div>
          {/each}
        </div>
      </div>

    {:else if slide.type === 'epic'}
      {@const e = slide.data as Epic}
      <div style="height:100%; display:grid; grid-template-columns: 1fr 380px; font-family:'Inter', sans-serif;">
        <div style="padding: 60px 80px; overflow-y: auto; display: flex; flex-direction: column; gap: 48px; border-right: 1px solid rgba(255,100,50,0.15);">
          <div class="fade-in-up">
            <h1 style="font-size: 68px; font-weight: 900; color: #FFC107; line-height: 1.1;">{e.title}</h1>
          </div>
          <div class="fade-in-up" style="animation-delay: 0.1s;">
            <h2 style="font-size: 26px; font-weight: 800; color: #FF8C42; margin-bottom: 20px;">Summary</h2>
            <p style="font-size: 22px; color: rgba(255,200,150,0.7); line-height: 1.6;">{e.summary}</p>
          </div>
          <div class="fade-in-up" style="animation-delay: 0.2s;">
            <h2 style="font-size: 26px; font-weight: 800; color: #FF8C42; margin-bottom: 24px;">Why we need it?</h2>
            <div style="display: flex; flex-direction: column; gap: 20px;">
              {#each (e.why_needed || '').split('\n').filter(Boolean) as line}
                <div class="fire-card" style="padding: 20px 24px; display: flex; gap: 20px; align-items: flex-start;">
                  <div style="width: 28px; height: 28px; background: rgba(255,107,53,0.2); border: 2px solid #FF6B35; border-radius: 50%; display: flex; align-items: center; justify-content: center; flex-shrink: 0; margin-top: 2px;">
                    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#FF6B35" stroke-width="4" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"></polyline></svg>
                  </div>
                  <p style="font-size: 19px; color: #FFC107; line-height: 1.5;">{line}</p>
                </div>
              {/each}
            </div>
          </div>
        </div>
        <div style="padding: 60px 40px; display: flex; flex-direction: column; gap: 56px;">
          <div class="fade-in-up" style="animation-delay: 0.3s;">
            <h2 style="font-size: 24px; font-weight: 800; color: #FF8C42; margin-bottom: 24px;">Audience</h2>
            <div style="display: flex; flex-direction: column; gap: 16px;">
              {#each (e.audience || '').split('\n').filter(Boolean) as aud}
                <div class="fire-card" style="padding: 18px 24px; display: flex; align-items: center; gap: 20px;">
                  <span style="font-size: 18px; font-weight: 700; color: #FFC107;">{aud}</span>
                </div>
              {/each}
            </div>
          </div>
          <div class="fade-in-up" style="animation-delay: 0.4s;">
            <h2 style="font-size: 24px; font-weight: 800; color: #FF8C42; margin-bottom: 24px;">Timeline</h2>
            <div class="fire-card" style="padding: 28px; position: relative; overflow: hidden;">
              <div style="position: absolute; left: 0; top: 0; bottom: 0; width: 6px; background: linear-gradient(180deg, #FFD700, #FF6B35, #FF3333);"></div>
              <div style="display: flex; flex-direction: column; gap: 6px;">
                <span style="font-size: 11px; font-weight: 800; color: rgba(255,200,150,0.4); text-transform: uppercase; letter-spacing: 0.1em;">Target Completion</span>
                <span style="font-size: 22px; font-weight: 900; color: #FF8C42;">{e.when_doing || 'TBD'}</span>
                {#if e.total_sp}
                  <div style="margin-top: 16px; display: flex; align-items: center; gap: 8px; color: rgba(255,200,150,0.5); font-size: 16px; font-weight: 600;">
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
            <div class="fire-card fade-in-up" style="padding:32px; display:flex; flex-direction:column; gap:24px;">
              <p style="font-size:22px; color:#fff; line-height:1.6;">{item}</p>
            </div>
          {/each}
        </div>
        <div class="fade-in-up" style="display:flex; gap:16px; background:rgba(255,60,30,0.08); border:1px solid rgba(255,100,50,0.15); border-radius:20px; padding:12px;">
          <input class="input" bind:value={newFeedback} placeholder="Share thoughts live…" onkeydown={e => e.key === 'Enter' && addFeedbackItem()} style="flex:1; background:transparent; color:#fff; border:none;" />
          <button class="btn btn-primary" onclick={addFeedbackItem} disabled={addingFeedback}>Post</button>
        </div>
      </div>

    {:else if slide.type === 'closing'}
      <div class="slide-content">
        <h1 class="fade-in-up" style="font-size:clamp(40px, 8vw, 100px); font-weight:900; color:#FFC107;">
          {slide.data.closing_text || (slide.data.isRetro ? 'Thank You!' : "Let's Ship It!")}
        </h1>
        <p class="fade-in-up" style="color:rgba(255,150,50,0.5); font-size:clamp(20px, 4vw, 36px);">{slide.data.sprint}</p>
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
    background: #080000;
    z-index: 0;
  }

  .flame {
    position: absolute;
    border-radius: 50%;
    filter: blur(80px);
    opacity: 0.35;
    mix-blend-mode: screen;
  }

  .flame-1 {
    width: 70vmax;
    height: 70vmax;
    background: radial-gradient(circle, #FF2200 0%, transparent 70%);
    bottom: -20%;
    left: -10%;
    animation: fire-rise 14s infinite alternate ease-in-out;
  }

  .flame-2 {
    width: 50vmax;
    height: 50vmax;
    background: radial-gradient(circle, #FF6600 0%, transparent 70%);
    bottom: -10%;
    right: -5%;
    animation: fire-rise 18s infinite alternate ease-in-out;
    animation-delay: -3s;
  }

  .flame-3 {
    width: 40vmax;
    height: 40vmax;
    background: radial-gradient(circle, #FFAA00 0%, transparent 70%);
    bottom: -15%;
    left: 30%;
    animation: fire-rise 12s infinite alternate ease-in-out;
    animation-delay: -5s;
  }

  .flame-4 {
    width: 60vmax;
    height: 60vmax;
    background: radial-gradient(circle, #FF4400 0%, transparent 70%);
    bottom: -30%;
    left: 50%;
    animation: fire-rise 16s infinite alternate ease-in-out;
    animation-delay: -7s;
  }

  .flame-5 {
    width: 45vmax;
    height: 45vmax;
    background: radial-gradient(circle, #FFD700 0%, transparent 70%);
    bottom: -5%;
    right: 20%;
    animation: fire-rise 20s infinite alternate ease-in-out;
    animation-delay: -9s;
  }

  @keyframes fire-rise {
    0% { transform: translate(0, 0) scale(1) rotate(0deg); opacity: 0.3; }
    25% { transform: translate(-10%, -15%) scale(1.15) rotate(10deg); opacity: 0.4; }
    50% { transform: translate(15%, -25%) scale(0.9) rotate(-15deg); opacity: 0.35; }
    75% { transform: translate(-5%, -35%) scale(1.1) rotate(5deg); opacity: 0.45; }
    100% { transform: translate(10%, -10%) scale(1) rotate(-5deg); opacity: 0.3; }
  }

  .ember {
    position: absolute;
    width: 4px;
    height: 4px;
    border-radius: 50%;
    background: #FFAA00;
    box-shadow: 0 0 6px 2px rgba(255, 170, 0, 0.6);
    opacity: 0;
    animation: ember-float 6s infinite ease-out;
  }

  .ember-1 { left: 15%; bottom: 10%; animation-delay: 0s; width: 3px; height: 3px; }
  .ember-2 { left: 35%; bottom: 5%; animation-delay: 1.2s; width: 5px; height: 5px; background: #FF6600; }
  .ember-3 { left: 55%; bottom: 15%; animation-delay: 2.5s; width: 3px; height: 3px; background: #FFD700; }
  .ember-4 { left: 75%; bottom: 8%; animation-delay: 3.8s; width: 4px; height: 4px; }
  .ember-5 { left: 25%; bottom: 20%; animation-delay: 0.6s; width: 2px; height: 2px; background: #FFD700; }
  .ember-6 { left: 65%; bottom: 12%; animation-delay: 4.5s; width: 4px; height: 4px; background: #FF4400; }
  .ember-7 { left: 45%; bottom: 3%; animation-delay: 2s; width: 3px; height: 3px; }
  .ember-8 { left: 85%; bottom: 18%; animation-delay: 5.2s; width: 3px; height: 3px; background: #FF6600; }

  @keyframes ember-float {
    0% { transform: translateY(0) scale(1); opacity: 0; }
    10% { opacity: 0.8; }
    50% { transform: translateY(-200px) translateX(30px) scale(0.6); opacity: 0.6; }
    80% { opacity: 0.3; }
    100% { transform: translateY(-400px) translateX(-20px) scale(0); opacity: 0; }
  }

  .slide-content { position:relative; z-index:2; display:flex; flex-direction:column; align-items:center; justify-content:center; height:100%; text-align:center; padding:60px; }
  .fade-in-up { animation: fadeInUp 0.8s forwards; opacity:0; }
  @keyframes fadeInUp { from { opacity:0; transform:translateY(30px); } to { opacity:1; transform:translateY(0); } }
  .fire-tag { background:rgba(255,60,30,0.1); backdrop-filter:blur(10px); border:1px solid rgba(255,100,50,0.2); padding:8px 16px; border-radius:100px; font-size:12px; font-weight:600; text-transform:uppercase; color:#FF8C42; margin-bottom:24px; }
  
  .fire-card { 
    background: rgba(255, 60, 30, 0.04); 
    backdrop-filter: blur(20px); 
    border: 1px solid rgba(255, 100, 50, 0.12); 
    border-radius: 24px; 
    position: relative; 
    transition: transform 300ms ease, border-color 300ms ease, box-shadow 300ms ease;
  }
  .fire-card::before {
    content: '';
    position: absolute;
    inset: -2px;
    padding: 2px;
    border-radius: 24px;
    background: linear-gradient(135deg, #FF6B35, #FFD700, #FF3333);
    -webkit-mask: linear-gradient(#fff 0 0) content-box, linear-gradient(#fff 0 0);
    mask: linear-gradient(#fff 0 0) content-box, linear-gradient(#fff 0 0);
    -webkit-mask-composite: xor;
    mask-composite: exclude;
    opacity: 0;
    transition: opacity 0.5s ease;
    pointer-events: none;
  }
  .fire-card:hover {
    border-color: rgba(255, 150, 50, 0.25);
    transform: translateY(-4px);
    box-shadow: 0 8px 32px rgba(255, 100, 50, 0.15);
  }
  .fire-card:hover::before {
    opacity: 1;
  }
</style>
