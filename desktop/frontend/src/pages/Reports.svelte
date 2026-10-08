<script lang="ts">
  import { onMount } from 'svelte';
  import { wails, type MonthRevenue, type MaterialStat, type PrinterStat, type Debt, type SpoolStat } from '../lib/wails';
  import { t } from '../lib/i18n';
  import { gotoOrder } from '../lib/nav';
  import { downloadPDF, registerPdfExporter } from '../lib/pdf';
  import { money, num, hms } from '../lib/format';

  let revenue: MonthRevenue[] = $state([]);
  let tops: MaterialStat[] = $state([]);
  let load: PrinterStat[] = $state([]);
  let debts: Debt[] = $state([]);
  let spools: SpoolStat[] = $state([]);
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
    } catch (e) {
      console.error(e);
    }
  }

  async function toPDF() {
    await downloadPDF($t('nav.reports'), [
      {
        Title: $t('rep.revenue'),
        Headers: [$t('rep.month'), $t('ord.total'), $t('rep.orders')],
        Rows: revenue.map((r) => [r.month, money(r.total / 100, 'RUB', 'ru'), String(r.orders)]),
      },
      {
        Title: $t('rep.debts'),
        Headers: ['#', $t('ord.number'), $t('ord.title'), $t('cust.name'), $t('ord.balance'), $t('ord.status')],
        Rows: debts.map((d) => [String(d.orderId), d.orderNo, debtTitle(d), d.customer, money(d.balance / 100, 'RUB', 'ru'), d.status]),
      },
      {
        Title: $t('rep.topmats'),
        Headers: [$t('mat.code'), $t('rep.jobs'), $t('hist.weight')],
        Rows: tops.map((m) => [m.code, String(m.jobs), num(m.weight)]),
      },
      {
        Title: $t('rep.load'),
        Headers: [$t('printer.name'), $t('rep.jobs'), $t('calc.time_h'), $t('rep.length'), $t('rep.weight'), $t('rep.remain')],
        Rows: load.map((p) => [p.name, String(p.jobs), hms(p.totalSec), num(p.lengthM), num(p.weightG), hms(Math.round(p.remainH * 3600))]),
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
    <table>
      <thead><tr><th>{$t('rep.month')}</th><th>{$t('ord.total')}</th><th>{$t('rep.orders')}</th></tr></thead>
      <tbody>{#each revenue as r}<tr><td>{r.month}</td><td class="n">{money(r.total / 100, 'RUB', 'ru')}</td><td class="n">{r.orders}</td></tr>{/each}</tbody>
    </table>
  </section>
  <section class="card">
    <h3>{$t('rep.debts')}</h3>
    <table>
      <thead><tr><th>#</th><th>{$t('ord.number')}</th><th>{$t('ord.title')}</th><th>{$t('cust.name')}</th><th class="n">{$t('ord.balance')}</th><th>{$t('ord.status')}</th></tr></thead>
      <tbody>{#each debts as d}<tr class="link" onclick={() => gotoOrder(d.orderId)}><td>{d.orderId}</td><td>{d.orderNo}</td><td>{debtTitle(d)}</td><td>{d.customer}</td><td class="n">{money(d.balance / 100, 'RUB', 'ru')}</td><td>{$t('st.' + d.status)}</td></tr>{/each}</tbody>
    </table>
  </section>
  <section class="card">
    <h3>{$t('rep.topmats')}</h3>
    <table>
      <thead><tr><th>{$t('mat.code')}</th><th>{$t('rep.jobs')}</th><th>{$t('hist.weight')}</th></tr></thead>
      <tbody>{#each tops as m}<tr><td>{m.code}</td><td class="n">{m.jobs}</td><td class="n">{num(m.weight)}</td></tr>{/each}</tbody>
    </table>
  </section>
  <section class="card">
    <h3>{$t('rep.load')}</h3>
    <table>
      <thead><tr><th>{$t('printer.name')}</th><th class="n">{$t('rep.jobs')}</th><th class="n">{$t('calc.time_h')}</th><th class="n">{$t('rep.length')}</th><th class="n">{$t('rep.weight')}</th><th class="n">{$t('rep.remain')}</th></tr></thead>
      <tbody>{#each load as p}<tr><td>{p.name}</td><td class="n">{p.jobs}</td><td class="n">{hms(p.totalSec)}</td><td class="n">{num(p.lengthM)}</td><td class="n">{num(p.weightG)}</td><td class="n">{hms(Math.round(p.remainH * 3600))}</td></tr>{/each}</tbody>
    </table>
  </section>
  <section class="card wide">
    <h3>{$t('rep.spools')}</h3>
    <table>
      <thead><tr>
        <th><button class="th" onclick={() => spoolSortBy('code')}>{$t('mat.code')}{spoolArrow('code')}</button></th>
        <th>{$t('mat.color')}</th>
        <th><button class="th" onclick={() => spoolSortBy('vendor')}>{$t('mat.vendor')}{spoolArrow('vendor')}</button></th>
        <th class="n"><button class="th" onclick={() => spoolSortBy('jobs')}>{$t('rep.jobs')}{spoolArrow('jobs')}</button></th>
        <th class="n"><button class="th" onclick={() => spoolSortBy('used')}>{$t('rep.used')}{spoolArrow('used')}</button></th>
        <th class="n"><button class="th" onclick={() => spoolSortBy('left')}>{$t('rep.left')}{spoolArrow('left')}</button></th>
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
  .sw { display: inline-block; width: 18px; height: 18px; border: 1px solid var(--sw-border); border-radius: 3px; vertical-align: middle; }
</style>
