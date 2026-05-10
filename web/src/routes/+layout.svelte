<script lang="ts">
  import '../app.css';
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { user, checkUser, logout } from '$lib/auth';

  let { children } = $props();

  onMount(() => {
    checkUser();
  });

  async function handleLogout() {
    await logout();
    window.location.href = '/';
  }
</script>

<div class="page-shell">
  <nav class="navbar">
    <a href="/" class="navbar-brand">
      <div class="logo-icon">S</div>
      Scrumy
    </a>
    <div style="flex:1" />
    
    {#if $user}
      <span class="navbar-text mr-4">Hi, {$user.username}</span>
      <button class="btn btn-ghost btn-sm" onclick={handleLogout}>Logout</button>
    {:else}
      <a href="/login" class="btn btn-ghost btn-sm">Login</a>
    {/if}

    {#if $page.url.pathname !== '/'}
      <div class="navbar-divider"></div>
      <a href="/" class="btn btn-ghost btn-sm">← All Plans</a>
    {/if}
  </nav>
  <main style="flex:1">
    {@render children()}
  </main>
</div>

<style>
  .navbar-text {
    font-size: 0.9rem;
    color: var(--text-muted);
    margin-right: 1rem;
  }
  .navbar-divider {
    width: 1px;
    height: 1.5rem;
    background: var(--border-color);
    margin: 0 0.5rem;
  }
  .mr-4 {
    margin-right: 1rem;
  }
</style>
