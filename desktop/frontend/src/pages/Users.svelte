<script lang="ts">
  import { onMount } from 'svelte';
  import { wails, type UserDTO } from '../lib/wails';
  import { t } from '../lib/i18n';
  import { askConfirm } from '../lib/confirm';
  import { notify } from '../lib/notice';
  import { downloadPDF, registerPdfExporter } from '../lib/pdf';
  import { thWidth, startColResize, resetColWidth } from '../lib/colwidths';
  import { mySettings } from '../lib/usersettings';
  import { orderFor, saveTableOrder } from '../lib/colorder';
  import { startColDrag } from '../lib/coldrag';
  import { get } from 'svelte/store';
  import { user, canWrite } from '../lib/session';

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

  const userKeys = ['login', 'role'];
  const userLabel = (k: string) => (k === 'role' ? 'users.role' : 'auth.login');

  let userOrd = $derived(orderFor($mySettings.colOrder, 'users', userKeys));

  let dragKey: string | null = $state(null);
  let dropX: number | null = $state(null);
  let wrapEl: HTMLDivElement | null = $state(null);

  async function dropCols(next: string[]) {
    error = '';
    try {
      await saveTableOrder('users', next);
    } catch (e) {
      error = String(e);
    }
  }
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
      notify($t('common.saved'));
    } catch (e) {
      error = String(e);
    }
  }

  async function setRole(id: number, r: string) {
    try {
      await wails.setUserRole(id, r);
      await load();
      notify($t('common.saved'));
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
        Headers: userOrd.map((k) => $t(userLabel(k))),
        Rows: shown.map((u) => userOrd.map((k) => (k === 'role' ? u.role : u.login))),
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
<div class="twrap" bind:this={wrapEl}>
{#if dropX != null}<div class="drop-line no-print" style="left:{dropX}px;"></div>{/if}
<table class="tw" class:dragging={dragKey != null}>
  <thead><tr>
    {#each userOrd as k}
      <th style={thWidth('users', k)} data-col={k} class:dragsrc={dragKey === k}>
        {#if canWrite($user)}<span class="grip no-print" role="separator" title={$t('hist.drag_cols')} onpointerdown={(e) => { if (canWrite($user)) startColDrag(e, wrapEl, k, userOrd, (dk, x) => { dragKey = dk; dropX = x; }, dropCols); }}>⋮⋮</span>{/if}
        <button class="th" onclick={() => toggleSort(k)}>{$t(userLabel(k))}{arrow(k)}</button>
        <span class="rz no-print" role="separator" onmousedown={(e) => startColResize(e, 'users', k)} ondblclick={() => resetColWidth('users', k)}></span>
      </th>
    {/each}
    <th class="no-print"></th>
  </tr></thead>
  <tbody>
    {#each shown as u}
      <tr>
        {#each userOrd as k}
          {#if k === 'role'}
            <td>
              <select value={u.role} onchange={(e) => setRole(u.id, e.currentTarget.value)}>
                <option value="admin">admin</option>
                <option value="manager">manager</option>
                <option value="viewer">viewer</option>
              </select>
            </td>
          {:else}
            <td>{u.login}</td>
          {/if}
        {/each}
        <td class="no-print"><button onclick={() => remove(u)} aria-label={$t('common.delete_q')}>✕</button></td>
      </tr>
    {/each}
  </tbody>
</table>
</div>

<style>
  .row { display: flex; gap: 8px; margin-bottom: 10px; }
  input, select, button { padding: 6px; font-size: 14px; }
  table { font-size: 14px; }
  button.th { border: none; background: none; font-weight: bold; padding: 0; cursor: pointer; font-size: 14px; }
  .err { color: var(--err); }
</style>
