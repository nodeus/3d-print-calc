<script lang="ts">
  import Calculator from './pages/Calculator.svelte';
  import History from './pages/History.svelte';
  import Materials from './pages/Materials.svelte';
  import Printers from './pages/Printers.svelte';
  import Settings from './pages/Settings.svelte';
  import { t, locale, initLocale, switchLocale } from './lib/i18n';
  import { onMount } from 'svelte';

  type Tab = 'calc' | 'history' | 'materials' | 'printers' | 'orders' | 'reports' | 'settings';
  let tab: Tab = $state('calc');

  onMount(() => {
    initLocale();
  });

  const tabs: { id: Tab; key: string }[] = [
    { id: 'calc', key: 'nav.calculator' },
    { id: 'history', key: 'nav.history' },
    { id: 'materials', key: 'nav.materials' },
    { id: 'printers', key: 'nav.printers' },
    { id: 'orders', key: 'nav.orders' },
    { id: 'reports', key: 'nav.reports' },
    { id: 'settings', key: 'nav.settings' },
  ];
</script>

<header>
  <h1>{$t('app.title')}</h1>
  <div class="lang">
    <button class:active={$locale === 'ru'} onclick={() => switchLocale('ru')}>RU</button>
    <button class:active={$locale === 'en'} onclick={() => switchLocale('en')}>EN</button>
  </div>
</header>
<nav>
  {#each tabs as tb}<button class:active={tab === tb.id} onclick={() => (tab = tb.id)}>{$t(tb.key)}</button>{/each}
</nav>
<main>
  {#if tab === 'calc'}<Calculator />
  {:else if tab === 'history'}<History />
  {:else if tab === 'materials'}<Materials />
  {:else if tab === 'printers'}<Printers />
  {:else if tab === 'settings'}<Settings />
  {:else if tab === 'orders'}<p>{$t('orders.soon')}</p>
  {:else}<p>{$t('reports.soon')}</p>{/if}
</main>

<style>
  :global(body) { font-family: system-ui, sans-serif; margin: 0; background: #fafafa; color: #222; }
  header { display: flex; justify-content: space-between; align-items: center; padding: 8px 16px; background: #1f2937; color: white; }
  h1 { font-size: 17px; margin: 0; }
  nav { display: flex; gap: 4px; padding: 8px 16px; background: #e5e7eb; }
  nav button, .lang button { padding: 6px 12px; border: 1px solid #ccc; background: white; cursor: pointer; border-radius: 4px; }
  button.active { background: #1f2937; color: white; border-color: #1f2937; }
  main { padding: 12px 16px; }
</style>
