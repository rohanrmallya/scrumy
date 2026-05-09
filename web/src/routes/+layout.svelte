<script lang="ts">
  import '../app.css';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';

  let { children } = $props();
</script>

<div class="page-shell">
  <nav class="navbar">
    <a href="/" class="navbar-brand">
      <div class="logo-icon">S</div>
      Scrumy
    </a>
    <div style="flex:1" />
    {#if $page.url.pathname !== '/'}
      <a href="/" class="btn btn-ghost btn-sm">← All Plans</a>
    {/if}
  </nav>
  <main style="flex:1">
    {@render children()}
  </main>
</div>
