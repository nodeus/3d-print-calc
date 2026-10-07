<script lang="ts">
  import { onMount } from 'svelte';
  import { wails } from '../lib/wails';
  import { t, locale, switchLocale } from '../lib/i18n';

  let currency = $state('RUB');
  const codes = ['RUB', 'USD', 'EUR', 'KZT', 'BYN'];

  onMount(async () => {
    try {
      currency = await wails.getBaseCurrency();
    } catch (e) {
      console.error(e);
    }
  });

  async function changeCurrency(code: string) {
    currency = code;
    try {
      await wails.setBaseCurrency(code);
    } catch (e) {
      console.error(e);
    }
  }
</script>

<div class="card">
  <label>{$t('set.language')}
    <select value={$locale} onchange={(e) => switchLocale(e.currentTarget.value)}>
      <option value="ru">Русский</option>
      <option value="en">English</option>
    </select>
  </label>
  <label>{$t('set.currency')}
    <select value={currency} onchange={(e) => changeCurrency(e.currentTarget.value)}>
      {#each codes as c}<option value={c}>{c}</option>{/each}
    </select>
  </label>
  <p class="note">{$t('set.rates_note')}</p>
</div>

<style>
  .card { border: 1px solid #ddd; border-radius: 8px; padding: 12px; max-width: 480px; }
  label { display: flex; flex-direction: column; margin-bottom: 8px; font-size: 13px; }
  select { padding: 6px; font-size: 14px; }
  .note { color: #666; font-size: 13px; }
</style>
