<script lang="ts">
  import { onMount } from 'svelte';
  import { wails, type Customer } from '../lib/wails';
  import { t } from '../lib/i18n';
  import { user, canWrite } from '../lib/session';

  let list: Customer[] = $state([]);
  let error = $state('');
  let name = $state('');
  let contact = $state('');
  let note = $state('');
  let editId = $state(0);
  let edit: Customer = $state({ ID: 0, Name: '', Contact: '', Currency: 'RUB', Note: '' });

  async function load() {
    try {
      list = (await wails.listCustomers()) ?? [];
    } catch (e) {
      error = String(e);
    }
  }
  onMount(load);

  async function add() {
    error = '';
    if (!name.trim()) return;
    try {
      await wails.saveCustomer({ ID: 0, Name: name.trim(), Contact: contact.trim(), Currency: 'RUB', Note: note.trim() });
      name = '';
      contact = '';
      note = '';
      await load();
    } catch (e) {
      error = String(e);
    }
  }

  function startEdit(c: Customer) {
    editId = c.ID;
    edit = { ...c };
  }

  async function saveEdit() {
    error = '';
    try {
      await wails.saveCustomer(edit);
      editId = 0;
      await load();
    } catch (e) {
      error = String(e);
    }
  }

  async function remove(c: Customer) {
    error = '';
    if (!confirm(`${c.Name}?`)) return;
    try {
      await wails.deleteCustomer(c.ID);
      await load();
    } catch (e) {
      error = String(e);
    }
  }
</script>

{#if canWrite($user)}
  <div class="row no-print">
    <input placeholder={$t('cust.name')} bind:value={name} />
    <input placeholder={$t('cust.contact')} bind:value={contact} />
    <input placeholder={$t('cust.note')} bind:value={note} />
    <button onclick={add}>{$t('common.save')}</button>
  </div>
{/if}
{#if error}<p class="err">{error}</p>{/if}

<table>
  <thead><tr><th>{$t('cust.name')}</th><th>{$t('cust.contact')}</th><th>{$t('cust.note')}</th><th class="no-print"></th></tr></thead>
  <tbody>
    {#each list as c}
      {#if editId === c.ID}
        <tr>
          <td><input bind:value={edit.Name} /></td>
          <td><input bind:value={edit.Contact} /></td>
          <td><input bind:value={edit.Note} /></td>
          <td class="no-print"><button onclick={saveEdit}>{$t('common.save')}</button> <button onclick={() => (editId = 0)}>{$t('common.cancel')}</button></td>
        </tr>
      {:else}
        <tr><td>{c.Name}</td><td>{c.Contact}</td><td>{c.Note}</td>
          <td class="no-print">{#if canWrite($user)}<button onclick={() => startEdit(c)}>✎</button> <button onclick={() => remove(c)}>✕</button>{/if}</td>
        </tr>
      {/if}
    {/each}
  </tbody>
</table>

<style>
  .row { display: flex; gap: 8px; margin-bottom: 10px; flex-wrap: wrap; }
  input, button { padding: 6px; font-size: 14px; }
  table { font-size: 14px; }
  .err { color: #b00; }
</style>
