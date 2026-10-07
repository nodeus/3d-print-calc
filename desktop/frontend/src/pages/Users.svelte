<script lang="ts">
  import { onMount } from 'svelte';
  import { wails, type UserDTO } from '../lib/wails';
  import { t } from '../lib/i18n';
  import { downloadPDF, registerPdfExporter } from '../lib/pdf';

  let list: UserDTO[] = $state([]);
  let login = $state('');
  let password = $state('');
  let role = $state('manager');
  let error = $state('');

  async function load() {
    try {
      list = (await wails.listUsers()) ?? [];
    } catch (e) {
      error = String(e);
    }
  }
  onMount(() => {
    load();
    registerPdfExporter(toPDF);
    return () => registerPdfExporter(null);
  });

  async function create() {
    error = '';
    try {
      await wails.createUser(login, password, role);
      login = '';
      password = '';
      await load();
    } catch (e) {
      error = String(e);
    }
  }

  async function setRole(id: number, r: string) {
    try {
      await wails.setUserRole(id, r);
      await load();
    } catch (e) {
      error = String(e);
    }
  }

  async function remove(u: UserDTO) {
    error = '';
    if (!confirm(`${u.login}?`)) return;
    try {
      await wails.deleteUser(u.id);
      await load();
    } catch (e) {
      error = String(e);
    }
  }

  async function toPDF() {
    await downloadPDF($t('nav.users'), [
      {
        Title: '',
        Headers: [$t('auth.login'), $t('users.role')],
        Rows: list.map((u) => [u.login, u.role]),
      },
    ]);
  }
</script>

<div class="row">
  <input placeholder={$t('auth.login')} bind:value={login} />
  <input type="password" placeholder={$t('auth.password')} bind:value={password} />
  <select bind:value={role}>
    <option value="admin">admin</option>
    <option value="manager">manager</option>
    <option value="viewer">viewer</option>
  </select>
  <button onclick={create}>{$t('users.add')}</button>
</div>
{#if error}<p class="err">{error}</p>{/if}
<table>
  <thead><tr><th>{$t('auth.login')}</th><th>{$t('users.role')}</th><th class="no-print"></th></tr></thead>
  <tbody>
    {#each list as u}
      <tr>
        <td>{u.login}</td>
        <td>
          <select value={u.role} onchange={(e) => setRole(u.id, e.currentTarget.value)}>
            <option value="admin">admin</option>
            <option value="manager">manager</option>
            <option value="viewer">viewer</option>
          </select>
        </td>
        <td class="no-print"><button onclick={() => remove(u)}>✕</button></td>
      </tr>
    {/each}
  </tbody>
</table>

<style>
  .row { display: flex; gap: 8px; margin-bottom: 10px; }
  input, select, button { padding: 6px; font-size: 14px; }
  table { font-size: 14px; }
  .err { color: #b00; }
</style>
