<script lang="ts">
  import { onMount } from 'svelte';
  import { wails, type Customer, type OrderRow } from '../lib/wails';
  import { t } from '../lib/i18n';
  import { user, canWrite } from '../lib/session';
  import { askConfirm } from '../lib/confirm';
  import { notify } from '../lib/notice';
  import { gotoCustomerOrders } from '../lib/nav';
  import { downloadPDF, registerPdfExporter } from '../lib/pdf';
  import { money } from '../lib/format';
  import { thWidth, startColResize, resetColWidth } from '../lib/colwidths';
  import { mySettings } from '../lib/usersettings';
  import { orderFor, saveTableOrder } from '../lib/colorder';
  import { startColDrag } from '../lib/coldrag';
  import { colKeyMove } from '../lib/colkeyboard';

  let list: Customer[] = $state([]);
  let rows: OrderRow[] = $state([]);
  let baseCur = $state('RUB');
  let error = $state('');
  let sortKey = $state('name');
  let sortDir: 1 | -1 = $state(1);

  function toggleSort(key: string) {
    if (sortKey === key) sortDir = sortDir === 1 ? -1 : 1;
    else { sortKey = key; sortDir = 1; }
  }

  function arrow(key: string): string {
    return sortKey === key ? (sortDir === 1 ? ' ▲' : ' ▼') : '';
  }

  function sortVal(c: Customer): string | number {
    const mine = rows.filter((r) => r.order.CustomerID === c.ID);
    switch (sortKey) {
      case 'contact': return (c.Contact || '').toLowerCase();
      case 'note': return (c.Note || '').toLowerCase();
      case 'orders': return mine.length;
      case 'paid': return mine.reduce((s, r) => s + r.paid, 0);
      case 'debt': return mine.reduce((s, r) => s + Math.max(0, r.balance), 0);
      default: return c.Name.toLowerCase();
    }
  }

  let shown = $derived([...list].sort((a, b) => {
    const va = sortVal(a);
    const vb = sortVal(b);
    const cmp = typeof va === 'number' ? va - (vb as number) : String(va).localeCompare(String(vb));
    return cmp * sortDir;
  }));

  // Агрегаты заказчика по строкам заказов.
  function custAgg(id: number): { orders: number; paid: number; debt: number } {
    const mine = rows.filter((r) => r.order.CustomerID === id);
    return {
      orders: mine.length,
      paid: mine.reduce((s, r) => s + r.paid, 0),
      debt: mine.reduce((s, r) => s + Math.max(0, r.balance), 0),
    };
  }

  const custCols = [
    { key: 'name', label: 'cust.name' }, { key: 'contact', label: 'cust.contact' },
    { key: 'note', label: 'cust.note' }, { key: 'orders', label: 'cust.orders', num: true },
    { key: 'paid', label: 'cust.paid', num: true }, { key: 'debt', label: 'cust.debt', num: true },
  ];
  const custKeys = custCols.map((c) => c.key);
  const custNum = (k: string) => custCols.some((c) => c.key === k && c.num);
  const custLabel = (k: string) => custCols.find((c) => c.key === k)?.label ?? k;

  let custOrd = $derived(orderFor($mySettings.colOrder, 'customers', custKeys));

  function custText(c: Customer, k: string): string {
    const a = custAgg(c.ID);
    switch (k) {
      case 'name': return c.Name;
      case 'contact': return c.Contact;
      case 'note': return c.Note;
      case 'orders': return String(a.orders);
      case 'paid': return money(a.paid / 100, baseCur, 'ru');
      case 'debt': return money(a.debt / 100, baseCur, 'ru');
      default: return '';
    }
  }

  let dragKey: string | null = $state(null);
  let dropX: number | null = $state(null);
  let wrapEl: HTMLDivElement | null = $state(null);

  async function dropCols(next: string[]) {
    error = '';
    try {
      await saveTableOrder('customers', next);
    } catch (e) {
      error = String(e);
    }
  }
  let name = $state('');
  let contact = $state('');
  let note = $state('');
  let editId = $state(0);
  let edit: Customer = $state({ ID: 0, Name: '', Contact: '', Currency: 'RUB', Note: '' });

  async function load() {
    try {
      list = (await wails.listCustomers()) ?? [];
      rows = (await wails.listOrderRows()) ?? [];
      baseCur = await wails.getBaseCurrency();
    } catch (e) {
      error = String(e);
    }
  }
  onMount(() => {
    load();
    registerPdfExporter(toPDF);
    return () => registerPdfExporter(null);
  });

  async function add() {
    error = '';
    if (!name.trim()) return;
    try {
      await wails.saveCustomer({ ID: 0, Name: name.trim(), Contact: contact.trim(), Currency: 'RUB', Note: note.trim() });
      name = '';
      contact = '';
      note = '';
      await load();
      notify($t('common.saved'));
    } catch (e) {
      error = String(e);
    }
  }

  function startEdit(c: Customer) {
    editId = c.ID;
    edit = { ...c };
  }

  async function saveEdit() {
    error = '';
    try {
      await wails.saveCustomer(edit);
      editId = 0;
      await load();
      notify($t('common.saved'));
    } catch (e) {
      error = String(e);
    }
  }

  async function remove(c: Customer) {
    error = '';
    if (!(await askConfirm(`${$t("common.delete_q")} «${c.Name}»?`))) return;
    try {
      await wails.deleteCustomer(c.ID);
      await load();
    } catch (e) {
      error = String(e);
    }
  }

  async function toPDF() {
    await downloadPDF($t('nav.customers'), [
      {
        Title: '',
        Headers: custOrd.map((k) => $t(custLabel(k))),
        Rows: shown.map((c) => custOrd.map((k) => custText(c, k))),
      },
    ]);
  }
</script>

{#if canWrite($user)}
  <div class="row no-print">
    <input placeholder={$t('cust.name')} bind:value={name} />
    <input placeholder={$t('cust.contact')} bind:value={contact} />
    <input placeholder={$t('cust.note')} bind:value={note} />
    <button onclick={add}>{$t('common.save')}</button>
  </div>
{/if}
{#if error}<p class="err">{error}</p>{/if}

<div class="twrap" bind:this={wrapEl}>
{#if dropX != null}<div class="drop-line no-print" style="left:{dropX}px;"></div>{/if}
<table class="tw" class:dragging={dragKey != null}>
  <thead><tr>
    {#each custOrd as k}
      <th class:n={custNum(k)} style={thWidth('customers', k)} data-col={k} class:dragsrc={dragKey === k}>
        {#if canWrite($user)}<span class="grip no-print" role="button" tabindex={0} title={$t('hist.drag_cols')} onpointerdown={(e) => { if (canWrite($user)) startColDrag(e, wrapEl, k, custOrd, (dk, x) => { dragKey = dk; dropX = x; }, dropCols); }} onkeydown={(e) => colKeyMove(e, custOrd, k, dropCols)}>⋮⋮</span>{/if}
        <button class="th" onclick={() => toggleSort(k)}>{$t(custLabel(k))}{arrow(k)}</button>
        <span class="rz no-print" role="separator" onmousedown={(e) => startColResize(e, 'customers', k)} ondblclick={() => resetColWidth('customers', k)}></span>
      </th>
    {/each}
    <th class="no-print"></th>
  </tr></thead>
  <tbody>
    {#each shown as c}
      {#if editId === c.ID}
        <tr>
          {#each custOrd as k}
            {#if k === 'name'}
              <td><input bind:value={edit.Name} placeholder={$t('cust.name')} /></td>
            {:else if k === 'contact'}
              <td><input bind:value={edit.Contact} placeholder={$t('cust.contact')} /></td>
            {:else if k === 'note'}
              <td><input bind:value={edit.Note} placeholder={$t('cust.note')} /></td>
            {/if}
          {/each}
          <td class="no-print"><button onclick={saveEdit}>{$t('common.save')}</button> <button onclick={() => (editId = 0)}>{$t('common.cancel')}</button></td>
        </tr>
      {:else}
        <tr>
          {#each custOrd as k}
            {#if k === 'name'}
              <td><button class="link" onclick={() => gotoCustomerOrders(c.ID)}>{c.Name}</button></td>
            {:else}
              <td class:n={custNum(k)}>{custText(c, k)}</td>
            {/if}
          {/each}
          <td class="no-print">{#if canWrite($user)}<button onclick={() => startEdit(c)} aria-label={$t('common.edit')}>✎</button> <button onclick={() => remove(c)} aria-label={$t('common.delete_q')}>✕</button>{/if}</td>
        </tr>
      {/if}
    {/each}
  </tbody>
</table>
</div>

<style>
  .row { display: flex; gap: 8px; margin-bottom: 10px; flex-wrap: wrap; }
  input, button { padding: 6px; font-size: 14px; }
  table { font-size: 14px; }
  button.th { border: none; background: none; font-weight: bold; padding: 0; cursor: pointer; font-size: 14px; }
  .err { color: var(--err); }
  button.link { background: none; border: none; color: var(--accent); cursor: pointer; padding: 0; font-size: inherit; text-decoration: underline; }
</style>
