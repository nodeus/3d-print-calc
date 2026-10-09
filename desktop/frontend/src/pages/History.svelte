<script lang="ts">
  import { onMount } from 'svelte';
  import { get } from 'svelte/store';
  import { wails, type JobDTO, type MaterialDTO } from '../lib/wails';
  import { t } from '../lib/i18n';
  import { user, canWrite } from '../lib/session';
  import { num, hms } from '../lib/format';
  import { askConfirm } from '../lib/confirm';
  import { prefillJob } from '../lib/prefill';
  import { goTab } from '../lib/nav';
  import { calcFormDefaults } from '../lib/calcform';
  import { downloadPDF, registerPdfExporter } from '../lib/pdf';
  import { myHistoryColumns, saveMySettingsPatch } from '../lib/usersettings';
  import { thWidth, startColResize, resetColWidth } from '../lib/colwidths';

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
    material: 'material',
    tariff: 'tariff',
    power: 'power',
    amort: 'amort',
    maint: 'maint',
    purge: 'purge',
    oprate: 'oprate',
    prep: 'prep',
    post: 'post',
    designer: 'designer',
    modelh: 'modelh',
    modelprice: 'modelprice',
    extra: 'extra',
    fail: 'fail',
    marginprofit: 'marginprofit',
    margindev: 'margindev',
    fees: 'fees',
    copies: 'copies',
    vat: 'vat',
    round: 'round',
  };

  // Вход расчёта из снапшота (v2); импорт без правок — пусто.
  function snapInput(j: JobDTO): Record<string, number> {
    try {
      return JSON.parse(j.CostSnapshot)?.input ?? {};
    } catch {
      return {};
    }
  }

  function snapMaterialOf(j: JobDTO): string {
    try {
      return JSON.parse(j.CostSnapshot)?.material ?? '';
    } catch {
      return '';
    }
  }

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
      material: $t('calc.material'),
      tariff: $t('calc.tariff'),
      power: $t('calc.power'),
      amort: $t('calc.amort'),
      maint: $t('calc.maint'),
      purge: $t('calc.purge'),
      oprate: $t('calc.op_rate'),
      prep: $t('calc.prep'),
      post: $t('calc.post'),
      designer: $t('calc.designer_rate'),
      modelh: $t('calc.model_h'),
      modelprice: $t('calc.model_price'),
      extra: $t('calc.extra'),
      fail: $t('calc.fail'),
      marginprofit: $t('calc.margin_profit'),
      margindev: $t('calc.margin_dev'),
      fees: $t('calc.fees'),
      copies: $t('calc.copies'),
      vat: $t('calc.vat'),
      round: $t('calc.round'),
    };
    return map[key] ?? key;
  }

  // Маппинг колонка → ключ snapshot.input.
  const inpKey: Record<string, string> = {
    tariff: 'tariff', power: 'powerKW', amort: 'amortPerHour', maint: 'maintPerHour',
    purge: 'purgeWasteG', oprate: 'opRate', prep: 'prepH', post: 'postH',
    designer: 'designerRate', modelh: 'modelH', modelprice: 'modelPrice',
    extra: 'extraMat', fail: 'failRate', marginprofit: 'marginProfit',
    margindev: 'marginDev', fees: 'fees', copies: 'copies',
    vat: 'vatRate', round: 'roundStep',
  };

  // Значение расчётной колонки: число из snapshot.input (доли → %) или null.
  function inpVal(j: JobDTO, key: string): number | null {
    const k = inpKey[key];
    if (!k) return null;
    const raw = snapInput(j)[k];
    if (raw == null) return null;
    if (['fail', 'marginprofit', 'margindev', 'vat'].includes(key)) return raw * 100;
    return raw;
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
      case 'material':
        return snapMaterialOf(j) || '—';
      case 'fail':
      case 'marginprofit':
      case 'margindev':
      case 'vat': {
        const v = inpVal(j, key);
        return v == null ? '—' : num(v) + '%';
      }
      case 'tariff':
      case 'power':
      case 'amort':
      case 'maint':
      case 'purge':
      case 'oprate':
      case 'prep':
      case 'post':
      case 'designer':
      case 'modelh':
      case 'modelprice':
      case 'extra':
      case 'fees':
      case 'copies':
      case 'round': {
        const v = inpVal(j, key);
        return v == null ? '—' : num(v);
      }
      default:
        return '';
    }
  }

  function isNum(key: string): boolean {
    return ['duration', 'weight', 'length', 'size', 'filament', 'cost', 'price',
      'tariff', 'power', 'amort', 'maint', 'purge', 'oprate', 'prep', 'post',
      'designer', 'modelh', 'modelprice', 'extra', 'fail', 'marginprofit',
      'margindev', 'fees', 'copies', 'vat', 'round'].includes(key);
  }

  async function load() {
    try {
      jobs = (await wails.jobs(0)) ?? []; // всё: раньше лимит 200 обрезал историю
      // Колонки: секция пользователя из файла, фолбэк — legacy-БД.
      columns = await myHistoryColumns(columns);
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
        case 'material':
          return snapMaterialOf(j).toLowerCase();
        case 'tariff':
        case 'power':
        case 'amort':
        case 'maint':
        case 'purge':
        case 'oprate':
        case 'prep':
        case 'post':
        case 'designer':
        case 'modelh':
        case 'modelprice':
        case 'extra':
        case 'fail':
        case 'marginprofit':
        case 'margindev':
        case 'fees':
        case 'copies':
        case 'vat':
        case 'round':
          return inpVal(j, sortKey) ?? -1;
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

  // Все колонки в фиксированном порядке бэкенда (настройка — в Настройках).
  const allColumns = Object.keys(sortable);

  function openColumns() {
    goTab.set('settings');
  }

  // Drag-перестановка колонок за грип (manager+). Порог 4px отделяет
  // перетаскивание от клика; линия показывает место вставки.
  let dragCol: string | null = $state(null);
  let dropX: number | null = $state(null);
  let wrapEl: HTMLDivElement | null = $state(null);

  async function persistColumns(ordered: string[]) {
    error = '';
    try {
      await wails.setHistoryColumns(ordered);
      await saveMySettingsPatch({ historyColumns: ordered });
      columns = ordered;
    } catch (e) {
      error = String(e);
    }
  }

  function gripDown(e: PointerEvent, col: string) {
    if (!canWrite(get(user))) return;
    e.preventDefault();
    e.stopPropagation();
    const grip = e.currentTarget as HTMLElement;
    const startX = e.clientX;
    let dragging = false;
    const ths = () =>
      [...(wrapEl?.querySelectorAll('thead th[data-col]') ?? [])] as HTMLElement[];
    const onMove = (ev: PointerEvent) => {
      if (!dragging) {
        if (Math.abs(ev.clientX - startX) < 4) return;
        dragging = true;
        dragCol = col;
      }
      // Позиция вставки: первый th, чья середина правее курсора.
      const list = ths();
      let idx = list.length;
      let lineX: number | null = null;
      const wrapR = wrapEl?.getBoundingClientRect();
      for (let i = 0; i < list.length; i++) {
        const r = list[i].getBoundingClientRect();
        if (ev.clientX < r.left + r.width / 2) {
          idx = i;
          lineX = wrapR ? r.left - wrapR.left : null;
          break;
        }
        lineX = wrapR ? r.right - wrapR.left : null;
      }
      (grip as HTMLElement & { __dropIdx?: number }).__dropIdx = idx;
      dropX = lineX;
    };
    const onUp = async () => {
      window.removeEventListener('pointermove', onMove);
      window.removeEventListener('pointerup', onUp);
      const idx = (grip as HTMLElement & { __dropIdx?: number }).__dropIdx;
      const from = columns.indexOf(col);
      dragCol = null;
      dropX = null;
      if (dragging && idx != null && from >= 0) {
        // Индекс вставки считался со строкой-источником: после удаления сдвиг.
        let to = idx > from ? idx - 1 : idx;
        const next = columns.filter((c) => c !== col);
        to = Math.max(0, Math.min(next.length, to));
        next.splice(to, 0, col);
        if (next.join() !== columns.join()) await persistColumns(next);
      }
      dragging = false;
    };
    window.addEventListener('pointermove', onMove);
    window.addEventListener('pointerup', onUp);
  }

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

  // Правка работы истории (manual и import) с автопересчётом на бэкенде.
  // Расчётные поля — как в Калькуляторе (проценты — %, в snapshot уйдут долями).
  let editId = $state(0);
  let edit = $state({
    filename: '', status: 'completed', startAt: '', endAt: '',
    printSec: 0, totalSec: 0, weightG: 0, lengthM: 0, slicer: '', material: '',
    tariff: 0, powerKW: 0, amortPerHour: 0, maintPerHour: 0, purgeG: 0,
    opRate: 0, prepH: 0, postH: 0, designerRate: 0, modelH: 0, modelPrice: 0,
    extraMat: 0, failPct: 0, marginProfitPct: 0, marginDevPct: 0, fees: 0,
    copies: 1, vatPct: 0, roundStep: 0,
  });

  function startEdit(j: JobDTO) {
    error = '';
    editId = j.ID;
    // Вход из снапшота; чего нет (импорт) — дефолты Калькулятора.
    const d = calcFormDefaults;
    let inp: Record<string, number> = {};
    try {
      inp = JSON.parse(j.CostSnapshot)?.input ?? {};
    } catch {
      inp = {};
    }
    const pick = (k: string, fb: number) => (inp[k] != null ? inp[k] : fb);
    edit = {
      filename: j.Filename, status: j.Status, startAt: j.StartAt, endAt: j.EndAt,
      printSec: j.PrintSec, totalSec: j.TotalSec, weightG: j.WeightG,
      lengthM: j.LengthM, slicer: j.Slicer, material: snapMaterial(j),
      tariff: pick('tariff', d.tariff), powerKW: pick('powerKW', d.powerKW),
      amortPerHour: pick('amortPerHour', d.amortPerHour),
      maintPerHour: pick('maintPerHour', d.maintPerHour),
      purgeG: pick('purgeWasteG', d.purgeG),
      opRate: pick('opRate', d.opRate), prepH: pick('prepH', d.prepH), postH: pick('postH', d.postH),
      designerRate: pick('designerRate', d.designerRate),
      modelH: pick('modelH', d.modelH), modelPrice: pick('modelPrice', d.modelPrice),
      extraMat: pick('extraMat', d.extraMat),
      failPct: pick('failRate', 0) * 100,
      marginProfitPct: pick('marginProfit', d.marginProfitPct / 100) * 100,
      marginDevPct: pick('marginDev', d.marginDevPct / 100) * 100,
      fees: pick('fees', d.fees), copies: Math.max(1, Math.round(pick('copies', 1))),
      vatPct: pick('vatRate', 0) * 100, roundStep: pick('roundStep', 0),
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
        tariff: edit.tariff, powerKW: edit.powerKW,
        amortPerHour: edit.amortPerHour, maintPerHour: edit.maintPerHour,
        purgeWasteG: edit.purgeG,
        opRate: edit.opRate, prepH: edit.prepH, postH: edit.postH,
        designerRate: edit.designerRate, modelH: edit.modelH, modelPrice: edit.modelPrice,
        extraMat: edit.extraMat, failRate: edit.failPct / 100,
        marginProfit: edit.marginProfitPct / 100, marginDev: edit.marginDevPct / 100,
        fees: edit.fees, copies: Math.max(1, Math.round(edit.copies)),
        vatRate: edit.vatPct / 100, roundStep: edit.roundStep,
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
          {#each materials.filter((m) => !m.archived || m.code === edit.material) as m}<option value={m.code}>{m.code}</option>{/each}
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
      <label>{$t('calc.tariff')}<input type="number" min="0" step="0.01" bind:value={edit.tariff} /></label>
      <label>{$t('calc.power')}<input type="number" min="0" step="0.01" bind:value={edit.powerKW} /></label>
      <label>{$t('calc.amort')}<input type="number" min="0" step="0.01" bind:value={edit.amortPerHour} /></label>
      <label>{$t('calc.maint')}<input type="number" min="0" step="0.01" bind:value={edit.maintPerHour} /></label>
      <label>{$t('calc.purge')}<input type="number" min="0" step="0.1" bind:value={edit.purgeG} /></label>
    </div>
    <div class="row">
      <label>{$t('calc.op_rate')}<input type="number" min="0" bind:value={edit.opRate} /></label>
      <label>{$t('calc.prep')}<input type="number" min="0" step="0.1" bind:value={edit.prepH} /></label>
      <label>{$t('calc.post')}<input type="number" min="0" step="0.1" bind:value={edit.postH} /></label>
      <label>{$t('calc.designer_rate')}<input type="number" min="0" bind:value={edit.designerRate} /></label>
      <label>{$t('calc.model_h')}<input type="number" min="0" step="0.1" bind:value={edit.modelH} /></label>
      <label>{$t('calc.model_price')}<input type="number" min="0" bind:value={edit.modelPrice} /></label>
    </div>
    <div class="row">
      <label>{$t('calc.extra')}<input type="number" min="0" bind:value={edit.extraMat} /></label>
      <label>{$t('calc.fees')}<input type="number" min="0" bind:value={edit.fees} /></label>
      <label>{$t('calc.fail')}<input type="number" min="0" step="0.1" bind:value={edit.failPct} /></label>
      <label>{$t('calc.margin_profit')}<input type="number" min="0" step="1" bind:value={edit.marginProfitPct} /></label>
      <label>{$t('calc.margin_dev')}<input type="number" min="0" step="1" bind:value={edit.marginDevPct} /></label>
      <label>{$t('calc.copies')}<input type="number" min="1" step="1" bind:value={edit.copies} /></label>
      <label>{$t('calc.vat')}<input type="number" min="0" step="1" bind:value={edit.vatPct} /></label>
      <label>{$t('calc.round')}<input type="number" min="0" step="1" bind:value={edit.roundStep} /></label>
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
  <button onclick={openColumns}>{$t('hist.columns_btn')}: {columns.length}/{allColumns.length}</button>
  <span class="pager">
    <button disabled={page === 0} onclick={() => page--}>‹</button>
    {page + 1} / {pages}
    <button disabled={page >= pages - 1} onclick={() => page++}>›</button>
  </span>
</div>

<div class="twrap" bind:this={wrapEl}>
{#if dropX != null}<div class="drop-line no-print" style="left:{dropX}px;"></div>{/if}
<table class="tw" class:dragging={dragCol != null}>
  <thead><tr>
    {#each columns as c}
      <th class:n={isNum(c)} style={thWidth('history', c)} data-col={c} class:dragsrc={dragCol === c}>
        {#if canWrite($user)}<span class="grip no-print" role="separator" title={$t('hist.drag_cols')} onpointerdown={(e) => gripDown(e, c)}>⋮⋮</span>{/if}
        {#if sortable[c]}
          <button class="th" onclick={() => toggleSort(sortable[c])}>{colLabel(c)}{arrow(sortable[c])}</button>
        {:else}{colLabel(c)}{/if}
        <span class="rz no-print" role="separator" onmousedown={(e) => startColResize(e, 'history', c)} ondblclick={() => resetColWidth('history', c)}></span>
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
</div>

<style>
  .toolbar { display: flex; gap: 8px; align-items: center; margin-bottom: 10px; flex-wrap: wrap; }
  .twrap { position: relative; }
  table.dragging { user-select: none; }
  th.dragsrc { opacity: 0.5; }
  .grip { cursor: grab; opacity: 0.45; margin-right: 4px; font-size: 12px; }
  .grip:active { cursor: grabbing; }
  .drop-line { position: absolute; top: 0; bottom: 0; width: 2px; background: var(--accent); z-index: 2; pointer-events: none; }
  @media print {
    .grip, .drop-line { display: none; }
  }
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
