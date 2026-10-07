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
  import Users from './pages/Users.svelte';
  import { t, locale, initLocale, switchLocale } from './lib/i18n';
  import { user, refreshUser, isAdmin } from './lib/session';
  import { wails } from './lib/wails';
  import { onMount } from 'svelte';

  type Tab = 'calc' | 'history' | 'materials' | 'printers' | 'orders' | 'customers' | 'reports' | 'users' | 'settings';
  let tab: Tab = $state('calc');

  onMount(async () => {
    await initLocale();
    await refreshUser();
  });

  const tabs: { id: Tab; key: string }[] = [
    { id: 'calc', key: 'nav.calculator' },
    { id: 'history', key: 'nav.history' },
    { id: 'materials', key: 'nav.materials' },
    { id: 'printers', key: 'nav.printers' },
    { id: 'orders', key: 'nav.orders' },
    { id: 'customers', key: 'nav.customers' },
    { id: 'reports', key: 'nav.reports' },
    { id: 'settings', key: 'nav.settings' },
  ];

  async function logout() {
    try {
      await wails.logout();
    } finally {
      await refreshUser();
    }
  }
</script>

{#if !$user}
  <Login />
{:else}
  <header>
    <h1>{$t('app.title')}</h1>
    <div class="lang">
      <span>{$user.login} ({$user.role})</span>
      <button onclick={logout}>{$t('auth.exit')}</button>
      <button class:active={$locale === 'ru'} onclick={() => switchLocale('ru')}>RU</button>
      <button class:active={$locale === 'en'} onclick={() => switchLocale('en')}>EN</button>
    </div>
  </header>
  <nav>
    {#each tabs as tb}<button class:active={tab === tb.id} onclick={() => (tab = tb.id)}>{$t(tb.key)}</button>{/each}
    {#if isAdmin($user)}<button class:active={tab === 'users'} onclick={() => (tab = 'users')}>{$t('nav.users')}</button>{/if}
  </nav>
  <main>
    {#if tab === 'calc'}<Calculator />
    {:else if tab === 'history'}<History />
    {:else if tab === 'materials'}<Materials />
    {:else if tab === 'printers'}<Printers />
    {:else if tab === 'settings'}<Settings />
    {:else if tab === 'orders'}<Orders />
    {:else if tab === 'customers'}<Customers />
    {:else if tab === 'users' && isAdmin($user)}<Users />
    {:else}<Reports />{/if}
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
</style>
