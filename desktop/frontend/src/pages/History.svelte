<script lang="ts">
  import { onMount } from 'svelte';
  import { wails, type JobDTO } from '../lib/wails';
  import { t } from '../lib/i18n';
  import { user, canWrite } from '../lib/session';
  import { num, hms } from '../lib/format';
  import { prefillJob } from '../lib/prefill';
  import { goTab } from '../lib/nav';
  import { downloadPDF, registerPdfExporter } from '../lib/pdf';

  let jobs: JobDTO[] = $state([]);
  let error = $state('');
  let query = $state('');
  let statusFilter = $state('');
  let sortKey: 'filename' | 'start' | 'status' | 'weight' | 'cost' = $state('start');
  let sortDir: 1 | -1 = $state(-1);
  let page = $state(0);
  const perPage = 50;
  // Видимые колонки (Настройки → Колонки истории).
  let columns: string[] = $state(['filename', 'start', 'status', 'duration', 'weight', 'cost', 'price', 'origin']);

  const sortable: Record<string, 'filename' | 'start' | 'status' | 'weight' | 'cost'> = {
    filename: 'filename',
    start: 'start',
    status: 'status',
    weight: 'weight',
    cost: 'cost',
  };

  function colLabel(key: string): string {
    const map: Record<string, string> = {
      filename: $t('hist.filename'),
      start: $t('hist.start'),
      end: $t('hist.end'),
      status: $t('hist.status'),
      duration: $t('hist.duration'),
      weight: $t('hist.weight'),
      length: $t('hist.length'),
      size: $t('hist.size'),
      filament: $t('hist.filament'),
      slicer: $t('hist.slicer'),
      cost: $t('hist.cost'),
      price: $t('calc.price'),
      origin: $t('hist.origin'),
    };
    return map[key] ?? key;
  }

  function cell(j: JobDTO, key: string): string {
    switch (key) {
      case 'filename':
        return j.Filename;
      case 'start':
        return j.StartAt;
      case 'end':
        return j.EndAt;
      case 'status':
        return j.Status;
      case 'duration':
        return hms(j.TotalSec);
      case 'weight':
        return num(j.WeightG);
      case 'length':
        return num(j.LengthM);
      case 'size':
        return j.SizeBytes ? String(j.SizeBytes) : '—';
      case 'filament':
        return num(j.FilamentTotalMm);
      case 'slicer':
        return j.Slicer || '—';
      case 'cost':
        return num(snap(j).cost);
      case 'price':
        return num(snap(j).price);
      case 'origin':
        return originBadge(j);
      default:
        return '';
    }
  }

  function isNum(key: string): boolean {
    return ['duration', 'weight', 'length', 'size', 'filament', 'cost', 'price'].includes(key);
  }

  async function load() {
    try {
      jobs = (await wails.jobs(0)) ?? []; // всё: раньше лимит 200 обрезал историю
      columns = await wails.historyColumns();
    } catch (e) {
      error = String(e);
    }
  }
  onMount(() => {
    load();
    registerPdfExporter(toPDF);
    return () => registerPdfExporter(null);
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
    goTab.set('calc');
  }

  async function toPDF() {
    await downloadPDF($t('nav.history'), [
      {
        Title: '',
        Headers: columns.map(colLabel),
        Rows: filtered.map((j) => columns.map((c) => cell(j, c))),
      },
    ]);
  }

  function originBadge(j: JobDTO): string {
    if (j.Origin === 'manual') return $t('hist.manual');
    if (j.Origin === 'moonraker') return 'Moonraker';
    return 'CSV';
  }

  async function remove(j: JobDTO) {
    error = '';
    if (!confirm(`${j.Filename}?`)) return;
    try {
      await wails.deleteJob(j.ID);
      await load();
    } catch (e) {
      error = String(e);
    }
  }
</script>

{#if error}<p class="err">{error}</p>{/if}
{#if !jobs.length}
  <p class="note">{$t('hist.empty')}</p>
{/if}
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
    {#each columns as c}
      <th class:n={isNum(c)}>
        {#if sortable[c]}
          <button class="th" onclick={() => toggleSort(sortable[c])}>{colLabel(c)}{arrow(sortable[c])}</button>
        {:else}{colLabel(c)}{/if}
      </th>
    {/each}
    <th class="no-print"></th>
  </tr></thead>
  <tbody>
    {#each visible as j}
      <tr>
        {#each columns as c}
          <td class:file={c === 'filename'} class:n={isNum(c)} title={c === 'filename' ? j.Filename : undefined}>{cell(j, c)}</td>
        {/each}
        <td class="no-print">
          <button onclick={() => toCalc(j)}>{$t('hist.to_calc')}</button>
          {#if canWrite($user) && j.Origin === 'manual'}<button onclick={() => remove(j)}>✕</button>{/if}
        </td>
      </tr>
    {/each}
  </tbody>
</table>

<style>
  .toolbar { display: flex; gap: 8px; align-items: center; margin-bottom: 10px; flex-wrap: wrap; }
  .note { color: #666; }
  input, select, button { padding: 6px; font-size: 14px; }
  .count { color: #666; font-size: 13px; }
  .pager { margin-left: auto; display: flex; gap: 4px; align-items: center; }
  table { font-size: 13px; }
  td.file { max-width: 380px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  button.th { border: none; background: none; font-weight: bold; padding: 0; cursor: pointer; font-size: 13px; }
  .err { color: #b00; }
</style>
