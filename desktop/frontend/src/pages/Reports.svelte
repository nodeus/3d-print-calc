<script lang="ts">
  import { onMount } from 'svelte';
  import { wails, type MonthRevenue, type MaterialStat, type PrinterStat, type Debt, type SpoolStat } from '../lib/wails';
  import { t, locale } from '../lib/i18n';
  import { gotoOrder } from '../lib/nav';
  import { downloadPDF, registerPdfExporter } from '../lib/pdf';
  import { money, num, hms } from '../lib/format';
  import { thWidth, startColResize, resetColWidth } from '../lib/colwidths';

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
        Headers: [$t('rep.month'), $t('ord.total'), $t('rep.orders')],
        Rows: revenueSorted.map((r) => [r.month, money(r.total / 100, baseCur, $locale), String(r.orders)]),
      },
      {
        Title: $t('rep.debts'),
        Headers: ['#', $t('ord.number'), $t('ord.title'), $t('cust.name'), $t('ord.balance'), $t('ord.status')],
        Rows: debtsSorted.map((d) => [String(d.orderId), d.orderNo, debtTitle(d), d.customer, money(d.balance / 100, baseCur, $locale), d.status]),
      },
      {
        Title: $t('rep.topmats'),
        Headers: [$t('mat.code'), $t('rep.jobs'), $t('hist.weight')],
        Rows: topsSorted.map((m) => [m.code, String(m.jobs), num(m.weight)]),
      },
      {
        Title: $t('rep.load'),
        Headers: [$t('printer.name'), $t('rep.jobs'), $t('calc.time_h'), $t('rep.length'), $t('rep.weight'), $t('rep.remain')],
        Rows: loadSorted.map((p) => [p.name, String(p.jobs), hms(p.totalSec), num(p.lengthM), num(p.weightG), hms(Math.round(p.remainH * 3600))]),
      },
      {
        Title: $t('rep.spools'),
        Headers: [$t('mat.code'), $t('mat.color'), $t('mat.vendor'), $t('rep.jobs'), $t('rep.used'), $t('rep.left')],
        Rows: spoolsSorted.map((s) => [s.code, s.colorHex, s.vendor, String(s.jobs), num(s.usedG), num(s.leftG)]),
      },
    ]);
  }
</script>

<div class="grid">
  <section class="card">
    <h3>{$t('rep.revenue')}</h3>
    <table class="tw">
      <thead><tr>
        {#each [{ key: 'month', label: 'rep.month' }, { key: 'total', label: 'ord.total' }, { key: 'orders', label: 'rep.orders' }] as c}
          <th style={thWidth('repRevenue', c.key)}>
            <button class="th" onclick={() => tg(revSort, c.key)}>{$t(c.label)}{ar(revSort, c.key)}</button>
            <span class="rz no-print" role="separator" onmousedown={(e) => startColResize(e, 'repRevenue', c.key)} ondblclick={() => resetColWidth('repRevenue', c.key)}></span>
          </th>
        {/each}
      </tr></thead>
      <tbody>{#each revenueSorted as r}<tr><td>{r.month}</td><td class="n">{money(r.total / 100, baseCur, $locale)}</td><td class="n">{r.orders}</td></tr>{/each}</tbody>
    </table>
  </section>
  <section class="card">
    <h3>{$t('rep.debts')}</h3>
    <table class="tw">
      <thead><tr>
        {#each [{ key: 'orderId', label: '#' }, { key: 'orderNo', label: 'ord.number' }, { key: 'title', label: 'ord.title' }, { key: 'customer', label: 'cust.name' }, { key: 'balance', label: 'ord.balance' }, { key: 'status', label: 'ord.status' }] as c}
          <th style={thWidth('repDebts', c.key)}>
            <button class="th" onclick={() => tg(debtSort, c.key)}>{c.key === 'orderId' ? '#' : $t(c.label)}{ar(debtSort, c.key)}</button>
            <span class="rz no-print" role="separator" onmousedown={(e) => startColResize(e, 'repDebts', c.key)} ondblclick={() => resetColWidth('repDebts', c.key)}></span>
          </th>
        {/each}
      </tr></thead>
      <tbody>{#each debtsSorted as d}<tr class="link" onclick={() => gotoOrder(d.orderId)}><td>{d.orderId}</td><td>{d.orderNo}</td><td>{debtTitle(d)}</td><td>{d.customer}</td><td class="n">{money(d.balance / 100, baseCur, $locale)}</td><td>{$t('st.' + d.status)}</td></tr>{/each}</tbody>
    </table>
  </section>
  <section class="card">
    <h3>{$t('rep.topmats')}</h3>
    <table class="tw">
      <thead><tr>
        {#each [{ key: 'code', label: 'mat.code' }, { key: 'jobs', label: 'rep.jobs' }, { key: 'weight', label: 'hist.weight' }] as c}
          <th style={thWidth('repTopmats', c.key)}>
            <button class="th" onclick={() => tg(topSort, c.key)}>{$t(c.label)}{ar(topSort, c.key)}</button>
            <span class="rz no-print" role="separator" onmousedown={(e) => startColResize(e, 'repTopmats', c.key)} ondblclick={() => resetColWidth('repTopmats', c.key)}></span>
          </th>
        {/each}
      </tr></thead>
      <tbody>{#each topsSorted as m}<tr><td>{m.code}</td><td class="n">{m.jobs}</td><td class="n">{num(m.weight)}</td></tr>{/each}</tbody>
    </table>
  </section>
  <section class="card">
    <h3>{$t('rep.load')}</h3>
    <table class="tw">
      <thead><tr>
        {#each [{ key: 'name', label: 'printer.name' }, { key: 'jobs', label: 'rep.jobs' }, { key: 'time', label: 'calc.time_h' }, { key: 'length', label: 'rep.length' }, { key: 'weight', label: 'rep.weight' }, { key: 'remain', label: 'rep.remain' }] as c}
          <th style={thWidth('repLoad', c.key)}>
            <button class="th" onclick={() => tg(loadSort, c.key)}>{$t(c.label)}{ar(loadSort, c.key)}</button>
            <span class="rz no-print" role="separator" onmousedown={(e) => startColResize(e, 'repLoad', c.key)} ondblclick={() => resetColWidth('repLoad', c.key)}></span>
          </th>
        {/each}
      </tr></thead>
      <tbody>{#each loadSorted as p}<tr><td>{p.name}</td><td class="n">{p.jobs}</td><td class="n">{hms(p.totalSec)}</td><td class="n">{num(p.lengthM)}</td><td class="n">{num(p.weightG)}</td><td class="n">{hms(Math.round(p.remainH * 3600))}</td></tr>{/each}</tbody>
    </table>
  </section>
  <section class="card wide">
    <h3>{$t('rep.spools')}</h3>
    <table class="tw">
      <thead><tr>
        {#each [{ key: 'code', label: 'mat.code' }, { key: 'color', label: 'mat.color' }, { key: 'vendor', label: 'mat.vendor' }, { key: 'jobs', label: 'rep.jobs' }, { key: 'used', label: 'rep.used' }, { key: 'left', label: 'rep.left' }] as c}
          <th style={thWidth('repSpools', c.key)}>
            {#if c.key === 'color'}
              {$t(c.label)}
            {:else}
              <button class="th" onclick={() => spoolSortBy(c.key)}>{$t(c.label)}{spoolArrow(c.key)}</button>
            {/if}
            <span class="rz no-print" role="separator" onmousedown={(e) => startColResize(e, 'repSpools', c.key)} ondblclick={() => resetColWidth('repSpools', c.key)}></span>
          </th>
        {/each}
      </tr></thead>
      <tbody>{#each spoolsSorted as s}<tr><td>{s.code}</td><td><span class="sw" style="background:{s.colorHex || 'transparent'}" title={s.colorHex}></span></td><td>{s.vendor}</td><td class="n">{s.jobs}</td><td class="n">{num(s.usedG)}</td><td class="n">{num(s.leftG)}</td></tr>{/each}</tbody>
    </table>
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
