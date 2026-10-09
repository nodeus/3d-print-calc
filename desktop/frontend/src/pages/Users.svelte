<script lang="ts">
  import { onMount } from 'svelte';
  import { wails, type UserDTO } from '../lib/wails';
  import { t } from '../lib/i18n';
  import { askConfirm } from '../lib/confirm';
  import { downloadPDF, registerPdfExporter } from '../lib/pdf';
  import { thWidth, startColResize, resetColWidth } from '../lib/colwidths';

  let list: UserDTO[] = $state([]);
  let sortKey = $state('login');
  let sortDir: 1 | -1 = $state(1);

  function toggleSort(key: string) {
    if (sortKey === key) sortDir = sortDir === 1 ? -1 : 1;
    else { sortKey = key; sortDir = 1; }
  }

  function arrow(key: string): string {
    return sortKey === key ? (sortDir === 1 ? ' ▲' : ' ▼') : '';
  }

  let shown = $derived([...list].sort((a, b) => {
    const va = sortKey === 'role' ? a.role : a.login.toLowerCase();
    const vb = sortKey === 'role' ? b.role : b.login.toLowerCase();
    return va.localeCompare(vb) * sortDir;
  }));
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
    if (!(await askConfirm(`${$t("common.delete_q")} «${u.login}»?`))) return;
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
<table class="tw">
  <thead><tr>
    {#each [{ key: 'login', label: 'auth.login' }, { key: 'role', label: 'users.role' }] as c}
      <th style={thWidth('users', c.key)}>
        <button class="th" onclick={() => toggleSort(c.key)}>{$t(c.label)}{arrow(c.key)}</button>
        <span class="rz no-print" role="separator" onmousedown={(e) => startColResize(e, 'users', c.key)} ondblclick={() => resetColWidth('users', c.key)}></span>
      </th>
    {/each}
    <th class="no-print"></th>
  </tr></thead>
  <tbody>
    {#each shown as u}
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
  button.th { border: none; background: none; font-weight: bold; padding: 0; cursor: pointer; font-size: 14px; }
  .err { color: var(--err); }
</style>
