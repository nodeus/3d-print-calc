<script lang="ts">
  import { onMount } from 'svelte';
  import { wails, type JobDTO } from '../lib/wails';
  import { t } from '../lib/i18n';
  import { num, hms } from '../lib/format';
  import { prefillJob } from '../lib/prefill';

  let jobs: JobDTO[] = $state([]);
  let query = $state('');
  let statusFilter = $state('');
  let sortKey: 'filename' | 'start' | 'status' | 'weight' | 'cost' = $state('start');
  let sortDir: 1 | -1 = $state(-1);
  let page = $state(0);
  const perPage = 50;

  onMount(async () => {
    try {
      jobs = await wails.jobs(0); // всё: раньше лимит 200 обрезал историю
    } catch (e) {
      console.error(e);
    }
  });

  function snap(j: JobDTO): { cost: number; price: number } {
    try {
      const s = JSON.parse(j.CostSnapshot);
      return { cost: Number(s.cost) || 0, price: Number(s.price) || 0 };
    } catch {
      return { cost: 0, price: 0 };
    }
  }

  function toggleSort(key: typeof sortKey) {
    if (sortKey === key) {
      sortDir = sortDir === 1 ? -1 : 1;
    } else {
      sortKey = key;
      sortDir = 1;
    }
    page = 0;
  }

  function arrow(key: typeof sortKey): string {
    return sortKey === key ? (sortDir === 1 ? ' ▲' : ' ▼') : '';
  }

  let filtered = $derived.by(() => {
    const q = query.trim().toLowerCase();
    let rows = jobs.filter(
      (j) =>
        (!q || j.Filename.toLowerCase().includes(q)) &&
        (!statusFilter || j.Status === statusFilter),
    );
    const dir = sortDir;
    const val = (j: JobDTO): string | number => {
      switch (sortKey) {
        case 'filename':
          return j.Filename.toLowerCase();
        case 'start':
          return j.StartAt;
        case 'status':
          return j.Status;
        case 'weight':
          return j.WeightG;
        case 'cost':
          return snap(j).cost;
      }
    };
    rows = [...rows].sort((a, b) => {
      const va = val(a);
      const vb = val(b);
      const cmp = typeof va === 'number' ? va - (vb as number) : String(va).localeCompare(String(vb));
      return cmp * dir;
    });
    return rows;
  });

  let pages = $derived(Math.max(1, Math.ceil(filtered.length / perPage)));
  let visible = $derived(filtered.slice(page * perPage, page * perPage + perPage));
  let statuses = $derived([...new Set(jobs.map((j) => j.Status))].sort());

  function toCalc(j: JobDTO) {
    prefillJob.set(j);
  }
</script>

<div class="toolbar no-print">
  <input placeholder={$t('hist.search')} bind:value={query} oninput={() => (page = 0)} />
  <select bind:value={statusFilter} onchange={() => (page = 0)}>
    <option value="">— {$t('hist.status')} —</option>
    {#each statuses as s}<option value={s}>{s}</option>{/each}
  </select>
  <span class="count">{$t('hist.shown')}: {visible.length} / {filtered.length} / {jobs.length}</span>
  <span class="pager">
    <button disabled={page === 0} onclick={() => page--}>‹</button>
    {page + 1} / {pages}
    <button disabled={page >= pages - 1} onclick={() => page++}>›</button>
  </span>
</div>

<table>
  <thead><tr>
    <th><button class="th" onclick={() => toggleSort('filename')}>{$t('hist.filename')}{arrow('filename')}</button></th>
    <th><button class="th" onclick={() => toggleSort('start')}>{$t('hist.start')}{arrow('start')}</button></th>
    <th><button class="th" onclick={() => toggleSort('status')}>{$t('hist.status')}{arrow('status')}</button></th>
    <th>{$t('hist.duration')}</th>
    <th><button class="th" onclick={() => toggleSort('weight')}>{$t('hist.weight')}{arrow('weight')}</button></th>
    <th><button class="th" onclick={() => toggleSort('cost')}>{$t('hist.cost')}{arrow('cost')}</button></th>
    <th>{$t('calc.price')}</th>
    <th class="no-print"></th>
  </tr></thead>
  <tbody>
    {#each visible as j}
      <tr>
        <td class="file" title={j.Filename}>{j.Filename}</td>
        <td>{j.StartAt}</td>
        <td>{j.Status}</td>
        <td class="n">{hms(j.TotalSec)}</td>
        <td class="n">{num(j.WeightG)}</td>
        <td class="n">{num(snap(j).cost)}</td>
        <td class="n">{num(snap(j).price)}</td>
        <td class="no-print"><button onclick={() => toCalc(j)}>{$t('hist.to_calc')}</button></td>
      </tr>
    {/each}
  </tbody>
</table>

<style>
  .toolbar { display: flex; gap: 8px; align-items: center; margin-bottom: 10px; flex-wrap: wrap; }
  input, select, button { padding: 6px; font-size: 14px; }
  .count { color: #666; font-size: 13px; }
  .pager { margin-left: auto; display: flex; gap: 4px; align-items: center; }
  table { width: 100%; border-collapse: collapse; font-size: 13px; }
  th, td { padding: 4px 6px; border-bottom: 1px solid #eee; text-align: left; }
  td.n { text-align: right; font-variant-numeric: tabular-nums; }
  td.file { max-width: 380px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  button.th { border: none; background: none; font-weight: bold; padding: 0; cursor: pointer; font-size: 13px; }
</style>
