<script lang="ts">
  import { api } from '$lib/api';
  import { goto } from '$app/navigation';

  let username = $state('');
  let password = $state('');
  let confirmPassword = $state('');
  let error = $state('');
  let loading = $state(false);

  async function handleSubmit(e: Event) {
    e.preventDefault();
    if (password !== confirmPassword) {
      error = 'Passwords do not match';
      return;
    }
    loading = true;
    error = '';
    try {
      await api.auth.register({ username, password });
      goto('/login');
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
      <h2 class="text-xl font-bold">Register</h2>
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
            placeholder="Choose a username"
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
            placeholder="Choose a password"
          />
        </div>
        <div>
          <label for="confirmPassword" class="label">Confirm Password</label>
          <input
            type="password"
            id="confirmPassword"
            bind:value={confirmPassword}
            class="input"
            required
            placeholder="Confirm your password"
          />
        </div>
        {#if error}
          <div class="text-error text-sm">{error}</div>
        {/if}
        <button type="submit" class="btn btn-primary w-full" disabled={loading}>
          {loading ? 'Creating account...' : 'Register'}
        </button>
      </form>
    </div>
    <div class="card-footer text-center">
      <p class="text-sm text-muted">
        Already have an account? <a href="/login" class="link">Login</a>
      </p>
    </div>
  </div>
</div>

<style>
  .auth-page {
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: calc(100vh - 100px);
    padding: 20px;
  }
</style>
