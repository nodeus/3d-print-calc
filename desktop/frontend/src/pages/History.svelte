<script lang="ts">
  import { onMount } from 'svelte';
  import { wails, type JobDTO, type MaterialDTO } from '../lib/wails';
  import { t } from '../lib/i18n';
  import { user, canWrite } from '../lib/session';
  import { num, hms } from '../lib/format';
  import { askConfirm } from '../lib/confirm';
  import { prefillJob } from '../lib/prefill';
  import { goTab } from '../lib/nav';
  import { downloadPDF, registerPdfExporter } from '../lib/pdf';

  let jobs: JobDTO[] = $state([]);
  let materials: MaterialDTO[] = $state([]);
  let error = $state('');
  let query = $state('');
  let statusFilter = $state('');
  let sortKey: string = $state('start');
  let sortDir: 1 | -1 = $state(-1);
  let page = $state(0);
  const perPage = 50;
  // Видимые колонки (Настройки → Колонки истории).
  let columns: string[] = $state(['filename', 'start', 'status', 'duration', 'weight', 'cost', 'price', 'origin']);

  const sortable: Record<string, string> = {
    filename: 'filename',
    start: 'start',
    end: 'end',
    status: 'status',
    duration: 'duration',
    weight: 'weight',
    length: 'length',
    size: 'size',
    filament: 'filament',
    slicer: 'slicer',
    cost: 'cost',
    price: 'price',
    origin: 'origin',
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
      columns = (await wails.historyColumns()) ?? columns;
      materials = (await wails.materials()) ?? [];
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

  function toggleSort(key: string) {
    if (sortKey === key) {
      sortDir = sortDir === 1 ? -1 : 1;
    } else {
      sortKey = key;
      sortDir = 1;
    }
    page = 0;
  }

  function arrow(key: string): string {
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
        case 'end':
          return j.EndAt;
        case 'status':
          return j.Status;
        case 'duration':
          return j.TotalSec;
        case 'weight':
          return j.WeightG;
        case 'length':
          return j.LengthM;
        case 'size':
          return j.SizeBytes;
        case 'filament':
          return j.FilamentTotalMm;
        case 'slicer':
          return (j.Slicer || '').toLowerCase();
        case 'cost':
          return snap(j).cost;
        case 'price':
          return snap(j).price;
        case 'origin':
          return j.Origin || 'import';
        default:
          return j.StartAt;
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
    let s = '';
    if (j.Origin === 'manual') s = $t('hist.manual');
    else if (j.Origin === 'moonraker') s = 'Moonraker';
    else s = 'CSV';
    if (j.Edited) s += ` · ${$t('hist.edited')}`;
    return s;
  }

  async function remove(j: JobDTO) {
    error = '';
    // Изменённая импортированная: удаление = откат к импорту (строка остаётся).
    const revert = j.Origin !== 'manual';
    const q = revert
      ? `${$t('common.revert_q')} «${j.Filename}»?`
      : `${$t('common.delete_q')} «${j.Filename}»?`;
    if (!(await askConfirm(q))) return;
    try {
      await wails.deleteJob(j.ID);
      await load();
    } catch (e) {
      error = String(e);
    }
  }

  // Правка ручной работы (с автопересчётом на бэкенде).
  let editId = $state(0);
  let edit = $state({
    filename: '', status: 'completed', startAt: '', endAt: '',
    printSec: 0, totalSec: 0, weightG: 0, lengthM: 0, slicer: '', material: '',
  });

  function startEdit(j: JobDTO) {
    error = '';
    editId = j.ID;
    edit = {
      filename: j.Filename, status: j.Status, startAt: j.StartAt, endAt: j.EndAt,
      printSec: j.PrintSec, totalSec: j.TotalSec, weightG: j.WeightG,
      lengthM: j.LengthM, slicer: j.Slicer, material: snapMaterial(j),
    };
  }

  function snapMaterial(j: JobDTO): string {
    try {
      return JSON.parse(j.CostSnapshot)?.material ?? '';
    } catch {
      return '';
    }
  }

  async function saveEdit() {
    error = '';
    try {
      await wails.updateManualJob({
        id: editId,
        filename: edit.filename, status: edit.status,
        startAt: edit.startAt, endAt: edit.endAt,
        printSec: edit.printSec, totalSec: edit.totalSec,
        estimatedSec: 0, sizeBytes: 0,
        weightG: edit.weightG, lengthM: edit.lengthM,
        slicer: edit.slicer, material: edit.material.trim(),
      });
      editId = 0;
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
{#if editId}
  <div class="card">
    <h3>{$t('hist.edit_title')}</h3>
    <div class="row">
      <label>{$t('hist.filename')}<input bind:value={edit.filename} /></label>
      <label>{$t('hist.status')}
        <select bind:value={edit.status}>
          {#each ['completed', 'cancelled', 'error'] as s}<option value={s}>{s}</option>{/each}
        </select>
      </label>
      <label>{$t('calc.material')}
        <select bind:value={edit.material}>
          <option value="">—</option>
          {#each materials as m}<option value={m.code}>{m.code}</option>{/each}
        </select>
      </label>
      <label>{$t('hist.slicer')}<input bind:value={edit.slicer} placeholder="OrcaSlicer" /></label>
    </div>
    <div class="row">
      <label>{$t('hist.start')}<input bind:value={edit.startAt} placeholder="2026-10-08 10:00" /></label>
      <label>{$t('hist.end')}<input bind:value={edit.endAt} placeholder="2026-10-08 12:00" /></label>
      <label>{$t('hist.print_sec')}<input placeholder="3600" type="number" min="0" bind:value={edit.printSec} /></label>
      <label>{$t('hist.total_sec')}<input placeholder="7200" type="number" min="0" bind:value={edit.totalSec} /></label>
    </div>
    <div class="row">
      <label>{$t('hist.weight')}<input placeholder="135" type="number" min="0" step="0.01" bind:value={edit.weightG} /></label>
      <label>{$t('hist.length')}<input placeholder="45" type="number" min="0" step="0.01" bind:value={edit.lengthM} /></label>
    </div>
    <p class="note">{$t('hist.edit_note')}</p>
    <div class="row">
      <button onclick={saveEdit}>{$t('common.save')}</button>
      <button onclick={() => (editId = 0)}>{$t('common.cancel')}</button>
    </div>
  </div>
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
          {#if canWrite($user)}
            <button onclick={() => startEdit(j)} title={$t('hist.edit_title')}>✎</button>
            {#if j.Origin === 'manual' || j.Edited}
              <button onclick={() => remove(j)} title={j.Origin === 'manual' ? $t('common.delete_q') : $t('common.revert_q')}>{j.Origin === 'manual' ? '✕' : '↩'}</button>
            {/if}
          {/if}
        </td>
      </tr>
    {/each}
  </tbody>
</table>

<style>
  .toolbar { display: flex; gap: 8px; align-items: center; margin-bottom: 10px; flex-wrap: wrap; }
  .note { color: var(--muted); }
  input, select, button { padding: 6px; font-size: 14px; }
  .count { color: var(--muted); font-size: 13px; }
  .pager { margin-left: auto; display: flex; gap: 4px; align-items: center; }
  table { font-size: 13px; }
  td.file { max-width: 380px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  button.th { border: none; background: none; font-weight: bold; padding: 0; cursor: pointer; font-size: 13px; }
  .err { color: var(--err); }
  .card { border: 1px solid var(--card-border); border-radius: 8px; padding: 12px; margin-bottom: 10px; }
  .card h3 { margin: 0 0 8px; font-size: 15px; }
  .row { display: flex; gap: 8px; margin-bottom: 8px; flex-wrap: wrap; }
  label { display: flex; flex-direction: column; font-size: 12px; flex: 1; min-width: 130px; }
</style>
