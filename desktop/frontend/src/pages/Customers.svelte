<script lang="ts">
  import { onMount } from 'svelte';
  import { wails, type Customer } from '../lib/wails';
  import { t } from '../lib/i18n';
  import { user, canWrite } from '../lib/session';

  let list: Customer[] = $state([]);
  let name = $state('');
  let contact = $state('');

  async function load() {
    try {
      list = await wails.listCustomers();
    } catch (e) {
      console.error(e);
    }
  }
  onMount(load);

  async function add() {
    if (!name.trim()) return;
    try {
      await wails.saveCustomer({ ID: 0, Name: name.trim(), Contact: contact.trim(), Currency: 'RUB', Note: '' });
      name = '';
      contact = '';
      await load();
    } catch (e) {
      console.error(e);
    }
  }
</script>

{#if canWrite($user)}
  <div class="row">
    <input placeholder={$t('cust.name')} bind:value={name} />
    <input placeholder={$t('cust.contact')} bind:value={contact} />
    <button onclick={add}>{$t('common.save')}</button>
  </div>
{/if}
<table>
  <thead><tr><th>{$t('cust.name')}</th><th>{$t('cust.contact')}</th></tr></thead>
  <tbody>{#each list as c}<tr><td>{c.Name}</td><td>{c.Contact}</td></tr>{/each}</tbody>
</table>

<style>
  .row { display: flex; gap: 8px; margin-bottom: 10px; }
  input, button { padding: 6px; font-size: 14px; }
  table { width: 100%; border-collapse: collapse; font-size: 14px; }
  th, td { padding: 4px 6px; border-bottom: 1px solid #eee; text-align: left; }
</style>
