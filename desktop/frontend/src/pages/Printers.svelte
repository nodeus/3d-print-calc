<script lang="ts">
  import { onMount } from 'svelte';
  import { wails, type PrinterDTO, type Printer, type ImportReport } from '../lib/wails';
  import { t } from '../lib/i18n';
  import { user, canWrite } from '../lib/session';
  import { downloadPDF, registerPdfExporter } from '../lib/pdf';
  import { money, num } from '../lib/format';

  let printers: PrinterDTO[] = $state([]);
  let error = $state('');
  let editId = $state(0);
  let form: Printer = $state(blank());
  let isNew = $state(false);

  // Moonraker
  let urls: Record<number, string> = $state({});
  let status: Record<number, string> = $state({});
  let report: ImportReport | null = $state(null);
  let applied: ImportReport | null = $state(null);
  let importing = $state(false);
  let progress = $state({ done: 0, total: 0 });

  function blank(): Printer {
    return {
      ID: 0, Name: '', Model: '', PurchasePrice: 0, Currency: 'RUB',
      CommissioningDate: '', LifeYears: 3, HoursPerDay: 8,
      CalendarMode: 'calendar', AmortMethod: 'calendar', PowerKW: 0.35,
      MaintPerHour: 0, ResourceHours: 0, MoonrakerURL: '',
    };
  }

  async function load() {
    try {
      printers = (await wails.printers()) ?? [];
      for (const p of printers) urls[p.ID] = p.MoonrakerURL ?? '';
    } catch (e) {
      error = String(e);
    }
  }
  onMount(() => {
    load();
    registerPdfExporter(toPDF);
    return () => registerPdfExporter(null);
  });

  function startAdd() {
    form = blank();
    isNew = true;
    editId = 0;
  }

  function startEdit(p: PrinterDTO) {
    form = {
      ID: p.ID, Name: p.Name, Model: p.Model, PurchasePrice: p.PurchasePrice / 100, Currency: p.Currency,
      CommissioningDate: p.CommissioningDate, LifeYears: p.LifeYears, HoursPerDay: p.HoursPerDay,
      CalendarMode: p.CalendarMode, AmortMethod: p.AmortMethod, PowerKW: p.PowerKW,
      MaintPerHour: p.MaintPerHour / 100, ResourceHours: p.ResourceHours, MoonrakerURL: p.MoonrakerURL ?? '',
    };
    isNew = false;
    editId = p.ID;
  }

  async function save() {
    error = '';
    try {
      const payload = {
        ...form,
        PurchasePrice: Math.round(form.PurchasePrice * 100),
        MaintPerHour: Math.round(form.MaintPerHour * 100),
      };
      if (isNew) await wails.addPrinter(payload);
      else await wails.updatePrinter(payload);
      editId = 0;
      isNew = false;
      await load();
    } catch (e) {
      error = String(e);
    }
  }

  async function remove(p: PrinterDTO) {
    error = '';
    if (!confirm(`${p.Name}?`)) return;
    try {
      await wails.deletePrinter(p.ID);
      await load();
    } catch (e) {
      error = String(e);
    }
  }

  async function saveURL(p: PrinterDTO) {
    error = '';
    try {
      await wails.setMoonrakerURL(p.ID, (urls[p.ID] ?? '').trim());
      await load();
    } catch (e) {
      error = String(e);
    }
  }

  async function ping(p: PrinterDTO) {
    error = '';
    try {
      status[p.ID] = await wails.moonrakerStatus(p.ID);
    } catch (e) {
      error = String(e);
      status[p.ID] = '';
    }
  }

  async function preview(p: PrinterDTO) {
    error = '';
    applied = null;
    try {
      report = await wails.moonrakerPreview(p.ID, 100);
    } catch (e) {
      error = String(e);
    }
  }

  async function apply() {
    error = '';
    importing = true;
    progress = { done: 0, total: 0 };
    wails.onImportProgress((done, total) => {
      progress = { done, total };
    });
    try {
      applied = await wails.moonrakerApply();
      report = null;
    } catch (e) {
      error = String(e);
    } finally {
      importing = false;
      wails.offImportProgress();
    }
  }

  async function toPDF() {
    await downloadPDF($t('nav.printers'), [
      {
        Title: '',
        Headers: [$t('printer.name'), $t('printer.model'), $t('printer.price'), $t('printer.amort'), $t('printer.method')],
        Rows: printers.map((p) => [p.Name, p.Model, money(p.PurchasePrice / 100, p.Currency, 'ru'), num(p.amortPerHour), p.AmortMethod]),
      },
    ]);
  }
</script>

{#if canWrite($user) && !editId && !isNew}
  <div class="row no-print"><button onclick={startAdd}>{$t('printer.add')}</button></div>
{/if}
{#if error}<p class="err">{error}</p>{/if}

{#if editId || isNew}
  <div class="card no-print">
    <div class="row">
      <label>{$t('printer.name')}<input bind:value={form.Name} /></label>
      <label>{$t('printer.model')}<input bind:value={form.Model} /></label>
      <label>{$t('printer.price')}<input type="number" min="0" bind:value={form.PurchasePrice} /></label>
    </div>
    <div class="row">
      <label>{$t('printer.date')}<input type="date" bind:value={form.CommissioningDate} /></label>
      <label>{$t('printer.life')}<input type="number" min="1" bind:value={form.LifeYears} /></label>
      <label>{$t('printer.hpd')}<input type="number" min="1" max="24" bind:value={form.HoursPerDay} /></label>
    </div>
    <div class="row">
      <label>{$t('printer.method')}
        <select bind:value={form.AmortMethod}>
          <option value="calendar">calendar</option>
          <option value="workdays">workdays</option>
          <option value="resource">resource</option>
        </select>
      </label>
      <label>{$t('printer.power')}<input type="number" min="0" step="0.01" bind:value={form.PowerKW} /></label>
      <label>{$t('printer.maint')}<input type="number" min="0" bind:value={form.MaintPerHour} /></label>
      <label>{$t('printer.resource')}<input type="number" min="0" bind:value={form.ResourceHours} /></label>
    </div>
    <div class="row">
      <button onclick={save}>{$t('common.save')}</button>
      <button onclick={() => { editId = 0; isNew = false; }}>{$t('common.cancel')}</button>
    </div>
  </div>
{/if}

<table>
  <thead><tr><th>{$t('printer.name')}</th><th>{$t('printer.model')}</th><th>{$t('printer.price')}</th><th>{$t('printer.amort')}</th><th>{$t('printer.method')}</th><th class="no-print"></th></tr></thead>
  <tbody>
    {#each printers as p}
      <tr><td>{p.Name}</td><td>{p.Model}</td><td class="n">{money(p.PurchasePrice / 100, p.Currency, 'ru')}</td><td class="n">{num(p.amortPerHour)}</td><td>{p.AmortMethod}</td>
        <td class="no-print">{#if canWrite($user)}<button onclick={() => startEdit(p)}>✎</button> <button onclick={() => remove(p)}>✕</button>{/if}</td>
      </tr>
    {/each}
  </tbody>
</table>

<h3>{$t('moon.title')}</h3>
{#each printers as p}
  <div class="card">
    <b>{p.Name}</b>
    {#if canWrite($user)}
      <div class="row">
        <input placeholder="http://192.168.1.50:7125" bind:value={urls[p.ID]} />
        <button onclick={() => saveURL(p)}>{$t('common.save')}</button>
        <button onclick={() => ping(p)}>{$t('moon.check')}</button>
        <button onclick={() => preview(p)}>{$t('moon.import')}</button>
      </div>
    {/if}
    {#if status[p.ID]}<p class="ok">Moonraker {status[p.ID]}</p>{/if}
  </div>
{/each}
{#if report}<p>{$t('set.csv_preview')}: total {report.total}, new {report.new}, existing {report.existing}</p>{/if}
{#if report?.dryRun && canWrite($user)}<button class="warn no-print" onclick={apply} disabled={importing}>{$t('set.csv_apply')}</button>{/if}
{#if importing}<progress value={progress.done} max={progress.total || 1}></progress> {progress.done}/{progress.total}{/if}
{#if applied}<p class="ok">{$t('set.csv_done')}: +{applied.new} (stubs {applied.stubs})</p>{/if}

<style>
  .card { border: 1px solid #ddd; border-radius: 8px; padding: 10px; margin-bottom: 8px; max-width: 720px; }
  .row { display: flex; gap: 6px; margin: 6px 0; flex-wrap: wrap; }
  label { display: flex; flex-direction: column; font-size: 12px; flex: 1; min-width: 120px; }
  input, select, button { padding: 6px; font-size: 14px; }
  table { font-size: 14px; margin-bottom: 12px; }
  button.warn { border-color: #b00; color: #b00; }
  .ok { color: green; }
  .err { color: #b00; }
</style>
