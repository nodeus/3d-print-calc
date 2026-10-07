<script lang="ts">
  import { onMount } from 'svelte';
  import { wails, type PrinterDTO } from '../lib/wails';
  import { t } from '../lib/i18n';
  import { money, num } from '../lib/format';

  let printers: PrinterDTO[] = $state([]);
  onMount(async () => {
    try {
      printers = await wails.printers();
    } catch (e) {
      console.error(e);
    }
  });
</script>

<table>
  <thead><tr><th>{$t('printer.name')}</th><th>{$t('printer.model')}</th><th>{$t('printer.price')}</th><th>{$t('printer.amort')}</th><th>{$t('printer.method')}</th></tr></thead>
  <tbody>
    {#each printers as p}
      <tr><td>{p.Name}</td><td>{p.Model}</td><td class="n">{money(p.PurchasePrice / 100, p.Currency, 'ru')}</td><td class="n">{num(p.amortPerHour)}</td><td>{p.AmortMethod}</td></tr>
    {/each}
  </tbody>
</table>

<style>
  table { width: 100%; border-collapse: collapse; font-size: 14px; }
  th, td { padding: 4px 6px; border-bottom: 1px solid #eee; text-align: left; }
  td.n { text-align: right; }
</style>
