<script lang="ts">
  import { api } from '$lib/api';
  import { user } from '$lib/auth';
  import { goto } from '$app/navigation';

  let currentPassword = $state('');
  let newPassword = $state('');
  let confirmPassword = $state('');
  let error = $state('');
  let success = $state('');
  let loading = $state(false);

  async function handleSubmit(e: Event) {
    e.preventDefault();
    error = '';
    success = '';
    if (newPassword !== confirmPassword) {
      error = 'New passwords do not match';
      return;
    }
    if (newPassword.length < 4) {
      error = 'New password must be at least 4 characters';
      return;
    }
    loading = true;
    try {
      await api.auth.changePassword({ current_password: currentPassword, new_password: newPassword });
      success = 'Password changed successfully';
      currentPassword = '';
      newPassword = '';
      confirmPassword = '';
    } catch (err: any) {
      error = err.message;
    } finally {
      loading = false;
    }
  }
</script>

<svelte:head>
  <title>Change password – Scrumy</title>
</svelte:head>

<div class="auth-page fade-in">
  <div class="card" style="width: 100%; max-width: 400px;">
    <div class="card-header">
      <h2 class="text-xl font-bold">Change password</h2>
    </div>
    <div class="card-body">
      {#if !$user}
        <p class="text-sm text-muted">
          Please <a href="/login" class="link">log in</a> to change your password.
        </p>
      {:else}
        <form onsubmit={handleSubmit} class="space-y-4">
          <div>
            <label for="current" class="label">Current password</label>
            <input
              type="password"
              id="current"
              bind:value={currentPassword}
              class="input"
              required
              placeholder="Enter current password"
              autocomplete="current-password"
            />
          </div>
          <div>
            <label for="new" class="label">New password</label>
            <input
              type="password"
              id="new"
              bind:value={newPassword}
              class="input"
              required
              placeholder="Enter new password"
              autocomplete="new-password"
            />
          </div>
          <div>
            <label for="confirm" class="label">Confirm new password</label>
            <input
              type="password"
              id="confirm"
              bind:value={confirmPassword}
              class="input"
              required
              placeholder="Confirm new password"
              autocomplete="new-password"
            />
          </div>
          {#if error}
            <div class="text-error text-sm">{error}</div>
          {/if}
          {#if success}
            <div class="text-sm" style="color: var(--c-success);">{success}</div>
          {/if}
          <button type="submit" class="btn btn-primary w-full" disabled={loading}>
            {loading ? 'Updating…' : 'Update password'}
          </button>
        </form>
      {/if}
    </div>
    <div class="card-footer text-center">
      <button class="btn btn-ghost btn-sm" onclick={() => goto('/')}>← Back</button>
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
