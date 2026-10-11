<script lang="ts">
  import { onMount, untrack } from 'svelte';
  import { tick } from 'svelte';
  import { get } from 'svelte/store';
  import { wails, type JobDTO, type MaterialDTO, type SpoolDTO, type BulkJobPreview, type JobBulkPatch } from '../lib/wails';
  import { t } from '../lib/i18n';
  import { user, canWrite } from '../lib/session';
  import { num, hms, money } from '../lib/format';
  import { locale } from '../lib/i18n';
  import { askConfirm } from '../lib/confirm';
  import { notify } from '../lib/notice';
  import { prefillJob } from '../lib/prefill';
  import { goTab } from '../lib/nav';
  import { calcFormDefaults } from '../lib/calcform';
  import { downloadPDF, registerPdfExporter } from '../lib/pdf';
  import { myHistoryColumns, saveMySettingsPatch, mySettings } from '../lib/usersettings';
  import { thWidth, startColResize, resetColWidth } from '../lib/colwidths';
  import { splitHours, joinHours } from '../lib/hm';
  import { startColDrag } from '../lib/coldrag';
  import { colKeyMove } from '../lib/colkeyboard';

  let jobs: JobDTO[] = $state([]);
  let materials: MaterialDTO[] = $state([]);
  let error = $state('');
  let query = $state('');
  let statusFilter = $state('');
  let originFilter = $state('');
  let sortKey: string = $state('start');
  let sortDir: 1 | -1 = $state(-1);
  let page = $state(0);
  let perPage = $derived($mySettings.historyPerPage > 0 ? $mySettings.historyPerPage : 50);
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
    spool: 'spool',
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
      spool: $t('calc.spool'),
    };
    return map[key] ?? key;
  }

  // Катушка работы: КОД-N из снапшота; старые без номера — голый код.
  function snapSpool(j: JobDTO): string {
    const code = snapMaterialOf(j);
    if (!code) return '—';
    const no = snapSpoolNo(j);
    return no > 0 ? `${code}-${no}` : code;
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
      case 'spool':
        return snapSpool(j);
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
      // Чистим сквозную выборку от исчезнувших строк.
      selected = selected.filter((id) => jobs.some((j) => j.ID === id));
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
        (!statusFilter || j.Status === statusFilter) &&
        (!originFilter || (j.Origin || 'import') === originFilter),
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
        case 'spool':
          return snapSpool(j).toLowerCase();
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

  $effect(() => {
    void filtered.length;
    void perPage;
    untrack(() => {
      if (page > pages - 1) page = pages - 1;
      if (page < 0) page = 0;
    });
  });
  let statuses = $derived([...new Set(jobs.map((j) => j.Status))].sort());
  let origins = $derived([...new Set(jobs.map((j) => j.Origin || 'import'))].sort());

  // Подпись статуса для фильтра: известные — перевод, чужие строки принтера — как есть.
  function statusLabel(status: string): string {
    if (status === 'completed') return $t('jobst.completed');
    if (status === 'cancelled') return $t('jobst.cancelled');
    if (status === 'error') return $t('jobst.error');
    return status;
  }

  // Подпись источника для фильтра (как originBadge, без пометки «изменена»).
  function originLabel(origin: string): string {
    if (origin === 'manual') return $t('hist.manual');
    if (origin === 'moonraker') return 'Moonraker';
    return 'CSV';
  }

  // Все колонки в фиксированном порядке бэкенда (настройка — в Настройках).
  const allColumns = Object.keys(sortable);

  function openColumns() {
    goTab.set('settings');
  }

  // Суммы по выборке: только аддитивные колонки (сырьё, не строки ячеек).
  const summable = ['duration', 'weight', 'length', 'size', 'filament', 'cost', 'price'];

  function rawVal(j: JobDTO, key: string): number {
    switch (key) {
      case 'duration': return j.TotalSec;
      case 'weight': return j.WeightG;
      case 'length': return j.LengthM;
      case 'size': return j.SizeBytes;
      case 'filament': return j.FilamentTotalMm;
      case 'cost': return snap(j).cost;
      case 'price': return snap(j).price;
      default: return 0;
    }
  }

  // Суммы по выборке — всегда актуальны (дешёвый проход, кнопки нет).
  let totals: Record<string, number> = $derived.by(() => {
    const t: Record<string, number> = {};
    for (const k of summable) {
      let s = 0;
      for (const j of filtered) s += rawVal(j, k) || 0;
      t[k] = Math.round(s * 100) / 100;
    }
    return t;
  });
  let totalsN = $derived(filtered.length);

  function footText(k: string): string {
    if (!totals) return '';
    const labelCol = columns.includes('filename') ? 'filename' : columns[0];
    if (k === labelCol) return `${$t('hist.total')} · ${totalsN}`;
    if (!(k in totals)) return '';
    const v = totals[k];
    if (k === 'duration') return hms(Math.round(v));
    return num(v);
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
    startColDrag(e, wrapEl, col, columns, (k, x) => { dragCol = k; dropX = x; }, persistColumns);
  }

  function toCalc(j: JobDTO) {
    prefillJob.set(j);
    goTab.set('calc');
  }

  async function toPDF() {
    const rows = filtered.map((j) => columns.map((c) => cell(j, c)));
    if (totals) rows.push(columns.map((c) => footText(c)));
    await downloadPDF($t('nav.history'), [
      {
        Title: '',
        Headers: columns.map(colLabel),
        Rows: rows,
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

  // Подсветка изменённой строки (вспышка гаснет сама через 650мс).
  let flashKey: string | null = $state(null);
  let flashTimer: number | undefined;
  function flash(key: string) {
    window.clearTimeout(flashTimer);
    flashKey = key;
    flashTimer = window.setTimeout(() => { flashKey = null; }, 650);
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

  // Одиночное удаление нетронутого импорта — тем же bulk-путём, что выборка.
  async function removeOne(j: JobDTO) {
    error = '';
    bulkMsg = '';
    if (!(await askConfirm(`${$t('hist.delete_one_q')} «${j.Filename}»?`))) return;
    try {
      const rep = await wails.deleteJobs([j.ID]);
      await load();
      const skipped = rep.skippedOrder ?? [];
      bulkMsg = `${$t('hist.bulk_deleted')}: ${rep.deleted}` +
        (skipped.length ? ` · ${$t('hist.bulk_skipped')}: ${skipped.join(', ')}` : '');
    } catch (e) {
      error = String(e);
    }
  }

  // Массовое удаление чекбоксами: выбор сквозной (страницы/фильтр/сортировка).
  let selected: number[] = $state([]);
  let bulkMsg = $state('');
  let headBox: HTMLInputElement | null = $state(null);

  let pageAll = $derived(visible.length > 0 && visible.every((j) => selected.includes(j.ID)));
  $effect(() => {
    if (headBox) headBox.indeterminate = !pageAll && visible.some((j) => selected.includes(j.ID));
  });

  function toggleSel(id: number) {
    selected = selected.includes(id) ? selected.filter((x) => x !== id) : [...selected, id];
    bulkMsg = '';
  }

  function togglePage() {
    const ids = visible.map((j) => j.ID);
    const all = ids.length > 0 && ids.every((id) => selected.includes(id));
    selected = all ? selected.filter((id) => !ids.includes(id)) : [...new Set([...selected, ...ids])];
    bulkMsg = '';
  }

  async function removeSelected() {
    error = '';
    bulkMsg = '';
    const ids = [...selected];
    if (!ids.length) return;
    if (!(await askConfirm(`${$t('hist.delete_selected_q')} (${ids.length})?`))) return;
    try {
      const rep = await wails.deleteJobs(ids);
      await load();
      selected = [];
      const skipped = rep.skippedOrder ?? [];
      bulkMsg = `${$t('hist.bulk_deleted')}: ${rep.deleted}` +
        (skipped.length ? ` · ${$t('hist.bulk_skipped')}: ${skipped.join(', ')}` : '');
    } catch (e) {
      error = String(e);
    }
  }

  // Массовая правка: разреженный патч (null/''/0 = поле нетронуто,
  // плейсхолдеры «—»). Применяются только реально изменившиеся поля.
  let bulkOpen = $state(false);
  let bulkReview: BulkJobPreview[] | null = $state(null);
  let bulkStatus = $state('');
  let bulkStartAt: string | null = $state(null);
  let bulkEndAt: string | null = $state(null);
  let bulkPrintSec: number | null = $state(null);
  let bulkTotalSec: number | null = $state(null);
  let bulkWeightG: number | null = $state(null);
  let bulkLengthM: number | null = $state(null);
  let bulkSlicer: string | null = $state(null);
  let bulkSpoolId = $state(0);
  let bulkTariff: number | null = $state(null);
  let bulkPowerKW: number | null = $state(null);
  let bulkAmort: number | null = $state(null);
  let bulkMaint: number | null = $state(null);
  let bulkPurgeG: number | null = $state(null);
  let bulkOpRate: number | null = $state(null);
  let bulkPrep = $state({ h: null as number | null, m: null as number | null });
  let bulkPost = $state({ h: null as number | null, m: null as number | null });
  let bulkDesignerRate: number | null = $state(null);
  let bulkModel = $state({ h: null as number | null, m: null as number | null });
  let bulkModelPrice: number | null = $state(null);
  let bulkExtraMat: number | null = $state(null);
  let bulkFailPct: number | null = $state(null);
  let bulkMarginProfitPct: number | null = $state(null);
  let bulkMarginDevPct: number | null = $state(null);
  let bulkFees: number | null = $state(null);
  let bulkCopies: number | null = $state(null);
  let bulkVatPct: number | null = $state(null);
  let bulkRoundStep: number | null = $state(null);

  function openBulkEdit() {
    bulkOpen = true;
    bulkReview = null;
  }

  function closeBulkEdit() {
    bulkOpen = false;
    bulkReview = null;
  }

  // Материал bulk-правки выводится из выбранной катушки (как в одиночной).
  function bulkMaterial(): string {
    if (!bulkSpoolId) return '';
    const f = spoolFind(bulkSpoolId);
    return f ? f.mat.code : '';
  }

  function bulkPatch(): JobBulkPatch {
    const txt = (v: string | null) => (v == null || v.trim() === '' ? null : v.trim());
    const dec = (p: { h: number | null; m: number | null }) =>
      p.h == null && p.m == null ? null : joinHours(p.h, p.m);
    const pct = (v: number | null) => (v == null ? null : v / 100);
    return {
      status: bulkStatus === '' ? null : bulkStatus,
      startAt: txt(bulkStartAt), endAt: txt(bulkEndAt),
      printSec: bulkPrintSec, totalSec: bulkTotalSec,
      weightG: bulkWeightG, lengthM: bulkLengthM, slicer: txt(bulkSlicer),
      material: bulkMaterial(), spoolId: bulkSpoolId,
      tariff: bulkTariff, powerKW: bulkPowerKW,
      amortPerHour: bulkAmort, maintPerHour: bulkMaint,
      purgeWasteG: bulkPurgeG, opRate: bulkOpRate,
      prepH: dec(bulkPrep), postH: dec(bulkPost),
      designerRate: bulkDesignerRate, modelH: dec(bulkModel), modelPrice: bulkModelPrice,
      extraMat: bulkExtraMat, failRate: pct(bulkFailPct),
      marginProfit: pct(bulkMarginProfitPct), marginDev: pct(bulkMarginDevPct),
      fees: bulkFees, copies: bulkCopies,
      vatRate: pct(bulkVatPct), roundStep: bulkRoundStep,
    };
  }

  // Тронуто ли поле (для бледно-красной подсветки).
  function bulkTouched(kind: string): boolean {
    switch (kind) {
      case 'status': return bulkStatus !== '';
      case 'spool': return bulkSpoolId !== 0;
      case 'startAt': return (bulkStartAt ?? '').trim() !== '';
      case 'endAt': return (bulkEndAt ?? '').trim() !== '';
      case 'slicer': return (bulkSlicer ?? '').trim() !== '';
      case 'printSec': return bulkPrintSec != null;
      case 'totalSec': return bulkTotalSec != null;
      case 'weightG': return bulkWeightG != null;
      case 'lengthM': return bulkLengthM != null;
      case 'tariff': return bulkTariff != null;
      case 'powerKW': return bulkPowerKW != null;
      case 'amort': return bulkAmort != null;
      case 'maint': return bulkMaint != null;
      case 'purge': return bulkPurgeG != null;
      case 'opRate': return bulkOpRate != null;
      case 'prep': return bulkPrep.h != null || bulkPrep.m != null;
      case 'post': return bulkPost.h != null || bulkPost.m != null;
      case 'designer': return bulkDesignerRate != null;
      case 'model': return bulkModel.h != null || bulkModel.m != null;
      case 'modelPrice': return bulkModelPrice != null;
      case 'extra': return bulkExtraMat != null;
      case 'fail': return bulkFailPct != null;
      case 'marginProfit': return bulkMarginProfitPct != null;
      case 'marginDev': return bulkMarginDevPct != null;
      case 'fees': return bulkFees != null;
      case 'copies': return bulkCopies != null;
      case 'vat': return bulkVatPct != null;
      case 'round': return bulkRoundStep != null;
      default: return false;
    }
  }

  async function showBulkReview() {
    error = '';
    const ids = [...selected];
    if (!ids.length) return;
    try {
      bulkReview = await wails.bulkPreviewJobs(ids, bulkPatch());
    } catch (e) {
      error = String(e);
    }
  }

  async function applyBulk() {
    error = '';
    const ids = [...selected];
    if (!ids.length) return;
    try {
      const rep = await wails.bulkApplyJobs(ids, bulkPatch());
      closeBulkEdit();
      await load();
      notify(`${$t('hist.bulk_updated')}: ${rep.updated}`);
    } catch (e) {
      error = String(e);
    }
  }

  // Подписи полей просмотра (ключи i18n) и форматирование старое → новое.
  const bulkLabels: Record<string, string> = {
    status: 'hist.status', startAt: 'hist.start', endAt: 'hist.end',
    printSec: 'hist.print_sec', totalSec: 'hist.total_sec',
    weightG: 'hist.weight', lengthM: 'hist.length', slicer: 'hist.slicer',
    material: 'calc.material', spoolNo: 'calc.spool',
    tariff: 'calc.tariff', powerKW: 'calc.power', amortPerHour: 'calc.amort',
    maintPerHour: 'calc.maint', purgeWasteG: 'calc.purge', opRate: 'calc.op_rate',
    prepH: 'calc.prep_h', postH: 'calc.post_h', designerRate: 'calc.designer_rate',
    modelH: 'calc.model_h', modelPrice: 'calc.model_price', extraMat: 'calc.extra',
    failRate: 'calc.fail', marginProfit: 'calc.margin_profit', marginDev: 'calc.margin_dev',
    fees: 'calc.fees', copies: 'calc.copies', vatRate: 'calc.vat', roundStep: 'calc.round',
    cost: 'hist.cost', price: 'calc.price',
  };
  const bulkFieldOrder = [...Object.keys(bulkLabels)];

  function bulkLabel(field: string): string {
    const k = bulkLabels[field] ?? field;
    return k.includes('.') ? $t(k) : k;
  }

  function jobCurrency(jobId: number): string {
    const j = jobs.find((x) => x.ID === jobId);
    try {
      return JSON.parse(j?.CostSnapshot ?? '{}')?.currency ?? 'RUB';
    } catch {
      return 'RUB';
    }
  }

  function bulkVal(field: string, v: number | string | null, jobId: number): string {
    if (v == null) return '—';
    if (field === 'cost' || field === 'price') {
      return typeof v === 'number' ? money(v, jobCurrency(jobId), $locale) : String(v);
    }
    if (typeof v === 'number' && ['failRate', 'marginProfit', 'marginDev', 'vatRate'].includes(field)) {
      return `${Math.round(v * 10000) / 100}%`;
    }
    return String(v);
  }

  let bulkCols: string[] = $derived.by((): string[] => {
    const rows: BulkJobPreview[] = bulkReview ?? [];
    return [...new Set(rows.flatMap((p) => p.changes.map((c) => c.field)))]
      .sort((a, b) => bulkFieldOrder.indexOf(a) - bulkFieldOrder.indexOf(b));
  });
  let bulkHasChanges = $derived.by((): boolean => {
    const rows: BulkJobPreview[] = bulkReview ?? [];
    return rows.some((p) => p.changes.length > 0);
  });

  // Правка работы истории (manual и import) с автопересчётом на бэкенде.
  // Расчётные поля — как в Калькуляторе (проценты — %, в snapshot уйдут долями).
  // Катушка — отдельным селектом (editSpoolId), материал выводится из неё.
  let editId = $state(0);
  let editSpoolId = $state(0);
  let edit = $state({
    filename: '', status: 'completed', startAt: '', endAt: '',
    printSec: 0, totalSec: 0, weightG: 0, lengthM: 0, slicer: '', material: '',
    tariff: 0, powerKW: 0, amortPerHour: 0, maintPerHour: 0, purgeG: 0,
    opRate: 0, designerRate: 0, modelPrice: 0,
    extraMat: 0, failPct: 0, marginProfitPct: 0, marginDevPct: 0, fees: 0,
    copies: 1, vatPct: 0, roundStep: 0,
  });

  function snapSpoolNo(j: JobDTO): number {
    try {
      return Number(JSON.parse(j.CostSnapshot)?.input?.spoolNo) || 0;
    } catch {
      return 0;
    }
  }

  function spoolFind(id: number): { mat: MaterialDTO; spool: SpoolDTO } | null {
    for (const m of materials) {
      const s = (m.spools ?? []).find((x) => x.id === id);
      if (s) return { mat: m, spool: s };
    }
    return null;
  }

  function spoolExists(id: number): boolean {
    return spoolFind(id) != null;
  }

  function startEdit(j: JobDTO) {
    error = '';
    editId = j.ID;
    // Катушка из работы: явный линк, иначе номер из снапшота, иначе первая живая.
    // Материал выводится из катушки (снапшот — фолбэк).
    editSpoolId = 0;
    let code = snapMaterial(j);
    if (j.SpoolID && spoolExists(j.SpoolID)) {
      const f = spoolFind(j.SpoolID)!;
      editSpoolId = f.spool.id;
      code = f.mat.code;
    } else {
      const no = snapSpoolNo(j);
      const m = materials.find((x) => x.code === code);
      const hit = no > 0 ? (m?.spools ?? []).find((s) => s.spoolNo === no && !s.archived) : undefined;
      if (hit) {
        editSpoolId = hit.id;
      } else {
        const live = (m?.spools ?? []).find((s) => !s.archived);
        if (live) editSpoolId = live.id;
      }
    }
    // Вход из снапшота; чего нет (импорт) — дефолты Калькулятора.
    const d = calcFormDefaults;
    let inp: Record<string, number> = {};
    try {
      inp = JSON.parse(j.CostSnapshot)?.input ?? {};
    } catch {
      inp = {};
    }
    const pick = (k: string, fb: number | null) => (inp[k] != null ? inp[k] : (fb ?? 0));
    edit = {
      filename: j.Filename, status: j.Status, startAt: j.StartAt, endAt: j.EndAt,
      printSec: j.PrintSec, totalSec: j.TotalSec, weightG: j.WeightG,
      lengthM: j.LengthM, slicer: j.Slicer, material: code,
      tariff: pick('tariff', d.tariff), powerKW: pick('powerKW', d.powerKW),
      amortPerHour: pick('amortPerHour', d.amortPerHour),
      maintPerHour: pick('maintPerHour', d.maintPerHour),
      purgeG: pick('purgeWasteG', d.purgeG),
      opRate: pick('opRate', d.opRate), designerRate: pick('designerRate', d.designerRate),
      modelPrice: pick('modelPrice', d.modelPrice),
      extraMat: pick('extraMat', d.extraMat),
      failPct: pick('failRate', 0) * 100,
      marginProfitPct: pick('marginProfit', (d.marginProfitPct ?? 0) / 100) * 100,
      marginDevPct: pick('marginDev', (d.marginDevPct ?? 0) / 100) * 100,
      fees: pick('fees', d.fees), copies: Math.max(1, Math.round(pick('copies', 1))),
      vatPct: pick('vatRate', 0) * 100, roundStep: pick('roundStep', 0),
    };
    editPrep = splitHours(pick('prepH', d.prepH ?? 0));
    editPost = splitHours(pick('postH', d.postH ?? 0));
    editModel = splitHours(pick('modelH', d.modelH ?? 0));
    editOrig = snapshotEdit();
    tick().then(() => {
      (editCard?.querySelector('input, select, button') as HTMLElement | null)?.focus();
    });
  }

  // Слепок формы для dirty-check отмены.
  function snapshotEdit(): string {
    return JSON.stringify({ edit, editSpoolId, editPrep, editPost, editModel });
  }

  let editCard: HTMLDivElement | null = $state(null);
  let editOrig: string | null = $state(null);

  async function cancelEdit() {
    if (editOrig !== null && snapshotEdit() !== editOrig) {
      if (!(await askConfirm($t('hist.discard_q')))) return;
    }
    const id = editId;
    editId = 0;
    editOrig = null;
    requestAnimationFrame(() => {
      (document.querySelector(`[data-edit-job="${id}"]`) as HTMLElement | null)?.focus();
    });
  }

  // Время работ в правке — парами часы+минуты (в снапшот — десятичные часы).
  let editPrep = $state({ h: null as number | null, m: null as number | null });
  let editPost = $state({ h: null as number | null, m: null as number | null });
  let editModel = $state({ h: null as number | null, m: null as number | null });

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
        printSec: edit.printSec ?? 0, totalSec: edit.totalSec ?? 0,
        estimatedSec: 0, sizeBytes: 0,
        weightG: edit.weightG ?? 0, lengthM: edit.lengthM ?? 0,
        slicer: edit.slicer, material: edit.material.trim(), spoolId: editSpoolId,
        tariff: edit.tariff ?? 0, powerKW: edit.powerKW ?? 0,
        amortPerHour: edit.amortPerHour ?? 0, maintPerHour: edit.maintPerHour ?? 0,
        purgeWasteG: edit.purgeG ?? 0,
        opRate: edit.opRate ?? 0, prepH: joinHours(editPrep.h, editPrep.m), postH: joinHours(editPost.h, editPost.m),
        designerRate: edit.designerRate ?? 0, modelH: joinHours(editModel.h, editModel.m), modelPrice: edit.modelPrice ?? 0,
        extraMat: edit.extraMat ?? 0, failRate: (edit.failPct ?? 0) / 100,
        marginProfit: (edit.marginProfitPct ?? 0) / 100, marginDev: (edit.marginDevPct ?? 0) / 100,
        fees: edit.fees ?? 0, copies: Math.max(1, Math.round(edit.copies ?? 1)),
        vatRate: (edit.vatPct ?? 0) / 100, roundStep: edit.roundStep ?? 0,
      });
      const savedId = editId;
      editId = 0;
      await load();
      notify($t('common.saved'));
      if (savedId) flash('j:' + savedId);
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
  <div class="card" bind:this={editCard}>
    <h3>{$t('hist.edit_title')}</h3>
    <div class="row">
      <label>{$t('hist.filename')}<input bind:value={edit.filename} /></label>
      <label>{$t('hist.status')}
        <select bind:value={edit.status}>
          {#each ['completed', 'cancelled', 'error'] as s}<option value={s}>{s}</option>{/each}
        </select>
      </label>
      <label>{$t('calc.spool')}
        <select value={editSpoolId} onchange={(e) => {
          editSpoolId = Number(e.currentTarget.value);
          const f = editSpoolId ? spoolFind(editSpoolId) : null;
          if (f) edit.material = f.mat.code;
        }}>
          <option value={0}>—</option>
          {#each materials.filter((m) => (m.spools ?? []).some((s) => !s.archived) || (m.spools ?? []).some((s) => s.id === editSpoolId)) as m}
            <optgroup label={m.code}>
              {#each (m.spools ?? []).filter((s) => !s.archived || s.id === editSpoolId) as s}
                <option value={s.id}>{m.code}-{s.spoolNo || '?'} — {money(s.price, s.currency, $locale)}/кг</option>
              {/each}
            </optgroup>
          {/each}
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
      <label>{$t('calc.prep_h')}<input type="number" min="0" bind:value={editPrep.h} /></label>
      <label>{$t('calc.prep_m')}<input type="number" min="0" bind:value={editPrep.m} /></label>
      <label>{$t('calc.post_h')}<input type="number" min="0" bind:value={editPost.h} /></label>
      <label>{$t('calc.post_m')}<input type="number" min="0" bind:value={editPost.m} /></label>
      <label>{$t('calc.designer_rate')}<input type="number" min="0" bind:value={edit.designerRate} /></label>
      <label>{$t('calc.model_h')}<input type="number" min="0" bind:value={editModel.h} /></label>
      <label>{$t('calc.model_m')}<input type="number" min="0" bind:value={editModel.m} /></label>
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
      <button onclick={cancelEdit}>{$t('common.cancel')}</button>
    </div>
  </div>
{/if}
{#if bulkOpen}
  <div class="card">
    <h3>{$t('hist.bulk_edit')} ({selected.length})</h3>
    <div class="row">
      <label>{$t('hist.status')}
        <select bind:value={bulkStatus} class:changed={bulkTouched('status')}>
          <option value="">—</option>
          {#each ['completed', 'cancelled', 'error'] as s}<option value={s}>{statusLabel(s)}</option>{/each}
        </select>
      </label>
      <label>{$t('calc.spool')}
        <select bind:value={bulkSpoolId} class:changed={bulkTouched('spool')}>
          <option value={0}>—</option>
          {#each materials.filter((m) => (m.spools ?? []).some((s) => !s.archived)) as m}
            <optgroup label={m.code}>
              {#each (m.spools ?? []).filter((s) => !s.archived) as s}
                <option value={s.id}>{m.code}-{s.spoolNo || '?'} — {money(s.price, s.currency, $locale)}/кг</option>
              {/each}
            </optgroup>
          {/each}
        </select>
      </label>
      {#if bulkSpoolId}<span class="count">{bulkMaterial()}</span>{/if}
      <label>{$t('hist.slicer')}<input bind:value={bulkSlicer} placeholder="—" class:changed={bulkTouched('slicer')} /></label>
    </div>
    <div class="row">
      <label>{$t('hist.start')}<input bind:value={bulkStartAt} placeholder="—" class:changed={bulkTouched('startAt')} /></label>
      <label>{$t('hist.end')}<input bind:value={bulkEndAt} placeholder="—" class:changed={bulkTouched('endAt')} /></label>
      <label>{$t('hist.print_sec')}<input placeholder="—" type="number" min="0" bind:value={bulkPrintSec} class:changed={bulkTouched('printSec')} /></label>
      <label>{$t('hist.total_sec')}<input placeholder="—" type="number" min="0" bind:value={bulkTotalSec} class:changed={bulkTouched('totalSec')} /></label>
    </div>
    <div class="row">
      <label>{$t('hist.weight')}<input placeholder="—" type="number" min="0" step="0.01" bind:value={bulkWeightG} class:changed={bulkTouched('weightG')} /></label>
      <label>{$t('hist.length')}<input placeholder="—" type="number" min="0" step="0.01" bind:value={bulkLengthM} class:changed={bulkTouched('lengthM')} /></label>
    </div>
    <div class="row">
      <label>{$t('calc.tariff')}<input placeholder="—" type="number" min="0" step="0.01" bind:value={bulkTariff} class:changed={bulkTouched('tariff')} /></label>
      <label>{$t('calc.power')}<input placeholder="—" type="number" min="0" step="0.01" bind:value={bulkPowerKW} class:changed={bulkTouched('powerKW')} /></label>
      <label>{$t('calc.amort')}<input placeholder="—" type="number" min="0" step="0.01" bind:value={bulkAmort} class:changed={bulkTouched('amort')} /></label>
      <label>{$t('calc.maint')}<input placeholder="—" type="number" min="0" step="0.1" bind:value={bulkMaint} class:changed={bulkTouched('maint')} /></label>
      <label>{$t('calc.purge')}<input placeholder="—" type="number" min="0" step="0.1" bind:value={bulkPurgeG} class:changed={bulkTouched('purge')} /></label>
    </div>
    <div class="row">
      <label>{$t('calc.op_rate')}<input placeholder="—" type="number" min="0" bind:value={bulkOpRate} class:changed={bulkTouched('opRate')} /></label>
      <label>{$t('calc.prep_h')}<input placeholder="—" type="number" min="0" bind:value={bulkPrep.h} class:changed={bulkTouched('prep')} /></label>
      <label>{$t('calc.prep_m')}<input placeholder="—" type="number" min="0" max="59" bind:value={bulkPrep.m} class:changed={bulkTouched('prep')} /></label>
      <label>{$t('calc.post_h')}<input placeholder="—" type="number" min="0" bind:value={bulkPost.h} class:changed={bulkTouched('post')} /></label>
      <label>{$t('calc.post_m')}<input placeholder="—" type="number" min="0" max="59" bind:value={bulkPost.m} class:changed={bulkTouched('post')} /></label>
    </div>
    <div class="row">
      <label>{$t('calc.designer_rate')}<input placeholder="—" type="number" min="0" bind:value={bulkDesignerRate} class:changed={bulkTouched('designer')} /></label>
      <label>{$t('calc.model_h')}<input placeholder="—" type="number" min="0" bind:value={bulkModel.h} class:changed={bulkTouched('model')} /></label>
      <label>{$t('calc.model_m')}<input placeholder="—" type="number" min="0" max="59" bind:value={bulkModel.m} class:changed={bulkTouched('model')} /></label>
      <label>{$t('calc.model_price')}<input placeholder="—" type="number" min="0" bind:value={bulkModelPrice} class:changed={bulkTouched('modelPrice')} /></label>
      <label>{$t('calc.extra')}<input placeholder="—" type="number" min="0" bind:value={bulkExtraMat} class:changed={bulkTouched('extra')} /></label>
    </div>
    <div class="row">
      <label>{$t('calc.fail')}<input placeholder="—" type="number" min="0" step="0.1" bind:value={bulkFailPct} class:changed={bulkTouched('fail')} /></label>
      <label>{$t('calc.margin_profit')}<input placeholder="—" type="number" min="0" step="1" bind:value={bulkMarginProfitPct} class:changed={bulkTouched('marginProfit')} /></label>
      <label>{$t('calc.margin_dev')}<input placeholder="—" type="number" min="0" step="1" bind:value={bulkMarginDevPct} class:changed={bulkTouched('marginDev')} /></label>
      <label>{$t('calc.fees')}<input placeholder="—" type="number" min="0" bind:value={bulkFees} class:changed={bulkTouched('fees')} /></label>
      <label>{$t('calc.copies')}<input placeholder="—" type="number" min="1" step="1" bind:value={bulkCopies} class:changed={bulkTouched('copies')} /></label>
      <label>{$t('calc.vat')}<input placeholder="—" type="number" min="0" step="1" bind:value={bulkVatPct} class:changed={bulkTouched('vat')} /></label>
      <label>{$t('calc.round')}<input placeholder="—" type="number" min="0" step="1" bind:value={bulkRoundStep} class:changed={bulkTouched('round')} /></label>
    </div>
    <div class="row">
      <button onclick={showBulkReview}>{$t('hist.bulk_show')}</button>
      <button onclick={closeBulkEdit}>{$t('common.cancel')}</button>
    </div>
  </div>
{/if}
<div class="toolbar no-print">
  <input placeholder={$t('hist.search')} bind:value={query} oninput={() => (page = 0)} />
  <select bind:value={statusFilter} onchange={() => (page = 0)}>
    <option value="">— {$t('hist.status')} —</option>
    {#each statuses as s}<option value={s}>{statusLabel(s)}</option>{/each}
  </select>
  <select bind:value={originFilter} onchange={() => (page = 0)}>
    <option value="">— {$t('hist.origin')} —</option>
    {#each origins as o}<option value={o}>{originLabel(o)}</option>{/each}
  </select>
  <span class="count">{$t('hist.shown')}: {visible.length} / {filtered.length} / {jobs.length}</span>
  <button onclick={openColumns}>{$t('hist.columns_btn')}: {columns.length}/{allColumns.length}</button>
  {#if canWrite($user)}
    <span class="count">{$t('hist.selected_n')}: {selected.length}</span>
    <button onclick={openBulkEdit} disabled={!selected.length}>{$t('hist.bulk_edit')}</button>
    <button onclick={removeSelected} disabled={!selected.length}>{$t('hist.delete_selected')}</button>
    {#if selected.length}<button onclick={() => { selected = []; bulkMsg = ''; }}>{$t('hist.clear_sel')}</button>{/if}
  {/if}
  {#if bulkMsg}<span class="count">{bulkMsg}</span>{/if}
  <span class="pager">
    <button disabled={page === 0} onclick={() => page--} aria-label={$t('common.prev')}>‹</button>
    {page + 1} / {pages}
    <button disabled={page >= pages - 1} onclick={() => page++} aria-label={$t('common.next')}>›</button>
  </span>
</div>

<div class="twrap" bind:this={wrapEl}>
{#if dropX != null}<div class="drop-line no-print" style="left:{dropX}px;"></div>{/if}
<table class="tw" class:dragging={dragCol != null}>
  <thead><tr>
    {#if canWrite($user)}<th class="no-print selcol"><input type="checkbox" bind:this={headBox} checked={pageAll} onchange={togglePage} title={$t('hist.select_page')} /></th>{/if}
    {#each columns as c}
      <th class:n={isNum(c)} style={thWidth('history', c)} data-col={c} class:dragsrc={dragCol === c}>
        {#if canWrite($user)}<span class="grip no-print" role="button" tabindex={0} title={$t('hist.drag_cols')} onpointerdown={(e) => gripDown(e, c)} onkeydown={(e) => colKeyMove(e, columns, c, persistColumns)}>⋮⋮</span>{/if}
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
      <tr class:row-flash={flashKey === 'j:' + j.ID}>
        {#if canWrite($user)}<td class="no-print selcol"><input type="checkbox" checked={selected.includes(j.ID)} onchange={() => toggleSel(j.ID)} /></td>{/if}
        {#each columns as c}
          <td class:file={c === 'filename'} class:n={isNum(c)} title={c === 'filename' ? j.Filename : undefined}>{cell(j, c)}</td>
        {/each}
        <td class="no-print">
          <button onclick={() => toCalc(j)}>{$t('hist.to_calc')}</button>
          {#if canWrite($user)}
            <button onclick={() => startEdit(j)} title={$t('hist.edit_title')} aria-label={$t('hist.edit_title')} data-edit-job={j.ID}>✎</button>
            {#if j.Origin === 'manual' || j.Edited}
              <button onclick={() => remove(j)} title={j.Origin === 'manual' ? $t('common.delete_q') : $t('common.revert_q')} aria-label={j.Origin === 'manual' ? $t('common.delete_q') : $t('common.revert_q')}>{j.Origin === 'manual' ? '✕' : '↩'}</button>
            {:else}
              <button onclick={() => removeOne(j)} title={$t('common.delete_q')} aria-label={$t('common.delete_q')}>✕</button>
            {/if}
          {/if}
        </td>
      </tr>
    {/each}
  </tbody>
  <tfoot><tr>
    {#if canWrite($user)}<td class="no-print"></td>{/if}
    {#each columns as c}<td class:n={isNum(c)}><b>{footText(c)}</b></td>{/each}
    <td class="no-print"></td>
  </tr></tfoot>
</table>
</div>

{#if bulkReview}
  <div class="bOverlay no-print" role="dialog" aria-modal="true" aria-label={$t('hist.bulk_review')}>
    <div class="bCard">
      <h3>{$t('hist.bulk_review')} ({bulkReview.length})</h3>
      {#if !bulkHasChanges}
        <p class="note">{$t('hist.bulk_no_changes')}</p>
      {:else}
        <div class="twrap"><table class="tw">
          <thead><tr><th>{$t('hist.filename')}</th>{#each bulkCols as k}<th class="n">{bulkLabel(k)}</th>{/each}</tr></thead>
          <tbody>
            {#each bulkReview as p}
              <tr>
                <td class="file" title={p.filename}>{p.filename}</td>
                {#each bulkCols as k}
                  {@const ch = p.changes.find((c) => c.field === k)}
                  <td class="n">{ch ? `${bulkVal(ch.field, ch.old, p.jobId)} → ${bulkVal(ch.field, ch.new, p.jobId)}` : '—'}</td>
                {/each}
              </tr>
            {/each}
          </tbody>
        </table></div>
      {/if}
      <div class="row">
        <button onclick={applyBulk} disabled={!bulkHasChanges}>{$t('hist.bulk_apply')}</button>
        <button onclick={() => (bulkReview = null)}>{$t('common.cancel')}</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .toolbar { display: flex; gap: 8px; align-items: center; margin-bottom: 10px; flex-wrap: wrap; }
  .note { color: var(--muted); }
  input.changed, select.changed {
    background-color: color-mix(in srgb, var(--err) 12%, transparent);
  }
  .bOverlay {
    position: fixed; inset: 0; background: rgba(0, 0, 0, 0.35);
    display: flex; align-items: center; justify-content: center; z-index: 100;
  }
  .bCard {
    background: var(--bg); border: 1px solid var(--card-border); border-radius: 8px;
    padding: 16px; max-width: min(1000px, 92vw); max-height: 84vh; overflow: auto;
  }
  input, select, button { padding: 6px; font-size: 14px; }
  .count { color: var(--muted); font-size: 13px; }
  .pager { margin-left: auto; display: flex; gap: 4px; align-items: center; }
  table { font-size: 13px; }
  td.file { max-width: 380px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  @media print {
    td.file { max-width: 200px; }
  }
  button.th { border: none; background: none; font-weight: bold; padding: 0; cursor: pointer; font-size: 13px; }
  .err { color: var(--err); }
  .card { border: 1px solid var(--card-border); border-radius: 8px; padding: 12px; margin-bottom: 10px; }
  .card h3 { margin: 0 0 8px; font-size: 15px; }
  .row { display: flex; gap: 8px; margin-bottom: 8px; flex-wrap: wrap; }
  label { display: flex; flex-direction: column; font-size: 12px; flex: 1; min-width: 130px; }
</style>
