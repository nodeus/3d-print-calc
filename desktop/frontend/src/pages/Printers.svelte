<script lang="ts">
  import { onMount } from 'svelte';
  import { wails, type PrinterDTO, type ImportReport } from '../lib/wails';
  import { t } from '../lib/i18n';
  import { user, canWrite } from '../lib/session';
  import { money, num } from '../lib/format';

  let printers: PrinterDTO[] = $state([]);
  let urls: Record<number, string> = $state({});
  let status: Record<number, string> = $state({});
  let report: ImportReport | null = $state(null);
  let applied: ImportReport | null = $state(null);
  let error = $state('');

  async function load() {
    try {
      printers = await wails.printers();
      for (const p of printers) urls[p.ID] = p.MoonrakerURL ?? '';
    } catch (e) {
      console.error(e);
    }
  }
  onMount(load);

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
    try {
      applied = await wails.moonrakerApply();
      report = null;
    } catch (e) {
      error = String(e);
    }
  }
</script>

<table>
  <thead><tr><th>{$t('printer.name')}</th><th>{$t('printer.model')}</th><th>{$t('printer.price')}</th><th>{$t('printer.amort')}</th><th>{$t('printer.method')}</th></tr></thead>
  <tbody>
    {#each printers as p}
      <tr><td>{p.Name}</td><td>{p.Model}</td><td class="n">{money(p.PurchasePrice / 100, p.Currency, 'ru')}</td><td class="n">{num(p.amortPerHour)}</td><td>{p.AmortMethod}</td></tr>
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
{#if report?.dryRun && canWrite($user)}<button class="warn" onclick={apply}>{$t('set.csv_apply')}</button>{/if}
{#if applied}<p class="ok">{$t('set.csv_done')}: +{applied.new} (stubs {applied.stubs})</p>{/if}
{#if error}<p class="err">{error}</p>{/if}

<style>
  table { width: 100%; border-collapse: collapse; font-size: 14px; margin-bottom: 12px; }
  th, td { padding: 4px 6px; border-bottom: 1px solid #eee; text-align: left; }
  td.n { text-align: right; }
  .card { border: 1px solid #ddd; border-radius: 8px; padding: 10px; margin-bottom: 8px; max-width: 640px; }
  .row { display: flex; gap: 6px; margin-top: 6px; }
  input { flex: 1; padding: 6px; }
  button { padding: 6px 12px; }
  button.warn { border-color: #b00; color: #b00; }
  .ok { color: green; }
  .err { color: #b00; }
</style>
