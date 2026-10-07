<script lang="ts">
  import { onMount } from 'svelte';
  import { wails, type MaterialDTO } from '../lib/wails';
  import { t } from '../lib/i18n';
  import { money } from '../lib/format';

  let materials: MaterialDTO[] = $state([]);
  onMount(async () => {
    try {
      materials = await wails.materials();
    } catch (e) {
      console.error(e);
    }
  });
</script>

<table>
  <thead><tr><th>{$t('mat.code')}</th><th>{$t('mat.type')}</th><th>{$t('mat.price')}</th><th>{$t('mat.density')}</th></tr></thead>
  <tbody>
    {#each materials as m}
      <tr><td>{m.code}</td><td>{m.type}</td><td class="n">{money(m.pricePerKg, m.currency, 'ru')}</td><td class="n">{m.density}</td></tr>
    {/each}
  </tbody>
</table>

<style>
  table { width: 100%; border-collapse: collapse; font-size: 14px; }
  th, td { padding: 4px 6px; border-bottom: 1px solid #eee; text-align: left; }
  td.n { text-align: right; }
</style>
