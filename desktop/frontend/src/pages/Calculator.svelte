<script lang="ts">
  import { onMount } from 'svelte';
  import { wails, type CalcInput, type CalcResult, type MaterialDTO, type PrinterDTO } from '../lib/wails';
  import { t, locale } from '../lib/i18n';
  import { money } from '../lib/format';

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

  let result: CalcResult | null = $state(null);
  let pricePerKg = $derived(materials.find((m) => m.code === matCode)?.pricePerKg ?? 0);
  let density = $derived(materials.find((m) => m.code === matCode)?.density ?? 1.27);

  onMount(async () => {
    try {
      materials = await wails.materials();
      printers = await wails.printers();
      currency = await wails.getBaseCurrency();
      if (materials.length && !materials.some((m) => m.code === matCode)) matCode = materials[0].code;
      if (printers.length) selectPrinter(printers[0].ID);
      await recalc();
    } catch (e) {
      console.error(e);
    }
  });

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

  const rows = $derived(
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
    {:else}<p>…</p>{/if}
  </section>
</div>

<style>
  .grid { display: grid; grid-template-columns: 1.2fr 1fr; gap: 12px; }
  .card { border: 1px solid #ddd; border-radius: 8px; padding: 12px; }
  .row { display: flex; gap: 8px; margin-bottom: 8px; flex-wrap: wrap; }
  label { display: flex; flex-direction: column; font-size: 12px; flex: 1; min-width: 110px; }
  input, select { padding: 6px; font-size: 14px; }
  table { width: 100%; border-collapse: collapse; font-size: 14px; }
  td { padding: 3px 4px; border-bottom: 1px solid #eee; }
  td.n { text-align: right; font-variant-numeric: tabular-nums; }
  .totals div { display: flex; justify-content: space-between; padding: 4px 0; }
  .final { font-size: 18px; border-top: 2px solid #333; margin-top: 4px; padding-top: 8px; }
  .percopy { color: #666; font-size: 13px; }
  .ok { color: green; }
  button { padding: 8px 14px; align-self: end; }
</style>
