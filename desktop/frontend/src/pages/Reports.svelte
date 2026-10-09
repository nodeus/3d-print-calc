<script lang="ts">
  import { onMount } from 'svelte';
  import { wails, type MonthRevenue, type MaterialStat, type PrinterStat, type Debt, type SpoolStat } from '../lib/wails';
  import { t, locale } from '../lib/i18n';
  import { gotoOrder } from '../lib/nav';
  import { downloadPDF, registerPdfExporter } from '../lib/pdf';
  import { money, num, hms } from '../lib/format';
  import { thWidth, startColResize, resetColWidth } from '../lib/colwidths';
  import { mySettings } from '../lib/usersettings';
  import { orderFor, saveTableOrder } from '../lib/colorder';
  import { startColDrag } from '../lib/coldrag';
  import { get } from 'svelte/store';
  import { user, canWrite } from '../lib/session';

  let revenue: MonthRevenue[] = $state([]);
  let tops: MaterialStat[] = $state([]);
  let load: PrinterStat[] = $state([]);
  let debts: Debt[] = $state([]);
  let spools: SpoolStat[] = $state([]);
  let baseCur = $state('RUB');

  function cmpVals(a: string | number, b: string | number, dir: 1 | -1): number {
    const cmp = typeof a === 'number' ? a - (b as number) : String(a).localeCompare(String(b));
    return cmp * dir;
  }

  function mkSort(defKey: string) {
    return { key: defKey, dir: 1 as 1 | -1 };
  }

  let revSort = $state(mkSort('month'));
  let debtSort = $state(mkSort('orderId'));
  let topSort = $state(mkSort('code'));
  let loadSort = $state(mkSort('name'));

  function tg(s: { key: string; dir: 1 | -1 }, key: string) {
    if (s.key === key) s.dir = s.dir === 1 ? -1 : 1;
    else { s.key = key; s.dir = 1; }
  }

  function ar(s: { key: string; dir: 1 | -1 }, key: string): string {
    return s.key === key ? (s.dir === 1 ? ' ▲' : ' ▼') : '';
  }
  let spoolSort: string = $state('code');
  let spoolDir: 1 | -1 = $state(1);

  function spoolVal(s: SpoolStat): string | number {
    switch (spoolSort) {
      case 'code':
        return s.code.toLowerCase();
      case 'vendor':
        return (s.vendor || '').toLowerCase();
      case 'jobs':
        return s.jobs;
      case 'used':
        return s.usedG;
      case 'left':
        return s.leftG;
      default:
        return s.code.toLowerCase();
    }
  }

  let spoolsSorted = $derived(
    [...spools].sort((a, b) => {
      const va = spoolVal(a);
      const vb = spoolVal(b);
      const cmp = typeof va === 'number' ? va - (vb as number) : String(va).localeCompare(String(vb));
      return cmp * spoolDir;
    }),
  );

  function spoolSortBy(key: string) {
    if (spoolSort === key) spoolDir = spoolDir === 1 ? -1 : 1;
    else {
      spoolSort = key;
      spoolDir = 1;
    }
  }

  function spoolArrow(key: string): string {
    return spoolSort === key ? (spoolDir === 1 ? ' ▲' : ' ▼') : '';
  }

  function revVal(r: MonthRevenue): string | number {
    switch (revSort.key) {
      case 'total': return r.total;
      case 'orders': return r.orders;
      default: return r.month;
    }
  }

  function debtVal(d: Debt): string | number {
    switch (debtSort.key) {
      case 'orderNo': return (d.orderNo || '').toLowerCase();
      case 'title': return debtTitle(d).toLowerCase();
      case 'customer': return (d.customer || '').toLowerCase();
      case 'balance': return d.balance;
      case 'status': return d.status;
      default: return d.orderId;
    }
  }

  function topVal(m: MaterialStat): string | number {
    switch (topSort.key) {
      case 'jobs': return m.jobs;
      case 'weight': return m.weight;
      default: return m.code.toLowerCase();
    }
  }

  function loadVal(p: PrinterStat): string | number {
    switch (loadSort.key) {
      case 'jobs': return p.jobs;
      case 'time': return p.totalSec;
      case 'length': return p.lengthM;
      case 'weight': return p.weightG;
      case 'remain': return p.remainH;
      default: return p.name.toLowerCase();
    }
  }

  let revenueSorted = $derived([...revenue].sort((a, b) => cmpVals(revVal(a), revVal(b), revSort.dir)));
  let debtsSorted = $derived([...debts].sort((a, b) => cmpVals(debtVal(a), debtVal(b), debtSort.dir)));
  let topsSorted = $derived([...tops].sort((a, b) => cmpVals(topVal(a), topVal(b), topSort.dir)));
  let loadSorted = $derived([...load].sort((a, b) => cmpVals(loadVal(a), loadVal(b), loadSort.dir)));

  // Порядок колонок (drag за грип), тексты ячеек, PDF — в экранном порядке.
  const revKeys = ['month', 'total', 'orders'];
  const debtKeys = ['orderId', 'orderNo', 'title', 'customer', 'balance', 'status'];
  const topKeys = ['code', 'jobs', 'weight'];
  const loadKeys = ['name', 'jobs', 'time', 'length', 'weight', 'remain'];
  const spoolKeys = ['code', 'color', 'vendor', 'jobs', 'used', 'left'];

  let revOrd = $derived(orderFor($mySettings.colOrder, 'repRevenue', revKeys));
  let debtOrd = $derived(orderFor($mySettings.colOrder, 'repDebts', debtKeys));
  let topOrd = $derived(orderFor($mySettings.colOrder, 'repTopmats', topKeys));
  let loadOrd = $derived(orderFor($mySettings.colOrder, 'repLoad', loadKeys));
  let spoolOrd = $derived(orderFor($mySettings.colOrder, 'repSpools', spoolKeys));

  function revText(r: MonthRevenue, k: string): string {
    switch (k) {
      case 'month': return r.month;
      case 'total': return money(r.total / 100, baseCur, $locale);
      case 'orders': return String(r.orders);
      default: return '';
    }
  }

  function debtText(d: Debt, k: string): string {
    switch (k) {
      case 'orderId': return String(d.orderId);
      case 'orderNo': return d.orderNo;
      case 'title': return debtTitle(d);
      case 'customer': return d.customer;
      case 'balance': return money(d.balance / 100, baseCur, $locale);
      case 'status': return $t('st.' + d.status);
      default: return '';
    }
  }

  function topText(m: MaterialStat, k: string): string {
    switch (k) {
      case 'code': return m.code;
      case 'jobs': return String(m.jobs);
      case 'weight': return num(m.weight);
      default: return '';
    }
  }

  function loadText(p: PrinterStat, k: string): string {
    switch (k) {
      case 'name': return p.name;
      case 'jobs': return String(p.jobs);
      case 'time': return hms(p.totalSec);
      case 'length': return num(p.lengthM);
      case 'weight': return num(p.weightG);
      case 'remain': return hms(Math.round(p.remainH * 3600));
      default: return '';
    }
  }

  function spoolText(s: SpoolStat, k: string): string {
    switch (k) {
      case 'code': return s.code;
      case 'vendor': return s.vendor;
      case 'jobs': return String(s.jobs);
      case 'used': return num(s.usedG);
      case 'left': return num(s.leftG);
      default: return '';
    }
  }

  // Подписи и числовые колонки по таблицам.
  const repLabels: Record<string, Record<string, string>> = {
    repRevenue: { month: 'rep.month', total: 'ord.total', orders: 'rep.orders' },
    repDebts: { orderId: '#', orderNo: 'ord.number', title: 'ord.title', customer: 'cust.name', balance: 'ord.balance', status: 'ord.status' },
    repTopmats: { code: 'mat.code', jobs: 'rep.jobs', weight: 'hist.weight' },
    repLoad: { name: 'printer.name', jobs: 'rep.jobs', time: 'calc.time_h', length: 'rep.length', weight: 'rep.weight', remain: 'rep.remain' },
    repSpools: { code: 'mat.code', color: 'mat.color', vendor: 'mat.vendor', jobs: 'rep.jobs', used: 'rep.used', left: 'rep.left' },
  };
  const repNum: Record<string, string[]> = {
    repRevenue: ['total', 'orders'], repDebts: ['balance'],
    repTopmats: ['jobs', 'weight'], repLoad: ['jobs', 'time', 'length', 'weight', 'remain'],
    repSpools: ['jobs', 'used', 'left'],
  };
  const repLabel = (t: string, k: string) => repLabels[t]?.[k] ?? k;
  const repIsNum = (t: string, k: string) => (repNum[t] ?? []).includes(k);
  let dragKey: string | null = $state(null);
  let dragTable = $state('');
  let dropX: number | null = $state(null);
  let wrapRev: HTMLDivElement | null = $state(null);
  let wrapDebt: HTMLDivElement | null = $state(null);
  let wrapTop: HTMLDivElement | null = $state(null);
  let wrapLoad: HTMLDivElement | null = $state(null);
  let wrapSpool: HTMLDivElement | null = $state(null);

  async function dropCols(table: string, next: string[]) {
    try {
      await saveTableOrder(table, next);
    } catch (e) {
      console.error(e);
    }
  }

  function gripDown(e: PointerEvent, table: string, wrap: HTMLDivElement | null, k: string, ord: string[]) {
    if (!canWrite(get(user))) return;
    dragTable = table;
    startColDrag(e, wrap, k, ord, (dk, x) => { dragKey = dk; dropX = x; }, (next) => dropCols(table, next));
  }

  function debtTitle(d: Debt): string {
    return d.title || d.orderNo || `#${d.orderId}`;
  }

  onMount(() => {
    loadAll();
    registerPdfExporter(toPDF);
    return () => registerPdfExporter(null);
  });

  async function loadAll() {
    try {
      [revenue, tops, load, debts] = (await Promise.all([
        wails.revenueByMonth(),
        wails.topMaterials(),
        wails.printerLoad(),
        wails.debts(),
      ]).then((r) => r.map((x) => x ?? []))) as [MonthRevenue[], MaterialStat[], PrinterStat[], Debt[]];
      spools = (await wails.spoolStats()) ?? [];
      baseCur = await wails.getBaseCurrency();
    } catch (e) {
      console.error(e);
    }
  }

  async function toPDF() {
    await downloadPDF($t('nav.reports'), [
      {
        Title: $t('rep.revenue'),
        Headers: revOrd.map((k) => $t(repLabel('repRevenue', k))),
        Rows: revenueSorted.map((r) => revOrd.map((k) => revText(r, k))),
      },
      {
        Title: $t('rep.debts'),
        Headers: debtOrd.map((k) => (k === 'orderId' ? '#' : $t(repLabel('repDebts', k)))),
        Rows: debtsSorted.map((d) => debtOrd.map((k) => debtText(d, k))),
      },
      {
        Title: $t('rep.topmats'),
        Headers: topOrd.map((k) => $t(repLabel('repTopmats', k))),
        Rows: topsSorted.map((m) => topOrd.map((k) => topText(m, k))),
      },
      {
        Title: $t('rep.load'),
        Headers: loadOrd.map((k) => $t(repLabel('repLoad', k))),
        Rows: loadSorted.map((p) => loadOrd.map((k) => loadText(p, k))),
      },
      {
        Title: $t('rep.spools'),
        Headers: spoolOrd.map((k) => $t(repLabel('repSpools', k))),
        Rows: spoolsSorted.map((s) => spoolOrd.map((k) => (k === 'color' ? s.colorHex : spoolText(s, k)))),
      },
    ]);
  }
</script>

<div class="grid">
  <section class="card">
    <h3>{$t('rep.revenue')}</h3>
    <div class="twrap" bind:this={wrapRev}>
    {#if dragTable === 'repRevenue' && dropX != null}<div class="drop-line no-print" style="left:{dropX}px;"></div>{/if}
    <table class="tw" class:dragging={dragTable === 'repRevenue' && dragKey != null}>
      <thead><tr>
        {#each revOrd as k}
          <th class:n={repIsNum('repRevenue', k)} style={thWidth('repRevenue', k)} data-col={k} class:dragsrc={dragTable === 'repRevenue' && dragKey === k}>
            {#if canWrite($user)}<span class="grip no-print" role="separator" title={$t('hist.drag_cols')} onpointerdown={(e) => gripDown(e, 'repRevenue', wrapRev, k, revOrd)}>⋮⋮</span>{/if}
            <button class="th" onclick={() => tg(revSort, k)}>{$t(repLabel('repRevenue', k))}{ar(revSort, k)}</button>
            <span class="rz no-print" role="separator" onmousedown={(e) => startColResize(e, 'repRevenue', k)} ondblclick={() => resetColWidth('repRevenue', k)}></span>
          </th>
        {/each}
      </tr></thead>
      <tbody>{#each revenueSorted as r}<tr>{#each revOrd as k}<td class:n={repIsNum('repRevenue', k)}>{revText(r, k)}</td>{/each}</tr>{/each}</tbody>
    </table>
    </div>
  </section>
  <section class="card">
    <h3>{$t('rep.debts')}</h3>
    <div class="twrap" bind:this={wrapDebt}>
    {#if dragTable === 'repDebts' && dropX != null}<div class="drop-line no-print" style="left:{dropX}px;"></div>{/if}
    <table class="tw" class:dragging={dragTable === 'repDebts' && dragKey != null}>
      <thead><tr>
        {#each debtOrd as k}
          <th class:n={repIsNum('repDebts', k)} style={thWidth('repDebts', k)} data-col={k} class:dragsrc={dragTable === 'repDebts' && dragKey === k}>
            {#if canWrite($user)}<span class="grip no-print" role="separator" title={$t('hist.drag_cols')} onpointerdown={(e) => gripDown(e, 'repDebts', wrapDebt, k, debtOrd)}>⋮⋮</span>{/if}
            <button class="th" onclick={() => tg(debtSort, k)}>{k === 'orderId' ? '#' : $t(repLabel('repDebts', k))}{ar(debtSort, k)}</button>
            <span class="rz no-print" role="separator" onmousedown={(e) => startColResize(e, 'repDebts', k)} ondblclick={() => resetColWidth('repDebts', k)}></span>
          </th>
        {/each}
      </tr></thead>
      <tbody>{#each debtsSorted as d}<tr class="link" onclick={() => gotoOrder(d.orderId)}>{#each debtOrd as k}<td class:n={repIsNum('repDebts', k)}>{debtText(d, k)}</td>{/each}</tr>{/each}</tbody>
    </table>
    </div>
  </section>
  <section class="card">
    <h3>{$t('rep.topmats')}</h3>
    <div class="twrap" bind:this={wrapTop}>
    {#if dragTable === 'repTopmats' && dropX != null}<div class="drop-line no-print" style="left:{dropX}px;"></div>{/if}
    <table class="tw" class:dragging={dragTable === 'repTopmats' && dragKey != null}>
      <thead><tr>
        {#each topOrd as k}
          <th class:n={repIsNum('repTopmats', k)} style={thWidth('repTopmats', k)} data-col={k} class:dragsrc={dragTable === 'repTopmats' && dragKey === k}>
            {#if canWrite($user)}<span class="grip no-print" role="separator" title={$t('hist.drag_cols')} onpointerdown={(e) => gripDown(e, 'repTopmats', wrapTop, k, topOrd)}>⋮⋮</span>{/if}
            <button class="th" onclick={() => tg(topSort, k)}>{$t(repLabel('repTopmats', k))}{ar(topSort, k)}</button>
            <span class="rz no-print" role="separator" onmousedown={(e) => startColResize(e, 'repTopmats', k)} ondblclick={() => resetColWidth('repTopmats', k)}></span>
          </th>
        {/each}
      </tr></thead>
      <tbody>{#each topsSorted as m}<tr>{#each topOrd as k}<td class:n={repIsNum('repTopmats', k)}>{topText(m, k)}</td>{/each}</tr>{/each}</tbody>
    </table>
    </div>
  </section>
  <section class="card">
    <h3>{$t('rep.load')}</h3>
    <div class="twrap" bind:this={wrapLoad}>
    {#if dragTable === 'repLoad' && dropX != null}<div class="drop-line no-print" style="left:{dropX}px;"></div>{/if}
    <table class="tw" class:dragging={dragTable === 'repLoad' && dragKey != null}>
      <thead><tr>
        {#each loadOrd as k}
          <th class:n={repIsNum('repLoad', k)} style={thWidth('repLoad', k)} data-col={k} class:dragsrc={dragTable === 'repLoad' && dragKey === k}>
            {#if canWrite($user)}<span class="grip no-print" role="separator" title={$t('hist.drag_cols')} onpointerdown={(e) => gripDown(e, 'repLoad', wrapLoad, k, loadOrd)}>⋮⋮</span>{/if}
            <button class="th" onclick={() => tg(loadSort, k)}>{$t(repLabel('repLoad', k))}{ar(loadSort, k)}</button>
            <span class="rz no-print" role="separator" onmousedown={(e) => startColResize(e, 'repLoad', k)} ondblclick={() => resetColWidth('repLoad', k)}></span>
          </th>
        {/each}
      </tr></thead>
      <tbody>{#each loadSorted as p}<tr>{#each loadOrd as k}<td class:n={repIsNum('repLoad', k)}>{loadText(p, k)}</td>{/each}</tr>{/each}</tbody>
    </table>
    </div>
  </section>
  <section class="card wide">
    <h3>{$t('rep.spools')}</h3>
    <div class="twrap" bind:this={wrapSpool}>
    {#if dragTable === 'repSpools' && dropX != null}<div class="drop-line no-print" style="left:{dropX}px;"></div>{/if}
    <table class="tw" class:dragging={dragTable === 'repSpools' && dragKey != null}>
      <thead><tr>
        {#each spoolOrd as k}
          <th class:n={repIsNum('repSpools', k)} style={thWidth('repSpools', k)} data-col={k} class:dragsrc={dragTable === 'repSpools' && dragKey === k}>
            {#if canWrite($user)}<span class="grip no-print" role="separator" title={$t('hist.drag_cols')} onpointerdown={(e) => gripDown(e, 'repSpools', wrapSpool, k, spoolOrd)}>⋮⋮</span>{/if}
            {#if k === 'color'}
              {$t(repLabel('repSpools', k))}
            {:else}
              <button class="th" onclick={() => spoolSortBy(k)}>{$t(repLabel('repSpools', k))}{spoolArrow(k)}</button>
            {/if}
            <span class="rz no-print" role="separator" onmousedown={(e) => startColResize(e, 'repSpools', k)} ondblclick={() => resetColWidth('repSpools', k)}></span>
          </th>
        {/each}
      </tr></thead>
      <tbody>{#each spoolsSorted as s}<tr>{#each spoolOrd as k}{#if k === 'color'}<td><span class="sw" style="background:{s.colorHex || 'transparent'}" title={s.colorHex}></span></td>{:else}<td class:n={repIsNum('repSpools', k)}>{spoolText(s, k)}</td>{/if}{/each}</tr>{/each}</tbody>
    </table>
    </div>
  </section>
</div>

<style>
  .grid { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
  .card { border: 1px solid var(--card-border); border-radius: 8px; padding: 12px; }
  .card.wide { grid-column: 1 / -1; }
  h3 { margin: 0 0 8px; font-size: 15px; }
  table { font-size: 13px; }
  th.n, td.n { text-align: right; font-variant-numeric: tabular-nums; }
  tr.link { cursor: pointer; }
  button.th { border: none; background: none; font-weight: bold; padding: 0; cursor: pointer; font-size: 13px; }
  .sw { display: inline-block; width: 18px; height: 18px; border: 1px solid var(--sw-border); border-radius: 3px; vertical-align: middle; print-color-adjust: exact; }
</style>
