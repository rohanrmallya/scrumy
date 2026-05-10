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

<div class="slide" style="height:100%; background:#FFFFFF; color: #111827; font-family: 'Inter', sans-serif;">
  <div style="height:100%; padding:60px 80px; display:flex; flex-direction:column;">
    
    {#if slide.type === 'title'}
      <div style="flex:1; display:flex; flex-direction:column; justify-content:center;">
        <div style="color: #4F46E5; font-weight: 700; text-transform: uppercase; letter-spacing: 0.1em; margin-bottom: 20px;">
          {slide.data.isRetro ? 'Retrospective' : 'Introduction'}
        </div>
        <h1 style="font-size: 80px; font-weight: 800; line-height: 1; letter-spacing: -0.02em; margin-bottom: 24px;">{slide.data.title}</h1>
        <div style="font-size: 32px; color: #6B7280;">{slide.data.sprint}</div>
      </div>

    {:else if slide.type === 'contributors'}
      <h2 style="font-size: 40px; font-weight: 800; margin-bottom: 48px; border-bottom: 2px solid #E5E7EB; padding-bottom: 20px;">Team Contributions</h2>
      <div style="flex:1; overflow-y:auto; display:grid; grid-template-columns: 1fr 1fr; gap:32px;">
        {#each slide.data.items as person}
          <div style="padding: 24px; border-left: 4px solid #4F46E5; background: #F9FAFB;">
            <div style="font-size: 24px; font-weight: 700; margin-bottom: 8px;">{person.name}</div>
            <div style="color: #4B5563; line-height: 1.5;">{person.contribution}</div>
          </div>
        {/each}
      </div>

    {:else if slide.type === 'metrics'}
      {@const d = slide.data as PreviousData}
      <h2 style="font-size: 40px; font-weight: 800; margin-bottom: 48px;">Sprint Metrics</h2>
      <div style="flex:1; display:grid; grid-template-columns: repeat(3, 1fr); gap:40px;">
        <div class="metric-box">
          <label>Story Points</label>
          <div class="value">{d.total_sp_delivered}</div>
        </div>
        <div class="metric-box">
          <label>Epics</label>
          <div class="value">{d.total_epics_delivered}</div>
        </div>
        <div class="metric-box">
          <label>Spillovers</label>
          <div class="value" style="color: {d.spillovers > 0 ? '#DC2626' : 'inherit'}">{d.spillovers}</div>
        </div>
        <div class="metric-box">
          <label>Work Hours</label>
          <div class="value">{d.total_hours_logged}</div>
        </div>
        <div class="metric-box">
          <label>Avg SP/Hr</label>
          <div class="value">{d.avg_hours_per_sp.toFixed(1)}</div>
        </div>
        <div class="metric-box">
          <label>Execution</label>
          <div class="value">{Math.round((d.executed_sp/d.planned_sp)*100)}%</div>
        </div>
      </div>

    {:else if slide.type === 'learnings' || slide.type === 'changes'}
      <h2 style="font-size: 40px; font-weight: 800; margin-bottom: 48px;">{slide.type === 'learnings' ? 'Key Learnings' : 'Process Changes'}</h2>
      <div style="flex:1; overflow-y:auto; display:flex; flex-direction:column; gap:24px;">
        {#each slide.data.items as item}
          <div style="display:flex; gap:32px; align-items:baseline;">
            <div style="font-size: 24px; font-weight: 800; color: #E5E7EB;">•</div>
            <div>
              <h3 style="font-size: 28px; font-weight: 700; margin-bottom: 8px;">{item.title}</h3>
              <p style="font-size: 20px; color: #4B5563; line-height: 1.6;">{item.content}</p>
            </div>
          </div>
        {/each}
      </div>

    {:else if slide.type === 'epic'}
      {@const e = slide.data as Epic}
      <div style="margin-bottom: 40px;">
        <div style="color: #4F46E5; font-weight: 700; margin-bottom: 8px;">{e.id}</div>
        <h2 style="font-size: 56px; font-weight: 800;">{e.title}</h2>
      </div>
      <div style="flex:1; display:grid; grid-template-columns: 1.5fr 1fr; gap:60px;">
        <div>
          <div class="minimal-section">
            <h4>Summary</h4>
            <p>{e.summary}</p>
          </div>
          <div class="minimal-section">
            <h4>Why it matters</h4>
            <p>{e.why_needed}</p>
          </div>
        </div>
        <div style="background: #F9FAFB; padding: 40px;">
          <div class="minimal-section">
            <h4>Target</h4>
            <p>{e.audience}</p>
          </div>
          <div class="minimal-section">
            <h4>Timeline</h4>
            <p>{e.when_doing}</p>
          </div>
        </div>
      </div>

    {:else if slide.type === 'retro-feedback'}
      <h2 style="font-size: 40px; font-weight: 800; margin-bottom: 48px;">Team Feedback</h2>
      <div style="flex:1; overflow-y:auto; display:flex; flex-direction:column; gap:16px;">
        {#each slide.data.items as item}
          <div style="padding: 20px; border: 1px solid #E5E7EB; border-radius: 8px;">{item}</div>
        {/each}
      </div>
      <div style="margin-top: 32px; display:flex; gap:12px;">
        <input class="input" bind:value={newFeedback} placeholder="Add feedback..." style="background:#F3F4F6; border:none;"/>
        <button class="btn btn-primary" onclick={addFeedbackItem}>Add</button>
      </div>

    {:else if slide.type === 'closing'}
      <div style="flex:1; display:flex; flex-direction:column; justify-content:center; text-align:center;">
        <h1 style="font-size: 72px; font-weight: 800; margin-bottom: 16px;">{slide.data.closing_text || "Thank You"}</h1>
        <p style="font-size: 24px; color: #6B7280;">{slide.data.sprint}</p>
      </div>
    {/if}
  </div>
</div>

<style>
  .metric-box label { display: block; font-size: 14px; font-weight: 600; color: #6B7280; text-transform: uppercase; margin-bottom: 8px; }
  .metric-box .value { font-size: 64px; font-weight: 800; }
  .minimal-section { margin-bottom: 32px; }
  .minimal-section h4 { font-size: 16px; font-weight: 700; text-transform: uppercase; color: #4F46E5; margin-bottom: 12px; }
  .minimal-section p { font-size: 20px; line-height: 1.5; color: #374151; }
</style>
