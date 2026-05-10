<script lang="ts">
  import { api } from '$lib/api';
  import { checkUser } from '$lib/auth';
  import { goto } from '$app/navigation';

  let username = $state('');
  let password = $state('');
  let error = $state('');
  let loading = $state(false);

  async function handleSubmit(e: Event) {
    e.preventDefault();
    loading = true;
    error = '';
    try {
      await api.auth.login({ username, password });
      await checkUser();
      goto('/');
    } catch (err: any) {
      error = err.message;
    } finally {
      loading = false;
    }
  }
</script>

<div class="auth-page fade-in">
  <div class="card" style="width: 100%; max-width: 400px;">
    <div class="card-header">
      <h2 class="text-xl font-bold">Login</h2>
    </div>
    <div class="card-body">
      <form onsubmit={handleSubmit} class="space-y-4">
        <div>
          <label for="username" class="label">Username</label>
          <input
            type="text"
            id="username"
            bind:value={username}
            class="input"
            required
            placeholder="Enter username"
          />
        </div>
        <div>
          <label for="password" class="label">Password</label>
          <input
            type="password"
            id="password"
            bind:value={password}
            class="input"
            required
            placeholder="Enter password"
          />
        </div>
        {#if error}
          <div class="text-error text-sm">{error}</div>
        {/if}
        <button type="submit" class="btn btn-primary w-full" disabled={loading}>
          {loading ? 'Logging in...' : 'Login'}
        </button>
      </form>
    </div>
    <div class="card-footer text-center">
      <p class="text-sm text-muted">
        Don't have an account? <a href="/register" class="link">Register</a>
      </p>
    </div>
  </div>
</div>

<style>
  .auth-page {
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: calc(100vh - 100px); /* Adjust based on navbar height */
    padding: 20px;
  }
</style>
