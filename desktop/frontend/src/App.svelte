<script lang="ts">
  import { get } from 'svelte/store';
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
  import Confirm from './lib/Confirm.svelte';
  import { t, initLocale } from './lib/i18n';
  import { user, refreshUser } from './lib/session';
  import { loadTheme } from './lib/theme';
  import { applyMySettings } from './lib/usersettings';
  import { goTab } from './lib/nav';
  import { exportCurrentTab, hasPdfExporter } from './lib/pdf';
  import { wails } from './lib/wails';
  import { onMount, onDestroy } from 'svelte';

  type Tab = 'calc' | 'history' | 'materials' | 'printers' | 'orders' | 'customers' | 'reports' | 'info' | 'settings';
  let tab: Tab = $state('calc');
  let tabError = $state('');

  onMount(async () => {
    window.addEventListener('resize', saveWinSize);
    await initLocale();
    await loadTheme();
    await refreshUser();
    // Сессия могла сохраниться: подтянуть настройки пользователя поверх глобальных.
    if (get(user)) await applyMySettings();
  });
  onDestroy(() => window.removeEventListener('resize', saveWinSize));

  // Живое сохранение размера окна (дебаунс): переживает даже убитый
  // рантайм на выходе. Наружные единицы WindowGetSize = единицы SetSize.
  let winTimer: number | undefined = $state(undefined);
  function saveWinSize() {
    window.clearTimeout(winTimer);
    winTimer = window.setTimeout(async () => {
      try {
        const s = await window.runtime.WindowGetSize();
        if (s && s.w > 0 && s.h > 0) await wails.saveWindowSize(s.w, s.h);
      } catch {
        /* рантайм недоступен (браузер) — молча */
      }
    }, 500);
  }

  const unsubGoTab = goTab.subscribe((v) => {
    if (v) {
      tabError = '';
      goTab.set(null);
      tab = v as Tab;
    }
  });
  onDestroy(unsubGoTab);

  // Текущая вкладка на body — для альбомной печати заказов/отчётов.
  $effect(() => {
    document.body.dataset.tab = tab;
  });

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
      {#if $hasPdfExporter}<button onclick={exportCurrentTab}>{$t('common.pdf')}</button>{/if}
      <button onclick={logout}>{$t('auth.exit')}</button>
    </div>
  </header>
  <div class="print-only"><h1>{$t('app.title')}</h1><p class="print-date">{new Date().toLocaleString()}</p></div>
  <nav class="no-print">
    {#each tabs as tb}<button class:active={tab === tb.id} onclick={() => (tab = tb.id)}>{$t(tb.key)}</button>{/each}
  </nav>
  <main>
    {#key tab}
      <svelte:boundary>        {#if tab === 'calc'}<Calculator />
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
  <Confirm />
{/if}

<style>
  :global(body) { font-family: system-ui, sans-serif; margin: 0; background: var(--bg); color: var(--fg); }
  header { display: flex; justify-content: space-between; align-items: center; padding: 8px 16px; background: var(--header-bg); color: var(--header-fg); }
  h1 { font-size: 17px; margin: 0; }
  nav { display: flex; gap: 4px; padding: 8px 16px; background: var(--nav-bg); }
  nav button, .lang button { padding: 6px 12px; border: 1px solid var(--btn-border); background: var(--btn-bg); color: var(--btn-fg); cursor: pointer; border-radius: 4px; }
  button.active { background: var(--header-bg); color: var(--header-fg); border-color: var(--header-bg); }
  .lang { display: flex; gap: 6px; align-items: center; font-size: 13px; }
  main { padding: 12px 16px; }
  .err { color: var(--err); }
  .print-only { display: none; }
  .print-date { color: var(--muted); font-size: 12px; }
  @media print {
    .no-print { display: none !important; }
    .print-only { display: block !important; }
    main { padding: 0; }
    :global(body) { background: white; }
    /* Строго портрет A4: именованных альбомных страниц нет —
       их частичная поддержка в Chromium давала пустые страницы. */
    @page { size: A4 portrait; margin: 12mm; }
  }
  @media print {
    :global(.cols) { grid-template-columns: 1fr !important; }
    :global(.grid) { grid-template-columns: 1fr !important; }
  }
</style>
