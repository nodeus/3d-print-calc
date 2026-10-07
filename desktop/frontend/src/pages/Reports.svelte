<script lang="ts">
  import { onMount } from 'svelte';
  import { wails, type MonthRevenue, type MaterialStat, type PrinterStat, type Debt } from '../lib/wails';
  import { t } from '../lib/i18n';
  import { gotoOrder } from '../lib/nav';
  import { downloadPDF } from '../lib/pdf';
  import { money, num, hms } from '../lib/format';

  let revenue: MonthRevenue[] = $state([]);
  let tops: MaterialStat[] = $state([]);
  let load: PrinterStat[] = $state([]);
  let debts: Debt[] = $state([]);

  onMount(async () => {
    try {
      [revenue, tops, load, debts] = (await Promise.all([
        wails.revenueByMonth(),
        wails.topMaterials(),
        wails.printerLoad(),
        wails.debts(),
      ]).then((r) => r.map((x) => x ?? []))) as [MonthRevenue[], MaterialStat[], PrinterStat[], Debt[]];
    } catch (e) {
      console.error(e);
    }
  });

  async function toPDF() {
    await downloadPDF($t('nav.reports'), [
      {
        Title: $t('rep.revenue'),
        Headers: [$t('rep.month'), $t('ord.total'), $t('rep.orders')],
        Rows: revenue.map((r) => [r.month, money(r.total / 100, 'RUB', 'ru'), String(r.orders)]),
      },
      {
        Title: $t('rep.debts'),
        Headers: ['#', $t('cust.name'), $t('ord.balance'), $t('ord.status')],
        Rows: debts.map((d) => [String(d.orderId), d.customer, money(d.balance / 100, 'RUB', 'ru'), d.status]),
      },
      {
        Title: $t('rep.topmats'),
        Headers: [$t('mat.code'), $t('rep.jobs'), $t('hist.weight')],
        Rows: tops.map((m) => [m.code, String(m.jobs), num(m.weight)]),
      },
      {
        Title: $t('rep.load'),
        Headers: [$t('printer.name'), $t('rep.jobs'), $t('calc.time_h')],
        Rows: load.map((p) => [p.name, String(p.jobs), hms(p.totalSec)]),
      },
    ]);
  }
</script>

<div class="grid">
  <div class="row no-print"><button onclick={toPDF}>{$t('common.pdf')}</button></div>
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
      <thead><tr><th>#</th><th>{$t('cust.name')}</th><th class="n">{$t('ord.balance')}</th><th>{$t('ord.status')}</th></tr></thead>
      <tbody>{#each debts as d}<tr class="link" onclick={() => gotoOrder(d.orderId)}><td>{d.orderId}</td><td>{d.customer}</td><td class="n">{money(d.balance / 100, 'RUB', 'ru')}</td><td>{$t('st.' + d.status)}</td></tr>{/each}</tbody>
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
      <thead><tr><th>{$t('printer.name')}</th><th>{$t('rep.jobs')}</th><th>{$t('calc.time_h')}</th></tr></thead>
      <tbody>{#each load as p}<tr><td>{p.name}</td><td class="n">{p.jobs}</td><td class="n">{hms(p.totalSec)}</td></tr>{/each}</tbody>
    </table>
  </section>
</div>

<style>
  .grid { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
  .card { border: 1px solid #ddd; border-radius: 8px; padding: 12px; }
  h3 { margin: 0 0 8px; font-size: 15px; }
  table { font-size: 13px; }
  tr.link { cursor: pointer; }
</style>
