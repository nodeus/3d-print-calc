<script lang="ts">
  import Calculator from './pages/Calculator.svelte';
  import History from './pages/History.svelte';
  import Materials from './pages/Materials.svelte';
  import Printers from './pages/Printers.svelte';
  import Settings from './pages/Settings.svelte';
  import Login from './pages/Login.svelte';
  import Customers from './pages/Customers.svelte';
  import Orders from './pages/Orders.svelte';
  import Reports from './pages/Reports.svelte';
  import Info from './pages/Info.svelte';
  import { t, locale, initLocale, switchLocale } from './lib/i18n';
  import { user, refreshUser } from './lib/session';
  import { goTab } from './lib/nav';
  import { wails } from './lib/wails';
  import { onMount, onDestroy } from 'svelte';

  type Tab = 'calc' | 'history' | 'materials' | 'printers' | 'orders' | 'customers' | 'reports' | 'info' | 'settings';
  let tab: Tab = $state('calc');
  let tabError = $state('');

  onMount(async () => {
    await initLocale();
    await refreshUser();
  });

  const unsubGoTab = goTab.subscribe((v) => {
    if (v) {
      tabError = '';
      goTab.set(null);
      tab = v as Tab;
    }
  });
  onDestroy(unsubGoTab);

  const tabs: { id: Tab; key: string }[] = [
    { id: 'calc', key: 'nav.calculator' },
    { id: 'history', key: 'nav.history' },
    { id: 'materials', key: 'nav.materials' },
    { id: 'printers', key: 'nav.printers' },
    { id: 'orders', key: 'nav.orders' },
    { id: 'customers', key: 'nav.customers' },
    { id: 'reports', key: 'nav.reports' },
    { id: 'info', key: 'nav.info' },
    { id: 'settings', key: 'nav.settings' },
  ];

  async function logout() {
    try {
      await wails.logout();
    } finally {
      await refreshUser();
    }
  }

  function printPage() {
    window.print();
  }
</script>

{#if !$user}
  <Login />
{:else}
  <header class="no-print">
    <h1>{$t('app.title')}</h1>
    <div class="lang">
      <span>{$user.login} ({$user.role})</span>
      <button onclick={printPage}>{$t('common.print')}</button>
      <button onclick={logout}>{$t('auth.exit')}</button>
      <button class:active={$locale === 'ru'} onclick={() => switchLocale('ru')}>RU</button>
      <button class:active={$locale === 'en'} onclick={() => switchLocale('en')}>EN</button>
    </div>
  </header>
  <div class="print-only"><h1>{$t('app.title')}</h1><p class="print-date">{new Date().toLocaleString()}</p></div>
  <nav class="no-print">
    {#each tabs as tb}<button class:active={tab === tb.id} onclick={() => (tab = tb.id)}>{$t(tb.key)}</button>{/each}
  </nav>
  <main>
    {#key tab}
      <svelte:boundary>
        {#if tab === 'calc'}<Calculator />
        {:else if tab === 'history'}<History />
        {:else if tab === 'materials'}<Materials />
        {:else if tab === 'printers'}<Printers />
        {:else if tab === 'settings'}<Settings />
        {:else if tab === 'orders'}<Orders />
        {:else if tab === 'customers'}<Customers />
        {:else if tab === 'info'}<Info />
        {:else}<Reports />{/if}
        {#snippet failed(err, reset)}
          <p class="err">{$t('app.tab_error')}: {String((err as Error)?.message ?? err)}</p>
          <button onclick={reset}>{$t('common.retry')}</button>
        {/snippet}
      </svelte:boundary>
    {/key}
  </main>
{/if}

<style>
  :global(body) { font-family: system-ui, sans-serif; margin: 0; background: #fafafa; color: #222; }
  header { display: flex; justify-content: space-between; align-items: center; padding: 8px 16px; background: #1f2937; color: white; }
  h1 { font-size: 17px; margin: 0; }
  nav { display: flex; gap: 4px; padding: 8px 16px; background: #e5e7eb; }
  nav button, .lang button { padding: 6px 12px; border: 1px solid #ccc; background: white; cursor: pointer; border-radius: 4px; }
  button.active { background: #1f2937; color: white; border-color: #1f2937; }
  .lang { display: flex; gap: 6px; align-items: center; font-size: 13px; }
  main { padding: 12px 16px; }
  .err { color: #b00; }
  .print-only { display: none; }
  .print-date { color: #666; font-size: 12px; }
  @media print {
    .no-print { display: none !important; }
    .print-only { display: block !important; }
    main { padding: 0; }
    :global(body) { background: white; }
    @page { size: A4; margin: 12mm; }
  }
</style>
