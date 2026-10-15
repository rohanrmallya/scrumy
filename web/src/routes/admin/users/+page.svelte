<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { api, type ManagedUser } from '$lib/api';
  import { user } from '$lib/auth';

  let users = $state<ManagedUser[]>([]);
  let loading = $state(true);
  let error = $state('');
  let notice = $state('');

  // Create form
  let newUsername = $state('');
  let newPassword = $state('');
  let newRole = $state('user');
  let creating = $state(false);

  // Reset-password inline state: userID -> new password
  let resetPasswords: Record<string, string> = $state({});
  let resetting: Record<string, boolean> = $state({});

  async function load() {
    loading = true;
    error = '';
    try {
      users = await api.users.list();
    } catch (e: any) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  onMount(load);

  async function createUser(e: Event) {
    e.preventDefault();
    error = '';
    notice = '';
    if (!newUsername.trim() || !newPassword) return;
    creating = true;
    try {
      const u = await api.users.create({ username: newUsername.trim(), password: newPassword, role: newRole });
      users = [...users, u].sort((a, b) => a.username.localeCompare(b.username));
      newUsername = '';
      newPassword = '';
      newRole = 'user';
      notice = `User "${u.username}" created`;
    } catch (e: any) {
      error = e.message;
    } finally {
      creating = false;
    }
  }

  async function resetPassword(id: string) {
    const pw = (resetPasswords[id] || '').trim();
    error = '';
    notice = '';
    if (!pw) {
      error = 'Enter a new password first';
      return;
    }
    resetting[id] = true;
    try {
      await api.users.resetPassword(id, pw);
      resetPasswords[id] = '';
      notice = 'Password updated — user sessions were signed out';
    } catch (e: any) {
      error = e.message;
    } finally {
      resetting[id] = false;
    }
  }

  async function changeRole(u: ManagedUser, role: string) {
    if (u.role === role) return;
    error = '';
    notice = '';
    try {
      const updated = await api.users.updateRole(u.id, role);
      users = users.map(x => x.id === u.id ? updated : x);
      notice = `Role for "${u.username}" set to ${role}`;
    } catch (e: any) {
      error = e.message;
    }
  }

  async function deleteUser(u: ManagedUser) {
    if (!confirm(`Delete user "${u.username}"?`)) return;
    error = '';
    notice = '';
    try {
      await api.users.delete(u.id);
      users = users.filter(x => x.id !== u.id);
      notice = `User "${u.username}" deleted`;
    } catch (e: any) {
      error = e.message;
    }
  }
</script>

<svelte:head>
  <title>User admin – Scrumy</title>
</svelte:head>

<div class="page-content fade-in">
  <div class="flex items-center justify-between" style="margin-bottom:20px;">
    <div>
      <h1 class="text-2xl font-bold">User admin</h1>
      <p class="text-sm text-muted" style="margin-top:2px;">Super admin only — add users and reset passwords.</p>
    </div>
    <button class="btn btn-secondary btn-sm" onclick={() => goto('/')}>← All Plans</button>
  </div>

  {#if $user && $user.role !== 'admin'}
    <div class="badge badge-danger" style="padding:12px 20px; border-radius:8px; font-size:14px; text-transform:none; letter-spacing:0; font-weight:400;">
      ⚠️ Super admin access required.
    </div>
  {:else}
    {#if error}
      <div class="badge badge-danger" style="margin-bottom:16px; padding:8px 16px; border-radius:8px; font-size:13px; text-transform:none; letter-spacing:0; font-weight:400;">
        ⚠️ {error}
      </div>
    {/if}
    {#if notice}
      <div class="badge badge-success" style="margin-bottom:16px; padding:8px 16px; border-radius:8px; font-size:13px; text-transform:none; letter-spacing:0; font-weight:400;">
        ✓ {notice}
      </div>
    {/if}

    <div class="card" style="margin-bottom:20px;">
      <div class="card-header">
        <h2 class="font-semibold" style="font-size:15px;">Add user</h2>
      </div>
      <div class="card-body">
        <form onsubmit={createUser} class="flex gap-2" style="flex-wrap:wrap; align-items:flex-end;">
          <div class="form-group" style="min-width:180px; flex:1;">
            <label class="label" for="nu">Username</label>
            <input id="nu" class="input" bind:value={newUsername} placeholder="e.g. jane" required />
          </div>
          <div class="form-group" style="min-width:180px; flex:1;">
            <label class="label" for="np">Password</label>
            <input id="np" type="password" class="input" bind:value={newPassword} placeholder="Min 4 characters" required autocomplete="new-password" />
          </div>
          <div class="form-group">
            <label class="label" for="nr">Role</label>
            <select id="nr" class="input" bind:value={newRole}>
              <option value="user">user</option>
              <option value="admin">admin</option>
            </select>
          </div>
          <button type="submit" class="btn btn-primary btn-sm" disabled={creating}>
            {creating ? 'Adding…' : 'Add user'}
          </button>
        </form>
      </div>
    </div>

    <div class="card">
      <div class="card-header">
        <h2 class="font-semibold" style="font-size:15px;">Users ({users.length})</h2>
      </div>
      <div class="card-body" style="padding:0;">
        {#if loading}
          <div class="flex items-center gap-3" style="padding:24px;">
            <span class="spinner"></span>
            <span class="text-muted">Loading users…</span>
          </div>
        {:else if users.length === 0}
          <div class="empty-state"><p>No users found.</p></div>
        {:else}
          <table class="table">
            <thead>
              <tr>
                <th>Username</th>
                <th>Role</th>
                <th>Created</th>
                <th>Set new password</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {#each users as u (u.id)}
                <tr>
                  <td class="font-medium">
                    {u.username}
                    {#if u.is_super_admin}
                      <span class="badge badge-primary" style="margin-left:8px;">super admin</span>
                    {/if}
                    {#if $user && $user.id === u.id}
                      <span class="badge badge-default" style="margin-left:8px;">you</span>
                    {/if}
                  </td>
                  <td>
                    <select
                      class="input"
                      style="width:auto; padding:4px 8px; font-size:12px;"
                      value={u.role}
                      disabled={u.is_super_admin || ($user && $user.id === u.id)}
                      onchange={(e) => changeRole(u, e.currentTarget.value)}
                    >
                      <option value="user">user</option>
                      <option value="admin">admin</option>
                    </select>
                  </td>
                  <td class="text-sm text-muted">{u.created_at ? new Date(u.created_at).toLocaleDateString() : '—'}</td>
                  <td>
                    <div class="flex gap-2">
                      <input
                        type="password"
                        class="input"
                        style="max-width:160px; padding:4px 8px; font-size:12px;"
                        placeholder="New password"
                        value={resetPasswords[u.id] || ''}
                        oninput={(e) => resetPasswords[u.id] = e.currentTarget.value}
                        autocomplete="new-password"
                      />
                      <button
                        class="btn btn-secondary btn-sm"
                        onclick={() => resetPassword(u.id)}
                        disabled={!!resetting[u.id]}
                      >
                        {resetting[u.id] ? 'Saving…' : 'Set'}
                      </button>
                    </div>
                  </td>
                  <td style="text-align:right;">
                    <button
                      class="btn btn-danger btn-sm"
                      onclick={() => deleteUser(u)}
                      disabled={u.is_super_admin || ($user && $user.id === u.id)}
                      title={u.is_super_admin ? 'Cannot delete the super admin account' : 'Delete user'}
                    >
                      Delete
                    </button>
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        {/if}
      </div>
    </div>
  {/if}
</div>
