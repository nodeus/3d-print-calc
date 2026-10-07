<script lang="ts">
  import { onMount } from 'svelte';
  import { wails, type CalcInput, type CalcResult, type MaterialDTO, type PrinterDTO, type Order, type JobDTO, type Customer } from '../lib/wails';
  import { t, locale } from '../lib/i18n';
  import { money } from '../lib/format';
  import { prefillJob } from '../lib/prefill';
  import { downloadPDF } from '../lib/pdf';
  import { get } from 'svelte/store';

  let materials: MaterialDTO[] = $state([]);
  let printers: PrinterDTO[] = $state([]);
  let currency = $state('RUB');

  // Форма
  let matCode = $state('PETG-NIT-W4');
  let printerId = $state(0);
  let timeH = $state(3);
  let timeM = $state(54);
  let weightG: number | null = $state(134.94);
  let filamentMM: number | null = $state(null);
  let tariff = $state(7.08);
  let powerKW = $state(0.35);
  let amortPerHour = $state(5.41);
  let opRate = $state(100);
  let prepH = $state(0);
  let postH = $state(0);
  let designerRate = $state(600);
  let modelH = $state(0);
  let modelPrice = $state(0);
  let extraMat = $state(50);
  let failPct = $state(0);
  let marginProfitPct = $state(80);
  let marginDevPct = $state(35);
  let copies = $state(1);
  let vatPct = $state(0);
  let roundStep = $state(0);
  let fees = $state(100);
  let maintPerHour = $state(0);
  let purgeG = $state(0);
  let filename = $state('');
  let savedMsg = $state(false);
  let orders: Order[] = $state([]);
  let customers: Customer[] = $state([]);
  let customerId = $state(0);
  let orderId = $state(0);
  let orderTitle = $state('');
  let orderQty = $state(1);
  let orderMsg = $state(false);

  let result = $state<CalcResult | null>(null);
  let pricePerKg = $derived(materials.find((m) => m.code === matCode)?.pricePerKg ?? 0);
  let density = $derived(materials.find((m) => m.code === matCode)?.density ?? 1.27);
  let historyJobs: JobDTO[] = $state([]);
  let historyId = $state(0);
  let stubWarn = $state('');

  function snapMaterial(j: JobDTO): string {
    try {
      return JSON.parse(j.CostSnapshot)?.material ?? '';
    } catch {
      return '';
    }
  }

  // Применить работу из истории: материал (если есть в справочнике), вес, время, имя.
  function applyJob(j: JobDTO) {
    const code = snapMaterial(j);
    stubWarn = '';
    if (code && materials.some((m) => m.code === code)) {
      matCode = code;
    } else if (code) {
      stubWarn = `${code}: ${$t('calc.stub_warn')}`;
    }
    weightG = Math.round(j.WeightG * 100) / 100;
    filamentMM = null;
    timeH = Math.floor(j.TotalSec / 3600);
    timeM = Math.round((j.TotalSec % 3600) / 60);
    filename = j.Filename;
    recalc();
  }

  function applyHistorySelect(id: number) {
    const j = historyJobs.find((x) => x.ID === id);
    if (j) applyJob(j);
  }

  onMount(() => {
    loadAll();
    // Подписка: prefill, пришедший при смонтированном калькуляторе.
    return prefillJob.subscribe((j) => {
      if (j) {
        prefillJob.set(null);
        applyJob(j);
      }
    });
  });

  async function loadAll() {
    try {
      materials = (await wails.materials()) ?? [];
      printers = (await wails.printers()) ?? [];
      currency = await wails.getBaseCurrency();
      orders = ((await wails.listOrders()) ?? []).filter((o) => ['draft', 'quoted', 'in_work'].includes(o.Status));
      customers = (await wails.listCustomers()) ?? [];
      if (orders.length) orderId = orders[0].ID;
      historyJobs = (await wails.jobs(0)) ?? [];
      if (materials.length && !materials.some((m) => m.code === matCode)) matCode = materials[0].code;
      if (printers.length) selectPrinter(printers[0].ID);
      // Переход из истории: применить висящий prefill (разовый).
      const pre = get(prefillJob);
      if (pre) {
        prefillJob.set(null);
        applyJob(pre);
      } else {
        await recalc();
      }
    } catch (e) {
      console.error(e);
    }
  }

  function selectPrinter(id: number) {
    printerId = id;
    const p = printers.find((x) => x.ID === id);
    if (p) {
      amortPerHour = Math.round(p.amortPerHour * 100) / 100;
      powerKW = p.PowerKW;
    }
  }

  async function syncWeightFromFilament() {
    if (filamentMM == null || filamentMM <= 0) return;
    try {
      weightG = Math.round((await wails.weightFromLength(density, 1.75, filamentMM)) * 100) / 100;
    } catch (e) {
      console.error(e);
    }
  }

  function buildInput(): CalcInput {
    return {
      TotalSec: timeH * 3600 + timeM * 60,
      WeightG: weightG ?? 0,
      PricePerKg: pricePerKg,
      Tariff: tariff,
      PowerKW: powerKW,
      AmortPerHour: amortPerHour,
      OpRate: opRate,
      PrepH: prepH,
      PostH: postH,
      DesignerRate: designerRate,
      ModelH: modelH,
      ModelPrice: modelPrice,
      ExtraMat: extraMat,
      FailRate: failPct / 100,
      MarginProfit: marginProfitPct / 100,
      MarginDev: marginDevPct / 100,
      Fees: fees,
      MaintPerHour: maintPerHour,
      PurgeWasteG: purgeG,
      Copies: copies,
      VATRate: vatPct / 100,
      RoundStep: roundStep,
    };
  }

  let timer: number | undefined;
  async function recalc() {
    window.clearTimeout(timer);
    timer = window.setTimeout(async () => {
      try {
        result = await wails.calc(buildInput());
      } catch (e) {
        console.error(e);
      }
    }, 120);
  }

  function orderNo(o: Order): string {
    return o.Number || `#${o.ID}`;
  }

  function custName(id: number): string {
    return customers.find((c) => c.ID === id)?.Name ?? '';
  }

  let visibleOrders = $derived(orders.filter((o) => !customerId || o.CustomerID === customerId));

  function pickCustomer(id: number) {
    customerId = id;
    const first = orders.filter((o) => !id || o.CustomerID === id)[0];
    orderId = first ? first.ID : 0;
  }

  async function save() {
    if (!result) return;
    try {
      await wails.saveJob({
        filename,
        printerId,
        spoolId: 0,
        status: 'completed',
        totalSec: Math.round(timeH * 3600 + timeM * 60),
        weightG: weightG ?? 0,
        lengthM: (filamentMM ?? 0) / 1000,
        cost: result.Cost,
        price: result.PriceVAT,
        currency,
      });
      savedMsg = true;
      setTimeout(() => (savedMsg = false), 2500);
    } catch (e) {
      console.error(e);
    }
  }

  async function addToOrder() {
    if (!result || !orderId || !orderTitle.trim()) return;
    try {
      await wails.addOrderItem({
        orderId, jobId: 0, title: orderTitle.trim(), qty: orderQty,
        cost: Math.round(result.Cost * 100), price: Math.round(result.PriceVAT * 100),
      });
      orderMsg = true;
      setTimeout(() => (orderMsg = false), 2500);
    } catch (e) {
      console.error(e);
    }
  }

  async function toPDF() {
    if (!result) return;
    const m = (v: number) => money(v, currency, $locale);
    await downloadPDF($t('nav.calculator'), [
      {
        Title: `${matCode} · ${filename || '—'}`,
        Headers: [$t('calc.breakdown'), $t('calc.cost')],
        Rows: [
          ...rows.map(([k, v]) => [$t(k), m(v)]),
          [$t('calc.cost'), m(result.Cost)],
          [$t('calc.price'), m(result.Price)],
          [$t('calc.final'), m(result.PriceVAT)],
        ],
      },
    ]);
  }

  const rows: [string, number][] = $derived(
    result
      ? [
          ['calc.row_mat', result.MatCost],
          ['calc.row_purge', result.PurgeCost],
          ['calc.row_elec', result.ElecCost],
          ['calc.row_amort', result.Amort],
          ['calc.row_maint', result.Maint],
          ['calc.row_labor', result.Labor],
          ['calc.row_fail', result.FailAdj],
          ['calc.row_subtotal', result.Subtotal],
        ]
      : [],
  );
</script>

<div class="grid">
  <section class="card">
    {#if !materials.length}<p class="warn">{$t('calc.no_materials')}</p>{/if}
    {#if !printers.length}<p class="warn">{$t('calc.no_printers')}</p>{/if}
    <div class="row">
      <label>{$t('calc.job')}
        <select value={historyId} onchange={(e) => applyHistorySelect(Number(e.currentTarget.value))}>
          <option value={0}>—</option>
          {#each historyJobs as j}<option value={j.ID}>{j.Filename} ({j.StartAt})</option>{/each}
        </select>
      </label>
    </div>
    {#if stubWarn}<p class="warn">{stubWarn}</p>{/if}
    <div class="row">
      <label>{$t('calc.material')}
        <select bind:value={matCode} onchange={recalc}>
          {#each materials as m}<option value={m.code}>{m.code} — {m.pricePerKg}/кг</option>{/each}
        </select>
      </label>
      <label>{$t('calc.printer')}
        <select value={printerId} onchange={(e) => { selectPrinter(Number(e.currentTarget.value)); recalc(); }}>
          {#each printers as p}<option value={p.ID}>{p.Name} ({p.AmortMethod})</option>{/each}
        </select>
      </label>
    </div>
    <div class="row">
      <label>{$t('calc.time_h')}<input type="number" min="0" bind:value={timeH} oninput={recalc} /></label>
      <label>{$t('calc.time_m')}<input type="number" min="0" max="59" bind:value={timeM} oninput={recalc} /></label>
      <label>{$t('calc.copies')}<input type="number" min="1" bind:value={copies} oninput={recalc} /></label>
    </div>
    <div class="row">
      <label>{$t('calc.weight')}<input type="number" min="0" step="0.01" bind:value={weightG} oninput={() => { filamentMM = null; recalc(); }} /></label>
      <label>{$t('calc.filament')}<input type="number" min="0" step="0.01" bind:value={filamentMM} oninput={() => { syncWeightFromFilament().then(recalc); }} /></label>
      <label>{$t('calc.purge')}<input type="number" min="0" step="0.1" bind:value={purgeG} oninput={recalc} /></label>
    </div>
    <div class="row">
      <label>{$t('calc.tariff')}<input type="number" min="0" step="0.01" bind:value={tariff} oninput={recalc} /></label>
      <label>{$t('calc.power')}<input type="number" min="0" step="0.01" bind:value={powerKW} oninput={recalc} /></label>
      <label>{$t('calc.amort')}<input type="number" min="0" step="0.01" bind:value={amortPerHour} oninput={recalc} /></label>
      <label>{$t('calc.maint')}<input type="number" min="0" step="0.1" bind:value={maintPerHour} oninput={recalc} /></label>
    </div>
    <div class="row">
      <label>{$t('calc.op_rate')}<input type="number" min="0" bind:value={opRate} oninput={recalc} /></label>
      <label>{$t('calc.prep')}<input type="number" min="0" step="0.1" bind:value={prepH} oninput={recalc} /></label>
      <label>{$t('calc.post')}<input type="number" min="0" step="0.1" bind:value={postH} oninput={recalc} /></label>
    </div>
    <div class="row">
      <label>{$t('calc.designer_rate')}<input type="number" min="0" bind:value={designerRate} oninput={recalc} /></label>
      <label>{$t('calc.model_h')}<input type="number" min="0" step="0.1" bind:value={modelH} oninput={recalc} /></label>
      <label>{$t('calc.model_price')}<input type="number" min="0" bind:value={modelPrice} oninput={recalc} /></label>
      <label>{$t('calc.extra')}<input type="number" min="0" bind:value={extraMat} oninput={recalc} /></label>
    </div>
    <div class="row">
      <label>{$t('calc.fail')}<input type="number" min="0" max="100" bind:value={failPct} oninput={recalc} /></label>
      <label>{$t('calc.margin_profit')}<input type="number" min="0" bind:value={marginProfitPct} oninput={recalc} /></label>
      <label>{$t('calc.margin_dev')}<input type="number" min="0" bind:value={marginDevPct} oninput={recalc} /></label>
      <label>{$t('calc.fees')}<input type="number" min="0" bind:value={fees} oninput={recalc} /></label>
    </div>
    <div class="row">
      <label>{$t('calc.vat')}<input type="number" min="0" bind:value={vatPct} oninput={recalc} /></label>
      <label>{$t('calc.round')}
        <select bind:value={roundStep} onchange={recalc}>
          <option value={0}>{$t('calc.round_none')}</option>
          <option value={1}>1</option>
          <option value={5}>5</option>
          <option value={10}>10</option>
          <option value={50}>50</option>
        </select>
      </label>
    </div>
  </section>

  <section class="card">
      <h3>{$t('calc.breakdown')}</h3>
      {#if result}
      <div class="row no-print"><button onclick={toPDF}>{$t('common.pdf')}</button></div>
      <table>
        <tbody>{#each rows as [key, v]}<tr><td>{$t(key)}</td><td class="n">{money(v, currency, $locale)}</td></tr>{/each}</tbody>
      </table>
      <div class="totals">
        <div><span>{$t('calc.cost')}</span><b>{money(result.Cost, currency, $locale)}</b></div>
        <div><span>{$t('calc.price')}</span><b>{money(result.Price, currency, $locale)}</b></div>
        <div class="final"><span>{$t('calc.final')}</span><b>{money(result.PriceVAT, currency, $locale)}</b></div>
        <div class="percopy">{money(result.CostPerCopy, currency, $locale)} / {money(result.PricePerCopy, currency, $locale)} {$t('calc.per_copy')}</div>
      </div>
      <div class="row">
        <label>{$t('calc.filename')}<input placeholder="part_PETG-NIT-W4_2h1m.gcode" bind:value={filename} /></label>
        <button onclick={save}>{$t('calc.save_calc')}</button>
      </div>
      {#if savedMsg}<p class="ok">{$t('calc.saved')}</p>{/if}
      {#if orders.length}
        <div class="row">
          <label>{$t('cust.name')}
            <select value={customerId} onchange={(e) => pickCustomer(Number(e.currentTarget.value))}>
              <option value={0}>—</option>
              {#each customers as c}<option value={c.ID}>{c.Name}</option>{/each}
            </select>
          </label>
          <label>{$t('ord.number')}
            <select bind:value={orderId}>
              {#each visibleOrders as o}<option value={o.ID}>{orderNo(o)} · {custName(o.CustomerID)} · {$t('st.' + o.Status)}</option>{/each}
            </select>
          </label>
          <label>{$t('ord.item')}<input bind:value={orderTitle} /></label>
          <label>{$t('ord.qty')}<input type="number" min="0.5" step="0.5" bind:value={orderQty} /></label>
          <button onclick={addToOrder}>{$t('ord.add')}</button>
        </div>
        {#if orderMsg}<p class="ok">{$t('ord.added')}</p>{/if}
      {/if}
    {:else}<p>…</p>{/if}
  </section>
</div>

<style>
  .grid { display: grid; grid-template-columns: 1.2fr 1fr; gap: 12px; }
  .card { border: 1px solid #ddd; border-radius: 8px; padding: 12px; }
  .row { display: flex; gap: 8px; margin-bottom: 8px; flex-wrap: wrap; }
  label { display: flex; flex-direction: column; font-size: 12px; flex: 1; min-width: 110px; }
  input, select { padding: 6px; font-size: 14px; }
  table { font-size: 14px; }
  .totals div { display: flex; justify-content: space-between; padding: 4px 0; }
  .final { font-size: 18px; border-top: 2px solid #333; margin-top: 4px; padding-top: 8px; }
  .percopy { color: #666; font-size: 13px; }
  .ok { color: green; }
  .warn { color: #b60; }
  button { padding: 8px 14px; align-self: end; }
</style>
